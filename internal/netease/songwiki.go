package netease

import (
	"strconv"
	"strings"
)

// A SongWiki is what NetEase's song wiki (音乐百科) says of a song: facts
// such as its genres, language and tempo, its awards and the shows it was
// in, a review, what the listener's own history with it is, and songs and
// playlists like it. Many songs have only some of it, or none.
type SongWiki struct {
	Album       *WikiSong // the album it is on, its cover the album's
	Intro       string    // what the wiki says of its making
	Credits     []WikiCredit
	AlbumAwards []WikiLine
	Artists     []WikiArtist
	Versions    []WikiSong // other recordings and covers of it
	Facts       []WikiFact
	Awards      []WikiLine
	Shows       []string
	Sheets      int // how many scores NetEase has of it
	Review      *WikiReview
	Memory      []WikiLine
	Similar     []WikiSong
	Playlists   []WikiPlaylist
}

// Empty reports whether the wiki says nothing of its song.
func (w *SongWiki) Empty() bool {
	return w == nil || len(w.Facts) == 0 && len(w.Awards) == 0 && len(w.Shows) == 0 && w.Sheets == 0 &&
		w.Review == nil && len(w.Memory) == 0 && len(w.Similar) == 0 && len(w.Playlists) == 0 &&
		w.Album == nil && w.Intro == "" && len(w.Credits) == 0 && len(w.Artists) == 0 && len(w.Versions) == 0
}

// A WikiFact is one fact with its values: 曲风 and its genres, 语种 and
// the language.
type WikiFact struct {
	Label  string
	Values []string
}

// A WikiLine is a line with what qualifies it: an award and what it was
// for, or a moment of the listener's history.
type WikiLine struct {
	Title, Detail string
}

// A WikiReview is a review of the song, and who wrote it.
type WikiReview struct {
	By, Text string
}

// A WikiSong is a song like the one the wiki is of.
type WikiSong struct {
	ID           int64
	Name, Artist string
	CoverURL     string
}

// A WikiPlaylist is a playlist with the song in it.
type WikiPlaylist struct {
	ID             int64
	Name, CoverURL string
}

type wikiTitle struct {
	Title string `json:"title"`
}

type wikiElement struct {
	MainTitle *wikiTitle  `json:"mainTitle"`
	SubTitles []wikiTitle `json:"subTitles"`
	TextLinks []struct {
		Text string `json:"text"`
	} `json:"textLinks"`
	Descriptions []struct {
		Description string `json:"description"`
	} `json:"descriptions"`
	Images []struct {
		Title    string `json:"title"`
		ImageURL string `json:"imageUrl"`
	} `json:"images"`
	Buttons []struct {
		Text string `json:"text"`
	} `json:"buttons"`
}

func (e wikiElement) title() string {
	if e.MainTitle == nil {
		return ""
	}
	return strings.TrimSpace(e.MainTitle.Title)
}

func (e wikiElement) subtitles() string {
	var parts []string
	for _, s := range e.SubTitles {
		if t := strings.TrimSpace(s.Title); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, " ")
}

func (e wikiElement) description() string {
	var parts []string
	for _, d := range e.Descriptions {
		if t := strings.TrimSpace(d.Description); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, "\n")
}

type wikiResource struct {
	ResourceType string      `json:"resourceType"`
	ResourceID   string      `json:"resourceId"`
	UIElement    wikiElement `json:"uiElement"`
	// ResourceExt holds what the moments of the listener's history say:
	// their titles alone are only the labels.
	ResourceExt struct {
		FirstListen *struct {
			Date   string `json:"date"`
			Season string `json:"season"`
			Period string `json:"period"`
		} `json:"musicFirstListenDto"`
		TotalPlay *struct {
			PlayCount int    `json:"playCount"`
			Text      string `json:"text"`
		} `json:"musicTotalPlayDto"`
	} `json:"resourceExt"`
}

// memoryDetail says what a moment of the listener's history holds, or
// "" when NetEase sent nothing to say.
func (r wikiResource) memoryDetail() string {
	ext := r.ResourceExt
	switch {
	case ext.FirstListen != nil && strings.TrimSpace(ext.FirstListen.Date) != "":
		f := ext.FirstListen
		d := strings.TrimSpace(f.Date)
		if f.Season != "" || f.Period != "" {
			d += " · " + f.Season + f.Period
		}
		return d
	case ext.TotalPlay != nil && ext.TotalPlay.PlayCount > 0:
		d := strconv.Itoa(ext.TotalPlay.PlayCount) + " 次"
		if t := strings.TrimSpace(ext.TotalPlay.Text); t != "" {
			d += " · " + t
		}
		return d
	}
	return ""
}

type wikiCreative struct {
	CreativeType string         `json:"creativeType"`
	UIElement    wikiElement    `json:"uiElement"`
	Resources    []wikiResource `json:"resources"`
}

type wikiPage struct {
	Code int `json:"code"`
	Data struct {
		Blocks []struct {
			Code      string         `json:"code"`
			Creatives []wikiCreative `json:"creatives"`
		} `json:"blocks"`
	} `json:"data"`
}

// SongWikiOf returns the wiki of song id.
// It asks for both of NetEase's wikis at once: the player's 音乐百科, which
// has the listener's history with the song and covers of the songs like it,
// and the 歌曲百科, which has more of the song. Either alone will do.
func SongWikiOf(id int64) (*SongWiki, error) {
	infoDone := make(chan struct{})
	var info *wikiInfoPage
	var infoErr error
	go func() {
		defer close(infoDone)
		info, infoErr = songWikiInfo(id)
	}()
	var res wikiPage
	err := eapiPost("/api/song/play/about/block/page", map[string]any{"songId": strconv.FormatInt(id, 10)}, &res)
	<-infoDone
	if err != nil && infoErr != nil {
		return nil, err
	}
	w := &SongWiki{}
	if err == nil {
		w = parseSongWiki(&res)
	}
	if infoErr == nil {
		w.addInfo(info)
	}
	return w, nil
}

func parseSongWiki(res *wikiPage) *SongWiki {
	w := &SongWiki{}
	for _, b := range res.Data.Blocks {
		switch b.Code {
		case "SONG_PLAY_ABOUT_SONG_BASIC":
			for _, c := range b.Creatives {
				w.addBasic(c)
			}
		case "SONG_PLAY_ABOUT_MUSIC_MEMORY":
			for _, c := range b.Creatives {
				if t := c.UIElement.title(); t != "" && len(c.Resources) == 0 {
					w.Memory = append(w.Memory, WikiLine{t, c.UIElement.subtitles()})
				}
				for _, r := range c.Resources {
					line := WikiLine{r.UIElement.title(), r.memoryDetail()}
					if line.Detail == "" {
						line.Detail = r.UIElement.subtitles()
					}
					if line.Detail == "" {
						line.Detail = r.UIElement.description()
					}
					// A label with nothing under it reads as a fact (a bare
					// 第一次听 looked like this was the first time), so
					// only moments with something to say are shown.
					if line.Title != "" && line.Detail != "" {
						w.Memory = append(w.Memory, line)
					}
				}
			}
		case "SONG_PLAY_ABOUT_SIMILAR_SONG":
			for _, c := range b.Creatives {
				for _, r := range c.Resources {
					id, _ := strconv.ParseInt(r.ResourceID, 10, 64)
					if r.ResourceType != "SONG" || id == 0 {
						continue
					}
					s := WikiSong{ID: id, Name: r.UIElement.title(), Artist: r.UIElement.subtitles()}
					if len(r.UIElement.Images) > 0 {
						s.CoverURL = r.UIElement.Images[0].ImageURL
					}
					w.Similar = append(w.Similar, s)
				}
			}
		case "SONG_PLAY_ABOUT_RELATED_PLAYLIST":
			for _, c := range b.Creatives {
				for _, r := range c.Resources {
					id, _ := strconv.ParseInt(r.ResourceID, 10, 64)
					if r.ResourceType != "PLAYLIST" || id == 0 {
						continue
					}
					p := WikiPlaylist{ID: id, Name: r.UIElement.title()}
					if len(r.UIElement.Images) > 0 {
						p.CoverURL = r.UIElement.Images[0].ImageURL
					}
					w.Playlists = append(w.Playlists, p)
				}
			}
		}
	}
	return w
}

// addBasic adds what one part of the wiki's basics says.
func (w *SongWiki) addBasic(c wikiCreative) {
	label := c.UIElement.title()
	switch c.CreativeType {
	case "songAward":
		for _, r := range c.Resources {
			if t := r.UIElement.title(); t != "" {
				w.Awards = append(w.Awards, WikiLine{t, r.UIElement.subtitles()})
			}
		}
		return
	case "entertainment":
		for _, r := range c.Resources {
			if t := r.UIElement.title(); t != "" {
				w.Shows = append(w.Shows, t)
			}
		}
		return
	case "sheet":
		w.Sheets = len(c.Resources)
		for _, b := range c.UIElement.Buttons {
			if n, err := strconv.Atoi(strings.TrimRight(b.Text, "个首份张")); err == nil && n > w.Sheets {
				w.Sheets = n
			}
		}
		return
	case "songComment":
		for _, r := range c.Resources {
			if text := r.UIElement.description(); text != "" {
				by := strings.TrimSpace(strings.TrimPrefix(r.UIElement.title(), "乐评来自"))
				w.Review = &WikiReview{By: by, Text: text}
				return
			}
		}
		return
	}
	if label == "" {
		return
	}
	var values []string
	for _, l := range c.UIElement.TextLinks {
		if t := strings.TrimSpace(l.Text); t != "" {
			values = append(values, t)
		}
	}
	for _, r := range c.Resources {
		if t := r.UIElement.title(); t != "" {
			values = append(values, t)
		}
	}
	if len(values) > 0 {
		w.Facts = append(w.Facts, WikiFact{Label: label, Values: values})
	}
}
