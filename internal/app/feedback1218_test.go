package app

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo"

	"netease-music-mygo/internal/netease"
)

// Started at login before the network is up, the session is asked about
// again instead of the listener being shown signed out.
func TestAccountAtBootRetries(t *testing.T) {
	old, oldRetries := accountOf, bootRetries
	defer func() { accountOf, bootRetries = old, oldRetries }()
	bootRetries.every, bootRetries.tries = time.Millisecond, 5
	calls := 0
	accountOf = func() (*netease.Profile, error) {
		calls++
		if calls < 3 {
			return nil, errors.New("dial tcp: no network")
		}
		return &netease.Profile{UserID: 7}, nil
	}
	if p := accountAtBoot(); p == nil || p.UserID != 7 || calls != 3 {
		t.Fatalf("profile %+v after %d calls", p, calls)
	}
	calls = 0
	accountOf = func() (*netease.Profile, error) { calls++; return nil, errors.New("offline") }
	if p := accountAtBoot(); p != nil || calls != 5 {
		t.Fatalf("offline: %+v after %d calls", p, calls)
	}
}

func TestAddableSongs(t *testing.T) {
	got := addableSongs([]netease.Track{{ID: 1}, {ID: 2}, {ID: 1}, {ID: -3}, {ID: 4, ProgramID: 9}})
	if !slices.Equal(got, []int64{1, 2}) {
		t.Fatalf("addable = %v", got)
	}
}

// With no app running, the screen cannot be asked where the pointer is.
func init() { npCursor = func() mygo.Point { return mygo.Point{} } }
