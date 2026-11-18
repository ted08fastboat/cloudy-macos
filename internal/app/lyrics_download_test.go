package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/download"
	"netease-music-mygo/internal/lyrics"
	"netease-music-mygo/internal/netease"
)

// Lyrics retain Unicode, translations and real timing in all three export formats.
func TestLyricsFileText(t *testing.T) {
	lines := []lyrics.Line{
		{At: 12500 * time.Millisecond, End: 20 * time.Second, Text: " 第一行🎵 ", Translation: " First line "},
		{At: 15 * time.Second, Text: "第二行"},
	}
	for _, tc := range []struct{ format, want string }{
		{"txt", "第一行🎵\r\nFirst line\r\n第二行\r\n"},
		{"lrc", "[00:12.500]第一行🎵\r\n[00:12.500]First line\r\n[00:15.000]第二行\r\n"},
		{"srt", "1\r\n00:00:12,500 --> 00:00:15,000\r\n第一行🎵\r\nFirst line\r\n\r\n2\r\n00:00:15,000 --> 00:00:18,000\r\n第二行\r\n\r\n"},
	} {
		t.Run(tc.format, func(t *testing.T) {
			got, err := lyricsFileText(lines, tc.format, true, 18*time.Second)
			if err != nil || got != tc.want {
				t.Fatalf("export = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

// SRT merges simultaneous lines and keeps an hour-long song's final cue valid.
func TestLyricsSubtitleTimes(t *testing.T) {
	lines := []lyrics.Line{
		{At: time.Hour, Text: "first"},
		{At: time.Hour, Text: "second", Translation: "translation"},
		{At: time.Hour + 2*time.Second, End: time.Hour + 3*time.Second, Text: "last"},
	}
	got, err := lyricsFileText(lines, "srt", true, 0)
	want := "1\r\n01:00:00,000 --> 01:00:02,000\r\nfirst\r\nsecond\r\ntranslation\r\n\r\n2\r\n01:00:02,000 --> 01:00:03,000\r\nlast\r\n\r\n"
	if err != nil || got != want {
		t.Fatalf("subtitle = %q, %v", got, err)
	}
	got, err = lyricsFileText(lines[:1], "lrc", true, 0)
	if err != nil || got != "[60:00.000]first\r\n" {
		t.Fatalf("long LRC = %q, %v", got, err)
	}
}

// Plain lyrics can be exported as text, but must never be given fabricated timing.
func TestPlainLyricsExport(t *testing.T) {
	lines := lyrics.FromText("plain lyrics\nsecond line")
	if text, err := lyricsFileText(lines, "txt", true, 0); err != nil || text != "plain lyrics\r\nsecond line\r\n" {
		t.Fatalf("plain text = %q, %v", text, err)
	}
	for _, format := range []string{"lrc", "srt"} {
		if _, err := lyricsFileText(lines, format, true, 0); err == nil {
			t.Fatalf("plain lyrics exported as timed %s", format)
		}
	}
	for _, lines := range [][]lyrics.Line{nil, {{Text: " "}}} {
		if _, err := lyricsFileText(lines, "txt", true, 0); err == nil {
			t.Fatal("empty lyrics exported")
		}
	}
}

// Switching songs or replacing the playback cache cannot change an export already requested.
func TestLyricDownloadSnapshotsSong(t *testing.T) {
	a := newTestApp()
	a.settings.DownloadFolder = t.TempDir()
	song := netease.Track{ID: 7, Name: "original", Duration: 10000}
	a.pl.Cue([]netease.Track{song})
	lines := lyrics.Build("[00:01.25]original lyrics", "", "")
	a.pl.SetLyrics(lines)
	r := a.lyricDownloadRequest(song, "lrc", true)
	lines[0].Text = "changed cache"
	a.pl.Cue([]netease.Track{{ID: 8, Name: "new song"}})
	a.pl.SetLyrics(lyrics.Build("[00:02]wrong lyrics", "", ""))
	old := chooseLyricPath
	defer func() { chooseLyricPath = old }()
	chooseLyricPath = func(opts mygo.SaveDialogOptions) (string, error) {
		if !strings.Contains(opts.DefaultPath, "original.lrc") {
			t.Errorf("filename changed to %q", opts.DefaultPath)
		}
		return opts.DefaultPath, nil
	}
	path, err := r.save()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != "[00:01.250]original lyrics\r\n" {
		t.Fatalf("saved lyrics = %q, %v", b, err)
	}
}

// Local sidecars work offline; an error, a missing lyric or cancelling never creates an export.
func TestLyricDownloadOfflineAndCancel(t *testing.T) {
	dir := t.TempDir()
	song := netease.Track{ID: 7, LocalPath: filepath.Join(dir, "song.mp3")}
	if err := os.WriteFile(download.LyricsPath(song.LocalPath), []byte("[00:02]offline"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldFetch, oldChoose := downloadLyricsOf, chooseLyricPath
	defer func() { downloadLyricsOf, chooseLyricPath = oldFetch, oldChoose }()
	downloadLyricsOf = func(int64) (string, string, string, error) { return "", "", "", errors.New("offline") }
	called := 0
	chooseLyricPath = func(opts mygo.SaveDialogOptions) (string, error) { called++; return "", nil }
	r := lyricDownloadRequest{song: song, format: "txt", folder: dir}
	if path, err := r.save(); path != "" || err != nil || called != 1 {
		t.Fatalf("cancel = %q, %v, calls %d", path, err, called)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 1 {
		t.Fatalf("cancelling changed directory: %v, %v", files, err)
	}
	r.song.LocalPath = ""
	if _, err := r.save(); err == nil || called != 1 {
		t.Fatal("network failure opened the save dialog")
	}
	downloadLyricsOf = func(int64) (string, string, string, error) { return "", "", "", nil }
	if _, err := r.save(); err == nil || called != 1 {
		t.Fatal("missing lyrics opened the save dialog")
	}
}

// A filename's extension must match its content, and appending one cannot bypass overwrite confirmation.
func TestLyricDownloadFileSafety(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "song.lrc")
	if err := os.WriteFile(path, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := lyricDownloadRequest{format: "lrc", loaded: true, lines: lyrics.Build("[00:01]new", "", "")}
	old := chooseLyricPath
	defer func() { chooseLyricPath = old }()
	for _, selected := range []string{strings.TrimSuffix(path, ".lrc"), filepath.Join(dir, "wrong.txt"), filepath.Join(dir, "folder.lrc")} {
		chooseLyricPath = func(mygo.SaveDialogOptions) (string, error) { return selected, nil }
		if selected == filepath.Join(dir, "folder.lrc") {
			if err := os.Mkdir(selected, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := r.save(); err == nil {
			t.Fatalf("unsafe path %q accepted", selected)
		}
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != "keep" {
		t.Fatalf("unconfirmed overwrite: %q, %v", b, err)
	}
}

// Both three-dot menus open the same options; the toggle and format picker control the saved extension.
func TestLyricDownloadMenuOptions(t *testing.T) {
	for _, tc := range []struct {
		name, button, format   string
		nowPlaying, timestamps bool
	}{
		{"bar text", "歌曲详情", "txt", false, false},
		{"playing lrc", "更多", "lrc", true, true},
		{"bar srt", "歌曲详情", "srt", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newTestApp()
			a.settings = defaultSettings()
			a.settings.DownloadFolder = t.TempDir()
			a.dl.loaded = true
			a.pl.Cue([]netease.Track{{ID: 7, Name: "song", Duration: 10000}})
			a.pl.SetLyrics(lyrics.Build("[00:01]lyrics", "", ""))
			a.nowPlaying = tc.nowPlaying
			tt := ui.NewTester(a.view, 1180, 800)
			tt.SetScale(0.3)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Advance(time.Second)
			tt.Frame()
			if err := chooseItem(tt, tc.button, "下载歌词"); err != nil {
				t.Fatal(err)
			}
			if !a.lyricDownload.open || a.lyricDownload.song.ID != 7 {
				t.Fatal("lyrics options did not open for this song")
			}
			if !tc.timestamps {
				if err := tt.Click("切换歌词时间戳"); err != nil {
					t.Fatal(err)
				}
			}
			if tc.format == "srt" {
				if err := chooseItem(tt, "歌词文件格式", "SRT"); err != nil {
					t.Fatal(err)
				}
			}
			old := chooseLyricPath
			defer func() { chooseLyricPath = old }()
			chosen := make(chan mygo.SaveDialogOptions, 1)
			chooseLyricPath = func(opts mygo.SaveDialogOptions) (string, error) { chosen <- opts; return "", nil }
			if err := tt.Click("选择保存位置"); err != nil {
				t.Fatal(err)
			}
			select {
			case opts := <-chosen:
				if filepath.Ext(opts.DefaultPath) != "."+tc.format {
					t.Fatalf("saved extension = %s", opts.DefaultPath)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("save action did not reach the file dialog")
			}
		})
	}
}

// Excluding translations applies to all formats, including simultaneous SRT lines.
func TestLyricsExportWithoutTranslations(t *testing.T) {
	lines := []lyrics.Line{
		{At: time.Second, End: 3 * time.Second, Text: "original", Translation: "translation"},
		{At: time.Second, End: 3 * time.Second, Text: "second original", Translation: "second translation"},
	}
	for _, format := range []string{"txt", "lrc", "srt"} {
		t.Run(format, func(t *testing.T) {
			text, err := lyricsFileText(lines, format, false, 0)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(text, "translation") || !strings.Contains(text, "original") || !strings.Contains(text, "second original") {
				t.Fatalf("original-only export = %q", text)
			}
			text, err = lyricsFileText(lines, format, true, 0)
			if err != nil || !strings.Contains(text, "translation") || !strings.Contains(text, "second translation") {
				t.Fatalf("bilingual export = %q, %v", text, err)
			}
			if lines[0].Translation != "translation" || lines[1].Translation != "second translation" {
				t.Fatal("export changed the playback cache")
			}
		})
	}
}

// The translation switch controls the saved file, and later option changes do not affect a pending export.
func TestLyricDownloadTranslationToggle(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.settings.DownloadFolder = t.TempDir()
	song := netease.Track{ID: 7, Name: "song"}
	a.pl.Cue([]netease.Track{song})
	a.pl.SetLyrics(lyrics.Build("[00:01]original", "[00:01]translation", ""))
	a.openLyricDownload(song)
	tt := ui.NewTester(func(c *ui.Context) { a.pal = paletteFor(c.Theme()); a.motion = motionOff; a.lyricDownloadDialog(c) }, 480, 400)
	if err := tt.Click("切换歌词翻译"); err != nil {
		t.Fatal(err)
	}
	if a.lyricDownload.translations {
		t.Fatal("translation switch did not turn off")
	}
	request := a.lyricDownloadRequest(song, "txt", a.lyricDownload.translations)
	if err := tt.Click("切换歌词翻译"); err != nil {
		t.Fatal(err)
	}
	if !a.lyricDownload.translations {
		t.Fatal("translation switch did not turn on")
	}
	old := chooseLyricPath
	defer func() { chooseLyricPath = old }()
	chooseLyricPath = func(opts mygo.SaveDialogOptions) (string, error) { return opts.DefaultPath, nil }
	path, err := request.save()
	if err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(path)
	if err != nil || string(text) != "original\r\n" {
		t.Fatalf("saved file = %q, %v", text, err)
	}
	lines, _ := a.pl.Lyrics()
	if lines[0].Translation != "translation" {
		t.Fatal("export removed the displayed translation")
	}
}

// Collapsing options removes their controls, retains the selected format and ignores hidden separation.
func TestLyricDownloadConditionalOptions(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	song := netease.Track{ID: 7, Name: "song"}
	a.openLyricDownload(song)
	tt := ui.NewTester(func(c *ui.Context) { a.pal = paletteFor(c.Theme()); a.motion = motionOff; a.lyricDownloadDialog(c) }, 480, 560)
	if err := chooseItem(tt, "歌词文件格式", "SRT"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := tt.Click("切换歌词时间戳"); err != nil {
			t.Fatal(err)
		}
		if _, ok := tt.Find("歌词文件格式"); ok {
			t.Fatal("format picker remained interactive without timestamps")
		}
		tt.Frame()
		if err := tt.Click("切换歌词时间戳"); err != nil {
			t.Fatal(err)
		}
		if _, ok := tt.Find("歌词文件格式"); !ok || a.lyricDownload.format != "SRT" {
			t.Fatal("reopening the format picker lost the selection")
		}
	}
	if err := tt.Click("切换分离翻译"); err != nil {
		t.Fatal(err)
	}
	if !a.lyricDownloadRequest(song, "srt", true).separate {
		t.Fatal("separation switch did not reach the export request")
	}
	if err := tt.Click("切换歌词翻译"); err != nil {
		t.Fatal(err)
	}
	if _, ok := tt.Find("切换分离翻译"); ok {
		t.Fatal("separation switch remained interactive without translations")
	}
	request := a.lyricDownloadRequest(song, "srt", a.lyricDownload.translations)
	if request.separate || request.translations {
		t.Fatal("a hidden separation option enabled translated output")
	}
	if err := tt.Click("切换歌词翻译"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("切换分离翻译"); err != nil {
		t.Fatal(err)
	}
	if a.lyricDownloadRequest(song, "srt", true).separate {
		t.Fatal("turning separation off did not restore a combined export")
	}
}

// Split exports produce two named files, preserve gaps in translation timing and leave playback lyrics intact.
func TestLyricDownloadSeparateFiles(t *testing.T) {
	old := chooseLyricFolder
	defer func() { chooseLyricFolder = old }()
	lines := []lyrics.Line{
		{At: time.Second, Text: "original one", Translation: "translation one"},
		{At: 3 * time.Second, Text: "untranslated"},
		{At: 6 * time.Second, Text: "original last", Translation: "translation last"},
	}
	for _, tc := range []struct{ format, original, translated string }{
		{"txt", "original one\r\nuntranslated\r\noriginal last\r\n", "translation one\r\ntranslation last\r\n"},
		{"lrc", "[00:01.000]original one\r\n[00:03.000]untranslated\r\n[00:06.000]original last\r\n", "[00:01.000]translation one\r\n[00:06.000]translation last\r\n"},
		{"srt", "1\r\n00:00:01,000 --> 00:00:03,000\r\noriginal one\r\n\r\n2\r\n00:00:03,000 --> 00:00:06,000\r\nuntranslated\r\n\r\n3\r\n00:00:06,000 --> 00:00:08,000\r\noriginal last\r\n\r\n", "1\r\n00:00:01,000 --> 00:00:03,000\r\ntranslation one\r\n\r\n2\r\n00:00:06,000 --> 00:00:08,000\r\ntranslation last\r\n\r\n"},
	} {
		t.Run(tc.format, func(t *testing.T) {
			dir := t.TempDir()
			calls := 0
			chooseLyricFolder = func(opts mygo.OpenDialogOptions) ([]string, error) {
				calls++
				if !opts.Directory || opts.DefaultPath != dir {
					t.Errorf("incorrect folder picker options: %+v", opts)
				}
				return []string{dir}, nil
			}
			r := lyricDownloadRequest{song: netease.Track{Name: "歌曲/名", Duration: 8000}, lines: lines, loaded: true, translations: true, separate: true, format: tc.format, folder: dir}
			if _, err := r.save(); err != nil || calls != 1 {
				t.Fatalf("split export = %v, chooser calls %d", err, calls)
			}
			for part, want := range map[string]string{"原词": tc.original, "翻译": tc.translated} {
				b, err := os.ReadFile(filepath.Join(dir, "歌曲-名-"+part+"."+tc.format))
				if err != nil || string(b) != want {
					t.Fatalf("%s file = %q, %v; want %q", part, b, err, want)
				}
			}
			files, _ := os.ReadDir(dir)
			if len(files) != 2 || lines[0].Text != "original one" || lines[0].Translation != "translation one" {
				t.Fatal("split export created extra files or changed the playback cache")
			}
		})
	}
}

// Cancelling either split dialog, absent translations and invalid companion paths cannot change existing files.
func TestLyricDownloadSeparateCancellation(t *testing.T) {
	oldFolder, oldConfirm := chooseLyricFolder, confirmLyricOverwrite
	defer func() { chooseLyricFolder, confirmLyricOverwrite = oldFolder, oldConfirm }()
	dir := t.TempDir()
	r := lyricDownloadRequest{song: netease.Track{Name: "song"}, loaded: true, translations: true, separate: true, format: "lrc", lines: lyrics.Build("[00:01]original", "[00:01]translation", "")}
	chooseLyricFolder = func(mygo.OpenDialogOptions) ([]string, error) { return nil, nil }
	if path, err := r.save(); path != "" || err != nil {
		t.Fatalf("cancelled folder selection = %q, %v", path, err)
	}
	chooseLyricFolder = func(mygo.OpenDialogOptions) ([]string, error) { return []string{dir}, nil }
	path := filepath.Join(dir, "song-翻译.lrc")
	if err := os.WriteFile(path, []byte("keep translation"), 0o644); err != nil {
		t.Fatal(err)
	}
	confirmed := false
	confirmLyricOverwrite = func(opts mygo.MessageOptions) (mygo.MessageResult, error) {
		confirmed = true
		if !strings.Contains(opts.Detail, "song-翻译.lrc") {
			t.Error("overwrite dialog omitted the existing companion file")
		}
		return mygo.MessageResult{Button: 0}, nil
	}
	if result, err := r.save(); result != "" || err != nil || !confirmed {
		t.Fatalf("cancelled overwrite = %q, %v, confirmed %v", result, err, confirmed)
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != "keep translation" {
		t.Fatalf("cancel changed translation: %q, %v", b, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "song-原词.lrc")); !os.IsNotExist(err) {
		t.Fatal("cancel wrote the original file")
	}
	confirmLyricOverwrite = func(mygo.MessageOptions) (mygo.MessageResult, error) { return mygo.MessageResult{Button: 1}, nil }
	if _, err := r.save(); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != "[00:01.000]translation\r\n" {
		t.Fatalf("confirmed overwrite = %q, %v", b, err)
	}
	r.song.Name = "blocked"
	if err := os.Mkdir(filepath.Join(dir, "blocked-翻译.lrc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := r.save(); err == nil {
		t.Fatal("a directory accepted as a companion file")
	}
	if _, err := os.Stat(filepath.Join(dir, "blocked-原词.lrc")); !os.IsNotExist(err) {
		t.Fatal("invalid companion path still wrote the original")
	}
	chooseLyricFolder = func(mygo.OpenDialogOptions) ([]string, error) {
		t.Error("absent translation opened picker")
		return nil, nil
	}
	r.lines = lyrics.Build("[00:01]original", "", "")
	if _, err := r.save(); err == nil {
		t.Fatal("missing translation exported as an empty companion")
	}
}

// A simultaneous line without a translation must still contribute its end time to the translated SRT cue.
func TestLyricDownloadSeparateSimultaneousCues(t *testing.T) {
	dir := t.TempDir()
	old := chooseLyricFolder
	defer func() { chooseLyricFolder = old }()
	chooseLyricFolder = func(mygo.OpenDialogOptions) ([]string, error) { return []string{dir}, nil }
	r := lyricDownloadRequest{song: netease.Track{Name: "song"}, loaded: true, translations: true, separate: true, format: "srt", lines: []lyrics.Line{
		{At: time.Second, End: 4 * time.Second, Text: "untranslated"},
		{At: time.Second, End: 2 * time.Second, Text: "original", Translation: "translation"},
	}}
	if _, err := r.save(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "song-翻译.srt"))
	want := "1\r\n00:00:01,000 --> 00:00:04,000\r\ntranslation\r\n\r\n"
	if err != nil || string(b) != want {
		t.Fatalf("translated cue lost a simultaneous line's end time: %q, %v", b, err)
	}
}

// Controls remain clickable while the dialog resizes, and the final choices still select a split export.
func TestLyricDownloadDuringResize(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	song := netease.Track{ID: 7, Name: "song"}
	a.pl.Cue([]netease.Track{song})
	a.pl.SetLyrics(lyrics.Build("[00:01]original", "[00:01]translation", ""))
	a.openLyricDownload(song)
	a.lyricDownload.timestamps, a.lyricDownload.translations = false, false
	tt := ui.NewTester(func(c *ui.Context) { a.pal = paletteFor(c.Theme()); a.motion = motionFull; a.lyricDownloadDialog(c) }, 600, 620)
	tt.Advance(time.Second)
	for _, control := range []string{"切换歌词时间戳", "切换歌词翻译", "切换分离翻译"} {
		if err := tt.Click(control); err != nil {
			t.Fatal(err)
		}
		tt.Advance(60 * time.Millisecond)
	}
	tt.Advance(time.Second)
	if err := chooseItem(tt, "歌词文件格式", "SRT"); err != nil {
		t.Fatal(err)
	}
	old := chooseLyricFolder
	defer func() { chooseLyricFolder = old }()
	chosen := make(chan bool, 1)
	chooseLyricFolder = func(opts mygo.OpenDialogOptions) ([]string, error) { chosen <- opts.Directory; return nil, nil }
	if err := tt.Click("选择保存位置"); err != nil {
		t.Fatal(err)
	}
	select {
	case directory := <-chosen:
		if !directory || !a.lyricDownload.separate || a.lyricDownload.format != "SRT" {
			t.Fatal("resizing lost an export option")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("save button did not reach the folder picker after resizing")
	}
}
