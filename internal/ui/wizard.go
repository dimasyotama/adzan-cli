package ui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dimasyotama/adzan-cli/internal/config"
	"github.com/dimasyotama/adzan-cli/internal/prayer"
)

// ErrAborted is returned when the user cancels the wizard with Ctrl-D.
var ErrAborted = errors.New("setup cancelled")

// Wizard walks the user through choosing a location and a calculation method.
// It validates every answer against the prayer-times service before saving, so
// a bad city name or an impossible coordinate never reaches the daemon.
type Wizard struct {
	in     *bufio.Reader
	out    io.Writer
	client *prayer.Client
}

// NewWizard builds a wizard reading from stdin.
func NewWizard() *Wizard {
	return &Wizard{
		in:     bufio.NewReader(os.Stdin),
		out:    os.Stdout,
		client: prayer.NewClient(),
	}
}

func (w *Wizard) printf(format string, a ...any) {
	fmt.Fprintf(w.out, format, a...)
}

// Run returns a validated configuration. Existing is used to pre-fill answers
// when re-running setup; pass nil for a first run.
func (w *Wizard) Run(ctx context.Context, existing *config.Config) (*config.Config, error) {
	w.printf("\n%s\n", Silhouette())
	w.printf("  %s\n", BoldFG("Welcome to adzan"))
	w.printf("  %s\n\n", Dim("Prayer times in your terminal, with the adhan at the right moment."))

	method, err := w.askMethod(existing)
	if err != nil {
		return nil, err
	}

	loc, err := w.askLocation(ctx, method)
	if err != nil {
		return nil, err
	}

	cfg := &config.Config{
		Version:  1,
		Location: loc,
		Method:   method,
	}
	if existing != nil {
		cfg.SoundPath = existing.SoundPath
		cfg.Muted = existing.Muted
	}

	w.printf("\n  %s %s\n", Green("OK"), "Location confirmed: "+BoldFG(loc.Label))
	w.printf("     %s\n", Dim(fmt.Sprintf("%.4f, %.4f  ·  %s  ·  %s",
		loc.Latitude, loc.Longitude, loc.Timezone, prayer.MethodName(method))))

	return cfg, nil
}

// askMethod offers the common calculation conventions.
func (w *Wizard) askMethod(existing *config.Config) (int, error) {
	def := prayer.DefaultMethod
	if existing != nil && existing.Method != 0 {
		def = existing.Method
	}

	w.printf("  %s\n", Bold("Calculation method"))
	for i, m := range prayer.Methods {
		marker := " "
		if m.ID == def {
			marker = Amber("*")
		}
		w.printf("   %s %d) %s\n", marker, i+1, m.Name)
	}
	w.printf("\n")

	for {
		ans, err := w.prompt(fmt.Sprintf("  Choose 1-%d [%s]: ", len(prayer.Methods), prayer.MethodName(def)))
		if err != nil {
			return 0, err
		}
		if ans == "" {
			return def, nil
		}
		n, err := strconv.Atoi(ans)
		if err != nil || n < 1 || n > len(prayer.Methods) {
			w.errorf("Enter a number between 1 and %d, or press Enter for the default.", len(prayer.Methods))
			continue
		}
		return prayer.Methods[n-1].ID, nil
	}
}

// askLocation loops until the user gives something the API can resolve.
func (w *Wizard) askLocation(ctx context.Context, method int) (config.Location, error) {
	w.printf("\n  %s\n", Bold("Where are you?"))
	w.printf("   %s\n", Dim("1) Coordinates      e.g. -6.2618, 107.1447   (exact, works offline)"))
	w.printf("   %s\n\n", Dim("2) City name        e.g. Cibitung, Indonesia  (needs a lookup)"))
	w.printf("  %s\n\n", Dim("Tip: long-press your location in any maps app to copy its coordinates."))

	for {
		mode, err := w.prompt("  Choose 1 or 2 [1]: ")
		if err != nil {
			return config.Location{}, err
		}
		switch mode {
		case "", "1":
			loc, err := w.askCoords(ctx, method)
			if errors.Is(err, errRetry) {
				continue
			}
			return loc, err
		case "2":
			loc, err := w.askCity(ctx, method)
			if errors.Is(err, errRetry) {
				continue
			}
			return loc, err
		default:
			w.errorf("Enter 1 for coordinates or 2 for a city name.")
		}
	}
}

// errRetry signals "ask again from the top of the location step".
var errRetry = errors.New("retry")

func (w *Wizard) askCity(ctx context.Context, method int) (config.Location, error) {
	for {
		city, err := w.prompt("\n  City: ")
		if err != nil {
			return config.Location{}, err
		}
		if city == "" {
			w.errorf("A city name is required.")
			continue
		}
		country, err := w.prompt("  Country: ")
		if err != nil {
			return config.Location{}, err
		}
		if country == "" {
			w.errorf("A country is required so the city can be resolved unambiguously.")
			continue
		}

		sp := NewSpinner(fmt.Sprintf("Looking up %s, %s...", city, country))
		var place prayer.Place
		sp.Start()
		place, err = w.client.ResolveCity(ctx, city, country, method)
		sp.Stop()

		if err != nil {
			if errors.Is(err, prayer.ErrLocationNotFound) {
				w.errorf("Could not find %q in %q. Check the spelling, or choose coordinates instead.",
					city, country)
				continue
			}
			w.errorf("%v", err)
			continue
		}

		// Show what the lookup actually resolved to and require a yes.
		// A geocoder can silently return a place thousands of kilometres away
		// that shares a name, and wrong coordinates mean wrong prayer times
		// with no other symptom.
		w.printf("\n  %s resolved to %s\n", Green("OK"),
			BoldFG(fmt.Sprintf("%.4f, %.4f", place.Latitude, place.Longitude)))
		w.printf("  %s\n", Dim("timezone "+place.Timezone))
		w.printf("  %s\n\n", Dim("Check this on a map before accepting - "+
			"a wrong match here is the one error adzan cannot detect for you."))

		if !Confirm("Is that the right place?") {
			w.printf("  %s\n", Dim("Enter the coordinates directly instead, or try a more specific name."))
			continue
		}

		return config.Location{
			Mode:      config.ModeCity,
			City:      city,
			Country:   country,
			Latitude:  place.Latitude,
			Longitude: place.Longitude,
			Timezone:  place.Timezone,
			Label:     city + ", " + country,
		}, nil
	}
}

func (w *Wizard) askCoords(ctx context.Context, method int) (config.Location, error) {
	for {
		latStr, err := w.prompt("\n  Latitude  (-90 to 90): ")
		if err != nil {
			return config.Location{}, err
		}
		lat, err := parseCoord(latStr)
		if err != nil {
			w.errorf("%q is not a valid number.", latStr)
			continue
		}
		// Check each field as it is entered, so the message points at the
		// value the user just typed rather than at the pair.
		if lat < -90 || lat > 90 {
			w.errorf("Latitude must be between -90 and 90, got %s.", latStr)
			continue
		}

		lonStr, err := w.prompt("  Longitude (-180 to 180): ")
		if err != nil {
			return config.Location{}, err
		}
		lon, err := parseCoord(lonStr)
		if err != nil {
			w.errorf("%q is not a valid number.", lonStr)
			continue
		}
		if lon < -180 || lon > 180 {
			w.errorf("Longitude must be between -180 and 180, got %s.", lonStr)
			continue
		}
		if err := prayer.ValidateCoords(lat, lon); err != nil {
			w.errorf("%v", err)
			continue
		}

		// No lookup needed: prayer times are computed from these numbers
		// directly. The timezone comes from the system, which is right for
		// anyone setting up on the machine they are actually sitting at.
		tz := time.Local.String()
		w.printf("\n  %s %s\n", Green("OK"), fmt.Sprintf("%.4f, %.4f  ·  timezone %s", lat, lon, tz))

		if !w.confirmTimezone(&tz) {
			continue
		}

		return config.Location{
			Mode:      config.ModeCoords,
			Latitude:  lat,
			Longitude: lon,
			Timezone:  tz,
			Label:     fmt.Sprintf("%.4f, %.4f", lat, lon),
		}, nil
	}
}

// confirmTimezone lets the user override a wrong system timezone, which
// matters when setting up on a server in another region.
func (w *Wizard) confirmTimezone(tz *string) bool {
	answer, err := w.prompt(fmt.Sprintf("  Timezone [%s]: ", *tz))
	if err != nil {
		return false
	}
	if answer == "" {
		return true
	}
	if _, lerr := time.LoadLocation(answer); lerr != nil {
		w.errorf("%q is not a known timezone. Use a name like Asia/Jakarta.", answer)
		return false
	}
	*tz = answer
	return true
}

func parseCoord(s string) (float64, error) {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), ","))
	return strconv.ParseFloat(s, 64)
}

func (w *Wizard) prompt(label string) (string, error) {
	w.printf("%s", label)
	line, err := w.in.ReadString('\n')
	if errors.Is(err, io.EOF) {
		w.printf("\n")
		return "", ErrAborted
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func (w *Wizard) errorf(format string, a ...any) {
	w.printf("  %s %s\n", Red("x"), fmt.Sprintf(format, a...))
}

// Countdown formats a duration as HH:MM:SS.
func Countdown(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d.Round(time.Second).Seconds())
	return fmt.Sprintf("%02d:%02d:%02d", total/3600, (total%3600)/60, total%60)
}

// Confirm asks a yes/no question, defaulting to no. A non-interactive stdin
// (a pipe, or EOF) answers no, so scripted runs never delete anything by
// accident.
func Confirm(question string) bool {
	fmt.Printf("  %s %s ", question, Dim("[y/N]:"))
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println()
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}
