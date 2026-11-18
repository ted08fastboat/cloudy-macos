package player

import (
	"context"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"netease-music-mygo/internal/netease"
)

func TestLoudGain(t *testing.T) {
	for _, c := range []struct {
		l    loudness
		want float64
	}{
		{loudness{Gain: -6.6, Peak: 1}, math.Pow(10, -2.6/20)},
		// Quiet, it is lifted, but not past its peak.
		{loudness{Gain: 3, Peak: 0.8}, 1.25},
		{loudness{Gain: -4}, 1},
		{loudness{Gain: unknownGain, Peak: 0.99}, math.Pow(10, (unknownGain+normPreamp)/20)},
	} {
		if g := loudGain(c.l); math.Abs(g-c.want) > 1e-9 {
			t.Errorf("%+v: %v, want %v", c.l, g, c.want)
		}
	}
}

// Gain 0 is what NetEase sends of a song it did not measure, peak or not:
// it is not taken for a song measured at 0 dB.
func TestStreamLoudnessUnknown(t *testing.T) {
	for _, st := range []*netease.Stream{nil, {}, {Peak: 0.98}} {
		if _, ok := streamLoudness(st); ok {
			t.Errorf("%+v taken as measured", st)
		}
	}
	if l, ok := streamLoudness(&netease.Stream{Gain: -9.2, Peak: 0.97}); !ok || l.Gain != -9.2 || l.Peak != 0.97 {
		t.Errorf("measured stream: %+v %v", l, ok)
	}
}

// sine is a decoder of a stereo sine, both channels alike.
type sine struct {
	rate   int
	freq   float64
	amp    float64
	frames int64
	at     int64
}

func (s *sine) SampleRate() int           { return s.rate }
func (s *sine) Channels() int             { return 2 }
func (s *sine) SetPosition(f int64) error { s.at = f; return nil }
func (s *sine) Len() int64                { return s.frames }
func (s *sine) Read(buf []float32) (int, error) {
	n := 0
	for n+2 <= len(buf) && s.at < s.frames {
		v := float32(s.amp * math.Sin(2*math.Pi*s.freq*float64(s.at)/float64(s.rate)))
		buf[n], buf[n+1] = v, v
		n += 2
		s.at++
	}
	if s.at >= s.frames {
		return n, io.EOF
	}
	return n, nil
}

// EBU Tech 3341's first case: a 1 kHz sine at -23 dBFS in both channels
// measures -23 LUFS, at the rates songs come in.
func TestMeasureReference(t *testing.T) {
	for _, rate := range []int{44100, 48000, 96000} {
		amp := math.Pow(10, -23.0/20)
		l, err := measure(context.Background(), &sine{rate: rate, freq: 1000, amp: amp, frames: int64(rate) * 20})
		if err != nil {
			t.Fatal(err)
		}
		if want := loudRef + 23; math.Abs(l.Gain-want) > 0.1 {
			t.Errorf("%d Hz: gain %.3f dB, want %.1f", rate, l.Gain, want)
		}
		if math.Abs(l.Peak-amp) > 1e-3 {
			t.Errorf("%d Hz: peak %.4f, want %.4f", rate, l.Peak, amp)
		}
	}
}

// Silence has no loudness to bring anything to.
func TestMeasureSilence(t *testing.T) {
	if _, err := measure(context.Background(), &sine{rate: 44100, frames: 44100 * 5}); err == nil {
		t.Error("silence measured")
	}
}

// What was measured is kept in the file, and read back from it.
func TestLoudnessKept(t *testing.T) {
	path := filepath.Join(t.TempDir(), "loudness.json")
	l := OpenLoudness(path)
	l.put("n42", loudness{Gain: -7.25, Peak: 0.912})
	l.put("bad", loudness{Gain: math.NaN()})
	l.save()
	again := OpenLoudness(path)
	if v, ok := again.get("n42"); !ok || v.Gain != -7.25 || v.Peak != 0.912 {
		t.Errorf("kept %+v %v", v, ok)
	}
	if _, ok := again.get("bad"); ok {
		t.Error("kept a gain that is no number")
	}
	var none *Loudness
	none.put("n1", loudness{Gain: -3})
	if _, ok := none.get("n1"); ok {
		t.Error("no store keeps nothing")
	}
}

// A song NetEase did not measure starts at the gain of most songs, and is
// measured; one measured before starts at its own gain, by its id or its
// file; one NetEase measured is kept by its id, for its download.
func TestStartGain(t *testing.T) {
	p := New(func(func()) {})
	p.Loud = OpenLoudness("")
	p.Normalize = func() bool { return false }
	song := netease.Track{ID: 7}
	if g := p.startGain(p.gen, song, &netease.Stream{Peak: 0.9}, nil); math.Abs(g-loudGain(loudness{Gain: unknownGain})) > 1e-9 {
		t.Errorf("unknown: %v", g)
	}
	p.Loud.put("n7", loudness{Gain: -2, Peak: 0.5})
	if g := p.startGain(p.gen, song, &netease.Stream{}, nil); math.Abs(g-loudGain(loudness{Gain: -2, Peak: 0.5})) > 1e-9 {
		t.Errorf("measured before: %v", g)
	}
	other := netease.Track{ID: 8}
	if g := p.startGain(p.gen, other, &netease.Stream{Gain: -10, Peak: 1}, nil); math.Abs(g-loudGain(loudness{Gain: -10, Peak: 1})) > 1e-9 {
		t.Errorf("NetEase's: %v", g)
	}
	if v, ok := p.Loud.get("n8"); !ok || v.Gain != -10 {
		t.Errorf("NetEase's not kept: %+v %v", v, ok)
	}
}

// The temporary file of a song streamed without being kept, left by an app
// that crashed, goes once untouched for an hour; a newer one stays.
func TestDropStreamSpools(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "cloudy-stream-1.part")
	fresh := filepath.Join(dir, "cloudy-stream-2.part")
	other := filepath.Join(dir, "other.part")
	for _, p := range []string{old, fresh, other} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	long := time.Now().Add(-2 * time.Hour)
	os.Chtimes(old, long, long)
	os.Chtimes(other, long, long)
	dropPartsLike(dir, streamSpools, time.Hour)
	if _, err := os.Stat(old); err == nil {
		t.Error("the old spool stays")
	}
	for _, p := range []string{fresh, other} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s went", filepath.Base(p))
		}
	}
}
