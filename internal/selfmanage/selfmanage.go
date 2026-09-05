// Package selfmanage implements `adzan remove` and `adzan update`: taking the
// tool off a machine, or pulling a newer build, without the user having to
// remember where anything was put.
package selfmanage

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dimasyotama/adzan-cli/internal/config"
	"github.com/dimasyotama/adzan-cli/internal/ipc"
	"github.com/dimasyotama/adzan-cli/internal/service"
	"github.com/dimasyotama/adzan-cli/internal/spawn"
)

// InstallScriptURL is the one-liner installer, reused by `adzan update`.
const InstallScriptURL = "https://raw.githubusercontent.com/dimasyotama/adzan-cli/main/install.sh"

// Plan is what a removal would touch, gathered before anything is deleted so
// the user can see the full list and say no.
type Plan struct {
	Binaries        []string
	ServiceFile     string
	TrayServiceFile string
	ConfigDir       string
	DaemonUp        bool
}

// BuildPlan inspects the system without changing it.
func BuildPlan() (*Plan, error) {
	p := &Plan{DaemonUp: ipc.Running()}

	if self, err := os.Executable(); err == nil {
		if resolved, rerr := filepath.EvalSymlinks(self); rerr == nil {
			self = resolved
		}
		p.Binaries = append(p.Binaries, self)

		daemon := filepath.Join(filepath.Dir(self), spawn.DaemonName())
		if _, err := os.Stat(daemon); err == nil {
			p.Binaries = append(p.Binaries, daemon)
		}

		tray := filepath.Join(filepath.Dir(self), spawn.TrayName())
		if _, err := os.Stat(tray); err == nil {
			p.Binaries = append(p.Binaries, tray)
		}
	}

	if dir, err := config.Dir(); err == nil {
		if _, serr := os.Stat(dir); serr == nil {
			p.ConfigDir = dir
		}
	}

	p.ServiceFile = servicePath("adzan.service", "com.github.dimasyotama.adzan.plist")
	p.TrayServiceFile = servicePath("adzan-tray.service", "com.github.dimasyotama.adzan.tray.plist")
	return p, nil
}

// servicePath reports an installed service file, or "" if there is none.
func servicePath(systemdName, launchdName string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	var candidate string
	switch runtime.GOOS {
	case "linux":
		candidate = filepath.Join(home, ".config", "systemd", "user", systemdName)
	case "darwin":
		candidate = filepath.Join(home, "Library", "LaunchAgents", launchdName)
	default:
		return ""
	}
	if _, err := os.Stat(candidate); err != nil {
		return ""
	}
	return candidate
}

// Remove tears down the installation. keepConfig leaves the config directory
// (and therefore the user's location and cached times) in place.
func Remove(p *Plan, keepConfig bool) []string {
	var log []string

	if p.DaemonUp {
		if _, err := ipc.Send(ipc.CmdQuit); err == nil {
			log = append(log, "stopped the running daemon")
		}
	}

	if p.ServiceFile != "" {
		_ = service.Disable()
		if _, err := service.Uninstall(); err == nil {
			log = append(log, "removed "+p.ServiceFile)
		} else {
			log = append(log, "could not remove "+p.ServiceFile+": "+err.Error())
		}
	}

	if p.TrayServiceFile != "" {
		_ = service.DisableTray()
		if _, err := service.UninstallTray(); err == nil {
			log = append(log, "removed "+p.TrayServiceFile)
		} else {
			log = append(log, "could not remove "+p.TrayServiceFile+": "+err.Error())
		}
	}

	if !keepConfig && p.ConfigDir != "" {
		if err := os.RemoveAll(p.ConfigDir); err == nil {
			log = append(log, "removed "+p.ConfigDir)
		} else {
			log = append(log, "could not remove "+p.ConfigDir+": "+err.Error())
		}
	}

	// Binaries last: on Unix a running executable can be unlinked, but once
	// it is gone there is nothing left to do anyway.
	for _, bin := range p.Binaries {
		if err := os.Remove(bin); err == nil {
			log = append(log, "removed "+bin)
		} else if runtime.GOOS == "windows" {
			log = append(log, "could not remove "+bin+
				" (Windows locks running executables - delete it manually)")
		} else {
			log = append(log, "could not remove "+bin+": "+err.Error())
		}
	}

	return log
}

// ErrNoUpdater means we cannot self-update on this platform.
var ErrNoUpdater = errors.New("automatic update is not supported on Windows")

// Update re-runs the install script, which fetches the newest release and
// overwrites the binaries in place.
func Update() error {
	if runtime.GOOS == "windows" {
		return ErrNoUpdater
	}

	fetcher, err := findFetcher()
	if err != nil {
		return err
	}

	// Pipe the script into sh, exactly as the documented one-liner does.
	var script string
	if fetcher == "curl" {
		script = fmt.Sprintf("curl -fsSL %s | sh", InstallScriptURL)
	} else {
		script = fmt.Sprintf("wget -qO- %s | sh", InstallScriptURL)
	}

	cmd := exec.Command("sh", "-c", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func findFetcher() (string, error) {
	for _, name := range []string{"curl", "wget"} {
		if _, err := exec.LookPath(name); err == nil {
			return name, nil
		}
	}
	return "", errors.New("neither curl nor wget is installed, so adzan cannot download an update")
}

// ManualUpdateHint is what Windows users get instead of an automatic update.
func ManualUpdateHint() string {
	return strings.Join([]string{
		"Update manually with:",
		"  git pull",
		`  go build -trimpath -ldflags "-s -w" -o bin\adzan.exe  .\cmd\adzan`,
		`  go build -trimpath -ldflags "-s -w" -o bin\adzand.exe .\cmd\adzand`,
	}, "\n  ")
}
