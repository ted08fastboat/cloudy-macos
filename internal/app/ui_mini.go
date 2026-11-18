package app

import (
	"fmt"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/lyrics"
	"netease-music-mygo/internal/netease"
)

// The mini window can be dragged larger or smaller by its edges and
// corners, within a range, and lays itself out for the room it has:
//
//   - a strip, when it is short: the disc, the title and the controls in a
//     row, the wheel turning the volume with its percent in place of the
//     artists;
//   - as it always was, a little taller;
//   - a card, tall and wide: the cover beside the title, the line sung and
//     the one after it, the controls and the volume;
//   - a square, tall and narrow: the cover filling it, and all that over
//     its foot.

const (
	miniMinW, miniMaxW = 260, 720
	miniMinH, miniMaxH = 64, 440
	miniGrip           = 6  // how far in from an edge the pointer resizes
	miniCorner         = 14 // and from a corner
)

type miniLayout int

const (
	miniStrip miniLayout = iota
	miniStandard
	miniCard
	miniSquare
)

// miniLayoutOf is the layout of a mini window w by h.
func miniLayoutOf(w, h float32) miniLayout {
	switch {
	case h < 100:
		return miniStrip
	case h < 170:
		return miniStandard
	case w >= h*1.45:
		return miniCard
	}
	return miniSquare
}

// miniSize is the mini window's size, as it was left, within the range.
func (a *app) miniSize() (int, int) {
	w, h := a.settings.MiniW, a.settings.MiniH
	if w == 0 || h == 0 {
		return miniW, miniH
	}
	return min(max(w, miniMinW), miniMaxW), min(max(h, miniMinH), miniMaxH)
}

// miniResize is a resize of the mini window under way: from which edges,
// and where the window and the pointer were as it began. On Windows it
// follows the pointer and the button over the whole screen, from a
// goroutine, rather than the grip's own press: a grip is a few DIPs wide,
// and a pointer moving faster than the window grows leaves it at once.
type miniResize struct {
	key            string
	left, top      bool // the edges that move: else the right and the foot
	horiz, vert    bool // which ways it resizes
	from           mygo.Rectangle
	pointer        mygo.Point
	shownW, shownH int
	gen            int  // which resize, for what the goroutine posts
	tracked        bool // the goroutine follows it
}

// miniGrips are the mini window's edges and corners, which resize it.
func (a *app) miniGrips(c *ui.Context) {
	type grip struct {
		key                    string
		left, top, horiz, vert bool
		cursor                 ui.Cursor
	}
	grips := []grip{
		{"n", false, true, false, true, ui.CursorResizeNS},
		{"s", false, false, false, true, ui.CursorResizeNS},
		{"w", true, false, true, false, ui.CursorResizeEW},
		{"e", false, false, true, false, ui.CursorResizeEW},
		{"nw", true, true, true, true, ui.CursorResizeNWSE},
		{"se", false, false, true, true, ui.CursorResizeNWSE},
		{"ne", false, true, true, true, ui.CursorResizeNESW},
		{"sw", true, false, true, true, ui.CursorResizeNESW},
	}
	r := &a.more.miniResize
	held := false
	for _, g := range grips {
		b := ui.ButtonBase(c.Key("grip-" + g.key)).Absolute().FocusRing(false).Cursor(g.cursor).Label("调整迷你窗口大小")
		switch g.key {
		case "n":
			b.Top(0).Left(miniCorner).Right(miniCorner).Height(miniGrip)
		case "s":
			b.Bottom(0).Left(miniCorner).Right(miniCorner).Height(miniGrip)
		case "w":
			b.Left(0).Top(miniCorner).Bottom(miniCorner).Width(miniGrip)
		case "e":
			b.Right(0).Top(miniCorner).Bottom(miniCorner).Width(miniGrip)
		case "nw":
			b.Top(0).Left(0).Size(miniCorner, miniCorner)
		case "se":
			b.Bottom(0).Right(0).Size(miniCorner, miniCorner)
		case "ne":
			b.Top(0).Right(0).Size(miniCorner, miniCorner)
		case "sw":
			b.Bottom(0).Left(0).Size(miniCorner, miniCorner)
		}
		if b.DoubleClicked() {
			// As it was at first.
			a.setMiniBounds(a.miniBoundsFor(miniW, miniH, g.left, g.top))
			a.settings.MiniW, a.settings.MiniH = 0, 0
			a.settings.save()
			continue
		}
		if !b.Pressed() || a.miniWin == nil {
			continue
		}
		held = true
		if r.key == "" {
			a.startMiniResize(g.key, g.left, g.top, g.horiz, g.vert)
		}
	}
	switch {
	case r.key == "" || r.tracked:
	case held:
		a.miniResizeTo(mygo.Screen.CursorScreenPoint())
		c.AnimationFrame()
	default:
		a.endMiniResize()
	}
}

func (a *app) startMiniResize(key string, left, top, horiz, vert bool) {
	r := &a.more.miniResize
	gen := r.gen + 1
	*r = miniResize{key: key, left: left, top: top, horiz: horiz, vert: vert, gen: gen,
		from: a.miniWin.Bounds(), pointer: mygo.Screen.CursorScreenPoint()}
	if _, known := primaryButtonDown(); !known {
		return
	}
	r.tracked = true
	go func() {
		tick := time.NewTicker(8 * time.Millisecond)
		defer tick.Stop()
		var last mygo.Point
		for range tick.C {
			down, _ := primaryButtonDown()
			p := mygo.Screen.CursorScreenPoint()
			if down && p == last {
				continue
			}
			last = p
			a.post(func() {
				if r.gen != gen || r.key == "" {
					return
				}
				a.miniResizeTo(p)
				if !down {
					a.endMiniResize()
				}
			})
			if !down {
				return
			}
		}
	}()
}

// miniResizeTo resizes the mini window for the pointer at p.
func (a *app) miniResizeTo(p mygo.Point) {
	r := &a.more.miniResize
	w, h := r.from.Width, r.from.Height
	if r.horiz {
		dx := p.X - r.pointer.X
		if r.left {
			dx = -dx
		}
		w = min(max(r.from.Width+dx, miniMinW), miniMaxW)
	}
	if r.vert {
		dy := p.Y - r.pointer.Y
		if r.top {
			dy = -dy
		}
		h = min(max(r.from.Height+dy, miniMinH), miniMaxH)
	}
	if w == r.shownW && h == r.shownH {
		return
	}
	r.shownW, r.shownH = w, h
	nb := mygo.Rectangle{X: r.from.X, Y: r.from.Y, Width: w, Height: h}
	if r.left {
		nb.X = r.from.X + r.from.Width - w
	}
	if r.top {
		nb.Y = r.from.Y + r.from.Height - h
	}
	a.setMiniBounds(nb)
}

// endMiniResize keeps the size let go at for the next time.
func (a *app) endMiniResize() {
	r := &a.more.miniResize
	if r.shownW > 0 {
		a.settings.MiniW, a.settings.MiniH = r.shownW, r.shownH
		a.settings.save()
	}
	*r = miniResize{gen: r.gen}
}

// miniBoundsFor is the mini window at size w by h, keeping the edges that
// do not move where they are.
func (a *app) miniBoundsFor(w, h int, left, top bool) mygo.Rectangle {
	b := a.miniWin.Bounds()
	nb := mygo.Rectangle{X: b.X, Y: b.Y, Width: w, Height: h}
	if left {
		nb.X = b.X + b.Width - w
	}
	if top {
		nb.Y = b.Y + b.Height - h
	}
	return nb
}

func (a *app) setMiniBounds(b mygo.Rectangle) {
	if a.miniWin == nil {
		return
	}
	a.miniWin.SetBounds(b)
	a.settings.MiniX, a.settings.MiniY = b.X, b.Y
}

// miniView is what plays, in small: over its cover's colors, as Now Playing,
// with how far it has played and how loud along with what controls it.
func (a *app) miniView(c *ui.Context) {
	a.floatTheme(c)
	a.followCover()
	pl := a.pl
	tr := pl.Current()
	w, h := c.Size()
	layout := miniLayoutOf(w, h)
	root := ui.Box(c.Key("mini")).Fill().Radius(miniRadius(layout, h)).TextColor(white).
		Background(a.miniSurface()).Border(1, white.Alpha(0.14)).DragWindow()
	over := root.Hovered()
	root.HandleInput(func(ev ui.InputEvent) bool {
		if a.scrollVolume(ev) {
			a.more.miniVolumeAt = c.Now()
			return true
		}
		return false
	})
	root.Children(func() {
		switch layout {
		case miniStrip:
			a.miniStripView(c, tr, w, h, over)
		case miniStandard:
			a.miniStandardView(c, tr, w, over)
		case miniCard:
			a.miniCardView(c, tr, w, h, over)
		default:
			a.miniSquareView(c, tr, w, h, over)
		}
		a.miniGrips(c)
	})
	switch {
	case layout >= miniCard && pl.Playing():
		c.After(200 * time.Millisecond) // the line sung
	case pl.Playing():
		c.After(time.Second)
	}
}

func miniRadius(l miniLayout, h float32) float32 {
	if l == miniStrip {
		return min(26, h/2)
	}
	return 26
}

// restoreFromMini is what a click on the mini window's cover does; tests
// count it. Set in init: closeMini leads back to miniCover.
var restoreFromMini func(*app)

func init() { restoreFromMini = (*app).closeMini }

// miniCover is the cover at size: the disc, turning, while small; else the
// cover, square. Hovered, it dims under the expand mark, and a click on it
// goes back to the main window. As a button it is no handle of the window,
// as the rest of it is: it does not move the window.
func (a *app) miniCover(c *ui.Context, tr *netease.Track, size float32, square bool) {
	box := ui.ButtonBase(c.Key("mini-cover")).Size(size, size).Shrink(0).FocusRing(false).
		Cursor(ui.CursorPointer).Label("迷你窗口封面")
	a.more.miniCoverAt = box.LayoutBounds()
	radius := size / 2
	if square && tr != nil {
		radius = 14
	}
	a.more.miniCoverRound = radius
	if box.Clicked() {
		restoreFromMini(a)
	}
	hovered := box.Hovered()
	box.Children(func() {
		switch {
		case tr == nil:
			ui.Box(c).Size(size, size).Radius(999).Center().Background(white.Alpha(0.08)).Children(func() { mascotView(c, size*0.72) })
		case square:
			ui.Box(c).Size(size, size).Radius(14).Shadow(0, 8, 24, -6, ui.RGBA(0, 0, 0, 0.45)).Children(func() {
				a.coverFill(c, tr.Album.PicURL, miniArtPx(size, size), 14)
			})
		default:
			frame := discSmooth
			if size < 60 {
				frame = discFrame
			}
			a.barDisc(c, tr, size, frame)
		}
		if hovered {
			mark := min(24, max(16, size*0.28))
			ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(radius).Center().PassThrough().
				Background(ui.RGBA(0, 0, 0, 0.35)).TextColor(white).Children(func() {
				ui.Icon(c, iconExpand).Size(mark, mark)
			})
		}
	})
}

// miniTitle is the title and the artists, or the volume's percent in the
// artists' place while the wheel turns it, where there is no slider.
func (a *app) miniTitle(c *ui.Context, tr *netease.Track, size float32, percent bool) {
	ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(1).Children(func() {
		name, sub := "未在播放", ""
		if tr != nil {
			name, sub = tr.Name, firstArtist(tr)
		}
		a.marqueeLine(c, "mini-title", name, size, 700, 0)
		if percent {
			if since := c.Now().Sub(a.more.miniVolumeAt); !a.more.miniVolumeAt.IsZero() && since < volumeShown {
				c.After(volumeShown - since)
				sub = fmt.Sprintf("音量 %d%%", int(a.volume*100+0.5))
			}
		}
		ui.Text(c, sub).FontSize(size * 0.86).TextColor(white.Alpha(0.62)).SingleLine()
	})
}

// miniBack goes back to the main window, after the pin if pin.
func (a *app) miniBack(c *ui.Context, over, pin bool) {
	if pin {
		a.miniPin(c, over)
	}
	ink := white.Alpha(0.35)
	if over {
		ink = white.Alpha(0.85)
	}
	if iconButton(c.Key("back"), iconExpand, 14, ink, "回到主窗口").Size(26, 26).Shrink(0).Tooltip("回到主窗口").Clicked() {
		a.closeMini()
	}
}

// miniControls are previous, play and next, then as room allows the heart
// and the lyrics on the desktop.
func (a *app) miniControls(c *ui.Context, tr *netease.Track, scale float32, extras bool) {
	pl := a.pl
	bw, bh := 30*scale, 28*scale
	if iconButton(c.Key("prev"), iconPrev, 14*scale, white, "上一首").Size(bw, bh).Clicked() {
		pl.Prev()
	}
	if playButton(c.Key("play"), pl.Playing(), 18*scale, white).Size(bw, bh).Clicked() {
		pl.TogglePlay()
	}
	if iconButton(c.Key("next"), iconNext, 14*scale, white, "下一首").Size(bw, bh).Clicked() {
		pl.Next()
	}
	if !extras {
		return
	}
	if tr != nil && tr.LocalPath == "" {
		a.heartButton(c.Key("like"), tr, 14*scale, white, ui.Hex("#ff3b43"), iconHeart, iconHeartFill).Size(bw, bh)
	}
	desk, tip := white.Alpha(0.45), "打开桌面歌词"
	if a.settings.DeskLyrics {
		desk, tip = white, "关闭桌面歌词"
	}
	if iconButton(c.Key("desk"), iconSubtitles, 14*scale, desk, tip).Size(bw, bh).Clicked() {
		a.toggleDeskLyrics()
	}
}

// miniVolume is the speaker and the slider, width wide in all.
func (a *app) miniVolume(c *ui.Context, width float32) {
	ui.Row(c.Key("mini-volume")).Gap(4).AlignItems(ui.Center).Shrink(0).Children(func() {
		a.muteButton(c, white.Alpha(0.75))
		a.volumeBar(c, "mini-volume-bar", max(48, width-36), white.Alpha(0.75), white.Alpha(0.18), white.Alpha(0.9),
			&a.more.miniVolumeAt, a.miniSurface().Alpha(0.92))
	})
}

func (a *app) miniScrubber(c *ui.Context) ui.Element {
	return a.scrubber(c.Key("mini-scrub"), &a.miniScrub, white.Alpha(0.18), white.Alpha(0.85), 14, ui.Color{})
}

// miniStripView is the strip: everything in a row.
func (a *app) miniStripView(c *ui.Context, tr *netease.Track, w, h float32, over bool) {
	disc := h - 20
	ui.Row(c.Key("strip")).Fill().Padding(0, 12, 0, 10).Gap(10).AlignItems(ui.Center).Children(func() {
		a.miniCover(c, tr, disc, false)
		a.miniTitle(c, tr, 13.5, true)
		ui.Row(c).Gap(0).AlignItems(ui.Center).Shrink(0).Children(func() {
			a.miniControls(c, tr, 1, w >= 380)
		})
		a.miniBack(c, over, w >= 340)
	})
	a.miniScrubber(c).Absolute().Left(h / 2).Right(h / 2).Bottom(-3)
}

// miniStandardView is the mini window as it always was.
func (a *app) miniStandardView(c *ui.Context, tr *netease.Track, w float32, over bool) {
	ui.Row(c.Key("top")).Absolute().Top(12).Left(14).Right(12).Gap(13).AlignItems(ui.Start).Children(func() {
		a.miniCover(c, tr, miniDisc, false)
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(2).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Start).Children(func() {
				a.miniTitle(c, tr, 14, w < 380)
				a.miniBack(c, over, true)
			})
			ui.Row(c).Gap(2).AlignItems(ui.Center).Margin(3, 0, 0, -6).Children(func() {
				a.miniControls(c, tr, 1, w >= 330)
				if w >= 380 {
					ui.Spacer(c)
					a.miniVolume(c, min(128, w-330))
				}
			})
		})
	})
	a.miniScrubber(c).Absolute().Left(18).Right(18).Bottom(5)
}

// miniLyric is the line sung, and the one after it or its translation.
func (a *app) miniLyric(c *ui.Context, tr *netease.Track, size float32, center bool) {
	main, next := "", ""
	if tr != nil {
		lines, _ := a.shownLyrics()
		if cur := lyrics.Index(lines, a.lyricPos()); cur >= 0 && cur < len(lines) {
			main = lines[cur].Text
			switch {
			case lines[cur].Translation != "" && a.settings.LyricTrans:
				next = lines[cur].Translation
			case cur+1 < len(lines):
				next = lines[cur+1].Text
			}
		} else if len(lines) > 0 {
			next = lines[0].Text
		}
		if main == "" && next == "" {
			main = tr.Album.Name
		}
	}
	// Centered, alone on the cover, a long line takes two lines rather
	// than ending in an ellipsis.
	col := ui.Column(c.Key("mini-lyric")).Gap(4).MinWidth(0).AlignItems(ui.Stretch)
	a.lyricFace(col)
	col.Children(func() {
		line := ui.Text(c.Key(main), main).FontSize(size).FontWeight(700).TextColor(white.Alpha(0.95)).
			Transition(ui.ElementTransition{Animation: &motionPane, Enter: &ui.Motion{Y: 8}, Exit: &ui.Motion{Y: -8}})
		after := ui.Text(c.Key("next"+next), next).FontSize(size * 0.78).TextColor(white.Alpha(0.5))
		if center {
			line.TextAlign(ui.Center).MaxLines(2).LineHeight(1.3)
			after.TextAlign(ui.Center).MaxLines(1)
		} else {
			line.SingleLine()
			after.SingleLine()
		}
	})
}

// miniCardView is the card: the cover beside all the rest.
func (a *app) miniCardView(c *ui.Context, tr *netease.Track, w, h float32, over bool) {
	side := min(h-32, w*0.42, 300)
	scale := min(1.3, max(1, h/220))
	ui.Row(c.Key("card")).Fill().Padding(16, 18, 16, 16).Gap(18).AlignItems(ui.Center).Children(func() {
		a.miniCover(c, tr, side, true)
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Height(side).Gap(0).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Start).Children(func() {
				a.miniTitle(c, tr, 16*scale, false)
				a.miniBack(c, over, true)
			})
			ui.Spacer(c)
			a.miniLyric(c, tr, 15*scale, false)
			ui.Spacer(c)
			ui.Row(c).Gap(2).AlignItems(ui.Center).Margin(0, 0, 4, -6).Children(func() {
				a.miniControls(c, tr, scale, true)
				ui.Spacer(c)
			})
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				a.miniScrubber(c).Grow(1)
				if w-side >= 330 {
					a.miniVolume(c, 120)
				}
			})
		})
	})
}

// miniSquareView is the square: the cover filling it, the rest over its
// foot, on a shade that keeps it legible. A moment after the pointer leaves
// it, all but the lyrics fade away and the lyrics settle, larger and in the
// middle, low on the cover, as an album's sleeve would set them; the
// pointer coming back brings the rest back at once.
func (a *app) miniSquareView(c *ui.Context, tr *netease.Track, w, h float32, over bool) {
	now := c.Now()
	if over || a.more.miniResize.key != "" {
		a.more.miniHoverAt = now
	}
	idle := false
	if since := now.Sub(a.more.miniHoverAt); !a.more.miniHoverAt.IsZero() && since >= miniIdleAfter {
		idle = true
	} else if !over {
		c.After(miniIdleAfter - since)
	}
	if a.more.miniHoverAt.IsZero() {
		a.more.miniHoverAt = now
		c.After(miniIdleAfter)
	}
	anim := ui.Box(c.Key("square-idle")).Absolute().Size(0, 0).PassThrough()
	target := float32(0)
	if idle {
		target = 1
	}
	dur := 520 * time.Millisecond
	if !idle {
		dur = 220 * time.Millisecond // back at once for the pointer
	}
	k := anim.AnimateTo("k", target, ui.Animation{Duration: dur, Ease: ui.EaseInOut})

	if tr != nil {
		ui.Box(c.Key("square-art")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(26).PassThrough().Children(func() {
			a.coverFill(c, tr.Album.PicURL, miniArtPx(w, h), 26)
		})
		a.more.miniCoverAt, a.more.miniCoverRound = ui.Rect{X: 0, Y: 0, W: w, H: h}, 26
	}
	ui.Box(c.Key("square-shade")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(26).PassThrough().
		Draw(func(p *ui.Painter, r ui.Rect) { drawSquareShade(p, r, k) })

	scale := min(1.3, max(1, w/300))
	rest := 1 - k
	if rest > 0.01 {
		ui.Row(c).Absolute().Top(10).Right(10).Opacity(rest).Children(func() { a.miniBack(c, true, true) })
		ui.Column(c.Key("square-foot")).Absolute().Left(18).Right(18).Bottom(14).Gap(6).AlignItems(ui.Stretch).
			Opacity(rest).Translate(0, 10*k).Children(func() {
			a.miniTitle(c, tr, 17*scale, false)
			if h >= 260 {
				a.miniLyric(c, tr, 14*scale, false)
			}
			ui.Row(c).Gap(2).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				a.miniControls(c, tr, scale, true)
			})
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				a.miniScrubber(c).Grow(1)
				if w >= 340 {
					a.miniVolume(c, 110)
				}
			})
		})
	}
	if k > 0.01 {
		// Alone, the lyrics: centered, low, and a size larger, rising into
		// place as the rest goes.
		ui.Column(c.Key("square-lyric")).Absolute().Left(22).Right(22).Bottom(h*0.1+8).AlignItems(ui.Center).
			Opacity(k).Translate(0, 14*(1-k)).PassThrough().Shadow(0, 2, 12, 0, ui.RGBA(0, 0, 0, 0.35)).Children(func() {
			a.miniLyric(c, tr, 19*scale, true)
		})
	}
}

// miniIdleAfter is how long after the pointer leaves the square that all
// but its lyrics fade away.
const miniIdleAfter = 1500 * time.Millisecond

// drawSquareShade darkens the cover of the square where things are over it:
// its top a little, for its buttons, and its foot, for the rest. Idle (k
// 1), only the foot, under the lyrics, and less.
func drawSquareShade(p *ui.Painter, r ui.Rect, k float32) {
	top := 0.25 * (1 - k)
	mid := 0.15 * (1 - k)
	foot := lerp(0.78, 0.62, k)
	p.FillGradient(r, ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
		{Color: ui.RGBA(0, 0, 0, top), At: 0}, {Color: ui.RGBA(0, 0, 0, 0), At: 0.22},
		{Color: ui.RGBA(0, 0, 0, mid), At: lerp(0.42, 0.55, k)}, {Color: ui.RGBA(0, 0, 0, foot), At: 1},
	}}, 26)
}

// miniArtPx is the size the mini window asks its cover at, w by h, in a few
// steps, for the morph and the window to share one image.
func miniArtPx(w, h float32) int {
	switch side := max(w, h) * 1.5; {
	case side <= 240:
		return 240
	case side <= 480:
		return 480
	}
	return 800
}

// applyMiniLayer keeps the mini window over the other windows, or, kept on
// the desktop, under them.
func (a *app) applyMiniLayer() {
	if a.miniWin == nil {
		return
	}
	a.miniWin.SetAlwaysOnTop(!a.settings.MiniDesk)
	a.sinkMini()
}

// sinkMini puts the mini window back under the others, when it is kept on
// the desktop.
func (a *app) sinkMini() {
	if a.miniWin != nil && a.settings.MiniDesk && a.more.miniResize.key == "" {
		sinkWindow(a.miniWin.NativeHandle())
	}
}

// miniPin keeps the mini window on the desktop, or over the others again.
func (a *app) miniPin(c *ui.Context, over bool) {
	ink, tip := white.Alpha(0.35), "固定在桌面"
	if over {
		ink = white.Alpha(0.85)
	}
	if a.settings.MiniDesk {
		ink, tip = ui.Hex("#7cc4ff"), "取消固定，回到最上层"
	}
	// Tilted, as a pushpin is pressed in: along the arrows beside it.
	pin := roundButton(c.Key("pin"), 14, ink, tip).Size(26, 26).Shrink(0).Tooltip(tip)
	pin.Children(func() { ui.Icon(c, iconPin).Size(14, 14).Rotate(45) })
	if pin.Clicked() {
		a.settings.MiniDesk = !a.settings.MiniDesk
		a.settings.save()
		a.applyMiniLayer()
	}
}
