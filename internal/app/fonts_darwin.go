//go:build darwin

package app

// systemFonts are the fonts the app offers besides its own: macOS's Chinese
// fonts, the ones a Mac reads Chinese in. PingFang SC is the system's own,
// so the default already is it; the others are there to read the app in, as
// Windows's are there.
func systemFonts() []uiFont {
	return []uiFont{
		{id: "pingfang", label: "苹方-简", family: "PingFang SC"},
		{id: "hiragino", label: "冬青黑体简体中文", family: "Hiragino Sans GB"},
		{id: "heiti", label: "黑体-简", family: "Heiti SC"},
		{id: "songti", label: "宋体-简", family: "Songti SC"},
		{id: "kaiti", label: "楷体-简", family: "Kaiti SC"},
		{id: "yuanti", label: "圆体-简", family: "Yuanti SC"},
	}
}
