//go:build !linux && !darwin

package service

import "errors"

// errUnsupported keeps Windows building today; Task Scheduler support is the
// natural place to add it when Windows becomes a first-class target.
var errUnsupported = errors.New("service installation is not supported on this platform yet - use `adzan start`, or add a Task Scheduler entry manually")

func Install() (string, error) { return "", errUnsupported }

func Uninstall() (string, error) { return "", errUnsupported }

func PostInstallHint() string { return "" }

// Disable has nothing to turn off on platforms without service support.
func Disable() error { return nil }

func InstallTray() (string, error) { return "", errUnsupported }

func UninstallTray() (string, error) { return "", errUnsupported }

func PostInstallTrayHint() string { return "" }

func DisableTray() error { return nil }
