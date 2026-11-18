package player

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"netease-music-mygo/internal/netease"
)

// A rejected or incomplete URL response must recover through the real request path without another click.
func TestAddressResponseAutomaticallyRecovers(t *testing.T) {
	for _, failure := range []struct{ name, body string }{
		{"400", `{"code":400}`},
		{"301", `{"code":301}`},
		{"negative_code", `{"code":-460}`},
		{"empty_data", `{"code":200,"data":[]}`},
		{"empty_url", `{"code":200,"data":[{"code":200,"url":""}]}`},
		{"missing_code", `{}`},
		{"invalid_json", `<html>temporary gateway failure</html>`},
	} {
		t.Run(failure.name, func(t *testing.T) {
			quietNavigation(t)
			retryAudio(t)
			transport := http.DefaultTransport
			requests := 0
			http.DefaultTransport = navigationTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/eapi/song/enhance/player/url/v1" {
					requests++
					body := failure.body
					if requests > 1 {
						body = `{"code":200,"data":[{"code":200,"url":"https://audio.test/song","type":"mp3","level":"exhigh"}]}`
					}
					return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
				}
				return transport.RoundTrip(r)
			})
			t.Cleanup(func() { http.DefaultTransport = transport })
			songURL = netease.SongURLContext
			p, ch := navigationPlayer(t, []netease.Track{{ID: 1}, {ID: 2, Duration: 5000}, {ID: 3}})
			p.Next()
			finishNavigation(t, p, ch)
			if requests != 2 || p.Current().ID != 2 || !p.Ready() || !p.Playing() || p.Loading() {
				t.Fatalf("URL failure stopped or skipped: requests=%d current=%d ready=%v playing=%v loading=%v error=%q", requests, p.Current().ID, p.Ready(), p.Playing(), p.Loading(), p.Err())
			}
		})
	}
}

// An unknown URL error code must keep loading beyond the old three-attempt fallback.
func TestAddressFailuresKeepRetrying(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		quietNavigation(t)
		calls := 0
		songURL = func(ctx context.Context, id int64, level string) (*netease.Stream, error) {
			calls++
			return nil, &netease.StreamError{Code: 400}
		}
		p, ch := navigationPlayer(t, []netease.Track{{ID: 1}, {ID: 2}, {ID: 3}})
		p.Next()
		settleRetries(ch)
		for _, delay := range []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 8 * time.Second} {
			time.Sleep(delay)
			settleRetries(ch)
		}
		if calls != 7 || p.Current().ID != 2 || !p.Loading() || !p.Playing() {
			t.Fatalf("URL retry stopped: calls=%d current=%d loading=%v playing=%v error=%q", calls, p.Current().ID, p.Loading(), p.Playing(), p.Err())
		}
	})
}

// A song the service refuses with its own code, or answers again and again
// with no address, is skipped instead of holding the queue on it.
func TestAddressRefusedSongIsSkipped(t *testing.T) {
	for _, refusal := range []struct {
		name  string
		err   func() error
		calls int
	}{
		{"song_code", func() error { return netease.ErrUnavailable }, 1},
		{"no_address", func() error { return &netease.StreamError{Code: 200, Song: true} }, 1 + songRetries},
	} {
		t.Run(refusal.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				quietNavigation(t)
				calls := 0
				songURL = func(ctx context.Context, id int64, level string) (*netease.Stream, error) {
					if id == 2 {
						calls++
						return nil, refusal.err()
					}
					return nil, &netease.StreamError{Code: 400}
				}
				p, ch := navigationPlayer(t, []netease.Track{{ID: 1}, {ID: 2}, {ID: 3}})
				p.Next()
				for range 6 {
					settleRetries(ch)
					time.Sleep(4 * time.Second)
				}
				settleRetries(ch)
				if calls != refusal.calls || p.Current().ID != 3 {
					t.Fatalf("refused song held the queue: calls=%d current=%d error=%q", calls, p.Current().ID, p.Err())
				}
			})
		})
	}
}
