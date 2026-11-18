package app

import "testing"

// A click anywhere on what plays opens Now Playing: on the cover too,
// which also answers a right click with its menu.
func TestPlayerBarClickOpensNowPlaying(t *testing.T) {
	for _, width := range []int{880, 1440} {
		for _, name := range []string{"播放栏封面", "播放栏歌名", "打开正在播放"} {
			a := playerBarLayoutApp()
			tt := playerBarTester(a, width)
			r := barElement(t, tt, name)
			tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
			tt.Frame()
			if !a.nowPlaying {
				t.Errorf("width %d: a click on %s did not open Now Playing", width, name)
			}
		}
	}
	a := playerBarLayoutApp()
	tt := playerBarTester(a, 1440)
	r := barElement(t, tt, "播放栏封面")
	tt.RightClickAt(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	if a.nowPlaying || !a.menu.open {
		t.Errorf("right click on the cover: nowPlaying %v, menu %v", a.nowPlaying, a.menu.open)
	}
}
