package app

// Where listeners reach the app's author and its installers, besides
// GitHub, which many of their networks cannot reach.

// updateMirrors are the prefixes the updater fetches a release on GitHub
// through when GitHub does not answer: public GitHub proxies, measured
// reachable from China. The updates are verified with the app's key
// whatever they come through, so a proxy only carries the bytes.
var updateMirrors = []string{
	"https://gh-proxy.com/",
	"https://ghfast.top/",
}

// manualDownloadURL is the cloud drive share of the installers, offered
// when an update cannot be fetched at all: the author's Quark folder, which
// each release's installers are put in, the link and its code the same.
var manualDownloadURL = "https://pan.quark.cn/s/58e05deef9f6?pwd=zBds"

// feedbackBilibiliUID is the author's Bilibili account, whose private
// messages feedback goes to.
var feedbackBilibiliUID int64 = 229474180
