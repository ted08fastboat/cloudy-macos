//go:build !windows

package app

// primaryButtonDown cannot tell off Windows: the mini window's grips follow
// the pointer only while it is over them.
func primaryButtonDown() (down, known bool) { return false, false }
