package netease

import (
	"strconv"
	"strings"
)

// An IntroSection is a part of an artist's introduction: 代表作品, 演艺经历
// and so on, with what it says.
type IntroSection struct {
	Title, Text string
}

// ArtistIntro returns the whole introduction of artist id: the brief one,
// and the parts after it. Either may be empty.
func ArtistIntro(id int64) (brief string, sections []IntroSection, err error) {
	var res struct {
		Code         int    `json:"code"`
		BriefDesc    string `json:"briefDesc"`
		Introduction []struct {
			Ti  string `json:"ti"`
			Txt string `json:"txt"`
		} `json:"introduction"`
	}
	if err := eapiPost("/api/artist/introduction", map[string]any{"id": strconv.FormatInt(id, 10)}, &res); err != nil {
		return "", nil, err
	}
	for _, s := range res.Introduction {
		t, x := strings.TrimSpace(s.Ti), strings.TrimSpace(s.Txt)
		if x != "" {
			sections = append(sections, IntroSection{Title: t, Text: x})
		}
	}
	return strings.TrimSpace(res.BriefDesc), sections, nil
}
