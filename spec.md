# Program Overview

A secure journal that is portable across operating systems, requires no installation, and has the simplicity of Windows Notepad. Single-user, single-machine-at-a-time use.

# Technology

- **Language:** Go
- **GUI toolkit:** [Fyne](https://fyne.io/) — pure Go, compiles to a single static cross-platform binary, no bundled runtime, good fit for "no installation." On Linux, `build.sh` compiles the native Wayland backend (`-tags wayland`) on a Wayland session so the window carries an `app_id` GNOME can match to the installed `.desktop` (otherwise it runs under XWayland with a generic taskbar icon); `SJOURNAL_X11=1` forces the portable X11/XWayland build.
- **Encryption:** AES-256-GCM (authenticated encryption)
- **Key derivation:** Argon2id
- **Distribution:** single binary; on first run it creates its own data folder next to itself. No installer, no system-wide state.
    - Optional installers (`packaging/`) add a Start Menu / applications-menu entry, an icon, and a `.desktop` file whose basename matches the Fyne app ID (`io.lottkowitz.sjournal`) so Wayland compositors can resolve the app's notifications. When installed, `SJOURNAL_HOME` (env var) points the binary at a data folder in the user's profile instead of the install tree. The portable single-binary mode is unchanged and remains the default.

# Data & File Layout

All paths are relative to the executable's directory by default.

- `notes/` — the data folder (path overridable via `config.yaml`: `notes_folder`)
    - `YYYY-MM-DD.md` — one Journal file per day
    - `TODO001.md`, `TODO002.md`, ... — Todo files, 3-digit zero-padded, starting at `001`
    - `NOTES001.md`, `NOTES002.md`, ... — Notes files, 3-digit zero-padded, starting at `001`
    - `.lock` — single-instance lock file (see [Single Instance](#single-instance))
- `config.yaml` — next to the executable (not inside `notes/`), so the settings travel with the binary even if the notes folder is relocated.

# File Format (on-disk block format)

Every note file is plain UTF-8 text. **Every line is exactly one encrypted block** — the file on disk is never anything other than a sequence of encrypted block lines; plaintext is never persisted.

Each line is a single base64 string, decoding to:

```
salt (16 bytes) || nonce (12 bytes) || ciphertext+tag (AES-256-GCM)
```

The plaintext inside a block is the literal entry text as typed — including the `[HH:mm] ` prefix if timestamping is enabled, and any embedded newlines from Shift+Enter within that entry.

Every block is fully self-contained: it carries its own salt and nonce, so any single line can be decrypted (given the right password) without needing any other line, file, or metadata store. This means copying just the `.md` file to another machine is sufficient to read it later.

**Salt reuse within a run:** the first time a password is used to encrypt a *new* block during the current run, a fresh random salt is generated for that (password, run) pair and cached in memory along with the derived key. Every subsequent new block written with that same password during the same run reuses that cached salt+key — only the nonce is freshly randomized per block (required for GCM safety, and free to do). This means Argon2id only runs once per password per run, not once per keystroke. Blocks from earlier runs, or written with a different password, carry their own (different) salt and are decrypted lazily by deriving and caching a key for that specific (password, salt) pair the first time it's needed.

# Encryption

- **Cipher:** AES-256-GCM
- **KDF:** Argon2id, with fixed sane parameters baked into the app (not user-configurable) — e.g. 64 MB memory, 3 iterations, parallelism matched to the KDF library defaults. Tuned so a single derivation is imperceptible in the UI (well under a second) without being cryptographically weak for this threat model (local, single-user file encryption).
- **No password recovery.** Forgetting the password used for a block means that block is permanently unreadable. There is no recovery key, no backdoor, no stored hint. This is treated as the actual security guarantee of the app, not a gap to fill later.
- **Multiple passwords per file are supported.** Different blocks in the same file may be encrypted with different passwords (see [Encryption & Edit vs Enter Mode](#encryption--edit-vs-enter-mode)) — this happens naturally over time as the user's password changes across runs, or via one-off decryption of old blocks.

## Startup password check

- On startup, prompt for a password.
- Determine "the most recent file": scan `Journal` + `Todo` + `Notes` files together (by filesystem mtime) and pick whichever was modified most recently, regardless of type. (Not necessarily today's journal — if the user was last working in a Todo file, that's what gets checked against.)
- If no note files exist at all (fresh install / empty `notes/` folder): accept the entered password as correct with no check, and use it as the initial session password.
- Otherwise, take the last line (block) of that most-recent file, derive a key from the entered password + that block's stored salt, and attempt to decrypt it.
    - **If correct:** proceed to the default screen (Journal tab, Enter Mode) using this as the session password.
    - **If incorrect:** prompt with **Continue**, **Quit**, or **Re-enter**.
        - **Continue:** proceed into the app anyway, using the entered password as the session password for *new* blocks going forward. Existing blocks encrypted with a different (the actual) password will not automatically decrypt — they show as encrypted (see [Todo Tab](#todo-tab) / [Edit Mode](#encryption--edit-vs-enter-mode)) until unlocked individually via the one-off right-click decrypt.
        - **Quit:** exit the app.
        - **Re-enter:** return to the password prompt.

# Single Instance

- On startup, check `notes/.lock`.
    - The lock file stores the PID of the process holding it.
    - If a lock file exists, check whether that PID is currently a running process on the system.
        - If it's still running: refuse to start, inform the user another instance is already open on this notes folder.
        - If it's not running (stale lock from a crash): remove the stale lock and proceed normally.
    - On clean startup, write the current process's PID into `notes/.lock`; remove it on clean shutdown.
- This behavior can be disabled entirely via a `single_instance` setting in `config.yaml` (see [Settings](#settings)).

# Main Window

- Three tabs: **Journal**, **Notes**, **Todo List**.
- Each tab has two modes:
    - **Edit Mode** — full file contents visible (decrypted where possible); has a **[Save]** button, and a button to switch to Enter Mode (which saves first, then switches).
    - **Enter Mode** — contents hidden; typed text is encrypted, base64-encoded, and appended as a new block on **Enter**. Removed from view immediately after submission.
        - **Shift+Enter** inserts a newline within the current entry without submitting/encrypting it.
    - Switching modes always saves the current mode's state first, then loads the new mode fresh from disk.
- If the user attempts to quit, switch files, or switch tabs while Edit Mode has unsaved changes: prompt to **Save**, **Discard**, or **Cancel**.
- All file-selection dropdowns (Journal dates, Todo###, Notes###) list entries **newest first**.

## Journal Tab

- Opens in Enter Mode by default.
- Dropdown lists all `YYYY-MM-DD.md` files present in `notes/`, newest date first.
- On load, selects/opens today's file. If today's file doesn't exist yet, it is not created until the first entry is submitted; it still appears (pending) at the top of the dropdown.
- **Midnight roll-over:** the app does not assume it is restarted each day. When an entry is submitted (via Enter Mode or a reminder prompt) and the tab is still pointed at an earlier date because the app was left running past midnight, it automatically switches to today's file (creating it on that first entry) and the entry lands there. This is suppressed while the user has deliberately selected an older date from the dropdown — entries then keep going to that pinned date until they select today (or a new day's file) again.
- **Timestamp checkbox:** when checked, each new entry is prefixed with `[HH:mm] ` (the time at which the user typed the first character of that entry), producing plaintext like `[HH:mm] <user text>` before encryption. The checkbox's on/off state persists across restarts as a default in `config.yaml`, but can be toggled per-session.
- **Remind-me feature** (configured via Settings):
    - Set a start time and end time of day, and an interval.
    - While within that window, at each interval boundary the app chimes (a short best-effort notification sound — `canberra`/`paplay` on Linux, `[console]::beep` on Windows, `afplay` on macOS; silently skipped if none is available), sends a desktop notification, and opens the reminder prompt. On Linux the notification is sent via `notify-send`/`gdbus` (reliable under Wayland) with a fallback to Fyne's built-in D-Bus path.
        - The prompt is its **own top-level window** — a journal-entry field with **[Add]** and **[Dismiss]** — deliberately separate from the main window. The reminder never raises, focuses, or un-stashes the main window, so it can't pull the user out of whatever tab or Edit Mode session they're in; they deal with the reminder window or dismiss it and their work is untouched.
        - Enter in the field adds the entry (Shift+Enter for a newline); **[Dismiss]**, the window's close button, or an empty **[Add]** all close it without writing anything to the file.
        - If a reminder is still on screen when the next interval boundary arrives, that interval is skipped rather than stacking a second window.
        - An entry added while the Journal tab happens to be in **Edit Mode** on today's file is folded into the open editor as well as appended to disk, so saving Edit Mode (a full-file rewrite) doesn't drop it.
    - The schedule (start/end/interval) is persisted in `config.yaml`, so restarting the app mid-window resumes chiming on the same schedule rather than needing to be re-armed.

## Todo Tab

- Items are stored as markdown checkbox lines once decrypted: `- [ ] Item to be done` / `- [x] Done item`. One item = one block.
- **Nesting:** items can be nested (checklists only — no chaining of the checkboxes themselves; checking a parent does nothing to its children). Depth is carried on disk as leading indentation, four spaces per level:
    ```
    - [ ] Main
        - [ ] Sub item
            - [ ] Sub-sub item
    ```
  On load, indentation is read tolerantly (a tab = one level; other space counts round to the nearest four) and re-normalised to four-space steps on save. Maximum depth 8, and an item can't be indented more than one level past the item above it.
- **Files:** `TODO001.md`, `TODO002.md`, ... . If none exist, the tab behaves like Notes' empty state: the dropdown shows a pending `TODO001.md`, which is only actually written to disk on the first entry.
- Otherwise, the most-recently-modified `TODO###.md` is opened in **Enter Mode** by default. The dropdown is populated with all existing Todo files (newest first) and lets the user switch between them.
- **Dropdown display name:** files keep their `TODO###.md` names on disk, but the dropdown shows a friendlier label when one is available — see [File dropdown display names](#file-dropdown-display-names).
- A **[New File]** button creates the next file in numerical sequence (zero-padded to 3 digits) and switches to it.
- **Enter Mode:** each submitted item (Enter) is encrypted as one new block appended to the current file — i.e. one checkbox item = one block. Shift+Enter is allowed within an item to add a continuation line without submitting; the resulting multi-line block is still a single checkbox item, with the `[ ]`/`[x]` marker understood to apply to the whole block (prefixing its first line). Leading whitespace (spaces, or a Tab) on the typed text nests the new item.
    - **Heading lines:** an entry that is exactly a Markdown H1 — `# Some title` (one `#`, whitespace, text, single line) — is stored verbatim as its own block rather than being wrapped as `- [ ] # Some title`. This is how a Todo list gets a heading (and, when it's the first block, a dropdown display name). Any other text, including `## sub-headings`, becomes a normal checklist item.
- **Edit Mode:**
    - Shows all existing items as checkboxes with their text, indented by depth.
    - A heading block (`# Title`) is shown as a bold, single-line field with no checkbox; editing it down to an empty string removes it. It is written back verbatim on save (never merged into an item).
    - Checking/unchecking a box toggles that item's `[ ]`/`[x]` marker.
    - Clicking an item's text allows inline editing.
    - **Tab** / **Shift+Tab** with the caret at the very start of an item indents / outdents it one level (elsewhere in the line they do nothing).
    - **Enter** at the end of an item opens a new empty item directly below it at the same depth; the trailing "New item…" row still adds a top-level item.
    - Editing an item's text down to an empty string removes that item entirely; an empty item left at save time is dropped.
    - A block that held several checklist lines (a legacy import) is split into one item per line; on the next save each is written as its own block.
    - Any block that fails to decrypt with the current session password (wrong password, unknown password, or corrupted data — the UI does not distinguish the cause) is shown greyed-out as "encrypted." Right-clicking it offers a one-off decrypt with a password entered for that purpose only (see [Encryption & Edit vs Enter Mode](#encryption--edit-vs-enter-mode) for what happens to it on save).
    - Exiting Edit Mode (via Save, or via the mode-switch button) re-encrypts and consolidates the file per the rules below.

## Notes Tab

- Same UI and entry/encryption behavior as Journal, but using `NOTES###.md` files (3-digit zero-padded, starting at `001`) instead of dates.
- If no Notes files exist, the dropdown shows a pending `NOTES001.md`, written to disk on first entry (same empty-state behavior as Todo).
- Selection dropdown and New File button work the same as Todo's, including the [display-name](#file-dropdown-display-names) behavior. No special heading affordance is needed — the user just makes `# Title` the first line of the note (in Enter Mode as the first entry, or at the top of the prose box in Edit Mode).
- No timestamp checkbox, no Remind-me feature — those are Journal-specific.

## File dropdown display names

- Applies to the **Todo** and **Notes** tabs only (Journal always shows its `YYYY-MM-DD.md` date).
- Files are never renamed on disk — this is purely how the dropdown labels them.
- For each file, the app decrypts its **first block** with the current session password and looks at that block's **first line**. If it is a Markdown H1 (`# Title`), the trimmed title text is used as the dropdown label.
- Otherwise — the first block doesn't decrypt (old/unknown password), the file is empty/pending, or the first line isn't an H1 — the label is just the filename.
- If two files would show the same label, the clashing ones are disambiguated by appending `  (FILENAME.md)`.
- Labels refresh whenever the file list does (opening a file, saving Edit Mode, adding an entry); the lookup is memoised per file by modification time.

# Encryption & Edit vs Enter Mode

- In Enter Mode, each Enter encrypts the current entry text (AES-256-GCM, using the cached session key for the current password — deriving and caching it via Argon2id the first time that password is used to write in this run) and appends it as one new base64 block/line to the file.
- Entering **Edit Mode**: every block is decrypted using whichever cached (password, salt) pair matches it, deriving+caching new ones as needed. Blocks that fail to decrypt under every password tried so far are left in their base64 form and shown greyed-out.
- Exiting Edit Mode (Save):
    - If **no** base64 (still-encrypted) blocks remain visible, the entire file's plaintext is consolidated and re-encrypted as a single new block, under the current session password.
    - If some base64 blocks remain (couldn't be decrypted / were never unlocked), the file keeps its original block ordering: each maximal run of consecutive plaintext blocks is merged (joined by newlines, in original order) and re-encrypted as one new block; each remaining base64 block is written back out byte-for-byte unchanged. The result is a file with fewer, larger blocks, some still under old/unknown passwords, interleaved in their original relative order.
    - A block unlocked in this Edit Mode session via the one-off right-click decrypt (below) counts as plaintext for this consolidation — i.e. it gets folded in and re-encrypted under the current session password, effectively migrating it off its old password.
- Right-clicking a base64 block in Edit Mode offers "decrypt with password" — a password entered there is used only to attempt decrypting that one block (deriving a key from it + that block's own stored salt), for viewing purposes; if it succeeds, that block becomes an ordinary decrypted/editable block for the rest of this Edit Mode session (and will be migrated as above if the file is then saved).
- Edit Mode has a **[Save]** button. Both Edit and Enter Mode have a button to switch to the other, which saves the current mode's state first, then loads the new mode.

# Settings

Everything in `config.yaml` is editable from the Settings screen, **except** program metadata (window size/position, last-used tab, etc.), which is persisted but not user-facing.

`config.yaml` contents:

- `remind_me.start_time`, `remind_me.end_time`, `remind_me.interval` — Journal reminder schedule.
- `timestamp_default` (bool) — default state of the Journal tab's "prefix timestamp" checkbox.
- `notes_folder` (path) — override for where the `notes/` data folder lives (default: next to the executable).
- `single_instance` (bool) — whether to enforce the PID-checked lock file on startup (default: on).
- `require_password_for_edit` (bool) — re-prompt for the session password when switching into Edit Mode (default: off).
- `minimize_to_tray` (bool) — make the window's close button hide to the system tray instead of quitting (default: off). Requires a system tray; on GNOME that means the AppIndicator shell extension, otherwise the close button quits as normal. The tray menu has **Show sJournal** and **Quit**.

# Open items / explicitly out of scope

- No password-change UI is specified beyond the natural mechanism above (enter a new password at startup via "Continue," then Save in Edit Mode to migrate blocks onto it). If a dedicated "change password" flow is wanted later, it would just be sugar over that same path.
- No auto-lock / inactivity timeout — the app is unlocked for the whole run once a password is accepted or "Continue" is chosen.
- No window-size/layout constraints specified — a plain resizable window is assumed, consistent with "simplicity of Notepad."
