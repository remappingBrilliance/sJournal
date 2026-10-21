#!/usr/bin/env bash
# Installs sJournal as a GNOME/KDE desktop application.
#
# By default this is a per-user install (no root): binary under
# ~/.local/lib, launcher + icon under ~/.local/share, so it shows up in the
# applications menu and its notifications resolve correctly on Wayland.
#
#   ./install-linux.sh                 # per-user install
#   sudo ./install-linux.sh --system   # system-wide (/usr/local)
#   ./install-linux.sh --notes-dir ~/Journal
#   ./build.sh && sudo ./install-linux.sh --system --no-build   # build as yourself, install as root
#   ./install-linux.sh --uninstall
set -euo pipefail

APP_ID="io.lottkowitz.sjournal"
here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$here/.." && pwd)"

mode="user"
notes_dir=""
do_uninstall=0
no_build=0
while [ $# -gt 0 ]; do
	case "$1" in
		--system) mode="system" ;;
		--user) mode="user" ;;
		--notes-dir) notes_dir="$2"; shift ;;
		--uninstall) do_uninstall=1 ;;
		--no-build) no_build=1 ;;
		-h|--help) sed -n '2,13p' "$0"; exit 0 ;;
		*) echo "unknown option: $1" >&2; exit 2 ;;
	esac
	shift
done

if [ "$mode" = "system" ]; then
	prefix="/usr/local"
	[ "$(id -u)" -eq 0 ] || { echo "--system needs root (use sudo)" >&2; exit 1; }
else
	prefix="${XDG_DATA_HOME:-$HOME/.local}"
	prefix="${prefix%/share}"          # accept either ~/.local or ~/.local/share
fi

libdir="$prefix/lib/sjournal"
bindir="$prefix/bin"
appsdir="$prefix/share/applications"
icondir="$prefix/share/icons/hicolor"
# Data lives outside the install tree so upgrades never touch it.
data_home="${SJOURNAL_HOME:-$HOME/.local/share/sjournal}"

seed_config() {
	cat <<EOF
remind_me:
    start_time: "09:00"
    end_time: "17:00"
    interval_minutes: 60
    enabled: false
timestamp_default: false
notes_folder: $1
single_instance: true
require_password_for_edit: false
minimize_to_tray: false
EOF
}

desktop_file="$appsdir/$APP_ID.desktop"
wrapper="$bindir/sjournal"

if [ "$do_uninstall" -eq 1 ]; then
	rm -f "$wrapper" "$desktop_file"
	rm -rf "$libdir"
	find "$icondir" -name "$APP_ID.*" -delete 2>/dev/null || true
	command -v update-desktop-database >/dev/null && update-desktop-database "$appsdir" 2>/dev/null || true
	echo "Removed sJournal. Your journal data in $data_home was left untouched."
	exit 0
fi

# --- build --------------------------------------------------------------
if [ "$no_build" -eq 0 ]; then
	echo ">> building sjournal"
	"$repo/build.sh"
fi
[ -x "$repo/sjournal" ] || { echo "no built binary at $repo/sjournal (run ./build.sh)" >&2; exit 1; }

# --- lay down files ----------------------------------------------------
echo ">> installing to $prefix"
mkdir -p "$libdir" "$bindir" "$appsdir" "$icondir/scalable/apps"
[ "$mode" = "system" ] || mkdir -p "$data_home"
install -m 0755 "$repo/sjournal" "$libdir/sjournal"
install -m 0644 "$repo/internal/ui/assets/icon.svg" "$icondir/scalable/apps/$APP_ID.svg"

if command -v rsvg-convert >/dev/null 2>&1; then
	for sz in 16 24 32 48 64 128 256 512; do
		mkdir -p "$icondir/${sz}x${sz}/apps"
		rsvg-convert -w "$sz" -h "$sz" "$repo/internal/ui/assets/icon.svg" \
			-o "$icondir/${sz}x${sz}/apps/$APP_ID.png"
	done
fi

if [ "$mode" = "system" ]; then
	# Resolved per user at launch: $HOME here is root's, not the end user's.
	# First run seeds that user's config with notes in their Documents folder.
	cat > "$wrapper" <<EOF
#!/bin/sh
# sJournal launcher (system-wide) — per-user data, kept out of the install tree.
data="\${SJOURNAL_HOME:-\${XDG_DATA_HOME:-\$HOME/.local/share}/sjournal}"
if [ ! -e "\$data/config.yaml" ]; then
	docs="\$HOME/Documents"
	command -v xdg-user-dir >/dev/null 2>&1 && docs="\$(xdg-user-dir DOCUMENTS)"
	mkdir -p "\$data" "\$docs/sJournal"
	cat > "\$data/config.yaml" <<CFG
$(seed_config '$docs/sJournal')
CFG
fi
export SJOURNAL_HOME="\$data"
exec "$libdir/sjournal" "\$@"
EOF
else
	cat > "$wrapper" <<EOF
#!/bin/sh
# sJournal launcher — keeps journal data out of the install tree.
export SJOURNAL_HOME="\${SJOURNAL_HOME:-$data_home}"
exec "$libdir/sjournal" "\$@"
EOF
fi
chmod 0755 "$wrapper"

sed "s|__EXEC__|$wrapper|" "$here/$APP_ID.desktop" > "$desktop_file"
chmod 0644 "$desktop_file"

# --- seed config (per-user install only, and only if none yet) --------
if [ "$mode" = "user" ] && [ ! -e "$data_home/config.yaml" ]; then
	if [ -z "$notes_dir" ]; then
		notes_dir="$HOME/Documents/sJournal"
		command -v xdg-user-dir >/dev/null 2>&1 && notes_dir="$(xdg-user-dir DOCUMENTS)/sJournal"
	fi
	mkdir -p "$notes_dir"
	seed_config "$notes_dir" > "$data_home/config.yaml"
	echo ">> journal files will be stored in $notes_dir"
fi

# --- refresh desktop caches ------------------------------------------
command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database "$appsdir" 2>/dev/null || true
command -v gtk-update-icon-cache   >/dev/null 2>&1 && gtk-update-icon-cache -qtf "$icondir" 2>/dev/null || true

if [ "$mode" = "system" ]; then
	data_note="per user: \$HOME/.local/share/sjournal (notes default to the user's Documents/sJournal)"
else
	data_note="$data_home"
fi
cat <<EOF

sJournal installed.
  launcher : $desktop_file
  binary   : $libdir/sjournal  (run "sjournal" from a terminal too)
  data     : ${data_note}

Note: on GNOME the system-tray icon (Settings > "minimize to tray") needs the
"AppIndicator and KStatusNotifierItem Support" shell extension. Without it the
window just closes normally. KDE, XFCE, Cinnamon and MATE show it out of the box.
EOF
