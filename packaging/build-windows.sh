#!/usr/bin/env bash
# Cross-compiles the Windows build of sJournal from Linux and assembles a
# distributable folder under packaging/dist/windows/ containing:
#
#   sjournal.exe     the GUI app (no console window)
#   sjournal.ico     icon for the Start Menu / Desktop shortcut
#   install.ps1      per-user installer (Start Menu entry)
#   uninstall.ps1
#
# Requirements (all present on a typical Fedora/EL dev box):
#   - Go toolchain
#   - mingw-w64 gcc            (package: mingw64-gcc / mingw-w64-gcc)
#   - rsvg-convert             (package: librsvg2-tools) for the icon
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$here/.." && pwd)"
dist="$here/dist/windows"

CC="${CC:-x86_64-w64-mingw32-gcc}"
command -v "$CC" >/dev/null 2>&1 || {
	echo "error: $CC not found — install the mingw-w64 gcc package" >&2
	exit 1
}

rm -rf "$dist"
mkdir -p "$dist"

# --- icon -------------------------------------------------------------
if command -v rsvg-convert >/dev/null 2>&1; then
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	for sz in 16 24 32 48 64 128 256; do
		rsvg-convert -w "$sz" -h "$sz" "$repo/internal/ui/assets/icon.svg" -o "$tmp/$sz.png"
	done
	( cd "$repo" && go run ./packaging/mkicon "$dist/sjournal.ico" \
		"$tmp"/16.png "$tmp"/24.png "$tmp"/32.png "$tmp"/48.png \
		"$tmp"/64.png "$tmp"/128.png "$tmp"/256.png )
	echo ">> built sjournal.ico"
else
	echo ">> rsvg-convert missing — skipping .ico (shortcut will use a generic icon)"
fi

# --- exe -------------------------------------------------------------
echo ">> cross-compiling sjournal.exe"
( cd "$repo" && \
	CGO_ENABLED=1 CC="$CC" GOOS=windows GOARCH=amd64 \
	go build -trimpath -ldflags "-s -w -H windowsgui" -o "$dist/sjournal.exe" . )

cp "$here/install-windows.ps1"   "$dist/install.ps1"
cp "$here/uninstall-windows.ps1" "$dist/uninstall.ps1"

echo
echo "Windows package ready:  $dist"
echo "Copy that folder to the Windows machine and run in PowerShell:"
echo "    powershell -ExecutionPolicy Bypass -File .\\install.ps1"
