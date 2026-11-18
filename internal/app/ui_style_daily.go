package app

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// The day's songs in the styles the listener picked, the official client's
// 风格推荐: a page of the home page's calendar under the daily card, opening a page of the
// songs, whose styles a dialog changes.

// styleState is the day's style songs, and the dialog that picks their
// styles.
type styleState struct {
	songs   netease.StyleSongs
	loaded  bool
	loading bool
	err     string

	// The dialog: the categories of styles, the one shown, and the tags
	// picked, all of one category.
	open        bool
	cats        []netease.StyleCategory
	catsLoading bool
	catsErr     string
	tab         int64
	cat         int64
	picked      []int64
	saving      bool
	saveErr     string
}

var (
	dailyStyleSongs = netease.DailyStyleSongs
	styleCategories = netease.StyleCategories
	saveStyleTags   = netease.SaveStyleTags
)

// loadStyle loads the day's style songs for whoever is signed in.
func (a *app) loadStyle() {
	if a.profile == nil {
		a.style = styleState{}
		return
	}
	if a.style.loading {
		return
	}
	a.style.loading, a.style.err = true, ""
	gen := a.accountGen
	go func() {
		songs, err := dailyStyleSongs()
		a.post(func() {
			if gen != a.accountGen {
				return
			}
			s := &a.style
			s.loading = false
			if err != nil {
				s.err = "风格推荐加载失败"
				return
			}
			s.songs, s.loaded = songs, true
		})
	}()
}

// styleNames names the styles picked, as "R&B · 欧美流行".
func styleNames(picked netease.StyleCategory) string {
	names := make([]string, 0, len(picked.Tags))
	for _, t := range picked.Tags {
		names = append(names, t.Name)
	}
	return strings.Join(names, " · ")
}

// styleCover is the cover of the style songs: their first song's.
func styleCover(songs []netease.Track) string {
	for _, tr := range songs {
		if tr.Album.PicURL != "" {
			return tr.Album.PicURL
		}
	}
	return ""
}

// styleCardShown reports whether the home page has the style songs' page.
func (a *app) styleCardShown() bool {
	return a.profile != nil && a.style.loaded && len(a.style.songs.Songs) > 0
}

// recommendedGrid is the day's playlists picked for the listener, two rows
// of them.
func (a *app) recommendedGrid(c *ui.Context) {
	cols := a.columns(c, 168)
	// 私人雷达 shows in the radars' section, under this one.
	recs := slices.DeleteFunc(slices.Clone(a.recPlaylists), func(pl netease.Playlist) bool { return isRadar(pl.ID) })
	recs = recs[:min(len(recs), cols*2)]
	ui.Grid(c).Columns(cols).GapX(20).GapY(28).Children(func() {
		for i := range recs {
			a.playlistCard(c.Key(recs[i].ID), &recs[i])
		}
	})
}

// gridCell is how wide a card is in a grid of cols on the page.
func (a *app) gridCell(c *ui.Context, cols int) float32 {
	w, _ := c.Size()
	return (w - a.sideWidth(w) - 2*pagePad - float32(cols-1)*20) / float32(cols)
}

// The style page's measures, beside the daily card's.
const (
	styleFan      = 150 // its first cover, the others a little smaller
	styleFanWidth = 280 // the room they fan out in
)

// styleCard is the style songs' page of the home page's calendar, laid out
// as the daily card is, in the colors of their first cover: the styles, a
// button that plays the songs and one that changes the styles; on wide
// pages the first songs, each of which plays from there; at the right
// their first three covers, fanned, spreading under the pointer. The rest
// of it opens their page. Lying under another page (back), it keeps still
// and takes no pointer.
func (a *app) styleCard(c *ui.Context, back bool) {
	s := &a.style
	songs := s.songs.Songs
	cover := styleCover(songs)
	w, _ := c.Size()
	inner := w - a.sideWidth(w) - 2*pagePad
	still := c.Preferences().ReduceMotion
	art := a.art.get(netease.Sized(cover, 600), true)
	card := ui.ButtonBase(c.Key("style")).Height(dailyHeight).Radius(dailyRadius).FocusRing(false).Label("风格推荐").TextColor(white)
	hovered := card.Hovered() && !back
	a.deckGrip(card, deckStyle)
	deep, rich, glow := dailyColors(art)
	fade := ui.Animation{Duration: 600 * time.Millisecond}
	deep, rich, glow = card.AnimateColor("deep", deep, fade), card.AnimateColor("rich", rich, fade), card.AnimateColor("glow", glow, fade)
	card.Background(deep)
	lift := card.AnimateTo("lift", b2f(hovered), ui.Snappy)
	if a.deckFlat(deckStyle) {
		card.Shadow(0, 14+8*lift, 34+12*lift, -14, a.pal.cardShade)
	}
	sweep := float32(0)
	if hovered && !still {
		sweep = card.AnimateTo("sweep", 1, ui.Animation{Duration: 1100 * time.Millisecond, Ease: ui.EaseInOut})
	} else {
		card.Jump("sweep", 0)
	}
	blurred := backdrop(art)
	ours := a.queueIs(songs)
	playing := ours && a.pl.Playing()
	showFan, showSongs := inner >= 620, inner >= 1040

	var artAt ui.Rect
	card.Draw(func(p *ui.Painter, r ui.Rect) {
		// The glow is behind the covers.
		cx := r.X + r.W - dailyPadX - 12 - deckSmallCover/2
		if showFan {
			cx = r.X + r.W - dailyPadX - styleFanWidth/2
		}
		drawStyleBackdrop(p, r, blurred, deep, rich, glow, true, cx, r.Y+r.H/2, lift, sweep)
	})
	card.Children(func() {
		ui.Row(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Padding(0, dailyPadX).Gap(28).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(6).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).TextColor(white.Alpha(0.66)).Children(func() {
					ui.Icon(c, iconWand).Size(13, 13)
					ui.Textf(c, "风格推荐 · 今日 %d 首", len(songs)).FontSize(12).FontWeight(600).LetterSpacing(0.3).MaxLines(1)
				})
				title := styleNames(s.songs.Picked)
				if title == "" {
					title = "按你选的风格挑选"
				}
				ui.Text(c, title).FontSize(32).FontWeight(800).LetterSpacing(-0.8).LineHeight(1.15).MaxLines(1)
				ui.Text(c, dailyArtists(songs)).FontSize(13).TextColor(white.Alpha(0.8)).MaxLines(1)
				ui.Row(c).Gap(10).Margin(12, 0, 0, 0).Children(func() {
					a.dailyPlayButton(c.Key("play"), songs, ours, playing)
					if pillButton(c.Key("change"), iconWand, "换风格", white.Alpha(0.16), white).Clicked() {
						a.openStylePicker()
					}
				})
			})
			if showSongs {
				a.dailyPicks(c, songs, ours)
			}
			switch {
			case showFan:
				artAt = a.styleFanned(c, songs, hovered && !still, func() { a.playList(songs, false) })
			case inner >= deckSmallMin:
				a.deckCoverArt(c, "small", cover, iconWand, hovered && !still, deckSmallCover)
			}
		})
	})
	if a.deckClicked(card.Clicked()) {
		a.flyFrom(c, cover, artAt, 14)
		a.open(route{page: pageStyle})
	}
}

// styleFanned is the style songs' first three covers, the first in front,
// the next two behind it at either side; fanned, they spread and the first
// shows a button that plays. It returns where the first is.
func (a *app) styleFanned(c *ui.Context, songs []netease.Track, fanned bool, play func()) ui.Rect {
	spring := ui.Animation{Spring: ui.SpringOptions{Duration: 600 * time.Millisecond, Bounce: 0.3}}
	var front ui.Rect
	ui.Box(c.Key("fan")).Size(styleFanWidth, styleFan).Shrink(0).Children(func() {
		// The covers behind, then the first.
		for _, i := range []int{2, 1, 0} {
			if i >= len(songs) {
				continue
			}
			side := float32(0)
			switch i {
			case 1:
				side = -1
			case 2:
				side = 1
			}
			size := float32(styleFan)
			if i > 0 {
				size -= 22
			}
			pose := ui.Pose{X: 44 * side, Rotate: 7 * side, Y: 4}
			if fanned {
				pose = ui.Pose{X: 64 * side, Rotate: 12 * side, Y: 8}
			}
			if i == 0 {
				pose = ui.Pose{}
				if fanned {
					pose = ui.Pose{Y: -4, Scale: 1.03}
				}
			}
			cov := ui.Box(c.Key(i)).Absolute().Left((styleFanWidth-size)/2).Top((styleFan-size)/2).Size(size, size).Radius(14).
				Shadow(0, 10, 22, -6, ui.RGBA(0, 0, 0, 0.45))
			if i > 0 {
				cov.Opacity(0.88)
			}
			cov.AnimatePose(pose, spring)
			if i == 0 {
				front = cov.Bounds()
			}
			cov.Children(func() {
				a.coverFill(c, songs[i].Album.PicURL, 240, 14)
				if i == 0 {
					a.coverPlayButton(c, fanned, play)
				}
			})
		}
	})
	return front
}

// drawStyleBackdrop paints the style card in r, as the daily card is
// painted: a gradient of the cover's colors with the cover blurred faintly
// through it, deeper at the right, where its songs are, and a glow behind
// the cover, if glows, whose middle is at cx, cy, brighter and wider as lit
// goes to 1; and, as sweep goes from 0 to 1, a sheen crossing it.
func drawStyleBackdrop(p *ui.Painter, r ui.Rect, blurred *ui.Bitmap, deep, rich, glow ui.Color, glows bool, cx, cy, lit, sweep float32) {
	const angle = 90
	p.Clip(r, dailyRadius, func() {
		p.FillGradient(r, ui.LinearGradient{Angle: angle + 10, Oklab: true, Stops: []ui.GradientStop{
			{Color: rich, At: 0}, {Color: deep, At: 0.8},
		}}, 0)
		if blurred != nil {
			p.ImageRegion(blurred, ui.Rect{W: 48, H: 48}, coverRect(r, 48, 48), ui.ImageOptions{Tint: white.Alpha(0.24)})
		}
		p.FillGradient(r, ui.LinearGradient{Angle: angle, Stops: []ui.GradientStop{
			{Color: deep.Alpha(0), At: 0.25}, {Color: deep.Alpha(0.8), At: 1},
		}}, 0)
		if glows {
			g := 50 + 20*lit
			p.Shadow(ui.Rect{X: cx - g, Y: cy - g, W: 2 * g, H: 2 * g}, g, 0, 0, 120+40*lit, 30, glow.Alpha(0.22+0.18*lit))
		}
		// Light from above, as the pointer comes.
		p.Shadow(ui.Rect{X: r.X + r.W*0.3, Y: r.Y - 80, W: r.W * 0.4, H: 40}, 20, 0, 0, 120, 0, white.Alpha(0.1*lit))
		if sweep > 0 && sweep < 1 {
			at := -0.25 + 1.5*sweep
			p.FillGradient(r, ui.LinearGradient{Angle: 110, Stops: []ui.GradientStop{
				{Color: white.Alpha(0), At: clamp01(at - 0.12)},
				{Color: white.Alpha(0.13 * (1 - sweep*0.5)), At: clamp01(at)},
				{Color: white.Alpha(0), At: clamp01(at + 0.12)},
			}}, 0)
		}
	})
	p.Stroke(r, white.Alpha(0.1+0.1*lit), dailyRadius, 1)
}

// stylePage is the day's songs in the styles picked, with a button that
// changes those.
func (a *app) stylePage(c *ui.Context) {
	s := &a.style
	if a.profile == nil {
		a.centered(c, "登录后才能使用风格推荐", nil)
		return
	}
	loading := !s.loaded
	if loading && !s.loading {
		err := s.err
		if err == "" {
			err = "暂时没有风格推荐"
		}
		a.centered(c, err, a.loadStyle)
		return
	}
	songs := s.songs.Songs
	sh := a.pageShimmer(c)
	header := func(c *ui.Context) {
		p := a.pal
		ui.Row(c).Gap(28).AlignItems(ui.End).Padding(0, 0, 26, 0).Children(func() {
			a.headCover(c, styleCover(songs))
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(8).Children(func() {
				ui.Text(c, "每日推荐").FontSize(12).FontWeight(600).TextColor(p.accent)
				ui.Text(c, "风格推荐").FontSize(30).FontWeight(800).LetterSpacing(-0.6)
				if loading {
					sh.text(c, 13, 1.3).Width(240)
					return
				}
				ui.Text(c, fmt.Sprintf("%d 首 · 按你选的风格挑选，每天更新", len(songs))).FontSize(13).TextColor(p.muted)
				ui.Row(c).Gap(10).Margin(8, 0, 0, 0).Children(func() {
					if pillButton(c.Key("play"), iconPlay, "播放全部", p.accent, p.onAccent).Clicked() {
						a.playList(songs, false)
					}
					if pillButton(c.Key("shuffle"), iconShuffle, "随机播放", p.hover.Over(p.bg), p.text).Clicked() {
						a.playList(songs, true)
					}
					label := styleNames(s.songs.Picked)
					if label == "" {
						label = "选择风格"
					}
					if pillButton(c.Key("styles"), iconWand, label, p.hover.Over(p.bg), p.text).Label("选择风格：" + label).Clicked() {
						a.openStylePicker()
					}
					a.batchEntry(c)
				})
			})
		})
	}
	if loading {
		a.trackSkeleton(c, sh, true, header)
		return
	}
	a.trackRows(c, songs, true, header)
}

// openStylePicker opens the dialog on the styles picked, loading what can
// be picked if it has not.
func (a *app) openStylePicker() {
	s := &a.style
	s.open, s.saveErr = true, ""
	s.cat = s.songs.Picked.ID
	s.picked = s.picked[:0]
	for _, t := range s.songs.Picked.Tags {
		s.picked = append(s.picked, t.ID)
	}
	s.tab = s.cat
	if len(s.cats) > 0 || s.catsLoading {
		a.styleTabFallback()
		return
	}
	s.catsLoading, s.catsErr = true, ""
	gen := a.accountGen
	go func() {
		cats, err := styleCategories()
		a.post(func() {
			if gen != a.accountGen {
				return
			}
			s.catsLoading = false
			if err != nil || len(cats) == 0 {
				s.catsErr = "风格列表加载失败"
				return
			}
			s.cats = cats
			a.styleTabFallback()
		})
	}()
}

// styleTabFallback shows the first category if the one picked in isn't
// one there is.
func (a *app) styleTabFallback() {
	s := &a.style
	if len(s.cats) > 0 && !slices.ContainsFunc(s.cats, func(cat netease.StyleCategory) bool { return cat.ID == s.tab }) {
		s.tab = s.cats[0].ID
	}
}

// pickStyle picks tag of category cat, or unpicks it. Tags are picked in
// one category: one of another starts the picking again.
func (a *app) pickStyle(cat, tag int64) {
	s := &a.style
	if s.cat != cat {
		s.cat, s.picked = cat, []int64{tag}
		return
	}
	if i := slices.Index(s.picked, tag); i >= 0 {
		s.picked = slices.Delete(s.picked, i, i+1)
		return
	}
	s.picked = append(s.picked, tag)
}

// saveStyles keeps the styles picked with the account, and loads the songs
// in them.
func (a *app) saveStyles() {
	s := &a.style
	if s.saving || len(s.picked) == 0 {
		return
	}
	s.saving, s.saveErr = true, ""
	cat, tags := s.cat, slices.Clone(s.picked)
	gen := a.accountGen
	go func() {
		err := saveStyleTags(cat, tags)
		var songs netease.StyleSongs
		if err == nil {
			songs, err = dailyStyleSongs()
		}
		a.post(func() {
			if gen != a.accountGen {
				return
			}
			s.saving = false
			if err != nil {
				s.saveErr = "保存失败，请稍后再试"
				return
			}
			s.songs, s.loaded, s.open = songs, true, false
		})
	}()
}

// stylePickerDialog picks the styles of the style songs: a tab for each
// category, its tags under it.
func (a *app) stylePickerDialog(c *ui.Context) {
	s := &a.style
	p := a.pal
	a.dialog(c, "style-picker", 0.35, &s.open, func(panel ui.Element) {
		a.pane(panel, 26, false)
		ink := a.inkOn(false)
		panel.Width(560).TextColor(ink.text)
		dialogHeader(c, ink, "选择风格", "同一类里可以多选，和网易云官方客户端同步", func() { s.open = false })
		ui.Column(c).Padding(0, 28).Gap(16).MinHeight(250).Children(func() {
			switch {
			case s.catsLoading:
				ui.Text(c, "正在加载…").FontSize(13).TextColor(ink.muted)
				return
			case s.catsErr != "":
				ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
					ui.Text(c, s.catsErr).FontSize(13).TextColor(ink.muted)
					if settingButton(c.Key("retry"), ink, "重试", nil, false, false).Clicked() {
						a.openStylePicker()
					}
				})
				return
			}
			ui.Row(c.Key("tabs")).Gap(6).Children(func() {
				for _, cat := range s.cats {
					on := cat.ID == s.tab
					b := ui.ButtonBase(c.Key(cat.ID)).Height(32).PaddingX(14).Radius(999).Center().
						FocusRing(false).Label("风格分类：" + cat.Name).Selected(on)
					bg, fg := ui.Transparent, ink.muted
					switch {
					case on:
						bg, fg = ink.hover, ink.text
					case b.Hovered():
						fg = ink.text
					}
					b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond}))
					b.Children(func() {
						label := cat.Name
						if n := a.pickedIn(cat.ID); n > 0 {
							label = fmt.Sprintf("%s %d", cat.Name, n)
						}
						ui.Text(c, label).FontSize(14).FontWeight(650).TextColor(fg).SingleLine()
					})
					if b.Clicked() {
						s.tab = cat.ID
					}
				}
			})
			for _, cat := range s.cats {
				if cat.ID != s.tab {
					continue
				}
				ui.Grid(c.Key(fmt.Sprint("tags-", cat.ID))).Columns(5).Gap(10).Children(func() {
					for _, t := range cat.Tags {
						on := s.cat == cat.ID && slices.Contains(s.picked, t.ID)
						b := ui.ButtonBase(c.Key(t.ID)).Height(34).Radius(999).Center().FocusRing(false).
							Label("风格：" + t.Name).Selected(on)
						bg, fg := ink.hover, ink.text
						switch {
						case on:
							bg, fg = p.accent, p.onAccent
						case b.Hovered():
							bg = ink.hover.Mix(ink.text, 0.08)
						}
						b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond}))
						b.Children(func() {
							ui.Text(c, t.Name).FontSize(13).FontWeight(600).TextColor(fg).SingleLine()
						})
						if b.Clicked() {
							a.pickStyle(cat.ID, t.ID)
						}
					}
				})
			}
			if s.saveErr != "" {
				ui.Text(c, s.saveErr).FontSize(12.5).TextColor(dangerColor)
			}
		})
		dialogFooter(c, func() {
			if settingButton(c.Key("cancel"), ink, "取消", nil, false, false).Clicked() {
				s.open = false
			}
			label := "确认"
			if s.saving {
				label = "正在保存…"
			}
			if primaryButton(c.Key("ok"), label, nil, p, len(s.picked) == 0 || s.saving || len(s.cats) == 0).Clicked() {
				a.saveStyles()
			}
		})
	})
}

// pickedIn says how many tags of category cat are picked.
func (a *app) pickedIn(cat int64) int {
	if a.style.cat != cat {
		return 0
	}
	return len(a.style.picked)
}
