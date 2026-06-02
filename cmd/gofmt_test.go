package cmd

import (
	"bytes"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSourcesGofmted fails if any .go file in the module is not gofmt-clean,
// catching formatting issues locally before CI does. Line endings are
// normalized to LF so the check matches CI (Linux/LF) and does not produce
// false positives on Windows checkouts that use CRLF.
func TestSourcesGofmted(t *testing.T) {
	root := moduleRoot(t)
	var bad []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		norm := bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))
		out, err := format.Source(norm)
		if err != nil {
			return nil // skip files gofmt itself cannot parse
		}
		if !bytes.Equal(norm, out) {
			rel, _ := filepath.Rel(root, path)
			bad = append(bad, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) > 0 {
		t.Fatalf("the following files are not gofmt-clean (run `gofmt -w .`):\n%s", strings.Join(bad, "\n"))
	}
}

func moduleRoot(t *testing.T) string {
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
			t.Fatal(fmt.Errorf("go.mod not found above %s", dir))
		}
		dir = parent
	}
}
