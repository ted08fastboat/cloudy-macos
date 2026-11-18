package app

import (
	"errors"
	"log"
	"time"

	"netease-music-mygo/internal/netease"
)

// scrobble tells NetEase of a song played; tests replace it.
var scrobble = netease.Scrobble

// scrobbleAfter is how long a song is listened to before it counts, as a
// skip a few seconds in should not.
const scrobbleAfter = 30 * time.Second

// scrobbleWorks says whether NetEase counts the plays told it, so whether
// the setting shows. The web player's log counted none of them (tried on
// 2026-11-07); the macOS client's, which netease.Scrobble now sends, is
// what other clients found NetEase counts. Turned back off, the setting
// hides and nothing is sent.
var scrobbleWorks = true

// listened tells NetEase that the listener played t for played, if they
// asked for their plays to count there: a NetEase song, signed in, heard
// to its end or for half a minute at least.
func (a *app) listened(t netease.Track, played time.Duration, ended bool) {
	if !scrobbleWorks || !a.settings.Scrobble || a.profile == nil || t.ID <= 0 || t.ProgramID != 0 {
		return
	}
	if !ended && played < scrobbleAfter {
		return
	}
	seconds := int(played.Seconds())
	if ended && t.Duration > 0 {
		seconds = int(t.Duration / 1000)
	}
	go func() {
		// Whether NetEase counted it cannot be known here: it answers
		// success either way, and shows plays minutes later.
		if err := scrobble(t.ID, 0, seconds); err != nil && !errors.Is(err, netease.ErrNotSignedIn) {
			log.Printf("scrobble %d: %v", t.ID, err)
		}
	}()
}
