package netease

import (
	"encoding/json"
	"testing"
)

func TestSongWikiMemoryDetail(t *testing.T) {
	var res wikiPage
	json.Unmarshal([]byte(`{"data":{"blocks":[{"code":"SONG_PLAY_ABOUT_MUSIC_MEMORY","creatives":[{"resources":[
		{"resourceType":"FIRST_LISTEN","resourceExt":{"musicFirstListenDto":{"date":"2022.02.26 08:15","season":"隆冬","period":"上午"}},"uiElement":{"mainTitle":{"title":"第一次听"}}},
		{"resourceType":"TOTAL_PLAY","resourceExt":{"musicTotalPlayDto":{"playCount":7,"text":"如同唱片旋转945圈"}},"uiElement":{"mainTitle":{"title":"累计播放"}}},
		{"resourceType":"TOTAL_PLAY","resourceExt":{"musicTotalPlayDto":{"playCount":0}},"uiElement":{"mainTitle":{"title":"空的"}}}
	]}]}]}}`), &res)
	w := parseSongWiki(&res)
	want := []WikiLine{{"第一次听", "2022.02.26 08:15 · 隆冬上午"}, {"累计播放", "7 次 · 如同唱片旋转945圈"}}
	if len(w.Memory) != 2 || w.Memory[0] != want[0] || w.Memory[1] != want[1] {
		t.Fatalf("memory = %+v", w.Memory)
	}
}

func TestSongWikiInfo(t *testing.T) {
	var res wikiInfoPage
	err := json.Unmarshal([]byte(`{"code":200,"data":{"blocks":[
	{"bizCode":"songDetailNewSongWiki","rnData":{"albumId":18905,"albumName":"叶惠美","coverImage":"c.jpg","blocks":[
		{"blockCode":"wikiSubBlockSongInfoVo","blockInfo":{"desc":"《晴天》是…","wikiSubElementVos":[{"title":"作词","wikiSubMetaVos":[{"id":6452,"text":"周杰伦"}]},{"title":"吉他","wikiSubMetaVos":[{"text":"蔡科俊Again"}]}]}},
		{"blockCode":"wikiSubBlockBaseInfoVo","blockInfo":{"wikiSubElementVos":[{"title":"曲风","wikiSubMetaVos":[{"id":1020,"text":"流行-华语流行"}]},{"title":"发行时间","content":"2003-07-31"},{"title":"乐器","content":"含电吉他、原声吉他"}]}},
		{"blockCode":"wikiSubBlockAlbumAwardRecordVo","blockInfo":{"wikiSubElementVos":[{"title":"第15届金曲奖","subTitle":"最佳流行音乐演唱专辑奖"}]}},
		{"blockCode":"wikiSubBlockMvInfoVo","blockInfo":null}]}},
	{"bizCode":"songWikiArtistInfo","rnData":{"artists":[{"artistId":6452,"artistName":"周杰伦","artistCoverUrl":"a.jpg","briefDesc":" 周杰伦（Jay Chou）… ","artistIdentifyDto":{"roleNames":["歌手","作曲"]}}]}},
	{"bizCode":"songWikiMoreVersions","rnData":{"moreVersionSongs":[{"id":1,"name":"晴天 (翻唱版)","artistNames":["原始之音"]}]}},
	{"bizCode":"songWikiSimPlaylist","rnData":{"simPlaylist":[{"playlistId":9,"name":"十万加","coverImgUrl":"p.jpg"}]}}
	]}}`), &res)
	if err != nil {
		t.Fatal(err)
	}
	w := &SongWiki{Facts: []WikiFact{{"BPM", []string{"69"}}}}
	w.addInfo(&res)
	if w.Album == nil || w.Album.ID != 18905 || w.Album.CoverURL != "c.jpg" {
		t.Fatalf("album = %+v", w.Album)
	}
	if w.Intro != "《晴天》是…" || len(w.Credits) != 2 || w.Credits[0].People[0] != (WikiPerson{6452, "周杰伦"}) {
		t.Fatalf("credits = %+v", w.Credits)
	}
	if len(w.Facts) != 3 || w.Facts[1].Label != "发行时间" || w.Facts[2].Values[0] != "含电吉他、原声吉他" {
		t.Fatalf("facts = %+v", w.Facts)
	}
	if len(w.AlbumAwards) != 1 || len(w.Artists) != 1 || w.Artists[0].Brief != "周杰伦（Jay Chou）…" || len(w.Artists[0].Roles) != 2 {
		t.Fatalf("awards %+v artists %+v", w.AlbumAwards, w.Artists)
	}
	if len(w.Versions) != 1 || len(w.Playlists) != 1 || w.Playlists[0].CoverURL != "p.jpg" {
		t.Fatalf("versions %+v playlists %+v", w.Versions, w.Playlists)
	}
}
