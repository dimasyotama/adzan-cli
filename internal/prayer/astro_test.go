package prayer

import (
	"testing"
	"time"
)

// mins parses "HH:MM" into minutes past midnight.
func mins(t *testing.T, s string) int {
	t.Helper()
	var h, m int
	if _, err := timeParse(s, &h, &m); err != nil {
		t.Fatalf("bad time %q: %v", s, err)
	}
	return h*60 + m
}

func timeParse(s string, h, m *int) (int, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, err
	}
	*h, *m = t.Hour(), t.Minute()
	return 2, nil
}

func mustLoad(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Skipf("tzdata for %s unavailable", name)
	}
	return loc
}

// TestCibitungAgainstPublishedTimes is the regression test for the bug that
// prompted local calculation: the API geocoded "Cibitung, Indonesia" to a point
// ~2000 km away and the wrong times were accepted silently.
//
// The reference values are the published Kemenag times for Cibitung,
// Kabupaten Bekasi on 2 September 2026.
func TestCibitungAgainstPublishedTimes(t *testing.T) {
	loc := mustLoad(t, "Asia/Jakarta")
	day := time.Date(2026, 9, 2, 0, 0, 0, 0, loc)

	got := Compute(-6.2618, 107.1447, loc, day, ParamsFor(20))

	published := map[string]string{
		"Fajr":    "04:36",
		"Sunrise": "05:48",
		"Dhuhr":   "11:55",
		"Asr":     "15:12",
		"Maghrib": "17:54",
		"Isha":    "19:00",
	}

	// Three minutes covers rounding and small differences in how each
	// publisher applies its safety margin.
	const tolerance = 3

	for name, want := range published {
		g, ok := got[name]
		if !ok {
			t.Errorf("%s missing from computed times", name)
			continue
		}
		diff := mins(t, g) - mins(t, want)
		if diff < -tolerance || diff > tolerance {
			t.Errorf("%s = %s, published %s (off by %+d min, tolerance %d)",
				name, g, want, diff, tolerance)
		}
	}
}

// The wrong coordinates must produce visibly wrong times, so this test proves
// the calculation is actually sensitive to location rather than accidentally
// right for the wrong reason.
func TestWrongCoordinatesProduceWrongTimes(t *testing.T) {
	loc := mustLoad(t, "Asia/Jakarta")
	day := time.Date(2026, 9, 2, 0, 0, 0, 0, loc)

	right := Compute(-6.2618, 107.1447, loc, day, ParamsFor(20))
	wrong := Compute(9.40, 97.85, loc, day, ParamsFor(20)) // what the API returned

	diff := mins(t, wrong["Dhuhr"]) - mins(t, right["Dhuhr"])
	if diff < 30 {
		t.Errorf("bad coordinates should shift Dhuhr by ~38 min, got %+d", diff)
	}
}

// Jakarta on a different date, to check the calculation is not tuned to one day.
func TestJakartaMidYear(t *testing.T) {
	loc := mustLoad(t, "Asia/Jakarta")
	day := time.Date(2026, 6, 15, 0, 0, 0, 0, loc)

	got := Compute(-6.2088, 106.8456, loc, day, ParamsFor(20))

	// Around the June solstice in Jakarta, Dhuhr sits near 11:53 and the day
	// is a little short of 12 hours.
	dhuhr := mins(t, got["Dhuhr"])
	if dhuhr < 11*60+45 || dhuhr > 12*60+5 {
		t.Errorf("Dhuhr = %s, expected close to 11:55", got["Dhuhr"])
	}

	dayLength := mins(t, got["Maghrib"]) - mins(t, got["Sunrise"])
	if dayLength < 11*60+30 || dayLength > 12*60 {
		t.Errorf("day length = %d min, expected a little under 12 h", dayLength)
	}
}

// Makkah with Umm al-Qura: Isha is a fixed 90 minutes after Maghrib.
func TestUmmAlQuraIshaInterval(t *testing.T) {
	loc := mustLoad(t, "Asia/Riyadh")
	day := time.Date(2026, 9, 2, 0, 0, 0, 0, loc)

	got := Compute(21.4225, 39.8262, loc, day, ParamsFor(4))

	gap := mins(t, got["Isha"]) - mins(t, got["Maghrib"])
	if gap != 90 {
		t.Errorf("Umm al-Qura Isha should be 90 min after Maghrib, got %d", gap)
	}
}

// Hanafi Asr must fall later than the majority position.
func TestHanafiAsrIsLater(t *testing.T) {
	loc := mustLoad(t, "Asia/Jakarta")
	day := time.Date(2026, 9, 2, 0, 0, 0, 0, loc)

	standard := Compute(-6.2618, 107.1447, loc, day, ParamsFor(20))
	hanafi := Compute(-6.2618, 107.1447, loc, day, ParamsFor(99))

	if mins(t, hanafi["Asr"]) <= mins(t, standard["Asr"]) {
		t.Errorf("Hanafi Asr (%s) should be later than standard (%s)",
			hanafi["Asr"], standard["Asr"])
	}
}

// Prayers must come out in the right order, every day of a year, at a spread
// of latitudes. This catches sign errors and midnight-wrap bugs.
func TestOrderingHoldsAllYear(t *testing.T) {
	locations := []struct {
		name     string
		lat, lon float64
		tz       string
	}{
		{"Cibitung", -6.26, 107.14, "Asia/Jakarta"},
		{"Makkah", 21.42, 39.83, "Asia/Riyadh"},
		{"London", 51.51, -0.13, "Europe/London"},
		{"Sydney", -33.87, 151.21, "Australia/Sydney"},
		{"Nairobi", -1.29, 36.82, "Africa/Nairobi"},
		{"Reykjavik", 64.13, -21.90, "Atlantic/Reykjavik"},
	}

	for _, place := range locations {
		t.Run(place.name, func(t *testing.T) {
			loc := mustLoad(t, place.tz)
			for d := 0; d < 365; d++ {
				day := time.Date(2026, 1, 1, 0, 0, 0, 0, loc).AddDate(0, 0, d)
				got := Compute(place.lat, place.lon, loc, day, ParamsFor(3))

				seq := []string{"Fajr", "Sunrise", "Dhuhr", "Asr", "Maghrib", "Isha"}
				prev := mins(t, got[seq[0]])
				for i := 1; i < len(seq); i++ {
					cur := mins(t, got[seq[i]])
					// Isha can legitimately cross midnight at high latitudes in
					// summer, so allow one wrap - but the gap must stay sane.
					if cur <= prev {
						cur += 24 * 60
					}
					if gap := cur - prev; gap > 12*60 {
						t.Fatalf("%s: %s (%s) is %d min after %s (%s), which is not plausible",
							day.Format("2006-01-02"), seq[i], got[seq[i]], gap, seq[i-1], got[seq[i-1]])
					}
					prev = cur
				}
			}
		})
	}
}

// Every method must yield all six entries, at a normal latitude and a hard one.
func TestAllMethodsProduceCompleteTimes(t *testing.T) {
	loc := time.UTC
	day := time.Date(2026, 6, 21, 0, 0, 0, 0, loc) // solstice, the awkward case

	for _, m := range Methods {
		for _, place := range []struct {
			name     string
			lat, lon float64
		}{
			{"Jakarta", -6.21, 106.85},
			{"Reykjavik", 64.13, -21.90},
		} {
			got := Compute(place.lat, place.lon, loc, day, m.Params)
			for _, name := range []string{"Fajr", "Sunrise", "Dhuhr", "Asr", "Maghrib", "Isha"} {
				if got[name] == "" {
					t.Errorf("%s at %s: %s is empty", m.Name, place.name, name)
				}
			}
		}
	}
}

func TestFormatClockWrapsMidnight(t *testing.T) {
	cases := map[float64]string{
		0:       "00:00",
		12.5:    "12:30",
		23.999:  "00:00", // rounds up past midnight
		-0.5:    "23:30", // negative wraps backward
		25:      "01:00",
		11.9917: "11:60 -> should not happen",
	}
	for in, want := range cases {
		got := formatClock(in)
		if want == "11:60 -> should not happen" {
			if got == "11:60" {
				t.Errorf("formatClock(%v) produced an invalid clock time %q", in, got)
			}
			continue
		}
		if got != want {
			t.Errorf("formatClock(%v) = %q, want %q", in, got, want)
		}
	}
}

// A prayer that crosses midnight must be scheduled on the following day, not
// treated as having already passed. This happens at high latitudes in summer.
func TestEventsOnRollsMidnightCrossing(t *testing.T) {
	loc := mustLoad(t, "Europe/London")
	s := &Schedule{
		Timezone: "Europe/London",
		Days: map[string]map[string]string{
			"2026-05-23": {
				"Fajr": "02:20", "Dhuhr": "13:00", "Asr": "17:15",
				"Maghrib": "20:56", "Isha": "00:04", // after midnight
			},
		},
	}

	day := time.Date(2026, 5, 23, 0, 0, 0, 0, loc)
	events := s.EventsOn(day)
	if len(events) != 5 {
		t.Fatalf("expected 5 prayers, got %d", len(events))
	}

	last := events[len(events)-1]
	if last.Name != "Isha" {
		t.Fatalf("last event should be Isha, got %s", last.Name)
	}
	if got := last.At.Format("2006-01-02 15:04"); got != "2026-05-24 00:04" {
		t.Errorf("Isha should roll to the next day, got %s", got)
	}

	// And every event must still be in ascending order.
	for i := 1; i < len(events); i++ {
		if !events[i].At.After(events[i-1].At) {
			t.Errorf("%s is not after %s", events[i].Name, events[i-1].Name)
		}
	}
}

// Evening on a day whose Isha crosses midnight must still find that Isha.
func TestNextFindsMidnightCrossingIsha(t *testing.T) {
	loc := mustLoad(t, "Europe/London")
	s := &Schedule{
		Timezone: "Europe/London",
		Days: map[string]map[string]string{
			"2026-05-23": {
				"Fajr": "02:20", "Dhuhr": "13:00", "Asr": "17:15",
				"Maghrib": "20:56", "Isha": "00:04",
			},
		},
	}

	now := time.Date(2026, 5, 23, 22, 0, 0, 0, loc) // after Maghrib
	ev, err := s.Next(now)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if ev.Name != "Isha" {
		t.Errorf("next prayer = %s, want Isha", ev.Name)
	}
}

func TestApplyTuneShiftsTimes(t *testing.T) {
	in := map[string]string{"Fajr": "04:36", "Isha": "19:03", "Dhuhr": "11:55"}
	got := applyTune(in, map[string]int{"Isha": -3, "Fajr": 1})

	if got["Isha"] != "19:00" {
		t.Errorf("Isha with -3 = %s, want 19:00", got["Isha"])
	}
	if got["Fajr"] != "04:37" {
		t.Errorf("Fajr with +1 = %s, want 04:37", got["Fajr"])
	}
	if got["Dhuhr"] != "11:55" {
		t.Errorf("untuned Dhuhr should be unchanged, got %s", got["Dhuhr"])
	}
}

func TestApplyTuneWrapsMidnight(t *testing.T) {
	got := applyTune(map[string]string{"Isha": "23:58"}, map[string]int{"Isha": 5})
	if got["Isha"] != "00:03" {
		t.Errorf("tuning past midnight = %s, want 00:03", got["Isha"])
	}
}

// Changing the tuning must invalidate the cache, or the user would edit the
// config and see no difference.
func TestTuneChangeInvalidatesCache(t *testing.T) {
	loc := mustLoad(t, "Asia/Jakarta")
	now := time.Date(2026, 9, 2, 8, 0, 0, 0, loc)

	s := &Schedule{Timezone: "Asia/Jakarta", Days: map[string]map[string]string{}}
	if _, err := s.Ensure(nil, -6.2618, 107.1447, 20, nil, now); err != nil {
		t.Fatal(err)
	}
	before := s.Days["2026-09-02"]["Isha"]

	if _, err := s.Ensure(nil, -6.2618, 107.1447, 20, map[string]int{"Isha": -3}, now); err != nil {
		t.Fatal(err)
	}
	after := s.Days["2026-09-02"]["Isha"]

	if before == after {
		t.Errorf("tuning change should have recomputed Isha, still %s", after)
	}
	if !s.Matches(-6.2618, 107.1447, 20, map[string]int{"Isha": -3}) {
		t.Error("schedule should match the tuning it was built with")
	}
}

// The full pipeline, tuned to match the published times exactly.
func TestTunedCibitungMatchesPhoneExactly(t *testing.T) {
	loc := mustLoad(t, "Asia/Jakarta")
	now := time.Date(2026, 9, 2, 3, 0, 0, 0, loc)

	tune := map[string]int{"Sunrise": -2, "Maghrib": 1, "Isha": -3}
	s := &Schedule{Timezone: "Asia/Jakarta", Days: map[string]map[string]string{}}
	if _, err := s.Ensure(nil, -6.2618, 107.1447, 20, tune, now); err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"Fajr": "04:36", "Sunrise": "05:48", "Dhuhr": "11:55",
		"Asr": "15:12", "Maghrib": "17:54", "Isha": "19:00",
	}
	got := s.Days["2026-09-02"]
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s = %s, want %s", name, got[name], w)
		}
	}
}
