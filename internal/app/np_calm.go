package app

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Now Playing grows calm while it is left alone: the pointer still for
// npCalmStill, or out of the window for npCalmAway. The controls fade away,
// but the play button, and those of them that take room in the cover's
// column fold up, so the cover grows into it; the pointer moving, or a
// touch, brings them back. Nothing calms while something is held or open.

const (
	npCalmStill = 3 * time.Second
	npCalmAway  = 1200 * time.Millisecond
)

// npCalmState follows the pointer over Now Playing.
type npCalmState struct {
	at    mygo.Point // the pointer on the screen
	size  ui.Point   // of the window, as last built
	woke  bool       // pressed, scrolled or typed on while calm
	in    bool
	calm  bool      // calm, or growing so
	moved time.Time // the pointer last moved, came in or went out
	// folded is how tall each piece that folds up was, open.
	folded map[string]float32
	// infoH is how tall what plays (title and artist) came out, and
	// under how tall what goes under it, last laid out awake.
	infoH, under float32
	// cardA and cardC are whether the cover went beside the title last,
	// awake and calm.
	cardA, cardC bool
}

// foldedTotal is how much room the pieces that fold up take, open.
func (st *npCalmState) foldedTotal() float32 {
	var sum float32
	for _, h := range st.folded {
		sum += h
	}
	return sum
}

// npCalmness is how calm Now Playing is, 0 to 1, animated, for root, the
// view: it hides the pointer while calm over it.
func (a *app) npCalmness(c *ui.Context, root ui.Element, pointerIn bool) float32 {
	st := &a.more.npCalm
	now := c.Now()
	// Where the pointer is on the screen, asked as the view is built:
	// awake, it is built again only once the pointer may have been still
	// long enough, not each time it moves, which would build all of Now
	// Playing at the pointer's rate. Calm, it is built as the pointer
	// moves over it, for the first move to wake it.
	if st.calm {
		root.PointerPosition()
	}
	p := npCursor()
	// Out of the window the pointer moves over other apps, which is no
	// sign of anyone wanting the controls: only going in or out counts.
	// Resizing the window is being busy with it too: the pointer is on its
	// edge, out of the view, and the column calming as it is dragged made
	// the dragging stutter.
	w, h := c.Size()
	size := ui.Point{X: w, Y: h}
	if pointerIn != st.in || pointerIn && p != st.at || size != st.size || st.woke || st.moved.IsZero() {
		st.moved = now
	}
	st.size, st.woke = size, false
	st.at, st.in = p, pointerIn
	wait := npCalmAway
	if st.in {
		wait = npCalmStill
	}
	calm := a.settings.NpCalm && !a.npBusy() && a.pl.Current() != nil &&
		a.npPane != paneStage && a.npPane != paneWall
	if calm {
		if left := wait - now.Sub(st.moved); left > 0 {
			calm = false
			c.After(left)
		}
	}
	st.calm = calm
	target := float32(0)
	if calm {
		target = 1
	}
	// Calming is slow and even, nothing in a hurry; waking is quicker, for
	// the controls to be there as soon as they are wanted, but not abrupt.
	anim := ui.Animation{Duration: 1100 * time.Millisecond, Ease: npCalmEase}
	if !calm {
		anim = ui.Animation{Duration: 800 * time.Millisecond, Ease: npWakeEase}
	}
	if a.motion == motionOff {
		anim = ui.Animation{}
	}
	return root.AnimateTo("calm", target, anim)
}

// npCalmEase starts and ends gently; npWakeEase starts sooner, for the
// controls to answer the pointer, but not with the jump of motionEase,
// whose first frames did most of the way (the layout, which follows only
// part of the curve, went in a tenth of a second).
var (
	npCalmEase = ui.CubicBezier(0.45, 0, 0.2, 1)
	npWakeEase = ui.CubicBezier(0.3, 0, 0.15, 1)
)

// npCalmFade is how far the controls have faded at calmness k: they are
// gone by its middle, before anything moves much.
// The calmness is eased already: these take their part of it as it is,
// for a second ease on top would hurry its middle.
func npCalmFade(k float32) float32 { return clamp01(k / 0.5) }

// npCalmLay is how far the layout has gone to its calm one at calmness k:
// it starts once the controls are mostly faded, so nothing moves while it
// can still be read.
func npCalmLay(k float32) float32 { return clamp01((k - 0.2) / 0.8) }

// npCalmCover lies over a calm Now Playing, under its top strip (which
// drags the window), with no pointer: the pointer over a button or a line
// of lyrics took theirs, a hand, and stayed. The first press, wheel or key
// on it wakes the view, and does nothing else: what was under it is
// faded and may not be where it looks.
func (a *app) npCalmCover(c *ui.Context) {
	st := &a.more.npCalm
	if !st.calm || !st.in {
		return
	}
	cover := ui.Box(c.Key("np-calm-cover")).Absolute().Top(topBar(c)).Left(0).Right(0).Bottom(0).Cursor(ui.CursorNone)
	cover.HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind == ui.InputPointerMove {
			return false
		}
		st.woke = true
		a.redraw()
		return true
	})
}

// npBusy reports whether something in Now Playing is in use: a menu, a
// dialog or a popover open, or the progress held.
func (a *app) npBusy() bool {
	return a.menu.open || a.npScrub.held || a.npScrub.active || a.more.queue.open
}

// npFade fades e as Now Playing calms, and lets the pointer through it once
// mostly gone: a first touch then wakes the view rather than pressing what
// was there.
func npFade(e ui.Element, calm float32) ui.Element {
	if calm <= 0 {
		return e
	}
	e.Opacity(1 - calm)
	if calm > 0.5 {
		e.PassThrough()
	}
	return e
}

// npFold is a piece of the cover's column that folds up as Now Playing
// calms, fading as it goes: as tall as body is, open, and none at all once
// calm.
func (a *app) npFold(c *ui.Context, key string, calm float32, body func()) {
	st := &a.more.npCalm
	outer := npFade(ui.Box(c.Key(key)), npCalmFade(calm))
	if calm > 0 {
		// Clipped only as it folds: awake, what stands out of it shows
		// whole, as the volume's percent over its slider and the
		// quality's badge, taller than its row.
		outer.ClipY()
	}
	if h := st.folded[key]; calm > 0 && h > 0 {
		outer.MaxHeight(h * (1 - npCalmLay(calm)))
	}
	outer.Children(func() {
		inner := ui.Column(c.Key("open")).Children(body)
		if calm == 0 {
			if b := inner.LayoutBounds(); b.H > 0 {
				if st.folded == nil {
					st.folded = map[string]float32{}
				}
				st.folded[key] = b.H
			}
		}
	})
}

// npCursor is where the pointer is on the screen; tests, with no app
// running to ask, replace it.
var npCursor = func() mygo.Point { return mygo.Screen.CursorScreenPoint() }
