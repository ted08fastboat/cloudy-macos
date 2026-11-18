package netease

import (
	"encoding/json"
	"slices"
	"strings"
)

// The song wiki as NetEase's own apps show it (歌曲百科): its details,
// who made it, its awards and those of its album, its artists, and other
// versions of it. The PC client asks for it as a page of blocks.

// A WikiPerson is someone credited with a song, with their NetEase artist
// ID if they have one.
type WikiPerson struct {
	ID   int64
	Name string
}

// A WikiCredit is a part in making a song (作词, 吉他, 混音) and who had it.
type WikiCredit struct {
	Role   string
	People []WikiPerson
}

// A WikiArtist is an artist of the song, as 关于艺人 introduces them.
type WikiArtist struct {
	ID        int64
	Name      string
	AvatarURL string
	Roles     []string // 歌手, 作曲, 制作人…
	Brief     string
}

type wikiInfoPage struct {
	Code int `json:"code"`
	Data struct {
		Blocks []struct {
			BizCode string          `json:"bizCode"`
			RnData  json.RawMessage `json:"rnData"`
		} `json:"blocks"`
	} `json:"data"`
}

type wikiSubMeta struct {
	ID   int64  `json:"id"`
	Text string `json:"text"`
}

type wikiSubElement struct {
	Title    string        `json:"title"`
	SubTitle string        `json:"subTitle"`
	Content  string        `json:"content"`
	Metas    []wikiSubMeta `json:"wikiSubMetaVos"`
}

type wikiDetail struct {
	AlbumID    int64  `json:"albumId"`
	AlbumName  string `json:"albumName"`
	CoverImage string `json:"coverImage"`
	Blocks     []struct {
		BlockCode string `json:"blockCode"`
		BlockInfo *struct {
			Desc     string           `json:"desc"`
			Elements []wikiSubElement `json:"wikiSubElementVos"`
		} `json:"blockInfo"`
	} `json:"blocks"`
}

type wikiArtists struct {
	Artists []struct {
		ArtistID   int64  `json:"artistId"`
		ArtistName string `json:"artistName"`
		CoverURL   string `json:"artistCoverUrl"`
		BriefDesc  string `json:"briefDesc"`
		Identify   *struct {
			RoleNames []string `json:"roleNames"`
		} `json:"artistIdentifyDto"`
	} `json:"artists"`
}

type wikiVersions struct {
	Songs []struct {
		ID          int64    `json:"id"`
		Name        string   `json:"name"`
		ArtistNames []string `json:"artistNames"`
	} `json:"moreVersionSongs"`
}

type wikiSimPlaylists struct {
	Playlists []struct {
		ID       int64  `json:"playlistId"`
		Name     string `json:"name"`
		CoverURL string `json:"coverImgUrl"`
	} `json:"simPlaylist"`
}

type wikiSimSongs struct {
	Songs []struct {
		ID      int64  `json:"id"`
		Name    string `json:"name"`
		Artists []struct {
			Name string `json:"name"`
		} `json:"artists"`
	} `json:"simSongs"`
}

// songWikiInfo asks for the wiki of song id as the PC client's 歌曲百科.
func songWikiInfo(id int64) (*wikiInfoPage, error) {
	ext, _ := json.Marshal(map[string]any{"states": map[string]any{"playingResource": map[string]any{"current": id, "scene": "songWiki"}}})
	var res wikiInfoPage
	err := eapiPost("/api/link/page/parent/relation/construct/info", map[string]any{
		"extJson": string(ext), "positionCode": "songWikiMainPosition",
	}, &res)
	return &res, err
}

// addInfo puts what the 歌曲百科 page says in w: its details in place of
// the older wiki's basics, which say less; the songs and playlists like it
// only where the older wiki, whose songs have covers, had none.
func (w *SongWiki) addInfo(res *wikiInfoPage) {
	for _, b := range res.Data.Blocks {
		switch b.BizCode {
		case "songDetailNewSongWiki":
			var d wikiDetail
			if json.Unmarshal(b.RnData, &d) == nil {
				w.addDetail(d)
			}
		case "songWikiArtistInfo":
			var d wikiArtists
			if json.Unmarshal(b.RnData, &d) != nil {
				continue
			}
			for _, ar := range d.Artists {
				if ar.ArtistName == "" {
					continue
				}
				a := WikiArtist{ID: ar.ArtistID, Name: ar.ArtistName, AvatarURL: ar.CoverURL, Brief: strings.TrimSpace(ar.BriefDesc)}
				if ar.Identify != nil {
					a.Roles = ar.Identify.RoleNames
				}
				w.Artists = append(w.Artists, a)
			}
		case "songWikiMoreVersions":
			var d wikiVersions
			if json.Unmarshal(b.RnData, &d) != nil {
				continue
			}
			for _, s := range d.Songs {
				if s.ID > 0 && s.Name != "" {
					w.Versions = append(w.Versions, WikiSong{ID: s.ID, Name: s.Name, Artist: strings.Join(s.ArtistNames, "/")})
				}
			}
		case "songWikiSimPlaylist":
			var d wikiSimPlaylists
			if len(w.Playlists) > 0 || json.Unmarshal(b.RnData, &d) != nil {
				continue
			}
			for _, p := range d.Playlists {
				if p.ID > 0 && p.Name != "" {
					w.Playlists = append(w.Playlists, WikiPlaylist{ID: p.ID, Name: p.Name, CoverURL: p.CoverURL})
				}
			}
		case "songWikiSimSongs":
			var d wikiSimSongs
			if len(w.Similar) > 0 || json.Unmarshal(b.RnData, &d) != nil {
				continue
			}
			for _, s := range d.Songs {
				var names []string
				for _, ar := range s.Artists {
					names = append(names, ar.Name)
				}
				if len(names) > 3 {
					names = append(names[:3], "等")
				}
				if s.ID > 0 && s.Name != "" {
					w.Similar = append(w.Similar, WikiSong{ID: s.ID, Name: s.Name, Artist: strings.Join(names, "/")})
				}
			}
		}
	}
}

// addCredit adds c, to the part of the same name if there is one already:
// NetEase lists each person of a part on their own (作词 four times over).
func (w *SongWiki) addCredit(c WikiCredit) {
	for i := range w.Credits {
		if w.Credits[i].Role != c.Role {
			continue
		}
		for _, p := range c.People {
			if !slices.Contains(w.Credits[i].People, p) {
				w.Credits[i].People = append(w.Credits[i].People, p)
			}
		}
		return
	}
	w.Credits = append(w.Credits, c)
}

func (w *SongWiki) addDetail(d wikiDetail) {
	if d.AlbumID > 0 && d.AlbumName != "" {
		w.Album = &WikiSong{ID: d.AlbumID, Name: d.AlbumName, CoverURL: d.CoverImage}
	}
	var facts []WikiFact
	var awards []WikiLine
	for _, blk := range d.Blocks {
		info := blk.BlockInfo
		if info == nil {
			continue
		}
		switch blk.BlockCode {
		case "wikiSubBlockSongInfoVo":
			w.Intro = strings.TrimSpace(info.Desc)
			for _, e := range info.Elements {
				c := WikiCredit{Role: strings.TrimSpace(e.Title)}
				for _, m := range e.Metas {
					if t := strings.TrimSpace(m.Text); t != "" {
						c.People = append(c.People, WikiPerson{ID: m.ID, Name: t})
					}
				}
				if c.Role != "" && len(c.People) > 0 {
					w.addCredit(c)
				}
			}
		case "wikiSubBlockBaseInfoVo":
			for _, e := range info.Elements {
				f := WikiFact{Label: strings.TrimSpace(e.Title)}
				for _, m := range e.Metas {
					if t := strings.TrimSpace(m.Text); t != "" {
						f.Values = append(f.Values, t)
					}
				}
				if t := strings.TrimSpace(e.Content); t != "" && len(f.Values) == 0 {
					f.Values = []string{t}
				}
				if f.Label != "" && len(f.Values) > 0 {
					facts = append(facts, f)
				}
			}
		case "wikiSubBlockAwardRecordVo":
			for _, e := range info.Elements {
				if t := strings.TrimSpace(e.Title); t != "" {
					awards = append(awards, WikiLine{t, strings.TrimSpace(e.SubTitle)})
				}
			}
		case "wikiSubBlockAlbumAwardRecordVo":
			for _, e := range info.Elements {
				if t := strings.TrimSpace(e.Title); t != "" {
					w.AlbumAwards = append(w.AlbumAwards, WikiLine{t, strings.TrimSpace(e.SubTitle)})
				}
			}
		case "wikiSubBlockEntertainmentVo":
			var shows []string
			for _, e := range info.Elements {
				if t := strings.TrimSpace(e.Title); t != "" {
					shows = append(shows, t)
				}
			}
			if len(shows) > 0 {
				w.Shows = shows
			}
		}
	}
	if len(facts) > 0 {
		w.Facts = facts
	}
	if len(awards) > 0 {
		w.Awards = awards
	}
}
