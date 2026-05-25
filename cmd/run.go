package cmd

import (
	"errors"
	"fmt"
	"os"
)

// Run executes the CLI and returns the process exit code.
func Run() int {
	if err := Execute(); err != nil {
		if errors.Is(err, ErrSilent) {
			return LastExitCode()
		}
		fmt.Fprintln(os.Stderr, "Error:", err)
		return ExitBadArgs
	}
	return ExitOK
}
