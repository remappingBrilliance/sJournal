# Packaging & install

sJournal still runs as a single portable binary with no installation — just
drop `sjournal` (or `sjournal.exe`) in a folder and run it; `config.yaml` and
`notes/` are created next to it. These scripts are for when you'd rather have
it behave like an installed app: a Start Menu / applications-menu entry, a
proper icon, and (on Linux/Wayland) reminder notifications that actually show
up.

## Linux (GNOME / KDE / XFCE / …)

```sh
./packaging/install-linux.sh                 # per-user, no root
sudo ./packaging/install-linux.sh --system   # system-wide (/usr/local)
./packaging/install-linux.sh --notes-dir ~/Journal
./packaging/install-linux.sh --uninstall
```

What it does:

| file | purpose |
|------|---------|
| `~/.local/lib/sjournal/sjournal` | the binary |
| `~/.local/bin/sjournal` | launcher shell shim (sets `SJOURNAL_HOME`) |
| `~/.local/share/applications/io.lottkowitz.sjournal.desktop` | menu entry |
| `~/.local/share/icons/hicolor/*/apps/io.lottkowitz.sjournal.*` | icon |
| `~/.local/share/sjournal/config.yaml` | settings |
| `~/Documents/sJournal/` | journal files (override with `--notes-dir`) |

Journal data lives in `SJOURNAL_HOME` (`~/.local/share/sjournal` by default),
never inside the install tree, so re-running the installer to upgrade never
touches your notes or settings.

### Taskbar / dash-to-panel icon

The window only carries an icon on Wayland if Fyne is built with `-tags
wayland` (native Wayland sets `xdg_toplevel` `app_id` =
`io.lottkowitz.sjournal`, which GNOME matches to the installed `.desktop`).
The default Fyne build runs under XWayland with no usable `WM_CLASS` and
gets a generic icon. `build.sh` adds the tag automatically on a Wayland
session; `install-linux.sh` inherits that. Force the portable X11 build
with `SJOURNAL_X11=1 ./packaging/install-linux.sh`.

### Why this fixes Wayland notifications

The `.desktop` file's basename matches the app's Fyne ID
(`io.lottkowitz.sjournal`). GNOME/KDE use that to resolve a notification back
to an application; without an installed desktop entry the reminder either
appears with no icon or is dropped by the compositor's notification filter.
The app also now shells out to `notify-send`/`gdbus` first (reliable on every
compositor) and only falls back to Fyne's built-in D-Bus path.

### System tray on GNOME

"Minimize to tray" (Settings) needs a StatusNotifierItem host. GNOME doesn't
ship one — install the **AppIndicator and KStatusNotifierItem Support** shell
extension. KDE, XFCE, Cinnamon and MATE work out of the box. With no tray the
close button just quits normally.

## Windows

Build the distributable folder on Linux (needs `mingw-w64` gcc + `rsvg-convert`,
both already on this dev box):

```sh
./packaging/build-windows.sh
# -> packaging/dist/windows/{sjournal.exe, sjournal.ico, install.ps1, uninstall.ps1}
```

Copy `packaging/dist/windows/` to the Windows machine, then in PowerShell:

```powershell
powershell -ExecutionPolicy Bypass -File .\install.ps1            # Start Menu entry
powershell -ExecutionPolicy Bypass -File .\install.ps1 -Desktop   # + desktop shortcut
powershell -ExecutionPolicy Bypass -File .\install.ps1 -NotesDir 'D:\Journal'
powershell -ExecutionPolicy Bypass -File .\uninstall.ps1
```

Installs to `%LOCALAPPDATA%\Programs\sJournal\`, journal files default to
`Documents\sJournal`. No admin rights required. Windows has a real system tray,
so "minimize to tray" works with no extra setup.

## Icon

Single source: [`../internal/ui/assets/icon.svg`](../internal/ui/assets/icon.svg),
embedded in the binary and rasterized by the install scripts. `mkicon/` is a
tiny helper that packs PNGs into a `.ico`.
