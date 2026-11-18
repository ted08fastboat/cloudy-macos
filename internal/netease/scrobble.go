package netease

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// NetEase counts a play only from its own clients' logs: the web player's
// /api/feedback/weblog, sent by the web or as the Windows client, answers
// success and counts nothing (tried on a real account on 2026-11-07). The
// log it counts goes to the client log host, through the eapi, from the
// macOS client, as two entries: "startplay" puts the song in recent plays,
// "play" counts it in the week's ranking and the listening total. Found by
// the NeteaseCloudMusicApiEnhanced project (issue #167) and used since by
// YesPlayMusic (#2485) and hana-music-api (#73).

// scrobbleDomain is the client log host; tests point it elsewhere.
var scrobbleDomain = "https://clientlog.music.163.com"

// scrobbleClient is the macOS client the logs come from.
var scrobbleClient = map[string]string{
	"os":      "osx",
	"appver":  "3.1.10.5100",
	"osver":   "15.5",
	"channel": "netease",
}

const scrobbleUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) NeteaseMusicDesktop/3.1.10.5100"

// ErrNotSignedIn says a play was not told, as no one is signed in.
var ErrNotSignedIn = errors.New("netease: scrobble: not signed in")

// Scrobble tells NetEase that the listener signed in played the song id,
// from the playlist or album source if known, for seconds: it counts in
// their recent plays and listening, as plays of NetEase's own clients do.
// The play log goes only once the start log was taken.
//
// These logs are NetEase's own, undocumented, sent as its macOS client
// would: the listener turns them on knowing that. A success does not say
// they counted, nor when: NetEase shows them some minutes later.
func Scrobble(id, source int64, seconds int) error {
	session := httpClient.Jar.Cookies(siteURL)
	signedIn := false
	for _, c := range session {
		if c.Name == "MUSIC_U" && c.Value != "" {
			signedIn = true
		}
	}
	if !signedIn {
		return ErrNotSignedIn
	}
	song := strconv.FormatInt(id, 10)
	from := strconv.FormatInt(max(source, 0), 10)
	base := map[string]any{
		"id":          song,
		"type":        "song",
		"mainsite":    "1",
		"mainsiteWeb": "1",
		"content":     "id=" + from,
	}
	play := map[string]any{
		"download": 0,
		"end":      "playend",
		"sourceId": from,
		"time":     max(seconds, 0),
		"wifi":     0,
		"source":   "list",
	}
	for k, v := range base {
		play[k] = v
	}
	as := eapiAs{domain: scrobbleDomain, client: scrobbleClient, ua: scrobbleUA, cookies: session}
	for _, entry := range []map[string]any{
		{"action": "startplay", "json": base},
		{"action": "play", "json": play},
	} {
		if err := scrobbleLog(as, entry); err != nil {
			return fmt.Errorf("netease: scrobble %s: %w", entry["action"], err)
		}
	}
	return nil
}

func scrobbleLog(as eapiAs, entry map[string]any) error {
	logs, err := json.Marshal([]any{entry})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var res struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := eapiSend(ctx, as, "/api/feedback/weblog", map[string]any{"logs": string(logs)}, &res); err != nil {
		return err
	}
	if res.Code != 200 {
		return fmt.Errorf("code %d %s", res.Code, res.Message)
	}
	return nil
}
