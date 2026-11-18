package netease

import (
	"crypto/aes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
)

// eapiOpen undoes eapiEncrypt: the path and the JSON sent.
func eapiOpen(t *testing.T, params string) (string, map[string]any) {
	t.Helper()
	sealed, err := hex.DecodeString(params)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := aes.NewCipher([]byte(eapiKey))
	plain := make([]byte, len(sealed))
	for i := 0; i < len(sealed); i += aes.BlockSize {
		block.Decrypt(plain[i:i+aes.BlockSize], sealed[i:i+aes.BlockSize])
	}
	plain = plain[:len(plain)-int(plain[len(plain)-1])]
	parts := strings.Split(string(plain), "-36cd479b6b5-")
	if len(parts) != 3 {
		t.Fatalf("sealed %q", plain)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(parts[1]), &data); err != nil {
		t.Fatal(err)
	}
	return parts[0], data
}

// A play goes to the client log host as the macOS client, start log first,
// then the play log, with the session of the one signed in; and not at all
// when no one is.
func TestScrobbleLogs(t *testing.T) {
	type sent struct {
		url, cookie, ua string
		path            string
		log             map[string]any
		header          map[string]any
	}
	var got []sent
	answer := `{"code":200,"data":"success"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		path, data := eapiOpen(t, r.PostForm.Get("params"))
		var logs []map[string]any
		json.Unmarshal([]byte(data["logs"].(string)), &logs)
		if len(logs) != 1 {
			t.Errorf("%d logs in one request", len(logs))
		}
		got = append(got, sent{r.URL.Path, r.Header.Get("Cookie"), r.Header.Get("User-Agent"), path, logs[0], data["header"].(map[string]any)})
		http.SetCookie(w, &http.Cookie{Name: "MUSIC_U", Value: "from-log-host"})
		w.Write([]byte(answer))
	}))
	defer srv.Close()
	defer func(d string, j http.CookieJar) { scrobbleDomain, httpClient.Jar = d, j }(scrobbleDomain, httpClient.Jar)
	scrobbleDomain = srv.URL
	httpClient.Jar, _ = cookiejar.New(nil)

	if err := Scrobble(7, 30, 200); !errors.Is(err, ErrNotSignedIn) {
		t.Fatalf("signed out: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("told signed out: %v", got)
	}

	httpClient.Jar.SetCookies(siteURL, []*http.Cookie{{Name: "MUSIC_U", Value: "listener"}, {Name: "__csrf", Value: "c"}})
	if err := Scrobble(7, 30, 200); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("%d requests", len(got))
	}
	for _, s := range got {
		if s.url != "/eapi/feedback/weblog" || s.path != "/api/feedback/weblog" {
			t.Errorf("sent to %s as %s", s.url, s.path)
		}
		for _, want := range []string{"os=osx", "appver=3.1.10.5100", "osver=15.5", "channel=netease", "MUSIC_U=listener"} {
			if !strings.Contains(s.cookie, want) {
				t.Errorf("cookie %q lacks %s", s.cookie, want)
			}
		}
		if s.header["os"] != "osx" || s.header["MUSIC_U"] != "listener" || s.header["__csrf"] != "c" {
			t.Errorf("header %v", s.header)
		}
		if !strings.Contains(s.ua, "Macintosh") {
			t.Errorf("user agent %q", s.ua)
		}
	}
	start, play := got[0].log, got[1].log
	if start["action"] != "startplay" || play["action"] != "play" {
		t.Fatalf("logs %v then %v", start["action"], play["action"])
	}
	sj := start["json"].(map[string]any)
	for k, v := range map[string]any{"id": "7", "type": "song", "mainsite": "1", "mainsiteWeb": "1", "content": "id=30"} {
		if sj[k] != v {
			t.Errorf("startplay %s = %v, want %v", k, sj[k], v)
		}
	}
	pj := play["json"].(map[string]any)
	for k, v := range map[string]any{"id": "7", "end": "playend", "sourceId": "30", "time": 200.0, "source": "list", "mainsite": "1", "content": "id=30"} {
		if pj[k] != v {
			t.Errorf("play %s = %v, want %v", k, pj[k], v)
		}
	}
	// The log host's cookies are not the session's.
	for _, c := range httpClient.Jar.Cookies(siteURL) {
		if c.Name == "MUSIC_U" && c.Value != "listener" {
			t.Errorf("session became %q", c.Value)
		}
	}

	// A start log refused: no play log after it.
	got, answer = nil, `{"code":400,"message":"bad"}`
	if err := Scrobble(7, 0, 200); err == nil {
		t.Fatal("refused, but no error")
	}
	if len(got) != 1 {
		t.Errorf("%d requests after a refusal", len(got))
	}
}
