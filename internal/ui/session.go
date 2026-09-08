package ui

import (
	"sjournal/internal/config"
	"sjournal/internal/crypto"
)

// Session holds the state shared across the whole running app once a
// password has been accepted at startup.
type Session struct {
	Config   *config.Config
	NotesDir string
	Codec    *crypto.Codec
	Password string

	// TrayEnabled is true when a usable system tray was registered at
	// startup (see ui.SystemTray). MinimizeToTray is only honored when this
	// is true.
	TrayEnabled bool
}
