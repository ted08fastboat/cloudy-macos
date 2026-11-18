package app

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// A title too long for its room slides, as players show one: it waits a
// moment at its start, slides left until its end shows, waits, and slides
// back. It does so a few times after it first shows, and again whenever the
// pointer comes over it, rather than for ever, which would keep the window
// drawing frames for nothing.

const (
	marqueeHold   = 2500 * time.Millisecond // at either end
	marqueeSpeed  = 32                      // DIPs a second
	marqueeRounds = 3                       // there and back, each time it starts
)

type marqueeState struct {
	text  string
	start time.Time
	hover bool
	slide time.Duration // how long it takes to cross, last it was laid out
}

// marqueeLine is text on one line in the room it is given, sliding when it
// does not fit. The box is what to place and size; the label is the text,
// for what flies from it.
func (a *app) marqueeLine(c *ui.Context, key, text string, size float32, weight int, spacing float32) (box, label ui.Element) {
	box = ui.Box(c.Key(key)).Shrink(1).MinWidth(0).ClipX()
	if a.more.marquees == nil {
		a.more.marquees = map[string]*marqueeState{}
	}
	s := a.more.marquees[key]
	now := time.Now()
	if s == nil || s.text != text {
		s = &marqueeState{text: text, start: now}
		a.more.marquees[key] = s
	}
	over := box.Hovered()
	if over && !s.hover && a.marqueeDone(s, now) {
		s.start = now
	}
	s.hover = over

	room := box.LayoutBounds().W
	w, _ := c.MeasureText(0, ui.Span{Text: text, Size: size, Weight: weight, LetterSpacing: spacing})
	run := w - room
	if room <= 0 || run <= 1 {
		box.Children(func() {
			label = ui.Text(c, text).FontSize(size).FontWeight(weight).LetterSpacing(spacing).SingleLine()
		})
		return box, label
	}
	slide := time.Duration(float64(run) / marqueeSpeed * float64(time.Second))
	s.slide = slide
	off := float32(0)
	if !a.marqueeDone(s, now) {
		round := 2*marqueeHold + 2*slide
		t := now.Sub(s.start) % round
		switch {
		case t < marqueeHold:
			c.After(marqueeHold - t)
		case t < marqueeHold+slide:
			off = run * float32(easeInOut(float64(t-marqueeHold)/float64(slide)))
			c.AnimationFrame()
		case t < 2*marqueeHold+slide:
			off = run
			c.After(2*marqueeHold + slide - t)
		default:
			off = run * float32(1-easeInOut(float64(t-2*marqueeHold-slide)/float64(slide)))
			c.AnimationFrame()
		}
	}
	box.Children(func() {
		label = ui.Text(c, text).FontSize(size).FontWeight(weight).LetterSpacing(spacing).NoWrap().Translate(-off, 0)
	})
	return box, label
}

// marqueeDone reports whether a title has slid its rounds.
func (a *app) marqueeDone(s *marqueeState, now time.Time) bool {
	return now.Sub(s.start) >= marqueeRounds*(2*marqueeHold+2*s.slide)
}

func easeInOut(t float64) float64 {
	t = max(0, min(1, t))
	return t * t * (3 - 2*t)
}
