package cmd

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"strings"
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

const (
	outputFormatJSON = "json"
	outputFormatText = "text"
	outputFormatRaw  = "raw"
)

// outputFormat is the global --format value.
var outputFormat = outputFormatJSON

// jsonCompatMode is the compatibility --json flag.
var jsonCompatMode bool

// jsonMode reports whether command success output should use the machine-readable branch.
var jsonMode = true

// compactMode is the global --compact flag.
var compactMode bool

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

// execCmd tracks the leaf command during Execute for post-run audit logging.
var execCmd *cobra.Command

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

	rootCmd.PersistentFlags().StringVar(&outputFormat, "format", outputFormatJSON, "Output format: json, text, or raw")
	rootCmd.PersistentFlags().BoolVar(&jsonCompatMode, "json", false, "Compatibility alias for --format json")
	rootCmd.PersistentFlags().BoolVar(&compactMode, "compact", false, "Emit compact JSON (only with --format json)")
	rootCmd.PersistentFlags().BoolVar(&forceMode, "force", false, "Skip confirmation prompts")
	rootCmd.PersistentFlags().BoolVar(&quietMode, "quiet", false, "Suppress auxiliary text output (does not suppress json/raw results)")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without executing")

	cobra.OnInitialize(func() {
		output.Quiet = false
	})

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		lastExit = 0
		cmdStartTime = time.Now()
		execCmd = cmd
		if err := configureOutputMode(cmd); err != nil {
			return err
		}
		return nil
	}
}

func configureOutputMode(cmd *cobra.Command) error {
	formatChanged := persistentFlagChanged(cmd, "format")
	jsonChanged := persistentFlagChanged(cmd, "json")
	effectiveFormat := strings.ToLower(strings.TrimSpace(outputFormat))
	if effectiveFormat == "" {
		effectiveFormat = outputFormatJSON
	}
	if jsonChanged && formatChanged && effectiveFormat != outputFormatJSON {
		return formatArgumentError(effectiveFormat, "--json cannot be used with --format "+effectiveFormat)
	}
	if jsonChanged && !formatChanged {
		effectiveFormat = outputFormatJSON
	}
	switch effectiveFormat {
	case outputFormatJSON, outputFormatText, outputFormatRaw:
	default:
		return formatArgumentError(outputFormatJSON, "--format must be one of: json, text, raw")
	}
	if legacyRawRequested(cmd) {
		if formatChanged && effectiveFormat == outputFormatText {
			return formatArgumentError(effectiveFormat, "--raw cannot be used with --format text")
		}
		effectiveFormat = outputFormatRaw
	}
	if compactMode && effectiveFormat != outputFormatJSON {
		return formatArgumentError(effectiveFormat, "--compact can only be used with --format json")
	}
	if fieldsFlagChanged(cmd) && effectiveFormat != outputFormatJSON {
		return formatArgumentError(effectiveFormat, "--fields can only be used with --format json")
	}
	if effectiveFormat == outputFormatRaw && !supportsRawFormat(cmd) {
		return formatArgumentError(effectiveFormat, cmd.CommandPath()+" does not support --format raw")
	}

	outputFormat = effectiveFormat
	jsonMode = effectiveFormat != outputFormatText
	output.CompactJSON = compactMode && effectiveFormat == outputFormatJSON
	output.ErrorJSON = effectiveFormat != outputFormatText
	output.Quiet = effectiveFormat != outputFormatText || quietMode
	return nil
}

func persistentFlagChanged(cmd *cobra.Command, name string) bool {
	f := cmd.Root().PersistentFlags().Lookup(name)
	return f != nil && f.Changed
}

func fieldsFlagChanged(cmd *cobra.Command) bool {
	f := cmd.Flags().Lookup("fields")
	return f != nil && f.Changed
}

func legacyRawRequested(cmd *cobra.Command) bool {
	f := cmd.Flags().Lookup("raw")
	if f == nil || !f.Changed {
		return false
	}
	v, err := cmd.Flags().GetBool("raw")
	return err == nil && v
}

func formatArgumentError(format, msg string) error {
	if format == outputFormatText {
		output.ErrorJSON = false
		output.Error(msg)
	} else {
		output.CompactJSON = compactMode
		output.PrintErrorJSONWithCode(msg, 0, output.ErrValidation)
	}
	return SilentErr(ExitBadArgs)
}

func rawOutputRequested(cmd *cobra.Command) bool {
	if outputFormat == outputFormatRaw {
		return true
	}
	return legacyRawRequested(cmd)
}

// Execute runs the root command.
func Execute() error {
	lastExit = 0
	execCmd = nil
	err := rootCmd.Execute()
	if err != nil && !errors.Is(err, ErrSilent) {
		err = handleCommandError(err)
	}
	if execCmd != nil && isWriteCommand(execCmd) {
		duration := time.Since(cmdStartTime)
		audit.Log(execCmd.CommandPath(), os.Args[1:], lastExit, duration.Milliseconds())
	}
	return err
}

func handleCommandError(err error) error {
	if formatErr := validateGlobalOutputFlagsForError(); formatErr != nil {
		return formatErr
	}
	if errorOutputFormat() == outputFormatText {
		output.ErrorJSON = false
		output.Error(err.Error())
	} else {
		output.CompactJSON = compactMode
		output.PrintErrorJSONWithCode(err.Error(), 0, output.ErrValidation)
	}
	setExitCode(ExitBadArgs)
	return ErrSilent
}

func validateGlobalOutputFlagsForError() error {
	formatChanged := persistentFlagChanged(rootCmd, "format")
	jsonChanged := persistentFlagChanged(rootCmd, "json")
	effectiveFormat := strings.ToLower(strings.TrimSpace(outputFormat))
	if effectiveFormat == "" {
		effectiveFormat = outputFormatJSON
	}
	if jsonChanged && formatChanged && effectiveFormat != outputFormatJSON {
		return formatArgumentError(effectiveFormat, "--json cannot be used with --format "+effectiveFormat)
	}
	switch effectiveFormat {
	case outputFormatJSON, outputFormatText, outputFormatRaw:
	default:
		return formatArgumentError(outputFormatJSON, "--format must be one of: json, text, raw")
	}
	if compactMode && effectiveFormat != outputFormatJSON {
		return formatArgumentError(effectiveFormat, "--compact can only be used with --format json")
	}
	return nil
}

func errorOutputFormat() string {
	format := strings.ToLower(strings.TrimSpace(outputFormat))
	if format == "" {
		format = outputFormatJSON
	}
	if format == outputFormatText {
		return outputFormatText
	}
	return outputFormatJSON
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
// Uses constant-time comparison to prevent timing attacks.
func confirmAction(prompt, expected string) bool {
	if forceMode {
		return true
	}
	if jsonMode {
		fmt.Fprintf(os.Stderr, "%s: ", prompt)
	} else {
		fmt.Printf("%s: ", prompt)
	}
	var input string
	_, err := fmt.Fscan(os.Stdin, &input)
	if err != nil {
		return false
	}
	// Constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare([]byte(input), []byte(expected)) == 1 {
		return true
	}
	return false
}

// dryRunOutput outputs a dry-run message and returns true if --dry-run is set.
// Machine-readable formats output JSON; text format outputs a human-readable note.
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

// markRawFormat marks a command as supporting --format raw.
func markRawFormat(cmd *cobra.Command) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations["format.raw"] = "true"
}

func supportsRawFormat(cmd *cobra.Command) bool {
	return cmd.Annotations["format.raw"] == "true"
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
