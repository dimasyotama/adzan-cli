//go:build windows

package ipc

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/dimasyotama/adzan-cli/internal/config"
)

// Windows has no Unix sockets in the standard library, so use a loopback TCP
// listener on an ephemeral port and record the port in a file the CLI reads.
func portFile() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, config.AppName+".port"), nil
}

func listen() (net.Listener, error) {
	pf, err := portFile()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(pf), 0o755); err != nil {
		return nil, err
	}
	if b, err := os.ReadFile(pf); err == nil {
		if c, derr := net.Dial("tcp", "127.0.0.1:"+strings.TrimSpace(string(b))); derr == nil {
			c.Close()
			return nil, errors.New("another adzan daemon is already running")
		}
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		l.Close()
		return nil, err
	}
	if err := os.WriteFile(pf, []byte(port), 0o600); err != nil {
		l.Close()
		return nil, err
	}
	return l, nil
}

func dial() (net.Conn, error) {
	pf, err := portFile()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(pf)
	if err != nil {
		return nil, err
	}
	return net.Dial("tcp", "127.0.0.1:"+strings.TrimSpace(string(b)))
}

func cleanup() {
	if pf, err := portFile(); err == nil {
		_ = os.Remove(pf)
	}
}
