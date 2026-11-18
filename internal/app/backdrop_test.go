package app

import (
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/player"
)

// The drift eases in as the music plays and glides to a stop as it
// pauses; the bass swells and settles.
func TestBackdropDrifts(t *testing.T) {
	var bd backdropCache
	now := time.Unix(0, 0)
	var quiet [player.Bands]float32
	for range 120 {
		now = now.Add(backdropFrame)
		bd.step(now, true, quiet, true)
	}
	if bd.speed != 1 || bd.drift <= 0 {
		t.Fatalf("playing: speed %v, drift %v", bd.speed, bd.drift)
	}
	var loud [player.Bands]float32
	for i := range loud {
		loud[i] = 1
	}
	now = now.Add(backdropFrame)
	bd.step(now, true, loud, true)
	if bd.level.group(0, 6) <= 0.2 {
		t.Errorf("bass %v on a beat", bd.level.group(0, 6))
	}
	moving := true
	for range 120 {
		now = now.Add(backdropFrame)
		moving = bd.step(now, false, quiet, false)
	}
	if moving || bd.speed != 0 {
		t.Errorf("paused: still moving, speed %v", bd.speed)
	}
	at := bd.drift
	now = now.Add(time.Second)
	bd.step(now, false, quiet, false)
	if bd.drift != at {
		t.Error("drifted while paused")
	}
}

// With NM_FRAMES set, saves the fog of a few covers as it drifts,
// and on a beat, one row a cover.
func TestBackdropDriftFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	const w, h = 300, 420
	var rows []*image.RGBA
	for _, q := range []string{"春日影 MyGO", "晴天 周杰伦", "夜に駆ける"} {
		hits, err := netease.Search(q, 1)
		if err != nil || len(hits) == 0 {
			t.Fatal(q, err)
		}
		img, err := fetchImage(netease.Sized(hits[0].Album.PicURL, 600))
		if err != nil {
			t.Fatal(err)
		}
		art := &artwork{tint: dominantColor(img)}
		shots := []struct{ drift, bass float32 }{{0, 0}, {2, 0}, {4, 0}, {4, 0.9}}
		row := image.NewRGBA(image.Rect(0, 0, w*len(shots), h))
		for i, s := range shots {
			tt := ui.NewTester(func(c *ui.Context) {
				ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) { drawFog(p, r, art.tint, s.drift, s.bass) })
			}, w, h)
			tt.Frame()
			shot := tt.Image()
			draw.Draw(row, image.Rect(i*w, 0, (i+1)*w, h), shot, shot.Bounds().Min, draw.Src)
		}
		rows = append(rows, row)
	}
	out := image.NewRGBA(image.Rect(0, 0, rows[0].Bounds().Dx(), h*len(rows)))
	for i, r := range rows {
		draw.Draw(out, image.Rect(0, i*h, r.Bounds().Dx(), (i+1)*h), r, image.Point{}, draw.Src)
	}
	f, _ := os.Create(filepath.Join(dir, "drift.png"))
	defer f.Close()
	png.Encode(f, out)
}

// Every scene of the living backdrop compiles, and paints something of
// the cover's color that is dark enough for white to read over.
func TestBackdropScenesPaint(t *testing.T) {
	for _, sc := range backdropScenes {
		tt := ui.NewTester(func(c *ui.Context) {
			ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
				drawScene(p, r, sc.id, ui.RGB(40, 90, 220), 3, 0.5)
				drawSceneOver(p, r, sc.id, ui.RGB(40, 90, 220), 3, 0.5, 1, nil, nil)
			})
		}, 160, 120)
		tt.Frame()
		img := tt.Image()
		var sum [3]float64
		b := img.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				r, g, bl, _ := img.At(x, y).RGBA()
				sum[0], sum[1], sum[2] = sum[0]+float64(r), sum[1]+float64(g), sum[2]+float64(bl)
			}
		}
		n := float64(b.Dx()*b.Dy()) * 65535
		r, g, bl := sum[0]/n, sum[1]/n, sum[2]/n
		if bl <= r || bl <= g {
			t.Errorf("%s: mean %.2f %.2f %.2f of a blue cover is not blue", sc.label, r, g, bl)
		}
		// A scene in the sun is lighter than the ones at night, as light
		// as white still reads over.
		most := 0.45
		if sc.own {
			most = 0.56
		}
		if lum := 0.2126*r + 0.7152*g + 0.0722*bl; lum < 0.02 || lum > most {
			t.Errorf("%s: mean luminance %.2f", sc.label, lum)
		}
	}
	if backdropSceneOf("gone") != &backdropScenes[0] {
		t.Error("an unknown scene is not the fog")
	}
}

// With NM_FRAMES set, saves each scene for covers of a few colors as it
// drifts, and on a beat, one row a cover.
func TestBackdropSceneFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	const w, h = 720, 450
	tints := []ui.Color{ui.RGB(220, 60, 70), ui.RGB(40, 90, 220), ui.RGB(60, 190, 120), ui.RGB(120, 120, 120)}
	shots := []struct{ drift, bass float32 }{{0, 0}, {6.4, 0}, {6.4, 0.9}}
	for _, sc := range backdropScenes {
		if only := os.Getenv("NM_SCENE"); only != "" && sc.id != only {
			continue
		}
		out := image.NewRGBA(image.Rect(0, 0, w*len(shots), h*len(tints)))
		for j, tint := range tints {
			for i, s := range shots {
				tt := ui.NewTester(func(c *ui.Context) {
					ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
						drawScene(p, r, sc.id, tint, s.drift, s.bass)
						drawSceneOver(p, r, sc.id, tint, s.drift, s.bass, 1, nil, nil)
					})
				}, w, h)
				tt.Frame()
				shot := tt.Image()
				draw.Draw(out, image.Rect(i*w, j*h, (i+1)*w, (j+1)*h), shot, shot.Bounds().Min, draw.Src)
			}
		}
		name := sc.id
		if name == "" {
			name = "fog"
		}
		f, err := os.Create(filepath.Join(dir, "scene-"+name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		png.Encode(f, out)
		f.Close()
	}
}

// A beat is the low end leaping over what it has been: it lets a drop
// fall, and no second one at once; still water lets one fall of itself.
func TestBeatsLetDropsFall(t *testing.T) {
	var b beatState
	drift := 0.0
	step := func(low float32, n int) {
		for range n {
			drift += 1.0 / 30
			b.step(low, 1.0/30, drift, true)
		}
	}
	fallen := func() int {
		n := 0
		for _, d := range b.drops {
			if d.hard > 0 {
				n++
			}
		}
		return n
	}
	step(0.2, 30)
	if fallen() != 0 {
		t.Fatalf("%d drops in a steady second", fallen())
	}
	step(0.7, 3)
	if fallen() != 1 {
		t.Fatalf("%d drops on a beat", fallen())
	}
	if d := b.drops[0]; d.hard < 0.6 || d.y < 0 || d.y > 1 || d.x < 0.06 || d.x > 0.94 {
		t.Errorf("drop %+v", d)
	}
	step(0.2, 20)
	step(0.75, 3)
	if fallen() != 2 {
		t.Fatalf("%d drops after a second beat", fallen())
	}
	step(0.75, 90)
	if fallen() != 3 {
		t.Fatalf("%d drops: none fell of itself in still water", fallen())
	}
	if got := len(b.uniform()); got != 4*sceneDrops {
		t.Errorf("uniform of %d", got)
	}
}

// The leaves of the pool go every way, in and out by every edge, and
// there are always from two to six of them in the picture.
func TestPoolLeavesComeAndGo(t *testing.T) {
	var ls leafState
	var drops [sceneDrops]sceneDrop
	const aspect = 1.6
	edges := map[string]bool{}
	for step := range 30 * 60 * 20 {
		drift := float64(step) / 30
		if step%45 == 0 {
			drops[(step/45)%sceneDrops] = sceneDrop{x: float32(step%7) / 7, y: float32(step%5) / 5, at: float32(drift), hard: 0.9}
		}
		ls.step(1.0/30, aspect, drift, &drops)
		seen := 0
		for _, l := range ls.leaves {
			if !l.alive {
				continue
			}
			if l.x > -l.half && l.x < aspect+l.half && l.y > -l.half && l.y < 1+l.half {
				seen++
			}
			switch {
			case l.x < 0:
				edges["left"] = true
			case l.x > aspect:
				edges["right"] = true
			case l.y < 0:
				edges["top"] = true
			case l.y > 1:
				edges["bottom"] = true
			}
		}
		if seen < 2 || seen > poolLeaves {
			t.Fatalf("%d leaves in the picture at %.1f s", seen, drift)
		}
	}
	if len(edges) != 4 {
		t.Errorf("leaves crossed only %v", edges)
	}
	if got := len(ls.uniform(aspect)); got != 4*poolLeaves {
		t.Errorf("uniform of %d", got)
	}
}
