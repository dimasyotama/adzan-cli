package prayer

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// The IPv6-reset case the user hit: the transport error must be translated
// into something that names the likely cause rather than leaking raw syscall
// wording.
func TestDescribeNetworkErrorExplainsResets(t *testing.T) {
	reset := &net.OpError{
		Op:  "read",
		Net: "tcp",
		Err: errors.New("read: connection reset by peer"),
	}
	got := describeNetworkError(reset).Error()
	if !strings.Contains(got, "blocking") {
		t.Errorf("a reset should be explained as possible blocking, got: %s", got)
	}
	if !strings.Contains(got, "IPv4") {
		t.Errorf("the message should mention that IPv4 was also tried, got: %s", got)
	}
}

func TestDescribeNetworkErrorCases(t *testing.T) {
	cases := map[string]string{
		"dial tcp: lookup api.aladhan.com: no such host": "DNS",
		"context deadline exceeded (i/o timeout)":        "did not respond in time",
		"tls: handshake failure":                         "proxy or firewall",
	}
	for input, want := range cases {
		got := describeNetworkError(errors.New(input)).Error()
		if !strings.Contains(got, want) {
			t.Errorf("describeNetworkError(%q) = %q, want it to mention %q", input, got, want)
		}
	}
	if describeNetworkError(nil) != nil {
		t.Error("nil error should stay nil")
	}
}

// An HTTP response, even an error status, must not trigger the IPv4 retry:
// the network is clearly working.
func TestDoDoesNotRetryOnHTTPStatus(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient()
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.do(context.Background(), req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()

	if hits != 1 {
		t.Errorf("a 500 response should be returned as-is, but the request ran %d times", hits)
	}
}

// A dead endpoint should be attempted over both families and then give up,
// rather than hanging or retrying forever.
func TestDoGivesUpAfterBothFamilies(t *testing.T) {
	c := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	// Port 1 on loopback refuses connections immediately.
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:1/", nil)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, err := c.do(ctx, req); err == nil {
		t.Fatal("expected a connection error")
	}
	if time.Since(start) > 7*time.Second {
		t.Error("do took too long to give up")
	}
}

// Aladhan's geocoder answers HTTP 200 with these exact coordinates (near
// Abuja, Nigeria) when it cannot resolve a city, instead of an error status.
// ResolveCity must treat that as "not found", not as a real place.
func TestIsUnresolvedSentinel(t *testing.T) {
	if !isUnresolvedSentinel(8.8888888, 7.7777777) {
		t.Error("the known Aladhan fallback coordinates should be flagged as unresolved")
	}
	if isUnresolvedSentinel(-6.5569, 107.4431) {
		t.Error("a real coordinate must not be flagged as unresolved")
	}
}

func TestUsingV4IsStickyAfterSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient()
	if c.usingV4() {
		t.Fatal("a fresh client should start dual-stack")
	}
	c.rememberV4(true)
	if !c.usingV4() {
		t.Error("rememberV4 should stick")
	}
}
