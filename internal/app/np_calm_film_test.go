package app

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/lyrics"
	"netease-music-mygo/internal/netease"
)

// calmFrames runs Now Playing w×h through calming and waking, a frame each
// 16 ms, and returns where the cover and the title were in each.
func calmFrames(t *testing.T, w, h int, film string) (covers, titles []ui.Rect) {
	t.Helper()
	a := newTestApp()
	a.settings = defaultSettings()
	a.pl.Cue([]netease.Track{{ID: 1, Name: "客客气气 COURTESY", Duration: 200000, Artists: []netease.Artist{{Name: "鹤 The Crane"}}}})
	var lines []lyrics.Line
	for i := 0; i < 20; i++ {
		lines = append(lines, lyrics.Line{At: time.Duration(i) * 4 * time.Second, Text: fmt.Sprintf("第 %d 句歌词在这里", i)})
	}
	a.pl.SetLyrics(lines)
	a.nowPlaying = true
	pointer := mygo.Point{X: 10, Y: 10}
	old := npCursor
	npCursor = func() mygo.Point { return pointer }
	defer func() { npCursor = old }()
	tt := ui.NewTester(a.view, w, h)
	tt.Move(float32(w)/2, float32(h)/2)
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	var shots []*image.RGBA
	step := func(n int) {
		for i := 0; i < n; i++ {
			tt.Advance(32 * time.Millisecond)
			tt.Frame()
			r, _ := tt.Find("专辑封面")
			ti, _ := tt.Find("客客气气 COURTESY")
			covers, titles = append(covers, r), append(titles, ti)
			if film != "" && i%3 == 0 {
				img := tt.Image()
				cp := image.NewRGBA(img.Bounds())
				draw.Draw(cp, cp.Bounds(), img, image.Point{}, draw.Src)
				shots = append(shots, cp)
			}
		}
	}
	// Still: it calms after npCalmStill.
	step(int(npCalmStill/(32*time.Millisecond)) + 50)
	// Moved: it wakes.
	pointer = mygo.Point{X: 20, Y: 20}
	tt.Move(float32(w)/2+5, float32(h)/2)
	step(30)
	if film != "" {
		writeFilm(t, film, shots)
	}
	return
}

// writeFilm writes the frames side by side, four to a row, at half size.
func writeFilm(t *testing.T, path string, shots []*image.RGBA) {
	if len(shots) == 0 {
		return
	}
	b := shots[0].Bounds()
	cw, ch := b.Dx()/2, b.Dy()/2
	cols := 4
	rows := (len(shots) + cols - 1) / cols
	out := image.NewRGBA(image.Rect(0, 0, cw*cols, ch*rows))
	for i, s := range shots {
		x0, y0 := (i%cols)*cw, (i/cols)*ch
		for y := 0; y < ch; y++ {
			for x := 0; x < cw; x++ {
				out.Set(x0+x, y0+y, s.At(x*2, y*2))
			}
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	png.Encode(f, out)
}

// Calming and waking move the cover and the title a little each frame:
// nothing jumps, at any size of window.
func TestNowPlayingCalmsSmoothly(t *testing.T) {
	dir := os.Getenv("CALM_FILM")
	for _, size := range calmSizes() {
		film := ""
		if dir != "" {
			film = filepath.Join(dir, fmt.Sprintf("calm-%dx%d.png", size[0], size[1]))
		}
		covers, titles := calmFrames(t, size[0], size[1], film)
		worst, at, what := float32(0), 0, ""
		// How far each went in all, calming: a step is a jump if it is a
		// large part of that.
		travel := func(rs []ui.Rect) float32 {
			var sum float32
			for i := 1; i < len(rs); i++ {
				sum += max(abs32(rs[i].Y-rs[i-1].Y), abs32(rs[i].W-rs[i-1].W))
			}
			return sum / 2 // there and back
		}
		limit := max(12, max(travel(covers), travel(titles))/5)
		for i := 1; i < len(covers); i++ {
			for _, d := range []struct {
				name string
				a, b ui.Rect
			}{{"cover", covers[i-1], covers[i]}, {"title", titles[i-1], titles[i]}} {
				if d.name == "title" && covers[0].W < npCardCover {
					// From the card, the title fades out beside the cover
					// and in under it, moving while it is not seen.
					continue
				}
				j := max(abs32(d.b.Y-d.a.Y), abs32(d.b.W-d.a.W), abs32(d.b.H-d.a.H))
				if d.name == "cover" {
					// The title's x is its marquee's too, which scrolls.
					j = max(j, abs32(d.b.X-d.a.X))
				}
				if j > worst {
					worst, at, what = j, i, d.name
				}
			}
		}
		if os.Getenv("CALM_DUMP") != "" {
			for i := range covers {
				if i > 80 && i < 100 || i > 140 && i < 152 {
					t.Logf("%3d cover %v title %v", i, covers[i], titles[i])
				}
			}
		}
		first, last := covers[0], covers[len(covers)-31]
		t.Logf("%dx%d: cover %.0f→%.0f at (%.0f,%.0f)→(%.0f,%.0f); worst step %.1f px (%s, frame %d)",
			size[0], size[1], first.W, last.W, first.X, first.Y, last.X, last.Y, worst, what, at)
		t.Logf("  limit %.1f px", limit)
		if worst > limit {
			t.Errorf("%dx%d: %s jumped %.1f px in one frame (frame %d)", size[0], size[1], what, worst, at)
		}
		if math.IsNaN(float64(worst)) {
			t.Fatal("NaN")
		}
	}
}

func calmSizes() [][2]int {
	if os.Getenv("CALM_ONE") != "" {
		return [][2]int{{870, 575}}
	}
	return [][2]int{{1200, 800}, {870, 575}, {900, 470}, {760, 420}}
}

// Resizing the window moves the line sung only as far as the window
// changes: it does not jump.
func TestNowPlayingLyricsHoldWhileResized(t *testing.T) {
	if os.Getenv("CALM_RESIZE") == "" {
		t.Skip("slow; set CALM_RESIZE")
	}
	a := newTestApp()
	a.settings = defaultSettings()
	a.pl.Cue([]netease.Track{{ID: 1, Name: "客客气气 COURTESY", Duration: 200000, Artists: []netease.Artist{{Name: "鹤 The Crane"}}}})
	var lines []lyrics.Line
	for i := 0; i < 20; i++ {
		lines = append(lines, lyrics.Line{At: time.Duration(i) * 4 * time.Second, Text: fmt.Sprintf("第 %d 句歌词在这里", i)})
	}
	a.pl.SetLyrics(lines)
	a.nowPlaying = true
	old := npCursor
	npCursor = func() mygo.Point { return mygo.Point{} }
	defer func() { npCursor = old }()
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Move(600, 400)
	tt.Frame()
	tt.Advance(2 * time.Second)
	tt.Frame()
	var prev ui.Rect
	w, h := 1200, 800
	for i := 0; i < 90; i++ {
		w, h = w-5, h-4
		tt.SetSize(w, h)
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
		r, _ := tt.Find("第 0 句歌词在这里")
		cv, _ := tt.Find("专辑封面")
		if i > 0 && (abs32(r.Y-prev.Y) > 12 || abs32(r.X-prev.X) > 12) {
			t.Errorf("%dx%d: line 0 jumped %v → %v (cover %v)", w, h, prev, r, cv)
		}
		prev = r
	}
}

// Calm, the pointer is hidden over a line of lyrics too, which shows a
// hand, and the first click on one wakes the view rather than seeking.
func TestNowPlayingCalmHidesPointerEverywhere(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.pl.Cue([]netease.Track{{ID: 1, Name: "一", Duration: 200000}})
	var lines []lyrics.Line
	for i := 0; i < 12; i++ {
		lines = append(lines, lyrics.Line{At: time.Duration(i) * 4 * time.Second, Text: fmt.Sprintf("第 %d 句", i)})
	}
	a.pl.SetLyrics(lines)
	a.nowPlaying = true
	old := npCursor
	npCursor = func() mygo.Point { return mygo.Point{} }
	defer func() { npCursor = old }()
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	r, ok := tt.Find("第 2 句")
	if !ok {
		t.Fatal("no line")
	}
	x, y := r.X+r.W/2, r.Y+r.H/2
	tt.Move(x, y)
	tt.Frame()
	if tt.Cursor() == ui.CursorNone {
		t.Fatal("pointer hidden while awake")
	}
	for i := 0; i < 40; i++ {
		tt.Advance(100 * time.Millisecond)
		tt.Frame()
	}
	if !a.more.npCalm.calm {
		t.Fatal("not calm after 4 s still")
	}
	if c := tt.Cursor(); c != ui.CursorNone {
		t.Fatalf("pointer over a line while calm is %v, want none", c)
	}
	tt.ClickAt(x, y)
	tt.Frame()
	if a.more.npCalm.calm {
		t.Error("a click did not wake the view")
	}
	if pos := a.pl.Position(); pos > time.Second {
		t.Errorf("the click went through to the line and seeked to %v", pos)
	}
}
