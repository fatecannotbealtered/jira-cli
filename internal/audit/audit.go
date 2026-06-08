package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fatecannotbealtered/jira-cli/internal/config"
)

var (
	auditJSONMarshal = json.Marshal
	auditReadDir     = os.ReadDir
)

// entry is a single audit log record written as one JSON line.
type entry struct {
	Ts   string   `json:"ts"`
	Cmd  string   `json:"cmd"`
	Args []string `json:"args"`
	Host string   `json:"host,omitempty"`
	Exit int      `json:"exit"`
	Ms   int64    `json:"ms"`
}

// Dir returns the audit log directory ~/.jira-cli/audit/
func Dir() string {
	if testDir != "" {
		return testDir
	}
	return filepath.Join(config.Dir(), "audit")
}

// testDir overrides Dir() when set (for testing).
var testDir string

// Log writes one audit entry to the current month's JSONL file.
// It is a no-op if JIRA_NO_AUDIT=1. It performs lazy cleanup of old files.
func Log(cmdPath string, args []string, exitCode int, durationMs int64) {
	if os.Getenv("JIRA_NO_AUDIT") == "1" {
		return
	}

	dir := Dir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return // silently skip if we can't create the directory
	}

	// Lazy cleanup of old files
	cleanup(dir)

	e := entry{
		Ts:   time.Now().UTC().Format(time.RFC3339Nano),
		Cmd:  cmdPath,
		Args: sanitizeArgs(args),
		Exit: exitCode,
		Ms:   durationMs,
	}
	if cfg, err := config.Load(); err == nil {
		e.Host = cfg.Host
	}

	data, err := auditJSONMarshal(e)
	if err != nil {
		return
	}
	data = append(data, '\n')

	filename := "audit-" + time.Now().UTC().Format("2006-01") + ".jsonl"
	path := filepath.Join(dir, filename)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	_, _ = f.Write(data)
}

// retentionMonths returns the number of months to keep audit files.
// Defaults to 3. Set JIRA_AUDIT_RETENTION_MONTHS=0 to disable cleanup.
func retentionMonths() int {
	s := os.Getenv("JIRA_AUDIT_RETENTION_MONTHS")
	if s == "" {
		return 3
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 3
	}
	return n
}

// cleanup removes audit JSONL files older than the retention period.
func cleanup(dir string) {
	months := retentionMonths()
	if months == 0 {
		return
	}

	cutoff := time.Now().AddDate(0, -months, 0).Format("2006-01")

	entries, err := auditReadDir(dir)
	if err != nil {
		return
	}

	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "audit-") || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		// Extract YYYY-MM from "audit-YYYY-MM.jsonl"
		ym := strings.TrimPrefix(name, "audit-")
		ym = strings.TrimSuffix(ym, ".jsonl")
		if ym < cutoff {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
}

// sensitiveFlags contains flag names that should have their values redacted.
// JDC-only CLI uses PAT (Personal Access Token), not passwords.
var sensitiveFlags = map[string]bool{
	"token": true,
	"t":     true, // short for --token
}

// sanitizeArgs removes sensitive flag values and returns a clean copy.
// Handles: --token value, --token=value, -t value, -t=value
func sanitizeArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		lower := strings.ToLower(arg)

		// Check for combined flag+value (e.g., -t=xxx or --token=xxx)
		if strings.Contains(lower, "=") {
			flag := strings.TrimPrefix(strings.TrimPrefix(strings.SplitN(lower, "=", 2)[0], "--"), "-")
			if sensitiveFlags[flag] {
				// Replace with redacted version
				parts := strings.SplitN(arg, "=", 2)
				out = append(out, parts[0]+"=***")
				continue
			}
		}

		// Check if this is a sensitive flag followed by a value
		stripped := strings.TrimPrefix(lower, "--")
		stripped = strings.TrimPrefix(stripped, "-")

		if sensitiveFlags[stripped] {
			i++ // skip next arg (the value)
			continue
		}

		out = append(out, arg)
	}
	return out
}

// Files returns the list of audit JSONL files in the audit directory, sorted by name.
// Exported for testing.
func Files() ([]string, error) {
	dir := Dir()
	entries, err := auditReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)
	return files, nil
}
