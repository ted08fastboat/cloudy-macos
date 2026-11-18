package app

import "runtime"

// pathIgnoresCase tells whether the system compares paths without their
// case: Windows does, and so does macOS's own filesystem, APFS, as it comes
// formatted. Those of Linux and the others keep the case.
func pathIgnoresCase() bool {
	return runtime.GOOS == "windows" || runtime.GOOS == "darwin"
}
