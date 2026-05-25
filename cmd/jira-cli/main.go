package main

import (
	"os"

	"github.com/fatecannotbealtered/jira-cli/cmd"
)

func main() {
	exitFn(Main())
}

var exitFn = os.Exit

// Main runs the CLI and returns the exit code (testable entry point).
func Main() int {
	return cmd.Run()
}
