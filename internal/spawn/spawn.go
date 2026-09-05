package spawn

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/dimasyotama/adzan-cli/internal/config"
	"github.com/dimasyotama/adzan-cli/internal/ipc"
)

func exeName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

// DaemonName is the daemon executable, looked for next to the CLI first.
func DaemonName() string { return exeName("adzand") }

// TrayName is the menu bar / tray executable, looked for next to the CLI first.
func TrayName() string { return exeName("adzantray") }

// findBinary locates a companion executable: alongside the CLI, then on PATH.
func findBinary(name string) (string, error) {
	self, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(self), name)
		if fi, serr := os.Stat(candidate); serr == nil && !fi.IsDir() {
			return candidate, nil
		}
	}
	if p, lerr := exec.LookPath(name); lerr == nil {
		return p, nil
	}
	return "", fmt.Errorf("could not find %s next to the adzan binary or on PATH", name)
}

// FindDaemon locates the daemon binary: alongside the CLI, then on PATH.
func FindDaemon() (string, error) { return findBinary(DaemonName()) }

// FindTray locates the tray binary: alongside the CLI, then on PATH.
func FindTray() (string, error) { return findBinary(TrayName()) }

// Start launches the daemon detached and waits briefly for it to answer.
func Start() error {
	if ipc.Running() {
		return fmt.Errorf("the adzan daemon is already running")
	}

	bin, err := FindDaemon()
	if err != nil {
		return err
	}
	if err := config.EnsureDirs(); err != nil {
		return err
	}

	logPath, err := config.LogPath()
	if err != nil {
		return err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("cannot open log file %s: %w", logPath, err)
	}
	defer logFile.Close()

	cmd := exec.Command(bin)
	cmd.Stdin = nil
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	detach(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not start the daemon: %w", err)
	}
	// Do not wait on the child; release it so it is reparented to init.
	_ = cmd.Process.Release()

	// Give it a moment to bind the socket, then confirm it is alive.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if ipc.Running() {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("the daemon did not start; see %s for details", logPath)
}

// StartTray launches the menu bar icon detached. There is no socket to
// confirm against, so a clean exec.Start is all the guarantee there is.
func StartTray() error {
	bin, err := FindTray()
	if err != nil {
		return err
	}
	cmd := exec.Command(bin)
	cmd.Stdin = nil
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not start the tray icon: %w", err)
	}
	return cmd.Process.Release()
}
