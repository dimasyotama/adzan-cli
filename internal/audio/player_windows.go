//go:build windows

package audio

import (
	"fmt"
	"os/exec"
)

// Windows has no simple MP3-capable CLI player, so drive Windows Media Player
// through PowerShell and block until playback finishes. Killing the PowerShell
// process stops the sound.
func command(path string) ([]string, error) {
	if _, err := exec.LookPath("powershell"); err != nil {
		return nil, ErrNoPlayer
	}
	script := fmt.Sprintf(
		`$p = New-Object -ComObject WMPlayer.OCX; `+
			`$p.URL = %q; `+
			`$p.controls.play(); `+
			`Start-Sleep -Milliseconds 800; `+
			`while ($p.playState -eq 3) { Start-Sleep -Milliseconds 300 }; `+
			`$p.close()`, path)
	return []string{"powershell", "-NoProfile", "-WindowStyle", "Hidden", "-Command", script}, nil
}
