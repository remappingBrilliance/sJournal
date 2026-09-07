#!/usr/bin/env bash
# Builds the sjournal binary. Works around a missing libXxf86vm.so dev
# symlink (present on some distros only via the -devel package) by pointing
# the linker at a local symlink to the runtime .so.1 instead of requiring
# root to install libXxf86vm-devel.
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

CGO_LDFLAGS="-L$LIBDIR" go build -o sjournal .
echo "Built ./sjournal"
