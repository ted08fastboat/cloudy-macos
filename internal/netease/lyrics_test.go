package netease

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"netease-music-mygo/internal/lyrics"
)

// Word-timed lyrics need their own translations: the ordinary timestamps
// dropped a line in songs 1924741068 and 1399847996; other sources still fall back.
func TestLyricsTranslationSources(t *testing.T) {
	for _, tc := range []struct {
		name                         string
		lrc, yrc                     string
		webTrans, pcTrans, wordTrans string
		pcFails                      bool
		want                         string
	}{
		{
			name: "milk crown mismatched ordinary timestamp",
			lrc:  "[02:35.720]original", yrc: "[140880,1000](140880,1000,0)original",
			webTrans: "[02:35.720]ordinary", pcTrans: "[02:35.720]ordinary",
			wordTrans: "[02:20.880]paired", want: "paired",
		},
		{
			name: "past first line mismatched ordinary timestamp",
			lrc:  "[00:00.300]original", yrc: "[22110,1000](22110,1000,0)original",
			webTrans: "[00:00.300]ordinary", pcTrans: "[00:00.300]ordinary",
			wordTrans: "[00:22.110]paired", want: "paired",
		},
		{
			name: "word translation missing",
			lrc:  "[00:10.000]original", yrc: "[10000,1000](10000,1000,0)original",
			webTrans: "[00:10.000]ordinary", want: "ordinary",
		},
		{
			name: "word lyrics missing",
			lrc:  "[00:10.000]original", webTrans: "[00:10.000]ordinary",
			wordTrans: "[00:20.000]paired", want: "ordinary",
		},
		{
			name: "web translation missing",
			lrc:  "[00:10.000]original", pcTrans: "[00:10.000]desktop", want: "desktop",
		},
		{
			name: "desktop API unavailable",
			lrc:  "[00:10.000]original", webTrans: "[00:10.000]ordinary",
			pcFails: true, want: "ordinary",
		},
		{
			name: "no translation",
			lrc:  "[00:10.000]original", yrc: "[10000,1000](10000,1000,0)original",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = old })
			http.DefaultTransport = songContextTransport(func(r *http.Request) (*http.Response, error) {
				body := map[string]any{"code": 200, "lrc": map[string]string{"lyric": tc.lrc}}
				trans := tc.webTrans
				if r.URL.Path == "/eapi/song/lyric/v1" {
					if tc.pcFails {
						body["code"] = 500
					}
					trans = tc.pcTrans
					body["yrc"] = map[string]string{"lyric": tc.yrc}
					body["ytlrc"] = map[string]string{"lyric": tc.wordTrans}
				}
				body["tlyric"] = map[string]string{"lyric": trans}
				encoded, err := json.Marshal(body)
				if err != nil {
					t.Fatal(err)
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(encoded))), Request: r}, nil
			})
			lrc, trans, yrc, err := Lyrics(1)
			if err != nil {
				t.Fatal(err)
			}
			lines := lyrics.Build(lrc, trans, yrc)
			if len(lines) != 1 || lines[0].Translation != tc.want {
				t.Fatalf("lyrics %+v, want one line translated as %q", lines, tc.want)
			}
		})
	}
}
