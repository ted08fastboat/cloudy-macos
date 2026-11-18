//go:build !windows

package app

// showAtOnce does nothing off Windows, which shows windows at once.
func showAtOnce(hwnd uintptr) {}
