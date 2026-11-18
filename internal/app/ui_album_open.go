package app

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// An album of a list sorted by album — the local music's, the downloads',
// a playlist's — opens as a page of its own within the list, as the
// albums of the listener's own page open theirs: its cover flies from its
// card to the head of the page, its songs come in from the right, and
// going back (the button, the mouse's back button) shows the albums as
// they were, scrolled where they were, the cover flying back to its card.
// Before 1.3.2 the list was kept to the album in place, at once, the
// album index's chip turned into its name, and leaving it went to the top
// (a viewer, 1.3.1).

// albumOpen is the album the list is opened to, if one is.
func (a *app) albumOpen() (albumGroup, bool) {
	v := &a.plView
	if v.sort != sortAlbum || v.album == "" || !a.albumGridPage() || len(v.groups) != 1 || v.groups[0].key != v.album {
		return albumGroup{}, false
	}
	return v.groups[0], true
}

// openAlbum opens the album of key from the albums of the list, its cover
// flying from from, where its card shows it, if it is. It does so as the
// next frame builds the list (albumMoved): a click on a card comes while
// the list builds its rows, from the state it would swap.
func (a *app) openAlbum(key, cover string, from ui.Rect) {
	a.plView.move = &albumMove{key: key, cover: cover, from: from}
	a.redraw()
}

// closeAlbum goes back from the album open to the albums, as they were.
func (a *app) closeAlbum() {
	if a.plView.album != "" {
		// Where the head's cover is now, for it to fly back from: the frame
		// that closes it builds no head.
		a.plView.move = &albumMove{from: a.headAt}
		a.redraw()
	}
}

// An albumMove is an album to open, or with no key, the albums to go back
// to, as the list builds next.
type albumMove struct {
	key, cover string
	from       ui.Rect
}

// albumMoved opens or closes the album asked for, before the list builds.
// The list stays the same, its rows' contents keyed by the album
// (listView), so that the new come in over the old as a transition does,
// as the list and the grid switch; the albums come back scrolled to the
// row they showed at the top.
func (a *app) albumMoved(now time.Time) {
	v := &a.plView
	m := v.move
	if m == nil {
		return
	}
	v.move, v.indexOpen = nil, false
	switch {
	case m.key == v.album:
	case m.key != "":
		if v.album == "" {
			// The albums as they show, to come back to: the first row the
			// list counts is under the bar over it, the next at its top.
			if first, _ := a.trackList.Visible(); first > 0 {
				v.backRow = first + 1
			} else {
				v.backRow = 0
			}
		}
		v.album = m.key
		a.trackList.ScrollTo(0, ui.Start)
		a.albumSlide(48, now)
		if m.cover != "" && m.from.W > 0 && a.motion == motionFull {
			a.flight = coverFlight{url: m.cover, from: m.from, rFrom: 14}
		}
	default:
		g, open := a.albumOpen()
		v.album = ""
		a.trackList.ScrollTo(max(0, v.backRow), ui.Start)
		v.backRow = 0
		a.albumSlide(-48, now)
		if open && g.cover != "" && a.motion == motionFull && m.from.W > 0 {
			a.flight = coverFlight{url: g.cover, from: m.from, rTo: 14, back: true}
		}
	}
}

// albumSlide has the rows coming in slide from dx, as an album opens or
// closes.
func (a *app) albumSlide(dx float32, now time.Time) {
	a.more.switchedAt, a.more.slideX = now, dx
}

// albumHeadIn has e, the head of the list, come in as the rows do while an
// album opens or closes.
func (a *app) albumHeadIn(c *ui.Context, e ui.Element) {
	if a.motion == motionOff || a.more.slideX == 0 || c.Now().Sub(a.more.switchedAt) > 600*time.Millisecond {
		return
	}
	e.Transition(ui.ElementTransition{Animation: &viewSlide, Enter: &ui.Motion{X: a.more.slideX}})
}

// albumHead heads the album open: back to the albums, its cover, name and
// what it holds, and what plays it.
func (a *app) albumHead(c *ui.Context, g albumGroup, tracks []netease.Track) {
	p := a.pal
	// It slides in with the rows under it (the list's head, albumSlideIn).
	head := ui.Column(c.Key("album-open-"+g.key)).Gap(14).Padding(0, 0, 26, 0)
	head.Children(func() {
		back := ui.ButtonBase(c.Key("album-back")).Height(30).PaddingX(10).Gap(4).Radius(999).AlignItems(ui.Center).
			AlignSelf(ui.Start).FocusRing(false).Label("返回全部专辑").Margin(0, 0, 0, -10)
		if back.Hovered() {
			back.Background(p.hover)
		}
		back.Children(func() {
			ui.Icon(c, iconDown).Size(14, 14).Rotate(90).TextColor(p.muted)
			ui.Text(c, "全部专辑").FontSize(12.5).TextColor(p.muted)
		})
		if back.Clicked() {
			a.closeAlbum()
		}
		ui.Row(c).Gap(28).AlignItems(ui.End).Children(func() {
			if g.cover != "" {
				a.headCover(c, g.cover)
			} else {
				ui.Box(c).Size(212, 212).Radius(14).Background(p.hover).Center().AlignSelf(ui.Start).Children(func() {
					ui.Icon(c, iconAlbum).Size(64, 64).TextColor(p.muted)
				})
			}
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(8).Children(func() {
				ui.Text(c, "专辑").FontSize(12).FontWeight(600).TextColor(p.accent)
				ui.Text(c, g.name).FontSize(30).FontWeight(800).LetterSpacing(-0.6).MaxLines(2).LineHeight(1.2)
				ui.Text(c, fmt.Sprintf("%s · %d 首 · %s", g.artist, g.n, albumLength(g.length))).FontSize(13).TextColor(p.muted).SingleLine()
				ui.Row(c).Gap(10).Margin(10, 0, 0, 0).Children(func() {
					if pillButton(c.Key("play"), iconPlay, "播放", p.accent, p.onAccent).Clicked() {
						a.playList(tracks, false)
					}
					if pillButton(c.Key("shuffle"), iconShuffle, "随机播放", p.hover.Over(p.bg), p.text).Clicked() {
						a.playList(tracks, true)
					}
					if g.id > 0 && pillButton(c.Key("netease"), iconAlbum, "网易云专辑页", p.hover.Over(p.bg), p.text).Clicked() {
						a.open(route{page: pageAlbum, id: g.id})
					}
				})
			})
		})
	})
}
