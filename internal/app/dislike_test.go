package app

import (
	"testing"

	"netease-music-mygo/internal/netease"
)

// Not into one of the day's songs, from its menu: it leaves the list, and
// the song NetEase gives for it takes its place.
func TestDailyDislike(t *testing.T) {
	asked := make(chan int64, 1)
	dailyDislike = func(id int64) (netease.Track, bool, error) {
		asked <- id
		return netease.Track{ID: 9, Name: "新的一首"}, true, nil
	}
	defer func() { dailyDislike = netease.DailyDislike }()
	a := editTestApp()
	a.daily = []netease.Track{{ID: 1, Name: "一"}, {ID: 2, Name: "二"}, {ID: 3, Name: "三"}}
	label, do := a.dislikeKind(&a.daily[1])
	if label != "不感兴趣" || do == nil {
		t.Fatalf("a daily song's menu has no 不感兴趣: %q", label)
	}
	do()
	if id := <-asked; id != 2 {
		t.Fatalf("told NetEase of song %d", id)
	}
	if len(a.daily) != 2 {
		t.Fatalf("the song stayed: %d songs", len(a.daily))
	}
	// What the request posts, without a window to post it to.
	a.dailyReplaced(1, netease.Track{ID: 9}, true, nil)
	var ids []int64
	for _, d := range a.daily {
		ids = append(ids, d.ID)
	}
	if len(ids) != 3 || ids[0] != 1 || ids[1] != 9 || ids[2] != 3 {
		t.Fatalf("daily songs after: %v", ids)
	}
	if _, do := a.dislikeKind(&netease.Track{ID: 50}); do != nil {
		t.Fatal("a song of no recommendation offers 不感兴趣")
	}
	a.profile = nil
	if _, do := a.dislikeKind(&a.daily[0]); do != nil {
		t.Fatal("signed out, 不感兴趣 shows")
	}
}
