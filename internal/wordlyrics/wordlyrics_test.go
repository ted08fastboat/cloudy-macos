package wordlyrics

import (
	"netease-music-mygo/internal/lyrics"
	"testing"
	"time"
)

func TestPick(t *testing.T) {
	s := Song{Name: "晴天 (Live)", Artists: []string{"周杰伦"}, Length: 269 * time.Second}
	cs := []kugouCandidate{
		{ID: "1", AccessKey: "k", Song: "晴天", Singer: "别人", Duration: 269000},
		{ID: "2", AccessKey: "k", Song: "晴天", Singer: "周杰伦", Duration: 280000},
		{ID: "3", AccessKey: "k", Song: "晴 天", Singer: "周杰伦、杨瑞代", Duration: 270500},
	}
	if c := pick(cs, s); c == nil || c.ID != "3" {
		t.Fatalf("picked %+v, want 3", c)
	}
	if pick(cs[:2], s) != nil {
		t.Fatal("picked one of another artist or length")
	}
}

func TestTrimKugou(t *testing.T) {
	s := Song{Name: "晴天"}
	lines := trimKugou(lyricsOf("周杰伦 - 晴天", "故事的小黄花"), s)
	if len(lines) != 1 || lines[0].Text != "故事的小黄花" {
		t.Fatalf("lines = %+v", lines)
	}
}

func lyricsOf(texts ...string) []lyrics.Line {
	var out []lyrics.Line
	for _, t := range texts {
		out = append(out, lyrics.Line{Text: t})
	}
	return out
}
