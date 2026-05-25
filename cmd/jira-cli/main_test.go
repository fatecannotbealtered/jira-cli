package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/fatecannotbealtered/jira-cli/cmd"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func TestMainReference_ExitZero(t *testing.T) {
	root := repoRoot(t)
	c := exec.Command("go", "run", "./cmd/jira-cli", "reference")
	c.Dir = root
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("go run ./cmd/jira-cli reference failed: %v\n%s", err, out)
	}
	if len(out) == 0 {
		t.Fatal("expected reference output")
	}
}

func TestMain_ReturnsZero(t *testing.T) {
	origArgs := os.Args
	os.Args = []string{"jira-cli", "reference"}
	t.Cleanup(func() { os.Args = origArgs })

	if code := Main(); code != cmd.ExitOK {
		t.Fatalf("Main() = %d, want 0", code)
	}
}

func TestMain_ExitFn(t *testing.T) {
	origExit := exitFn
	origArgs := os.Args
	t.Cleanup(func() {
		exitFn = origExit
		os.Args = origArgs
	})
	os.Args = []string{"jira-cli", "reference"}
	var gotCode int
	exitFn = func(code int) { gotCode = code }
	main()
	if gotCode != cmd.ExitOK {
		t.Fatalf("main exit = %d, want 0", gotCode)
	}
}

func TestMain_SilentErrorViaDoctor(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)
	t.Setenv("JIRA_HOST", "")
	t.Setenv("JIRA_TOKEN", "")

	origArgs := os.Args
	os.Args = []string{"jira-cli", "doctor"}
	t.Cleanup(func() { os.Args = origArgs })

	if code := Main(); code != cmd.ExitAuth {
		t.Fatalf("Main() = %d, want %d", code, cmd.ExitAuth)
	}
}
