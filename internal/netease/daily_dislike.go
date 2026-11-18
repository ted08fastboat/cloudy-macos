package netease

import (
	"fmt"
	"strconv"
)

// DailyDislike tells the day's recommendation the listener is not into a
// song of it (不感兴趣), and returns the song NetEase puts in its place,
// with ok false when it gives none.
func DailyDislike(id int64) (song Track, ok bool, err error) {
	var res struct {
		Code int    `json:"code"`
		Data *Track `json:"data"`
	}
	data := map[string]any{"resId": strconv.FormatInt(id, 10), "resType": 4, "sceneType": 1}
	if err := eapiPost("/api/v2/discovery/recommend/dislike", data, &res); err != nil {
		return Track{}, false, err
	}
	if res.Code != 200 {
		return Track{}, false, fmt.Errorf("netease: daily dislike: code %d", res.Code)
	}
	if res.Data == nil || res.Data.ID == 0 {
		return Track{}, false, nil
	}
	songs := []Track{*res.Data}
	detailPrivileges(songs)
	return songs[0], true, nil
}
