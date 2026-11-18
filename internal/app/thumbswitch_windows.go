package app

import (
	"syscall"
	"unsafe"
)

// Alt+Tab, unlike the taskbar, shows the window as itself: a cover in its
// place is drawn no larger than DWM asks for, which there is shorter than
// the windows beside it, and DWM refuses a larger one. Nothing tells a window
// which of the two asks, so the cover gives way while Alt+Tab is open.
//
// Windows 11 gives the foreground to a window of class ForegroundStaging as
// Alt+Tab opens, a moment before DWM asks for the thumbnails, and to another
// as it closes. Windows 10 says so with EVENT_SYSTEM_SWITCHSTART and
// SWITCHEND, and its Alt+Tab is MultitaskingViewFrame.

const (
	eventSystemForeground  = 0x0003
	eventSystemSwitchStart = 0x0014
	eventSystemSwitchEnd   = 0x0015
	wineventOutOfContext   = 0x0000
)

var (
	procSetWinEventHook = user32.NewProc("SetWinEventHook")
	altTabHook          uintptr // the callback, made once
)

// watchAltTab follows Alt+Tab opening and closing, on the main thread, whose
// message loop the hook's events come through.
func (a *app) watchAltTab() {
	if altTabHook != 0 {
		return
	}
	altTabHook = syscall.NewCallback(func(hook, event, hwnd, object, child, thread, at uintptr) uintptr {
		open := a.altTab
		switch event {
		case eventSystemSwitchStart:
			open = true
		case eventSystemSwitchEnd:
			open = false
		case eventSystemForeground:
			cls := windowClass(hwnd)
			open = cls == "ForegroundStaging" || cls == "MultitaskingViewFrame"
		}
		if open != a.altTab {
			a.altTab = open
			a.syncThumbs()
		}
		return 0
	})
	procSetWinEventHook.Call(eventSystemForeground, eventSystemForeground, 0, altTabHook, 0, 0, wineventOutOfContext)
	procSetWinEventHook.Call(eventSystemSwitchStart, eventSystemSwitchEnd, 0, altTabHook, 0, 0, wineventOutOfContext)
}

func windowClass(hwnd uintptr) string {
	var buf [64]uint16
	procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf[:])
}
