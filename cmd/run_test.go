package cmd

import (
	"errors"
	"io"
	"os"
	"testing"

	"github.com/spf13/cobra"
)

func TestRun_SuccessReference(t *testing.T) {
	lastExit = 0
	rootCmd.SetArgs([]string{"reference"})
	if code := Run(); code != ExitOK {
		t.Fatalf("Run() = %d, want 0", code)
	}
	rootCmd.SetArgs(nil)
	lastExit = 0
}

func TestRun_SilentErrorDoctor(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)
	t.Setenv("JIRA_HOST", "")
	t.Setenv("JIRA_TOKEN", "")
	lastExit = 0
	rootCmd.SetArgs([]string{"doctor"})
	if code := Run(); code != ExitAuth {
		t.Fatalf("Run() = %d, want %d", code, ExitAuth)
	}
	rootCmd.SetArgs(nil)
	lastExit = 0
}

func TestRun_GenericError(t *testing.T) {
	failCmd := &cobra.Command{
		Use: "test-run-fail",
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("boom")
		},
	}
	rootCmd.AddCommand(failCmd)
	t.Cleanup(func() { rootCmd.RemoveCommand(failCmd) })

	lastExit = 0
	rootCmd.SetArgs([]string{"test-run-fail"})
	rootCmd.SetErr(io.Discard)
	if code := Run(); code != ExitBadArgs {
		t.Fatalf("Run() = %d, want %d", code, ExitBadArgs)
	}
	rootCmd.SetErr(os.Stderr)
	rootCmd.SetArgs(nil)
	lastExit = 0
}
