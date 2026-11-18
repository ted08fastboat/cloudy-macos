package app

import (
	"image/color"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// The mini window seeks where its bar is dragged, and sets the volume where its
// slider is clicked: neither goes to the window, which drags from elsewhere.
func TestMiniSeeksAndSetsVolume(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.volume = 0.8
	a.pl.Cue([]netease.Track{{ID: 7, Name: "宝船", Duration: 200000}})
	tt := ui.NewTester(a.miniView, miniW, miniH)
	tt.Frame()

	const y = miniH - 12
	left, width := float32(18), float32(miniW-36)
	tt.Press(left+width/4, y)
	tt.Frame()
	tt.Move(left+width*3/4, y)
	tt.Frame()
	tt.Release(left+width*3/4, y)
	tt.Frame()
	if got := a.pl.Position(); got < 140*time.Second || got > 160*time.Second {
		t.Errorf("dragged to three quarters of 200s, at %v", got)
	}

	r, ok := tt.Find("音量")
	if !ok {
		t.Fatal("no volume slider")
	}
	tt.ClickAt(r.X+r.W/4, r.Y+r.H/2)
	tt.Frame()
	if a.volume < 0.15 || a.volume > 0.35 {
		t.Errorf("clicked a quarter along, volume %v", a.volume)
	}
}

// Hovering the mini window's cover dims it under the expand mark, and a
// click on it goes back to the main window, in the strip, standard and card
// layouts.
func TestMiniCoverHoverAndRestore(t *testing.T) {
	orig := restoreFromMini
	defer func() { restoreFromMini = orig }()

	for _, tc := range []struct {
		name string
		w, h int
	}{
		{"strip", 420, 80},
		{"standard", miniW, miniH},
		{"card", 480, 220},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newTestApp()
			a.settings = defaultSettings()
			a.art.add("mini-cover-art", &artwork{bmp: ui.NewBitmap(coverTestImage()), used: time.Now(), size: 1})
			a.pl.Cue([]netease.Track{{ID: 7, Name: "Flower of Life", Album: netease.Album{PicURL: "mini-cover-art"}, Duration: 200000}})

			restored := 0
			restoreFromMini = func(*app) { restored++ }

			tt := ui.NewTester(a.miniView, tc.w, tc.h)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Advance(time.Second)
			tt.Frame()

			cover := barElement(t, tt, "迷你窗口封面")
			img := tt.Image()
			scale := float32(img.Bounds().Dx()) / float32(tc.w)
			// Off the middle, where the expand mark is.
			sx := int((cover.X + cover.W*0.25) * scale)
			sy := int((cover.Y + cover.H*0.5) * scale)
			before := color.RGBAModel.Convert(img.At(sx, sy)).(color.RGBA)

			tt.Move(cover.X+cover.W/2, cover.Y+cover.H/2)
			tt.Frame()
			after := color.RGBAModel.Convert(tt.Image().At(sx, sy)).(color.RGBA)
			if after.R >= before.R || after.G >= before.G || after.B >= before.B {
				t.Fatalf("hovered cover not dimmed: before %+v, after %+v", before, after)
			}

			if err := tt.Click("迷你窗口封面"); err != nil {
				t.Fatal(err)
			}
			if restored != 1 {
				t.Fatalf("clicked the cover: went back %d times", restored)
			}
		})
	}
}

// The mini window's button sits left of the settings in the top bar, at
// any width (PR #11), and opens the mini window.
func TestTopBarMiniButton(t *testing.T) {
	orig := onShowMini
	defer func() { onShowMini = orig }()
	for _, width := range []int{880, 1440} {
		a := playerBarLayoutApp()
		opened := 0
		onShowMini = func(*app) { opened++ }
		tt := playerBarTester(a, width)
		// The top bar is built before the player bar, whose button this is not.
		mini, _ := tt.Find("迷你窗口")
		settings := barElement(t, tt, "设置")
		if mini.W == 0 || mini.Y > 60 || mini.X+mini.W > settings.X {
			t.Fatalf("%d: no mini button left of settings in the top bar: mini=%+v settings=%+v", width, mini, settings)
		}
		tt.ClickAt(mini.X+mini.W/2, mini.Y+mini.H/2)
		if opened != 1 {
			t.Fatalf("%d: clicking it opened the mini window %d times", width, opened)
		}
	}
}
