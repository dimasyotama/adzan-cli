// Package prayer talks to the Aladhan API and turns its responses into a
// local, cacheable schedule.
package prayer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// BaseURL is the Aladhan API root. No key required.
const BaseURL = "https://api.aladhan.com/v1"

// Place is the resolved metadata the API echoes back for a location.
type Place struct {
	Latitude  float64
	Longitude float64
	Timezone  string
}

// Day holds one date's timings, keyed by prayer name.
type Day struct {
	Date    string            `json:"date"` // YYYY-MM-DD
	Timings map[string]string `json:"timings"`
}

// Client is a thin HTTP wrapper with an IPv6 -> IPv4 fallback.
//
// Some networks (several Indonesian ISPs among them) accept an IPv6 connection
// and then reset it mid-TLS-handshake. Go's built-in dual-stack fallback only
// covers *connect* failures, so a post-connect reset surfaces as a hard error
// with no retry. We therefore keep two clients and fall back explicitly.
type Client struct {
	dual *http.Client
	v4   *http.Client

	mu       sync.Mutex
	preferV4 bool
}

// NewClient returns a client with a sane timeout and IPv4 fallback.
func NewClient() *Client {
	return &Client{
		dual: newHTTPClient("tcp"),
		v4:   newHTTPClient("tcp4"),
	}
}

// newHTTPClient builds a client pinned to one address family.
func newHTTPClient(network string) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	return &http.Client{
		Timeout: 25 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, addr)
			},
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second,
			ForceAttemptHTTP2:     true,
		},
	}
}

func (c *Client) usingV4() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.preferV4
}

// rememberV4 makes later requests in this process start with IPv4, so we pay
// the fallback cost once rather than on every call.
func (c *Client) rememberV4(v bool) {
	c.mu.Lock()
	c.preferV4 = v
	c.mu.Unlock()
}

// attempt describes one try: which client to use and how to name it in errors.
type attempt struct {
	client *http.Client
	label  string
	v4     bool
}

// do sends the request, retrying over IPv4 when the dual-stack attempt fails
// at the network level. HTTP responses (even 4xx/5xx) are returned as-is: a
// reply means the network is fine and retrying would be pointless.
func (c *Client) do(ctx context.Context, req *http.Request) (*http.Response, error) {
	attempts := []attempt{
		{c.dual, "IPv6/IPv4", false},
		{c.v4, "IPv4", true},
	}
	if c.usingV4() {
		attempts = []attempt{{c.v4, "IPv4", true}}
	}

	var lastErr error
	for _, a := range attempts {
		for try := 0; try < 2; try++ {
			resp, err := a.client.Do(req.Clone(ctx))
			if err == nil {
				c.rememberV4(a.v4)
				return resp, nil
			}
			lastErr = err

			// A cancelled context is the caller giving up, not a bad network.
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if try == 0 {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(400 * time.Millisecond):
				}
			}
		}
	}
	return nil, describeNetworkError(lastErr)
}

// describeNetworkError turns Go's raw transport errors into something a user
// can act on, because "read: connection reset by peer" explains nothing.
func describeNetworkError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()

	switch {
	case strings.Contains(msg, "connection reset by peer"),
		strings.Contains(msg, "connection refused"):
		return fmt.Errorf("the connection to the prayer-times service was reset, "+
			"over both IPv6 and IPv4. This usually means a network or ISP is "+
			"blocking the request rather than the service being down - try a "+
			"different network, or a VPN (%w)", err)

	case strings.Contains(msg, "no such host"),
		strings.Contains(msg, "server misbehaving"):
		return fmt.Errorf("could not resolve api.aladhan.com - check your DNS "+
			"settings or internet connection (%w)", err)

	case strings.Contains(msg, "timeout"),
		strings.Contains(msg, "deadline exceeded"),
		strings.Contains(msg, "i/o timeout"):
		return fmt.Errorf("the prayer-times service did not respond in time - "+
			"check your internet connection (%w)", err)

	case strings.Contains(msg, "certificate"),
		strings.Contains(msg, "tls:"):
		return fmt.Errorf("the secure connection to the prayer-times service "+
			"failed - a proxy or firewall may be intercepting traffic (%w)", err)
	}
	return fmt.Errorf("cannot reach the prayer-times service: %w", err)
}

// envelope is the outer shape every Aladhan response shares. Data is raw
// because it is an object for /timings, an array for /calendar, and a plain
// string for errors.
type envelope struct {
	Code   int             `json:"code"`
	Status string          `json:"status"`
	Data   json.RawMessage `json:"data"`
}

type apiDay struct {
	Timings map[string]string `json:"timings"`
	Date    struct {
		Gregorian struct {
			Date string `json:"date"` // DD-MM-YYYY
		} `json:"gregorian"`
	} `json:"date"`
	Meta struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Timezone  string  `json:"timezone"`
	} `json:"meta"`
}

func (c *Client) get(ctx context.Context, path string, q url.Values) (*envelope, error) {
	u := BaseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "adzan-cli")

	resp, err := c.do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}

	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("unexpected response from the prayer-times service (HTTP %d)", resp.StatusCode)
	}

	// A 400 with a string payload is how Aladhan reports a bad location.
	if resp.StatusCode == http.StatusBadRequest || env.Code == http.StatusBadRequest {
		var msg string
		_ = json.Unmarshal(env.Data, &msg)
		if msg == "" {
			msg = env.Status
		}
		return nil, fmt.Errorf("%w: %s", ErrLocationNotFound, msg)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prayer-times service returned HTTP %d", resp.StatusCode)
	}
	return &env, nil
}

// unresolvedLat and unresolvedLon are the coordinates Aladhan's geocoder
// falls back to (near Abuja, Nigeria) when it cannot resolve a city/country
// pair, instead of returning an error. It still answers with HTTP 200 and a
// plausible-looking timezone, so this has to be checked explicitly.
const unresolvedLat, unresolvedLon = 8.8888888, 7.7777777

func isUnresolvedSentinel(lat, lon float64) bool {
	return lat == unresolvedLat && lon == unresolvedLon
}

// ResolveCity validates a city/country pair and returns its coordinates.
func (c *Client) ResolveCity(ctx context.Context, city, country string, method int) (Place, error) {
	q := url.Values{}
	q.Set("city", city)
	q.Set("country", country)
	q.Set("method", strconv.Itoa(method))

	env, err := c.get(ctx, "/timingsByCity", q)
	if err != nil {
		return Place{}, err
	}
	var d apiDay
	if err := json.Unmarshal(env.Data, &d); err != nil {
		return Place{}, fmt.Errorf("could not read the location response: %w", err)
	}
	if (d.Meta.Latitude == 0 && d.Meta.Longitude == 0) || isUnresolvedSentinel(d.Meta.Latitude, d.Meta.Longitude) {
		return Place{}, fmt.Errorf("%w: %q, %q", ErrLocationNotFound, city, country)
	}
	return Place{
		Latitude:  d.Meta.Latitude,
		Longitude: d.Meta.Longitude,
		Timezone:  d.Meta.Timezone,
	}, nil
}

// ResolveCoords validates a latitude/longitude pair against the API and
// returns the timezone the service resolved for it.
func (c *Client) ResolveCoords(ctx context.Context, lat, lon float64, method int) (Place, error) {
	if err := ValidateCoords(lat, lon); err != nil {
		return Place{}, err
	}
	q := url.Values{}
	q.Set("latitude", formatFloat(lat))
	q.Set("longitude", formatFloat(lon))
	q.Set("method", strconv.Itoa(method))

	env, err := c.get(ctx, "/timings/"+time.Now().Format("02-01-2006"), q)
	if err != nil {
		return Place{}, err
	}
	var d apiDay
	if err := json.Unmarshal(env.Data, &d); err != nil {
		return Place{}, fmt.Errorf("could not read the location response: %w", err)
	}
	if len(d.Timings) == 0 {
		return Place{}, fmt.Errorf("%w: no timings for %s, %s", ErrLocationNotFound, formatFloat(lat), formatFloat(lon))
	}
	tz := d.Meta.Timezone
	if tz == "" {
		tz = "UTC"
	}
	return Place{Latitude: lat, Longitude: lon, Timezone: tz}, nil
}

// ValidateCoords does the cheap local range check before any network call.
func ValidateCoords(lat, lon float64) error {
	if lat < -90 || lat > 90 {
		return fmt.Errorf("latitude %s is out of range (must be between -90 and 90)", formatFloat(lat))
	}
	if lon < -180 || lon > 180 {
		return fmt.Errorf("longitude %s is out of range (must be between -180 and 180)", formatFloat(lon))
	}
	return nil
}

// Calendar fetches a whole month in one request. Fetching monthly rather than
// daily keeps the daemon off the network almost all of the time.
func (c *Client) Calendar(ctx context.Context, lat, lon float64, method, month, year int) ([]Day, error) {
	q := url.Values{}
	q.Set("latitude", formatFloat(lat))
	q.Set("longitude", formatFloat(lon))
	q.Set("method", strconv.Itoa(method))
	q.Set("month", strconv.Itoa(month))
	q.Set("year", strconv.Itoa(year))

	env, err := c.get(ctx, "/calendar", q)
	if err != nil {
		return nil, err
	}
	var days []apiDay
	if err := json.Unmarshal(env.Data, &days); err != nil {
		return nil, fmt.Errorf("could not read the calendar response: %w", err)
	}

	out := make([]Day, 0, len(days))
	for _, d := range days {
		iso, err := toISODate(d.Date.Gregorian.Date)
		if err != nil {
			continue
		}
		timings := make(map[string]string, len(d.Timings))
		for k, v := range d.Timings {
			timings[k] = CleanTime(v)
		}
		out = append(out, Day{Date: iso, Timings: timings})
	}
	if len(out) == 0 {
		return nil, errors.New("the calendar response contained no usable days")
	}
	return out, nil
}

// toISODate converts Aladhan's DD-MM-YYYY into YYYY-MM-DD.
func toISODate(s string) (string, error) {
	t, err := time.Parse("02-01-2006", s)
	if err != nil {
		return "", err
	}
	return t.Format("2006-01-02"), nil
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}
