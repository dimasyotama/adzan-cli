// Package audio plays the adhan by shelling out to whatever media player the
// host already has. That keeps the binary small and avoids CGo entirely.
package audio

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
)

// ErrNoPlayer means no supported command-line player was found on this system.
var ErrNoPlayer = errors.New("no supported audio player found")

// commandFor resolves the player argv. It is a variable so tests can swap in a
// predictable long-running process instead of a real audio player.
var commandFor = command

// Player runs at most one playback process at a time.
type Player struct {
	mu   sync.Mutex
	cmd  *exec.Cmd
	name string
}

// New returns an idle player.
func New() *Player { return &Player{} }

// Available reports whether a usable player exists, and which one.
func Available() (string, error) {
	argv, err := command("dummy")
	if err != nil {
		return "", err
	}
	return argv[0], nil
}

// Play starts the given audio file in the background, replacing anything that
// is already playing. It returns as soon as playback has been started.
func (p *Player) Play(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("adhan sound not found at %s: %w", path, err)
	}

	p.Stop()

	argv, err := commandFor(path)
	if err != nil {
		return err
	}

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	setProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not start %s: %w", argv[0], err)
	}

	p.mu.Lock()
	p.cmd = cmd
	p.name = argv[0]
	p.mu.Unlock()

	go func() {
		_ = cmd.Wait()
		p.mu.Lock()
		if p.cmd == cmd {
			p.cmd = nil
			p.name = ""
		}
		p.mu.Unlock()
	}()

	return nil
}

// Stop silences any current playback. It reports whether something was
// actually playing, so the CLI can tell the user "nothing to stop".
func (p *Player) Stop() bool {
	p.mu.Lock()
	cmd := p.cmd
	p.cmd = nil
	p.name = ""
	p.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return false
	}
	killProcess(cmd)
	return true
}

// Playing reports whether the adhan is sounding right now.
func (p *Player) Playing() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cmd != nil
}
