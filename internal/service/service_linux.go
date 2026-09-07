//go:build linux

// Package service installs the daemon into the platform's own service manager
// so it survives logout and starts on login.
package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dimasyotama/adzan-cli/internal/spawn"
)

const daemonUnitName = "adzan.service"
const trayUnitName = "adzan-tray.service"

const daemonUnitTemplate = `[Unit]
Description=adzan - prayer times and the adhan
Documentation=https://github.com/dimasyotama/adzan-cli
After=network-online.target sound.target

[Service]
Type=simple
ExecStart=%s
Restart=on-failure
RestartSec=30

[Install]
WantedBy=default.target
`

// The tray is a GUI client, not the thing playing the adhan, so it wants the
// graphical session rather than default.target, and no restart-on-failure
// storm if the display server is briefly unavailable.
const trayUnitTemplate = `[Unit]
Description=adzan - next prayer in the tray
Documentation=https://github.com/dimasyotama/adzan-cli
After=graphical-session.target

[Service]
Type=simple
ExecStart=%s
Restart=on-failure
RestartSec=30

[Install]
WantedBy=graphical-session.target
`

func unitPath(name string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "systemd", "user", name), nil
}

func installUnit(name, tmpl, bin string) (string, error) {
	abs, err := filepath.Abs(bin)
	if err != nil {
		abs = bin
	}
	path, err := unitPath(name)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	content := fmt.Sprintf(tmpl, abs)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func uninstallUnit(name string) (string, error) {
	path, err := unitPath(name)
	if err != nil {
		return "", err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return path, nil
}

// disableUnit stops and disables a unit, best effort. A missing systemctl or
// an unregistered unit is not an error - there is simply nothing to turn off.
func disableUnit(name string) error {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return nil
	}
	_ = exec.Command("systemctl", "--user", "disable", "--now", name).Run()
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	return nil
}

// enableUnit reloads systemd and enables+starts the unit now, so it also
// comes back on every future login. A missing systemctl (no systemd, a
// container) means there is nothing to enable, not a failure.
func enableUnit(name string) error {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return nil
	}
	if err := exec.Command("systemctl", "--user", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("systemctl --user daemon-reload: %w", err)
	}
	if err := exec.Command("systemctl", "--user", "enable", "--now", name).Run(); err != nil {
		return fmt.Errorf("systemctl --user enable --now %s: %w", name, err)
	}
	return nil
}

// Install writes a systemd user unit pointing at the daemon binary and
// enables it, so it starts now and again on every future login/reboot.
func Install() (string, error) {
	bin, err := spawn.FindDaemon()
	if err != nil {
		return "", err
	}
	path, err := installUnit(daemonUnitName, daemonUnitTemplate, bin)
	if err != nil {
		return "", err
	}
	if err := enableUnit(daemonUnitName); err != nil {
		return path, fmt.Errorf("wrote %s but could not enable it: %w (enable manually: systemctl --user daemon-reload && systemctl --user enable --now %s)", path, err, daemonUnitName)
	}
	return path, nil
}

// Uninstall removes the unit file. Stopping it is left to the user so the
// command never silently kills a running service.
func Uninstall() (string, error) { return uninstallUnit(daemonUnitName) }

// Disable stops and disables the unit, best effort.
func Disable() error { return disableUnit(daemonUnitName) }

// InstallTray writes a systemd user unit pointing at the tray binary.
func InstallTray() (string, error) {
	bin, err := spawn.FindTray()
	if err != nil {
		return "", err
	}
	return installUnit(trayUnitName, trayUnitTemplate, bin)
}

// UninstallTray removes the tray's unit file.
func UninstallTray() (string, error) { return uninstallUnit(trayUnitName) }

// DisableTray stops and disables the tray's unit, best effort.
func DisableTray() error { return disableUnit(trayUnitName) }

// PostInstallTrayHint tells the user the two commands systemd still needs.
func PostInstallTrayHint() string {
	return "Enable it with: systemctl --user daemon-reload && systemctl --user enable --now adzan-tray"
}
