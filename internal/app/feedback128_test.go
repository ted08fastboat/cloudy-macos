package app

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/lyrics"
	"netease-music-mygo/internal/netease"
)

// Credits leave the lyrics shown once hidden, and come back when shown.
func TestShownLyricsHideCredits(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	lines := []lyrics.Line{
		{At: 0, Text: "作词 : 林夕"},
		{At: time.Second, Text: "作曲：陈小霞"},
		{At: 5 * time.Second, Text: "第一句"},
		{At: 9 * time.Second, Text: "制作人: 某某"},
		{At: 12 * time.Second, Text: "第二句"},
	}
	a.pl.SetLyrics(lines)
	if got, _ := a.shownLyrics(); len(got) != 5 {
		t.Fatalf("credits hidden by default: %d lines", len(got))
	}
	a.settings.HideCredits = true
	got, _ := a.shownLyrics()
	if len(got) != 2 || got[0].Text != "第一句" || got[1].Text != "第二句" {
		t.Fatalf("shown %v", got)
	}
	// The same lines are sifted once.
	if again, _ := a.shownLyrics(); &again[0] != &got[0] {
		t.Fatal("sifted again for the same lyrics")
	}
	// Lyrics of credits alone stay.
	a.pl.SetLyrics(lines[:2])
	if got, _ := a.shownLyrics(); len(got) != 2 {
		t.Fatalf("only credits: %d lines", len(got))
	}
}

// Ctrl+= and Ctrl+- zoom the window by tens, within bounds, and Ctrl+0
// zooms it back; the zoom is kept in the settings.
func TestZoomShortcuts(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	tt := ui.NewTester(a.view, 1000, 700)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyEqual)
	tt.Frame()
	if a.settings.UIZoom != 1.1 || a.uiZoom() != 1.1 {
		t.Fatalf("zoom %v after Ctrl+=", a.settings.UIZoom)
	}
	for range 20 {
		tt.Key(ui.Cmd, ui.KeyEqual)
	}
	if a.uiZoom() != maxUIZoom {
		t.Fatalf("zoom %v past the most", a.uiZoom())
	}
	for range 30 {
		tt.Key(ui.Cmd, ui.KeyMinus)
	}
	if a.uiZoom() != minUIZoom {
		t.Fatalf("zoom %v past the least", a.uiZoom())
	}
	tt.Key(ui.Cmd, ui.Key0)
	if a.settings.UIZoom != 0 || a.uiZoom() != 1 {
		t.Fatalf("zoom %v after Ctrl+0", a.settings.UIZoom)
	}
}

// Off, the line sung in Now Playing lights whole, as one with no words.
func TestNowPlayingWordsSwitch(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	l := lyrics.Line{Text: "谁知道这份渴望", Words: []lyrics.Word{{Text: "谁知道"}, {Text: "这份渴望"}}}
	if len(a.npLine(l).Words) != 2 {
		t.Fatal("not word by word by default")
	}
	a.settings.NpWholeLines = true
	if got := a.npLine(l); len(got.Words) != 0 || got.Text != l.Text {
		t.Fatalf("off: %+v", got)
	}
	if len(l.Words) != 2 {
		t.Fatal("the lyrics themselves lost their words")
	}
}

// The login dialog turns to the cookie field and back.
func TestLoginDialogCookieFace(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.login = loginState{open: true, gen: 1}
	tt := ui.NewTester(a.view, 1000, 700)
	tt.Advance(time.Second)
	if err := tt.Click("没有手机 App？使用 Cookie 登录"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	if !a.login.cookie || !tt.HasText("Cookie 登录") {
		t.Fatal("no cookie face")
	}
	if _, ok := tt.Find("Cookie"); !ok {
		t.Fatal("no cookie field")
	}
	if err := tt.Click("返回扫码登录"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	if a.login.cookie || !tt.HasText("扫码登录") {
		t.Fatal("did not go back to the QR code")
	}
}

// In 热评模式 the hot comments show all along, through the singing too;
// only in the quiet parts, the singing keeps its lyrics.
func TestHotCommentsAllAlong(t *testing.T) {
	a, tr, lines := quietApp(0)
	if cm, _ := a.quietComment(&tr, lines, true, 22*time.Second); cm != nil {
		t.Fatalf("quiet parts only, showed %q in the singing", cm.Content)
	}
	a.settings.HotAll = true
	cm, next := a.quietComment(&tr, lines, true, 22*time.Second)
	if cm == nil || next <= 0 {
		t.Fatalf("all along, showed %v in the singing, next in %v", cm, next)
	}
	// They take turns, about one a slot.
	if later, _ := a.quietComment(&tr, lines, true, 22*time.Second+next); later == nil || later.ID == cm.ID {
		t.Fatalf("after %v, still %v", next, later)
	}
}

// 仅在间奏时显示热评 shows under 热评模式 once that is on, which turned on
// shows them all along.
func TestHotCommentsSettings(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.settingsView.open, a.settingsView.tab = true, settingsLyrics
	tt := ui.NewTester(a.view, 1100, 760)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Advance(time.Second)
	toggle := func(label, desc string) {
		t.Helper()
		_ = desc
		title, ok1 := tt.Find(label)
		pane, ok3 := tt.Find("歌词设置项")
		if !ok1 || !ok3 {
			t.Fatalf("no %s control: %v %v", label, ok1, ok3)
		}
		tt.ClickAt(pane.X+pane.W-30-14-22, title.Y+title.H/2)
		tt.Advance(time.Second)
	}
	if !tt.HasText("热评模式") || tt.HasText("仅在间奏时显示热评") {
		t.Fatal("want 热评模式 alone while off")
	}
	toggle("热评模式", "歌词处换成这首歌的热门评论，一条接一条地轮换")
	if !a.settings.HotComments || !a.settings.HotAll || !tt.HasText("仅在间奏时显示热评") {
		t.Fatalf("on: hot %v all %v", a.settings.HotComments, a.settings.HotAll)
	}
	toggle("仅在间奏时显示热评", "只在前奏、间奏与尾奏时显示热评，有歌词的地方照常显示歌词")
	if !a.settings.HotComments || a.settings.HotAll {
		t.Fatalf("quiet parts only: hot %v all %v", a.settings.HotComments, a.settings.HotAll)
	}
}

// The radars have a section of their own on the home page, a row that
// unfolds to all of them, each opening its playlist; 私人雷达 leaves the
// day's playlists above it.
func TestRadarSection(t *testing.T) {
	a := newTestApp()
	accentHome(a)
	a.profile = &netease.Profile{UserID: 1, Nickname: "听众"}
	a.recPlaylists[1] = netease.Playlist{ID: radars[0].id, Name: "私人雷达"}
	a.radar.lists = map[int64]netease.Playlist{radars[0].id: {ID: radars[0].id, Name: "今天从《喵》听起|私人雷达"}}
	// The calendar's page of 私人雷达 names it too.
	a.settings.DeckOff = []string{"radar"}
	tt := ui.NewTester(a.view, 1200, 1500)
	tt.Advance(time.Second)
	if !tt.HasText("今天从《喵》听起") || tt.HasText("韩系私人雷达") {
		t.Fatal("want a row of radars, the day's title under 私人雷达")
	}
	n := 0
	for _, s := range tt.Texts() {
		if s == "私人雷达" {
			n++
		}
	}
	if n != 3 { // the section's title, and its card's label and name
		t.Fatalf("私人雷达 shows %d times", n)
	}
	if err := tt.Click(fmt.Sprintf("展开全部 %d", len(radars))); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	if !a.radar.open || !tt.HasText("韩系私人雷达") {
		t.Fatal("did not unfold")
	}
	if err := tt.Click("时光雷达"); err != nil {
		t.Fatal(err)
	}
	if a.route.page != pagePlaylist || a.route.id != 5320167908 {
		t.Fatalf("route %+v", a.route)
	}
}

// The daily card and 风格推荐 are the pages of a calendar at the head of
// the home page, the daily card in front: the strip at its foot flips it
// away, up and out, and the style page comes to the front, whose songs
// each play from there, whose 换风格 opens the dialog, and the rest of
// which opens the songs' page; pulled up and let go, it flips back.
func TestHomeCalendar(t *testing.T) {
	a := newTestApp()
	accentHome(a)
	a.profile = &netease.Profile{UserID: 1, Nickname: "听众"}
	// The two pages alone; TestCalendarBookmarks has all of them.
	a.settings.DeckOff = []string{"fm", "radar", "week"}
	var songs []netease.Track
	for i, name := range []string{"Best Part", "Location", "Japanese Denim", "第四首"} {
		songs = append(songs, netease.Track{ID: int64(100 + i), Name: name, Duration: 200000, Artists: []netease.Artist{{Name: "歌手"}}, Album: netease.Album{PicURL: fmt.Sprintf("cover-%d", i+1)}})
	}
	a.style.songs = netease.StyleSongs{Songs: songs, Picked: netease.StyleCategory{ID: 1000, Tags: []netease.StyleTag{{ID: 1, Name: "R&B"}, {ID: 2, Name: "欧美流行"}}}}
	a.style.loaded = true
	a.style.cats = []netease.StyleCategory{{ID: 1000, Name: "曲风"}}
	tt := ui.NewTester(a.view, 1700, 1100)
	tt.Advance(time.Second)
	dir := os.Getenv("NM_FRAMES")
	shot := func(name string) {
		if dir != "" {
			f, _ := os.Create(filepath.Join(dir, name+".png"))
			png.Encode(f, tt.Image())
			f.Close()
		}
	}
	shot("deck-daily")
	if a.deck.front != deckDaily {
		t.Fatalf("front %v", a.deck.front)
	}
	r, ok := tt.Find("翻到风格推荐")
	if !ok {
		t.Fatal("no strip to flip")
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(400 * time.Millisecond)
	shot("deck-lift")
	if err := tt.Click("翻到风格推荐"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(120 * time.Millisecond)
	shot("deck-flip-1")
	if !a.deck.flipping || a.deck.front != deckStyle {
		t.Fatalf("flipping %v, front %v", a.deck.flipping, a.deck.front)
	}
	tt.Advance(100 * time.Millisecond)
	shot("deck-flip-2")
	tt.Advance(time.Second)
	shot("deck-style")
	if a.deck.flipping || !tt.HasText("R&B · 欧美流行") || !tt.HasText("Japanese Denim") {
		t.Fatalf("flipping %v; want the styles and the songs", a.deck.flipping)
	}
	if err := tt.Click("Location"); err != nil {
		t.Fatal(err)
	}
	if cur := a.pl.Current(); cur == nil || cur.ID != 101 || len(a.pl.Queue()) != 4 {
		t.Fatalf("playing %v", cur)
	}
	if a.route.page == pageStyle {
		t.Fatal("a song opened the page")
	}
	// Pulled up, its foot under the pointer, on past edge on, where the
	// pointer is off the page, the page stays up, never falling back to
	// start again; let go, it flips.
	// The pointer leaves the strip first, which lifts the page while hovered.
	tt.Move(5, 5)
	tt.Advance(time.Second)
	r, _ = tt.Find("R&B · 欧美流行")
	x, y := r.X+400, r.Y+140
	tt.Press(x, y)
	last := float32(0)
	for i := 1; i <= 22; i++ {
		tt.Move(x, y-float32(i)*14)
		tt.Advance(16 * time.Millisecond)
		if i == 8 {
			shot("deck-pull")
		}
		if !a.deck.pulling || a.deck.lift < last-0.001 {
			t.Fatalf("step %d: pulling %v, lift %v after %v", i, a.deck.pulling, a.deck.lift, last)
		}
		last = a.deck.lift
	}
	if last < 0.99 {
		t.Fatalf("pulled to %v", last)
	}
	tt.Release(x, y-22*14)
	tt.Advance(1500 * time.Millisecond)
	if a.deck.front != deckDaily || a.deck.flipping || a.route.page == pageStyle {
		t.Fatalf("let go: front %v, flipping %v, page %v", a.deck.front, a.deck.flipping, a.route.page)
	}

	// Flipped back to it, 换风格 opens the dialog.
	if err := tt.Click("翻到风格推荐"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(1500 * time.Millisecond)
	if err := tt.Click("换风格"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	if !a.style.open || a.route.page == pageStyle {
		t.Fatalf("换风格: open %v, page %v", a.style.open, a.route.page)
	}

	b := newTestApp()
	accentHome(b)
	b.profile, b.style = a.profile, styleState{songs: a.style.songs, loaded: true}
	b.deck = deckState{front: deckStyle, chosen: true}
	b.settings.DeckOff = a.settings.DeckOff
	tt = ui.NewTester(b.view, 1200, 1100)
	tt.Advance(time.Second)
	if err := tt.Click("R&B · 欧美流行"); err != nil {
		t.Fatal(err)
	}
	if b.route.page != pageStyle {
		t.Fatal("the page did not open the songs' page")
	}

	// Signed out there is one page, and nothing to flip.
	o := newTestApp()
	accentHome(o)
	tt = ui.NewTester(o.view, 1200, 1100)
	tt.Advance(time.Second)
	if _, ok := tt.Find("翻到风格推荐"); ok || !tt.HasText("每日推荐") {
		t.Fatal("want the daily card alone")
	}
}

// A song on the daily or the style card, once it plays, pauses and plays
// on from its row rather than starting again.
func TestCardPicksToggle(t *testing.T) {
	a := newTestApp()
	accentHome(a)
	a.profile = &netease.Profile{UserID: 1, Nickname: "听众"}
	var songs, day []netease.Track
	for i, name := range []string{"Best Part", "Location", "Japanese Denim", "第四首"} {
		songs = append(songs, netease.Track{ID: int64(100 + i), Name: name, Duration: 200000, Artists: []netease.Artist{{Name: "歌手"}}})
		day = append(day, netease.Track{ID: int64(200 + i), Name: "日" + name, Duration: 200000, Artists: []netease.Artist{{Name: "歌手"}}})
	}
	a.style.songs = netease.StyleSongs{Songs: songs, Picked: netease.StyleCategory{ID: 1000, Tags: []netease.StyleTag{{ID: 1, Name: "R&B"}}}}
	a.style.loaded = true
	a.daily = day
	a.settings.DeckOff = []string{"fm", "radar", "week"}
	tt := ui.NewTester(a.view, 1700, 1100)
	tt.Advance(time.Second)
	toggle := func(play, name string, id int64) {
		t.Helper()
		if err := tt.Click(play + name); err != nil {
			t.Fatal(err)
		}
		tt.Advance(time.Second)
		if cur := a.pl.Current(); cur == nil || cur.ID != id {
			t.Fatalf("%s: playing %v", name, cur)
		}
		was := a.pl.Playing()
		label := play + name
		if was {
			label = "暂停 " + name
		}
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Advance(time.Second)
		if cur := a.pl.Current(); cur == nil || cur.ID != id || a.pl.Playing() == was {
			t.Fatalf("%s: clicked again, playing %v %v", name, cur, a.pl.Playing())
		}
	}
	// The rows are named by their songs; the style page is under the daily
	// card till it flips.
	toggle("", "日Best Part", 200)
	if err := tt.Click("翻到风格推荐"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(1500 * time.Millisecond)
	toggle("", "Location", 101)
	if dir := os.Getenv("NM_FRAMES"); dir != "" {
		r, _ := tt.Find("R&B")
		tt.Move(r.X+40, r.Y+60)
		tt.Advance(300 * time.Millisecond)
		f, _ := os.Create(filepath.Join(dir, "picks.png"))
		png.Encode(f, tt.Image())
		f.Close()
	}
}

// The calendar holds the FM, 私人雷达 and 听歌周报 too, a bookmark each at
// its right: a bookmark flips the pages before its own by, one after the
// other, to its page; the bookmark under them opens what the calendar
// holds, where a page goes out of it, and its bookmark with it.
func TestCalendarBookmarks(t *testing.T) {
	a := newTestApp()
	accentHome(a)
	a.profile = &netease.Profile{UserID: 1, Nickname: "听众"}
	var songs []netease.Track
	for i, name := range []string{"Best Part", "Location", "Japanese Denim"} {
		songs = append(songs, netease.Track{ID: int64(100 + i), Name: name, Duration: 200000, Artists: []netease.Artist{{Name: "歌手"}}, Album: netease.Album{PicURL: fmt.Sprintf("cover-%d", i+1)}})
	}
	a.style.songs = netease.StyleSongs{Songs: songs, Picked: netease.StyleCategory{ID: 1000, Tags: []netease.StyleTag{{ID: 1, Name: "R&B"}}}}
	a.style.loaded = true
	for i, tr := range songs {
		a.week.songs = append(a.week.songs, netease.Ranked{Track: tr, Plays: 9 - 3*i, Score: 100 - 30*i})
	}
	tt := ui.NewTester(a.view, 1700, 1100)
	tt.Advance(time.Second)
	dir := os.Getenv("NM_FRAMES")
	shot := func(name string) {
		if dir != "" {
			f, _ := os.Create(filepath.Join(dir, name+".png"))
			png.Encode(f, tt.Image())
			f.Close()
		}
	}
	shot("tabs-daily")
	if got := a.deckPages(); !slices.Equal(got, []deckPage{deckDaily, deckStyle, deckRadar, deckFM, deckWeek}) {
		t.Fatalf("pages %v", got)
	}
	if err := tt.Click("翻到听歌周报"); err != nil {
		t.Fatal(err)
	}
	flips := 0
	was := deckDaily
	for range 120 {
		tt.Advance(16 * time.Millisecond)
		if a.deck.front != was {
			flips++
			was = a.deck.front
		}
	}
	shot("tabs-week")
	if a.deck.front != deckWeek || a.deck.flipping || a.deck.targeting || flips != 4 {
		t.Fatalf("front %v, flipping %v, targeting %v, flips %d", a.deck.front, a.deck.flipping, a.deck.targeting, flips)
	}
	if !tt.HasText("听了 3 首 · 最常听 歌手") || !tt.HasText("Japanese Denim") {
		t.Fatal("want the week's page")
	}
	if err := tt.Click("播放 Location"); err != nil {
		t.Fatal(err)
	}
	if cur := a.pl.Current(); cur == nil || cur.ID != 101 {
		t.Fatalf("playing %v", cur)
	}

	// Back round to the daily card, on through the pages after.
	if err := tt.Click("翻到每日推荐"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(2 * time.Second)
	if a.deck.front != deckDaily {
		t.Fatalf("front %v", a.deck.front)
	}

	if err := tt.Click("编辑台历"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	shot("tabs-edit")
	if err := tt.Click("删除 私人 FM"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	if !slices.Contains(a.settings.DeckOff, "fm") || slices.Contains(a.deckPages(), deckFM) {
		t.Fatalf("off %v", a.settings.DeckOff)
	}
	if _, ok := tt.Find("翻到私人 FM"); ok {
		t.Fatal("its bookmark stayed")
	}
	shot("tabs-edit-removed")
	// Dragged a row down, past the next page kept, the one taken out not
	// counting.
	r, ok := tt.Find("拖动排序 每日推荐")
	if !ok {
		t.Fatal("no row to drag")
	}
	x, y := r.X+16, r.Y+r.H/2
	tt.Press(x, y)
	for i := 1; i <= 8; i++ {
		tt.Move(x, y+float32(i)*7)
		tt.Advance(16 * time.Millisecond)
	}
	shot("deck-drag")
	tt.Release(x, y+56)
	tt.Advance(time.Second)
	if got := a.deckKept(); !slices.Equal(got, []deckPage{deckStyle, deckDaily, deckRadar, deckWeek}) {
		t.Fatalf("kept %v", got)
	}
	// Added back, after the rest.
	if err := tt.Click("添加 私人 FM"); err != nil {
		t.Fatal(err)
	}
	if got := a.deckKept(); !slices.Equal(got, []deckPage{deckStyle, deckDaily, deckRadar, deckWeek, deckFM}) {
		t.Fatalf("kept %v", got)
	}

	// The settings name what it holds; their drawer changes it too.
	a.deck.editing = false
	a.openSettings()
	a.settingsView.query = "台历"
	tt.Frame()
	tt.Advance(time.Second)
	if !tt.HasText("5 页：风格 · 日推 · 雷达 · 周报 · FM") || tt.HasText("最近 7 天听得最多的歌") {
		t.Fatal("want the summary alone")
	}
	if err := tt.Click("管理台历内容"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	shot("deck-settings")
	if err := tt.Click("删除 听歌周报"); err != nil {
		t.Fatal(err)
	}
	if a.settings.deckOn(deckWeek) {
		t.Fatal("the drawer did not take it out")
	}
}

// The FM's page switches its modes as the FM's own page does; 听歌周报
// opens 本周听歌排行; the week's plays count by day and by device.
func TestCalendarFMAndWeek(t *testing.T) {
	a := newTestApp()
	accentHome(a)
	a.profile = &netease.Profile{UserID: 1, Nickname: "听众"}
	for i, name := range []string{"FIRESTARTER", "Come Around Me", "Forever", "第四首"} {
		tr := netease.Track{ID: int64(300 + i), Name: name, Duration: 180000, Artists: []netease.Artist{{Name: fmt.Sprint("歌手", i%2)}}, Album: netease.Album{PicURL: fmt.Sprintf("cover-%d", i)}}
		a.week.songs = append(a.week.songs, netease.Ranked{Track: tr, Plays: 5 - i, Score: 100 - 20*i})
	}
	a.week.stats = weekStats{days: [7]int{3, 0, 5, 9, 2, 4, 6}, daysKnown: true, phone: 13, computer: 8}
	a.deck = deckState{front: deckFM, chosen: true}
	tt := ui.NewTester(a.view, 1700, 1100)
	tt.Advance(time.Second)
	dir := os.Getenv("NM_FRAMES")
	shot := func(name string) {
		if dir != "" {
			f, _ := os.Create(filepath.Join(dir, name+".png"))
			png.Encode(f, tt.Image())
			f.Close()
		}
	}
	shot("deck-fm")
	if err := tt.Click("私人 FM 熟悉"); err != nil {
		t.Fatal(err)
	}
	if a.fmMode != 1 {
		t.Fatalf("mode %d", a.fmMode)
	}
	if err := tt.Click("翻到听歌周报"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(2 * time.Second)
	shot("deck-week")
	// 4 songs: (5+4+3+2) plays of 3 minutes.
	if !tt.HasText("约 42 分钟") || !tt.HasText("手机 62% · 电脑 38%") || !tt.HasText("每天听过的歌") {
		t.Fatal("want the week's length, devices and days")
	}
	if err := tt.Click("听歌周报"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	shot("week-page")
	if a.route.page != pageWeek || !tt.HasText("本周听歌排行") || !tt.HasText("5 次") {
		t.Fatalf("page %v", a.route.page)
	}

	now := time.Date(2026, 11, 15, 20, 0, 0, 0, time.Local)
	plays := []netease.Play{
		{At: now.Add(-time.Hour), Device: "android"},
		{At: now.Add(-30 * time.Hour), Device: "pc"},
		{At: now.Add(-50 * time.Hour), Device: "osx"},
		{At: now.AddDate(0, 0, -9), Device: "android"},
	}
	s := weekStatsOf(plays, now)
	if !s.daysKnown || s.days[6] != 1 || s.days[5] != 1 || s.days[4] != 1 || s.phone != 1 || s.computer != 2 {
		t.Fatalf("stats %+v", s)
	}
	if got := weekStatsOf(plays[:2], now); got.daysKnown {
		t.Fatal("two plays of today and yesterday do not reach back a week")
	}
	if weekLength(380) != "约 6 小时 20 分" || weekRange(now) != "11.09 – 11.15" {
		t.Fatalf("%q %q", weekLength(380), weekRange(now))
	}
}

// With NM_FRAMES set, saves each page of the calendar in a narrow window,
// where a small cover stands at its right in place of what wider pages
// show there.
func TestCalendarNarrowFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip("set NM_FRAMES to a directory to save frames")
	}
	a := newTestApp()
	accentHome(a)
	a.profile = &netease.Profile{UserID: 1, Nickname: "听众"}
	songs := []netease.Track{{ID: 100, Name: "Best Part", Duration: 200000, Album: netease.Album{PicURL: "cover-1"}}}
	a.style.songs = netease.StyleSongs{Songs: songs, Picked: netease.StyleCategory{ID: 1000, Tags: []netease.StyleTag{{ID: 1, Name: "R&B"}}}}
	a.style.loaded = true
	a.week.songs = []netease.Ranked{{Track: songs[0], Plays: 3, Score: 100}}
	for _, pg := range deckAll {
		a.deck = deckState{front: pg, chosen: true}
		tt := ui.NewTester(a.view, 1000, 700)
		tt.Advance(time.Second)
		f, _ := os.Create(filepath.Join(dir, "narrow-"+deckID(pg)+".png"))
		png.Encode(f, tt.Image())
		f.Close()
	}
}
