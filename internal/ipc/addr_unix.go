//go:build !windows

package ipc

import (
	"errors"
	"net"
	"os"
	"path/filepath"

	"github.com/dimasyotama/adzan-cli/internal/config"
)

// address returns the Unix domain socket path. XDG_RUNTIME_DIR is preferred
// because it is cleaned up on logout; otherwise the config dir is used.
func address() (string, error) {
	if r := os.Getenv("XDG_RUNTIME_DIR"); r != "" {
		return filepath.Join(r, config.AppName+".sock"), nil
	}
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, config.AppName+".sock"), nil
}

func listen() (net.Listener, error) {
	addr, err := address()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(addr), 0o755); err != nil {
		return nil, err
	}
	// A socket left behind by a crashed daemon would block Listen, so probe it
	// and remove it if nothing is answering.
	if _, err := os.Stat(addr); err == nil {
		if c, derr := net.Dial("unix", addr); derr == nil {
			c.Close()
			return nil, errors.New("another adzan daemon is already running")
		}
		_ = os.Remove(addr)
	}
	l, err := net.Listen("unix", addr)
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(addr, 0o600)
	return l, nil
}

func dial() (net.Conn, error) {
	addr, err := address()
	if err != nil {
		return nil, err
	}
	return net.Dial("unix", addr)
}

func cleanup() {
	if addr, err := address(); err == nil {
		_ = os.Remove(addr)
	}
}
