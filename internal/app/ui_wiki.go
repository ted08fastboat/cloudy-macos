package app

import (
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// The 百科 pane of Now Playing: what NetEase's song wiki says of the song
// playing, as its app shows it, with the songs and playlists like it.

var fetchSongWiki = netease.SongWikiOf

// wikiState is the wiki of one song, as it loads.
type wikiState struct {
	track   int64
	wiki    *netease.SongWiki
	loading bool
	err     string
	scroll  ui.ScrollState
	opening int64 // the similar song being looked up to play

	introOpen  bool           // 创作信息 shown whole
	artistOpen map[int64]bool // the artists whose introductions show whole
}

// wikiShown reports whether tr has a wiki: a song of NetEase's.
func wikiShown(tr *netease.Track) bool {
	return tr != nil && tr.ID > 0 && tr.LocalPath == "" && tr.ProgramID == 0
}

// wikiFor returns the wiki of tr, loading it the first time.
func (a *app) wikiFor(tr *netease.Track) *wikiState {
	ws := &a.more.wiki
	if ws.track != tr.ID {
		*ws = wikiState{track: tr.ID}
		a.loadWiki()
	}
	return ws
}

func (a *app) loadWiki() {
	ws := &a.more.wiki
	if ws.loading {
		return
	}
	id := ws.track
	ws.loading, ws.err = true, ""
	go func() {
		w, err := fetchSongWiki(id)
		a.post(func() {
			if ws.track != id {
				return
			}
			ws.loading = false
			if err != nil {
				ws.err = "百科加载失败"
				return
			}
			ws.wiki = w
		})
	}()
}

// playSimilar plays a song the wiki says is like the one playing, once
// NetEase has said what it is.
func (a *app) playSimilar(s netease.WikiSong) {
	ws := &a.more.wiki
	if ws.opening != 0 {
		return
	}
	ws.opening = s.ID
	go func() {
		tracks, err := trackDetails([]int64{s.ID})
		a.post(func() {
			ws.opening = 0
			if err != nil || len(tracks) == 0 {
				a.say("wiki", "没能播放「"+s.Name+"」")
				return
			}
			a.pl.PlayNext(tracks[:1])
			a.pl.Next()
		})
	}()
}

func (a *app) wikiPane(c *ui.Context, tr *netease.Track) {
	if !wikiShown(tr) {
		wikiNote(c, "本地歌曲和节目没有百科")
		return
	}
	ws := a.wikiFor(tr)
	w := ws.wiki
	if w == nil {
		if ws.err != "" {
			ui.Column(c).Grow(1).Center().Gap(14).Children(func() {
				ui.Text(c, ws.err).FontSize(15).TextColor(white.Alpha(0.5))
				if pillButton(c.Key("retry"), iconInfo, "重试", white.Alpha(0.14), white).Clicked() {
					a.loadWiki()
				}
			})
			return
		}
		ui.Column(c).Grow(1).Padding(4, 8, 0, 0).Gap(10).Children(func() {
			s := npShimmer(c)
			s.text(c, 13, 1.3).Width(72).Margin(18, 12, 6, 12)
			for i := 0; i < 5; i++ {
				s.text(c, 14, 1.4).Width(float32(220+40*(i%3))).Margin(0, 12, 0, 12)
			}
		})
		return
	}
	if w.Empty() {
		wikiNote(c, "这首歌还没有百科")
		return
	}
	scroll := ui.Scroll(c.Key("wiki")).Grow(1).TrackScroll(&ws.scroll).Padding(4, 8, 48, 0)
	cols, cardW := wikiGrid(a.more.npPaneW)
	scroll.Children(func() {
		ui.Column(c).Gap(2).Children(func() {
			if len(w.Memory) > 0 {
				commentHeading(c, "回忆坐标")
				ui.Row(c.Key("memory")).Wrap().Gap(8).Padding(0, 12).Children(func() {
					for i, l := range w.Memory {
						wikiTile(c.Key(i), l, cardW*float32(min(2, cols))/2)
					}
				})
			}
			if w.Album != nil || len(w.Facts) > 0 || len(w.Shows) > 0 || w.Sheets > 0 {
				commentHeading(c, "歌曲详情")
				if w.Album != nil {
					a.wikiAlbum(c.Key("album"), *w.Album)
				}
				for _, f := range w.Facts {
					wikiFact(c, f.Label, f.Values)
				}
				if len(w.Shows) > 0 {
					wikiFact(c, "影视节目", w.Shows)
				}
			}
			if w.Intro != "" || len(w.Credits) > 0 {
				commentHeading(c, "创作信息")
				if w.Intro != "" {
					wikiMore(c.Key("intro"), w.Intro, 4, &ws.introOpen)
				}
				ui.Column(c.Key("credits")).Margin(4, 12, 0, 12).Padding(4, 14).Radius(14).Background(white.Alpha(0.05)).Children(func() {
					for i, cr := range w.Credits {
						a.wikiCredit(c.Key(i), cr, i > 0)
					}
				})
			}
			if len(w.Awards) > 0 || len(w.AlbumAwards) > 0 {
				commentHeading(c, "获奖成就")
				for i, l := range w.Awards {
					wikiLine(c.Key(i), l.Title, l.Detail)
				}
				for i, l := range w.AlbumAwards {
					wikiLine(c.Key(100+i), l.Title, "专辑 · "+l.Detail)
				}
			}
			if w.Review != nil {
				commentHeading(c, "乐评")
				ui.Column(c).Padding(12).Margin(0, 12, 0, 12).Gap(8).Radius(14).Background(white.Alpha(0.06)).Children(func() {
					ui.Text(c, w.Review.Text).FontSize(14).LineHeight(1.6).TextColor(white.Alpha(0.86))
					if w.Review.By != "" {
						ui.Text(c, "—— "+w.Review.By).FontSize(12).TextColor(white.Alpha(0.5)).AlignSelf(ui.End)
					}
				})
			}
			if len(w.Artists) > 0 {
				commentHeading(c, "关于艺人")
				for _, ar := range w.Artists {
					a.wikiArtist(c.Key(ar.ID), ar, ws)
				}
			}
			if len(w.Similar) > 0 {
				commentHeading(c, "相似歌曲")
				ui.Row(c.Key("similar")).Wrap().Gap(10).Padding(0, 12).Children(func() {
					for _, s := range w.Similar {
						a.wikiSong(c.Key(s.ID), s, ws.opening == s.ID, cardW)
					}
				})
			}
			if len(w.Versions) > 0 {
				commentHeading(c, "更多版本")
				ui.Row(c.Key("versions")).Wrap().Gap(8).Padding(0, 12).Children(func() {
					for _, s := range w.Versions {
						a.wikiVersion(c.Key(s.ID), s, ws.opening == s.ID, cardW*float32(min(2, cols))/2-4)
					}
				})
			}
			if len(w.Playlists) > 0 {
				commentHeading(c, "相关歌单")
				ui.Row(c.Key("playlists")).Wrap().Gap(10).Padding(0, 12).Children(func() {
					for _, p := range w.Playlists {
						a.wikiPlaylist(c.Key(p.ID), p, cardW)
					}
				})
			}
		})
	})
}

// wikiGrid is how many cards go in a row of the wiki pane width wide, and
// how wide each is: those of songs and playlists, a cover and two lines.
func wikiGrid(width float32) (int, float32) {
	const gap, pad = 10, 2*12 + 8
	inner := max(200, width-pad)
	cols := max(2, min(6, int((inner+gap)/(130+gap))))
	return cols, (inner - gap*float32(cols-1)) / float32(cols)
}

// wikiTile is a moment of the listener's history, as a small card: what it
// is over what it says.
func wikiTile(c *ui.Context, l netease.WikiLine, width float32) {
	ui.Column(c).Width(width-4).Padding(12, 14).Gap(4).Radius(14).Background(white.Alpha(0.07)).Children(func() {
		ui.Text(c, l.Title).FontSize(12).TextColor(white.Alpha(0.55))
		ui.Text(c, l.Detail).FontSize(14).FontWeight(600).TextColor(white.Alpha(0.9)).MaxLines(2)
	})
}

// wikiMore is text cut to lines lines, with 展开 to show it all.
func wikiMore(c *ui.Context, text string, lines int, open *bool) {
	ui.Column(c).Padding(4, 12, 6, 12).Gap(4).Children(func() {
		t := ui.Text(c, text).FontSize(13.5).LineHeight(1.6).TextColor(white.Alpha(0.8))
		if !*open {
			t.MaxLines(lines)
		}
		if utf8.RuneCountInString(text) > lines*28 {
			label := "展开"
			if *open {
				label = "收起"
			}
			b := ui.ButtonBase(c.Key("more")).FocusRing(false).Label(label).AlignSelf(ui.End)
			b.Children(func() {
				ui.Text(c, label).FontSize(12).FontWeight(600).TextColor(white.Alpha(0.6))
			})
			if b.Clicked() {
				*open = !*open
			}
		}
	})
}

// wikiCredit is a row of the credits table: a part in making the song,
// in a column of its own, and everyone who had it after it, wrapping under
// one another rather than under the part, those with a page as links.
func (a *app) wikiCredit(c *ui.Context, cr netease.WikiCredit, rule bool) {
	row := ui.Row(c).Padding(9, 0).Gap(14).AlignItems(ui.Start)
	if rule {
		row.BorderWidth(1, 0, 0, 0).BorderColor(white.Alpha(0.07))
	}
	row.Children(func() {
		ui.Text(c, cr.Role).FontSize(12.5).LineHeight(1.5).TextColor(white.Alpha(0.5)).Width(76).Shrink(0).SingleLine()
		ui.RichText(c.Key("people")).Grow(1).Shrink(1).MinWidth(0).FontSize(13).LineHeight(1.5).FontWeight(600).TextColor(white.Alpha(0.88)).Children(func() {
			for i, p := range cr.People {
				if i > 0 {
					ui.Text(c.Key(-1-i), "  ·  ").TextColor(white.Alpha(0.35))
				}
				if p.ID > 0 {
					a.link(c.Key(i), p.Name, white.Alpha(0.88), white, route{page: pageArtist, id: p.ID})
				} else {
					ui.Text(c.Key(i), p.Name)
				}
			}
		})
	})
}

// wikiAlbum is the album the song is on, as a row that opens it.
func (a *app) wikiAlbum(c *ui.Context, al netease.WikiSong) {
	row := ui.ButtonBase(c).Padding(8, 12).Margin(0, 0, 4, 0).Gap(12).Radius(14).AlignItems(ui.Center).FocusRing(false).Label("打开专辑 " + al.Name)
	if row.Hovered() {
		row.Background(white.Alpha(0.06))
	}
	row.Children(func() {
		a.cover(c, al.CoverURL, 96, 44, 8).Shrink(0)
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, "专辑").FontSize(12).TextColor(white.Alpha(0.5))
			ui.Text(c, al.Name).FontSize(14).FontWeight(600).TextColor(white.Alpha(0.9)).SingleLine()
		})
		ui.Icon(c, iconForward).Size(14, 14).TextColor(white.Alpha(0.45))
	})
	if row.Clicked() {
		a.openPage(route{page: pageAlbum, id: al.ID})
	}
}

// wikiArtist introduces an artist of the song: who, in what parts, and
// what their page says, cut short until 展开.
func (a *app) wikiArtist(c *ui.Context, ar netease.WikiArtist, ws *wikiState) {
	ui.Column(c).Margin(0, 12, 8, 12).Padding(12).Gap(10).Radius(14).Background(white.Alpha(0.06)).Children(func() {
		head := ui.ButtonBase(c.Key("head")).Gap(12).AlignItems(ui.Center).FocusRing(false).Label("打开艺人 " + ar.Name)
		head.Children(func() {
			a.avatar(c, ar.AvatarURL, 44, white.Alpha(0.1))
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
				name := ui.Text(c, ar.Name).FontSize(15).FontWeight(700).TextColor(white.Alpha(0.92)).SingleLine()
				if head.Hovered() {
					name.TextColor(white)
				}
				if len(ar.Roles) > 0 {
					ui.Text(c, strings.Join(ar.Roles, " · ")).FontSize(12).TextColor(white.Alpha(0.55)).SingleLine()
				}
			})
			ui.Icon(c, iconForward).Size(14, 14).TextColor(white.Alpha(0.45))
		})
		if head.Clicked() && ar.ID > 0 {
			a.openArtist(ar.ID)
		}
		if ar.Brief != "" {
			if ws.artistOpen == nil {
				ws.artistOpen = map[int64]bool{}
			}
			open := ws.artistOpen[ar.ID]
			wikiMore(c.Key("brief"), ar.Brief, 3, &open)
			ws.artistOpen[ar.ID] = open
		}
	})
}

func wikiNote(c *ui.Context, text string) {
	ui.Column(c).Grow(1).Center().Gap(14).Children(func() {
		ui.Icon(c, iconInfo).Size(40, 40).TextColor(white.Alpha(0.3))
		ui.Text(c, text).FontSize(17).FontWeight(600).TextColor(white.Alpha(0.45))
	})
}

// wikiFact is a label and its values, the values as chips.
func wikiFact(c *ui.Context, label string, values []string) {
	ui.Row(c).Padding(8, 12).Gap(14).AlignItems(ui.Start).Children(func() {
		ui.Text(c, label).FontSize(13).TextColor(white.Alpha(0.5)).Width(64).Shrink(0).Margin(4, 0, 0, 0)
		ui.Row(c).Grow(1).Shrink(1).MinWidth(0).Wrap().Gap(6).Children(func() {
			for _, v := range values {
				ui.Text(c, v).FontSize(13).FontWeight(600).TextColor(white.Alpha(0.88)).
					Padding(3, 10).Radius(999).Background(white.Alpha(0.08))
			}
		})
	})
}

func wikiLine(c *ui.Context, title, detail string) {
	ui.Column(c).Padding(8, 12).Gap(3).Children(func() {
		ui.Text(c, title).FontSize(14).FontWeight(600).TextColor(white.Alpha(0.88))
		if detail != "" {
			ui.Text(c, detail).FontSize(12.5).TextColor(white.Alpha(0.55))
		}
	})
}

// wikiSong is a song like the one playing, as a card: its cover, name and
// artist; a click plays it next.
func (a *app) wikiSong(c *ui.Context, s netease.WikiSong, opening bool, width float32) {
	card := ui.ButtonBase(c).Width(width).Column().Gap(6).Padding(0, 0, 4, 0).FocusRing(false).
		Label("播放 " + s.Name).Tooltip(s.Name + " - " + s.Artist)
	hot := card.Hovered()
	card.Children(func() {
		ui.Box(c.Key("art")).Width(width).Height(width).Children(func() {
			if s.CoverURL != "" {
				a.cover(c, s.CoverURL, int(min(300, width*2)), width, 14)
			} else {
				ui.Column(c).Width(width).Height(width).Radius(14).Background(white.Alpha(0.08)).Center().Children(func() {
					ui.Icon(c, iconNote).Size(width*0.28, width*0.28).TextColor(white.Alpha(0.35))
				})
			}
			if hot || opening {
				ui.Column(c).Absolute().Left(0).Top(0).Width(width).Height(width).Radius(14).Background(ui.RGBA(0, 0, 0, 0.35)).Center().Children(func() {
					if opening {
						ui.Text(c, "正在打开…").FontSize(12).TextColor(white)
					} else {
						ui.Icon(c, iconPlay).Size(26, 26).TextColor(white)
					}
				})
			}
		})
		ui.Text(c, s.Name).FontSize(13).FontWeight(600).TextColor(white.Alpha(0.9)).SingleLine()
		ui.Text(c, strings.ReplaceAll(s.Artist, "/", " / ")).FontSize(11.5).TextColor(white.Alpha(0.55)).SingleLine()
	})
	if card.Clicked() {
		a.playSimilar(s)
	}
}

// wikiVersion is another version of the song, as a small card without a
// cover (NetEase gives none): its name and who sings it.
func (a *app) wikiVersion(c *ui.Context, s netease.WikiSong, opening bool, width float32) {
	card := ui.ButtonBase(c).Width(width).Padding(10, 12).Gap(10).Radius(14).AlignItems(ui.Center).FocusRing(false).
		Label("播放 " + s.Name)
	bg := white.Alpha(0.06)
	if card.Hovered() {
		bg = white.Alpha(0.11)
	}
	card.Background(bg).Children(func() {
		icon := iconPlay
		if opening {
			icon = iconReload
		}
		ui.Icon(c, icon).Size(14, 14).TextColor(white.Alpha(0.6)).Shrink(0)
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, s.Name).FontSize(13).FontWeight(600).TextColor(white.Alpha(0.9)).SingleLine()
			ui.Text(c, strings.ReplaceAll(s.Artist, "/", " / ")).FontSize(11.5).TextColor(white.Alpha(0.55)).SingleLine()
		})
	})
	if card.Clicked() {
		a.playSimilar(s)
	}
}

// wikiPlaylist is a playlist with the song in it, as a card that opens it.
func (a *app) wikiPlaylist(c *ui.Context, p netease.WikiPlaylist, width float32) {
	card := ui.ButtonBase(c).Width(width).Column().Gap(6).Padding(0, 0, 4, 0).FocusRing(false).Label("打开歌单 " + p.Name).Tooltip(p.Name)
	hot := card.Hovered()
	card.Children(func() {
		art := a.cover(c, p.CoverURL, int(min(300, width*2)), width, 14)
		if hot {
			art.Opacity(0.85)
		}
		ui.Text(c, p.Name).FontSize(12.5).FontWeight(600).LineHeight(1.35).TextColor(white.Alpha(0.9)).MaxLines(2)
	})
	if card.Clicked() {
		a.openPage(route{page: pagePlaylist, id: p.ID})
	}
}
