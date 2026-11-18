// Package wordlyrics finds word-timed lyrics for songs NetEase has only
// line-timed lyrics of: the AMLL TTML database first, which keys its
// lyrics by NetEase's song IDs, then Kugou's KRC lyrics, matched by the
// song's name, artist and length.
package wordlyrics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"netease-music-mygo/internal/lyrics"
)

// A Song is what the lyrics are looked up by.
type Song struct {
	ID      int64 // NetEase's
	Name    string
	Artists []string
	Length  time.Duration
}

// client keeps a lookup short: it only stands in for lyrics already shown.
var client = &http.Client{Timeout: 8 * time.Second}

// amllBases are where the AMLL TTML database's NetEase lyrics are read
// from: GitHub, then mirrors of it for where GitHub is slow or blocked.
var amllBases = []string{
	"https://raw.githubusercontent.com/amll-dev/amll-ttml-db/main/ncm-lyrics/",
	"https://gh-proxy.com/https://raw.githubusercontent.com/amll-dev/amll-ttml-db/main/ncm-lyrics/",
	"https://ghfast.top/https://raw.githubusercontent.com/amll-dev/amll-ttml-db/main/ncm-lyrics/",
}

// kugouBase is Kugou's lyrics server; tests replace it.
var kugouBase = "http://lyrics.kugou.com"

// errNone says a source has no lyrics for the song.
var errNone = errors.New("wordlyrics: none")

// Find returns word-timed lyrics of s, and where they are from ("AMLL",
// "酷狗"), or none. Lyrics whose words are all spread evenly are not
// returned: they are no better than NetEase's lines.
func Find(ctx context.Context, s Song) ([]lyrics.Line, string) {
	if s.ID > 0 {
		if lines, err := amll(ctx, s.ID); err == nil && len(lines) > 0 {
			return lines, "AMLL"
		}
	}
	if s.Name != "" && s.Length > 0 {
		if lines, err := kugou(ctx, s); err == nil && len(lines) > 0 && lyrics.Uneven(lines) {
			return lines, "酷狗"
		}
	}
	return nil, ""
}

func get(ctx context.Context, u string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	res, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	return b, res.StatusCode, err
}

// amll reads song id's lyrics from the AMLL TTML database. A 404 from one
// base is the answer: the others mirror it.
func amll(ctx context.Context, id int64) ([]lyrics.Line, error) {
	for _, base := range amllBases {
		b, code, err := get(ctx, fmt.Sprintf("%s%d.ttml", base, id))
		if err != nil || code >= 500 || code == 403 || code == 429 {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		if code != http.StatusOK {
			return nil, errNone
		}
		return lyrics.ParseTTML(b), nil
	}
	return nil, errNone
}

type kugouCandidate struct {
	ID        string `json:"id"`
	AccessKey string `json:"accesskey"`
	Song      string `json:"song"`
	Singer    string `json:"singer"`
	Duration  int64  `json:"duration"`
}

// kugou finds s among Kugou's lyrics: a candidate of the same name, within
// three seconds of its length, by one of its artists if Kugou names any.
func kugou(ctx context.Context, s Song) ([]lyrics.Line, error) {
	keyword := s.Name
	if len(s.Artists) > 0 {
		keyword = strings.Join(s.Artists, "、") + " - " + s.Name
	}
	q := url.Values{"ver": {"1"}, "man": {"yes"}, "client": {"pc"}, "keyword": {keyword},
		"duration": {fmt.Sprint(s.Length.Milliseconds())}, "hash": {""}}
	b, code, err := get(ctx, kugouBase+"/search?"+q.Encode())
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, errNone
	}
	var res struct {
		Candidates []kugouCandidate `json:"candidates"`
	}
	if err := json.Unmarshal(b, &res); err != nil {
		return nil, err
	}
	c := pick(res.Candidates, s)
	if c == nil {
		return nil, errNone
	}
	q = url.Values{"ver": {"1"}, "client": {"pc"}, "id": {c.ID}, "accesskey": {c.AccessKey}, "fmt": {"krc"}, "charset": {"utf8"}}
	b, code, err = get(ctx, kugouBase+"/download?"+q.Encode())
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, errNone
	}
	var dl struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(b, &dl); err != nil || dl.Content == "" {
		return nil, errNone
	}
	text, err := lyrics.DecodeKRC(dl.Content)
	if err != nil {
		return nil, err
	}
	return trimKugou(lyrics.ParseKRC(text), s), nil
}

// trimKugou leaves out the line Kugou's lyrics open with, "artist - name",
// and the note on who translated them that stands as the first
// translation.
func trimKugou(lines []lyrics.Line, s Song) []lyrics.Line {
	if len(lines) > 0 && strings.Contains(lines[0].Text, " - ") && strings.Contains(fold(lines[0].Text), fold(s.Name)) {
		lines = lines[1:]
	}
	for i := range lines {
		if t := lines[i].Translation; strings.Contains(t, "翻译由") || strings.Contains(t, "大模型") {
			lines[i].Translation = ""
		}
	}
	return lines
}

// pick is the first of cs that is s: Kugou lists the likeliest first, but
// also songs of other names and lengths.
func pick(cs []kugouCandidate, s Song) *kugouCandidate {
	name := fold(s.Name)
	for i := range cs {
		c := &cs[i]
		gap := time.Duration(c.Duration)*time.Millisecond - s.Length
		if gap < -3*time.Second || gap > 3*time.Second || c.AccessKey == "" {
			continue
		}
		if fold(c.Song) != name {
			continue
		}
		if len(s.Artists) > 0 && c.Singer != "" && !sharesArtist(c.Singer, s.Artists) {
			continue
		}
		return c
	}
	return nil
}

func sharesArtist(singer string, artists []string) bool {
	got := fold(singer)
	for _, a := range artists {
		if f := fold(a); f != "" && strings.Contains(got, f) {
			return true
		}
	}
	return false
}

// fold is a name as compared: lower case, without spaces, punctuation or
// what follows its first bracket ("(feat. …)", "(Live)"): a live take or
// an accompaniment is told apart by its length instead.
func fold(s string) string {
	s = strings.ToLower(s)
	if i := strings.IndexAny(s, "(（[【"); i > 0 {
		s = s[:i]
	}
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
