// Package config loads and saves sJournal's config.yaml, which lives next
// to the executable and holds every user-facing setting.
package config

import (
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

const FileName = "config.yaml"

type RemindMe struct {
	StartTime string `yaml:"start_time"` // "HH:MM"
	EndTime   string `yaml:"end_time"`   // "HH:MM"
	Interval  int    `yaml:"interval_minutes"`
	Enabled   bool   `yaml:"enabled"`
}

type Config struct {
	RemindMe         RemindMe `yaml:"remind_me"`
	TimestampDefault bool     `yaml:"timestamp_default"`
	NotesFolder      string   `yaml:"notes_folder"`
	SingleInstance   bool     `yaml:"single_instance"`
	// RequirePasswordForEdit, when true, prompts for the session password
	// again each time the user switches from Enter Mode into Edit Mode.
	RequirePasswordForEdit bool `yaml:"require_password_for_edit"`

	// mu guards RemindMe, which is read by the reminder ticker's background
	// goroutine and written from the Settings screen on the UI goroutine.
	mu sync.RWMutex `yaml:"-"`
	// path this config was loaded from/will be saved to; not persisted.
	path string `yaml:"-"`
}

// GetRemindMe returns a copy of the current reminder settings. Safe to call
// from any goroutine.
func (c *Config) GetRemindMe() RemindMe {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.RemindMe
}

// SetRemindMe updates the reminder settings. Safe to call from any
// goroutine.
func (c *Config) SetRemindMe(rm RemindMe) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.RemindMe = rm
}

func defaults(execDir string) *Config {
	return &Config{
		RemindMe: RemindMe{
			StartTime: "09:00",
			EndTime:   "17:00",
			Interval:  60,
			Enabled:   false,
		},
		TimestampDefault:       false,
		NotesFolder:            filepath.Join(execDir, "notes"),
		SingleInstance:         true,
		RequirePasswordForEdit: false,
		path:                   filepath.Join(execDir, FileName),
	}
}

// Load reads config.yaml from execDir, creating it with defaults if it
// doesn't exist yet.
func Load(execDir string) (*Config, error) {
	cfg := defaults(execDir)

	data, err := os.ReadFile(cfg.path)
	if os.IsNotExist(err) {
		if err := cfg.Save(); err != nil {
			return nil, err
		}
		return cfg, nil
	} else if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	cfg.path = filepath.Join(execDir, FileName)
	return cfg, nil
}

func (c *Config) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, data, 0o600)
}
