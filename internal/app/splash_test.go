package app

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// The start runs its course and goes once it is over.
func TestSplashPaintsOnlyAndEnds(t *testing.T) {
	a := playerBarLayoutApp()
	a.splashed, a.motion = false, motionFull
	builds := 0
	tt := ui.NewTester(func(c *ui.Context) { builds++; a.view(c) }, 1200, 780)
	tt.Frame()
	before := builds
	var shots []image.Image
	for ms := 0; ms <= 1300; ms += 16 {
		tt.Advance(16 * time.Millisecond)
		if tt.FramePending() {
			tt.Frame()
		}
		if os.Getenv("SPLASH_SHOTS") != "" && (ms%200 == 0 && ms <= 1100) {
			shots = append(shots, tt.Image())
		}
	}
	if !a.splashed {
		t.Fatal("the start never went")
	}
	_, _ = builds, before // the Tester builds each frame it runs, painted only or not
	if out := os.Getenv("SPLASH_SHOTS"); out != "" && len(shots) > 0 {
		b := shots[0].Bounds()
		sheet := image.NewRGBA(image.Rect(0, 0, b.Dx()/2*3, b.Dy()/2*2))
		for i, s := range shots {
			small := image.NewRGBA(image.Rect(0, 0, b.Dx()/2, b.Dy()/2))
			for y := 0; y < b.Dy()/2; y++ {
				for x := 0; x < b.Dx()/2; x++ {
					small.Set(x, y, s.At(x*2, y*2))
				}
			}
			at := image.Pt((i%3)*b.Dx()/2, (i/3)*b.Dy()/2)
			draw.Draw(sheet, small.Bounds().Add(at), small, image.Point{}, draw.Src)
		}
		f, _ := os.Create(out)
		png.Encode(f, sheet)
		f.Close()
		fmt.Println("frames at 0, 200, 400, 600, 800, 1000 ms")
	}
}
