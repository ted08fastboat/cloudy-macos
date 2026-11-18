package app

var procGetSystemMetricsMini = user32.NewProc("GetSystemMetrics")

// primaryButtonDown reports whether the primary mouse button is down now,
// wherever the pointer is, and that it could tell.
func primaryButtonDown() (down, known bool) {
	vk := uintptr(0x01)                                                   // VK_LBUTTON
	if swapped, _, _ := procGetSystemMetricsMini.Call(23); swapped != 0 { // SM_SWAPBUTTON
		vk = 0x02
	}
	r, _, _ := procGetAsyncKeyState.Call(vk)
	return r&0x8000 != 0, true
}
