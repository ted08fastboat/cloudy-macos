package app

import (
	"testing"

	"netease-music-mygo/internal/download"
	"netease-music-mygo/internal/netease"
)

// The downloads page asks for its songs every frame: they are made once
// for each change of the downloads, the same slice meanwhile, so that the
// search and the sort of the page keep their cache.
func TestDownloadTracksKept(t *testing.T) {
	a := newTestApp()
	a.dl.items = []download.Item{{Track: netease.Track{ID: 1, Name: "一"}, Path: `C:\m\1.mp3`}}
	a.noteDownloads()
	first := a.downloadTracks()
	if again := a.downloadTracks(); &again[0] != &first[0] {
		t.Fatal("made again with nothing changed")
	}
	a.dl.items = append(a.dl.items, download.Item{Track: netease.Track{ID: 2, Name: "二"}, Path: `C:\m\2.mp3`})
	a.noteDownloads()
	if got := a.downloadTracks(); len(got) != 2 || got[1].LocalPath != `C:\m\2.mp3` {
		t.Fatalf("after a download: %+v", got)
	}
	// Changed in place, as tagging a file does.
	a.dl.items[0].Track.Name = "壹"
	a.dl.rev++
	if got := a.downloadTracks(); got[0].Name != "壹" {
		t.Errorf("after a change in place: %q", got[0].Name)
	}
}

// The progress of a download or an upload redraws the window only where
// it shows: its page, under no Now Playing, or its queue's window.
func TestJobProgressShown(t *testing.T) {
	a := newTestApp()
	a.route.page = pageDownloads
	if !a.downloadProgressShown() || a.uploadProgressShown() {
		t.Error("on the downloads page")
	}
	a.nowPlaying = true
	if a.downloadProgressShown() {
		t.Error("under Now Playing")
	}
	a.dl.view.open = true
	if !a.downloadProgressShown() {
		t.Error("in the queue's window")
	}
	a.nowPlaying, a.dl.view.open = false, false
	a.route.page = pageCloud
	if a.downloadProgressShown() || !a.uploadProgressShown() {
		t.Error("on the cloud drive's page")
	}
}
