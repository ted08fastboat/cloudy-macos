package app

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"netease-music-mygo/internal/netease"
)

var fetchRedCount = netease.SongRedCount

type redCountState struct {
	value       netease.RedCount
	known, busy bool
	failed      bool
	at          time.Time
}

// Refresh a count without removing its last successful value from the UI.
func (a *app) redCountChanged(id int64) {
	if s := a.redCounts[id]; s != nil {
		s.at = time.Time{}
	}
}

// askRedCount shares the current song's count between the bar and playback
// page. Ordinary list rows do not fetch counts.
func (a *app) askRedCount(id int64) *redCountState {
	if a.redCounts == nil {
		a.redCounts = map[int64]*redCountState{}
	}
	s := a.redCounts[id]
	if s == nil {
		if len(a.redCounts) >= 128 {
			clear(a.redCounts)
		}
		s = &redCountState{}
		a.redCounts[id] = s
	}
	gap := time.Minute
	if s.known && !s.failed {
		gap = 10 * time.Minute
	}
	if a.win == nil || s.busy || time.Since(s.at) < gap {
		return s
	}
	s.busy, s.at = true, time.Now()
	gen := a.accountGen
	go func() {
		value, err := fetchRedCount(id)
		a.post(func() {
			if gen != a.accountGen || a.redCounts[id] != s {
				return
			}
			a.redCountAnswered(id, s, value, err)
		})
	}()
	return s
}

func (a *app) redCountAnswered(id int64, s *redCountState, value netease.RedCount, err error) {
	if a.redCounts[id] != s {
		return
	}
	s.busy = false
	s.failed = err != nil
	if err == nil {
		s.value, s.known = value, true
	}
}

func (a *app) likeCountButton(c *ui.Context, tr *netease.Track, size float32, color ui.Color) ui.Element {
	return a.countedHeart(c, tr, size, color, true)
}

// countedHeart can put the count in its tooltip only, without reserving
// empty space for it in a narrow player bar.
func (a *app) countedHeart(c *ui.Context, tr *netease.Track, size float32, color ui.Color, showCount bool) ui.Element {
	if tr.LocalPath != "" || tr.ProgramID != 0 {
		return a.likeButton(c, tr, size, color)
	}
	s := a.askRedCount(tr.ID)
	red := a.heartRed()
	shade := color
	if a.isLiked(tr) {
		shade = red
	}
	wrap := ui.Box(c).Width(countSlot(size, showCount)).Height(size + 16).AlignItems(ui.Start)
	wrap.Children(func() {
		// Counted, the count sits in a notch of the heart's upper right,
		// as NetEase's apps have it.
		notched := s.known && showCount
		outline, filled := iconHeart, iconHeartFill
		if notched {
			outline, filled = iconHeartNotch, iconHeartNotchFill
		}
		heart := a.heartButton(c.Key("heart"), tr, size, color, red, outline, filled)
		if s.known {
			heart.Tooltip(fmt.Sprintf("%d 人喜欢", s.value.Count))
		}
		if notched {
			label := s.value.Description
			if label == "" {
				label = count(s.value.Count)
			}
			notchCount(c.Key("red-count"), label, size, shade).Label(fmt.Sprintf("%d 人喜欢", s.value.Count))
		}
	})
	return wrap
}

// countSlot is the width the heart, the comments and the buttons beside
// them each take, so that they stand at even steps: an icon button's, and
// with counts, room for one of four characters out of its notch.
func countSlot(size float32, counted bool) float32 {
	if !counted {
		return size + 16
	}
	return 8 + size*countNotchX + 2.5*countFont(size) + 2
}

// countNotchX is where a count starts in its icon's notch, of its size.
const countNotchX = 0.76

func countFont(size float32) float32 { return max(9, size*0.44) }

// notchCount is a count set in the notch of an icon button of size, at
// the left of the slot it is in.
func notchCount(c *ui.Context, label string, size float32, color ui.Color) ui.Element {
	fs := countFont(size)
	return ui.Text(c, label).FontSize(fs).FontWeight(700).FontFeatures("tnum").TextColor(color).LineHeight(1).
		Absolute().Top(8 - size*0.06 - fs/2).Left(8 + size*countNotchX).SingleLine().PassThrough()
}

// heartRed is the red of a liked song's heart, brighter on the dark.
func (a *app) heartRed() ui.Color {
	if a.pal.dark || a.nowPlaying {
		return ui.Hex("#ff3b43")
	}
	return ui.Hex("#d92d43")
}
