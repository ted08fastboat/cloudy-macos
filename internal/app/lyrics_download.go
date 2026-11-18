package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/download"
	"netease-music-mygo/internal/local"
	"netease-music-mygo/internal/lyrics"
	"netease-music-mygo/internal/netease"
)

type lyricDownloadState struct {
	open, saving bool
	song         netease.Track
	timestamps   bool
	translations bool
	separate     bool
	format       string
}

func (a *app) openLyricDownload(song netease.Track) {
	if a.lyricDownload.saving {
		a.toast = "正在保存歌词，请稍候"
		return
	}
	a.lyricDownload = lyricDownloadState{open: true, song: song, timestamps: true, translations: true, format: "LRC"}
	a.redraw()
}

func (a *app) lyricDownloadDialog(c *ui.Context) {
	s := &a.lyricDownload
	a.dialog(c, "download-lyrics", 0.25, &s.open, func(panel ui.Element) {
		ink := a.inkOn(a.nowPlaying)
		panel.Width(480).Clip().TextColor(ink.text).Label("下载歌词")
		// Animate layout heights directly: overlay layout runs again after
		// Element.Transition and would otherwise snap the dialog to its new size.
		reveal := func(key string, shown bool, height float32, build func() ui.Element) {
			group := ui.Column(c.Key(key)).Shrink(0).Clip()
			k := b2f(shown)
			if a.motion > motionOff {
				k = clamp01(group.AnimateTo("expanded", k, motionPane))
			}
			if shown || k > 0 {
				group.Disabled(!shown).Children(func() {
					content := build().Margin(18, 0, 0, 0).Shrink(0)
					if measured := content.LayoutBounds().H; measured > 0 {
						height = measured
					}
				})
			}
			group.Height((height + 18) * k).Opacity(k)
		}
		a.pane(panel, 26, a.nowPlaying)
		panel.Children(func() {
			dialogHeader(c, ink, "下载歌词", s.song.Name, func() { s.open = false })
			ui.Column(c.Key("options")).PaddingX(28).Children(func() {
				ui.Row(c.Key("timestamp-row")).Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "包含时间戳").FontSize(14)
					ui.Switch(c.Key("timestamps"), &s.timestamps).Label("切换歌词时间戳")
				})
				reveal("format-section", s.timestamps, 32, func() ui.Element {
					return ui.Row(c.Key("format-row")).Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "时间戳格式").FontSize(14)
						a.paneSelect(c.Key("format"), "歌词文件格式", &s.format, []string{"LRC", "SRT"})
					})
				})
				hint := "LRC 保留歌词时间轴，适合音乐播放器。"
				if !s.timestamps {
					hint = "保存为 TXT 纯文本，不包含时间戳。"
				} else if s.format == "SRT" {
					hint = "SRT 包含字幕起止时间，适合导入 PR、达芬奇等剪辑软件。"
				}
				ui.Text(c.Key("format-hint"), hint).Margin(18, 0, 0, 0).FontSize(12.5).LineHeight(1.5).TextColor(ink.muted)
				ui.Row(c.Key("translation-row")).Margin(18, 0, 0, 0).Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "包含翻译").FontSize(14)
					ui.Switch(c.Key("translations"), &s.translations).Label("切换歌词翻译")
				})
				reveal("translation-section", s.translations, 20, func() ui.Element {
					return ui.Row(c.Key("separate-row")).Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "分离翻译").FontSize(14)
						ui.Switch(c.Key("separate"), &s.separate).Label("切换分离翻译")
					})
				})
				reveal("separate-explanation", s.translations && s.separate, 20, func() ui.Element {
					return ui.Text(c.Key("separate-hint"), "在所选文件夹中保存为“歌曲名-原词”和“歌曲名-翻译”两个文件。").FontSize(12.5).LineHeight(1.5).TextColor(ink.muted)
				})
			})
			dialogFooter(c, func() {
				if settingButton(c.Key("cancel"), ink, "取消", nil, false, false).Clicked() {
					s.open = false
				}
				if primaryButton(c.Key("save"), "选择保存位置", iconDownload, a.pal, false).Disabled(s.saving).Clicked() && !s.saving {
					format := "txt"
					if s.timestamps {
						format = strings.ToLower(s.format)
					}
					r := a.lyricDownloadRequest(s.song, format, s.translations)
					s.open, s.saving = false, true
					go func() {
						path, err := r.save()
						a.post(func() {
							s.saving = false
							if err != nil {
								a.toast = "下载歌词失败：" + err.Error()
							} else if path != "" {
								a.toast = "歌词已保存：" + path
							}
						})
					}()
				}
			})
		})
	})
}

// A request keeps the song, options and cached lyrics from the UI thread.
// Changing tracks while a file dialog or network request is open cannot change it.
type lyricDownloadRequest struct {
	song         netease.Track
	lines        []lyrics.Line
	loaded       bool
	format       string
	translations bool
	separate     bool
	folder       string
	parent       *mygo.Window
}

func (a *app) lyricDownloadRequest(song netease.Track, format string, translations bool) lyricDownloadRequest {
	r := lyricDownloadRequest{song: song, format: format, translations: translations, separate: translations && a.lyricDownload.separate, folder: a.downloadFolder(), parent: a.win}
	if current := a.pl.Current(); current != nil && current.ID == song.ID && current.LocalPath == song.LocalPath {
		lines, loaded := a.pl.Lyrics()
		r.lines, r.loaded = append([]lyrics.Line(nil), lines...), loaded
	}
	return r
}

// Tests replace the network and native save dialog; file writes remain real.
var downloadLyricsOf = netease.Lyrics
var chooseLyricPath = mygo.Dialog.Save
var chooseLyricFolder = mygo.Dialog.Open
var confirmLyricOverwrite = mygo.Dialog.Message

func (r lyricDownloadRequest) load() ([]lyrics.Line, error) {
	if r.loaded {
		return r.lines, nil
	}
	var fetchErr error
	if r.song.ID > 0 {
		lrc, trans, yrc, err := downloadLyricsOf(r.song.ID)
		fetchErr = err
		if err == nil {
			if lines := lyrics.Build(lrc, trans, yrc); len(lines) > 0 {
				return lines, nil
			}
		}
	}
	if r.song.LocalPath != "" {
		b, err := os.ReadFile(download.LyricsPath(r.song.LocalPath))
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if lines := lyrics.FromText(string(b)); len(lines) > 0 {
			return lines, nil
		}
		if lines := lyrics.FromText(local.LyricsOf(r.song.LocalPath)); len(lines) > 0 {
			return lines, nil
		}
	}
	return nil, fetchErr
}

func (r lyricDownloadRequest) save() (string, error) {
	lines, err := r.load()
	if err != nil {
		return "", err
	}
	if r.translations && r.separate {
		return r.saveSeparate(lines)
	}
	text, err := lyricsFileText(lines, r.format, r.translations, time.Duration(r.song.Duration)*time.Millisecond)
	if err != nil {
		return "", err
	}
	path, err := chooseLyricPath(mygo.SaveDialogOptions{
		Parent: r.parent, Title: "下载歌词", ButtonLabel: "保存",
		DefaultPath:       filepath.Join(r.folder, download.FileName(r.song, r.format)),
		Filters:           []mygo.FileFilter{{Name: strings.ToUpper(r.format) + " 文件", Extensions: []string{r.format}}},
		CreateDirectories: true,
	})
	if err != nil || path == "" {
		return "", err
	}
	ext := "." + r.format
	if filepath.Ext(path) == "" {
		path += ext
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			return "", fmt.Errorf("请在保存窗口中选择 %s 以确认覆盖", filepath.Base(path))
		}
	} else if !strings.EqualFold(filepath.Ext(path), ext) {
		return "", fmt.Errorf("文件扩展名应为 %s", ext)
	}
	if err := writeFileAtomic(path, []byte(text), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func (r lyricDownloadRequest) saveSeparate(lines []lyrics.Line) (string, error) {
	duration := time.Duration(r.song.Duration) * time.Millisecond
	original, err := lyricsFileText(lines, r.format, false, duration)
	if err != nil {
		return "", err
	}
	translated := make([]lyrics.Line, len(lines))
	for i, line := range lines {
		// Retain even untranslated lines' boundaries so SRT cues end at the
		// same time in both files, rather than lingering across a missing line.
		translated[i] = lyrics.Line{At: line.At, End: line.End, Text: line.Translation}
	}
	translation, err := lyricsFileText(translated, r.format, false, duration)
	if err != nil {
		return "", errors.New("这首歌暂无翻译，请关闭分离翻译或包含翻译后重试")
	}
	folders, err := chooseLyricFolder(mygo.OpenDialogOptions{
		Parent: r.parent, Title: "选择歌词保存文件夹", ButtonLabel: "保存到此文件夹",
		DefaultPath: r.folder, Directory: true, CreateDirectories: true,
	})
	if err != nil || len(folders) == 0 {
		return "", err
	}
	names := []string{download.LyricFileName(r.song, "原词", r.format), download.LyricFileName(r.song, "翻译", r.format)}
	paths := []string{filepath.Join(folders[0], names[0]), filepath.Join(folders[0], names[1])}
	var existing []string
	for i, path := range paths {
		info, err := os.Lstat(path)
		if err == nil {
			if !info.Mode().IsRegular() {
				return "", fmt.Errorf("无法覆盖 %s：目标不是普通文件", names[i])
			}
			existing = append(existing, names[i])
		} else if !os.IsNotExist(err) {
			return "", err
		}
	}
	if len(existing) > 0 {
		result, err := confirmLyricOverwrite(mygo.MessageOptions{
			Parent: r.parent, Type: mygo.MessageQuestion, Title: "覆盖歌词文件",
			Message: "以下文件已存在，是否覆盖？", Detail: strings.Join(existing, "\n"),
			Buttons: []string{"取消", "覆盖"}, DefaultButton: 0, CancelButton: 0,
		})
		if err != nil || result.Button != 1 {
			return "", err
		}
	}
	if err := writeFileAtomic(paths[0], []byte(original), 0o644); err != nil {
		return "", err
	}
	if err := writeFileAtomic(paths[1], []byte(translation), 0o644); err != nil {
		return "", fmt.Errorf("原词已保存至 %s，翻译保存失败：%w", paths[0], err)
	}
	return strings.Join(paths, "；"), nil
}

func lyricsFileText(lines []lyrics.Line, format string, translations bool, duration time.Duration) (string, error) {
	if format != "txt" && format != "lrc" && format != "srt" {
		return "", errors.New("不支持的歌词格式")
	}
	if format != "txt" && lyrics.IsPlain(lines) {
		return "", errors.New("这首歌没有时间轴，请关闭时间戳后保存为 TXT")
	}
	var out strings.Builder
	cue := 0
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		text := strings.TrimSpace(line.Text)
		if text == "" && format != "srt" {
			continue
		}
		trans := ""
		if translations {
			trans = strings.TrimSpace(line.Translation)
		}
		switch format {
		case "txt":
			out.WriteString(text + "\r\n")
			if trans != "" {
				out.WriteString(trans + "\r\n")
			}
		case "lrc":
			ms := max(0, line.At.Milliseconds())
			stamp := fmt.Sprintf("[%02d:%02d.%03d]", ms/60000, ms/1000%60, ms%1000)
			out.WriteString(stamp + text + "\r\n")
			if trans != "" {
				out.WriteString(stamp + trans + "\r\n")
			}
		case "srt":
			start, end := max(0, line.At), line.End
			if trans != "" {
				if text != "" {
					text += "\r\n"
				}
				text += trans
			}
			// Lines sharing a timestamp form one bilingual or multiline cue.
			for i+1 < len(lines) && lines[i+1].At == line.At {
				i++
				other := lines[i]
				for j, extra := range []string{other.Text, other.Translation} {
					if j == 1 && !translations {
						continue
					}
					if extra = strings.TrimSpace(extra); extra != "" {
						if text != "" {
							text += "\r\n"
						}
						text += extra
					}
				}
				end = max(end, other.End)
			}
			if text == "" {
				continue
			}
			if end <= start {
				end = start + 5*time.Second
			}
			if i+1 < len(lines) && lines[i+1].At > start {
				end = min(end, lines[i+1].At)
			}
			if duration > start {
				end = min(end, duration)
			}
			if end.Milliseconds() <= start.Milliseconds() {
				end = start + time.Millisecond
			}
			cue++
			fmt.Fprintf(&out, "%d\r\n%s --> %s\r\n%s\r\n\r\n", cue, subtitleTime(start), subtitleTime(end), text)
		}
	}
	if out.Len() == 0 {
		return "", errors.New("这首歌暂无歌词")
	}
	return out.String(), nil
}

func subtitleTime(at time.Duration) string {
	ms := at.Milliseconds()
	return fmt.Sprintf("%02d:%02d:%02d,%03d", ms/3600000, ms/60000%60, ms/1000%60, ms%1000)
}
