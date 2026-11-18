package app

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// Sorted by album and shown as a grid, an album opens as a page of its
// own: its head, back to the albums, and only its songs. Going back, from
// its button or the mouse's, shows the grid where it was scrolled.
func TestAlbumOpensAndComesBack(t *testing.T) {
	a := editTestApp()
	a.settings.AlbumGrid = true
	var tracks []netease.Track
	for i := range 60 {
		for k := range 2 {
			tracks = append(tracks, netease.Track{ID: int64(1000 + 2*i + k), Name: fmt.Sprintf("歌%d-%d", i, k),
				Album: netease.Album{Name: fmt.Sprintf("专辑%02d", i)}, Artists: []netease.Artist{{Name: "歌手"}}, Duration: 180000})
		}
	}
	a.playlists[10].Tracks = tracks
	tt := ui.NewTester(a.view, 1200, 800)
	settle := func() {
		for range 6 {
			tt.Advance(time.Second / 4)
			tt.Frame()
		}
	}
	settle()
	a.plView.sort = sortAlbum
	settle()
	if !a.albumGridShown() {
		t.Fatal("the albums do not show as a grid")
	}
	tt.Wheel(700, 400, 0, 1600)
	settle()
	before, _ := a.trackList.Visible()
	if before < 2 {
		t.Fatalf("the wheel left the grid at line %d", before)
	}
	var card ui.Rect
	var name string
	for i := range 60 {
		if r, ok := tt.Find(fmt.Sprintf("专辑%02d", i)); ok && r.Y > 100 && r.Y < 600 {
			card, name = r, fmt.Sprintf("专辑%02d", i)
			break
		}
	}
	if name == "" {
		t.Fatal("no album card in view")
	}
	tt.ClickAt(card.X+card.W/2, card.Y+card.W/3)
	settle()
	if _, ok := tt.Find("返回全部专辑"); !ok {
		t.Fatal("the album opened without its head")
	}
	if g, open := a.albumOpen(); !open || g.name != name || len(a.shownTracks) != 2 {
		t.Fatalf("open %v %q, %d songs", open, g.name, len(a.shownTracks))
	}
	if first, _ := a.trackList.Visible(); first != 0 {
		t.Fatalf("the album opened at row %d, not its top", first)
	}
	a.back()
	settle()
	if _, open := a.albumOpen(); open {
		t.Fatal("back left the album open")
	}
	if after, _ := a.trackList.Visible(); after != before {
		t.Fatalf("the grid came back at line %d, was %d", after, before)
	}
	if _, ok := tt.Find(name); !ok {
		t.Fatalf("the card of %s is out of view after coming back", name)
	}
}

// timeZero is a zero time, for tests that move an album outside a frame.
var timeZero time.Time

// Sorted by album, then by default, then by album again, the albums are
// found again: the switch and the album index stay.
func TestAlbumSortAgain(t *testing.T) {
	a := newTestApp()
	a.route = route{page: pageDownloads}
	song := func(name, album string) netease.Track {
		return netease.Track{Name: name, Album: netease.Album{Name: album}, Artists: []netease.Artist{{Name: "r"}}}
	}
	tracks := []netease.Track{song("1", "X"), song("2", "Y"), song("3", "X")}
	a.listShown(tracks)
	v := &a.plView
	for _, sort := range []int{sortAlbum, 0, sortAlbum} {
		v.sort = sort
		a.listShown(tracks)
	}
	if len(v.all) != 2 || len(v.groups) != 2 {
		t.Fatalf("by album again: %d albums, %d groups", len(v.all), len(v.groups))
	}
}
