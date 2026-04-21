package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/fatecannotbealtered/jira-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		if errors.Is(err, cmd.ErrSilent) {
			os.Exit(cmd.LastExitCode())
		}
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(cmd.ExitBadArgs)
	}
}
