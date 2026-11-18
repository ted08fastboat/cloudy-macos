package app

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// moreState holds what 1.2.13's additions keep between frames.
type moreState struct {
	npCalm npCalmState // how long Now Playing has been left alone
	// queue is the queue's popover over the player bar.
	queue struct {
		open bool
		list ui.ListState
		at   int // as app.queueAt
	}
	// draft is the comment being written in Now Playing.
	draft commentDraft
	// numbers are the album places of NetEase songs whose copies lacked
	// them, as songs kept before 1.2.13 do (album_numbers.go).
	numbers numberCache
	// switchedAt is when the songs sorted by album last switched between
	// the list and the grid, and gridCols how many cards a line of the
	// grid held (ui_album_grid.go).
	switchedAt time.Time
	gridCols   int
	// capsule is the glide of the capsule of the list/grid switch: from
	// where, and since when (albumViewSwitch).
	capsule struct {
		from float32
		at   time.Time
	}
	// slideX is where the rows come in from as an album opens or closes
	// (ui_album_open.go), or 0 as the view switches.
	slideX float32
	// intro is the dialog of an introduction; introCache, the artists'.
	intro      introView
	introCache map[int64]artistIntro
	// The mini window's resize under way, when its wheel last turned the
	// volume, and where its cover was laid out, for the morph to go to.
	miniResize     miniResize
	miniVolumeAt   time.Time
	miniCoverAt    ui.Rect
	miniCoverRound float32   // its corners
	miniHoverAt    time.Time // when the pointer was last over the square
	// wiki is the 百科 of the song playing, for Now Playing.
	wiki wikiState
	// marquees are the titles that slide, by where they show.
	marquees map[string]*marqueeState
	// npPaneW is how wide Now Playing's pane beside the cover was laid
	// out, for the lyrics' type to grow with it.
	npPaneW float32
	// volumeAt is when Now Playing's volume last moved, for its percent
	// to show a moment (npVolume).
	volumeAt time.Time
}
