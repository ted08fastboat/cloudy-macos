package netease

import (
	"fmt"
	"net/url"
)

// Ranked is a song of the listener's ranking: how many times they played
// it, and NetEase's score of it, 100 for the most played.
type Ranked struct {
	Track Track
	Plays int
	Score int
}

// WeekRanking returns the songs the listener uid played most in the last
// seven days, the most played first: NetEase's 听歌排行 of the week.
func WeekRanking(uid int64) ([]Ranked, error) {
	var res struct {
		WeekData []struct {
			PlayCount int   `json:"playCount"`
			Score     int   `json:"score"`
			Song      Track `json:"song"`
		} `json:"weekData"`
	}
	form := url.Values{"uid": {fmt.Sprint(uid)}, "type": {"1"}}
	if err := post("/api/v1/play/record", form, &res); err != nil {
		return nil, err
	}
	out := make([]Ranked, len(res.WeekData))
	songs := make([]Track, len(out))
	for i, w := range res.WeekData {
		songs[i] = w.Song
		out[i] = Ranked{Plays: w.PlayCount, Score: w.Score}
	}
	// They come without privileges, as the daily songs do.
	detailPrivileges(songs)
	for i := range out {
		out[i].Track = songs[i]
	}
	return out, nil
}
