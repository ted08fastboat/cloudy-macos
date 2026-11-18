#!/bin/sh
# Builds Cloudy as a macOS app: the .app and the .dmg in build/, signed with
# an ad hoc signature, which is enough to run it on the Mac that built it.
#
# Set DEVELOPER_ID to sign for other Macs, and to have mygo build notarize
# it: DEVELOPER_ID="Developer ID Application: You (TEAMID)".
#
# Needs Go 1.27 or later. It is not on the PATH by default here, so set
# GO_BIN to where it is, or install it with Homebrew.
set -eu

cd "$(dirname "$0")/.."

GO_BIN="${GO_BIN:-$HOME/.local/go/bin}"
if [ -x "$GO_BIN/go" ]; then
	PATH="$GO_BIN:$PATH"
fi
if ! command -v go >/dev/null 2>&1; then
	echo "Go 1.27 or later is needed: https://go.dev/dl/" >&2
	exit 1
fi

# The module proxy, for networks where proxy.golang.org is slow or blocked.
GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
export GOPROXY

# The icon macOS shows: on the canvas and in the squircle a Mac lays an icon
# out in, where the Windows one fills its square.
go run ./icon/macos

# -sign - is the ad hoc signature; without it, mygo build signs only what
# macos.signingIdentity names, and an unsigned app is refused on some Macs.
if [ -n "${DEVELOPER_ID:-}" ]; then
	go tool mygo build -sign "$DEVELOPER_ID"
else
	go tool mygo build -sign -
fi

echo
echo "Cloudy.app: build/darwin-arm64/Cloudy.app"
echo "disk image: build/darwin-arm64/*.dmg"
