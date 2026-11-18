package app

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// dislikeKind is what not liking tr tells NetEase, if anything: the
// personal FM playing it, or the day's recommendation it is one of.
func (a *app) dislikeKind(tr *netease.Track) (label string, do func()) {
	if a.profile == nil || tr == nil || tr.ProgramID != 0 || tr.ID <= 0 {
		return "", nil
	}
	if cur := a.pl.Current(); cur != nil && cur.ID == tr.ID && strings.HasPrefix(a.pl.Radio(), "fm:") {
		return "不喜欢", a.trashFM
	}
	if slices.ContainsFunc(a.daily, func(d netease.Track) bool { return d.ID == tr.ID }) {
		song := *tr
		return "不感兴趣", func() { a.dislikeDaily(song) }
	}
	return "", nil
}

// dislikeButton is, by the heart in Now Playing, not liking the song: on
// the FM, and for the day's songs (viewers, 1.3.1: only the FM's own page
// had it).
func (a *app) dislikeButton(c *ui.Context, tr *netease.Track, size float32, color ui.Color) {
	label, do := a.dislikeKind(tr)
	if do == nil {
		return
	}
	// In a slot as wide as the heart's and the comments', which it stands
	// between, at the same steps.
	ui.Box(c.Key("dislike-slot")).Width(countSlot(size, true)).Shrink(0).Children(func() {
		if iconButton(c.Key("dislike"), iconTrash, size*0.9, color, label).Size(size+16, size+16).Clicked() {
			do()
		}
	})
}

// dislikeDaily takes song out of the day's songs, and puts in its place
// the one NetEase gives for it. Playing, it goes on to the next.
func (a *app) dislikeDaily(song netease.Track) {
	at := slices.IndexFunc(a.daily, func(d netease.Track) bool { return d.ID == song.ID })
	if at < 0 {
		return
	}
	if cur := a.pl.Current(); cur != nil && cur.ID == song.ID {
		a.pl.Next()
	}
	a.daily = slices.Delete(slices.Clone(a.daily), at, at+1)
	a.toast = "已减少推荐：" + song.Name
	a.redraw()
	go func() {
		next, ok, err := dailyDislike(song.ID)
		a.post(func() { a.dailyReplaced(at, next, ok, err) })
	}()
}

// dailyReplaced puts next where the song not liked was, at, as NetEase
// answered.
func (a *app) dailyReplaced(at int, next netease.Track, ok bool, err error) {
	switch {
	case err != nil:
		a.toast = "没能告诉网易云：" + err.Error()
	case ok && !slices.ContainsFunc(a.daily, func(d netease.Track) bool { return d.ID == next.ID }):
		a.daily = slices.Insert(slices.Clone(a.daily), min(at, len(a.daily)), next)
	}
	a.redraw()
}

// dailyDislike is netease.DailyDislike, for the tests to replace.
var dailyDislike = netease.DailyDislike
