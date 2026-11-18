//go:build !windows

package app

// watchAltTab does nothing off Windows, which alone has the thumbnail.
func (a *app) watchAltTab() {}
