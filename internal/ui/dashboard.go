package ui

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/dimasyotama/adzan-cli/internal/ipc"
	"github.com/dimasyotama/adzan-cli/internal/prayer"
)

// pollInterval is how often the dashboard re-asks the daemon for state. The
// countdown itself ticks locally every second, so this can stay lazy.
const pollInterval = 5 * time.Second

// Dashboard renders the live view until the user presses Ctrl-C.
func Dashboard() error {
	st, err := fetchStatus()
	if err != nil {
		return err
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigs)

	enter()
	defer leave()

	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	poll := time.NewTicker(pollInterval)
	defer poll.Stop()

	render(st)
	for {
		select {
		case <-sigs:
			return nil
		case <-poll.C:
			if fresh, err := fetchStatus(); err == nil {
				st = fresh
			} else {
				leave()
				return err
			}
			render(st)
		case <-tick.C:
			render(st)
		}
	}
}

func fetchStatus() (*ipc.Status, error) {
	resp, err := ipc.Send(ipc.CmdStatus)
	if err != nil {
		return nil, err
	}
	if !resp.OK || resp.Status == nil {
		if resp.Error != "" {
			return nil, fmt.Errorf("%s", resp.Error)
		}
		return nil, fmt.Errorf("the daemon returned no status")
	}
	return resp.Status, nil
}

func enter() {
	fmt.Print(escAltScreen, escHideCursor, escClearScreen)
}

func leave() {
	fmt.Print(escShowCursor, escMainScreen)
}

// Render writes one frame of the dashboard.
func render(st *ipc.Status) {
	var b strings.Builder
	b.WriteString(escHome)

	for _, line := range strings.Split(strings.TrimRight(Silhouette(), "\n"), "\n") {
		b.WriteString("  " + line + escClearLine + "\n")
	}
	b.WriteString(escClearLine + "\n")

	// Header line: name on the left, daemon state on the right.
	state := Green("* running")
	if st.Muted {
		state = Amber("* muted")
	}
	if st.Playing {
		state = Amber("* adhan playing")
	}
	b.WriteString("  " + BoldFG("ADZAN") + "   " + state + escClearLine + "\n")
	b.WriteString("  " + Rule(SilhouetteWidth()) + escClearLine + "\n")

	b.WriteString("  " + Field("location", st.Location, 10) + escClearLine + "\n")
	if st.Coords != "" && st.Coords != st.Location {
		b.WriteString("  " + Field("", Dim(st.Coords), 10) + escClearLine + "\n")
	}
	b.WriteString("  " + Field("method", st.Method, 10) + escClearLine + "\n")
	if st.Timezone != "" {
		b.WriteString("  " + Field("timezone", st.Timezone, 10) + escClearLine + "\n")
	}
	b.WriteString(escClearLine + "\n")

	// Countdown to the next prayer.
	if !st.NextAt.IsZero() {
		remaining := time.Until(st.NextAt)
		b.WriteString("  " + Field("next",
			Amber(BoldFG(st.NextName))+"  "+BoldFG(Countdown(remaining)), 10) + escClearLine + "\n")
		b.WriteString("  " + Field("", Dim("at "+st.NextAt.Format("15:04")+" "+dayLabel(st.NextAt)), 10) + escClearLine + "\n")
	} else {
		b.WriteString("  " + Field("next", Dim("not available yet"), 10) + escClearLine + "\n")
	}
	b.WriteString(escClearLine + "\n")

	// Today's timetable, with the upcoming prayer marked.
	rows := append(append([]string{}, prayer.Order[:1]...), "Sunrise")
	rows = append(rows, prayer.Order[1:]...)
	for _, name := range rows {
		t, ok := st.Today[name]
		if !ok {
			continue
		}
		marker := "  "
		label := name
		if name == st.NextName {
			marker = Amber("> ")
			label = BoldFG(name)
		} else if name == "Sunrise" {
			label = Dim(name)
			t = Dim(t)
		}
		b.WriteString(fmt.Sprintf("  %s%-19s %s%s\n", marker, label, t, escClearLine))
	}

	b.WriteString(escClearLine + "\n")
	b.WriteString("  " + Dim("adzan stop") + Dim(" to silence") + Dim("   ") +
		Dim("adzan mute") + Dim(" to stay quiet") + Dim("   ") +
		Dim("ctrl-c") + Dim(" to exit") + escClearLine + "\n")
	b.WriteString("\x1b[J") // clear anything below

	fmt.Print(b.String())
}

// dayLabel distinguishes today's remaining prayers from tomorrow's first one.
func dayLabel(at time.Time) string {
	now := time.Now().In(at.Location())
	// Compare calendar days, not YearDay, so 31 Dec -> 1 Jan still reads right.
	day := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, at.Location())
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, at.Location())

	switch int(day.Sub(today).Hours() / 24) {
	case 0:
		return "today"
	case 1:
		return "tomorrow"
	default:
		return at.Format("Mon 2 Jan")
	}
}

// StatusOnce prints a single non-interactive snapshot, for `adzan status`.
func StatusOnce(st *ipc.Status) {
	fmt.Println()
	fmt.Println("  " + BoldFG("ADZAN") + "  " + Dim(st.Location))
	fmt.Println("  " + Rule(SilhouetteWidth()))
	if st.Coords != "" {
		fmt.Println("  " + Field("coords", st.Coords, 10))
	}
	fmt.Println("  " + Field("method", st.Method, 10))
	if st.Timezone != "" {
		fmt.Println("  " + Field("timezone", st.Timezone, 10))
	}
	if st.Muted {
		fmt.Println("  " + Field("state", Amber("muted"), 10))
	} else if st.Playing {
		fmt.Println("  " + Field("state", Amber("adhan playing"), 10))
	} else {
		fmt.Println("  " + Field("state", Green("running"), 10))
	}
	fmt.Println()

	if !st.NextAt.IsZero() {
		fmt.Println("  " + Field("next",
			BoldFG(st.NextName)+"  "+Countdown(time.Until(st.NextAt))+
				Dim("  at "+st.NextAt.Format("15:04")+" "+dayLabel(st.NextAt)), 10))
		fmt.Println()
	}

	rows := []string{"Fajr", "Sunrise", "Dhuhr", "Asr", "Maghrib", "Isha"}
	for _, name := range rows {
		t, ok := st.Today[name]
		if !ok {
			continue
		}
		marker := "  "
		if name == st.NextName {
			marker = Amber("> ")
		}
		fmt.Printf("  %s%-19s %s\n", marker, name, t)
	}
	fmt.Println()
}
