package app

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// The introduction of an artist, an album, a playlist or a podcast shows
// two lines in its page's header; a click on them shows all of it in a
// dialog, its paragraphs as they were written. An artist's has more than
// its page says, which the dialog asks NetEase for.

var fetchArtistIntro = netease.ArtistIntro

// introView is the dialog of an introduction.
type introView struct {
	open     bool
	title    string
	text     string
	artist   int64 // whose whole introduction to ask for, if an artist's
	loading  bool
	sections []netease.IntroSection
	err      string
	scroll   ui.ScrollState
}

// introLine is the two lines of an introduction in a page's header, which
// open the whole of it. artist is the artist's id, for theirs.
func (a *app) introLine(c *ui.Context, title, text string, artist int64) {
	p := a.pal
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	b := ui.ButtonBase(c.Key("intro-line")).Gap(8).AlignItems(ui.End).Justify(ui.Start).FocusRing(false).
		Label(title).Tooltip("查看全部").Margin(2, 0, 0, 0)
	b.Children(func() {
		ui.Text(c, strings.Join(strings.Fields(text), " ")).FontSize(12.5).TextColor(p.muted).MaxLines(2).LineHeight(1.45).
			Grow(1).Shrink(1).MinWidth(0)
		ink := p.muted
		if b.Hovered() {
			ink = p.text
		}
		ui.Row(c).Gap(2).AlignItems(ui.Center).Shrink(0).Children(func() {
			ui.Text(c, "全部").FontSize(12).FontWeight(600).TextColor(ink)
			ui.Icon(c, iconForward).Size(12, 12).TextColor(ink)
		})
	})
	if b.Clicked() {
		a.openIntro(title, text, artist)
	}
}

func (a *app) openIntro(title, text string, artist int64) {
	v := &a.more.intro
	*v = introView{open: true, title: title, text: text, artist: artist}
	if artist == 0 {
		return
	}
	if got, ok := a.more.introCache[artist]; ok {
		v.useArtistIntro(got)
		return
	}
	v.loading = true
	go func() {
		var got artistIntro
		var err error
		got.brief, got.sections, err = fetchArtistIntro(artist)
		a.post(func() {
			if v.artist != artist {
				return
			}
			v.loading = false
			if err != nil {
				v.err = "完整介绍加载失败"
				return
			}
			if a.more.introCache == nil {
				a.more.introCache = map[int64]artistIntro{}
			}
			a.more.introCache[artist] = got
			v.useArtistIntro(got)
		})
	}()
}

// artistIntro is an artist's whole introduction, as NetEase gave it.
type artistIntro struct {
	brief    string
	sections []netease.IntroSection
}

func (v *introView) useArtistIntro(got artistIntro) {
	if len([]rune(got.brief)) > len([]rune(v.text)) {
		v.text = got.brief
	}
	v.sections = got.sections
}

// introDialog shows a whole introduction.
func (a *app) introDialog(c *ui.Context) {
	v := &a.more.intro
	a.dialog(c, "intro", 0.35, &v.open, func(panel ui.Element) {
		a.pane(panel, 26, false)
		ink := a.inkOn(false)
		_, h := c.Size()
		panel.Width(560).MaxHeight(max(240, h-120)).TextColor(ink.text)
		dialogHeader(c, ink, v.title, "", func() { v.open = false })
		ui.Scroll(c.Key("intro-scroll")).TrackScroll(&v.scroll).Shrink(1).MinHeight(0).Padding(0, 28, 26, 28).Children(func() {
			ui.Column(c).Gap(18).Children(func() {
				introText(c, v.text, ink)
				for i, s := range v.sections {
					ui.Column(c.Key(i)).Gap(8).Children(func() {
						if s.Title != "" {
							ui.Text(c, s.Title).FontSize(15).FontWeight(700)
						}
						introText(c, s.Text, ink)
					})
				}
				switch {
				case v.loading:
					ui.Text(c, "正在加载完整介绍…").FontSize(13).TextColor(ink.muted)
				case v.err != "":
					ui.Text(c, v.err).FontSize(13).TextColor(ink.muted)
				}
			})
		})
	})
}

// introText is paragraphs, as they were written.
func introText(c *ui.Context, text string, ink paneInk) {
	ui.Column(c).Gap(8).Children(func() {
		for i, para := range strings.Split(text, "\n") {
			if para = strings.TrimSpace(para); para != "" {
				ui.Text(c.Key(i), para).FontSize(14).LineHeight(1.65).TextColor(ink.text.Alpha(0.86)).Selectable()
			}
		}
	})
}
