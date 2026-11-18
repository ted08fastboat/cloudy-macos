package player

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/audio"

	"netease-music-mygo/internal/netease"
)

// Many songs come from NetEase without their loudness: gain 0, and only
// the peak, lossless ones above all. Taken as measured at 0 dB, they played
// some 4 dB louder than the songs it measured, and not evened out among
// themselves; songs played from files had no gain at all. The player now
// measures those itself, the way ReplayGain 2 does (EBU R128's integrated
// loudness, against -18 LUFS), and keeps what it measured.

// loudRef is the loudness ReplayGain 2 brings tracks to, as NetEase does:
// a track's gain is loudRef less its integrated loudness.
const loudRef = -18.0

// unknownGain is the gain a track is given until its loudness is known:
// about what NetEase measures of most songs, which are mastered loud, so an
// unmeasured one plays about as loud as the others meanwhile.
const unknownGain = -8.0

// loudGlide is how long a track's gain takes to move to what was measured
// of it as it plays.
const loudGlide = 2 * time.Second

// loudness is what is known of how loud a track is: its gain, in dB, and
// its peak, 1 at full scale, or 0 if unknown.
type loudness struct {
	Gain float64
	Peak float64
}

// streamLoudness returns what NetEase says of a stream's loudness, if it
// measured it. Gain 0 is what it sends of songs it did not.
func streamLoudness(st *netease.Stream) (loudness, bool) {
	if st == nil || st.Gain == 0 {
		return loudness{}, false
	}
	return loudness{Gain: st.Gain, Peak: st.Peak}, true
}

// loudGain is the gain that brings a track of loudness l as loud as
// others, but never so loud that its peaks would clip.
func loudGain(l loudness) float64 {
	g := math.Pow(10, (l.Gain+normPreamp)/20)
	if l.Peak > 0 {
		g = min(g, 1/l.Peak)
	}
	return g
}

// Loudness keeps what was measured of how loud tracks are, in a small file,
// for them to play evened out from their start the next time.
type Loudness struct {
	path string

	mu    sync.Mutex
	known map[string]loudness
	due   bool // a save is waiting to run
}

// loudLimit is how many tracks the file keeps at most: some 30 bytes each.
const loudLimit = 30000

// OpenLoudness reads the loudness kept in the file at path, which need
// not exist yet. With path "", nothing is kept past the run.
func OpenLoudness(path string) *Loudness {
	l := &Loudness{path: path, known: map[string]loudness{}}
	if path == "" {
		return l
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return l
	}
	var kept map[string][2]float64
	if json.Unmarshal(b, &kept) != nil {
		return l
	}
	for k, v := range kept {
		if validLoudness(loudness{Gain: v[0], Peak: v[1]}) {
			l.known[k] = loudness{Gain: v[0], Peak: v[1]}
		}
	}
	return l
}

func validLoudness(l loudness) bool {
	return l.Gain >= -40 && l.Gain <= 40 && l.Peak >= 0 && l.Peak <= 4 && !math.IsNaN(l.Gain) && !math.IsNaN(l.Peak)
}

func (l *Loudness) get(key string) (loudness, bool) {
	if l == nil || key == "" {
		return loudness{}, false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	v, ok := l.known[key]
	return v, ok
}

// put keeps v as the loudness of the track of key, saving the file a
// little later, once for all that are put meanwhile.
func (l *Loudness) put(key string, v loudness) {
	if l == nil || key == "" || !validLoudness(v) {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if old, ok := l.known[key]; ok && old == v {
		return
	}
	if len(l.known) >= loudLimit {
		// Over the limit: some go, any of them.
		n := len(l.known) - loudLimit + loudLimit/10
		for k := range l.known {
			if n <= 0 {
				break
			}
			delete(l.known, k)
			n--
		}
	}
	l.known[key] = v
	if l.path == "" || l.due {
		return
	}
	l.due = true
	time.AfterFunc(5*time.Second, l.save)
}

// save writes the file, in place of the one before.
func (l *Loudness) save() {
	l.mu.Lock()
	kept := make(map[string][2]float64, len(l.known))
	for k, v := range l.known {
		kept[k] = [2]float64{math.Round(v.Gain*100) / 100, math.Round(v.Peak*10000) / 10000}
	}
	l.due = false
	l.mu.Unlock()
	b, err := json.Marshal(kept)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return
	}
	tmp := l.path + ".part"
	if os.WriteFile(tmp, b, 0o600) != nil {
		os.Remove(tmp)
		return
	}
	if os.Rename(tmp, l.path) != nil {
		os.Remove(tmp)
	}
}

// loudKey names a track among those whose loudness is kept: a NetEase
// song or episode by its id, a file of the listener's by its path, size
// and time, so that a file changed is measured again. "" for none.
func loudKey(t netease.Track, path string) string {
	if path != "" {
		info, err := os.Stat(path)
		if err != nil {
			return ""
		}
		return "f" + strings.ToLower(filepath.Clean(path)) + "|" + strconv.FormatInt(info.Size(), 10) + "|" + strconv.FormatInt(info.ModTime().Unix(), 10)
	}
	if t.ProgramID != 0 {
		return "p" + strconv.FormatInt(t.ProgramID, 10)
	}
	if t.ID > 0 {
		return "n" + strconv.FormatInt(t.ID, 10)
	}
	return ""
}

// measuring lets one track be measured at a time: decoding a whole song
// is work, and the one playing is the one that matters.
var measuring = make(chan struct{}, 1)

// measure decodes s to its end and returns its loudness.
func measure(ctx context.Context, s audio.Decoder) (loudness, error) {
	select {
	case measuring <- struct{}{}:
	case <-ctx.Done():
		return loudness{}, ctx.Err()
	}
	defer func() { <-measuring }()
	m := newLoudMeter(s.SampleRate(), s.Channels())
	buf := make([]float32, 8192*s.Channels())
	for n := 0; ; n++ {
		if n%16 == 0 {
			if err := ctx.Err(); err != nil {
				return loudness{}, err
			}
		}
		k, err := s.Read(buf)
		m.feed(buf[:k])
		if err == io.EOF {
			break
		}
		if err != nil {
			return loudness{}, err
		}
		if k == 0 {
			return loudness{}, io.ErrNoProgress
		}
	}
	lufs, ok := m.integrated()
	if !ok {
		return loudness{}, errSilent
	}
	return loudness{Gain: loudRef - lufs, Peak: m.peak}, nil
}

var errSilent = errors.New("player: nothing to measure")

// loudMeter measures integrated loudness as ITU-R BS.1770-4 and EBU R128
// define it: K-weighted, in blocks of 400 ms overlapping by 75 %, gated
// at -70 LUFS and at 10 LU under the loudness of what passed that.
type loudMeter struct {
	channels int
	pre, rlb [][4]float64 // the filters' state, for each channel: x1 x2 y1 y2
	// The two filters' coefficients: b0 b1 b2 a1 a2.
	preB, rlbB [5]float64

	step  int       // frames in 100 ms
	n     int       // frames in the step under way
	sum   float64   // its weighted squares, of all channels
	steps []float64 // the mean square of each step done
	peak  float64
}

func newLoudMeter(rate, channels int) *loudMeter {
	m := &loudMeter{channels: channels, pre: make([][4]float64, channels), rlb: make([][4]float64, channels)}
	m.step = max(1, rate/10)
	fs := float64(rate)
	// The high shelf that models the head, and the high pass of the RLB
	// curve, at any rate, as libebur128 works them out.
	f0, gain, q := 1681.974450955533, 3.999843853973347, 0.7071752369554196
	k := math.Tan(math.Pi * f0 / fs)
	vh := math.Pow(10, gain/20)
	vb := math.Pow(vh, 0.4996667741545416)
	a0 := 1 + k/q + k*k
	m.preB = [5]float64{(vh + vb*k/q + k*k) / a0, 2 * (k*k - vh) / a0, (vh - vb*k/q + k*k) / a0, 2 * (k*k - 1) / a0, (1 - k/q + k*k) / a0}
	f0, q = 38.13547087602444, 0.5003270373238773
	k = math.Tan(math.Pi * f0 / fs)
	a0 = 1 + k/q + k*k
	m.rlbB = [5]float64{1, -2, 1, 2 * (k*k - 1) / a0, (1 - k/q + k*k) / a0}
	return m
}

func kStage(c *[5]float64, s *[4]float64, x float64) float64 {
	y := c[0]*x + c[1]*s[0] + c[2]*s[1] - c[3]*s[2] - c[4]*s[3]
	s[1], s[0] = s[0], x
	s[3], s[2] = s[2], y
	return y
}

// feed measures interleaved samples, a whole number of frames.
func (m *loudMeter) feed(samples []float32) {
	ch := m.channels
	for i := 0; i+ch <= len(samples); i += ch {
		for c := 0; c < ch; c++ {
			x := float64(samples[i+c])
			if a := math.Abs(x); a > m.peak {
				m.peak = a
			}
			if c >= 2 {
				continue // as the player, which plays the first two
			}
			y := kStage(&m.rlbB, &m.rlb[c], kStage(&m.preB, &m.pre[c], x))
			m.sum += y * y
		}
		if m.n++; m.n == m.step {
			m.steps = append(m.steps, m.sum/float64(m.step))
			m.n, m.sum = 0, 0
		}
	}
}

func lufsOf(power float64) float64 { return -0.691 + 10*math.Log10(power) }

// integrated returns the integrated loudness, in LUFS, or false if what
// was fed is too short or too quiet to have one.
func (m *loudMeter) integrated() (float64, bool) {
	steps := m.steps
	if m.n > m.step/2 {
		// The end, if it fills most of a step.
		steps = append(steps, m.sum/float64(m.n))
	}
	var blocks []float64
	for i := 0; i+4 <= len(steps); i++ {
		blocks = append(blocks, (steps[i]+steps[i+1]+steps[i+2]+steps[i+3])/4)
	}
	var sum float64
	var n int
	for _, b := range blocks {
		if b > 0 && lufsOf(b) > -70 {
			sum += b
			n++
		}
	}
	if n == 0 {
		return 0, false
	}
	relative := lufsOf(sum/float64(n)) - 10
	sum, n = 0, 0
	for _, b := range blocks {
		if b > 0 && lufsOf(b) > -70 && lufsOf(b) > relative {
			sum += b
			n++
		}
	}
	if n == 0 {
		return 0, false
	}
	return lufsOf(sum / float64(n)), true
}
