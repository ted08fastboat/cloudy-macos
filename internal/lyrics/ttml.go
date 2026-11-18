package lyrics

import (
	"encoding/xml"
	"io"
	"strconv"
	"strings"
	"time"
)

// ParseTTML reads word-timed lyrics in the TTML of the AMLL TTML database
// (what Apple Music serves too): a <p> a line, a <span> with begin and end
// a word, the spaces between them text of their own. A span of role
// x-translation is the line's translation; romanizations and background
// vocals (x-roman, x-bg) are left out.
func ParseTTML(data []byte) []Line {
	d := xml.NewDecoder(strings.NewReader(string(data)))
	d.Strict = false
	var (
		out  []Line
		line *Line
		// The roles of the spans open, innermost last; a word is in the
		// one innermost span with times.
		roles []string
		word  *Word
		trans strings.Builder
	)
	skipping := func() bool {
		for _, r := range roles {
			if r == "x-roman" || r == "x-bg" || r == "x-translation" {
				return true
			}
		}
		return false
	}
	for {
		tok, err := d.Token()
		if err == io.EOF || err != nil && tok == nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				begin, _ := ttmlTime(attr(t, "begin"))
				end, _ := ttmlTime(attr(t, "end"))
				line = &Line{At: begin, End: end, Timed: true}
				roles, word = nil, nil
				trans.Reset()
			case "span":
				if line == nil {
					continue
				}
				role := attr(t, "role")
				roles = append(roles, role)
				if skipping() {
					continue
				}
				start, ok1 := ttmlTime(attr(t, "begin"))
				end, ok2 := ttmlTime(attr(t, "end"))
				if ok1 && ok2 {
					line.Words = append(line.Words, Word{Start: start, End: end})
					word = &line.Words[len(line.Words)-1]
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "p":
				if line != nil {
					if tr := strings.TrimSpace(trans.String()); tr != "" {
						line.Translation = tr
					}
					finishTTMLLine(line)
					if line.Text != "" {
						out = append(out, *line)
					}
				}
				line, roles, word = nil, nil, nil
			case "span":
				if len(roles) > 0 {
					roles = roles[:len(roles)-1]
				}
				word = nil
			}
		case xml.CharData:
			if line == nil {
				continue
			}
			text := string(t)
			if len(roles) > 0 && roles[len(roles)-1] == "x-translation" {
				trans.WriteString(text)
				continue
			}
			if skipping() {
				continue
			}
			switch {
			case word != nil:
				word.Text += text
			case len(line.Words) > 0:
				// Space between words belongs to the word before it.
				if strings.TrimSpace(text) == "" {
					if !strings.HasSuffix(line.Words[len(line.Words)-1].Text, " ") {
						line.Words[len(line.Words)-1].Text += " "
					}
				} else {
					line.Words[len(line.Words)-1].Text += text
				}
			}
		}
	}
	return out
}

// finishTTMLLine sets the line's text from its words, dropping empty ones,
// and its times from theirs where the <p> gave none.
func finishTTMLLine(l *Line) {
	words := l.Words[:0]
	for _, w := range l.Words {
		if w.Text != "" {
			words = append(words, w)
		}
	}
	l.Words = words
	var b strings.Builder
	for _, w := range words {
		b.WriteString(w.Text)
	}
	l.Text = strings.TrimSpace(b.String())
	if len(words) == 0 {
		return
	}
	if l.At == 0 || words[0].Start < l.At {
		l.At = words[0].Start
	}
	if last := words[len(words)-1].End; l.End < last {
		l.End = last
	}
}

func attr(t xml.StartElement, name string) string {
	for _, a := range t.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

// ttmlTime reads a TTML clock time: "1:02:03.456", "02:03.456",
// "3.456" or "3.456s".
func ttmlTime(s string) (time.Duration, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	s = strings.TrimSuffix(s, "s")
	parts := strings.Split(s, ":")
	var total float64
	for _, p := range parts {
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return 0, false
		}
		total = total*60 + v
	}
	return time.Duration(total * float64(time.Second)), true
}
