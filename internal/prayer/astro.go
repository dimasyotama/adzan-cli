package prayer

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// This file computes prayer times locally from the sun's position. No network
// is involved: the same arithmetic a prayer-times server runs, done here.
//
// The algorithm is the standard NOAA low-precision solar position model, which
// is accurate to well under a minute for these purposes. Prayer times are then
// hour angles on that solar position:
//
//	Fajr     sun is `FajrAngle` degrees below the horizon, before sunrise
//	Sunrise  upper limb touches the horizon (0.833 deg, allowing for refraction)
//	Dhuhr    solar noon (the sun crosses the meridian)
//	Asr      an object's shadow reaches `AsrShadow` times its length, plus noon shadow
//	Maghrib  sunset, the mirror of sunrise
//	Isha     sun is `IshaAngle` degrees below the horizon, after sunset

// ErrLocationNotFound means a city name could not be geocoded.
var ErrLocationNotFound = errors.New("location not found")

// Order is the five daily prayers the daemon announces. Sunrise is computed
// and displayed but never announced.
var Order = []string{"Fajr", "Dhuhr", "Asr", "Maghrib", "Isha"}

const (
	deg2rad = math.Pi / 180
	rad2deg = 180 / math.Pi

	// horizonAngle accounts for atmospheric refraction and the sun's radius.
	horizonAngle = 0.833
)

// Params describes one calculation convention.
type Params struct {
	FajrAngle float64 // degrees below horizon for Fajr
	IshaAngle float64 // degrees below horizon for Isha (0 if IshaInterval is used)

	// IshaInterval sets Isha a fixed number of minutes after Maghrib instead of
	// using an angle. Umm al-Qura works this way.
	IshaInterval int

	// AsrShadow is the shadow-length multiplier: 1 for the majority (Shafi,
	// Maliki, Hanbali), 2 for Hanafi.
	AsrShadow float64

	// Ihtiyat is the safety margin in minutes applied per prayer. Indonesian
	// practice (Kemenag) adds a couple of minutes to each time and subtracts
	// from sunrise, so nobody prays a moment early.
	Ihtiyat map[string]int
}

// Method describes a calculation convention for the setup wizard.
type Method struct {
	ID     int
	Name   string
	Params Params
}

// kemenagIhtiyat is the Indonesian safety margin: forward on every prayer,
// backward on sunrise so the Fajr window is never overstated.
var kemenagIhtiyat = map[string]int{
	"Fajr": 2, "Sunrise": -2, "Dhuhr": 4, "Asr": 2, "Maghrib": 2, "Isha": 2,
}

// noIhtiyat is used by conventions that publish unadjusted times.
var noIhtiyat = map[string]int{"Sunrise": 0}

// Methods lists the conventions adzan can compute. Each is a plain angle pair,
// which is why they can be evaluated offline.
var Methods = []Method{
	{20, "Kemenag - Indonesia", Params{FajrAngle: 20, IshaAngle: 18, AsrShadow: 1, Ihtiyat: kemenagIhtiyat}},
	{3, "Muslim World League", Params{FajrAngle: 18, IshaAngle: 17, AsrShadow: 1, Ihtiyat: noIhtiyat}},
	{2, "ISNA - North America", Params{FajrAngle: 15, IshaAngle: 15, AsrShadow: 1, Ihtiyat: noIhtiyat}},
	{4, "Umm al-Qura, Makkah", Params{FajrAngle: 18.5, IshaInterval: 90, AsrShadow: 1, Ihtiyat: noIhtiyat}},
	{5, "Egyptian General Authority", Params{FajrAngle: 19.5, IshaAngle: 17.5, AsrShadow: 1, Ihtiyat: noIhtiyat}},
	{1, "University of Islamic Sciences, Karachi", Params{FajrAngle: 18, IshaAngle: 18, AsrShadow: 1, Ihtiyat: noIhtiyat}},
	{11, "JAKIM - Malaysia, Singapore, Brunei", Params{FajrAngle: 20, IshaAngle: 18, AsrShadow: 1, Ihtiyat: noIhtiyat}},
	{13, "Diyanet - Turkey", Params{FajrAngle: 18, IshaAngle: 17, AsrShadow: 1, Ihtiyat: noIhtiyat}},
	{99, "Kemenag, Hanafi Asr", Params{FajrAngle: 20, IshaAngle: 18, AsrShadow: 2, Ihtiyat: kemenagIhtiyat}},
}

// DefaultMethod is Kemenag, the Indonesian convention.
const DefaultMethod = 20

// ParamsFor returns the convention for a method id, falling back to the
// default rather than failing on an unknown id from an old config.
func ParamsFor(id int) Params {
	for _, m := range Methods {
		if m.ID == id {
			return m.Params
		}
	}
	for _, m := range Methods {
		if m.ID == DefaultMethod {
			return m.Params
		}
	}
	return Params{FajrAngle: 20, IshaAngle: 18, AsrShadow: 1, Ihtiyat: kemenagIhtiyat}
}

// MethodName renders a method id for display.
func MethodName(id int) string {
	for _, m := range Methods {
		if m.ID == id {
			return m.Name
		}
	}
	return fmt.Sprintf("method %d", id)
}

// julianDay converts a civil date to a Julian day number at 00:00 UT.
func julianDay(y int, m int, d float64) float64 {
	if m <= 2 {
		y--
		m += 12
	}
	a := math.Floor(float64(y) / 100)
	b := 2 - a + math.Floor(a/4)
	return math.Floor(365.25*float64(y+4716)) +
		math.Floor(30.6001*float64(m+1)) + d + b - 1524.5
}

// sunPosition returns the sun's declination in degrees and the equation of
// time in minutes for a Julian day.
func sunPosition(jd float64) (declination, eqTime float64) {
	d := jd - 2451545.0

	g := math.Mod(357.529+0.98560028*d, 360) // mean anomaly
	q := math.Mod(280.459+0.98564736*d, 360) // mean longitude

	// Ecliptic longitude: mean longitude plus the equation of the centre.
	l := math.Mod(q+1.915*math.Sin(g*deg2rad)+0.020*math.Sin(2*g*deg2rad), 360)

	e := 23.439 - 0.00000036*d // obliquity of the ecliptic

	ra := math.Atan2(math.Cos(e*deg2rad)*math.Sin(l*deg2rad), math.Cos(l*deg2rad)) * rad2deg / 15
	ra = math.Mod(ra+24, 24)

	declination = math.Asin(math.Sin(e*deg2rad)*math.Sin(l*deg2rad)) * rad2deg

	eqTime = (q/15 - ra) * 60
	for eqTime > 20 {
		eqTime -= 1440
	}
	for eqTime < -20 {
		eqTime += 1440
	}
	return declination, eqTime
}

// hourAngle returns the hours between solar noon and the moment the sun sits
// `angle` degrees below the horizon. ok is false in polar conditions where the
// sun never reaches that angle.
func hourAngle(lat, declination, angle float64) (h float64, ok bool) {
	num := -math.Sin(angle*deg2rad) - math.Sin(lat*deg2rad)*math.Sin(declination*deg2rad)
	den := math.Cos(lat*deg2rad) * math.Cos(declination*deg2rad)
	if den == 0 {
		return 0, false
	}
	x := num / den
	if x < -1 || x > 1 {
		return 0, false
	}
	return math.Acos(x) * rad2deg / 15, true
}

// asrHourAngle returns the hours after solar noon at which an object's shadow
// reaches `shadow` times its length plus its noon shadow.
func asrHourAngle(lat, declination, shadow float64) (float64, bool) {
	altitude := math.Atan(1/(shadow+math.Tan(math.Abs(lat-declination)*deg2rad))) * rad2deg
	return hourAngle(lat, declination, -altitude)
}

// Compute returns the day's prayer times as "HH:MM" strings in loc.
//
// The returned map always contains the five prayers plus Sunrise. In polar
// regions where Fajr or Isha have no true solution, the middle-of-the-night
// rule is used and the value is still filled in, because a missing prayer time
// is worse than an approximated one.
func Compute(lat, lon float64, loc *time.Location, day time.Time, p Params) map[string]string {
	if loc == nil {
		loc = time.UTC
	}
	day = day.In(loc)

	// Timezone offset in hours, taken at local noon so DST is resolved for the
	// day being computed rather than for "now".
	noonLocal := time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, loc)
	_, offsetSeconds := noonLocal.Zone()
	tz := float64(offsetSeconds) / 3600

	// Julian day at local midnight, corrected toward the observer's meridian.
	jd := julianDay(day.Year(), int(day.Month()), float64(day.Day())) - lon/(15*24)
	declination, eqTime := sunPosition(jd)

	// Solar noon in local clock time.
	noon := 12 + tz - lon/15 - eqTime/60

	out := make(map[string]string, 6)
	set := func(name string, hours float64) {
		out[name] = formatClock(hours + float64(p.Ihtiyat[name])/60)
	}

	set("Dhuhr", noon)

	// Sunrise and sunset anchor everything else. In polar day or night the sun
	// never crosses the horizon, so fall back to a nominal 12-hour day: the
	// times are approximations, but a missing prayer time is worse.
	sunHA, haveSun := hourAngle(lat, declination, horizonAngle)
	if !haveSun {
		sunHA = 6
	}
	sunrise := noon - sunHA
	sunset := noon + sunHA
	set("Sunrise", sunrise)
	set("Maghrib", sunset)

	// night is the span from sunset to the next sunrise, used by the
	// high-latitude fallback below.
	night := 24 - 2*sunHA

	// Fajr. Above roughly 48 degrees of latitude the sun may never reach the
	// required depression angle in summer, leaving no true solution. The
	// angle-based rule (the PrayTimes convention) then places Fajr and Isha a
	// proportion of the night away from sunrise and sunset, where the
	// proportion is the method's own angle over 60.
	if fa, ok := hourAngle(lat, declination, p.FajrAngle); ok {
		set("Fajr", noon-fa)
	} else {
		set("Fajr", sunrise-night*(p.FajrAngle/60))
	}

	switch {
	case p.IshaInterval > 0:
		// Fixed interval after Maghrib (Umm al-Qura).
		set("Isha", sunset+float64(p.IshaInterval)/60)
	default:
		if ia, ok := hourAngle(lat, declination, p.IshaAngle); ok {
			set("Isha", noon+ia)
		} else {
			set("Isha", sunset+night*(p.IshaAngle/60))
		}
	}

	if aa, ok := asrHourAngle(lat, declination, p.AsrShadow); ok {
		set("Asr", noon+aa)
	} else {
		// Only reachable in polar conditions; put Asr midway to sunset.
		set("Asr", noon+sunHA/2)
	}

	return out
}

// formatClock renders decimal hours as HH:MM, wrapping at midnight.
func formatClock(h float64) string {
	// Round to the nearest minute before wrapping so 23:59.6 becomes 00:00.
	totalMinutes := int(math.Round(h * 60))
	totalMinutes = ((totalMinutes % 1440) + 1440) % 1440
	return fmt.Sprintf("%02d:%02d", totalMinutes/60, totalMinutes%60)
}
