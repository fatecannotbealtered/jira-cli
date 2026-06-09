package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// ─── reference walkCommands ───────────────────────────────────────────────────

func TestWalkCommands_HiddenAndFlags(t *testing.T) {
	root := &cobra.Command{Use: "root", Short: "root cmd"}
	root.PersistentFlags().String("verbose", "", "verbose output")
	root.PersistentFlags().Lookup("verbose").DefValue = ""
	root.PersistentFlags().StringSlice("tags", nil, "tags")
	root.PersistentFlags().Lookup("tags").DefValue = "[]"

	child := &cobra.Command{Use: "child", Short: "child cmd", RunE: func(_ *cobra.Command, _ []string) error { return nil }}
	child.Flags().String("name", "", "name flag")
	child.Flags().Lookup("name").DefValue = ""
	child.Flags().String("hidden-flag", "", "hidden")
	child.Flags().Lookup("hidden-flag").Hidden = true
	child.PersistentFlags().String("inherit", "", "inherited on child")
	child.PersistentFlags().Lookup("inherit").DefValue = ""
	root.AddCommand(child)

	hidden := &cobra.Command{Use: "hidden", Hidden: true}
	root.AddCommand(hidden)

	unavailable := &cobra.Command{Use: "deprecated", Deprecated: "use child instead"}
	root.AddCommand(unavailable)

	var lines []string
	walkCommands(root, &lines, "")

	out := strings.Join(lines, "\n")
	if !strings.Contains(out, "## root") {
		t.Fatalf("expected root section, got:\n%s", out)
	}
	if !strings.Contains(out, "root child") {
		t.Fatalf("expected child section, got:\n%s", out)
	}
	if strings.Contains(out, "## root hidden") {
		t.Fatal("hidden command should be omitted")
	}
	if !strings.Contains(out, "### Flags") {
		t.Fatal("expected flags table")
	}
	if strings.Contains(out, "hidden-flag") {
		t.Fatal("hidden flag should be omitted")
	}

	// Persistent flags on a child with a parent are documented when walking the child.
	var childLines []string
	walkCommands(child, &childLines, "root ")
	childOut := strings.Join(childLines, "\n")
	if !strings.Contains(childOut, "`--inherit`") {
		t.Fatalf("expected persistent flag on child walk, got:\n%s", childOut)
	}
}

func TestWalkCommands_HiddenCommandSkipped(t *testing.T) {
	hidden := &cobra.Command{Use: "hidden", Hidden: true, Short: "hidden cmd"}
	var lines []string
	walkCommands(hidden, &lines, "")
	if len(lines) != 0 {
		t.Fatalf("hidden command should produce no lines, got: %v", lines)
	}
}

func TestWalkCommands_DuplicatePersistentFlagSkipped(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().String("overlap", "parent", "overlap persistent")
	child := &cobra.Command{Use: "child", RunE: func(_ *cobra.Command, _ []string) error { return nil }}
	root.AddCommand(child)

	var lines []string
	walkCommands(child, &lines, "root ")
	out := strings.Join(lines, "\n")
	// Inherited persistent flag appears once even when also registered on the local flag set.
	if strings.Count(out, "`--overlap`") > 1 {
		t.Fatalf("--overlap should not be duplicated, got:\n%s", out)
	}
}

func TestWalkCommands_SkipsDeprecatedChild(t *testing.T) {
	root := &cobra.Command{Use: "root", Short: "root"}
	root.AddCommand(&cobra.Command{Use: "live", RunE: func(_ *cobra.Command, _ []string) error { return nil }})
	root.AddCommand(&cobra.Command{Use: "old", Deprecated: "use live"})

	var lines []string
	walkCommands(root, &lines, "")
	out := strings.Join(lines, "\n")
	if strings.Contains(out, "root old") {
		t.Fatalf("deprecated child should be skipped, got:\n%s", out)
	}
	if !strings.Contains(out, "root live") {
		t.Fatalf("expected live child, got:\n%s", out)
	}
}

func TestWalkCommands_PersistentHiddenAndEmptyDefaults(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	child := &cobra.Command{Use: "child", RunE: func(_ *cobra.Command, _ []string) error { return nil }}
	child.PersistentFlags().String("hiddenp", "", "hidden persistent")
	child.PersistentFlags().Lookup("hiddenp").Hidden = true
	child.PersistentFlags().String("emptydef", "", "empty default")
	child.PersistentFlags().Lookup("emptydef").DefValue = ""
	child.PersistentFlags().StringSlice("sliceempty", nil, "slice empty")
	child.PersistentFlags().Lookup("sliceempty").DefValue = "[]"
	root.AddCommand(child)

	var lines []string
	walkCommands(child, &lines, "root ")
	out := strings.Join(lines, "\n")
	if strings.Contains(out, "hiddenp") {
		t.Fatal("hidden persistent flag should be omitted")
	}
	if !strings.Contains(out, "`--emptydef`") || !strings.Contains(out, "`--sliceempty`") {
		t.Fatalf("expected empty-default persistent flags, got:\n%s", out)
	}
}

func TestWalkCommands_PersistentSkippedWhenLocalExists(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	child := &cobra.Command{Use: "child", RunE: func(_ *cobra.Command, _ []string) error { return nil }}
	child.Flags().String("dup", "local-val", "dup flag")
	if pf := child.Flags().Lookup("dup"); pf != nil {
		child.PersistentFlags().AddFlag(pf)
	}
	root.AddCommand(child)

	var lines []string
	walkCommands(child, &lines, "root ")
	out := strings.Join(lines, "\n")
	if strings.Count(out, "`--dup`") != 1 {
		t.Fatalf("--dup should appear once, got:\n%s", out)
	}
}

func TestWalkCommands_RootWithoutShort(t *testing.T) {
	root := &cobra.Command{Use: "bare"}
	var lines []string
	walkCommands(root, &lines, "")
	if !strings.Contains(strings.Join(lines, "\n"), "## bare") {
		t.Fatal("expected bare command heading")
	}
}

func TestPrintReference_WritesMarkdown(t *testing.T) {
	mini := &cobra.Command{Use: "mini", Short: "mini root", Version: "1.0.0"}
	mini.AddCommand(&cobra.Command{Use: "sub", Short: "sub cmd", RunE: func(_ *cobra.Command, _ []string) error { return nil }})
	buf := new(bytes.Buffer)
	outCmd := &cobra.Command{}
	outCmd.SetOut(buf)
	printReference(outCmd, mini)
	out := buf.String()
	if !strings.Contains(out, "# jira-cli Command Reference") {
		t.Fatalf("unexpected output: %s", out)
	}
	if !strings.Contains(out, "mini sub") {
		t.Fatalf("expected subcommand in reference, got: %s", out)
	}
}

// ─── login ReadPassword error ─────────────────────────────────────────────────

func TestLogin_InteractiveReadPasswordError(t *testing.T) {
	resetLoginFlags(t)
	origIsTerm := loginIsTerminal
	origReadPW := loginReadPassword
	defer func() {
		loginIsTerminal = origIsTerm
		loginReadPassword = origReadPW
	}()
	loginIsTerminal = func(_ int) bool { return true }
	loginReadPassword = func(_ int) ([]byte, error) {
		return nil, fmt.Errorf("password read failed")
	}

	ts := setupMockJira(t, jiraSearchHandler(t, nil))
	_, stderr, err := runRootWithStdin(t, ts.URL+"\n", "--format", "text", "login")
	if !errors.Is(err, ErrSilent) || LastExitCode() != ExitBadArgs {
		t.Fatalf("err=%v exit=%d", err, LastExitCode())
	}
	if !containsAny(stderr, "failed to read token") {
		t.Fatalf("stderr=%q", stderr)
	}
}
