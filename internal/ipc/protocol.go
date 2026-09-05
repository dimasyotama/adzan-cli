// Package ipc carries commands between the `adzan` CLI and the `adzand`
// daemon over a local socket. One JSON object per line, request/response.
package ipc

import "time"

// Command names understood by the daemon.
const (
	CmdStatus = "status" // report location, next prayer, playback state
	CmdStop   = "stop"   // silence the adhan that is playing right now
	CmdTest   = "test"   // play the adhan immediately, to check audio works
	CmdMute   = "mute"   // suppress future adhans until unmuted
	CmdUnmute = "unmute"
	CmdReload = "reload" // re-read config.json and refresh the schedule
	CmdQuit   = "quit"   // shut the daemon down
)

// Request is one command from the CLI.
type Request struct {
	Command string `json:"command"`
}

// Response is the daemon's reply.
type Response struct {
	OK      bool    `json:"ok"`
	Message string  `json:"message,omitempty"`
	Error   string  `json:"error,omitempty"`
	Status  *Status `json:"status,omitempty"`
}

// Status is the snapshot the dashboard renders.
type Status struct {
	Location  string            `json:"location"`
	Coords    string            `json:"coords"`
	Method    string            `json:"method"`
	Timezone  string            `json:"timezone"`
	SoundPath string            `json:"sound_path"`
	Muted     bool              `json:"muted"`
	Playing   bool              `json:"playing"`
	NextName  string            `json:"next_name"`
	NextAt    time.Time         `json:"next_at"`
	Today     map[string]string `json:"today"`
	StartedAt time.Time         `json:"started_at"`
}
