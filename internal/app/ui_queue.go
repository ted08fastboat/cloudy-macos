package app

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// queueLook is how the rows of the queue are colored: white over Now
// Playing, the pane's ink in the bar's popover.
type queueLook struct {
	text, head, artist, time ui.Color
	on, hover, pill          ui.Color
	dark                     bool // its menus go over dark
}

// queueRows lists the songs to play, in the order they will, the one
// playing lit: a click plays one, a drag moves it, and the ✕ takes it out.
// list scrolls them; at is the row it last brought into view, -1 while
// they were not shown, for them to open at the song playing.
func (a *app) queueRows(c *ui.Context, list *ui.ListState, at *int, look queueLook, rowH, coverSize float32) {
	pl := a.pl
	order, cur := pl.PlayOrder()
	// Opened, it shows the track playing at its top; after, it follows it
	// as little as keeps it in view.
	if cur >= 0 && cur != *at {
		if *at < 0 {
			list.ScrollTo(cur, ui.Start)
		} else {
			list.ScrollIntoView(cur)
		}
		*at = cur
	}
	queue := pl.Queue()
	// Keyed by the song: removing one shifts the indices of those after it.
	list.Key = func(row int) any { return queue[order[row]].ID }
	// Dragged, the rows move in the play order; the song playing plays on.
	list.Reorder = func(rows []int, to int) { pl.Reorder(rows, to) }
	if len(order) > 0 {
		ui.Row(c.Key("queue-head")).Height(36).PaddingX(10).Gap(6).AlignItems(ui.Center).Children(func() {
			left := len(order) - max(cur, 0) - 1
			ui.Text(c, fmt.Sprintf("共 %d 首 · 接下来 %d 首", len(order), left)).FontSize(12).TextColor(look.head).Grow(1).MinWidth(0).SingleLine().
				Tooltip("拖动歌曲可调整顺序")
			// A playlist of theirs onto the end of the queue. The playlists
			// open in a drawer under the head, not a menu: over the queue's
			// pane a menu's press closed the pane, and the menu with it,
			// before it chose.
			add := iconButton(c.Key("queue-add-list"), iconPlus, 14, look.text, "添加歌单到播放列表").Size(28, 28)
			if a.queueAdding {
				add.Background(look.hover)
			}
			if add.Clicked() {
				if a.profile == nil {
					a.more.queue.open = false
					a.toast = "登录后才能添加歌单"
					a.openLogin()
				} else {
					a.queueAdding = !a.queueAdding
				}
			}
			if pillButton(c.Key("clear-queue"), nil, "清空待播", look.pill, look.text).Tooltip("清空整个播放列表，正在播放的歌曲也会停止").Clicked() {
				a.queueAdding = false
				pl.Clear()
				a.toast = "已清空播放列表"
			}
		})
		a.queueAddDrawer(c, look)
	}
	ui.List(c.Key("queue"), list, len(order), func(r int) {
		i := order[r]
		tr := &queue[i]
		playing := r == cur
		row := ui.ButtonBase(c.Key(tr.ID)).Height(rowH).PaddingX(10).Gap(14).Radius(14).AlignItems(ui.Center).FocusRing(false).Label(tr.Name)
		a.contextMenu(row, look.dark, func() [][]menuAction {
			acts := a.songActions(tr, 0)
			if !playing {
				acts = append([][]menuAction{{{label: "从待播清单移除", icon: iconX, do: func() { pl.RemoveAt(r) }}}}, acts...)
			}
			return acts
		}, a.songHead(tr))
		switch {
		case playing:
			row.Background(look.on)
		case row.Hovered():
			row.Background(look.hover)
		}
		if !tr.Playable() {
			row.Opacity(0.4)
		}
		row.Children(func() {
			a.cover(c, tr.Album.PicURL, 96, coverSize, 8)
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				name := ui.Text(c, tr.Name).FontSize(14).FontWeight(600).SingleLine()
				if playing && !look.dark {
					name.TextColor(a.pal.accent)
				}
				a.artistLinks(c, tr, look.artist, look.text).FontSize(12.5).SingleLine()
			})
			ui.Text(c, clock(tr.Length())).FontSize(12).FontFeatures("tnum").TextColor(look.time)
			// 播放中的歌曲不可移除，但仍保留操作区，让各行时长对齐。
			reveal(ui.Box(c.Key("remove-reveal")).Size(26, 26).Shrink(0), row.Hovered()).Children(func() {
				if !playing {
					if iconButton(c.Key("remove"), iconX, 12, look.artist, "从待播清单移除").Size(26, 26).Clicked() {
						pl.RemoveAt(r)
					}
				}
			})
		})
		if row.Clicked() && tr.Playable() && !playing {
			pl.Jump(i)
		}
	}).Grow(1).Padding(8, 0, 40, 0).Gap(2)
}

// queueButton opens the queue from the player bar, in a pane over the
// page: the songs to play, the one playing in view.
func (a *app) queueButton(c *ui.Context, color ui.Color) {
	q := &a.more.queue
	tint := color
	if q.open {
		tint = a.pal.accent
	}
	b := iconButton(c.Key("queue-button"), iconQueue, 16, tint, "播放列表")
	if b.Clicked() {
		q.open = !q.open
	}
	if !q.open {
		q.at = -1
	}
	ui.PopoverBase(c.Key("queue-popover"), b, &q.open, func(panel ui.Element) {
		ink := a.inkOn(false)
		a.pane(panel, 20, false)
		h := a.winHeight(c)
		panel.AttachTo(b, ui.AnchorTopEnd, ui.AnchorBottomEnd).Margin(0, 0, 12, 0).Width(400).Height(max(240, min(600, h-160))).
			Padding(10, 8, 0, 8).TextColor(ink.text).Label("播放列表").TransformOrigin(1, 1)
		a.paneMotion(panel)
		panel.Children(func() {
			ui.Row(c).PaddingX(10).Height(30).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "播放列表").FontSize(15).FontWeight(700).Grow(1)
				if iconButton(c.Key("queue-close"), iconX, 12, ink.muted, "关闭").Size(26, 26).Clicked() {
					q.open = false
				}
			})
			if len(a.pl.Queue()) == 0 {
				a.queueEmpty(c, ink)
				return
			}
			a.queueRows(c, &q.list, &q.at, queueLook{
				text: ink.text, head: ink.muted, artist: ink.muted, time: ink.faint,
				on: a.pal.accent.Alpha(0.12), hover: ink.hover, pill: ink.hover,
			}, 52, 38)
		})
	})
}

// queueEmpty is the queue's pane with nothing queued: says so, and, signed
// in, lists the listener's own playlists, the liked songs first, each
// playing all its songs from the start when clicked.
func (a *app) queueEmpty(c *ui.Context, ink paneInk) {
	lists, names := a.ownLists()
	if len(lists) == 0 {
		ui.Column(c).Grow(1).Center().Children(func() {
			ui.Text(c, "还没有要播放的歌曲").FontSize(13).TextColor(ink.muted)
		})
		return
	}
	ui.Column(c).Grow(1).MinHeight(0).Children(func() {
		ui.Column(c).Padding(28, 10, 14, 10).Gap(4).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "还没有要播放的歌曲").FontSize(13).TextColor(ink.muted)
			ui.Text(c, "从你的歌单开始播放").FontSize(12).TextColor(ink.faint)
		})
		ui.Scroll(c.Key("queue-lists")).Grow(1).MinHeight(0).Padding(0, 0, 12, 0).Children(func() {
			for i, pl := range lists {
				id, name := pl.ID, names[i]
				b := ui.ButtonBase(c.Key(id)).Height(52).PaddingX(10).Gap(12).Radius(14).AlignItems(ui.Center).Justify(ui.Start).
					FocusRing(false).Label("播放歌单 " + name)
				bg := ink.hover.Alpha(0)
				if b.Hovered() {
					bg = ink.hover
				}
				b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 100 * time.Millisecond}))
				loading := a.loadingPl[id]
				b.Children(func() {
					ui.Box(c).Size(38, 38).Radius(8).Shrink(0).Children(func() { a.coverFill(c, pl.Cover(), 120, 8) })
					ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(2).Children(func() {
						ui.Text(c, name).FontSize(13.5).FontWeight(600).TextColor(ink.text).SingleLine()
						sub := fmt.Sprintf("%d 首", pl.TrackCount)
						if loading {
							sub = "正在载入…"
						}
						ui.Text(c, sub).FontSize(12).TextColor(ink.muted).SingleLine()
					})
					ui.Icon(c, iconPlay).Size(14, 14).TextColor(ink.muted.Alpha(b.AnimateTo("play", b2f(b.Hovered()), ui.Snappy))).Shrink(0)
				})
				if b.Clicked() && !loading {
					a.loadPlaylist(id, true)
				}
			}
		})
	})
}

// ownLists is the listener's own playlists, the liked songs first, each
// with the name the queue shows it by.
func (a *app) ownLists() (lists []*netease.Playlist, names []string) {
	if a.profile == nil {
		return nil, nil
	}
	for i := range a.mine {
		pl := &a.mine[i]
		if pl.SpecialType == 5 {
			lists = append([]*netease.Playlist{pl}, lists...)
			names = append([]string{"我喜欢的音乐"}, names...)
		} else {
			lists = append(lists, pl)
			names = append(names, pl.Name)
		}
	}
	return lists, names
}

// queueAddDrawer is the listener's playlists, under the queue's head while
// they add one to the queue: the liked songs first, then the others; the
// one chosen goes onto the end of the queue, and the drawer closes.
func (a *app) queueAddDrawer(c *ui.Context, look queueLook) {
	drawer(c, "queue-add", a.queueAdding && a.profile != nil, func() {
		lists, names := a.ownLists()
		ui.Column(c).Padding(2, 6, 8, 6).Gap(2).Children(func() {
			ui.Text(c, "添加歌单到播放列表").FontSize(12).FontWeight(600).TextColor(look.head).Margin(2, 0, 4, 6)
			for i, pl := range lists {
				id, name := pl.ID, names[i]
				b := ui.ButtonBase(c.Key(id)).Height(40).PaddingX(10).Gap(10).Radius(14).AlignItems(ui.Center).Justify(ui.Start).
					FocusRing(false).Label("加入播放列表 " + name)
				bg := look.hover.Alpha(0)
				if b.Hovered() {
					bg = look.hover
				}
				b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 100 * time.Millisecond})).TextColor(look.text)
				b.Children(func() {
					ui.Box(c).Size(28, 28).Radius(8).Shrink(0).Children(func() { a.coverFill(c, pl.Cover(), 80, 8) })
					ui.Text(c, name).FontSize(13.5).SingleLine().Shrink(1).MinWidth(0)
					ui.Box(c).Grow(1)
					ui.Textf(c, "%d 首", pl.TrackCount).FontSize(12).TextColor(look.time).SingleLine().Shrink(0)
				})
				if b.Clicked() {
					a.queueAdding = false
					a.queuePlaylist(id)
				}
			}
		})
	})
}

// queuePlaylist puts the playlist id's songs onto the end of the queue,
// loading the playlist first if it has not been.
func (a *app) queuePlaylist(id int64) {
	if pl := a.playlists[id]; pl != nil && len(pl.Tracks) > 0 {
		a.addAllToQueue(pl.Tracks)
		return
	}
	a.toast = "正在载入歌单…"
	gen := a.accountGen
	go func() {
		pl, err := fetchPlaylist(id)
		a.post(func() {
			if gen != a.accountGen {
				return
			}
			if err != nil || pl == nil {
				a.toast = "歌单载入失败，请稍后再试"
				return
			}
			a.addAllToQueue(pl.Tracks)
		})
	}()
}
