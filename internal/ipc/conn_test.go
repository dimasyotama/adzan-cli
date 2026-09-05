package ipc

import (
	"testing"
	"time"
)

func TestRequestResponseRoundTrip(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	srv, err := Listen()
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer srv.Close()

	go srv.Serve(func(req Request) Response {
		if req.Command != CmdStatus {
			return Response{Error: "unexpected command " + req.Command}
		}
		return Response{OK: true, Status: &Status{
			Location: "Purwakarta, Indonesia",
			NextName: "Maghrib",
			NextAt:   time.Date(2026, 9, 2, 17, 52, 0, 0, time.UTC),
		}}
	})

	if !Running() {
		t.Fatal("Running should be true while the server is up")
	}

	resp, err := Send(CmdStatus)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK || resp.Status == nil {
		t.Fatalf("bad response: %+v", resp)
	}
	if resp.Status.Location != "Purwakarta, Indonesia" || resp.Status.NextName != "Maghrib" {
		t.Errorf("status did not survive the round trip: %+v", resp.Status)
	}
}

func TestSendWithoutDaemon(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if Running() {
		t.Fatal("Running should be false with no server")
	}
	if _, err := Send(CmdStatus); err != ErrNoDaemon {
		t.Errorf("expected ErrNoDaemon, got %v", err)
	}
}

// Two daemons must not both bind the socket.
func TestDoubleListenRejected(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	srv, err := Listen()
	if err != nil {
		t.Fatalf("first Listen: %v", err)
	}
	defer srv.Close()
	go srv.Serve(func(Request) Response { return Response{OK: true} })

	if _, err := Listen(); err == nil {
		t.Error("a second Listen should be rejected while one is running")
	}
}
