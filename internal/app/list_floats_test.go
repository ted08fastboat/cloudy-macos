package app

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// A long playlist has, at its lower right, a button to the song playing
// while it is out of view, and above it, scrolled well down, one back to
// the top.
func TestListFloats(t *testing.T) {
	a := editTestApp()
	var tracks []netease.Track
	for i := range 80 {
		tracks = append(tracks, netease.Track{ID: int64(100 + i), Name: fmt.Sprintf("第%d首", i), Duration: 200000})
	}
	a.playlists[10].Tracks = tracks
	a.pl.Cue(tracks[60:])
	tt := ui.NewTester(a.view, 1200, 800)
	settle := func() {
		for range 4 {
			tt.Advance(time.Second / 2)
			tt.Frame()
		}
	}
	settle()
	if _, ok := tt.Find("回到顶部"); ok {
		t.Fatal("at the top, the button back to it shows")
	}
	locate, ok := tt.Find("定位到正在播放")
	if !ok {
		t.Fatal("the song playing is out of view and no button goes to it")
	}
	tt.ClickAt(locate.X+locate.W/2, locate.Y+locate.H/2)
	for range 15 {
		tt.Advance(time.Second / 60)
		tt.Frame()
	}
	if mid, _ := a.trackList.Visible(); mid == 0 || mid >= a.trackRow[60]-8 {
		t.Fatalf("locating jumped: first row %d a quarter second in", mid)
	}
	settle()
	if first, last := a.trackList.Visible(); a.trackRow[60] < first || a.trackRow[60] > last {
		t.Fatalf("locating left row %d out of view (%d–%d)", a.trackRow[60], first, last)
	}
	if _, ok := tt.Find("定位到正在播放"); ok {
		t.Fatal("the song in view, the locate button stays")
	}
	top, ok := tt.Find("回到顶部")
	if !ok {
		t.Fatal("scrolled down, no button back to the top")
	}
	before, _ := a.trackList.Visible()
	tt.ClickAt(top.X+top.W/2, top.Y+top.H/2)
	tt.Advance(150 * time.Millisecond)
	tt.Frame()
	if mid, _ := a.trackList.Visible(); mid == 0 || mid >= before {
		t.Fatalf("back to the top jumped: row %d, then %d", before, mid)
	}
	settle()
	if first, _ := a.trackList.Visible(); first != 0 {
		t.Fatalf("back to the top left the list at row %d", first)
	}
}
