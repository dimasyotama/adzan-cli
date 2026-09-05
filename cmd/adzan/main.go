// Command adzan is the user-facing CLI: first-run setup, the live dashboard,
// and the small set of commands that talk to the background daemon.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/dimasyotama/adzan-cli/internal/assets"
	"github.com/dimasyotama/adzan-cli/internal/audio"
	"github.com/dimasyotama/adzan-cli/internal/config"
	"github.com/dimasyotama/adzan-cli/internal/ipc"
	"github.com/dimasyotama/adzan-cli/internal/prayer"
	"github.com/dimasyotama/adzan-cli/internal/selfmanage"
	"github.com/dimasyotama/adzan-cli/internal/service"
	"github.com/dimasyotama/adzan-cli/internal/spawn"
	"github.com/dimasyotama/adzan-cli/internal/ui"
)

// Version is stamped at build time with -ldflags "-X main.Version=...".
var Version = "dev"

func main() {
	args := os.Args[1:]
	cmd := ""
	if len(args) > 0 {
		cmd = args[0]
		args = args[1:]
	}

	if err := run(cmd, args); err != nil {
		if errors.Is(err, ui.ErrAborted) {
			fmt.Fprintln(os.Stderr, "\nCancelled.")
			os.Exit(130)
		}
		fmt.Fprintf(os.Stderr, "%s %v\n", ui.Red("error:"), err)
		os.Exit(1)
	}
}

func run(cmd string, args []string) error {
	switch cmd {
	case "", "dashboard":
		return cmdDashboard()
	case "setup":
		return cmdSetup()
	case "start":
		return cmdStart()
	case "stop":
		return cmdSimple(ipc.CmdStop)
	case "status":
		return cmdStatus()
	case "mute":
		return cmdSimple(ipc.CmdMute)
	case "unmute":
		return cmdSimple(ipc.CmdUnmute)
	case "test":
		return cmdSimple(ipc.CmdTest)
	case "reload":
		return cmdSimple(ipc.CmdReload)
	case "quit":
		return cmdQuit()
	case "times":
		return cmdTimes()
	case "install":
		return cmdInstallService()
	case "uninstall":
		return cmdUninstallService()
	case "tray":
		return cmdTray(args)
	case "doctor":
		return cmdDoctor()
	case "remove", "uninstall-all":
		return cmdRemove(args)
	case "update", "upgrade":
		return cmdUpdate()
	case "version", "--version", "-v":
		fmt.Printf("adzan %s (%s/%s)\n", Version, runtime.GOOS, runtime.GOARCH)
		return nil
	case "help", "--help", "-h":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func usage() {
	fmt.Print(`
  adzan - prayer times and the adhan, in your terminal

  USAGE
    adzan                 live dashboard (default)
    adzan setup           choose your location and calculation method
    adzan start           start the background daemon
    adzan stop            silence the adhan that is playing right now
    adzan quit            stop the background daemon
    adzan status          one-shot status, no live view
    adzan times           today's prayer times
    adzan mute            stop announcing prayers until unmuted
    adzan unmute          resume announcing prayers
    adzan test            play the adhan now, to check your audio
    adzan reload          re-read the config after editing it by hand
    adzan install         install a systemd user unit / launchd agent
    adzan uninstall       remove it
    adzan tray install    show the next prayer in the menu bar / tray
    adzan tray uninstall  remove it
    adzan doctor          check audio, config and daemon health
    adzan update          update to the latest release
    adzan remove          uninstall adzan completely
    adzan version

`)
}

// ensureConfigured loads config, running the wizard on first use.
func ensureConfigured() (*config.Config, error) {
	cfg, err := config.Load()
	if errors.Is(err, config.ErrNotConfigured) {
		fmt.Println(ui.Dim("  No configuration found - starting setup."))
		if err := cmdSetup(); err != nil {
			return nil, err
		}
		return config.Load()
	}
	return cfg, err
}

// cmdSetup runs the wizard and installs the bundled adhan.
func cmdSetup() error {
	existing, err := config.Load()
	if err != nil && !errors.Is(err, config.ErrNotConfigured) {
		return err
	}

	cfg, err := ui.NewWizard().Run(context.Background(), existing)
	if err != nil {
		return err
	}

	soundPath, err := installDefaultSound()
	if err != nil {
		return err
	}
	if cfg.SoundPath == "" {
		cfg.SoundPath = soundPath
	}

	if err := cfg.Save(); err != nil {
		return err
	}

	path, _ := config.Path()
	fmt.Printf("\n  %s %s\n", ui.Green("OK"), "Saved to "+ui.Dim(path))

	// If the daemon is already up, make it pick up the new location.
	if ipc.Running() {
		if _, err := ipc.Send(ipc.CmdReload); err == nil {
			fmt.Printf("  %s %s\n", ui.Green("OK"), "Running daemon reloaded")
		}
	} else {
		fmt.Printf("\n  Next: %s to run it in the background, then %s\n\n",
			ui.BoldFG("adzan start"), ui.BoldFG("adzan"))
	}

	offerTray()
	return nil
}

// offerTray asks, once, whether to show the next prayer in the menu bar /
// tray. Only asked when the tray binary is actually present - no point
// offering it on a platform build that does not ship one, or before `make
// install` has put it next to the CLI.
func offerTray() {
	if _, err := spawn.FindTray(); err != nil {
		return
	}
	fmt.Println()
	if !ui.Confirm("Show the next prayer in the menu bar / tray?") {
		return
	}
	if err := installTray(); err != nil {
		fmt.Printf("  %s %s\n", ui.Red("!"), err.Error())
	}
}

// installDefaultSound writes the embedded adhan into the sounds directory if
// it is not already there, and returns its path.
func installDefaultSound() (string, error) {
	dir, err := config.SoundsDir()
	if err != nil {
		return "", err
	}
	if err := config.EnsureDirs(); err != nil {
		return "", err
	}
	path := filepath.Join(dir, assets.DefaultAdhanName)
	if fi, err := os.Stat(path); err == nil && fi.Size() == int64(len(assets.DefaultAdhan)) {
		return path, nil
	}
	if err := os.WriteFile(path, assets.DefaultAdhan, 0o644); err != nil {
		return "", fmt.Errorf("could not write the adhan to %s: %w", path, err)
	}
	return path, nil
}

func cmdStart() error {
	if _, err := ensureConfigured(); err != nil {
		return err
	}
	if ipc.Running() {
		fmt.Printf("  %s %s\n", ui.Green("OK"), "The daemon is already running.")
		return nil
	}
	sp := ui.NewSpinner("Starting the daemon...")
	sp.Start()
	err := spawn.Start()
	sp.Stop()
	if err != nil {
		return err
	}
	fmt.Printf("  %s %s\n", ui.Green("OK"), "Daemon started. Run "+ui.BoldFG("adzan")+" for the dashboard.")
	return nil
}

func cmdQuit() error {
	resp, err := ipc.Send(ipc.CmdQuit)
	if errors.Is(err, ipc.ErrNoDaemon) {
		fmt.Printf("  %s\n", ui.Dim("The daemon is not running."))
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Printf("  %s %s\n", ui.Green("OK"), resp.Message)
	return nil
}

// cmdSimple forwards a one-word command and prints the daemon's reply.
func cmdSimple(command string) error {
	resp, err := ipc.Send(command)
	if errors.Is(err, ipc.ErrNoDaemon) {
		return fmt.Errorf("the daemon is not running - start it with `adzan start`")
	}
	if err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	fmt.Printf("  %s %s\n", ui.Green("OK"), resp.Message)
	return nil
}

func cmdStatus() error {
	resp, err := ipc.Send(ipc.CmdStatus)
	if errors.Is(err, ipc.ErrNoDaemon) {
		return fmt.Errorf("the daemon is not running - start it with `adzan start`")
	}
	if err != nil {
		return err
	}
	if resp.Status == nil {
		return errors.New("the daemon returned no status")
	}
	ui.StatusOnce(resp.Status)
	return nil
}

func cmdTimes() error {
	return cmdStatus()
}

func cmdDashboard() error {
	if _, err := ensureConfigured(); err != nil {
		return err
	}
	if !ipc.Running() {
		sp := ui.NewSpinner("Daemon not running - starting it...")
		sp.Start()
		err := spawn.Start()
		sp.Stop()
		if err != nil {
			return err
		}
	}
	return ui.Dashboard()
}

func cmdInstallService() error {
	if _, err := ensureConfigured(); err != nil {
		return err
	}
	path, err := service.Install()
	if err != nil {
		return err
	}
	fmt.Printf("  %s %s\n", ui.Green("OK"), "Service installed at "+ui.Dim(path))
	fmt.Printf("  %s\n", ui.Dim(service.PostInstallHint()))
	return nil
}

func cmdUninstallService() error {
	path, err := service.Uninstall()
	if err != nil {
		return err
	}
	fmt.Printf("  %s %s\n", ui.Green("OK"), "Service removed: "+ui.Dim(path))
	return nil
}

func cmdTray(args []string) error {
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "install":
		return installTray()
	case "uninstall":
		path, err := service.UninstallTray()
		if err != nil {
			return err
		}
		_ = service.DisableTray()
		fmt.Printf("  %s %s\n", ui.Green("OK"), "Tray removed: "+ui.Dim(path))
		return nil
	default:
		return fmt.Errorf("usage: adzan tray install|uninstall")
	}
}

// installTray writes the login-item file and launches the icon right away,
// so the user sees it working instead of having to log out and back in.
func installTray() error {
	if _, err := ensureConfigured(); err != nil {
		return err
	}
	path, err := service.InstallTray()
	if err != nil {
		return err
	}
	fmt.Printf("  %s %s\n", ui.Green("OK"), "Tray installed at "+ui.Dim(path))

	if err := spawn.StartTray(); err != nil {
		fmt.Printf("  %s %s\n", ui.Red("!"), "Could not start it now: "+err.Error())
		fmt.Printf("  %s\n", ui.Dim(service.PostInstallTrayHint()))
		return nil
	}
	fmt.Printf("  %s %s\n", ui.Green("OK"), "Tray started")
	return nil
}

// cmdDoctor reports on the things that most often go wrong.
func cmdDoctor() error {
	fmt.Println()

	cfgPath, _ := config.Path()
	cfg, err := config.Load()
	switch {
	case errors.Is(err, config.ErrNotConfigured):
		fail("config", "not set up yet - run `adzan setup`")
	case err != nil:
		fail("config", err.Error())
	default:
		ok("config", cfgPath)
		ok("location", fmt.Sprintf("%s (%.4f, %.4f)", cfg.Location.Label, cfg.Location.Latitude, cfg.Location.Longitude))
		ok("method", prayer.MethodName(cfg.Method))
		fmt.Printf("       %s\n", ui.Dim("verify the coordinates on a map if times look wrong - "+
			"a wrong location is the most common cause"))
	}

	if player, perr := audio.Available(); perr != nil {
		fail("audio", "no supported player found - install ffmpeg (ffplay), mpv or mpg123")
	} else {
		ok("audio", "using "+player)
	}

	if cfg != nil {
		if _, serr := os.Stat(cfg.SoundPath); serr != nil {
			fail("sound", "missing at "+cfg.SoundPath+" - re-run `adzan setup`")
		} else {
			ok("sound", cfg.SoundPath)
		}
	}

	if ipc.Running() {
		ok("daemon", "running")
	} else {
		fail("daemon", "not running - start it with `adzan start`")
	}

	logPath, _ := config.LogPath()
	fmt.Printf("  %s %s\n", ui.Dim("log     "), ui.Dim(logPath))
	fmt.Println()
	return nil
}

// cmdRemove uninstalls adzan: daemon, service file, binaries and (unless
// --keep-config) the configuration directory.
func cmdRemove(args []string) error {
	assumeYes := false
	keepConfig := false
	for _, a := range args {
		switch a {
		case "-y", "--yes":
			assumeYes = true
		case "--keep-config":
			keepConfig = true
		default:
			return fmt.Errorf("unknown option %q (accepts --yes, --keep-config)", a)
		}
	}

	plan, err := selfmanage.BuildPlan()
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("  " + ui.BoldFG("This will remove:"))
	if plan.DaemonUp {
		fmt.Println("    " + ui.Dim("the running daemon (it will be stopped)"))
	}
	for _, b := range plan.Binaries {
		fmt.Println("    " + b)
	}
	if plan.ServiceFile != "" {
		fmt.Println("    " + plan.ServiceFile)
	}
	if plan.TrayServiceFile != "" {
		fmt.Println("    " + plan.TrayServiceFile)
	}
	if plan.ConfigDir != "" {
		if keepConfig {
			fmt.Println("    " + ui.Dim(plan.ConfigDir+"  (kept, --keep-config)"))
		} else {
			fmt.Println("    " + plan.ConfigDir + ui.Dim("  (location, cached times and the adhan)"))
		}
	}
	fmt.Println()

	if !assumeYes && !ui.Confirm("Remove adzan?") {
		fmt.Println("  " + ui.Dim("Cancelled - nothing was removed."))
		return nil
	}

	for _, line := range selfmanage.Remove(plan, keepConfig) {
		fmt.Printf("  %s %s\n", ui.Green("OK"), line)
	}

	fmt.Printf("\n  %s\n", ui.Dim("adzan is gone. You may want to drop its directory from your PATH."))
	return nil
}

// cmdUpdate re-runs the install script to fetch the newest release.
func cmdUpdate() error {
	if runtime.GOOS == "windows" {
		fmt.Printf("\n  %s\n\n", ui.Dim(selfmanage.ManualUpdateHint()))
		return nil
	}
	fmt.Printf("  %s\n\n", ui.Dim("Fetching the latest release..."))
	if err := selfmanage.Update(); err != nil {
		return err
	}
	return nil
}

func ok(label, detail string) {
	fmt.Printf("  %s %-8s %s\n", ui.Green("OK  "), label, ui.Dim(detail))
}

func fail(label, detail string) {
	fmt.Printf("  %s %-8s %s\n", ui.Red("FAIL"), label, detail)
}
