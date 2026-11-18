package app

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// The pages of the home page's calendar beyond the daily card and 风格推荐:
// the personal FM, playing there; 私人雷达, with the other radars beside
// it; and 听歌周报, the songs played most in the last seven days. Which of
// them the calendar holds, and in what order, the listener picks, in the
// settings or under the bookmarks at the calendar's side.

// deckAll is every page the calendar may hold, in the order it holds them
// unless the listener orders them otherwise.
var deckAll = []deckPage{deckDaily, deckStyle, deckRadar, deckFM, deckWeek}

// deckID names a page in the settings.
func deckID(pg deckPage) string {
	return [...]string{"daily", "style", "fm", "radar", "week"}[pg]
}

// deckName names a page, for the strip that flips to it and the settings.
func deckName(pg deckPage) string {
	return [...]string{"每日推荐", "风格推荐", "私人 FM", "私人雷达", "听歌周报"}[pg]
}

// deckShort is a page's name on its bookmark, two characters, one over
// the other.
func deckShort(pg deckPage) string {
	return [...]string{"日推", "风格", "FM", "雷达", "周报"}[pg]
}

// deckOrder is the pages in the order the listener put them in, those
// they never placed after, as deckAll has them.
func (s *settings) deckOrder() []deckPage {
	var order []deckPage
	for _, id := range s.DeckOrder {
		for _, pg := range deckAll {
			if deckID(pg) == id && !slices.Contains(order, pg) {
				order = append(order, pg)
			}
		}
	}
	for _, pg := range deckAll {
		if !slices.Contains(order, pg) {
			order = append(order, pg)
		}
	}
	return order
}

// deckOn reports whether the calendar holds pg.
func (s *settings) deckOn(pg deckPage) bool {
	return !slices.Contains(s.DeckOff, deckID(pg))
}

// deckRemove takes pg out of the calendar, keeping one page in it at
// least.
func (a *app) deckRemove(pg deckPage) {
	s := &a.settings
	if !s.deckOn(pg) || len(a.deckKept()) <= 1 {
		return
	}
	s.DeckOff = append(s.DeckOff, deckID(pg))
	a.changed()
}

// deckAdd puts pg back in the calendar, after the pages in it.
func (a *app) deckAdd(pg deckPage) {
	s := &a.settings
	if s.deckOn(pg) {
		return
	}
	s.DeckOff = slices.DeleteFunc(s.DeckOff, func(id string) bool { return id == deckID(pg) })
	kept := a.deckKept()
	order := slices.DeleteFunc(s.deckOrder(), func(p deckPage) bool { return p == pg })
	at := 0
	for i, p := range order {
		if slices.Contains(kept, p) {
			at = i + 1
		}
	}
	a.deckSetOrder(slices.Insert(order, at, pg))
}

// deckKept is the pages the calendar keeps, in their order.
func (a *app) deckKept() []deckPage {
	var kept []deckPage
	for _, pg := range a.settings.deckOrder() {
		if a.settings.deckOn(pg) {
			kept = append(kept, pg)
		}
	}
	return kept
}

// deckSetOrder keeps order as the calendar's.
func (a *app) deckSetOrder(order []deckPage) {
	ids := make([]string, len(order))
	for k, p := range order {
		ids[k] = deckID(p)
	}
	a.settings.DeckOrder = ids
	a.changed()
}

// deckHas reports whether pg has what it shows.
func (a *app) deckHas(pg deckPage) bool {
	switch pg {
	case deckDaily:
		return len(a.daily) > 0
	case deckStyle:
		return a.styleCardShown()
	case deckFM, deckRadar:
		return a.profile != nil
	case deckWeek:
		return a.profile != nil && len(a.week.songs) > 0
	}
	return false
}

// deckCover is the cover whose colors a page takes, if it has one yet.
func (a *app) deckCover(pg deckPage) string {
	switch pg {
	case deckDaily:
		if len(a.daily) > 0 {
			return a.daily[0].Album.PicURL
		}
	case deckStyle:
		return styleCover(a.style.songs.Songs)
	case deckFM:
		if tr := a.fmPlaying(); tr != nil {
			return tr.Album.PicURL
		}
	case deckRadar:
		pl := a.radar.lists[radars[0].id]
		return pl.Cover()
	case deckWeek:
		if len(a.week.songs) > 0 {
			return a.week.songs[0].Track.Album.PicURL
		}
	}
	return ""
}

// deckTint is a page's color, for its bookmark: as its card's.
func (a *app) deckTint(pg deckPage) ui.Color {
	var art *artwork
	if url := a.deckCover(pg); url != "" {
		art = a.art.get(netease.Sized(url, 600), true)
	}
	_, rich, _ := dailyColors(art)
	return rich
}

// deckInner is how wide the calendar's pages are.
func (a *app) deckInner(c *ui.Context) float32 {
	w, _ := c.Size()
	return w - a.sideWidth(w) - 2*pagePad - deckTabW
}

// deckSheet is a page of the calendar laid out as the daily card is: a
// card as tall, in the colors of cover, words at the left and side at the
// right; clicked, it opens what open opens, if anything. Lying under
// another page (back), it keeps still and takes no pointer.
func (a *app) deckSheet(c *ui.Context, pg deckPage, back bool, cover string, open func(), words func(), side func(hovered bool)) {
	// The glow is behind the cover at the right; 听歌周报's page, with bars
	// there and covers beside them, has none.
	glows := pg != deckWeek
	// Its middle, from the right: the large cover's, or the small one's on
	// narrow pages.
	glowAt := float32(dailyPadX + 12 + 75)
	if a.deckInner(c) < 620 {
		glowAt = dailyPadX + 12 + deckSmallCover/2
	}
	var art *artwork
	if cover != "" {
		art = a.art.get(netease.Sized(cover, 600), true)
	}
	still := c.Preferences().ReduceMotion
	card := ui.ButtonBase(c.Key(deckID(pg))).Height(dailyHeight).Radius(dailyRadius).FocusRing(false).Label(deckName(pg)).TextColor(white)
	hovered := card.Hovered() && !back
	a.deckGrip(card, pg)
	deep, rich, glow := dailyColors(art)
	fade := ui.Animation{Duration: 600 * time.Millisecond}
	deep, rich, glow = card.AnimateColor("deep", deep, fade), card.AnimateColor("rich", rich, fade), card.AnimateColor("glow", glow, fade)
	card.Background(deep)
	lift := card.AnimateTo("lift", b2f(hovered), ui.Snappy)
	if a.deckFlat(pg) {
		card.Shadow(0, 14+8*lift, 34+12*lift, -14, a.pal.cardShade)
	}
	sweep := float32(0)
	if hovered && !still {
		sweep = card.AnimateTo("sweep", 1, ui.Animation{Duration: 1100 * time.Millisecond, Ease: ui.EaseInOut})
	} else {
		card.Jump("sweep", 0)
	}
	blurred := backdrop(art)
	card.Draw(func(p *ui.Painter, r ui.Rect) {
		drawStyleBackdrop(p, r, blurred, deep, rich, glow, glows, r.X+r.W-glowAt, r.Y+r.H/2, lift, sweep)
	})
	card.Children(func() {
		ui.Row(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Padding(0, dailyPadX).Gap(28).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(6).Children(words)
			side(hovered && !still)
		})
	})
	if open != nil && a.deckClicked(card.Clicked()) {
		open()
	}
}

// deckLabel is the small line over a page's title: an icon and words.
func deckLabel(c *ui.Context, icon *ui.SVG, words string) {
	ui.Row(c).Gap(6).AlignItems(ui.Center).TextColor(white.Alpha(0.66)).Children(func() {
		ui.Icon(c, icon).Size(13, 13)
		ui.Text(c, words).FontSize(12).FontWeight(600).LetterSpacing(0.3).MaxLines(1)
	})
}

// deckTitle is a page's title, as large as the daily card's.
func deckTitle(c *ui.Context, words string) {
	ui.Text(c, words).FontSize(32).FontWeight(800).LetterSpacing(-0.8).LineHeight(1.15).MaxLines(1)
}

// deckCoverArt is a cover at the right of a page, large, or an icon in its
// place while there is none.
func (a *app) deckCoverArt(c *ui.Context, key, url string, icon *ui.SVG, hovered bool, size float32) {
	box := ui.Box(c.Key(key)).Size(size, size).Shrink(0).Radius(14).Shadow(0, 10, 24, -6, ui.RGBA(0, 0, 0, 0.45)).Margin(0, 12, 0, 0)
	pose := ui.Pose{}
	if hovered {
		pose = ui.Pose{Y: -3, Scale: 1.03, Rotate: -1.5}
	}
	box.AnimatePose(pose, ui.Animation{Spring: ui.SpringOptions{Duration: 600 * time.Millisecond, Bounce: 0.3}})
	box.Children(func() {
		if url == "" {
			ui.Box(c).Fill().Radius(14).Background(white.Alpha(0.12)).Center().Children(func() {
				ui.Icon(c, icon).Size(56, 56).TextColor(white.Alpha(0.8))
			})
			return
		}
		a.coverFill(c, url, 400, 14)
	})
}

// fmPlaying is the song the personal FM plays, if it plays.
func (a *app) fmPlaying() *netease.Track {
	radio := a.pl.Radio()
	if strings.HasPrefix(radio, "fm:") || isRoam(radio) {
		return a.pl.Current()
	}
	return nil
}

// fmSheet is the personal FM's page: what it plays, with its controls, or
// a button that starts it. The rest of it opens the FM's page.
func (a *app) fmSheet(c *ui.Context, back bool) {
	pl := a.pl
	tr := a.fmPlaying()
	cover := ""
	if tr != nil {
		cover = tr.Album.PicURL
	}
	wide := a.deckInner(c) >= 620
	a.deckSheet(c, deckFM, back, cover, func() { a.open(route{page: pageFM}) }, func() {
		a.fmModeRow(c)
		if tr == nil {
			deckTitle(c, "听见更多好音乐")
			ui.Text(c, "根据你的口味，一首接一首，不停推荐").FontSize(13).TextColor(white.Alpha(0.8)).MaxLines(1)
			ui.Row(c).Gap(10).Margin(12, 0, 0, 0).Children(func() {
				label := "开始收听"
				if strings.HasPrefix(a.radioStarting, "fm:") {
					label = "正在调频…"
				}
				if pillButton(c.Key("start"), iconPlay, label, white, ui.Hex("#1d1d1f")).Clicked() {
					a.playFM()
				}
			})
			return
		}
		deckTitle(c, tr.Name)
		a.artistLinks(c, tr, white.Alpha(0.8), white).FontSize(13).MaxLines(1)
		ui.Row(c).Gap(12).AlignItems(ui.Center).Margin(12, 0, 0, 0).Children(func() {
			icon, label := iconPlay, "播放"
			if pl.Playing() {
				icon, label = iconPause, "暂停"
			}
			if pillButton(c.Key("play"), icon, label, white, ui.Hex("#1d1d1f")).Clicked() {
				pl.TogglePlay()
			}
			if iconButton(c.Key("next"), iconNext, 20, white.Alpha(0.85), "下一首").Clicked() {
				pl.Next()
			}
			if a.profile != nil && iconButton(c.Key("trash"), iconTrash, 19, white.Alpha(0.85), "不喜欢").Clicked() {
				a.trashFM()
			}
			a.likeButton(c.Key("like"), tr, 20, white.Alpha(0.85))
		})
	}, func(hovered bool) {
		if wide {
			a.deckCoverArt(c, "fm-cover", cover, iconRadio, hovered, 150)
		} else if a.deckInner(c) >= deckSmallMin {
			a.deckCoverArt(c, "fm-cover", cover, iconRadio, hovered, deckSmallCover)
		}
	})
}

// radarSheet is 私人雷达's page: the day's title NetEase gave it and a
// button that plays it, the other radars beside it on wide pages, each
// opening its playlist. The rest of it opens 私人雷达's playlist.
func (a *app) radarSheet(c *ui.Context, back bool) {
	r := radars[0]
	pl := a.radar.lists[r.id]
	inner := a.deckInner(c)
	a.deckSheet(c, deckRadar, back, pl.Cover(), func() { a.open(route{page: pagePlaylist, id: r.id}) }, func() {
		deckLabel(c, iconRadar, "私人雷达 · 每天更新")
		title, sub := radarTitle(r, pl), r.desc
		if title == sub {
			sub = "根据你的听歌口味，每天更新"
		}
		deckTitle(c, title)
		ui.Text(c, sub).FontSize(13).TextColor(white.Alpha(0.8)).MaxLines(1)
		ui.Row(c).Gap(10).Margin(12, 0, 0, 0).Children(func() {
			if pillButton(c.Key("play"), iconPlay, "播放", white, ui.Hex("#1d1d1f")).Clicked() {
				a.loadPlaylist(r.id, true)
			}
		})
	}, func(hovered bool) {
		if inner >= 1040 {
			ui.Row(c.Key("more")).Gap(14).Shrink(0).Children(func() {
				for _, o := range radars[1:5] {
					opl := a.radar.lists[o.id]
					b := ui.ButtonBase(c.Key(o.id)).Column().Width(72).Gap(6).AlignItems(ui.Center).FocusRing(false).Label(o.name)
					up := b.AnimateTo("up", b2f(b.Hovered()), ui.Snappy)
					b.Children(func() {
						ui.Box(c).Size(72, 72).Radius(14).Shadow(0, 8, 18, -6, ui.RGBA(0, 0, 0, 0.4)).Translate(0, -3*up).Children(func() {
							if url := opl.Cover(); url != "" {
								a.coverFill(c, url, 200, 14)
							} else {
								ui.Box(c).Fill().Radius(14).Background(white.Alpha(0.12))
							}
						})
						ui.Text(c, strings.TrimSuffix(o.name, "雷达")+"雷达").FontSize(11.5).FontWeight(600).TextColor(white.Alpha(0.7 + 0.3*up)).SingleLine()
					})
					if b.Clicked() {
						a.open(route{page: pagePlaylist, id: o.id})
					}
				}
			})
		}
		switch {
		case inner >= 620:
			a.deckCoverArt(c, "radar-cover", pl.Cover(), iconRadar, hovered, 150)
		case inner >= deckSmallMin:
			a.deckCoverArt(c, "radar-cover", pl.Cover(), iconRadar, hovered, deckSmallCover)
		}
	})
}

// deckTabs are the calendar's bookmarks, at its right: one a page, the
// front page's out the farthest and lit, each flipping to its page when
// clicked, the pages before it flipping by in turn; and one under them
// that opens what the calendar holds.
func (a *app) deckTabs(c *ui.Context, pages []deckPage) {
	d := &a.deck
	ui.Column(c.Key("tabs")).Absolute().Top(deckTabTop).Right(0).Width(deckTabW + dailyRadius).Gap(deckTabGap).Children(func() {
		for _, pg := range pages {
			front := pg == d.front && !d.flipping || d.targeting && pg == d.target
			tab := ui.ButtonBase(c.Key(pg)).Height(deckTabH).Width(deckTabW + dailyRadius).FocusRing(false).Label("翻到" + deckName(pg))
			hovered := tab.Hovered()
			out := tab.AnimateTo("out", b2f(front)*4+b2f(hovered)*3, ui.Snappy)
			lit := tab.AnimateTo("lit", 0.55+0.45*b2f(front || hovered), ui.Animation{Duration: 160 * time.Millisecond})
			tint := tab.AnimateColor("tint", a.deckTint(pg), ui.Animation{Duration: 600 * time.Millisecond})
			tab.Children(func() {
				ui.Column(c).Absolute().Top(0).Bottom(0).Right(4-out).Width(deckTabW+dailyRadius-4).Radius(8).
					Background(tint.Mix(ui.RGB(0, 0, 0), 0.35*(1-lit))).Shadow(0, 4, 10, -4, ui.RGBA(0, 0, 0, 0.3)).
					Padding(0, 0, 0, dailyRadius).AlignItems(ui.Center).Justify(ui.Center).TextColor(white.Alpha(0.5 + 0.5*lit)).Children(func() {
					// Characters one over the other; Latin letters stay side by side.
					if short := deckShort(pg); short == "FM" {
						ui.Text(c, short).FontSize(10).FontWeight(800).SingleLine()
					} else {
						for _, ch := range short {
							ui.Text(c, string(ch)).FontSize(10.5).FontWeight(700).LineHeight(1.1)
						}
					}
				})
			})
			if tab.Clicked() && !(pg == d.front && !d.flipping) {
				d.target, d.targeting = pg, true
				a.deckLearned()
			}
		}
		edit := ui.ButtonBase(c.Key("deck-edit")).Height(26).Width(deckTabW + dailyRadius).FocusRing(false).Label("编辑台历")
		hovered := edit.Hovered()
		out := edit.AnimateTo("out", b2f(hovered || d.editing)*3, ui.Snappy)
		edit.Children(func() {
			ui.Box(c).Absolute().Top(0).Bottom(0).Right(4-out).Width(deckTabW+dailyRadius-4).Radius(8).
				Background(a.pal.faint.Alpha(0.6+0.4*b2f(hovered || d.editing))).Padding(0, 0, 0, dailyRadius).Center().Children(func() {
				ui.Icon(c, iconSettings).Size(12, 12).TextColor(a.pal.text)
			})
		})
		if edit.Clicked() {
			d.editing = !d.editing
		}
	})
}

// deckDesc says what a page shows.
func deckDesc(pg deckPage) string {
	return [...]string{"今天的 30 首推荐", "按你选的风格挑选", "无尽推荐，直接在首页收听", "每天更新的雷达歌单", "最近 7 天听得最多的歌"}[pg]
}

// drawer builds fn under what comes before it while open, growing into
// view as it opens and shrinking away as it closes, clipped.
func drawer(c *ui.Context, key string, open bool, fn func()) {
	box := ui.Column(c.Key(key)).Shrink(0)
	p := box.AnimateTo("open", b2f(open), ui.Animation{Duration: 240 * time.Millisecond, Ease: ui.EaseOut})
	if p < 0.001 {
		return
	}
	var inner ui.Element
	box.Children(func() {
		inner = ui.Column(c).Shrink(0)
		inner.Opacity(min(1, p*1.4)).Children(fn)
	})
	if p < 1 {
		box.ClipY().Height(p * inner.Bounds().H)
	}
}

// deckSort is a page of the calendar dragged to a new place in the
// drawer of what it holds: which, and how far it was dragged.
type deckSort struct {
	pg       deckPage
	dragging bool
	dy       float32
}

// The rows of the drawer of what the calendar holds.
const (
	deckRowH   = 50
	deckRowGap = 6
)

// deckManager is what the calendar holds: the pages it keeps, in their
// order, each dragged by its row to a new place, the others making room,
// each with a button that takes it out, the last one staying; and the
// pages taken out, each with a button that adds it back after the rest.
func (a *app) deckManager(c *ui.Context, ink paneInk) {
	kept := a.deckKept()
	var out []deckPage
	for _, pg := range a.settings.deckOrder() {
		if !slices.Contains(kept, pg) {
			out = append(out, pg)
		}
	}
	caption := func(words string) {
		ui.Text(c, words).FontSize(12).FontWeight(600).TextColor(ink.muted).Margin(10, 0, 4, 4)
	}
	words := func(pg deckPage) {
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, deckName(pg)).FontSize(13.5).FontWeight(600).TextColor(ink.text).SingleLine()
			ui.Text(c, deckDesc(pg)).FontSize(12).TextColor(ink.muted).SingleLine()
		})
	}
	const step = deckRowH + deckRowGap
	s := &a.deckSort
	if s.dragging && !slices.Contains(kept, s.pg) {
		s.dragging = false
	}
	// Where the dragged row would go, and so where each other row is.
	from, to := -1, -1
	if s.dragging {
		from = slices.Index(kept, s.pg)
		to = max(0, min(len(kept)-1, from+int(math.Round(float64(s.dy/step)))))
	}
	slot := func(i int) int {
		switch {
		case from < 0 || i == from:
			return i
		case from < i && i <= to:
			return i - 1
		case to <= i && i < from:
			return i + 1
		}
		return i
	}
	glide := ui.Animation{Spring: ui.SpringOptions{Duration: 320 * time.Millisecond, Bounce: 0.15}}
	row := func(i int, pg deckPage) {
		r := ui.Row(c.Key(pg)).Absolute().Top(0).Left(0).Right(0).Height(deckRowH).Gap(10).Padding(6, 8, 6, 8).Radius(14).
			Background(ink.hover).AlignItems(ui.Center).Label("拖动排序 " + deckName(pg)).Cursor(ui.CursorGrab)
		dragged := s.dragging && s.pg == pg
		if _, dy, ok := r.Dragged(); ok {
			if !s.dragging || s.pg != pg {
				*s = deckSort{pg: pg, dragging: true}
			}
			s.dy += dy
			dragged = true
		}
		if _, _, ok := r.DragEnded(); ok && s.dragging && s.pg == pg {
			if from >= 0 && to != from {
				moved := slices.Delete(slices.Clone(kept), from, from+1)
				moved = slices.Insert(moved, to, pg)
				a.deckReorder(moved)
			}
			s.dragging, dragged = false, false
		}
		pose := ui.Pose{Y: float32(slot(i)) * step}
		if dragged {
			lo, hi := -float32(i)*step-6, float32(len(kept)-1-i)*step+6
			pose = ui.Pose{Y: float32(i)*step + max(lo, min(hi, s.dy)), Scale: 1.02}
			lifted := ui.Hex("#ffffff")
			if a.pal.dark {
				lifted = ui.Hex("#323238")
			}
			r.Cursor(ui.CursorGrabbing).Shadow(0, 10, 24, -8, ui.RGBA(0, 0, 0, 0.35)).Background(lifted)
			r.AnimatePose(pose, ui.Animation{Spring: ui.SpringOptions{Duration: 70 * time.Millisecond}})
		} else {
			r.AnimatePose(pose, glide)
		}
		r.Children(func() {
			// The grip: two columns of dots.
			ui.Box(c).Size(14, 20).Shrink(0).Draw(func(p *ui.Painter, b ui.Rect) {
				for x := range 2 {
					for y := range 3 {
						p.Fill(ui.Rect{X: b.X + 3 + float32(x)*6, Y: b.Y + 3 + float32(y)*6, W: 2.5, H: 2.5}, ink.faint, 1.25)
					}
				}
			})
			words(pg)
			if len(kept) > 1 && iconButton(c.Key("remove"), iconTrash, 16, dangerColor, "删除 "+deckName(pg)).Clicked() {
				a.deckRemove(pg)
			}
		})
	}
	ui.Column(c).Gap(6).Children(func() {
		caption("已添加 · 拖动调整顺序，从上到下就是台历的页序")
		ui.Box(c.Key("kept")).Height(float32(len(kept))*step - deckRowGap).Children(func() {
			// The dragged row last, over the others.
			for i, pg := range kept {
				if !(s.dragging && s.pg == pg) {
					row(i, pg)
				}
			}
			if from >= 0 {
				row(from, s.pg)
			}
		})
		if len(out) > 0 {
			caption("可添加")
			for _, pg := range out {
				ui.Row(c.Key(pg)).Height(deckRowH).Gap(10).Padding(6, 8, 6, 8).Radius(14).Background(ink.hover).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(14, 20).Shrink(0).Center().Children(func() {
						ui.Icon(c, iconPlus).Size(12, 12).TextColor(ink.faint)
					})
					words(pg)
					if settingButton(c.Key("add"), ink, "添加", iconPlus, false, false).Label("添加 " + deckName(pg)).Clicked() {
						a.deckAdd(pg)
					}
				})
			}
		}
	})
}

// deckReorder keeps kept as the order of the pages the calendar keeps, those
// taken out where they were.
func (a *app) deckReorder(kept []deckPage) {
	order := a.settings.deckOrder()
	k := 0
	for i, pg := range order {
		if a.settings.deckOn(pg) {
			order[i] = kept[k]
			k++
		}
	}
	a.deckSetOrder(order)
}

// deckSummary names the pages the calendar keeps, in their order.
func (a *app) deckSummary() string {
	kept := a.deckKept()
	names := make([]string, len(kept))
	for i, pg := range kept {
		names[i] = deckShort(pg)
	}
	return fmt.Sprintf("%d 页：%s", len(kept), strings.Join(names, " · "))
}

// deckSettings is the calendar's group in the settings: a row naming what
// it holds, whose button opens a drawer under it to change that.
func (a *app) deckSettings(k *settingsKit) {
	c := k.c
	k.group("首页台历", func() {
		open := a.deckDrawer
		k.row("台历内容", a.deckSummary(), func() {
			label := "管理"
			if open {
				label = "收起"
			}
			if k.disclosure("deck-manage", label+"台历内容", open).Clicked() {
				a.deckDrawer = !open
			}
		})
		if k.probe {
			return
		}
		drawer(c, "deck-drawer", open, func() {
			ui.Box(c).Padding(0, 20, 16, 20).Children(func() { a.deckManager(c, k.ink) })
		})
	})
}

// deckEditor is the drawer under the calendar that the bookmark under its
// bookmarks opens: what the calendar holds, as the settings have it.
func (a *app) deckEditor(c *ui.Context) {
	ink := a.inkOn(false)
	fill, edge := a.cardColors()
	drawer(c, "deck-editor", a.deck.editing, func() {
		ui.Row(c).Justify(ui.End).Padding(14, deckTabW, 4, 0).Children(func() {
			ui.Column(c).Width(min(560, a.deckInner(c))).Radius(20).Background(fill).Border(0.5, edge).Padding(14, 16, 16, 16).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "编辑台历").FontSize(15).FontWeight(700).TextColor(ink.text)
					ui.Box(c).Grow(1)
					if settingButton(c.Key("done"), ink, "完成", nil, false, false).Clicked() {
						a.deck.editing = false
					}
				})
				a.deckManager(c, ink)
			})
		})
	})
}

// fmModeRow is the FM's label with its modes beside it, 推荐, 熟悉 and
// 探索, the one chosen lit, as the FM's page has them: choosing one while
// the FM plays tunes it anew; else the next start plays it. While a style
// roams in its place, none is lit.
func (a *app) fmModeRow(c *ui.Context) {
	chosen := a.radioStarting
	if chosen == "" {
		chosen = a.pl.Radio()
	}
	for i, m := range fmModes {
		if chosen == fmRadio(m.mode) {
			a.fmMode = i
		}
	}
	roaming := isRoam(chosen)
	ui.Row(c.Key("fm-modes")).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).TextColor(white.Alpha(0.66)).Children(func() {
			ui.Icon(c, iconRadio).Size(13, 13)
			ui.Text(c, "私人 FM").FontSize(12).FontWeight(600).LetterSpacing(0.3).SingleLine()
		})
		ui.Row(c).Height(24).Padding(2, 2, 2, 2).Gap(2).Radius(999).Background(white.Alpha(0.14)).AlignItems(ui.Center).Children(func() {
			for i, m := range fmModes {
				on := i == a.fmMode && !roaming
				b := ui.ButtonBase(c.Key(m.mode)).Height(20).PaddingX(10).Radius(999).Center().FocusRing(false).
					Role(ui.RoleRadio).Label("私人 FM " + m.label).Checked(on)
				fill := white.Alpha(0)
				switch {
				case on:
					fill = white
				case b.Hovered():
					fill = white.Alpha(0.14)
				}
				b.Background(b.AnimateColor("bg", fill, ui.Animation{Duration: 140 * time.Millisecond}))
				ink := white.Alpha(0.85)
				if on {
					ink = ui.Hex("#1d1d1f")
				}
				b.Children(func() {
					ui.Text(c, m.label).FontSize(11.5).FontWeight(650).TextColor(ink).SingleLine()
				})
				if b.Clicked() && !on {
					a.chooseFMMode(i)
				}
			}
		})
		if roaming {
			ui.Text(c, radioLabel(chosen)).FontSize(11.5).TextColor(white.Alpha(0.6)).SingleLine().Shrink(1).MinWidth(0)
		}
	})
}
