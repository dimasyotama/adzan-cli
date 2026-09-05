package ui

import (
	"testing"
	"time"
)

func TestSpinnerStartStopIsSafe(t *testing.T) {
	s := NewSpinner("Looking up Cibitung, Indonesia...")
	s.Start()
	time.Sleep(300 * time.Millisecond)
	s.Stop()
	s.Stop() // double stop must not panic or block
}

func TestSpinnerRunPropagatesError(t *testing.T) {
	s := NewSpinner("working")
	want := errTest
	if got := s.Run(func() error { return want }); got != want {
		t.Errorf("Run should return the callback error, got %v", got)
	}
}

func TestSpinnerStartTwice(t *testing.T) {
	s := NewSpinner("working")
	s.Start()
	s.Start()
	s.Stop()
}
