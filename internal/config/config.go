// Package config handles on-disk configuration and the well-known paths
// the CLI and the daemon both use to find each other.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// AppName is used for directory names and the socket/pid file prefixes.
const AppName = "adzan"

// ErrNotConfigured is returned by Load when no config file exists yet.
var ErrNotConfigured = errors.New("adzan is not configured yet")

// ModeCity and ModeCoords are the two ways a user can specify their location.
const (
	ModeCity   = "city"
	ModeCoords = "coords"
)

// Location is a resolved, validated place. Latitude/Longitude are always
// populated (even in city mode) because the daemon schedules from coordinates.
type Location struct {
	Mode      string  `json:"mode"`
	City      string  `json:"city,omitempty"`
	Country   string  `json:"country,omitempty"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timezone  string  `json:"timezone,omitempty"`
	Label     string  `json:"label"`
}

// Config is the full user configuration.
//
// SoundPath is a single default for now. When per-reciter selection lands,
// this becomes a "selected + library" pair without changing anything else.
type Config struct {
	Version   int      `json:"version"`
	Location  Location `json:"location"`
	Method    int      `json:"method"`
	SoundPath string   `json:"sound_path"`
	Muted     bool     `json:"muted"`

	// FajrSoundPath, if set, plays instead of SoundPath for Fajr - its adhan
	// traditionally has different wording ("as-salatu khayrun min an-nawm").
	FajrSoundPath string `json:"fajr_sound_path,omitempty"`

	// Tune nudges individual prayers by whole minutes, so the computed times
	// can be lined up exactly with whatever the local mosque announces.
	// Keys are Fajr, Sunrise, Dhuhr, Asr, Maghrib, Isha.
	Tune map[string]int `json:"tune,omitempty"`
}

// Dir returns the configuration directory, honouring XDG_CONFIG_HOME.
func Dir() (string, error) {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, AppName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", AppName), nil
}

// Path is the location of config.json.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// SoundsDir is where adhan audio lives.
func SoundsDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sounds"), nil
}

// SchedulePath is the cached prayer-time calendar.
func SchedulePath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "schedule.json"), nil
}

// PIDPath is where the daemon records its process id.
func PIDPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, AppName+".pid"), nil
}

// LogPath is where a detached daemon sends its output.
func LogPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, AppName+".log"), nil
}

// EnsureDirs creates the config tree if it does not exist.
func EnsureDirs() error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	sounds, err := SoundsDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(sounds, 0o755)
}

// Load reads config.json. It returns ErrNotConfigured if the file is absent,
// which callers use to trigger the first-run wizard.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotConfigured
	}
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("config file at %s is corrupt: %w", path, err)
	}
	return &c, nil
}

// Save writes config.json atomically so a crash mid-write cannot corrupt it.
func (c *Config) Save() error {
	if err := EnsureDirs(); err != nil {
		return err
	}
	path, err := Path()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
