//go:build linux

package audio

import "os/exec"

// candidates are tried in order; the first one present in PATH wins.
// All of these can decode MP3 and exit on their own when the file ends.
var candidates = [][]string{
	{"ffplay", "-nodisp", "-autoexit", "-loglevel", "quiet"},
	{"mpv", "--no-video", "--really-quiet"},
	{"mpg123", "-q"},
	{"cvlc", "--intf", "dummy", "--play-and-exit"},
	{"paplay"},
	{"aplay", "-q"},
}

func command(path string) ([]string, error) {
	for _, c := range candidates {
		if _, err := exec.LookPath(c[0]); err == nil {
			return append(append([]string{}, c...), path), nil
		}
	}
	return nil, ErrNoPlayer
}
