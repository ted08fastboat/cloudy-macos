package app

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/lyrics"
)

// deskTestLine is a line of mixed scripts sung word by word, half sung at
// deskTestPos.
func deskTestLine(words bool) deskMain {
	parts := []string{"我们在", "晚风里", "说再见 ", "Goodbye ", "かな"}
	m := deskMain{size: 32}
	at := time.Duration(0)
	for _, p := range parts {
		m.text += p
		if words {
			m.words = append(m.words, lyrics.Word{Text: p, Start: at, End: at + 400*time.Millisecond})
		}
		at += 400 * time.Millisecond
	}
	return m
}

// Down, the characters are the line as the lyrics' window shows it, to the
// pixel, in every style, sung word by word or not: that window takes over
// from them with nothing to see.
func TestDeskGlyphsLandOnTheLine(t *testing.T) {
	for _, style := range deskStyles {
		for _, words := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-words-%v", style.id, words), func(t *testing.T) {
				a := newTestApp()
				a.settings = defaultSettings()
				a.settings.DeskStyle = style.id
				a.settings.DeskWords = true
				m := deskTestLine(words)
				look := a.deskLookOf(style.id)
				pos := 900 * time.Millisecond
				own := ui.NewTester(func(c *ui.Context) {
					// On nothing, as the lyrics' window is.
					a.floatTheme(c)
					ui.Box(c).Absolute().Left(40.4).Top(30.6).Children(func() {
						a.deskLine(c, "main", m.text, m.words, pos, m.size, look, true)
					})
				}, 760, 120)
				own.Frame()
				flown := ui.NewTester(func(c *ui.Context) {
					a.floatTheme(c)
					ui.Box(c).Fill().Draw(func(p *ui.Painter, _ ui.Rect) {
						a.paintDeskGlyphs(p, m, look, pos, 40.4, 30.6, func(int, ui.Rect) (float32, float32) { return 1, 0 }, 0, 0)
					})
				}, 760, 120)
				flown.Frame()
				if n := pixelsApart(own.Image(), flown.Image(), 2); n > 0 {
					t.Errorf("%d pixels differ between the line and the characters down", n)
				}
			})
		}
	}
}

// pixelsApart counts the pixels of a and b more than by apart in a channel.
func pixelsApart(a, b *image.RGBA, by int) int {
	n := 0
	for i := 0; i+3 < len(a.Pix) && i+3 < len(b.Pix); i += 4 {
		for j := range 4 {
			d := int(a.Pix[i+j]) - int(b.Pix[i+j])
			if d > by || d < -by {
				n++
				break
			}
		}
	}
	return n
}

// The characters set off one after another, the last no later than
// deskGlyphLags after the first, however long the line.
func TestDeskFlyLags(t *testing.T) {
	for _, n := range []int{1, 2, 8, 40} {
		lag, total := deskFlyLags(n)
		if lag > deskGlyphLag || total > deskGlyphLags+deskGlyphFly || total < deskGlyphFly {
			t.Errorf("%d characters: %v apart, %v in all", n, lag, total)
		}
	}
}

// With NM_FRAMES set, saves the characters on their way out of the button,
// and back, a frame every 40 ms, in each style.
func TestDeskFlyFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	for _, style := range []string{"glass", "outline"} {
		for _, opening := range []bool{true, false} {
			a := newTestApp()
			a.settings = defaultSettings()
			a.settings.DeskStyle = style
			m := deskTestLine(true)
			f := &deskFlight{opening: opening, line: m, from: ui.Rect{X: 900, Y: 330, W: 32, H: 32},
				desk: ui.Point{X: 40, Y: 10}, at: ui.Rect{X: 120, Y: 50, W: 600, H: 45}}
			_, total := deskFlyLags(len(deskGlyphs(a.deskShape(m, a.deskLookOf(style), 0, false), m)))
			for i, gone := 0, time.Duration(0); gone <= total+40*time.Millisecond; i, gone = i+1, gone+40*time.Millisecond {
				tt := ui.NewTester(func(c *ui.Context) {
					a.pal = paletteFor(c.Theme())
					a.floatTheme(c)
					ui.Box(c).Fill().Background(ui.Hex("#5b6b7a")).Draw(func(p *ui.Painter, _ ui.Rect) {
						a.paintDeskFly(p, f, min(gone, total))
					})
				}, 980, 380)
				tt.Frame()
				file, err := os.Create(filepath.Join(dir, fmt.Sprintf("deskfly-%s-%v-%02d.png", style, opening, i)))
				if err != nil {
					t.Fatal(err)
				}
				if err := png.Encode(file, tt.Image()); err != nil {
					t.Fatal(err)
				}
				file.Close()
			}
		}
	}
}
