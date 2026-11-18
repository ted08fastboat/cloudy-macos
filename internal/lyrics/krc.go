package lyrics

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// krcKey is what Kugou's KRC lyrics are XORed with, after their "krc1".
var krcKey = []byte{64, 71, 97, 119, 94, 50, 116, 71, 81, 54, 49, 45, 206, 210, 110, 105}

// DecodeKRC turns a KRC file, as Kugou's lyrics server sends it (base64 of
// "krc1", then the XORed zlib stream), into its text.
func DecodeKRC(content string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(content))
	if err != nil {
		return "", err
	}
	if len(raw) < 5 || string(raw[:4]) != "krc1" {
		return "", errors.New("lyrics: not a KRC file")
	}
	raw = raw[4:]
	for i := range raw {
		raw[i] ^= krcKey[i%len(krcKey)]
	}
	z, err := zlib.NewReader(bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	defer z.Close()
	b, err := io.ReadAll(io.LimitReader(z, 4<<20))
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(string(b), string(rune(0xfeff))), nil
}

var (
	krcLine = regexp.MustCompile(`^\[(\d+),(\d+)\](.*)$`)
	krcWord = regexp.MustCompile(`<(\d+),(\d+),\d+>([^<]*)`)
	krcLang = regexp.MustCompile(`^\[language:([^\]]*)\]`)
)

// ParseKRC reads the text of a KRC file: "[start,length]" a line, then
// "<offset,length,0>" before each of its words, in milliseconds, the
// offsets from the line's start. Its translations come base64'd as JSON in
// a [language:] tag, a line for each line of the lyrics.
func ParseKRC(text string) []Line {
	var out []Line
	var trans []string
	for _, raw := range strings.Split(text, "\n") {
		raw = strings.TrimSpace(raw)
		if m := krcLang.FindStringSubmatch(raw); m != nil {
			trans = krcTranslations(m[1])
			continue
		}
		m := krcLine.FindStringSubmatch(raw)
		if m == nil {
			continue
		}
		start, _ := strconv.Atoi(m[1])
		length, _ := strconv.Atoi(m[2])
		l := Line{At: msec(start), End: msec(start + length), Timed: true}
		for _, w := range krcWord.FindAllStringSubmatch(m[3], -1) {
			off, _ := strconv.Atoi(w[1])
			d, _ := strconv.Atoi(w[2])
			if w[3] == "" {
				continue
			}
			l.Words = append(l.Words, Word{Text: w[3], Start: msec(start + off), End: msec(start + off + d)})
		}
		var b strings.Builder
		for _, w := range l.Words {
			b.WriteString(w.Text)
		}
		l.Text = strings.TrimSpace(b.String())
		out = append(out, l)
	}
	// Translations are by the lines of the file, those empty too.
	for i := range out {
		if i < len(trans) {
			out[i].Translation = strings.TrimSpace(trans[i])
		}
	}
	kept := out[:0]
	for _, l := range out {
		if l.Text != "" {
			kept = append(kept, l)
		}
	}
	return kept
}

func msec(n int) time.Duration { return time.Duration(n) * time.Millisecond }

// krcTranslations reads the translations out of a KRC's [language:] tag,
// one string a line: those of type 1, the translation (0 is romanization).
func krcTranslations(b64 string) []string {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return nil
	}
	var lang struct {
		Content []struct {
			Type         int        `json:"type"`
			LyricContent [][]string `json:"lyricContent"`
		} `json:"content"`
	}
	if json.Unmarshal(raw, &lang) != nil {
		return nil
	}
	for _, c := range lang.Content {
		if c.Type != 1 {
			continue
		}
		out := make([]string, len(c.LyricContent))
		for i, parts := range c.LyricContent {
			out[i] = strings.Join(parts, "")
		}
		return out
	}
	return nil
}

// Uneven reports whether the words of lines have times of their own worth
// showing: fewer than seven lines in eight have words all of one length,
// as lyrics timed only by line spread them. Lyrics a source calls
// word-timed may still be so.
func Uneven(lines []Line) bool {
	var checked, even int
	for _, l := range lines {
		if len(l.Words) < 3 {
			continue
		}
		checked++
		first := l.Words[0].End - l.Words[0].Start
		same := true
		for _, w := range l.Words[1:] {
			if d := w.End - w.Start - first; d > 20*time.Millisecond || d < -20*time.Millisecond {
				same = false
				break
			}
		}
		if same {
			even++
		}
	}
	return checked > 0 && even*8 < checked*7
}
