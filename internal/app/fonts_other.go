//go:build !windows && !darwin

package app

// systemFonts are the fonts the app offers besides its own: the Chinese
// fonts a Linux desktop usually has, whichever of them is installed.
func systemFonts() []uiFont {
	return []uiFont{
		{id: "noto-sans", label: "思源黑体", family: "Noto Sans CJK SC"},
		{id: "noto-serif", label: "思源宋体", family: "Noto Serif CJK SC"},
		{id: "source-han-sans", label: "Source Han Sans SC", family: "Source Han Sans SC"},
		{id: "wqy", label: "文泉驿微米黑", family: "WenQuanYi Micro Hei"},
	}
}
