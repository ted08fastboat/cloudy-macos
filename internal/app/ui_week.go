package app

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// 听歌周报: the songs the listener played most in the last seven days, as
// NetEase counts them for the account, on every device it plays on; and,
// from the songs played last, on which days and on which devices. The
// calendar has a page of it, which opens 本周听歌排行, all of them.

// weekState is the week's ranking, and what the songs played last tell of
// the week.
type weekState struct {
	songs   []netease.Ranked
	stats   weekStats
	loading bool
}

// weekStats is what the songs played last tell of the last seven days:
// how many were played each day, the last day last, if they reach back
// that far; and how many on phones and on computers.
type weekStats struct {
	days            [7]int
	daysKnown       bool
	phone, computer int
}

// Tests replace the ranking and the plays without signing in to a real
// account.
var weekRanking = netease.WeekRanking

// loadWeek loads the week's ranking for whoever is signed in, and the
// songs they played last.
func (a *app) loadWeek() {
	if a.profile == nil {
		a.week = weekState{}
		return
	}
	if a.week.loading {
		return
	}
	a.week.loading = true
	uid, gen := a.profile.UserID, a.accountGen
	go func() {
		songs, err := weekRanking(uid)
		plays, _ := recentPlays(300)
		stats := weekStatsOf(plays, time.Now())
		a.post(func() {
			if gen != a.accountGen {
				return
			}
			a.week.loading = false
			if err == nil {
				a.week.songs, a.week.stats = songs, stats
			}
		})
	}()
}

// weekStart is the first of the seven days ending with the one now is in.
func weekStart(now time.Time) time.Time {
	y, m, d := now.Date()
	return time.Date(y, m, d-6, 0, 0, 0, 0, now.Location())
}

// weekStatsOf counts plays, the latest first, by day and by device over
// the seven days ending now. NetEase keeps the last few hundred, the last
// play of each song; the days are known when they reach back past the
// first day.
func weekStatsOf(plays []netease.Play, now time.Time) weekStats {
	var s weekStats
	start := weekStart(now)
	for _, p := range plays {
		at := p.At.In(now.Location())
		if at.Before(start) {
			s.daysKnown = true
			continue
		}
		day := int(at.Sub(start).Hours() / 24)
		if day < 0 || day > 6 {
			continue
		}
		s.days[day]++
		switch strings.ToLower(p.Device) {
		case "android", "iphone", "ios", "ipad", "harmony", "harmonyos", "wp":
			s.phone++
		case "pc", "osx", "mac", "macos", "web", "linux", "uwp":
			s.computer++
		}
	}
	return s
}

// devices says how the week's plays split between phones and computers,
// as "手机 62% · 电脑 38%", or nothing when they were all on one kind.
func (s weekStats) devices() string {
	n := s.phone + s.computer
	if n == 0 || s.phone == 0 || s.computer == 0 {
		switch {
		case s.phone > 0:
			return "全在手机上听"
		case s.computer > 0:
			return "全在电脑上听"
		}
		return ""
	}
	p := (s.phone*100 + n/2) / n
	return fmt.Sprintf("手机 %d%% · 电脑 %d%%", p, 100-p)
}

// weekMinutes is about how long the listener listened in the week: each
// song as long as it is, as many times as they played it.
func weekMinutes(songs []netease.Ranked) int {
	var ms int64
	for _, s := range songs {
		ms += s.Track.Duration * int64(max(1, s.Plays))
	}
	return int(ms / 60000)
}

// weekLength says minutes as "约 6 小时 20 分".
func weekLength(minutes int) string {
	switch {
	case minutes < 60:
		return fmt.Sprintf("约 %d 分钟", max(1, minutes))
	case minutes%60 == 0:
		return fmt.Sprintf("约 %d 小时", minutes/60)
	}
	return fmt.Sprintf("约 %d 小时 %d 分", minutes/60, minutes%60)
}

// weekRange names the seven days ending now, as "11.09 – 11.15".
func weekRange(now time.Time) string {
	s := weekStart(now)
	return fmt.Sprintf("%d.%02d – %d.%02d", s.Month(), s.Day(), now.Month(), now.Day())
}

// weekTracks is the week's songs, the most played first.
func (a *app) weekTracks() []netease.Track {
	out := make([]netease.Track, len(a.week.songs))
	for i, s := range a.week.songs {
		out[i] = s.Track
	}
	return out
}

// topArtists names the singers of songs, the most played first, n of them
// at most.
func topArtists(songs []netease.Ranked, n int) string {
	plays := map[string]int{}
	var names []string
	for _, s := range songs {
		if len(s.Track.Artists) == 0 {
			continue
		}
		name := s.Track.Artists[0].Name
		if name == "" {
			continue
		}
		if _, ok := plays[name]; !ok {
			names = append(names, name)
		}
		plays[name] += max(1, s.Plays)
	}
	slices.SortStableFunc(names, func(x, y string) int { return plays[y] - plays[x] })
	return strings.Join(names[:min(n, len(names))], "、")
}

// weekSongCount says how many songs the week had, the ranking holding a
// hundred at most.
func weekSongCount(n int) string {
	if n >= 100 {
		return "听了 100+ 首"
	}
	return fmt.Sprintf("听了 %d 首", n)
}

// weekSheet is 听歌周报's page: about how long the listener listened in the
// last seven days, how many songs and whose most, how they split between
// phones and computers, a button that plays the songs, the most played
// first; the first three, their covers numbered, each playing from
// there; and how many songs each day had, as bars. The rest of it opens
// 本周听歌排行.
func (a *app) weekSheet(c *ui.Context, back bool) {
	w := &a.week
	songs := w.songs
	if len(songs) == 0 {
		return
	}
	tracks := a.weekTracks()
	inner := a.deckInner(c)
	ours := a.queueIs(tracks)
	playing := a.pl.Playing()
	now := time.Now()
	a.deckSheet(c, deckWeek, back, songs[0].Track.Album.PicURL, func() { a.open(route{page: pageWeek}) }, func() {
		deckLabel(c, iconHistory, "听歌周报 · "+weekRange(now))
		deckTitle(c, weekLength(weekMinutes(songs)))
		line := weekSongCount(len(songs))
		if who := topArtists(songs, 2); who != "" {
			line += " · 最常听 " + who
		}
		ui.Text(c, line).FontSize(13).TextColor(white.Alpha(0.8)).MaxLines(1)
		ui.Row(c).Gap(12).AlignItems(ui.Center).Margin(12, 0, 0, 0).Children(func() {
			a.dailyPlayButton(c.Key("play"), tracks, ours, ours && playing)
			if d := w.stats.devices(); d != "" {
				ui.Row(c).Gap(5).AlignItems(ui.Center).TextColor(white.Alpha(0.72)).Children(func() {
					ui.Icon(c, iconMiniWin).Size(13, 13)
					ui.Text(c, d).FontSize(12).FontWeight(600).FontFeatures("tnum").SingleLine()
				})
			}
		})
	}, func(hovered bool) {
		if inner >= 860 {
			a.weekTop(c, tracks, ours)
		}
		switch {
		case inner >= 620 && w.stats.daysKnown:
			weekBars(c, w.stats.days, now)
		case inner >= 620:
			a.weekMosaic(c, tracks, hovered, 150)
		case inner >= deckSmallMin:
			a.weekMosaic(c, tracks, hovered, deckSmallCover)
		}
	})
}

// weekTop is the week's first three songs, their covers numbered, each
// playing the week's songs from it.
func (a *app) weekTop(c *ui.Context, tracks []netease.Track, ours bool) {
	songs := a.week.songs
	cur := a.pl.Current()
	playing := a.pl.Playing()
	ui.Row(c.Key("top")).Gap(14).Shrink(0).AlignItems(ui.Start).Children(func() {
		for i := range min(3, len(songs)) {
			s := &songs[i]
			current := ours && cur != nil && cur.ID == s.Track.ID
			label := "播放 " + s.Track.Name
			if current && playing {
				label = "暂停 " + s.Track.Name
			}
			b := ui.ButtonBase(c.Key(s.Track.ID)).Column().Width(84).Gap(6).FocusRing(false).Label(label)
			up := b.AnimateTo("up", b2f(b.Hovered()), ui.Snappy)
			b.Children(func() {
				ui.Box(c).Size(84, 84).Radius(14).Shadow(0, 8, 18, -6, ui.RGBA(0, 0, 0, 0.45)).Translate(0, -3*up).Children(func() {
					a.coverFill(c, s.Track.Album.PicURL, 200, 14)
					ui.Box(c).Absolute().Top(6).Left(6).Size(22, 22).Radius(999).Center().Background(ui.RGBA(0, 0, 0, 0.55)).Children(func() {
						ui.Text(c, fmt.Sprint(i+1)).FontSize(11.5).FontWeight(800).TextColor(white)
					})
					weekPlayMark(c, b.Hovered(), current, current && playing)
				})
				ui.Text(c, s.Track.Name).FontSize(12).FontWeight(600).SingleLine()
				ui.Textf(c, "%d 次", s.Plays).FontSize(11).FontFeatures("tnum").TextColor(white.Alpha(0.6)).SingleLine()
			})
			if b.Clicked() {
				if current {
					a.pl.TogglePlay()
				} else {
					a.pl.PlayQueue(tracks, i)
				}
			}
		}
	})
}

// weekBars is how many songs each of the seven days had, as bars, the
// day now is in lit.
func weekBars(c *ui.Context, days [7]int, now time.Time) {
	most := 1
	for _, n := range days {
		most = max(most, n)
	}
	start := weekStart(now)
	ui.Column(c.Key("bars")).Gap(6).Shrink(0).Margin(0, 6, 0, 0).Children(func() {
		ui.Text(c, "每天听过的歌").FontSize(11).FontWeight(600).TextColor(white.Alpha(0.6))
		ui.Row(c).Height(96).Gap(8).AlignItems(ui.End).Children(func() {
			for i, n := range days {
				share := float32(n) / float32(most)
				today := i == 6
				ui.Column(c).Width(16).Height(96).Justify(ui.End).Tooltip(fmt.Sprintf("%d 首", n)).Children(func() {
					fill := white.Alpha(0.5)
					if today {
						fill = white.Alpha(0.95)
					}
					ui.Box(c).Height(max(3, 96*share)).Radius(4).Background(fill)
				})
			}
		})
		ui.Row(c).Gap(8).Children(func() {
			for i := range 7 {
				day := start.AddDate(0, 0, i)
				ui.Text(c, weekdayShort(day)).Width(16).FontSize(10.5).TextColor(white.Alpha(0.6)).TextAlign(ui.Center)
			}
		})
	})
}

// weekdayShort names t's day of the week in one character, 一 to 日.
func weekdayShort(t time.Time) string {
	return [...]string{"日", "一", "二", "三", "四", "五", "六"}[t.Weekday()]
}

// weekMosaic is the week's first four covers, two by two, while the songs
// played last do not reach back over the week.
func (a *app) weekMosaic(c *ui.Context, tracks []netease.Track, hovered bool, size float32) {
	box := ui.Box(c.Key("mosaic")).Size(size, size).Shrink(0).Radius(14).ClipY().Shadow(0, 10, 24, -6, ui.RGBA(0, 0, 0, 0.45)).Margin(0, 12, 0, 0)
	pose := ui.Pose{}
	if hovered {
		pose = ui.Pose{Y: -3, Scale: 1.03}
	}
	box.AnimatePose(pose, ui.Snappy)
	box.Children(func() {
		for i := range min(4, len(tracks)) {
			ui.Box(c).Absolute().Left(float32(i%2)*size/2).Top(float32(i/2)*size/2).Size(size/2, size/2).Children(func() {
				a.coverFill(c, tracks[i].Album.PicURL, 200, 0)
			})
		}
	})
}

// weekPage is 本周听歌排行: the week's songs, the most played first, under
// how long, how many and on which devices, and the first ten as bars as
// long as they were played.
func (a *app) weekPage(c *ui.Context) {
	w := &a.week
	if len(w.songs) == 0 {
		msg := "还没有本周的听歌记录"
		if a.profile == nil {
			msg = "登录后才能查看听歌排行"
		}
		var retry func()
		if a.profile != nil && !w.loading {
			retry = a.loadWeek
		}
		a.centered(c, msg, retry)
		return
	}
	tracks := a.weekTracks()
	header := func(c *ui.Context) {
		p := a.pal
		sub := weekRange(time.Now()) + " · 累计听了" + weekLength(weekMinutes(w.songs)) + " · " + weekSongCount(len(w.songs))
		if d := w.stats.devices(); d != "" {
			sub += " · " + d
		}
		a.heading(c, "本周听歌排行", sub)
		ui.Row(c).Gap(10).Margin(0, 0, 18, 0).Children(func() {
			if pillButton(c.Key("play"), iconPlay, "播放全部", p.accent, p.onAccent).Clicked() {
				a.playList(tracks, false)
			}
			if pillButton(c.Key("shuffle"), iconShuffle, "随机播放", p.hover.Over(p.bg), p.text).Clicked() {
				a.playList(tracks, true)
			}
		})
		// The first ten, each as long as it was played.
		top := max(1, w.songs[0].Plays)
		ui.Grid(c.Key("top-ten")).Columns(2).GapX(28).GapY(10).Margin(0, 0, 22, 0).Children(func() {
			for i := range min(10, len(w.songs)) {
				s := &w.songs[i]
				ui.Row(c.Key(s.Track.ID)).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Text(c, fmt.Sprint(i+1)).Width(20).FontSize(13).FontWeight(700).FontFeatures("tnum").TextColor(p.muted)
					ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(5).Children(func() {
						ui.Text(c, s.Track.Name).FontSize(13).FontWeight(600).SingleLine()
						ui.Box(c).Height(4).Radius(999).Background(p.hover).Children(func() {
							ui.Box(c).Absolute().Top(0).Left(0).Bottom(0).WidthPercent(100 * float32(s.Plays) / float32(top)).Radius(999).Background(p.accent)
						})
					})
					ui.Textf(c, "%d 次", s.Plays).Width(44).FontSize(12).FontFeatures("tnum").TextColor(p.muted).TextAlign(ui.End)
				})
			}
		})
	}
	a.toolRows(c, tracks, true, header)
}

// weekPlayMark is the small round button at the foot of one of the week's
// first covers, its tile's click being its own: play, under the pointer;
// on the song playing, pause, or play while it is paused.
func weekPlayMark(c *ui.Context, hovered, current, playing bool) {
	const size = 26
	shown := ui.Box(c.Key("mark")).Absolute().Right(6).Bottom(6).Size(size, size).Radius(999).Center().PassThrough().
		Background(white).Shadow(0, 4, 10, -2, ui.RGBA(0, 0, 0, 0.35))
	on := shown.AnimateTo("on", b2f(hovered || current), ui.Snappy)
	shown.Opacity(on).AnimatePose(ui.Pose{Scale: 0.7 + 0.3*on}, ui.Snappy)
	icon := iconPlay
	if playing {
		icon = iconPause
	}
	shown.TextColor(ui.Hex("#1d1d1f")).Children(func() {
		// A play icon sits a little right of its middle, as it looks.
		m := ui.Icon(c, icon).Size(11, 11)
		if !playing {
			m.Margin(0, 0, 0, 1)
		}
	})
}
