package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"

	"sjournal/internal/config"
	"sjournal/internal/crypto"
	"sjournal/internal/store"
	"sjournal/internal/ui"
	"sjournal/internal/ui/assets"
)

func main() {
	home, err := resolveHome()
	if err != nil {
		fmt.Fprintln(os.Stderr, "sjournal: could not determine data location:", err)
		os.Exit(1)
	}

	cfg, err := config.Load(home)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sjournal: could not load config.yaml:", err)
		os.Exit(1)
	}

	if err := store.EnsureNotesDir(cfg.NotesFolder); err != nil {
		fmt.Fprintln(os.Stderr, "sjournal: could not create notes folder:", err)
		os.Exit(1)
	}

	var lock *store.Lock
	if cfg.SingleInstance {
		l, err := store.AcquireLock(cfg.NotesFolder)
		if err != nil {
			var locked *store.ErrLocked
			if errors.As(err, &locked) {
				// Another instance owns the folder — ask it to surface its
				// window (it may be hidden in the tray) rather than just
				// failing.
				store.NudgeInstance(locked.PID)
			}
			fmt.Fprintln(os.Stderr, "sjournal:", err)
			os.Exit(1)
		}
		lock = l
	}
	releaseLock := func() {
		if lock != nil {
			_ = lock.Release()
		}
	}

	fyneApp := fyneapp.NewWithID(assets.AppID)
	fyneApp.SetIcon(assets.AppIcon)
	win := fyneApp.NewWindow("sJournal")
	win.SetIcon(assets.AppIcon)
	win.Resize(fyne.NewSize(760, 600))

	// Must happen before fyneApp.Run(): the desktop driver only starts the
	// tray subsystem once, at the top of its event loop.
	trayEnabled := ui.SystemTray(fyneApp, win, releaseLock)

	codec := crypto.NewCodec()

	win.Show()

	ui.RunStartup(win, cfg.NotesFolder, codec, func(password string) {
		sess := &ui.Session{
			Config:      cfg,
			NotesDir:    cfg.NotesFolder,
			Codec:       codec,
			Password:    password,
			TrayEnabled: trayEnabled,
		}
		app := ui.BuildApp(fyneApp, win, sess)
		app.SetOnQuit(releaseLock)
	}, func() {
		releaseLock()
		fyneApp.Quit()
	})

	fyneApp.Run()
}

// resolveHome returns the directory that holds config.yaml and (by default)
// the notes/ folder. It is the executable's own directory — keeping the
// "portable folder" model from the spec — unless SJOURNAL_HOME overrides it,
// which the installers use to point an installed binary at a data folder in
// the user's profile.
func resolveHome() (string, error) {
	if h := strings.TrimSpace(os.Getenv("SJOURNAL_HOME")); h != "" {
		if err := os.MkdirAll(h, 0o700); err != nil {
			return "", err
		}
		return h, nil
	}
	return executableDir()
}

func executableDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		resolved = exe
	}
	return filepath.Dir(resolved), nil
}
