package app

import (
	"context"
	"sync/atomic"
	"time"

	"netease-music-mygo/internal/lyrics"
	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/player"
	"netease-music-mygo/internal/wordlyrics"
)

// moreWordsOn is settings.MoreWords where the lyrics are fetched, off the
// main thread.
var moreWordsOn atomic.Bool

// findMoreWords is wordlyrics.Find; tests replace it.
var findMoreWords = wordlyrics.Find

// useMoreWords has the player look for word-timed lyrics elsewhere, for
// songs NetEase times only by line, while the setting is on.
func (a *app) useMoreWords() {
	moreWordsOn.Store(a.settings.MoreWords)
	player.MoreWords = func(t netease.Track) []lyrics.Line {
		if !moreWordsOn.Load() {
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var artists []string
		for _, ar := range t.Artists {
			artists = append(artists, ar.Name)
		}
		lines, _ := findMoreWords(ctx, wordlyrics.Song{ID: t.ID, Name: t.Name, Artists: artists, Length: t.Length()})
		return lines
	}
}
