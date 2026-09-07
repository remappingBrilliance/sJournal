package main

import (
	"fmt"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"

	"sjournal/internal/config"
	"sjournal/internal/crypto"
	"sjournal/internal/store"
	"sjournal/internal/ui"
)

func main() {
	execDir, err := executableDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "sjournal: could not determine executable location:", err)
		os.Exit(1)
	}

	cfg, err := config.Load(execDir)
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
			fmt.Fprintln(os.Stderr, "sjournal: ", err)
			os.Exit(1)
		}
		lock = l
	}
	releaseLock := func() {
		if lock != nil {
			_ = lock.Release()
		}
	}

	fyneApp := fyneapp.New()
	win := fyneApp.NewWindow("sJournal")
	win.Resize(fyne.NewSize(760, 600))

	codec := crypto.NewCodec()

	win.Show()

	ui.RunStartup(win, cfg.NotesFolder, codec, func(password string) {
		sess := &ui.Session{
			Config:   cfg,
			NotesDir: cfg.NotesFolder,
			Codec:    codec,
			Password: password,
		}
		app := ui.BuildApp(fyneApp, win, sess)
		app.SetOnQuit(releaseLock)
	}, func() {
		releaseLock()
		fyneApp.Quit()
	})

	fyneApp.Run()
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
