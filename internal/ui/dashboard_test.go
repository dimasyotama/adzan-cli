package ui

import (
	"testing"
	"time"

	"github.com/dimasyotama/adzan-cli/internal/ipc"
)

// render must never panic, including on the partial states the daemon reports
// before its first successful schedule fetch.
func TestRenderHandlesPartialStatus(t *testing.T) {
	cases := map[string]*ipc.Status{
		"empty":        {},
		"no timetable": {Location: "Purwakarta, Indonesia", NextName: "Isha", NextAt: time.Now().Add(time.Hour)},
		"no next":      {Location: "Purwakarta, Indonesia", Today: map[string]string{"Fajr": "04:34"}},
		"muted":        {Location: "Purwakarta, Indonesia", Muted: true},
		"playing":      {Location: "Purwakarta, Indonesia", Playing: true},
	}
	for name, st := range cases {
		t.Run(name, func(t *testing.T) {
			render(st)     // must not panic
			StatusOnce(st) // must not panic
		})
	}
}

func TestDayLabel(t *testing.T) {
	now := time.Now()
	if got := dayLabel(now.Add(2 * time.Hour).Truncate(time.Minute)); got != "today" && got != "tomorrow" {
		t.Errorf("two hours out should be today or tomorrow, got %q", got)
	}
	if got := dayLabel(time.Date(now.Year(), now.Month(), now.Day(), 5, 0, 0, 0, time.Local)); got != "today" {
		t.Errorf("same calendar day should be today, got %q", got)
	}
	tomorrow := time.Date(now.Year(), now.Month(), now.Day(), 5, 0, 0, 0, time.Local).AddDate(0, 0, 1)
	if got := dayLabel(tomorrow); got != "tomorrow" {
		t.Errorf("next calendar day should be tomorrow, got %q", got)
	}
}

func TestCountdown(t *testing.T) {
	cases := map[time.Duration]string{
		0:                "00:00:00",
		-5 * time.Second: "00:00:00", // never show negatives
		90 * time.Second: "00:01:30",
		2*time.Hour + 14*time.Minute + 7*time.Second: "02:14:07",
		25 * time.Hour: "25:00:00",
	}
	for in, want := range cases {
		if got := Countdown(in); got != want {
			t.Errorf("Countdown(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestSilhouetteIsRectangular(t *testing.T) {
	width := SilhouetteWidth()
	for i, line := range splitLines(Silhouette()) {
		if line == "" {
			continue
		}
		if n := len([]rune(stripANSI(line))); n != width {
			t.Errorf("silhouette line %d is %d wide, want %d", i, n, width)
		}
	}
}
