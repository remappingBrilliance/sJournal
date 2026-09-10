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

# If sJournal has been installed per-user (packaging/install-linux.sh), drop
# the fresh binary in over the installed one so a rebuild is enough to update
# it. Only the executable is refreshed — not the launcher, icons or config.
# Write a temp file then rename it into place: overwriting a running
# executable in-place fails with ETXTBSY, but an atomic rename works (the
# running process keeps its old inode until it exits).
if [ -z "${SJOURNAL_NO_INSTALL:-}" ]; then
	prefix="${XDG_DATA_HOME:-$HOME/.local}"
	prefix="${prefix%/share}"                  # accept ~/.local or ~/.local/share
	installed="$prefix/lib/sjournal/sjournal"
	if [ -d "$(dirname "$installed")" ]; then
		cp sjournal "$installed.new"
		chmod 0755 "$installed.new"
		mv -f "$installed.new" "$installed"
		echo "Updated $installed (restart sJournal to pick it up)"
	fi
fi
