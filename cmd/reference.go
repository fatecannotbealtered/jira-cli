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
	Name    string `json:"name"`
	Type    string `json:"type"`
	Default string `json:"default"`
	Usage   string `json:"usage"`
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
			Name:    f.Name,
			Type:    f.Value.Type(),
			Default: formatFlagDefault(f.DefValue),
			Usage:   f.Usage,
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
				Name:    f.Name,
				Type:    f.Value.Type(),
				Default: formatFlagDefault(f.DefValue),
				Usage:   f.Usage,
			})
		})
	}
	return flags
}

type commandReference struct {
	CommandPath      string             `json:"commandPath"`
	Use              string             `json:"use"`
	Short            string             `json:"short,omitempty"`
	Flags            []referenceFlag    `json:"flags,omitempty"`
	SupportedFormats []string           `json:"supportedFormats"`
	Write            bool               `json:"write,omitempty"`
	Subcommands      []commandReference `json:"subcommands,omitempty"`
}

type referenceDocument struct {
	Version  string             `json:"version"`
	Root     commandReference   `json:"root"`
	Commands []commandReference `json:"commands"`
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
		Version:  root.Version,
		Root:     rootRef,
		Commands: commands,
	}
}

func buildCommandReference(cmd *cobra.Command, prefix string, commands *[]commandReference) commandReference {
	ref := commandReference{
		CommandPath:      strings.TrimSpace(prefix + cmd.Name()),
		Use:              cmd.Use,
		Short:            cmd.Short,
		Flags:            collectReferenceFlags(cmd.LocalFlags(), cmd.PersistentFlags(), cmd.Parent() != nil),
		SupportedFormats: supportedFormatsForCommand(cmd),
		Write:            isWriteCommand(cmd),
	}
	children := cmd.Commands()
	sort.Slice(children, func(i, j int) bool {
		return children[i].Name() < children[j].Name()
	})
	for _, child := range children {
		if child.Hidden || !child.IsAvailableCommand() {
			continue
		}
		childRef := buildCommandReference(child, ref.CommandPath+" ", commands)
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
