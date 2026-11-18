package netease

import (
	"encoding/json"
	"testing"
)

func TestParseSongWiki(t *testing.T) {
	const page = `{"code":200,"data":{"blocks":[
	{"code":"SONG_PLAY_ABOUT_SONG_BASIC","creatives":[
		{"creativeType":"songTag","uiElement":{"mainTitle":{"title":"曲风"}},"resources":[{"uiElement":{"mainTitle":{"title":"流行-华语流行"}}}]},
		{"creativeType":"language","uiElement":{"mainTitle":{"title":"语种"},"textLinks":[{"text":"国语"}]}},
		{"creativeType":"songAward","uiElement":{"mainTitle":{"title":"获奖成就"}},"resources":[{"uiElement":{"mainTitle":{"title":"东方风云榜"},"subTitles":[{"title":"获"},{"title":"年度金曲"}]}}]},
		{"creativeType":"entertainment","uiElement":{"mainTitle":{"title":"影视节目"}},"resources":[{"uiElement":{"mainTitle":{"title":"动画主题曲"}}}]},
		{"creativeType":"sheet","uiElement":{"mainTitle":{"title":"乐谱"},"buttons":[{"text":"25个"}]},"resources":[{},{}]},
		{"creativeType":"songComment","uiElement":{"mainTitle":{"title":"乐评"}},"resources":[{"uiElement":{"mainTitle":{"title":"乐评来自 某人"},"descriptions":[{"description":"好听"}]}}]}
	]},
	{"code":"SONG_PLAY_ABOUT_SIMILAR_SONG","creatives":[{"resources":[{"resourceType":"SONG","resourceId":"7","uiElement":{"mainTitle":{"title":"像它的歌"},"subTitles":[{"title":"某歌手"}],"images":[{"imageUrl":"http://x/a.jpg"}]}}]}]},
	{"code":"SONG_PLAY_ABOUT_RELATED_PLAYLIST","creatives":[{"resources":[{"resourceType":"PLAYLIST","resourceId":"9","uiElement":{"mainTitle":{"title":"歌单"}}}]}]}
	]}}`
	var res wikiPage
	if err := json.Unmarshal([]byte(page), &res); err != nil {
		t.Fatal(err)
	}
	w := parseSongWiki(&res)
	if len(w.Facts) != 2 || w.Facts[0].Label != "曲风" || w.Facts[1].Values[0] != "国语" {
		t.Errorf("facts = %+v", w.Facts)
	}
	if len(w.Awards) != 1 || w.Awards[0].Detail != "获 年度金曲" {
		t.Errorf("awards = %+v", w.Awards)
	}
	if len(w.Shows) != 1 || w.Sheets != 25 {
		t.Errorf("shows = %v, sheets = %d", w.Shows, w.Sheets)
	}
	if w.Review == nil || w.Review.By != "某人" || w.Review.Text != "好听" {
		t.Errorf("review = %+v", w.Review)
	}
	if len(w.Similar) != 1 || w.Similar[0].ID != 7 || w.Similar[0].CoverURL == "" {
		t.Errorf("similar = %+v", w.Similar)
	}
	if len(w.Playlists) != 1 || w.Playlists[0].ID != 9 {
		t.Errorf("playlists = %+v", w.Playlists)
	}
	if w.Empty() || (&SongWiki{}).Empty() == false {
		t.Error("Empty is wrong")
	}
}
