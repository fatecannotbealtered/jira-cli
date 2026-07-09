package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func runRootExpectSilentClean(t *testing.T, code int, args ...string) (stdout, stderr string) {
	t.Helper()
	resetCmdState(t)
	resetDomainCmdFlags(t)
	return runRootExpectSilent(t, code, textFormatUnlessSpecified(args)...)
}

func runRootOKCleanDomain(t *testing.T, args ...string) (stdout, stderr string) {
	t.Helper()
	resetCmdState(t)
	resetDomainCmdFlags(t)
	return runRootOK(t, textFormatUnlessSpecified(args)...)
}

// resetDomainCmdFlags clears subcommand flags that cobra does not reset between runs.
func resetDomainCmdFlags(t *testing.T) {
	t.Helper()
	for _, cmd := range []*cobra.Command{
		sprintListCmd, sprintActiveCmd, sprintIssuesCmd, sprintCreateCmd, sprintUpdateCmd,
		sprintCloseCmd, sprintMoveCmd, boardListCmd, boardGetCmd, boardBacklogCmd, boardEpicsCmd,
		boardSprintsCmd, projectListCmd, projectVersionsCmd, projectFieldsCmd,
		filterCreateCmd, filterRunCmd, epicListCmd, epicIssuesCmd, userSearchCmd,
	} {
		resetFlagValue(cmd.Flags(), "board", "0")
		resetFlagValue(cmd.Flags(), "sprint", "0")
		resetFlagValue(cmd.Flags(), "state", "")
		resetFlagValue(cmd.Flags(), "raw", "false")
		resetFlagValue(cmd.Flags(), "fields", "")
		resetFlagValue(cmd.Flags(), "name", "")
		resetFlagValue(cmd.Flags(), "jql", "")
		resetFlagValue(cmd.Flags(), "description", "")
		resetFlagValue(cmd.Flags(), "issues", "")
		resetFlagValue(cmd.Flags(), "start-date", "")
		resetFlagValue(cmd.Flags(), "end-date", "")
		resetFlagValue(cmd.Flags(), "goal", "")
		resetFlagValue(cmd.Flags(), "project", "")
		resetFlagValue(cmd.Flags(), "type", "")
		// Reset "limit" to each command's own default (50 for most; 0 = no limit
		// for project list), not a hardcoded value that would mask the real default.
		if lf := cmd.Flags().Lookup("limit"); lf != nil {
			resetFlagValue(cmd.Flags(), "limit", lf.DefValue)
		}
		resetFlagValue(cmd.Flags(), "done", "false")
		resetFlagValue(cmd.Flags(), "released", "false")
		resetFlagValue(cmd.Flags(), "unreleased", "false")
		resetFlagValue(cmd.Flags(), "custom", "false")
		resetFlagValue(cmd.Flags(), "query", "")
		resetFlagValue(cmd.Flags(), "assignable", "false")
	}
}

// domainMockHandler serves Jira REST/Agile responses for domain command tests.
func domainMockHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	method := r.Method

	writeJSON := func(status int, body string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = fmt.Fprint(w, body)
	}

	switch {
	case path == "/rest/agile/1.0/board" && method == http.MethodGet:
		writeJSON(http.StatusOK, `{"startAt":0,"maxResults":50,"total":2,"isLast":true,"values":[
			{"id":1,"name":"Scrum Board","type":"scrum","location":{"projectKey":"PROJ"}},
			{"id":2,"name":"Kanban Board","type":"kanban","location":{"projectKey":"OPS"}}
		]}`)

	case strings.HasPrefix(path, "/rest/agile/1.0/board/") && strings.HasSuffix(path, "/backlog"):
		writeJSON(http.StatusOK, `{"startAt":0,"maxResults":50,"total":3,"isLast":true,"issues":[
			{"id":"1","key":"P-1","fields":{"summary":"backlog one","status":{"name":"To Do"},"issuetype":{"name":"Story"},"updated":"2024-01-01T00:00:00.000+0000"}},
			{"id":"2","key":"P-2","fields":{"summary":"backlog two","status":{"name":"To Do"},"issuetype":{"name":"Story"},"updated":"2024-01-02T00:00:00.000+0000"}},
			{"id":"3","key":"P-3","fields":{"summary":"backlog three","status":{"name":"To Do"},"issuetype":{"name":"Story"},"updated":"2024-01-03T00:00:00.000+0000"}}
		]}`)

	case strings.HasPrefix(path, "/rest/agile/1.0/board/") && strings.HasSuffix(path, "/epic"):
		done := r.URL.Query().Get("done") == "true"
		if done {
			writeJSON(http.StatusOK, `{"startAt":0,"maxResults":50,"total":0,"isLast":true,"values":[]}`)
			return
		}
		writeJSON(http.StatusOK, `{"startAt":0,"maxResults":50,"total":1,"isLast":true,"values":[
			{"id":"100","key":"PROJ-10","fields":{"summary":"Epic One","status":{"name":"In Progress"},"issuetype":{"name":"Epic"},"updated":"2024-01-01T00:00:00.000+0000"}}
		]}`)

	case strings.HasPrefix(path, "/rest/agile/1.0/board/") && strings.HasSuffix(path, "/sprint"):
		state := r.URL.Query().Get("state")
		if state == "empty" {
			writeJSON(http.StatusOK, `{"startAt":0,"maxResults":50,"total":0,"isLast":true,"values":[]}`)
			return
		}
		writeJSON(http.StatusOK, `{"startAt":0,"maxResults":50,"total":3,"isLast":true,"values":[
			{"id":1,"name":"Sprint Active","state":"active","startDate":"2024-01-01T00:00:00.000Z","endDate":"2024-01-14T00:00:00.000Z","goal":"Ship it"},
			{"id":2,"name":"Sprint Future","state":"future","startDate":"2024-02-01","endDate":"2024-02-14","goal":"`+strings.Repeat("x", 45)+`"},
			{"id":3,"name":"Sprint Closed","state":"closed","startDate":"2023","endDate":"2023-12","goal":""}
		]}`)

	case strings.HasPrefix(path, "/rest/agile/1.0/board/") && method == http.MethodGet:
		writeJSON(http.StatusOK, `{"id":42,"name":"My Board","type":"scrum","location":{"projectKey":"PROJ"}}`)

	case strings.HasPrefix(path, "/rest/agile/1.0/sprint/") && strings.HasSuffix(path, "/issue") && method == http.MethodPost:
		writeJSON(http.StatusNoContent, ``)

	case strings.HasPrefix(path, "/rest/agile/1.0/sprint/") && strings.HasSuffix(path, "/issue"):
		writeJSON(http.StatusOK, `{"startAt":0,"maxResults":50,"total":1,"isLast":true,"issues":[
			{"id":"1","key":"P-1","fields":{"summary":"sprint issue","status":{"name":"Open"},"issuetype":{"name":"Task"},"updated":"2024-01-01T00:00:00.000+0000"}}
		]}`)

	case strings.HasPrefix(path, "/rest/agile/1.0/sprint/") && method == http.MethodPut:
		writeJSON(http.StatusOK, `{"id":10,"name":"Updated Sprint","state":"closed"}`)

	case strings.HasPrefix(path, "/rest/agile/1.0/sprint/") && method == http.MethodGet:
		if strings.Contains(path, "/sprint/99") {
			writeJSON(http.StatusOK, `{"id":99,"name":"Closed Sprint","state":"closed"}`)
			return
		}
		writeJSON(http.StatusOK, `{"id":10,"name":"Active Sprint","state":"active","goal":"Finish"}`)

	case path == "/rest/agile/1.0/sprint" && method == http.MethodPost:
		writeJSON(http.StatusCreated, `{"id":20,"name":"New Sprint","state":"future"}`)

	case strings.HasPrefix(path, "/rest/agile/1.0/epic/") && strings.HasSuffix(path, "/issue"):
		writeJSON(http.StatusOK, `{"startAt":0,"maxResults":50,"total":1,"isLast":true,"issues":[
			{"id":"1","key":"P-11","fields":{"summary":"epic child","status":{"name":"Open"},"issuetype":{"name":"Story"},"updated":"2024-01-01T00:00:00.000+0000"}}
		]}`)

	case path == "/rest/api/2/project" && method == http.MethodGet:
		writeJSON(http.StatusOK, `[
			{"id":"1","key":"PROJ","name":"My Project","projectTypeKey":"software","lead":{"displayName":"Alice"}},
			{"id":"2","key":"OPS","name":"Operations","projectTypeKey":"business"}
		]`)

	case strings.HasPrefix(path, "/rest/api/2/project/") && strings.HasSuffix(path, "/components"):
		writeJSON(http.StatusOK, `[{"id":"10","name":"Backend"},{"id":"11","name":"Frontend"}]`)

	case strings.HasPrefix(path, "/rest/api/2/project/") && strings.HasSuffix(path, "/versions"):
		writeJSON(http.StatusOK, `[
			{"id":"1","name":"v1.0","released":true,"releaseDate":"2024-01-01","description":"First release"},
			{"id":"2","name":"v2.0","released":false,"releaseDate":"","description":""}
		]`)

	case strings.HasPrefix(path, "/rest/api/2/project/") && method == http.MethodGet:
		writeJSON(http.StatusOK, `{"id":"1","key":"PROJ","name":"My Project","projectTypeKey":"software","style":"classic","description":"A test project","lead":{"displayName":"Alice"},"issueTypes":[
			{"id":"1","name":"Story","subtask":false},
			{"id":"2","name":"Sub-task","subtask":true}
		]}`)

	case path == "/rest/api/2/field":
		writeJSON(http.StatusOK, `[
			{"id":"summary","name":"Summary","custom":false,"schema":{"type":"string"}},
			{"id":"customfield_10001","name":"Custom Field","custom":true,"schema":{"type":"array","items":"string"}}
		]`)

	case path == "/rest/api/2/filter/favourite":
		writeJSON(http.StatusOK, `[
			{"id":"100","name":"My Bugs","jql":"type = Bug","favourite":true},
			{"id":"101","name":"Long JQL","jql":"`+strings.Repeat("project = PROJ AND ", 10)+`type = Bug","favourite":false}
		]`)

	case strings.HasPrefix(path, "/rest/api/2/filter/") && method == http.MethodDelete:
		writeJSON(http.StatusNoContent, ``)

	case strings.HasPrefix(path, "/rest/api/2/filter/") && method == http.MethodGet:
		writeJSON(http.StatusOK, `{"id":"100","name":"My Bugs","jql":"type = Bug","description":"All bugs","favourite":true}`)

	case path == "/rest/api/2/filter" && method == http.MethodPost:
		writeJSON(http.StatusOK, `{"id":"200","name":"New Filter","jql":"project = PROJ"}`)

	case path == "/rest/api/2/search" && method == http.MethodPost:
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		jql, _ := body["jql"].(string)
		if strings.Contains(jql, "Epic Link") {
			writeJSON(http.StatusBadRequest, `{"errorMessages":["Epic Link field not found"]}`)
			return
		}
		writeJSON(http.StatusOK, `{"startAt":0,"maxResults":50,"total":1,"issues":[
			{"id":"1","key":"P-20","fields":{"summary":"jql epic issue","status":{"name":"Open"},"issuetype":{"name":"Story"},"updated":"2024-01-01T00:00:00.000+0000"}}
		]}`)

	case path == "/rest/api/2/user/search":
		writeJSON(http.StatusOK, `[{"name":"johndoe","displayName":"John Doe","emailAddress":"john@example.com","active":true}]`)

	case path == "/rest/api/2/user/assignable/search":
		writeJSON(http.StatusOK, `[{"name":"janedoe","displayName":"Jane Doe","emailAddress":"jane@example.com","active":true}]`)

	case path == "/rest/api/2/myself":
		writeJSON(http.StatusOK, `{"name":"currentuser","displayName":"Current User","emailAddress":"me@example.com","active":true}`)

	default:
		writeJSON(http.StatusNotFound, `{"errorMessages":["not found"]}`)
	}
}

func emptyDomainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	path := r.URL.Path
	switch {
	case strings.Contains(path, "/sprint") && strings.Contains(path, "/issue"):
		_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":50,"total":0,"isLast":true,"issues":[]}`)
	case strings.Contains(path, "/backlog"), strings.Contains(path, "/epic/"):
		_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":50,"total":0,"isLast":true,"issues":[]}`)
	case strings.Contains(path, "/epic"):
		_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":50,"total":0,"isLast":true,"values":[]}`)
	case strings.Contains(path, "/sprint"):
		_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":50,"total":0,"isLast":true,"values":[]}`)
	case path == "/rest/agile/1.0/board":
		_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":50,"total":0,"isLast":true,"values":[]}`)
	case path == "/rest/api/2/project":
		_, _ = fmt.Fprint(w, `[]`)
	case strings.HasSuffix(path, "/components"), strings.HasSuffix(path, "/versions"):
		_, _ = fmt.Fprint(w, `[]`)
	case path == "/rest/api/2/field":
		_, _ = fmt.Fprint(w, `[]`)
	case path == "/rest/api/2/filter/favourite":
		_, _ = fmt.Fprint(w, `[]`)
	case path == "/rest/api/2/user/search", path == "/rest/api/2/user/assignable/search":
		_, _ = fmt.Fprint(w, `[]`)
	case path == "/rest/api/2/search":
		_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":50,"total":0,"issues":[]}`)
	default:
		domainMockHandler(w, r)
	}
}

// ─── Sprint ─────────────────────────────────────────────────────────────────

func TestSprintCommands(t *testing.T) {
	mockJiraServer(t, domainMockHandler)

	t.Run("list missing board", func(t *testing.T) {
		runRootExpectSilentClean(t, ExitBadArgs, "sprint", "list")
	})

	t.Run("list table", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "sprint", "list", "--board", "42")
		if !containsAny(stdout, "Sprint Active", "ID") {
			t.Fatalf("expected sprint table output, got: %s", stdout)
		}
	})

	t.Run("list empty", func(t *testing.T) {
		mockJiraServer(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/sprint") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":50,"total":0,"isLast":true,"values":[]}`)
				return
			}
			domainMockHandler(w, r)
		})
		stdout, _ := runRootOKCleanDomain(t, "sprint", "list", "--board", "42")
		if !containsAny(stdout, "No sprints found") {
			t.Fatalf("expected empty message, got: %s", stdout)
		}
	})

	t.Run("list json flat fields", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--json", "sprint", "list", "--board", "42", "--fields", "id,name")
		if !strings.Contains(stdout, `"id"`) || !strings.Contains(stdout, `"name"`) {
			t.Fatalf("expected filtered JSON, got: %s", stdout)
		}
	})

	t.Run("list json raw", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--json", "sprint", "list", "--board", "42", "--raw")
		if !strings.Contains(stdout, "Sprint Active") {
			t.Fatalf("expected raw sprint JSON, got: %s", stdout)
		}
	})

	t.Run("active missing board", func(t *testing.T) {
		runRootExpectSilentClean(t, ExitBadArgs, "sprint", "active")
	})

	t.Run("active table", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "sprint", "active", "--board", "42")
		if !containsAny(stdout, "Sprint Active", "sprint issue") {
			t.Fatalf("expected active sprint output, got: %s", stdout)
		}
	})

	t.Run("active json flat", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--json", "sprint", "active", "--board", "42")
		if !strings.Contains(stdout, `"sprint"`) || !strings.Contains(stdout, `"issues"`) {
			t.Fatalf("expected flat JSON, got: %s", stdout)
		}
	})

	t.Run("active json raw", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--json", "sprint", "active", "--board", "42", "--raw")
		if !strings.Contains(stdout, "Sprint Active") {
			t.Fatalf("expected raw JSON, got: %s", stdout)
		}
	})

	t.Run("active no sprint", func(t *testing.T) {
		mockJiraServer(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/board/") && strings.HasSuffix(r.URL.Path, "/sprint") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":50,"total":0,"isLast":true,"values":[]}`)
				return
			}
			domainMockHandler(w, r)
		})
		stdout, _ := runRootOKCleanDomain(t, "sprint", "active", "--board", "42")
		if !containsAny(stdout, "No active sprint") {
			t.Fatalf("expected no active sprint message, got: %s", stdout)
		}
	})

	t.Run("issues missing sprint", func(t *testing.T) {
		runRootExpectSilentClean(t, ExitBadArgs, "sprint", "issues")
	})

	t.Run("issues table", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "sprint", "issues", "--sprint", "10")
		if !containsAny(stdout, "P-1", "sprint issue") {
			t.Fatalf("expected issue output, got: %s", stdout)
		}
	})

	t.Run("issues empty", func(t *testing.T) {
		mockJiraServer(t, emptyDomainHandler)
		stdout, _ := runRootOKCleanDomain(t, "sprint", "issues", "--sprint", "10")
		if !containsAny(stdout, "No issues in this sprint") {
			t.Fatalf("expected empty message, got: %s", stdout)
		}
	})

	t.Run("issues json", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--json", "sprint", "issues", "--sprint", "10", "--fields", "key")
		if !strings.Contains(stdout, "P-1") {
			t.Fatalf("expected JSON issues, got: %s", stdout)
		}
	})

	t.Run("create missing args", func(t *testing.T) {
		runRootExpectSilentClean(t, ExitBadArgs, "sprint", "create", "--board", "42")
	})

	t.Run("create dry-run", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--dry-run", "sprint", "create", "--board", "42", "--name", "S1")
		if !containsAny(stdout, "dry-run", "create sprint") {
			t.Fatalf("expected dry-run output, got: %s", stdout)
		}
	})

	t.Run("create success", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "sprint", "create", "--board", "42", "--name", "S1", "--goal", "G", "--start-date", "2024-01-01", "--end-date", "2024-01-14")
		if !containsAny(stdout, "Created sprint", "New Sprint") {
			t.Fatalf("expected create success, got: %s", stdout)
		}
	})

	t.Run("create json", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--json", "sprint", "create", "--board", "42", "--name", "S1")
		if !strings.Contains(stdout, `"id"`) && !strings.Contains(stdout, "20") {
			t.Fatalf("expected JSON sprint, got: %s", stdout)
		}
	})

	t.Run("update missing sprint", func(t *testing.T) {
		runRootExpectSilentClean(t, ExitBadArgs, "sprint", "update", "--name", "X")
	})

	t.Run("update dry-run", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--dry-run", "sprint", "update", "--sprint", "10", "--name", "X")
		if !containsAny(stdout, "dry-run") {
			t.Fatalf("expected dry-run, got: %s", stdout)
		}
	})

	t.Run("update success", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "sprint", "update", "--sprint", "10", "--name", "Renamed", "--goal", "G", "--start-date", "2024-01-01", "--end-date", "2024-01-14")
		if !containsAny(stdout, "Updated sprint") {
			t.Fatalf("expected update success, got: %s", stdout)
		}
	})

	t.Run("update json", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--json", "sprint", "update", "--sprint", "10", "--name", "Renamed")
		if !strings.Contains(stdout, "Updated Sprint") {
			t.Fatalf("expected JSON, got: %s", stdout)
		}
	})

	t.Run("close missing sprint", func(t *testing.T) {
		runRootExpectSilentClean(t, ExitBadArgs, "sprint", "close")
	})

	t.Run("close already closed", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "sprint", "close", "--sprint", "99")
		if !containsAny(stdout, "already closed") {
			t.Fatalf("expected already closed, got: %s", stdout)
		}
	})

	t.Run("close gate blocks without --dangerous", func(t *testing.T) {
		runRootExpectSilentClean(t, ExitConfirmRequired, "--dry-run", "sprint", "close", "--sprint", "10")
	})

	t.Run("close dry-run", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--dangerous", "--dry-run", "sprint", "close", "--sprint", "10")
		if !containsAny(stdout, "dry-run") {
			t.Fatalf("expected dry-run, got: %s", stdout)
		}
	})

	t.Run("close success", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--dangerous", "sprint", "close", "--sprint", "10")
		if !containsAny(stdout, "closed") {
			t.Fatalf("expected close success, got: %s", stdout)
		}
	})

	t.Run("close json", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--dangerous", "--json", "sprint", "close", "--sprint", "10")
		if !strings.Contains(stdout, "closed") {
			t.Fatalf("expected JSON, got: %s", stdout)
		}
	})

	t.Run("move missing args", func(t *testing.T) {
		runRootExpectSilentClean(t, ExitBadArgs, "sprint", "move", "--sprint", "10")
	})

	t.Run("move dry-run", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--dry-run", "sprint", "move", "--sprint", "10", "--issues", "A-1, A-2")
		if !containsAny(stdout, "dry-run") {
			t.Fatalf("expected dry-run, got: %s", stdout)
		}
	})

	t.Run("move success", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "sprint", "move", "--sprint", "10", "--issues", "A-1, A-2")
		if !containsAny(stdout, "Moved 2 issue") {
			t.Fatalf("expected move success, got: %s", stdout)
		}
	})

	t.Run("move json", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--json", "sprint", "move", "--sprint", "10", "--issues", "A-1")
		if !strings.Contains(stdout, "sprintId") || !strings.Contains(stdout, "10") {
			t.Fatalf("expected JSON, got: %s", stdout)
		}
	})
}

// ─── Board ──────────────────────────────────────────────────────────────────

func TestBoardCommands(t *testing.T) {
	mockJiraServer(t, domainMockHandler)

	t.Run("list table", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "board", "list")
		if !containsAny(stdout, "Scrum Board", "Kanban Board") {
			t.Fatalf("expected boards, got: %s", stdout)
		}
	})

	t.Run("list json", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--json", "board", "list", "--project", "PROJ", "--type", "scrum")
		if !strings.Contains(stdout, "PROJ") {
			t.Fatalf("expected JSON boards, got: %s", stdout)
		}
	})

	t.Run("list empty", func(t *testing.T) {
		mockJiraServer(t, emptyDomainHandler)
		stdout, _ := runRootOKCleanDomain(t, "board", "list")
		if !containsAny(stdout, "No boards found") {
			t.Fatalf("expected empty message, got: %s", stdout)
		}
	})

	t.Run("get missing board", func(t *testing.T) {
		runRootExpectSilentClean(t, ExitBadArgs, "board", "get")
	})

	t.Run("get table", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "board", "get", "--board", "42")
		if !containsAny(stdout, "My Board", "PROJ") {
			t.Fatalf("expected board details, got: %s", stdout)
		}
	})

	t.Run("get json", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--json", "board", "get", "--board", "42")
		if !strings.Contains(stdout, "My Board") {
			t.Fatalf("expected JSON, got: %s", stdout)
		}
	})

	t.Run("backlog missing board", func(t *testing.T) {
		runRootExpectSilentClean(t, ExitBadArgs, "board", "backlog")
	})

	t.Run("backlog table limit", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "board", "backlog", "--board", "42", "--limit", "2")
		if !containsAny(stdout, "P-1", "P-2") {
			t.Fatalf("expected backlog issues, got: %s", stdout)
		}
	})

	t.Run("backlog empty", func(t *testing.T) {
		mockJiraServer(t, emptyDomainHandler)
		stdout, _ := runRootOKCleanDomain(t, "board", "backlog", "--board", "42")
		if !containsAny(stdout, "Backlog is empty") {
			t.Fatalf("expected empty backlog, got: %s", stdout)
		}
	})

	t.Run("backlog json", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--json", "board", "backlog", "--board", "42")
		if !strings.Contains(stdout, "P-1") {
			t.Fatalf("expected JSON backlog, got: %s", stdout)
		}
	})

	t.Run("epics missing board", func(t *testing.T) {
		runRootExpectSilentClean(t, ExitBadArgs, "board", "epics")
	})

	t.Run("epics table done", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "board", "epics", "--board", "42", "--done")
		if !containsAny(stdout, "No epics found") {
			t.Fatalf("expected no epics for done filter, got: %s", stdout)
		}
	})

	t.Run("epics table", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "board", "epics", "--board", "42", "--limit", "1")
		if !containsAny(stdout, "PROJ-10", "Epic One") {
			t.Fatalf("expected epics, got: %s", stdout)
		}
	})

	t.Run("epics json", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--json", "board", "epics", "--board", "42")
		if !strings.Contains(stdout, "PROJ-10") {
			t.Fatalf("expected JSON epics, got: %s", stdout)
		}
	})

	t.Run("sprints missing board", func(t *testing.T) {
		runRootExpectSilentClean(t, ExitBadArgs, "board", "sprints")
	})

	t.Run("sprints table", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "board", "sprints", "--board", "42", "--state", "active")
		if !containsAny(stdout, "Sprint Active") {
			t.Fatalf("expected sprints, got: %s", stdout)
		}
	})

	t.Run("sprints empty", func(t *testing.T) {
		mockJiraServer(t, emptyDomainHandler)
		stdout, _ := runRootOKCleanDomain(t, "board", "sprints", "--board", "42")
		if !containsAny(stdout, "No sprints found") {
			t.Fatalf("expected empty sprints, got: %s", stdout)
		}
	})

	t.Run("sprints json", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--json", "board", "sprints", "--board", "42")
		if !strings.Contains(stdout, "active") {
			t.Fatalf("expected JSON sprints, got: %s", stdout)
		}
	})
}

// ─── Project ─────────────────────────────────────────────────────────────────

func TestProjectCommands(t *testing.T) {
	mockJiraServer(t, domainMockHandler)

	t.Run("list table", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "project", "list")
		if !containsAny(stdout, "PROJ", "Alice") {
			t.Fatalf("expected projects, got: %s", stdout)
		}
	})

	t.Run("list type filter", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "project", "list", "--type", "software")
		if !containsAny(stdout, "PROJ") || containsAny(stdout, "OPS") {
			t.Fatalf("expected filtered projects, got: %s", stdout)
		}
	})

	t.Run("list empty", func(t *testing.T) {
		mockJiraServer(t, emptyDomainHandler)
		stdout, _ := runRootOKCleanDomain(t, "project", "list")
		if !containsAny(stdout, "No projects found") {
			t.Fatalf("expected empty message, got: %s", stdout)
		}
	})

	t.Run("list json", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--json", "project", "list")
		if !strings.Contains(stdout, "PROJ") {
			t.Fatalf("expected JSON projects, got: %s", stdout)
		}
	})

	t.Run("list query filter", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "project", "list", "--query", "ops")
		if !strings.Contains(stdout, "OPS") || strings.Contains(stdout, "PROJ") {
			t.Fatalf("expected only OPS by query, got: %s", stdout)
		}
	})

	t.Run("list limit caps with warning", func(t *testing.T) {
		stdout, stderr := runRootOKCleanDomain(t, "project", "list", "--limit", "1")
		if !strings.Contains(stderr, "Showing 1 of 2") {
			t.Fatalf("expected cap warning on stderr, got: %s", stderr)
		}
		if strings.Contains(stdout, "OPS") {
			t.Fatalf("expected OPS omitted after --limit 1, got: %s", stdout)
		}
	})

	t.Run("list limit json keeps stdout clean", func(t *testing.T) {
		stdout, stderr := runRootOKCleanDomain(t, "--json", "project", "list", "--limit", "1")
		// The cap warning must be on stderr only; stdout must stay a parseable envelope.
		if !strings.Contains(stderr, "Showing 1 of 2") {
			t.Fatalf("expected cap warning on stderr, got: %s", stderr)
		}
		if strings.Contains(stdout, "Showing 1 of 2") {
			t.Fatalf("warning leaked into stdout JSON: %s", stdout)
		}
		var env map[string]any
		if err := json.Unmarshal([]byte(stdout), &env); err != nil {
			t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout)
		}
		if env["ok"] != true {
			t.Fatalf("expected ok:true envelope, got: %s", stdout)
		}
		data, ok := env["data"].([]any)
		if !ok || len(data) != 1 {
			t.Fatalf("expected 1 project in data, got: %s", stdout)
		}
	})

	t.Run("list query no match", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "project", "list", "--query", "zzznope")
		if !containsAny(stdout, "No projects found") {
			t.Fatalf("expected empty message for no-match query, got: %s", stdout)
		}
	})

	t.Run("list negative limit rejected", func(t *testing.T) {
		runRootExpectSilentClean(t, ExitBadArgs, "project", "list", "--limit", "-1")
	})

	t.Run("get table", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "project", "get", "PROJ")
		if !containsAny(stdout, "My Project", "Alice", "A test project") {
			t.Fatalf("expected project details, got: %s", stdout)
		}
	})

	t.Run("get json", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--json", "project", "get", "PROJ")
		if !strings.Contains(stdout, "A test project") {
			t.Fatalf("expected JSON project, got: %s", stdout)
		}
	})

	t.Run("components table", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "project", "components", "PROJ")
		if !containsAny(stdout, "Backend", "Frontend") {
			t.Fatalf("expected components, got: %s", stdout)
		}
	})

	t.Run("components empty", func(t *testing.T) {
		mockJiraServer(t, emptyDomainHandler)
		stdout, _ := runRootOKCleanDomain(t, "project", "components", "PROJ")
		if !containsAny(stdout, "No components found") {
			t.Fatalf("expected empty components, got: %s", stdout)
		}
	})

	t.Run("components json", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--json", "project", "components", "PROJ")
		if !strings.Contains(stdout, "Backend") {
			t.Fatalf("expected JSON components, got: %s", stdout)
		}
	})

	t.Run("versions table", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "project", "versions", "PROJ")
		if !containsAny(stdout, "v1.0", "v2.0") {
			t.Fatalf("expected versions, got: %s", stdout)
		}
	})

	t.Run("versions released", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "project", "versions", "PROJ", "--released")
		if !containsAny(stdout, "v1.0") || containsAny(stdout, "v2.0") {
			t.Fatalf("expected released only, got: %s", stdout)
		}
	})

	t.Run("versions unreleased", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "project", "versions", "PROJ", "--unreleased")
		if !containsAny(stdout, "v2.0") || containsAny(stdout, "v1.0") {
			t.Fatalf("expected unreleased only, got: %s", stdout)
		}
	})

	t.Run("versions empty", func(t *testing.T) {
		mockJiraServer(t, emptyDomainHandler)
		stdout, _ := runRootOKCleanDomain(t, "project", "versions", "PROJ")
		if !containsAny(stdout, "No versions found") {
			t.Fatalf("expected empty versions, got: %s", stdout)
		}
	})

	t.Run("issue-types table", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "project", "issue-types", "PROJ")
		if !containsAny(stdout, "Story", "Sub-task") {
			t.Fatalf("expected issue types, got: %s", stdout)
		}
	})

	t.Run("issue-types json", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--json", "project", "issue-types", "PROJ")
		if !strings.Contains(stdout, "subtask") {
			t.Fatalf("expected JSON issue types, got: %s", stdout)
		}
	})

	t.Run("fields all", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "project", "fields")
		if !containsAny(stdout, "Summary", "Custom Field") {
			t.Fatalf("expected fields, got: %s", stdout)
		}
	})

	t.Run("fields custom", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "project", "fields", "--custom")
		if !containsAny(stdout, "Custom Field") || containsAny(stdout, "Summary") {
			t.Fatalf("expected custom fields only, got: %s", stdout)
		}
	})

	t.Run("fields empty", func(t *testing.T) {
		mockJiraServer(t, emptyDomainHandler)
		stdout, _ := runRootOKCleanDomain(t, "project", "fields")
		if !containsAny(stdout, "No fields found") {
			t.Fatalf("expected empty fields, got: %s", stdout)
		}
	})

	t.Run("fields json", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--json", "project", "fields")
		if !strings.Contains(stdout, "custom") {
			t.Fatalf("expected JSON fields, got: %s", stdout)
		}
	})
}

// ─── Filter ─────────────────────────────────────────────────────────────────

func TestFilterCommands(t *testing.T) {
	mockJiraServer(t, domainMockHandler)

	t.Run("list table", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "filter", "list")
		if !containsAny(stdout, "My Bugs", "Long JQL") {
			t.Fatalf("expected filters, got: %s", stdout)
		}
	})

	t.Run("list empty", func(t *testing.T) {
		mockJiraServer(t, emptyDomainHandler)
		stdout, _ := runRootOKCleanDomain(t, "filter", "list")
		if !containsAny(stdout, "No filters found") {
			t.Fatalf("expected empty filters, got: %s", stdout)
		}
	})

	t.Run("list json", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--json", "filter", "list")
		if !strings.Contains(stdout, "favourite") {
			t.Fatalf("expected JSON filters, got: %s", stdout)
		}
	})

	t.Run("get table", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "filter", "get", "100")
		if !containsAny(stdout, "My Bugs", "All bugs") {
			t.Fatalf("expected filter details, got: %s", stdout)
		}
	})

	t.Run("get json", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--json", "filter", "get", "100")
		if !strings.Contains(stdout, "type = Bug") {
			t.Fatalf("expected JSON filter, got: %s", stdout)
		}
	})

	t.Run("create missing args", func(t *testing.T) {
		runRootExpectSilentClean(t, ExitBadArgs, "filter", "create", "--name", "X")
	})

	t.Run("create success", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "filter", "create", "--name", "New", "--jql", "project = PROJ", "--description", "desc")
		if !containsAny(stdout, "Created filter") {
			t.Fatalf("expected create success, got: %s", stdout)
		}
	})

	t.Run("create json", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--json", "filter", "create", "--name", "New", "--jql", "project = PROJ")
		if !strings.Contains(stdout, "200") {
			t.Fatalf("expected JSON filter, got: %s", stdout)
		}
	})

	t.Run("delete gate blocks without --dangerous", func(t *testing.T) {
		runRootExpectSilentClean(t, ExitConfirmRequired, "filter", "delete", "100")
	})

	t.Run("delete success", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--dangerous", "filter", "delete", "100")
		if !containsAny(stdout, "deleted") {
			t.Fatalf("expected delete success, got: %s", stdout)
		}
	})

	t.Run("run table", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "filter", "run", "100")
		if !containsAny(stdout, "P-1", "Showing") {
			t.Fatalf("expected filter run results, got: %s", stdout)
		}
	})

	t.Run("run empty", func(t *testing.T) {
		mockJiraServer(t, emptyDomainHandler)
		stdout, _ := runRootOKCleanDomain(t, "filter", "run", "100")
		if !containsAny(stdout, "No issues found") {
			t.Fatalf("expected empty run results, got: %s", stdout)
		}
	})

	t.Run("run json flat", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--json", "filter", "run", "100")
		if !strings.Contains(stdout, `"total"`) || !strings.Contains(stdout, `"issues"`) {
			t.Fatalf("expected flat JSON search result, got: %s", stdout)
		}
	})

	t.Run("run json raw", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--json", "filter", "run", "100", "--raw")
		if !strings.Contains(stdout, `"maxResults"`) {
			t.Fatalf("expected raw JSON, got: %s", stdout)
		}
	})

	t.Run("run json fields", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--json", "filter", "run", "100", "--fields", "key")
		if !strings.Contains(stdout, "P-20") {
			t.Fatalf("expected filtered JSON, got: %s", stdout)
		}
	})
}

// ─── Epic ─────────────────────────────────────────────────────────────────────

func TestEpicCommands(t *testing.T) {
	mockJiraServer(t, domainMockHandler)

	t.Run("list missing board", func(t *testing.T) {
		runRootExpectSilentClean(t, ExitBadArgs, "epic", "list")
	})

	t.Run("list table", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "epic", "list", "--board", "42")
		if !containsAny(stdout, "PROJ-10") {
			t.Fatalf("expected epics, got: %s", stdout)
		}
	})

	t.Run("list empty", func(t *testing.T) {
		mockJiraServer(t, emptyDomainHandler)
		stdout, _ := runRootOKCleanDomain(t, "epic", "list", "--board", "42")
		if !containsAny(stdout, "No epics found") {
			t.Fatalf("expected empty epics, got: %s", stdout)
		}
	})

	t.Run("list json", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--json", "epic", "list", "--board", "42", "--done")
		if !strings.Contains(stdout, `[`) {
			t.Fatalf("expected JSON epics, got: %s", stdout)
		}
	})

	t.Run("issues missing board", func(t *testing.T) {
		runRootExpectSilentClean(t, ExitBadArgs, "epic", "issues", "PROJ-10")
	})

	t.Run("issues agile path", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "epic", "issues", "PROJ-10", "--board", "42")
		if !containsAny(stdout, "P-11", "epic child") {
			t.Fatalf("expected epic issues via agile API, got: %s", stdout)
		}
	})

	t.Run("issues agile json", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--json", "epic", "issues", "PROJ-10", "--board", "42")
		if !strings.Contains(stdout, "P-11") {
			t.Fatalf("expected JSON epic issues, got: %s", stdout)
		}
	})

	t.Run("issues jql fallback", func(t *testing.T) {
		mockJiraServer(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/epic") && !strings.Contains(r.URL.Path, "/issue") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":50,"total":0,"isLast":true,"values":[]}`)
				return
			}
			domainMockHandler(w, r)
		})
		stdout, _ := runRootOKCleanDomain(t, "epic", "issues", "PROJ-99", "--board", "42")
		if !containsAny(stdout, "P-20", "jql epic issue") {
			t.Fatalf("expected JQL fallback issues, got: %s", stdout)
		}
	})

	t.Run("issues empty agile", func(t *testing.T) {
		mockJiraServer(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/epic/") && strings.HasSuffix(r.URL.Path, "/issue") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":50,"total":0,"isLast":true,"issues":[]}`)
				return
			}
			domainMockHandler(w, r)
		})
		stdout, _ := runRootOKCleanDomain(t, "epic", "issues", "PROJ-10", "--board", "42")
		if !containsAny(stdout, "No issues found in this epic") {
			t.Fatalf("expected empty epic issues, got: %s", stdout)
		}
	})
}

// ─── User ─────────────────────────────────────────────────────────────────────

func TestUserCommands(t *testing.T) {
	mockJiraServer(t, domainMockHandler)

	t.Run("me table", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "user", "me")
		if !containsAny(stdout, "currentuser", "Current User") {
			t.Fatalf("expected current user, got: %s", stdout)
		}
	})

	t.Run("me json", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--json", "user", "me")
		if !strings.Contains(stdout, "currentuser") {
			t.Fatalf("expected JSON user, got: %s", stdout)
		}
	})

	t.Run("search missing query", func(t *testing.T) {
		runRootExpectSilentClean(t, ExitBadArgs, "user", "search")
	})

	t.Run("search table", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "user", "search", "--query", "john")
		if !containsAny(stdout, "John Doe", "johndoe") {
			t.Fatalf("expected users, got: %s", stdout)
		}
	})

	t.Run("search assignable", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "user", "search", "--query", "jane", "--assignable", "--project", "PROJ")
		if !containsAny(stdout, "Jane Doe") {
			t.Fatalf("expected assignable users, got: %s", stdout)
		}
	})

	t.Run("search empty", func(t *testing.T) {
		mockJiraServer(t, emptyDomainHandler)
		stdout, _ := runRootOKCleanDomain(t, "user", "search", "--query", "nobody")
		if !containsAny(stdout, "No users found") {
			t.Fatalf("expected empty users, got: %s", stdout)
		}
	})

	t.Run("search json", func(t *testing.T) {
		stdout, _ := runRootOKCleanDomain(t, "--json", "user", "search", "--query", "john")
		if !strings.Contains(stdout, "John Doe") {
			t.Fatalf("expected JSON users, got: %s", stdout)
		}
	})
}
