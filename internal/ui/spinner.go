package ui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// spinnerFrames is the braille cycle; it renders in any UTF-8 terminal and
// stays on one cell width so the line never jitters.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// asciiFrames is the fallback where the terminal cannot be trusted with UTF-8.
var asciiFrames = []string{"|", "/", "-", "\\"}

// Spinner animates a single line while a slow operation runs. On a
// non-interactive stdout it degrades to printing the message once, so piped
// output and CI logs do not fill up with escape codes.
type Spinner struct {
	message string

	mu      sync.Mutex
	stop    chan struct{}
	done    chan struct{}
	running bool
	static  bool
}

// NewSpinner creates a stopped spinner.
func NewSpinner(message string) *Spinner {
	return &Spinner{
		message: message,
		static:  !colorEnabled, // same TTY probe the rest of the UI uses
	}
}

// Start begins animating. Calling Start twice is a no-op.
func (s *Spinner) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	stop, done := s.stop, s.done
	s.mu.Unlock()

	if s.static {
		fmt.Printf("  %s\n", Dim(s.message))
		close(done)
		return
	}

	frames := spinnerFrames
	if !utf8Terminal() {
		frames = asciiFrames
	}

	go func() {
		defer close(done)
		ticker := time.NewTicker(90 * time.Millisecond)
		defer ticker.Stop()

		i := 0
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				fmt.Printf("\r  %s %s%s", Cyan(frames[i%len(frames)]), Dim(s.message), escClearLine)
				i++
			}
		}
	}()
}

// Stop halts the animation and clears the line.
func (s *Spinner) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	stop, done := s.stop, s.done
	s.mu.Unlock()

	if !s.static {
		close(stop)
	}
	<-done

	if !s.static {
		// Wipe the whole line so the next print starts clean.
		fmt.Printf("\r%s\r", strings.Repeat(" ", len([]rune(s.message))+6))
	}
}

// StopWith clears the animation and leaves a final line in its place.
func (s *Spinner) StopWith(line string) {
	s.Stop()
	fmt.Printf("  %s\n", line)
}

// Run animates while fn executes, and always clears the line afterwards.
func (s *Spinner) Run(fn func() error) error {
	s.Start()
	err := fn()
	s.Stop()
	return err
}

// utf8Terminal guesses whether box-drawing and braille will render.
func utf8Terminal() bool {
	for _, key := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		v := strings.ToUpper(os.Getenv(key))
		if v == "" {
			continue
		}
		return strings.Contains(v, "UTF-8") || strings.Contains(v, "UTF8")
	}
	// Windows Terminal and modern macOS/Linux terminals are UTF-8 by default
	// even with no locale set, so default to yes rather than to ASCII.
	return true
}
