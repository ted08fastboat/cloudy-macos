package player

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/audio"

	"netease-music-mygo/internal/netease"
)

func TestPlayEmptyQueue(t *testing.T) {
	p := &Player{at: -1}
	p.PlayQueue(nil, 0) // an empty playlist's play button
	if p.Current() != nil {
		t.Error("playing from an empty queue")
	}
}

// Adding to the queue keeps what plays and leaves out songs already in it.
func TestAddToQueueKeepsCurrent(t *testing.T) {
	p := &Player{at: -1}
	p.Cue([]netease.Track{{ID: 1}, {ID: 2}})
	if added := p.AddToQueue([]netease.Track{{ID: 2}, {ID: 3}}); added != 1 {
		t.Fatalf("added %d, want 1", added)
	}
	if q := p.Queue(); len(q) != 3 || q[2].ID != 3 {
		t.Fatalf("queue %+v", q)
	}
	if cur := p.Current(); cur == nil || cur.ID != 1 {
		t.Fatalf("current %+v", cur)
	}
}

// The next cover must use the same order as Next and disappear at a radio's end.
func TestNextIndexUsesPlayOrder(t *testing.T) {
	p := &Player{at: -1}
	p.Cue([]netease.Track{{ID: 1}, {ID: 2}, {ID: 3}})
	p.order = []int{0, 2, 1}
	p.AttachRadio("fm:DEFAULT", func(func([]netease.Track)) {})
	if got := p.NextIndex(); got != 2 {
		t.Fatalf("next index = %d, want 2", got)
	}
	p.at = 2
	if got := p.NextIndex(); got != -1 {
		t.Fatalf("next index at the radio's end = %d, want -1", got)
	}
	p.endRadio()
	if got := p.NextIndex(); got != 0 {
		t.Fatalf("next index after leaving the radio = %d, want 0", got)
	}
}

// silence is a long song of nothing.
type silence struct{}

func (silence) SampleRate() int               { return 48000 }
func (silence) Channels() int                 { return 2 }
func (silence) Len() int64                    { return 48000 * 600 }
func (silence) SetPosition(int64) error       { return nil }
func (silence) Read(b []float32) (int, error) { clear(b); return len(b), nil }

// posted collects what the player posts to the main thread, for the test
// to run there.
func posted() (chan func(), *Player) {
	ch := make(chan func(), 16)
	p := New(func(fn func()) { ch <- fn })
	p.Cue([]netease.Track{{ID: 1, Name: "一", Duration: 600000}})
	out := audio.NewPlayer(silence{})
	out.SetVolume(0)
	out.Play()
	p.Attach(out)
	p.fade.attach(out, 1)
	return ch, p
}

func next(t *testing.T, ch chan func()) {
	t.Helper()
	select {
	case fn := <-ch:
		fn()
	case <-time.After(2 * time.Second):
		t.Fatal("nothing posted")
	}
}

// Pausing fades out before the music stops; playing again fades it in.
func TestPauseFades(t *testing.T) {
	ch, p := posted()
	defer p.out.Close()
	p.TogglePlay()
	if p.Playing() || !p.out.Playing() {
		t.Fatal("paused at once, or not at all")
	}
	next(t, ch)
	if p.out.Playing() {
		t.Fatal("still playing after fading out")
	}
	if !p.fade.silent() {
		t.Error("faded out to something")
	}
	p.TogglePlay()
	if !p.Playing() || !p.out.Playing() {
		t.Fatal("did not play again")
	}
	time.Sleep(fadeIn + 50*time.Millisecond)
	p.fade.mu.Lock()
	level := p.fade.level(time.Now())
	p.fade.mu.Unlock()
	if level != 1 {
		t.Errorf("faded in to %v", level)
	}
}

// A seek playing pauses, which fades out over a few milliseconds, and
// plays again once there, its position the one asked for meanwhile.
func TestSeekWithoutClick(t *testing.T) {
	ch, p := posted()
	defer p.out.Close()
	p.Seek(90 * time.Second)
	if p.out.Playing() {
		t.Error("jumped while playing")
	}
	if p.Position() != 90*time.Second {
		t.Errorf("position %v while seeking", p.Position())
	}
	next(t, ch)
	if !p.out.Playing() || p.out.Position() < 90*time.Second {
		t.Errorf("after the seek: playing %v at %v", p.out.Playing(), p.out.Position())
	}
}

// Seeks one after the other, as the progress bar asks when let go, play
// on from the last.
func TestSeekTwicePlaysOn(t *testing.T) {
	ch, p := posted()
	defer p.out.Close()
	p.Seek(90 * time.Second)
	p.Seek(95 * time.Second)
	next(t, ch)
	for len(ch) > 0 {
		next(t, ch)
	}
	time.Sleep(seekGap * 2)
	for len(ch) > 0 {
		next(t, ch)
	}
	if !p.out.Playing() || p.out.Position() < 95*time.Second {
		t.Errorf("after two seeks: playing %v at %v", p.out.Playing(), p.out.Position())
	}
}

// A radio asks for more as its queue nears its end, and adds what is not
// in it already.
func TestRadioRefills(t *testing.T) {
	p := &Player{at: -1}
	p.Cue([]netease.Track{{ID: 1}, {ID: 2}})
	asked := 0
	p.AttachRadio("fm:DEFAULT", func(got func([]netease.Track)) {
		asked++
		got([]netease.Track{{ID: 2}, {ID: 3}})
	})
	if asked != 1 || len(p.Queue()) != 3 || p.Queue()[2].ID != 3 {
		t.Fatalf("asked %d, queue %v", asked, p.Queue())
	}
	if p.Radio() != "fm:DEFAULT" {
		t.Errorf("radio %q", p.Radio())
	}
	// Playing something else ends the radio.
	p.endRadio()
	if p.Radio() != "" || p.feed != nil {
		t.Error("radio still on")
	}
}

// The cache gives back what it kept, at the level asked for or a better
// one, and drops what was played longest ago once over its limit.
func TestCache(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(dir, 400)
	store := func(id int64, st *netease.Stream, n int) {
		f := c.part(st)
		f.Write(make([]byte, n))
		f.Close()
		if path, ok := c.keep(id, st, f.Name(), int64(n)); ok {
			c.settled(path)
		}
	}
	store(1, &netease.Stream{Level: "lossless", Type: "flac", Gain: -3}, 100)
	dl, st, ok := c.lookup(1, "exhigh")
	if !ok || dl.size != 100 || st.Type != "flac" || st.Gain != -3 {
		t.Fatalf("lookup: %v %+v", ok, st)
	}
	dl.Close()
	if _, _, ok := c.lookup(1, "hires"); ok {
		t.Error("found a lossless song for Hi-Res")
	}
	old := time.Now().Add(-time.Hour)
	os.Chtimes(filepath.Join(dir, "1-lossless.flac"), old, old)
	store(2, &netease.Stream{Level: "exhigh", Type: "mp3"}, 100)
	store(3, &netease.Stream{Level: "exhigh", Type: "mp3"}, 100)
	store(4, &netease.Stream{Level: "exhigh", Type: "mp3"}, 100)
	store(5, &netease.Stream{Level: "exhigh", Type: "mp3"}, 100)
	// Prune runs aside, so the oldest song is gone once it finishes.
	deadline := time.Now().Add(2 * time.Second)
	oldSong := filepath.Join(dir, "1-lossless.flac")
	for {
		_, err := os.Stat(oldSong)
		if os.IsNotExist(err) && c.Size() <= 400 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("prune did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, _, ok := c.lookup(1, "standard"); ok {
		t.Error("kept the song played longest ago")
	}
	if dl, _, ok := c.lookup(5, "standard"); !ok {
		t.Error("dropped the song just kept")
	} else {
		dl.Close()
	}
	if n := c.Size(); n > 400 {
		t.Errorf("takes %d of 400", n)
	}
	c.Clear()
	if n := c.Size(); n != 0 {
		t.Errorf("cleared to %d", n)
	}
}

// Keeping a song returns before the cache directory is cleaned. The download
// lock is held across keep, and the decoder waits on it.
func TestKeepReturnsBeforePrune(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(dir, 1000)
	old := filepath.Join(dir, "1-exhigh.mp3")
	if err := os.WriteFile(old, make([]byte, 900), 0o600); err != nil {
		t.Fatal(err)
	}
	part := filepath.Join(dir, "new.part")
	if err := os.WriteFile(part, make([]byte, 200), 0o600); err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	entered := make(chan struct{})
	setPruneHook(func() {
		select {
		case <-entered:
		default:
			close(entered)
		}
		<-release
	})
	t.Cleanup(func() {
		setPruneHook(nil)
		select {
		case <-release:
		default:
			close(release)
		}
	})
	done := make(chan struct{})
	go func() {
		c.keep(2, &netease.Stream{Level: "exhigh", Type: "mp3"}, part, 200)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("keep waited for the cache to be pruned")
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("prune did not start")
	}
	if _, err := os.Stat(old); err != nil {
		t.Fatal("prune deleted before keep returned")
	}
	close(release)
	deadline := time.Now().Add(2 * time.Second)
	for {
		_, err := os.Stat(old)
		if os.IsNotExist(err) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the older song was not dropped")
		}
		time.Sleep(5 * time.Millisecond)
	}
	kept := filepath.Join(dir, "2-exhigh.mp3")
	if _, err := os.Stat(kept); err != nil {
		t.Fatal("the song just kept was dropped")
	}
	c.settled(kept)
}

// Two songs kept while a clean is waiting must both survive it. The first
// clean used to protect only its own path and delete the other.
func TestPruneKeepsSongsKeptTogether(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(dir, 1000)
	old := filepath.Join(dir, "1-exhigh.mp3")
	if err := os.WriteFile(old, make([]byte, 900), 0o600); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	entered := make(chan struct{})
	setPruneHook(func() {
		select {
		case <-entered:
		default:
			close(entered)
		}
		<-release
	})
	t.Cleanup(func() {
		setPruneHook(nil)
		select {
		case <-release:
		default:
			close(release)
		}
	})
	keep := func(id int64) {
		t.Helper()
		part := filepath.Join(dir, "new"+string(rune('0'+id))+".part")
		if err := os.WriteFile(part, make([]byte, 200), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, ok := c.keep(id, &netease.Stream{Level: "exhigh", Type: "mp3"}, part, 200); !ok {
			t.Fatalf("song %d was not kept", id)
		}
	}
	keep(2)
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("prune did not start")
	}
	keep(3)
	older := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "3-exhigh.mp3"), older, older); err != nil {
		t.Fatal(err)
	}
	close(release)
	deadline := time.Now().Add(2 * time.Second)
	for {
		_, err := os.Stat(old)
		if os.IsNotExist(err) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the older song was not dropped")
		}
		time.Sleep(5 * time.Millisecond)
	}
	for _, name := range []string{"2-exhigh.mp3", "3-exhigh.mp3"} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s was dropped", name)
		}
		c.settled(path)
	}
}

// Changing the limit waits until songs over it are gone.
func TestSetLimitDropsBeforeReturn(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(dir, 1000)
	part := filepath.Join(dir, "new.part")
	if err := os.WriteFile(part, make([]byte, 100), 0o600); err != nil {
		t.Fatal(err)
	}
	path, ok := c.keep(1, &netease.Stream{Level: "exhigh", Type: "mp3"}, part, 100)
	if !ok {
		t.Fatal("not kept")
	}
	c.settled(path)
	c.SetLimit(50)
	if _, err := os.Stat(filepath.Join(dir, "1-exhigh.mp3")); !os.IsNotExist(err) {
		t.Fatal("SetLimit returned before dropping the song")
	}
}

// A song opened while a scan is already underway must be pruned by the next
// pass. Forgetting it left the cache over its limit with no cleaner running.
func TestPruneSeesFileSettledDuringScan(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(dir, 1000)
	old := filepath.Join(dir, "1-exhigh.mp3")
	if err := os.WriteFile(old, make([]byte, 900), 0o600); err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	entered := make(chan struct{})
	setPruneHook(func() {
		select {
		case <-entered:
		default:
			close(entered)
		}
		<-release
	})
	t.Cleanup(func() {
		setPruneHook(nil)
		select {
		case <-release:
		default:
			close(release)
		}
	})
	done := make(chan struct{})
	go func() {
		c.SetLimit(1000)
		close(done)
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("prune did not start")
	}
	part := filepath.Join(dir, "new.part")
	if err := os.WriteFile(part, make([]byte, 200), 0o600); err != nil {
		t.Fatal(err)
	}
	path, ok := c.keep(2, &netease.Stream{Level: "exhigh", Type: "mp3"}, part, 200)
	if !ok {
		t.Fatal("not kept")
	}
	c.settled(path)
	close(release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SetLimit did not return")
	}
	if n := c.Size(); n > 1000 {
		t.Fatalf("cache kept %d of 1000", n)
	}
}

// While a just-kept song is still unopened, the cleaner waits instead of
// scanning the directory again and again.
func TestPruneWaitsWhileSongIsUnopened(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(dir, 1000)
	var passes atomic.Int32
	setPruneHook(func() { passes.Add(1) })
	t.Cleanup(func() { setPruneHook(nil) })
	part := filepath.Join(dir, "new.part")
	if err := os.WriteFile(part, make([]byte, 200), 0o600); err != nil {
		t.Fatal(err)
	}
	path, ok := c.keep(1, &netease.Stream{Level: "exhigh", Type: "mp3"}, part, 200)
	if !ok {
		t.Fatal("not kept")
	}
	time.Sleep(50 * time.Millisecond)
	if n := passes.Load(); n > 3 {
		t.Fatalf("scanned %d times while the song was unopened", n)
	}
	c.settled(path)
	c.SetLimit(50)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("SetLimit returned before dropping the song")
	}
}

// Each overlapping hand-off owns its protection: opening one download or
// failing another rename must not expose the still-unopened song to pruning.
func TestPruneProtectsOverlappingHandOffs(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "opened", true: "rename-failed"}[failed], func(t *testing.T) {
			dir := t.TempDir()
			c := NewCache(dir, 1000)
			st := &netease.Stream{Level: "exhigh", Type: "mp3"}
			part := filepath.Join(dir, "first.part")
			if err := os.WriteFile(part, make([]byte, 100), 0o600); err != nil {
				t.Fatal(err)
			}
			path, ok := c.keep(1, st, part, 100)
			if !ok {
				t.Fatal("first download was not kept")
			}
			part = filepath.Join(dir, "second.part")
			if !failed {
				if err := os.WriteFile(part, make([]byte, 100), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			_, ok = c.keep(1, st, part, 100)
			if ok == failed {
				t.Fatalf("second download kept = %v", ok)
			}
			if ok {
				c.settled(path)
			}
			c.SetLimit(0)
			f, err := os.Open(path)
			if err != nil {
				t.Fatalf("unopened download lost its protection: %v", err)
			}
			f.Close()
			c.settled(path)
			c.SetLimit(0)
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("settled song was not pruned: %v", err)
			}
		})
	}
}

// Leaving a radio keeps the song playing and those before it, drops those
// it queued after, and asks it for no more.
func TestEndRadio(t *testing.T) {
	p := &Player{at: -1}
	p.Cue([]netease.Track{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}})
	p.at, p.track = 1, &p.queue[1]
	asked := 0
	p.AttachRadio("heart", func(got func([]netease.Track)) { asked++ })
	p.EndRadio()
	if p.Radio() != "" || p.Current().ID != 2 {
		t.Fatalf("radio %q, current %v", p.Radio(), p.Current())
	}
	if q := p.Queue(); len(q) != 2 || q[0].ID != 1 || q[1].ID != 2 {
		t.Fatalf("queue %v", q)
	}
	p.refill()
	if asked != 0 {
		t.Error("asked the radio after leaving it")
	}
}
