package app

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// transportWidth returns how wide transport's row of controls of size is:
// the mode, previous, play and next buttons, and the spacer after next
// that keeps play in the middle.
func transportWidth(size float32) float32 {
	mode := size*0.62 + 16
	step := size*0.85 + 16
	play := size*1.25 + 16
	return 2*mode + 2*step + play + 4*size*0.5
}

// volumeReach is how far past the controls, in their size, the volume
// reaches on either side.
const volumeReach = 0.5

// volumeShown is how long the volume's percent stays after the slider was
// last moved or the wheel last turned it.
const volumeShown = 1200 * time.Millisecond

// npVolume is Now Playing's volume under its controls of size: the speaker,
// which mutes, and a slim slider. Like the controls it is centered on
// play, as wide as their row and a little more on either side: the
// speaker starts a little before the mode's button, and the slider ends
// as far past the spacer on the right. The wheel over them turns the
// volume, finely, and while it moves its percent shows over the middle of
// the slider, as the times do by the progress.
func (a *app) npVolume(c *ui.Context, size float32, color, track, digits ui.Color) {
	const speaker, gap = 32, 4
	width := transportWidth(size) + 2*size*volumeReach
	slider := max(60, width-speaker-gap)
	now := c.Now()
	moved := func() { a.more.volumeAt = now }
	row := ui.Row(c.Key("np-volume")).Width(width).Gap(gap).AlignItems(ui.Center).AlignSelf(ui.Center)
	row.HandleInput(func(ev ui.InputEvent) bool {
		if a.scrollVolume(ev) {
			moved()
			return true
		}
		return false
	})
	row.Children(func() {
		a.muteButton(c, color)
		a.volumeBar(c, "volume", slider, color, track, digits, &a.more.volumeAt, ui.Color{})
	})
}

// volumeTouch is how far over and under the volume's slider it can be
// taken hold of.
const volumeTouch = 8

// volumeBar is a slim volume slider width wide. While it moves, or the wheel
// turns it (which sets at), its percent shows over its middle, on back if
// that is not clear, and fades after volumeShown.
func (a *app) volumeBar(c *ui.Context, key string, width float32, color, track, digits ui.Color, at *time.Time, back ui.Color) {
	now := c.Now()
	box := ui.Box(c.Key(key)).Width(width).Height(16)
	box.Children(func() {
		// Taller than it draws, 8 more over and under, for a finger to
		// find it on a touch screen; the hand says it can be dragged.
		s := ui.SliderBase(c.Key("volume"), &a.volume, 0, 1).Width(width).Height(16 + 2*volumeTouch).Absolute().Top(-volumeTouch).Left(0).
			FocusRing(false).Label("音量").Cursor(ui.CursorPointer)
		if s.Changed() {
			a.pl.SetVolume(a.volume)
			*at = now
		}
		pressed := s.Pressed()
		hot := s.Hovered() || pressed
		thick, knob := float32(4), float32(0)
		if hot {
			// As the progress does: thicker, with its knob.
			thick, knob = 6, 1
		}
		thick = s.AnimateTo("thick", thick, ui.Snappy)
		knob = s.AnimateTo("knob", knob, ui.Snappy)
		v := float32(a.volume)
		lamp := a.settings.NpLight
		s.Draw(func(p *ui.Painter, r ui.Rect) {
			y := r.Y + (r.H-thick)/2
			p.Fill(ui.Rect{X: r.X, Y: y, W: r.W, H: thick}, track, thick/2)
			filled := ui.Rect{X: r.X, Y: y, W: max(thick, r.W*v), H: thick}
			if lamp {
				drawBarGlow(p, filled, y+thick/2, a.glowColor, a.glowGain())
			}
			p.Fill(filled, color, thick/2)
			if knob > 0.01 {
				k := 12 * knob
				head, cy := r.X+r.W*v, r.Y+r.H/2
				p.Shadow(ui.Rect{X: head - k/2, Y: cy - k/2, W: k, H: k}, k/2, 0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.3))
				p.Fill(ui.Rect{X: head - k/2, Y: cy - k/2, W: k, H: k}, color, k/2)
			}
		})
		if pressed {
			*at = now
		}
		fade := float32(0)
		if since := now.Sub(*at); pressed || !at.IsZero() && since < volumeShown {
			fade = 1
			if !pressed {
				c.After(volumeShown - since)
			}
		}
		label := ui.Textf(c.Key("volume-percent"), "%d%%", int(a.volume*100+0.5)).FontSize(11).FontFeatures("tnum").
			TextColor(digits).TextAlign(ui.Center).Absolute().Top(-17).PassThrough()
		if back.A > 0 {
			label.Left(width/2-22).Width(44).Padding(1, 0).Radius(8).Background(back)
		} else {
			label.Left(0).Right(0)
		}
		label.Opacity(label.AnimateTo("shown", fade, ui.Animation{Duration: 220 * time.Millisecond}))
	})
}
