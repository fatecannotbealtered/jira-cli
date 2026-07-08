package cmd

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
)

// failOnNetwork is an http.RoundTripper that fails the test if any HTTP request
// is issued, proving the meta.notices piggyback path does zero network I/O.
type failOnNetwork struct{ t *testing.T }

func (f failOnNetwork) RoundTrip(r *http.Request) (*http.Response, error) {
	f.t.Fatalf("unexpected network call to %s", r.URL)
	return nil, nil
}

// enableNoticeCache lets a test exercise the otherwise-auto-disabled cache,
// pointing it at a temp HOME, and restores everything afterwards.
func enableNoticeCache(t *testing.T) {
	t.Helper()
	old := updateNoticeAutoDisabled
	t.Cleanup(func() { updateNoticeAutoDisabled = old })
	updateNoticeAutoDisabled = func() bool { return false }
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv(updateNoticeEnvOptOut, "")
}

func metaNoticesFromCommand(t *testing.T) []map[string]any {
	t.Helper()
	resetCLIState(t)
	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"changelog", "--compact"})
		_ = Execute()
		rootCmd.SetArgs(nil)
	})
	// changelog may emit several lines in some modes; the envelope is the line
	// that parses as the success envelope.
	var notices []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		var payload struct {
			OK   bool `json:"ok"`
			Meta struct {
				Notices []map[string]any `json:"notices"`
			} `json:"meta"`
		}
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			continue
		}
		if payload.OK {
			notices = payload.Meta.Notices
		}
	}
	return notices
}

func TestMetaNotices_PresentFromCache(t *testing.T) {
	enableNoticeCache(t)
	// Seed the cache directly via the read-only-tested writer.
	writeUpdateNoticeCache(updateNoticesFromValues("1.0.0", "1.1.0", "npm", "", "update_check"))

	// Any network access during the piggyback path is a contract violation.
	oldClient := updateHTTPClient
	t.Cleanup(func() { updateHTTPClient = oldClient })
	updateHTTPClient = &http.Client{Transport: failOnNetwork{t}}

	notices := metaNoticesFromCommand(t)
	if len(notices) != 1 {
		t.Fatalf("meta.notices = %v, want 1 cached notice", notices)
	}
	if notices[0]["type"] != "update_available" {
		t.Errorf("notice type = %v", notices[0]["type"])
	}
}

func TestMetaNotices_AbsentWhenCacheEmpty(t *testing.T) {
	enableNoticeCache(t)
	// No cache written → nothing to report.
	notices := metaNoticesFromCommand(t)
	if len(notices) != 0 {
		t.Fatalf("meta.notices = %v, want none on empty cache", notices)
	}
}

func TestUpdateNoticeAutoDisabledDetectsWindowsGoTestBinary(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{`C:\Users\me\AppData\Local\Temp\cmd.test.exe`}
	t.Setenv(updateNoticeEnvOptOut, "")

	if !updateNoticeAutoDisabled() {
		t.Fatal("Windows Go test binary must not write the real update notice cache")
	}
}

func TestUpdateNoticeSeverity(t *testing.T) {
	const securityChangelog = `## [1.1.0] - 2026-06-20
### Security
- patch an auth bypass
## [1.0.0] - 2026-06-01
### Added
- initial
`
	const plainChangelog = `## [1.0.1] - 2026-06-20
### Fixed
- a small bug
## [1.0.0] - 2026-06-01
### Added
- initial
`
	cases := []struct {
		name            string
		current, latest string
		markdown        string
		want            string
	}{
		{"security entry in delta", "1.0.0", "1.1.0", securityChangelog, "warning"},
		{"major bump", "1.5.0", "2.0.0", plainChangelog, "warning"},
		{"plain patch", "1.0.0", "1.0.1", plainChangelog, "info"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := updateNoticeSeverityFromChangelog(tc.current, tc.latest, tc.markdown)
			if got != tc.want {
				t.Errorf("severity(%s->%s) = %q, want %q", tc.current, tc.latest, got, tc.want)
			}
		})
	}
}

func TestNoticesAsAny(t *testing.T) {
	if got := noticesAsAny(nil); got != nil {
		t.Errorf("noticesAsAny(nil) = %v, want nil", got)
	}
	notices := updateNoticesFromValues("1.0.0", "1.1.0", "npm", "", "cache")
	out := noticesAsAny(notices)
	if len(out) != len(notices) {
		t.Fatalf("noticesAsAny len = %d, want %d", len(out), len(notices))
	}
}
