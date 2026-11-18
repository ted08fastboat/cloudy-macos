//go:build windows

package app

// systemFonts are the fonts the app offers besides its own, those Windows
// comes with in Chinese.
func systemFonts() []uiFont {
	return []uiFont{
		{id: "yahei", label: "微软雅黑", family: "Microsoft YaHei UI"},
		{id: "dengxian", label: "等线", family: "DengXian"},
		{id: "simhei", label: "黑体", family: "SimHei"},
		{id: "noto-serif", label: "思源宋体", family: "Noto Serif SC"},
		{id: "simsun", label: "宋体", family: "SimSun"},
		{id: "kaiti", label: "楷体", family: "KaiTi"},
		{id: "fangsong", label: "仿宋", family: "FangSong"},
	}
}
