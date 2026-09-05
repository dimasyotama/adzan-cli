package audio

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakePlayer substitutes a long-running process so playback state is testable
// without a sound card.
func fakePlayer(t *testing.T) {
	t.Helper()
	orig := commandFor
	commandFor = func(path string) ([]string, error) {
		return []string{"sleep", "30"}, nil
	}
	t.Cleanup(func() { commandFor = orig })
}

func tempSound(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "adhan.mp3")
	if err := os.WriteFile(p, []byte("not really audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPlayThenStop(t *testing.T) {
	fakePlayer(t)
	p := New()
	sound := tempSound(t)

	if p.Playing() {
		t.Fatal("a fresh player must not report playing")
	}
	if err := p.Play(sound); err != nil {
		t.Fatalf("Play: %v", err)
	}
	if !p.Playing() {
		t.Fatal("player should report playing after Play")
	}
	if !p.Stop() {
		t.Error("Stop should report that it silenced something")
	}

	// The reaper goroutine clears state shortly after the kill.
	deadline := time.Now().Add(2 * time.Second)
	for p.Playing() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if p.Playing() {
		t.Error("player still reports playing after Stop")
	}
}

func TestStopWhenIdle(t *testing.T) {
	fakePlayer(t)
	if New().Stop() {
		t.Error("Stop on an idle player should report false")
	}
}

// A second Play must replace the first rather than overlapping two adhans.
func TestPlayReplacesPrevious(t *testing.T) {
	fakePlayer(t)
	p := New()
	sound := tempSound(t)

	if err := p.Play(sound); err != nil {
		t.Fatalf("first Play: %v", err)
	}
	first := p.cmd
	if err := p.Play(sound); err != nil {
		t.Fatalf("second Play: %v", err)
	}
	if p.cmd == first {
		t.Error("second Play should have started a new process")
	}
	p.Stop()
}

func TestPlayMissingFile(t *testing.T) {
	fakePlayer(t)
	err := New().Play(filepath.Join(t.TempDir(), "absent.mp3"))
	if err == nil {
		t.Fatal("playing a missing file should fail")
	}
}
