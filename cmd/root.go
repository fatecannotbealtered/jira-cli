package cmd

import (
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"github.com/fatecannotbealtered/jira-cli/internal/api"
	"github.com/fatecannotbealtered/jira-cli/internal/audit"
	"github.com/fatecannotbealtered/jira-cli/internal/config"
	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

// Exit codes for machine-readable error classification.
const (
	ExitOK        = 0
	ExitBadArgs   = 2
	ExitAuth      = 3
	ExitNotFound  = 4
	ExitForbidden = 5
	ExitRateLimit = 6
	ExitNetwork   = 7
)

// ErrSilent indicates the error has been printed; cobra should not print again.
var ErrSilent = errors.New("")

// version is injected by goreleaser ldflags.
var version = "dev"

// jsonMode is the global --json flag.
var jsonMode bool

// forceMode is the global --force flag.
var forceMode bool

// quietMode is the global --quiet flag.
var quietMode bool

// dryRun is the global --dry-run flag.
var dryRun bool

// lastExit tracks the exit code for the current command execution.
var lastExit int

// cmdStartTime records when the current command began (for audit logging).
var cmdStartTime time.Time

// LastExitCode returns the exit code from the last command execution.
func LastExitCode() int { return lastExit }

// setExitCode sets the exit code (only increases severity, never decreases).
func setExitCode(code int) {
	if code > lastExit {
		lastExit = code
	}
}

var rootCmd = &cobra.Command{
	Use:           "jira-cli",
	Short:         "Full-featured Jira Data Center CLI for humans and AI Agents",
	Version:       version,
	SilenceErrors: true,
	SilenceUsage:  true,
	Long: fmt.Sprintf("\n  %s\n  %s",
		output.FormatCyanBold("jira-cli"),
		output.FormatGray("Full Jira Data Center control from your terminal")),
}

func init() {
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
			version = info.Main.Version
		}
	}
	rootCmd.Version = version
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	rootCmd.PersistentFlags().BoolVar(&jsonMode, "json", false, "Output result as JSON")
	rootCmd.PersistentFlags().BoolVar(&forceMode, "force", false, "Skip confirmation prompts")
	rootCmd.PersistentFlags().BoolVar(&quietMode, "quiet", false, "Suppress non-JSON stdout output (for scripts and AI Agents)")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without executing")

	cobra.OnInitialize(func() {
		output.Quiet = quietMode
	})

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		cmdStartTime = time.Now()
		return nil
	}

	rootCmd.PersistentPostRunE = func(cmd *cobra.Command, args []string) error {
		if !isWriteCommand(cmd) {
			return nil
		}
		duration := time.Since(cmdStartTime)
		audit.Log(cmd.CommandPath(), os.Args[1:], lastExit, duration.Milliseconds())
		return nil
	}
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// handleAPIError handles API errors with JSON mode support.
func handleAPIError(err error, jsonMode bool) error {
	var apiErr *api.APIError
	if errors.As(err, &apiErr) {
		msg := apiErr.Error()
		code := output.ErrorCodeFromStatus(apiErr.StatusCode)
		if jsonMode {
			output.PrintErrorJSONWithCode(msg, apiErr.StatusCode, code)
		} else {
			output.Error(msg)
		}
		setExitCode(exitCodeForStatus(apiErr.StatusCode))
		return ErrSilent
	}
	msg := err.Error()
	if jsonMode {
		output.PrintErrorJSONWithCode(msg, 0, output.ErrNetwork)
	} else {
		output.Error(msg)
	}
	setExitCode(ExitNetwork)
	return ErrSilent
}

// exitCodeForStatus maps HTTP status codes to semantic exit codes.
func exitCodeForStatus(status int) int {
	switch {
	case status == 401:
		return ExitAuth
	case status == 403:
		return ExitForbidden
	case status == 404:
		return ExitNotFound
	case status == 429:
		return ExitRateLimit
	case status >= 500:
		return ExitNetwork
	default:
		return ExitBadArgs
	}
}

// confirmAction asks the user to type a specific string to confirm an action.
// Returns true immediately if --force is set.
func confirmAction(prompt, expected string) bool {
	if forceMode {
		return true
	}
	fmt.Printf("%s: ", prompt)
	var input string
	_, err := fmt.Fscan(os.Stdin, &input)
	if err != nil {
		return false
	}
	return input == expected
}

// dryRunOutput outputs a dry-run message and returns true if --dry-run is set.
// If --json is also set, outputs as JSON; otherwise plain text.
func dryRunOutput(action string, detail map[string]any) bool {
	if !dryRun {
		return false
	}
	if jsonMode {
		detail["action"] = action
		detail["dryRun"] = true
		output.PrintJSON(detail)
	} else {
		output.Info("[dry-run] " + action)
	}
	return true
}

// isWriteCommand returns true if the command has the "write" annotation.
func isWriteCommand(cmd *cobra.Command) bool {
	return cmd.Annotations["write"] == "true"
}

// markWrite sets the "write" annotation on a command for audit logging.
func markWrite(cmd *cobra.Command) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations["write"] = "true"
}

// newClient loads config and creates an API client.
func newClient() (*api.Client, *config.Config, error) {
	cfg, err := config.MustLoad()
	if err != nil {
		if jsonMode {
			output.PrintErrorJSONWithCode(err.Error(), 0, output.ErrConfig)
		} else {
			output.Error(err.Error())
		}
		setExitCode(ExitAuth)
		return nil, nil, ErrSilent
	}
	return api.NewClient(cfg), cfg, nil
}

// resolveUsername resolves "me" to the current user's username (DC uses name, not accountId).
func resolveUsername(client *api.Client, assignee string) (string, error) {
	if assignee == "me" {
		myself, err := client.Users.Me()
		if err != nil {
			return "", handleAPIError(err, jsonMode)
		}
		return myself.Name, nil
	}
	return assignee, nil
}
