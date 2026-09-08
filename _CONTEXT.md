# sJournal
dev=Linux/Wayland (also builds Windows via mingw cross-compile)  prod=portable single binary; optional per-user install
stack=Go 1.26 · Fyne v2.8 (GUI, cgo) · AES-256-GCM + Argon2id · yaml.v3 · godbus/dbus v5 (Linux notify/tray)
schema=none (no DB — encrypted flat files, see Schema below)  api=none

Spec is the source of truth: `spec.md` (written via a Q&A design pass). When it's silent, ask — don't guess.

---

## Services

None — desktop app, no server, no network. Single-instance enforced via a PID-checked `notes/.lock` (toggle: `single_instance`).

---

## Key Files

- `main.go` — wiring: resolve data dir (`$SJOURNAL_HOME` or exe dir) → load config → lock → build Fyne app → **register tray before `Run()`** → startup password flow → `ui.BuildApp`
- `internal/crypto/block.go` — the encrypted block format + `KeyCache` (one Argon2id derivation per (password, salt) per process)
- `internal/store/files.go` — notes-folder layout, file-kind regexes, line read/write
- `internal/store/document.go` — `LoadDocument` (decrypt each line against known passwords) + block consolidation for Edit-Mode save
- `internal/store/lock.go` — single-instance lock; `nudge_unix.go` sends SIGUSR1 to an existing instance
- `internal/ui/app.go` — `App`: tabs, menu, quit guard, `surface`/`stashToTray` window-state, package-var hooks (`showMainWindow`, `trayQuit`, `OnNotificationActivated`)
- `internal/ui/tab.go` — shared Journal/Todo/Notes tab (Edit vs Enter mode)
- `internal/ui/entry_widget.go` — `EnterModeEntry`: Enter submits, Shift+Enter newlines (via `desktop.Keyable`, see Gotchas)
- `internal/ui/reminder.go` — 20s ticker, schedule check, chime + `Alert` + prompt; `active` guard against stacked prompts
- `internal/ui/notify*.go` — `Notify`/`Alert`; Linux path is D-Bus direct (action + `ActionInvoked` listener); `notify_other.go` stubs non-Linux
- `internal/ui/tray*.go` — `SystemTray()` (call before `Run()`); `tray_linux.go` `trayHostAvailable()` gates minimize-to-tray
- `internal/ui/startup.go` — password prompt + last-block decrypt check (Continue/Quit/Re-enter)
- `internal/ui/assets/` — embedded `icon.svg` (single source; packaging scripts rasterize it)
- `build.sh` — **use this, not `go build`** (see Gotchas)
- `packaging/` — `install-linux.sh`, `build-windows.sh` (mingw), `install-windows.ps1`, `mkicon/` (PNG→ICO)

---

## Hot Paths

```
startup      → startup.go RunStartup → store.MostRecentFile → crypto.Decrypt(last block) → BuildApp
Enter submit → tab.go handleEnter → crypto.Encrypt (cached salt/key) → store.AppendLine (one block = one line)
Edit save    → tab.go saveEditMode → BlockEditor.Lines → store consolidation → store.WriteLines (full rewrite)
reminder     → reminder.go loop (20s) → check (window/interval/active guard) → fire → Chime + Alert + prompt → done() (dismiss notif, maybe re-stash to tray)
```

---

## Integration Points

| Direction | Protocol | Endpoint / Env var |
| --------- | -------- | ------------------ |
| Outbound  | D-Bus session | `org.freedesktop.Notifications` (Notify + CloseNotification + ActionInvoked signal) |
| Outbound  | D-Bus session | `org.kde.StatusNotifierWatcher` (probe), StatusNotifierItem (tray, via `fyne.io/systray`) |
| Inbound   | signal | `SIGUSR1` → raise/focus window (Unix; sent by a blocked second launch) |
| Env       | — | `SJOURNAL_HOME` (override data dir), `SJOURNAL_X11=1` (force X11/XWayland build) |

---

## Schema

No database. Every note file (`YYYY-MM-DD.md`, `TODO###.md`, `NOTES###.md`) is UTF-8 text, **one encrypted block per line**:

```
line = base64( salt[16] || nonce[12] || AES-256-GCM(ciphertext + tag) )
key  = Argon2id(password, salt, t=3, m=64MiB, p=4) → 32 bytes
```

Each line is self-contained — copy a single `.md` file elsewhere and it still decrypts with the password. Multiple passwords per file are normal (password changes over time). Plaintext includes the `[HH:mm] ` prefix (if timestamping) and any embedded newlines.

---

## Gotchas

- **Build with `./build.sh`, never `go build`.** (a) this dev box lacks a `libXxf86vm.so` dev symlink glfw links even in the Wayland build — the script symlinks the runtime `.so.1`; (b) it adds `-tags wayland` on a Wayland session. Without that tag Fyne runs under XWayland with no usable `WM_CLASS` → GNOME/dash-to-panel can't match the window to `io.lottkowitz.sjournal.desktop` → generic icon. `SJOURNAL_X11=1` forces the portable X11 build. Vet the Wayland path with `go vet -tags wayland ./...`.
- **`win.SetIcon()` does nothing on Wayland.** Taskbar icon = window `app_id` (`io.lottkowitz.sjournal`, set by the `-tags wayland` build) → installed `.desktop` basename → `Icon=`. Needs `install-linux.sh` to have run.
- **`SetSystemTrayMenu` must be called before `fyne.App.Run()`** — the glfw driver starts the tray subsystem once at the top of its loop. Hence `ui.SystemTray()` is in `main()`, not `BuildApp()`.
- **GNOME hides notification banners from the focused app.** Reminders therefore use `Alert()` = critical urgency so the banner shows regardless. `Notify()` (normal) is only for the tray hint.
- **GNOME has no built-in tray.** `trayHostAvailable()` checks `IsStatusNotifierHostRegistered`; if false, `minimize_to_tray` falls back to a plain quit. Needs the AppIndicator / "Tray Icons" shell extension.
- **Fyne never synthesizes a `CustomShortcut` for a bare Shift modifier** (`internal/driver/glfw/window.go` `triggersShortcut`). Shift+Enter is detected by implementing `desktop.Keyable` and tracking Shift up/down manually — see `entry_widget.go`. `canvas.AddShortcut(Shift+…)` compiles but silently never fires.
- **`notes_folder` and `single_instance` take effect only on restart** (read once at startup — `NotesDir` is copied into `Session`, lock is acquired in `main`). Everything else in `config.yaml` is live. The Settings screen only shows the restart warning when one of those two changed.
- **`config.yaml` lives next to the executable** (or in `$SJOURNAL_HOME`), *not* inside `notes/`, so settings survive relocating the notes folder.
- **No password recovery.** A forgotten password = those blocks are permanently unreadable. This is the security guarantee, not a bug.
- **Salt is reused within a run** for new blocks written with the same password (cached in `KeyCache`) — deliberate, so Argon2id runs once per password per process, not per keystroke. Nonce is always fresh.
- **Reminder is a 20s poll**, not a precise timer; `reminder.go` logs its state to stderr (`reminders armed…` / `…still open — skipping`). The `active atomic.Bool` guard stops a slow user from stacking prompts.
- **Tests:** `go test ./...` — `internal/crypto` and `internal/store` are covered; `internal/ui` has none (no headless Fyne). UI changes are compile + run-smoke only; real verification is a human running the binary.
- **Windows build** is cross-compiled from Linux (`packaging/build-windows.sh`, needs `mingw64-gcc` + `rsvg-convert`); ~3.5 min because cgo deps recompile.
