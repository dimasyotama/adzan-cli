package config

import (
	"errors"
	"testing"
)

func TestLoadUnconfigured(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := Load(); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("expected ErrNotConfigured, got %v", err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	want := &Config{
		Version: 1,
		Method:  20,
		Location: Location{
			Mode: ModeCity, City: "Purwakarta", Country: "Indonesia",
			Latitude: -6.5569, Longitude: 107.4431,
			Timezone: "Asia/Jakarta", Label: "Purwakarta, Indonesia",
		},
		SoundPath: "/tmp/adhan.mp3",
	}
	if err := want.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Location.Label != want.Location.Label || got.Method != want.Method {
		t.Errorf("round trip mismatch: %+v", got)
	}
	if got.Location.Latitude != want.Location.Latitude {
		t.Errorf("latitude lost precision: %v", got.Location.Latitude)
	}
}
