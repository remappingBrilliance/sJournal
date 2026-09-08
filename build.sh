#!/usr/bin/env bash
# Builds the sjournal binary.
#
# On a Wayland session it builds Fyne's native Wayland backend (-tags
# wayland). This matters for more than cosmetics: without it Fyne runs under
# XWayland with no usable WM_CLASS, so GNOME/dash-to-panel can't match the
# window to its .desktop file and shows a generic icon. Native Wayland sets
# xdg_toplevel app_id = io.lottkowitz.sjournal, which resolves the icon.
# Force the portable X11/XWayland build with SJOURNAL_X11=1.
#
# Also works around a missing libXxf86vm.so dev symlink (shipped only in the
# -devel package on some distros) by pointing the linker at a local symlink
# to the runtime .so.1 — glfw links libXxf86vm even in the Wayland build.
set -euo pipefail
cd "$(dirname "$0")"

LIBDIR="$(pwd)/.buildlibs"
mkdir -p "$LIBDIR"
if [ ! -e "$LIBDIR/libXxf86vm.so" ]; then
	RUNTIME_LIB="$(ldconfig -p | awk '/libXxf86vm\.so\.1/{print $NF; exit}')"
	if [ -n "$RUNTIME_LIB" ]; then
		ln -sf "$RUNTIME_LIB" "$LIBDIR/libXxf86vm.so"
	fi
fi

TAGS=()
if [ -z "${SJOURNAL_X11:-}" ] && [ -n "${WAYLAND_DISPLAY:-}" ] && pkg-config --exists wayland-client 2>/dev/null; then
	TAGS=(-tags wayland)
	echo "Building native Wayland backend (SJOURNAL_X11=1 to force X11/XWayland)"
fi

CGO_LDFLAGS="-L$LIBDIR" go build "${TAGS[@]}" -o sjournal .
echo "Built ./sjournal"
