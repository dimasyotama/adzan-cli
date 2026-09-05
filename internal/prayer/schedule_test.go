package prayer

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testSchedule(t *testing.T) *Schedule {
	t.Helper()
	return &Schedule{
		Latitude:  -6.5569,
		Longitude: 107.4431,
		Method:    20,
		Timezone:  "Asia/Jakarta",
		Days: map[string]map[string]string{
			"2026-09-02": {"Fajr": "04:34", "Sunrise": "05:47", "Dhuhr": "11:52", "Asr": "15:11", "Maghrib": "17:52", "Isha": "19:01"},
			"2026-09-03": {"Fajr": "04:34", "Sunrise": "05:47", "Dhuhr": "11:52", "Asr": "15:11", "Maghrib": "17:52", "Isha": "19:02"},
		},
	}
}

func jakarta(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Skip("tzdata for Asia/Jakarta not available")
	}
	return loc
}

func TestCleanTime(t *testing.T) {
	cases := map[string]string{
		"04:34 (WIB)":  "04:34",
		"17:52":        "17:52",
		" 19:01 (+07)": "19:01",
	}
	for in, want := range cases {
		if got := CleanTime(in); got != want {
			t.Errorf("CleanTime(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNextWithinDay(t *testing.T) {
	s := testSchedule(t)
	loc := jakarta(t)

	now := time.Date(2026, 9, 2, 12, 30, 0, 0, loc) // after Dhuhr, before Asr
	ev, err := s.Next(now)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if ev.Name != "Asr" {
		t.Errorf("next prayer = %q, want Asr", ev.Name)
	}
	if ev.At.Format("15:04") != "15:11" {
		t.Errorf("next time = %s, want 15:11", ev.At.Format("15:04"))
	}
}

// After the last prayer of the day, Next must roll over into tomorrow rather
// than reporting that nothing is scheduled.
func TestNextRollsOverMidnight(t *testing.T) {
	s := testSchedule(t)
	loc := jakarta(t)

	now := time.Date(2026, 9, 2, 23, 30, 0, 0, loc)
	ev, err := s.Next(now)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if ev.Name != "Fajr" {
		t.Errorf("next prayer = %q, want Fajr", ev.Name)
	}
	if got := ev.At.Format("2006-01-02 15:04"); got != "2026-09-03 04:34" {
		t.Errorf("next time = %s, want 2026-09-03 04:34", got)
	}
}

// Sunrise is displayed but must never be announced as a prayer.
func TestNextSkipsSunrise(t *testing.T) {
	s := testSchedule(t)
	loc := jakarta(t)

	now := time.Date(2026, 9, 2, 5, 0, 0, 0, loc) // between Fajr and Sunrise
	ev, err := s.Next(now)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if ev.Name == "Sunrise" {
		t.Fatal("Sunrise must not be returned as an announced prayer")
	}
	if ev.Name != "Dhuhr" {
		t.Errorf("next prayer = %q, want Dhuhr", ev.Name)
	}
}

func TestMatchesDetectsMovedLocation(t *testing.T) {
	s := testSchedule(t)
	if !s.Matches(-6.5569, 107.4431, 20, nil) {
		t.Error("Matches should accept the schedule's own coordinates")
	}
	if s.Matches(-6.2, 106.8, 20, nil) {
		t.Error("Matches should reject a different city")
	}
	if s.Matches(-6.5569, 107.4431, 3, nil) {
		t.Error("Matches should reject a different calculation method")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schedule.json")

	s := testSchedule(t)
	if err := s.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("schedule file missing: %v", err)
	}

	got := LoadSchedule(path)
	if got.Timezone != s.Timezone || len(got.Days) != len(s.Days) {
		t.Errorf("round trip lost data: %+v", got)
	}
	if got.Days["2026-09-02"]["Maghrib"] != "17:52" {
		t.Error("timings did not survive the round trip")
	}
}

// A missing or corrupt cache must degrade to empty, never panic.
func TestLoadScheduleHandlesMissingAndCorrupt(t *testing.T) {
	dir := t.TempDir()

	if s := LoadSchedule(filepath.Join(dir, "nope.json")); s == nil || len(s.Days) != 0 {
		t.Error("missing file should yield an empty schedule")
	}

	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if s := LoadSchedule(bad); s == nil || len(s.Days) != 0 {
		t.Error("corrupt file should yield an empty schedule")
	}
}

func TestPruneDropsOldDays(t *testing.T) {
	s := testSchedule(t)
	s.Days["2026-06-01"] = map[string]string{"Fajr": "04:00"}
	s.prune(time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC))
	if _, ok := s.Days["2026-06-01"]; ok {
		t.Error("prune should have removed the stale day")
	}
	if _, ok := s.Days["2026-09-02"]; !ok {
		t.Error("prune must keep current days")
	}
}

func TestValidateCoords(t *testing.T) {
	if err := ValidateCoords(-6.5569, 107.4431); err != nil {
		t.Errorf("valid coordinates rejected: %v", err)
	}
	if err := ValidateCoords(91, 0); err == nil {
		t.Error("latitude 91 should be rejected")
	}
	if err := ValidateCoords(0, -181); err == nil {
		t.Error("longitude -181 should be rejected")
	}
}
