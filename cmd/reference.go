package cmd

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var referenceCmd = &cobra.Command{
	Use:   "reference",
	Short: "Print all commands and flags in a structured format",
	Long:  "Outputs every command, subcommand, and flag in a machine-parseable format. Designed for AI Agents and script integration.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
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

	// Collect flags
	local := cmd.LocalFlags()
	persistent := cmd.PersistentFlags()

	type flagInfo struct {
		Name    string
		Type    string
		Default string
		Usage   string
	}
	var flags []flagInfo

	local.VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		def := f.DefValue
		if def == "" || def == "[]" {
			def = "-"
		}
		flags = append(flags, flagInfo{f.Name, f.Value.Type(), def, f.Usage})
	})

	if cmd.Parent() != nil {
		persistent.VisitAll(func(f *pflag.Flag) {
			if f.Hidden {
				return
			}
			if local.Lookup(f.Name) != nil {
				return
			}
			def := f.DefValue
			if def == "" || def == "[]" {
				def = "-"
			}
			flags = append(flags, flagInfo{f.Name, f.Value.Type(), def, f.Usage})
		})
	}

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

	// Subcommands
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
