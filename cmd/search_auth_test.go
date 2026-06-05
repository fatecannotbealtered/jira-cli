package cmd

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fatecannotbealtered/jira-cli/internal/config"
	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/pflag"
)

func TestMain(m *testing.M) {
	// Allow httptest.NewTLSServer URLs in login tests (login requires https:// hosts).
	http.DefaultTransport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // test-only
	}
	os.Exit(m.Run())
}

func resetLoginFlags(t *testing.T) {
	t.Helper()
	loginHostFlag = ""
	loginTokenFlag = ""
	t.Cleanup(func() {
		loginHostFlag = ""
		loginTokenFlag = ""
	})
}

func resetCmdState(t *testing.T) {
	t.Helper()
	outputFormat = outputFormatJSON
	jsonCompatMode = false
	jsonMode = true
	compactMode = false
	quietMode = false
	dryRun = false
	forceMode = false
	loginHostFlag = ""
	loginTokenFlag = ""
	output.CompactJSON = false
	output.ErrorJSON = false
	output.Quiet = false
	resetFlagValue(rootCmd.PersistentFlags(), "format", outputFormatJSON)
	resetFlagValue(rootCmd.PersistentFlags(), "json", "false")
	resetFlagValue(rootCmd.PersistentFlags(), "compact", "false")
	resetFlagValue(rootCmd.PersistentFlags(), "quiet", "false")
	resetFlagValue(rootCmd.PersistentFlags(), "dry-run", "false")
	resetFlagValue(rootCmd.PersistentFlags(), "force", "false")
	resetFlagValue(searchCmd.Flags(), "limit", "50")
	resetFlagValue(searchCmd.Flags(), "fields", "")
	resetFlagValue(searchCmd.Flags(), "order-by", "")
	resetFlagValue(searchCmd.Flags(), "all", "false")
	resetFlagValue(searchCmd.Flags(), "count", "false")
	resetFlagValue(searchCmd.Flags(), "raw", "false")
}

func resetFlagValue(flags *pflag.FlagSet, name, value string) {
	if f := flags.Lookup(name); f != nil {
		_ = f.Value.Set(value)
		f.Changed = false
	}
}

func runRootClean(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	resetCmdState(t)
	return runRoot(t, args...)
}

func runRootOKClean(t *testing.T, args ...string) (stdout, stderr string) {
	t.Helper()
	resetCmdState(t)
	return runRootOK(t, args...)
}

func runRootWithStdin(t *testing.T, stdin string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	resetCmdState(t)
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("stdin pipe: %v", pipeErr)
	}
	go func() {
		_, _ = io.WriteString(w, stdin)
		_ = w.Close()
	}()
	oldStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = oldStdin })
	return runRoot(t, args...)
}

func clearJiraEnv(t *testing.T) {
	t.Helper()
	t.Setenv("JIRA_HOST", "")
	t.Setenv("JIRA_TOKEN", "")
}

func setupTestHome(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)
	clearJiraEnv(t)
	return tmpDir
}

func writeBadConfig(t *testing.T) {
	t.Helper()
	tmpDir := setupTestHome(t)
	dir := filepath.Join(tmpDir, ".jira-cli")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{invalid"), 0600); err != nil {
		t.Fatalf("write bad config: %v", err)
	}
}

func blockConfigSave(t *testing.T, home string) {
	t.Helper()
	dir := filepath.Join(home, ".jira-cli")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "config.json"), 0700); err != nil {
		t.Fatalf("mkdir config.json: %v", err)
	}
}

func blockConfigDelete(t *testing.T, home string) {
	t.Helper()
	dir := filepath.Join(home, ".jira-cli")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	configPath := filepath.Join(dir, "config.json")
	if err := os.Mkdir(configPath, 0700); err != nil {
		t.Fatalf("mkdir config.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configPath, "nested"), []byte("x"), 0600); err != nil {
		t.Fatalf("write nested file: %v", err)
	}
}

func saveTestConfig(t *testing.T, host, token string) {
	t.Helper()
	setupTestHome(t)
	if err := config.Save(&config.Config{Host: host, Token: token}); err != nil {
		t.Fatalf("save config: %v", err)
	}
}

// setupMockJira starts a TLS httptest server (login requires https://) and sets JIRA_HOST/JIRA_TOKEN.
func setupMockJira(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	ts := httptest.NewTLSServer(handler)
	t.Cleanup(ts.Close)
	setupTestHome(t)
	t.Setenv("JIRA_HOST", ts.URL)
	t.Setenv("JIRA_TOKEN", "test-pat-token")
	return ts
}

func setupMockJiraBlockedConfig(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	ts := httptest.NewTLSServer(handler)
	t.Cleanup(ts.Close)
	home := setupTestHome(t)
	blockConfigSave(t, home)
	return ts
}

const searchIssuesJSON = `{"startAt":0,"maxResults":50,"total":2,"issues":[
	{"id":"1","key":"P-1","fields":{"summary":"Alpha","status":{"name":"Open"},"issuetype":{"name":"Task"}}},
	{"id":"2","key":"P-2","fields":{"summary":"Beta","status":{"name":"Done"},"issuetype":{"name":"Bug"}}}
]}`

const emptySearchJSON = `{"startAt":0,"maxResults":50,"total":0,"issues":[]}`

func jiraSearchHandler(t *testing.T, opts func(w http.ResponseWriter, r *http.Request, body map[string]any)) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/myself"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"jdoe","displayName":"John Doe"}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/search"):
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if opts != nil {
				opts(w, r, body)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(searchIssuesJSON))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

// ─── search runSearch ───────────────────────────────────────────────────────

func TestSearch_NotConfigured(t *testing.T) {
	setupTestHome(t)

	_, _, err := runRootClean(t, "search", "project = P")
	if err != ErrSilent {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	if LastExitCode() != ExitAuth {
		t.Fatalf("exit=%d, want %d", LastExitCode(), ExitAuth)
	}
}

func TestSearch_EmptyJQL(t *testing.T) {
	setupMockJira(t, jiraSearchHandler(t, nil))
	_, stderr, err := runRootClean(t, "--format", "text", "search", "")
	if err != ErrSilent {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	if LastExitCode() != ExitBadArgs {
		t.Fatalf("exit=%d, want %d", LastExitCode(), ExitBadArgs)
	}
	if !containsAny(stderr, "JQL query cannot be empty") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestSearch_LimitValidation(t *testing.T) {
	setupMockJira(t, jiraSearchHandler(t, nil))

	for _, limit := range []string{"0", "501"} {
		t.Run("limit="+limit, func(t *testing.T) {
			_, stderr, err := runRootClean(t, "--format", "text", "search", "project = P", "--limit", limit)
			if err != ErrSilent {
				t.Fatalf("expected ErrSilent, got %v", err)
			}
			if LastExitCode() != ExitBadArgs {
				t.Fatalf("exit=%d, want %d", LastExitCode(), ExitBadArgs)
			}
			if !containsAny(stderr, "--limit must be between 1 and 500") {
				t.Fatalf("stderr=%q", stderr)
			}
		})
	}
}

func TestSearch_DefaultPlain(t *testing.T) {
	setupMockJira(t, jiraSearchHandler(t, nil))
	stdout, _ := runRootOKClean(t, "--format", "text", "search", "project = P", "--limit", "10")
	if !containsAny(stdout, "P-1", "P-2", "Showing 2 of 2") {
		t.Fatalf("stdout=%q", stdout)
	}
}

func TestSearch_DefaultJSONFlat(t *testing.T) {
	setupMockJira(t, jiraSearchHandler(t, nil))
	stdout, _ := runRootOKClean(t, "--json", "search", "project = P")
	var result map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["total"].(float64) != 2 {
		t.Fatalf("total=%v", result["total"])
	}
	issues, ok := result["issues"].([]any)
	if !ok || len(issues) != 2 {
		t.Fatalf("issues=%v", result["issues"])
	}
	first := issues[0].(map[string]any)
	if first["key"] != "P-1" {
		t.Fatalf("first issue key=%v", first["key"])
	}
}

func TestSearch_DefaultJSONRaw(t *testing.T) {
	setupMockJira(t, jiraSearchHandler(t, nil))
	stdout, _ := runRootOKClean(t, "--json", "search", "project = P", "--raw")
	var result map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	issues := result["issues"].([]any)
	first := issues[0].(map[string]any)
	if _, ok := first["fields"]; !ok {
		t.Fatalf("expected raw issue with fields, got %v", first)
	}
}

func TestSearch_DefaultEmptyPlain(t *testing.T) {
	setupMockJira(t, jiraSearchHandler(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(emptySearchJSON))
	}))
	stdout, _ := runRootOKClean(t, "--format", "text", "search", "project = NONE")
	if !containsAny(stdout, "No issues found") {
		t.Fatalf("stdout=%q", stdout)
	}
}

func TestSearch_OrderByAndFields(t *testing.T) {
	setupMockJira(t, jiraSearchHandler(t, func(w http.ResponseWriter, _ *http.Request, body map[string]any) {
		jql, _ := body["jql"].(string)
		if !strings.Contains(jql, "ORDER BY created DESC") {
			t.Errorf("jql=%q, want ORDER BY appended", jql)
		}
		fields, ok := body["fields"].([]any)
		if !ok || len(fields) != 2 {
			t.Fatalf("fields=%v", body["fields"])
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(searchIssuesJSON))
	}))
	runRootOKClean(t, "search", "project = P", "--order-by", "created DESC", "--fields", "summary, status")
}

func TestSearch_OrderBySkippedWhenPresent(t *testing.T) {
	setupMockJira(t, jiraSearchHandler(t, func(w http.ResponseWriter, _ *http.Request, body map[string]any) {
		jql, _ := body["jql"].(string)
		if strings.Count(strings.ToLower(jql), "order by") != 1 {
			t.Errorf("jql=%q, expected single ORDER BY clause", jql)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(searchIssuesJSON))
	}))
	runRootOKClean(t, "search", "project = P ORDER BY updated", "--order-by", "created DESC")
}

func TestSearch_CountPlainAndJSON(t *testing.T) {
	setupMockJira(t, jiraSearchHandler(t, func(w http.ResponseWriter, _ *http.Request, body map[string]any) {
		if maxResults, _ := body["maxResults"].(float64); maxResults != 50 {
			t.Errorf("maxResults=%v, want 50 (API default when 0 passed)", maxResults)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"startAt":0,"maxResults":50,"total":42,"issues":[]}`))
	}))

	stdout, _ := runRootOKClean(t, "--format", "text", "search", "project = P", "--count")
	if !strings.Contains(stdout, "Total: 42") {
		t.Fatalf("stdout=%q", stdout)
	}

	stdout, _ = runRootOKClean(t, "--json", "search", "project = P", "--count")
	var result map[string]int
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["total"] != 42 {
		t.Fatalf("total=%d", result["total"])
	}
}

func TestSearch_AllPlainAndJSON(t *testing.T) {
	callCount := 0
	setupMockJira(t, jiraSearchHandler(t, func(w http.ResponseWriter, _ *http.Request, body map[string]any) {
		callCount++
		startAt, _ := body["startAt"].(float64)
		w.WriteHeader(http.StatusOK)
		if startAt == 0 {
			_, _ = w.Write([]byte(`{"startAt":0,"maxResults":100,"total":2,"issues":[
				{"id":"1","key":"P-1","fields":{"summary":"a","status":{"name":"Open"},"issuetype":{"name":"Task"}}}
			]}`))
			return
		}
		_, _ = w.Write([]byte(`{"startAt":1,"maxResults":100,"total":2,"issues":[
			{"id":"2","key":"P-2","fields":{"summary":"b","status":{"name":"Done"},"issuetype":{"name":"Bug"}}}
		]}`))
	}))

	stdout, _ := runRootOKClean(t, "--format", "text", "search", "--all", "project = P")
	if !containsAny(stdout, "P-1", "P-2") {
		t.Fatalf("stdout=%q", stdout)
	}
	if callCount != 2 {
		t.Fatalf("callCount=%d, want 2", callCount)
	}

	stdout, _ = runRootOKClean(t, "--json", "search", "--all", "project = P", "--raw")
	var rawResult map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &rawResult); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if rawResult["total"].(float64) != 2 {
		t.Fatalf("total=%v", rawResult["total"])
	}

	stdout, _ = runRootOKClean(t, "--json", "search", "--all", "project = P")
	var flatResult map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &flatResult); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	issues := flatResult["issues"].([]any)
	first := issues[0].(map[string]any)
	if _, ok := first["summary"]; !ok {
		t.Fatalf("expected flat issue, got %v", first)
	}
}

func TestSearch_AllEmptyPlain(t *testing.T) {
	setupMockJira(t, jiraSearchHandler(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(emptySearchJSON))
	}))
	stdout, _ := runRootOKClean(t, "--format", "text", "search", "--all", "project = NONE")
	if !containsAny(stdout, "No issues found") {
		t.Fatalf("stdout=%q", stdout)
	}
}

func TestSearch_APIError(t *testing.T) {
	setupMockJira(t, jiraSearchHandler(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errorMessages":["bad JQL"]}`))
	}))
	_, _, err := runRootClean(t, "search", "bad jql")
	if err != ErrSilent {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	if LastExitCode() != ExitBadArgs {
		t.Fatalf("exit=%d, want %d", LastExitCode(), ExitBadArgs)
	}
}

func TestSearch_CountAPIError(t *testing.T) {
	setupMockJira(t, jiraSearchHandler(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errorMessages":["auth"]}`))
	}))
	_, _, err := runRootClean(t, "search", "project = P", "--count")
	if err != ErrSilent || LastExitCode() != ExitAuth {
		t.Fatalf("err=%v exit=%d", err, LastExitCode())
	}
}

func TestSearch_AllAPIError(t *testing.T) {
	setupMockJira(t, jiraSearchHandler(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"errorMessages":["boom"]}`))
	}))
	_, _, err := runRootClean(t, "search", "project = P", "--all")
	if err != ErrSilent || LastExitCode() != ExitNetwork {
		t.Fatalf("err=%v exit=%d", err, LastExitCode())
	}
}

// ─── doctor runDoctor ───────────────────────────────────────────────────────

func TestDoctor_ConfigLoadError(t *testing.T) {
	writeBadConfig(t)
	_, stderr, err := runRootClean(t, "--format", "text", "doctor")
	if err != ErrSilent || LastExitCode() != ExitAuth {
		t.Fatalf("err=%v exit=%d", err, LastExitCode())
	}
	if !containsAny(stderr, "Reading config") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestDoctor_ConfigLoadErrorJSON(t *testing.T) {
	writeBadConfig(t)
	stdout, _, err := runRootClean(t, "--json", "doctor")
	if err != ErrSilent || LastExitCode() != ExitAuth {
		t.Fatalf("err=%v exit=%d", err, LastExitCode())
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["error"] == nil || result["error"] == "" {
		t.Fatalf("result=%v", result)
	}
}

func TestDoctor_NotConfiguredPlain(t *testing.T) {
	setupTestHome(t)

	_, stderr, err := runRootClean(t, "--format", "text", "doctor")
	if err != ErrSilent || LastExitCode() != ExitAuth {
		t.Fatalf("err=%v exit=%d", err, LastExitCode())
	}
	if !containsAny(stderr, "Not configured") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestDoctor_NotConfiguredJSON(t *testing.T) {
	setupTestHome(t)

	stdout, _, err := runRootClean(t, "--json", "doctor")
	if err != ErrSilent || LastExitCode() != ExitAuth {
		t.Fatalf("err=%v exit=%d", err, LastExitCode())
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["configExists"].(bool) {
		t.Fatalf("configExists=%v", result["configExists"])
	}
}

func TestDoctor_ConnectionFailedPlain(t *testing.T) {
	setupMockJira(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/myself") {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"errorMessages":["invalid token"]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	_, stderr, err := runRootClean(t, "--format", "text", "doctor")
	if err != ErrSilent || LastExitCode() != ExitAuth {
		t.Fatalf("err=%v exit=%d", err, LastExitCode())
	}
	if !containsAny(stderr, "Connection failed") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestDoctor_ConnectionFailedJSON(t *testing.T) {
	setupMockJira(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/myself") {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"errorMessages":["invalid token"]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	stdout, _, err := runRootClean(t, "--json", "doctor")
	if err != ErrSilent || LastExitCode() != ExitAuth {
		t.Fatalf("err=%v exit=%d", err, LastExitCode())
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["authValid"].(bool) {
		t.Fatalf("authValid=%v", result["authValid"])
	}
}

func TestDoctor_SuccessPlain(t *testing.T) {
	setupMockJira(t, jiraSearchHandler(t, nil))
	stdout, _ := runRootOKClean(t, "--format", "text", "doctor")
	if !containsAny(stdout, "Config found", "PAT valid", "John Doe", "jdoe") {
		t.Fatalf("stdout=%q", stdout)
	}
}

func TestDoctor_SuccessJSON(t *testing.T) {
	setupMockJira(t, jiraSearchHandler(t, nil))
	stdout, _ := runRootOKClean(t, "--json", "doctor")
	var result map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !result["configExists"].(bool) || !result["authValid"].(bool) {
		t.Fatalf("result=%v", result)
	}
	if result["username"] != "jdoe" || result["displayName"] != "John Doe" {
		t.Fatalf("user fields=%v", result)
	}
}

// ─── login runLogin / runLogout ─────────────────────────────────────────────

func TestLogin_NonInteractiveInvalidHost(t *testing.T) {
	resetLoginFlags(t)
	_, stderr, err := runRootClean(t, "--format", "text", "login", "--host", "http://jira.example.com", "--token", "pat")
	if err != ErrSilent || LastExitCode() != ExitBadArgs {
		t.Fatalf("err=%v exit=%d", err, LastExitCode())
	}
	if !containsAny(stderr, "host must start with https://") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestLogin_NonInteractiveEmptyToken(t *testing.T) {
	resetLoginFlags(t)
	_, stderr, err := runRootClean(t, "--format", "text", "login", "--host", "https://jira.example.com", "--token", "   ")
	if err != ErrSilent || LastExitCode() != ExitBadArgs {
		t.Fatalf("err=%v exit=%d", err, LastExitCode())
	}
	if !containsAny(stderr, "token cannot be empty") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestLogin_NonInteractiveInvalidCredentials(t *testing.T) {
	resetLoginFlags(t)
	ts := setupMockJira(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/myself") {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"errorMessages":["bad token"]}`))
		}
	})
	_, stderr, err := runRootClean(t, "--format", "text", "login", "--host", ts.URL, "--token", "bad-token")
	if err != ErrSilent || LastExitCode() != ExitAuth {
		t.Fatalf("err=%v exit=%d", err, LastExitCode())
	}
	if !containsAny(stderr, "invalid credentials") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestLogin_NonInteractiveSaveFailure(t *testing.T) {
	resetLoginFlags(t)
	ts := setupMockJiraBlockedConfig(t, jiraSearchHandler(t, nil))
	_, stderr, err := runRootClean(t, "--format", "text", "login", "--host", ts.URL, "--token", "good-token")
	if err != ErrSilent || LastExitCode() != ExitAuth {
		t.Fatalf("err=%v exit=%d", err, LastExitCode())
	}
	if !containsAny(stderr, "failed to save config") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestLogin_NonInteractiveSuccessPlain(t *testing.T) {
	resetLoginFlags(t)
	ts := setupMockJira(t, jiraSearchHandler(t, nil))
	stdout, _ := runRootOKClean(t, "--format", "text", "login", "--host", ts.URL, "--token", "good-token")
	if !containsAny(stdout, "Logged in as John Doe", "Config saved") {
		t.Fatalf("stdout=%q", stdout)
	}
	clearJiraEnv(t)
	cfg, err := config.Load()
	if err != nil || cfg.Host != ts.URL || cfg.Token != "good-token" {
		t.Fatalf("config=%+v err=%v", cfg, err)
	}
}

func TestLogin_NonInteractiveSuccessJSON(t *testing.T) {
	resetLoginFlags(t)
	ts := setupMockJira(t, jiraSearchHandler(t, nil))
	stdout, _ := runRootOKClean(t, "--json", "login", "--host", ts.URL, "--token", "good-token")
	var result map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["status"] != "ok" || result["username"] != "jdoe" {
		t.Fatalf("result=%v", result)
	}
}

func TestLogin_InteractiveSuccess(t *testing.T) {
	resetLoginFlags(t)
	ts := setupMockJira(t, jiraSearchHandler(t, nil))
	stdout, _, err := runRootWithStdin(t, ts.URL+"\ngood-token\n", "--format", "text", "login")
	if err != nil {
		t.Fatalf("unexpected error %v (exit=%d)", err, LastExitCode())
	}
	if !containsAny(stdout, "Logged in as John Doe", "Config saved", "Try: jira-cli doctor") {
		t.Fatalf("stdout=%q", stdout)
	}
}

func TestLogin_InteractiveSuccessDefaultJSONStdout(t *testing.T) {
	resetLoginFlags(t)
	ts := setupMockJira(t, jiraSearchHandler(t, nil))
	stdout, stderr, err := runRootWithStdin(t, ts.URL+"\ngood-token\n", "login")
	if err != nil {
		t.Fatalf("unexpected error %v (exit=%d)", err, LastExitCode())
	}
	var result map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("stdout is not clean JSON: %v\nstdout=%q\nstderr=%q", err, stdout, stderr)
	}
	if result["status"] != "ok" || result["username"] != "jdoe" {
		t.Fatalf("result=%v", result)
	}
	if strings.Contains(stdout, "Jira host") || !strings.Contains(stderr, "Jira host") {
		t.Fatalf("interactive prompt should be on stderr only, stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestLogin_InteractiveInvalidHost(t *testing.T) {
	resetLoginFlags(t)
	_, stderr, err := runRootWithStdin(t, "http://bad.example.com\n", "--format", "text", "login")
	if err != ErrSilent || LastExitCode() != ExitBadArgs {
		t.Fatalf("err=%v exit=%d", err, LastExitCode())
	}
	if !containsAny(stderr, "host must start with https://") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestLogin_InteractiveEmptyToken(t *testing.T) {
	resetLoginFlags(t)
	ts := setupMockJira(t, jiraSearchHandler(t, nil))
	_, stderr, err := runRootWithStdin(t, ts.URL+"\n\n", "--format", "text", "login")
	if err != ErrSilent || LastExitCode() != ExitBadArgs {
		t.Fatalf("err=%v exit=%d", err, LastExitCode())
	}
	if !containsAny(stderr, "token cannot be empty") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestLogin_InteractiveInvalidCredentials(t *testing.T) {
	resetLoginFlags(t)
	ts := setupMockJira(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/myself") {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"errorMessages":["bad token"]}`))
		}
	})
	_, stderr, err := runRootWithStdin(t, ts.URL+"\nbad-token\n", "--format", "text", "login")
	if err != ErrSilent || LastExitCode() != ExitAuth {
		t.Fatalf("err=%v exit=%d", err, LastExitCode())
	}
	if !containsAny(stderr, "invalid credentials") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestLogin_InteractiveSaveFailure(t *testing.T) {
	resetLoginFlags(t)
	ts := setupMockJiraBlockedConfig(t, jiraSearchHandler(t, nil))
	_, stderr, err := runRootWithStdin(t, ts.URL+"\ngood-token\n", "--format", "text", "login")
	if err != ErrSilent || LastExitCode() != ExitAuth {
		t.Fatalf("err=%v exit=%d", err, LastExitCode())
	}
	if !containsAny(stderr, "failed to save config") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestLogin_InteractiveHostFlagTokenFromStdin(t *testing.T) {
	resetLoginFlags(t)
	ts := setupMockJira(t, jiraSearchHandler(t, nil))
	stdout, _, err := runRootWithStdin(t, "good-token\n", "--format", "text", "login", "--host", ts.URL)
	if err != nil {
		t.Fatalf("unexpected error %v (exit=%d)", err, LastExitCode())
	}
	if !containsAny(stdout, "Logged in as John Doe") {
		t.Fatalf("stdout=%q", stdout)
	}
}

func TestLogout_Success(t *testing.T) {
	saveTestConfig(t, "https://jira.example.com", "token")
	stdout, _ := runRootOKClean(t, "--format", "text", "logout")
	if !containsAny(stdout, "Logged out") {
		t.Fatalf("stdout=%q", stdout)
	}
	if config.IsConfigured() {
		t.Fatal("config should be removed after logout")
	}
}

func TestLogout_DeleteFailure(t *testing.T) {
	home := setupTestHome(t)
	blockConfigDelete(t, home)
	_, stderr, err := runRootClean(t, "--format", "text", "logout")
	if err != ErrSilent || LastExitCode() != ExitAuth {
		t.Fatalf("err=%v exit=%d", err, LastExitCode())
	}
	if !containsAny(stderr, "failed to remove config") {
		t.Fatalf("stderr=%q", stderr)
	}
}
