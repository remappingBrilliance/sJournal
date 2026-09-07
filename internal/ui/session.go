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
}
