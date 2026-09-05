package prayer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// Schedule is the locally cached calendar. It is written to disk so a restart
// (or an offline machine) does not need the network.
type Schedule struct {
	Latitude  float64                      `json:"latitude"`
	Longitude float64                      `json:"longitude"`
	Method    int                          `json:"method"`
	Timezone  string                       `json:"timezone"`
	Days      map[string]map[string]string `json:"days"` // YYYY-MM-DD -> prayer -> HH:MM
	FetchedAt time.Time                    `json:"fetched_at"`

	// Tune is the per-prayer minute offset the times were built with. A change
	// here invalidates the cache, same as moving location.
	Tune map[string]int `json:"tune,omitempty"`
}

// Event is a single upcoming prayer.
type Event struct {
	Name string
	At   time.Time
}

// CleanTime strips the timezone suffix Aladhan appends, e.g. "04:34 (WIB)".
func CleanTime(s string) string {
	if i := strings.IndexByte(s, '('); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// Location returns the schedule's time.Location, falling back to local time
// if the stored zone name is not present in the system tzdata.
func (s *Schedule) Location() *time.Location {
	if s.Timezone == "" {
		return time.Local
	}
	loc, err := time.LoadLocation(s.Timezone)
	if err != nil {
		return time.Local
	}
	return loc
}

// Matches reports whether the cache was built for the given place, method and
// tuning. Any difference means every cached day is stale.
func (s *Schedule) Matches(lat, lon float64, method int, tune map[string]int) bool {
	const eps = 0.0001
	if abs(s.Latitude-lat) >= eps || abs(s.Longitude-lon) >= eps || s.Method != method {
		return false
	}
	return sameTune(s.Tune, tune)
}

// sameTune compares two offset maps, treating absent and zero as equal so an
// empty map and a nil map do not needlessly invalidate the cache.
func sameTune(a, b map[string]int) bool {
	for _, name := range append(append([]string{}, Order...), "Sunrise") {
		if a[name] != b[name] {
			return false
		}
	}
	return true
}

// Covers reports whether the cache has an entry for the given day.
func (s *Schedule) Covers(day time.Time) bool {
	_, ok := s.Days[day.Format("2006-01-02")]
	return ok
}

// Timings returns one day's prayer times, or nil if not cached.
func (s *Schedule) Timings(day time.Time) map[string]string {
	return s.Days[day.Format("2006-01-02")]
}

// EventsOn builds the ordered list of the five prayers for a given date,
// resolved to absolute times in the schedule's timezone.
//
// A prayer that lands earlier on the clock than the one before it has crossed
// midnight and belongs to the next day. This is real at high latitudes in
// summer, where Isha can fall after 00:00; without this the daemon would treat
// it as having already passed at the start of the day.
func (s *Schedule) EventsOn(day time.Time) []Event {
	loc := s.Location()
	timings := s.Timings(day)
	if timings == nil {
		return nil
	}
	var out []Event
	var prev time.Time
	for _, name := range Order {
		raw, ok := timings[name]
		if !ok {
			continue
		}
		t, err := parseAt(day, CleanTime(raw), loc)
		if err != nil {
			continue
		}
		if !prev.IsZero() && t.Before(prev) {
			t = t.AddDate(0, 0, 1)
		}
		prev = t
		out = append(out, Event{Name: name, At: t})
	}
	return out
}

// Next returns the first prayer strictly after now, looking into tomorrow when
// the day's last prayer has already passed.
func (s *Schedule) Next(now time.Time) (Event, error) {
	loc := s.Location()
	now = now.In(loc)
	for i := 0; i < 3; i++ {
		day := now.AddDate(0, 0, i)
		for _, ev := range s.EventsOn(day) {
			if ev.At.After(now) {
				return ev, nil
			}
		}
	}
	return Event{}, errors.New("no upcoming prayer found in the cached schedule")
}

// parseAt combines a YYYY-MM-DD day with an HH:MM clock time in a location.
func parseAt(day time.Time, clock string, loc *time.Location) (time.Time, error) {
	t, err := time.Parse("15:04", clock)
	if err != nil {
		return time.Time{}, fmt.Errorf("cannot parse time %q: %w", clock, err)
	}
	return time.Date(day.Year(), day.Month(), day.Day(), t.Hour(), t.Minute(), 0, 0, loc), nil
}

// LoadSchedule reads the cached schedule from disk. A missing or unreadable
// file is not an error: it yields an empty schedule that Ensure will refill.
func LoadSchedule(path string) *Schedule {
	s := &Schedule{Days: map[string]map[string]string{}}
	b, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	if err := json.Unmarshal(b, s); err != nil {
		return &Schedule{Days: map[string]map[string]string{}}
	}
	if s.Days == nil {
		s.Days = map[string]map[string]string{}
	}
	return s
}

// Save writes the schedule atomically.
func (s *Schedule) Save(path string) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Ensure guarantees the schedule covers the days around now, computing them
// locally from the sun's position. No network is involved, so this cannot fail
// because of DNS, an ISP reset, or a service outage.
//
// The signature keeps a context and an error for callers, but both are now
// only about arithmetic, not I/O.
func (s *Schedule) Ensure(ctx context.Context, lat, lon float64, method int, tune map[string]int, now time.Time) (bool, error) {
	if !s.Matches(lat, lon, method, tune) {
		// Location, method or tuning changed: everything cached is wrong.
		s.Days = map[string]map[string]string{}
		s.Latitude, s.Longitude, s.Method = lat, lon, method
		s.Tune = tune
	}
	if s.Days == nil {
		s.Days = map[string]map[string]string{}
	}

	loc := s.Location()
	params := ParamsFor(method)

	// Compute a rolling window so the daemon always has today, tomorrow, and
	// enough slack to survive a long sleep without recomputing.
	const windowDays = 40

	changed := false
	start := now.In(loc).AddDate(0, 0, -1)
	for i := 0; i < windowDays; i++ {
		day := start.AddDate(0, 0, i)
		key := day.Format("2006-01-02")
		if _, ok := s.Days[key]; ok {
			continue
		}
		s.Days[key] = applyTune(Compute(lat, lon, loc, day, params), tune)
		changed = true
	}

	if changed {
		s.Latitude, s.Longitude, s.Method = lat, lon, method
		s.Tune = tune
		s.FetchedAt = time.Now()
		s.prune(now)
	}
	return changed, nil
}

// applyTune shifts each time by its configured whole-minute offset.
func applyTune(times map[string]string, tune map[string]int) map[string]string {
	if len(tune) == 0 {
		return times
	}
	out := make(map[string]string, len(times))
	for name, clock := range times {
		off := tune[name]
		if off == 0 {
			out[name] = clock
			continue
		}
		t, err := time.Parse("15:04", clock)
		if err != nil {
			out[name] = clock
			continue
		}
		out[name] = t.Add(time.Duration(off) * time.Minute).Format("15:04")
	}
	return out
}

// Rebuild discards every cached day and recomputes the window. Use it when the
// calculation itself changes (a new method, or tuning offsets).
func (s *Schedule) Rebuild(lat, lon float64, method int, tune map[string]int, tz string, now time.Time) {
	s.Days = map[string]map[string]string{}
	s.Timezone = tz
	s.Latitude, s.Longitude, s.Method = lat, lon, method
	_, _ = s.Ensure(context.Background(), lat, lon, method, tune, now)
}

// prune drops days more than a month old so the cache cannot grow forever.
func (s *Schedule) prune(now time.Time) {
	cutoff := now.AddDate(0, -1, 0).Format("2006-01-02")
	for k := range s.Days {
		if k < cutoff {
			delete(s.Days, k)
		}
	}
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
