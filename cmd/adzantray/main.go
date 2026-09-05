// Command adzantray shows the next prayer in the system menu bar / tray.
// It holds no schedule logic of its own: it only polls the adzand daemon's
// existing status command and displays whatever it says is next, so it
// automatically switches (e.g. Dhuhr -> Asr) the moment the daemon does.
package main

import (
	"errors"
	"fmt"
	"time"

	"fyne.io/systray"

	"github.com/dimasyotama/adzan-cli/internal/ipc"
)

const pollInterval = 20 * time.Second

func main() {
	systray.Run(onReady, func() {})
}

func onReady() {
	systray.SetTitle("adzan")
	mQuit := systray.AddMenuItem("Quit", "Hide this icon (the daemon keeps running)")

	refresh()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-mQuit.ClickedCh:
			systray.Quit()
			return
		case <-ticker.C:
			refresh()
		}
	}
}

func refresh() {
	resp, err := ipc.Send(ipc.CmdStatus)
	if err != nil || resp.Status == nil {
		systray.SetTitle("adzan: daemon not running")
		if err != nil && !errors.Is(err, ipc.ErrNoDaemon) {
			systray.SetTooltip(err.Error())
		} else {
			systray.SetTooltip("start it with `adzan start`")
		}
		return
	}

	st := resp.Status
	label := st.NextName
	if st.Muted {
		label = "\U0001F507 " + label // 🔇
	}
	systray.SetTitle(fmt.Sprintf("%s %s", label, st.NextAt.Local().Format("15:04")))
	systray.SetTooltip(st.Location)
}
