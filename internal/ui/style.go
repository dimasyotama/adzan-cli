// Package ui renders the interactive wizard and the live dashboard using
// plain ANSI escapes - no TUI framework, so the binary stays small.
package ui

import (
	"fmt"
	"os"
	"strings"
)

// ANSI escapes used by the dashboard.
const (
	escAltScreen   = "\x1b[?1049h"
	escMainScreen  = "\x1b[?1049l"
	escHideCursor  = "\x1b[?25l"
	escShowCursor  = "\x1b[?25h"
	escHome        = "\x1b[H"
	escClearScreen = "\x1b[2J"
	escClearLine   = "\x1b[K"
)

var colorEnabled = detectColor()

func detectColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	term := os.Getenv("TERM")
	if term == "" || term == "dumb" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func paint(code, s string) string {
	if !colorEnabled {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

// Colour helpers, deliberately few. Green marks the mosque and healthy state,
// amber the next prayer, dim grey the supporting text.
func Green(s string) string  { return paint("38;5;42", s) }
func Amber(s string) string  { return paint("38;5;214", s) }
func Cyan(s string) string   { return paint("38;5;80", s) }
func Red(s string) string    { return paint("38;5;203", s) }
func Dim(s string) string    { return paint("2", s) }
func Bold(s string) string   { return paint("1", s) }
func BoldFG(s string) string { return paint("1;38;5;255", s) }

// silhouette is the mosque drawn above the dashboard. It is intentionally
// ASCII/box-drawing rather than a real image: every terminal renders it the
// same way, with no Sixel or Kitty protocol detection needed.
var silhouette = []string{
	`                    .                    `,
	`                   /|\                   `,
	`                  ( o )                  `,
	`                   \|/                   `,
	`                    |                    `,
	`      ___       _________       ___      `,
	`     |   |    ,'         ',    |   |     `,
	`     | o |   /             \   | o |     `,
	`     |___|  |               |  |___|     `,
	`     |   |  |    _______    |  |   |     `,
	`     |   |  |   /       \   |  |   |     `,
	`     |   |  |  |  .   .  |  |  |   |     `,
	`   __|___|__|__|_________|__|__|___|__   `,
	`  |_____________________________________| `,
}

// Silhouette returns the mosque art, padded to a uniform width and coloured
// if the terminal supports it.
func Silhouette() string {
	width := 0
	for _, line := range silhouette {
		if n := len([]rune(line)); n > width {
			width = n
		}
	}
	var b strings.Builder
	for _, line := range silhouette {
		padded := line + strings.Repeat(" ", width-len([]rune(line)))
		b.WriteString(Green(padded))
		b.WriteByte('\n')
	}
	return b.String()
}

// SilhouetteWidth is the rendered width of the art, used to size the rules
// beneath it so the whole panel lines up.
func SilhouetteWidth() int {
	width := 0
	for _, line := range silhouette {
		if n := len([]rune(line)); n > width {
			width = n
		}
	}
	return width
}

// Rule draws a horizontal separator of the given width.
func Rule(width int) string {
	return Dim(strings.Repeat("-", width))
}

// Field renders an aligned "label  value" row. The label is padded before it
// is coloured, because escape sequences would otherwise count toward width.
func Field(label, value string, width int) string {
	return Dim(fmt.Sprintf("%-*s", width, label)) + " " + value
}
