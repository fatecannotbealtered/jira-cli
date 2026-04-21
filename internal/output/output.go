package output

import (
	"fmt"
	"os"
	"strings"
)

// ANSI color codes
const (
	ansiReset    = "\033[0m"
	ansiBold     = "\033[1m"
	ansiRed      = "\033[31m"
	ansiGreen    = "\033[32m"
	ansiYellow   = "\033[33m"
	ansiBlue     = "\033[34m"
	ansiCyan     = "\033[36m"
	ansiWhite    = "\033[97m"
	ansiGray     = "\033[90m"
	ansiBoldCyan = "\033[1;36m"
)

// isTerminal checks if a file descriptor is a terminal (TTY).
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// noColor is true when NO_COLOR env var is set or stdout is not a TTY.
var noColor = os.Getenv("NO_COLOR") != "" || !isTerminal(os.Stdout)

// Quiet suppresses all non-error output when true.
var Quiet bool

// colorize wraps msg with ANSI color codes when color is enabled.
func colorize(code, msg string) string {
	if noColor {
		return msg
	}
	return code + msg + ansiReset
}

// Success prints a green checkmark message to stdout.
func Success(msg string) {
	if Quiet {
		return
	}
	fmt.Println(colorize(ansiGreen, "✔ "+msg))
}

// Error prints a red cross message to stderr.
func Error(msg string) {
	fmt.Fprintln(os.Stderr, colorize(ansiRed, "✖ "+msg))
}

// Warn prints a yellow warning message to stderr.
func Warn(msg string) {
	fmt.Fprintln(os.Stderr, colorize(ansiYellow, "⚠ "+msg))
}

// Info prints a blue info message to stdout.
func Info(msg string) {
	if Quiet {
		return
	}
	fmt.Println(colorize(ansiBlue, "ℹ "+msg))
}

// Bold prints a bold message to stdout.
func Bold(msg string) {
	if Quiet {
		return
	}
	fmt.Println(colorize(ansiBold, msg))
}

// Gray prints a gray message to stdout.
func Gray(msg string) {
	if Quiet {
		return
	}
	fmt.Println(colorize(ansiGray, msg))
}

// FormatCyan returns a cyan formatted string.
func FormatCyan(s string) string {
	return colorize(ansiCyan, s)
}

// FormatCyanBold returns a cyan bold formatted string.
func FormatCyanBold(s string) string {
	return colorize(ansiBoldCyan, s)
}

// FormatGray returns a gray formatted string.
func FormatGray(s string) string {
	return colorize(ansiGray, s)
}

// FormatGreen returns a green formatted string.
func FormatGreen(s string) string {
	return colorize(ansiGreen, s)
}

// FormatRed returns a red formatted string.
func FormatRed(s string) string {
	return colorize(ansiRed, s)
}

// FormatYellow returns a yellow formatted string.
func FormatYellow(s string) string {
	return colorize(ansiYellow, s)
}

// StatusBadge returns a colored string based on issue status name.
func StatusBadge(status string) string {
	switch strings.ToLower(status) {
	case "to do", "open", "backlog":
		return colorize(ansiWhite, status)
	case "in progress", "in review", "doing":
		return colorize(ansiBlue, status)
	case "done", "closed", "resolved":
		return colorize(ansiGreen, status)
	default:
		return colorize(ansiGray, status)
	}
}

// PriorityBadge returns a colored string based on priority name.
func PriorityBadge(priority string) string {
	switch strings.ToLower(priority) {
	case "highest", "high":
		return colorize(ansiRed, priority)
	case "medium":
		return colorize(ansiYellow, priority)
	case "low", "lowest":
		return colorize(ansiGray, priority)
	default:
		return colorize(ansiGray, priority)
	}
}
