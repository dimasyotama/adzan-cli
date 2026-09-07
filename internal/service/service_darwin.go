//go:build darwin

package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/dimasyotama/adzan-cli/internal/config"
	"github.com/dimasyotama/adzan-cli/internal/spawn"
)

const daemonLabel = "com.github.dimasyotama.adzan"
const trayLabel = "com.github.dimasyotama.adzan.tray"

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <dict>
        <key>SuccessfulExit</key>
        <false/>
    </dict>
    <key>StandardOutPath</key>
    <string>%s</string>
    <key>StandardErrorPath</key>
    <string>%s</string>
</dict>
</plist>
`

func agentPath(label string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", label+".plist"), nil
}

// installAgent writes a launchd user agent pointing at bin, restarting it on
// crash (KeepAlive) but not after a clean exit - e.g. a deliberate `quit`.
func installAgent(label, bin string) (string, error) {
	abs, err := filepath.Abs(bin)
	if err != nil {
		abs = bin
	}
	logPath, err := config.LogPath()
	if err != nil {
		return "", err
	}
	path, err := agentPath(label)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	content := fmt.Sprintf(plistTemplate, label, abs, logPath, logPath)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func uninstallAgent(label string) (string, error) {
	path, err := agentPath(label)
	if err != nil {
		return "", err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return path, nil
}

// disableAgent unloads a launch agent, best effort. Both the modern and
// legacy syntaxes are tried because which one works depends on the macOS
// version.
func disableAgent(label string) error {
	path, err := agentPath(label)
	if err != nil {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		return nil // never installed
	}
	uid := strconv.Itoa(os.Getuid())
	_ = exec.Command("launchctl", "bootout", "gui/"+uid+"/"+label).Run()
	_ = exec.Command("launchctl", "unload", "-w", path).Run()
	return nil
}

// loadAgent (re)loads a launch agent, so it starts now and again at every
// future login. bootout-ing first makes this safe to call on a plist that
// was already loaded (e.g. re-running setup). Both the modern and legacy
// load syntaxes are tried, since which one works depends on the macOS version.
func loadAgent(label, path string) error {
	uid := strconv.Itoa(os.Getuid())
	_ = exec.Command("launchctl", "bootout", "gui/"+uid+"/"+label).Run()
	if err := exec.Command("launchctl", "bootstrap", "gui/"+uid, path).Run(); err == nil {
		return nil
	}
	return exec.Command("launchctl", "load", "-w", path).Run()
}

// Install writes a launchd user agent pointing at the daemon binary and
// loads it, so it starts now and again on every future login/reboot.
func Install() (string, error) {
	bin, err := spawn.FindDaemon()
	if err != nil {
		return "", err
	}
	path, err := installAgent(daemonLabel, bin)
	if err != nil {
		return "", err
	}
	if err := loadAgent(daemonLabel, path); err != nil {
		return path, fmt.Errorf("wrote %s but could not load it: %w (load manually: launchctl load -w %s)", path, err, path)
	}
	return path, nil
}

// Uninstall removes the agent plist.
func Uninstall() (string, error) { return uninstallAgent(daemonLabel) }

// Disable unloads the launch agent, best effort.
func Disable() error { return disableAgent(daemonLabel) }

// InstallTray writes a launchd user agent pointing at the tray binary.
func InstallTray() (string, error) {
	bin, err := spawn.FindTray()
	if err != nil {
		return "", err
	}
	return installAgent(trayLabel, bin)
}

// UninstallTray removes the tray's agent plist.
func UninstallTray() (string, error) { return uninstallAgent(trayLabel) }

// DisableTray unloads the tray's launch agent, best effort.
func DisableTray() error { return disableAgent(trayLabel) }

// PostInstallTrayHint tells the user how to load the tray agent.
func PostInstallTrayHint() string {
	return "Load it with: launchctl load -w ~/Library/LaunchAgents/" + trayLabel + ".plist"
}
