package lyrics

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"testing"
	"time"
)

func TestParseTTML(t *testing.T) {
	ttml := `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:ttm="http://www.w3.org/ns/ttml#metadata"><body><div>` +
		`<p begin="00:20.686" end="00:22.460"><span begin="00:20.686" end="00:20.904">Hello</span> <span begin="00:20.904" end="00:21.500">world</span>` +
		`<span ttm:role="x-translation" xml:lang="zh-CN">你好世界</span><span ttm:role="x-roman">ha</span>` +
		`<span ttm:role="x-bg"><span begin="00:21.6" end="00:22">(ooh)</span></span></p>` +
		`<p begin="1:00:01.000" end="1:00:02.000"><span begin="1:00:01.000" end="1:00:02.000">晚</span></p>` +
		`</div></body></tt>`
	lines := ParseTTML([]byte(ttml))
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2: %+v", len(lines), lines)
	}
	l := lines[0]
	if l.Text != "Hello world" || l.Translation != "你好世界" || !l.Timed || len(l.Words) != 2 {
		t.Fatalf("line = %+v", l)
	}
	if l.Words[0].Text != "Hello " || l.Words[1].Start != 20904*time.Millisecond || l.At != 20686*time.Millisecond {
		t.Fatalf("words = %+v", l.Words)
	}
	if lines[1].At != time.Hour+time.Second {
		t.Fatalf("hour time = %v", lines[1].At)
	}
}

func encodeKRC(text string) string {
	var z bytes.Buffer
	w := zlib.NewWriter(&z)
	w.Write([]byte(text))
	w.Close()
	b := z.Bytes()
	for i := range b {
		b[i] ^= krcKey[i%len(krcKey)]
	}
	return base64.StdEncoding.EncodeToString(append([]byte("krc1"), b...))
}

func TestKRC(t *testing.T) {
	lang := base64.StdEncoding.EncodeToString([]byte(`{"content":[{"type":0,"lyricContent":[["a"],["b"]]},{"type":1,"lyricContent":[["故事"],["译二"]]}]}`))
	text := "[ti:晴天]\n[language:" + lang + "]\n[1000,2000]<0,500,0>故<500,700,0>事\n[3000,1000]<0,300,0>二<300,700,0>行\n"
	got, err := DecodeKRC(encodeKRC(text))
	if err != nil || got != text {
		t.Fatalf("decode = %q, %v", got, err)
	}
	lines := ParseKRC(got)
	if len(lines) != 2 || lines[0].Text != "故事" || lines[0].Translation != "故事" || lines[1].Translation != "译二" {
		t.Fatalf("lines = %+v", lines)
	}
	if w := lines[0].Words[1]; w.Start != 1500*time.Millisecond || w.End != 2200*time.Millisecond {
		t.Fatalf("word = %+v", w)
	}
	if _, err := DecodeKRC(base64.StdEncoding.EncodeToString([]byte("nope"))); err == nil {
		t.Fatal("a file that is no KRC decoded")
	}
}

func TestUneven(t *testing.T) {
	even := Build("[00:01.00]一二三四\n[00:05.00]五六七八\n", "", "")
	if Uneven(even) {
		t.Fatal("words spread evenly count as timed")
	}
	timed := []Line{{Words: []Word{{"a", 0, 100 * time.Millisecond}, {"b", 100 * time.Millisecond, 600 * time.Millisecond}, {"c", 600 * time.Millisecond, 700 * time.Millisecond}}}}
	if !Uneven(timed) {
		t.Fatal("timed words count as even")
	}
}

func TestKeepTranslations(t *testing.T) {
	from := []Line{{At: time.Second, Translation: "网易"}}
	more := []Line{{At: 1100 * time.Millisecond, Translation: "机器"}, {At: 5 * time.Second}}
	got := KeepTranslations(more, from)
	if got[0].Translation != "网易" || got[1].Translation != "" {
		t.Fatalf("NetEase's translation not kept: %+v", got)
	}
	own := KeepTranslations([]Line{{At: 0, Translation: "自带"}}, []Line{{At: 0}})
	if own[0].Translation != "自带" {
		t.Fatal("a source's own translation dropped though NetEase has none")
	}
}
