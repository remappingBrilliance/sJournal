# sJournal

A secure, portable journal with the simplicity of Notepad. Single user, one
machine at a time. Go rewrite of an old .NET journal app, running on Linux,
Windows and macOS.

- **Journal / Notes / Todo** tabs, each with an **Enter Mode** (type, press
  Enter, the line is encrypted and appended — then hidden) and an **Edit
  Mode** (full decrypted contents, editable, with a Save button).
- **Encryption:** every line of every file is an independent AES-256-GCM
  block with its own salt and nonce, keyed by Argon2id from your password.
  Copy a single `.md` file anywhere and it still opens with the password.
- **No installation required** — one binary, creates its own data folder
  next to itself. Optional installers add a Start Menu / app-menu entry.
- **No password recovery.** A forgotten password means those entries are
  permanently unreadable. That is the security guarantee.

`spec.md` is the authoritative description of intended behaviour.

## Build

```sh
./build.sh          # -> ./sjournal
```

Use `build.sh`, not `go build` directly: it works around a missing
`libXxf86vm.so` dev symlink, and on a Wayland session it compiles Fyne's
native Wayland backend (`-tags wayland`) so the window gets a proper taskbar
icon. Set `SJOURNAL_X11=1` to force the portable X11/XWayland build.

If sJournal is already installed for your user (see below), `build.sh` also
drops the fresh binary over `~/.local/lib/sjournal/sjournal` (atomic rename,
so it's safe while the app is running — restart to pick it up). Set
`SJOURNAL_NO_INSTALL=1` to skip that.

Requirements: Go 1.26+, a C toolchain, and the usual Fyne build deps
(OpenGL/X11 or Wayland + `libxkbcommon` headers). `go test ./...` runs the
crypto and storage tests.

Windows binaries are cross-compiled from Linux — see
[`packaging/`](packaging/).

## Run

```sh
./sjournal
```

On first run it creates `config.yaml` and `notes/` next to the binary. It
prompts for a password; if note files already exist it checks the password
against the most recently modified one (Continue / Quit / Re-enter on a
mismatch). An empty `notes/` accepts any password as the new session
password.

Data location, in order of precedence:

1. `$SJOURNAL_HOME` if set (the installers use this)
2. the directory containing the executable

`config.yaml` always lives at that location — never inside `notes/` — so
settings travel with the binary even if you point `notes_folder` elsewhere.

## Install (optional)

Adds a desktop entry, an icon, and (Linux/Wayland) working reminder
notifications. Full detail in [`packaging/README.md`](packaging/README.md).

```sh
./packaging/install-linux.sh                # per-user, no root
./packaging/install-linux.sh --uninstall
```

```sh
./packaging/build-windows.sh                # -> packaging/dist/windows/
# then on Windows:  powershell -ExecutionPolicy Bypass -File .\install.ps1
```

## Configuration — `config.yaml`

| Key | Type | Meaning |
| --- | ---- | ------- |
| `remind_me.enabled` | bool | master switch for Journal reminders |
| `remind_me.start_time` / `end_time` | `"HH:MM"` | daily window reminders fire within (local time) |
| `remind_me.interval_minutes` | int | minutes between reminders inside the window |
| `timestamp_default` | bool | default state of the Journal tab's "timestamp each entry" checkbox |
| `notes_folder` | path | where the note files live (default: `<data dir>/notes`) — **restart to apply** |
| `single_instance` | bool | enforce the PID-checked `notes/.lock` (default: on) — **restart to apply** |
| `require_password_for_edit` | bool | re-prompt for the password when switching into Edit Mode |
| `minimize_to_tray` | bool | the window's close button hides to the system tray instead of quitting (needs a tray; on GNOME that means the AppIndicator extension) |

Everything is editable from **File → Settings…**. Only `notes_folder` and
`single_instance` need a restart; the Settings screen says so when you change
one. The reminder section has a **Test alert now** button.

When a reminder fires it chimes, sends a desktop notification, and opens a
small **journal-entry window of its own** — separate from the main window, so
it never pulls you out of the tab or Edit Mode session you're in. Add the
entry, or dismiss the window, and your work is untouched.

## Keyboard

| Context | Key | Action |
| ------- | --- | ------ |
| Enter Mode, reminder entry window | **Enter** | submit / add the entry |
| Enter Mode, reminder entry window | **Shift+Enter** | newline within the entry |
| Todo Edit Mode, start of an item | **Tab** / **Shift+Tab** | indent / outdent (nest) the item |
| Todo Edit Mode, in an item | **Enter** | new item below at the same indent |

## File format

Each note file is UTF-8 text, **one encrypted block per line**:

```
line = base64( salt[16] || nonce[12] || AES-256-GCM(ciphertext + tag) )
key  = Argon2id(password, salt, time=3, memory=64 MiB, parallelism=4)
```

Blocks are self-contained and independently decryptable. A file may contain
blocks under several passwords (e.g. after a password change); Edit Mode
shows undecryptable blocks greyed out, with a right-click "unlock with a
different password" option.

**Naming a Todo or Notes file:** the files keep their `TODO###.md` /
`NOTES###.md` names on disk, but if a file's first line (once decrypted) is a
Markdown H1 — `# Weekly plan` — the dropdown shows that title instead of the
filename. In the Todo tab, type `# Weekly plan` as an entry to lay down that
heading (it's stored as-is, not as a checkbox item); in Notes it's just the
first line you write.

**Nested Todo items:** checklists nest as standard Markdown — four spaces of
indent per level:

```
- [ ] Main
    - [ ] Sub item
        - [ ] Sub-sub item
```

In Edit Mode, Tab / Shift+Tab at the start of an item nests / un-nests it,
and Enter opens a new item below at the same level. (Only the items nest —
ticking a parent doesn't tick its children.)

## Layout

```
main.go              wiring / startup
internal/crypto/     block format, Argon2id key cache
internal/store/      notes-folder layout, file I/O, single-instance lock
internal/ui/         Fyne app: tabs, Edit/Enter modes, reminders, tray, notifications, settings
internal/ui/assets/  embedded icon
packaging/           installers, Windows cross-compile, icon tooling
```

See [`.CONTEXT.md`](.CONTEXT.md) for the hot paths and the pile of
platform-integration gotchas.

## License

MIT — see [LICENSE](LICENSE).
