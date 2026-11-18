//go:build !windows

package app

// sinkWindow does nothing off Windows: the window is just not kept on top.
func sinkWindow(hwnd uintptr) {}
