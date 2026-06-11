package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type referenceFlag struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Default  string `json:"default"`
	Usage    string `json:"usage"`
	Required bool   `json:"required"`
}

func formatFlagDefault(def string) string {
	if def == "" || def == "[]" {
		return "-"
	}
	return def
}

func collectReferenceFlags(local, persistent *pflag.FlagSet, inheritPersistent bool) []referenceFlag {
	var flags []referenceFlag
	local.VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		flags = append(flags, referenceFlag{
			Name:     f.Name,
			Type:     f.Value.Type(),
			Default:  formatFlagDefault(f.DefValue),
			Usage:    f.Usage,
			Required: strings.Contains(strings.ToLower(f.Usage), "required"),
		})
	})
	if inheritPersistent {
		persistent.VisitAll(func(f *pflag.Flag) {
			if f.Hidden {
				return
			}
			if local.Lookup(f.Name) != nil {
				return
			}
			flags = append(flags, referenceFlag{
				Name:     f.Name,
				Type:     f.Value.Type(),
				Default:  formatFlagDefault(f.DefValue),
				Usage:    f.Usage,
				Required: strings.Contains(strings.ToLower(f.Usage), "required"),
			})
		})
	}
	return flags
}

type commandReference struct {
	Path             string             `json:"path"`
	Use              string             `json:"use"`
	Short            string             `json:"short,omitempty"`
	Type             string             `json:"type"`
	PermissionTier   string             `json:"permission_tier"`
	BlastRadius      string             `json:"blast_radius,omitempty"`
	Flags            []referenceFlag    `json:"flags,omitempty"`
	SupportedFormats []string           `json:"supported_formats"`
	Write            bool               `json:"write,omitempty"`
	OutputSchema     map[string]any     `json:"output_schema,omitempty"`
	Subcommands      []commandReference `json:"subcommands,omitempty"`
}

type referenceDocument struct {
	Tool             string             `json:"tool"`
	Version          string             `json:"version"`
	SecurityTier     string             `json:"security_tier"`
	ReleaseReadiness releaseReadiness   `json:"release_readiness"`
	Root             commandReference   `json:"root"`
	Commands         []commandReference `json:"commands"`
	ExitCodes        map[string]string  `json:"exit_codes"`
	ErrorCodes       map[string]string  `json:"error_codes"`
}

var referenceCmd = &cobra.Command{
	Use:   "reference",
	Short: "Print all commands and flags in a structured format",
	Long:  "Outputs every command, subcommand, and flag in a machine-parseable format. Designed for AI Agents and script integration.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if jsonMode {
			output.PrintJSON(buildReferenceDocument(rootCmd))
			return nil
		}
		printReference(cmd, rootCmd)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(referenceCmd)
}

func printReference(cmd *cobra.Command, root *cobra.Command) {
	var lines []string
	lines = append(lines, "# jira-cli Command Reference")
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Version: %s", root.Version))
	lines = append(lines, "")

	walkCommands(root, &lines, "")

	for _, line := range lines {
		cmd.Println(line)
	}
}

func buildReferenceDocument(root *cobra.Command) referenceDocument {
	var commands []commandReference
	rootRef := buildCommandReference(root, "", &commands)
	return referenceDocument{
		Tool:             "jira-cli",
		Version:          root.Version,
		SecurityTier:     "T1",
		ReleaseReadiness: buildReleaseReadiness(),
		Root:             rootRef,
		Commands:         commands,
		ExitCodes:        referenceExitCodes(),
		ErrorCodes:       referenceErrorCodes(),
	}
}

func buildCommandReference(cmd *cobra.Command, prefix string, commands *[]commandReference) commandReference {
	ref := commandReference{
		Path:             strings.TrimSpace(prefix + cmd.Name()),
		Use:              cmd.Use,
		Short:            cmd.Short,
		Type:             commandType(cmd),
		PermissionTier:   permissionTier(cmd),
		BlastRadius:      blastRadius(cmd),
		Flags:            collectReferenceFlags(cmd.LocalFlags(), cmd.PersistentFlags(), cmd.Parent() != nil),
		SupportedFormats: supportedFormatsForCommand(cmd),
		Write:            isWriteCommand(cmd),
		OutputSchema:     outputSchemaForCommand(cmd),
	}
	children := cmd.Commands()
	sort.Slice(children, func(i, j int) bool {
		return children[i].Name() < children[j].Name()
	})
	for _, child := range children {
		if child.Hidden || !child.IsAvailableCommand() {
			continue
		}
		childRef := buildCommandReference(child, ref.Path+" ", commands)
		ref.Subcommands = append(ref.Subcommands, childRef)
	}
	if cmd.Runnable() {
		*commands = append(*commands, ref)
	}
	return ref
}

func supportedFormatsForCommand(cmd *cobra.Command) []string {
	formats := []string{outputFormatJSON, outputFormatText}
	if supportsRawFormat(cmd) {
		formats = append(formats, outputFormatRaw)
	}
	return formats
}

func commandType(cmd *cobra.Command) string {
	if isWriteCommand(cmd) {
		return "write"
	}
	return "read"
}

func permissionTier(cmd *cobra.Command) string {
	if !isWriteCommand(cmd) {
		return "read"
	}
	switch cmd.CommandPath() {
	case "jira-cli issue delete", "jira-cli issue bulk-transition", "jira-cli sprint close", "jira-cli filter delete", "jira-cli update":
		return "write-dangerous"
	default:
		return "write"
	}
}

func blastRadius(cmd *cobra.Command) string {
	if !isWriteCommand(cmd) {
		return "reads Jira data visible to the configured account"
	}
	switch cmd.CommandPath() {
	case "jira-cli issue delete":
		return "deletes one Jira issue visible and deletable by the configured account"
	case "jira-cli issue bulk-transition":
		return "transitions multiple Jira issues selected by explicit keys or JQL"
	case "jira-cli sprint close":
		return "closes one Jira sprint"
	case "jira-cli filter delete":
		return "deletes one saved Jira filter visible and deletable by the configured account"
	case "jira-cli update":
		return "replaces the local jira-cli executable"
	default:
		return "modifies Jira state within the configured account permissions"
	}
}

func outputSchemaForCommand(cmd *cobra.Command) map[string]any {
	schema := map[string]any{
		"format": "json success/failure envelope",
		"data":   "command-specific payload",
	}
	if supportsRawFormat(cmd) {
		schema["raw"] = "raw Jira API JSON without the envelope when --format raw is selected"
	}
	return schema
}

func referenceExitCodes() map[string]string {
	return map[string]string{
		"0": "success",
		"1": "generic error",
		"2": "argument or validation error",
		"3": "resource not found",
		"4": "auth, permission, or config failure",
		"5": "confirmation token required",
		"6": "conflict or invalid confirmation token",
		"7": "retryable transient error",
		"8": "timeout",
	}
}

func referenceErrorCodes() map[string]string {
	return map[string]string{
		string(output.ErrConfig):          "configuration is missing or invalid",
		string(output.ErrAuth):            "authentication failed",
		string(output.ErrForbidden):       "permission denied",
		string(output.ErrNotFound):        "resource not found",
		string(output.ErrRateLimit):       "rate limited",
		string(output.ErrServer):          "Jira server error",
		string(output.ErrValidation):      "arguments or request are invalid",
		string(output.ErrNetwork):         "network failure",
		string(output.ErrConfirmRequired): "write requires a dry-run confirmation token",
		string(output.ErrConflict):        "confirmation token expired or no longer matches",
		string(output.ErrTimeout):         "request timed out",
		string(output.ErrUnknown):         "unclassified error",
	}
}

func walkCommands(cmd *cobra.Command, lines *[]string, prefix string) {
	if cmd.Hidden {
		return
	}

	name := prefix + cmd.Use
	*lines = append(*lines, "## "+name)
	*lines = append(*lines, "")
	if cmd.Short != "" {
		*lines = append(*lines, cmd.Short)
		*lines = append(*lines, "")
	}

	flags := collectReferenceFlags(cmd.LocalFlags(), cmd.PersistentFlags(), cmd.Parent() != nil)

	if len(flags) > 0 {
		*lines = append(*lines, "### Flags")
		*lines = append(*lines, "")
		*lines = append(*lines, "| Flag | Type | Default | Description |")
		*lines = append(*lines, "|------|------|---------|-------------|")
		for _, f := range flags {
			*lines = append(*lines, fmt.Sprintf("| `--%s` | %s | %s | %s |", f.Name, f.Type, f.Default, f.Usage))
		}
		*lines = append(*lines, "")
	}

	children := cmd.Commands()
	if len(children) > 0 {
		sort.Slice(children, func(i, j int) bool {
			return children[i].Name() < children[j].Name()
		})
		for _, child := range children {
			if !child.Hidden && child.IsAvailableCommand() {
				walkCommands(child, lines, prefix+cmd.Name()+" ")
			}
		}
	}
}
