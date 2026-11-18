package app

import "unsafe"

var procDwmSetWindowAttrFly = dwmapi.NewProc("DwmSetWindowAttribute")

// showAtOnce has the window hwnd show and hide without the system's fade:
// what it draws moves from its first frame, which a fade would hide.
func showAtOnce(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	const dwmwaTransitionsForceDisabled = 3
	on := int32(1)
	procDwmSetWindowAttrFly.Call(hwnd, dwmwaTransitionsForceDisabled, uintptr(unsafe.Pointer(&on)), unsafe.Sizeof(on))
}
