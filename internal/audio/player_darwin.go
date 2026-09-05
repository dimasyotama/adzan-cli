//go:build darwin

package audio

import "os/exec"

func command(path string) ([]string, error) {
	// afplay ships with macOS and handles MP3 natively.
	if _, err := exec.LookPath("afplay"); err == nil {
		return []string{"afplay", path}, nil
	}
	for _, name := range []string{"ffplay", "mpv", "mpg123"} {
		if _, err := exec.LookPath(name); err == nil {
			switch name {
			case "ffplay":
				return []string{"ffplay", "-nodisp", "-autoexit", "-loglevel", "quiet", path}, nil
			case "mpv":
				return []string{"mpv", "--no-video", "--really-quiet", path}, nil
			default:
				return []string{"mpg123", "-q", path}, nil
			}
		}
	}
	return nil, ErrNoPlayer
}
