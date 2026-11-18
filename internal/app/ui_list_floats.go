package app

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// listFloatAway is how many rows a list is scrolled past before its button
// back to the top shows.
const listFloatAway = 12

// listFloats are the two small round buttons at the lower right of a list
// of songs, over the rows: back to the top, once scrolled well away from
// it, and under it, to the song playing, when it is in the list and out of
// view. Long playlists and the daily songs had no way back but scrolling
// (viewers, 1.3.1).
func (a *app) listFloats(c *ui.Context, bottom float32, grid bool) {
	first, last := a.trackList.Visible()
	if last < first {
		return
	}
	// The glide to the song playing went where the rows' heights put it:
	// where that fell short of it (album titles are taller), the list
	// steps the rest once it stops.
	// The glide to the song playing knows where it starts, and how far
	// the list goes, once the list took it in a frame: it heads there now.
	if a.listAim > 0 {
		if a.listWait--; a.listWait <= 0 {
			// Rows are of a height, nearly all: as many rows from those in
			// view as it is, by the height they take.
			rowH := a.listBox.H / float32(last-first+1)
			y := a.listScroll.Y + (float32(a.listAim)-float32(first+last)/2)*rowH
			a.listScroll.ScrollTo(0, max(0, min(y, a.listScroll.MaxY)), listGlide)
			a.listLocate, a.listAim = a.listAim, 0
		}
		c.AnimationFrame()
	}
	if a.listLocate > 0 && !a.listScroll.Scrolling() {
		if row := a.listLocate; row < first || row > last {
			a.trackList.ScrollTo(row, ui.Center)
		}
		a.listLocate = -1
	}
	top := first > listFloatAway
	locate := -1
	if cur := a.pl.Current(); cur != nil && !grid {
		for i, tr := range a.shownTracks {
			if tr.ID == cur.ID && i < len(a.trackRow) {
				if row := a.trackRow[i]; row < first || row > last {
					locate = row
				}
				break
			}
		}
	}
	if !top && locate < 0 {
		return
	}
	p := a.pal
	ui.Column(c.Key("list-floats")).Absolute().Right(pagePad).Bottom(bottom + 16).Gap(10).Children(func() {
		float := func(key string, label string, icon *ui.SVG, turn float32) ui.Element {
			b := ui.ButtonBase(c.Key(key)).Size(40, 40).Center().Radius(999).TextColor(p.text.Alpha(0.8)).
				FocusRing(false).Label(label).Tooltip(label)
			a.pane(b, 20, false).Shadow(0, 4, 14, -4, ui.RGBA(0, 0, 0, 0.3))
			if b.Hovered() {
				b.TextColor(p.text)
			}
			b.Transition(ui.ElementTransition{Animation: &motionPane, Enter: &ui.Motion{Scale: 0.6}, Exit: &ui.Motion{Scale: 0.6}})
			b.Children(func() { ui.Icon(c, icon).Size(16, 16).Rotate(turn) })
			return b
		}
		// Both glide there, rather than jump (the user, 1.3.2-test.1).
		// The list takes a ScrollState only while it glides (trackRowsGrouped),
		// which starts from where it shows.
		if top && float("to-top", "回到顶部", iconDown, 180).Clicked() {
			a.listLocate, a.listAim = -1, 0
			a.listScroll = ui.ScrollState{}
			a.listScroll.ScrollTo(0, 0, listGlide)
		}
		if locate >= 0 && float("to-playing", "定位到正在播放", iconLocate, 0).Clicked() {
			// Held where it is until the list says where that is (listAim).
			a.listScroll = ui.ScrollState{}
			a.listScroll.ScrollTo(0, 0, ui.Animation{Delay: time.Hour, Duration: time.Millisecond})
			a.listLocate, a.listAim, a.listWait = -1, locate, 2
		}
	})
}

// listGlide is how the list glides to the top or to the song playing.
var listGlide = ui.Animation{Duration: 520 * time.Millisecond, Ease: ui.EaseInOut}
