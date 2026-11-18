package app

var procSetWindowPosMini = user32.NewProc("SetWindowPos")

// sinkWindow puts the window hwnd under all the others, over the desktop,
// without activating, moving or resizing it.
func sinkWindow(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	const hwndBottom = 1
	const swpNoSize, swpNoMove, swpNoActivate = 0x0001, 0x0002, 0x0010
	procSetWindowPosMini.Call(hwnd, hwndBottom, 0, 0, 0, 0, swpNoSize|swpNoMove|swpNoActivate)
}
