package output

import "strings"

// FlatIssue is a flattened representation of a Jira issue for token-efficient output.
type FlatIssue struct {
	Key       string `json:"key"`
	Summary   string `json:"summary"`
	Status    string `json:"status"`
	Type      string `json:"type"`
	Assignee  string `json:"assignee,omitempty"`
	Reporter  string `json:"reporter,omitempty"`
	Priority  string `json:"priority,omitempty"`
	Created   string `json:"created,omitempty"`
	Updated   string `json:"updated,omitempty"`
	Labels    string `json:"labels,omitempty"`
	Component string `json:"component,omitempty"`
	Parent    string `json:"parent,omitempty"`
}

// FlatSprint is a flattened representation of a Jira sprint.
type FlatSprint struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	State     string `json:"state"`
	StartDate string `json:"startDate,omitempty"`
	EndDate   string `json:"endDate,omitempty"`
	Goal      string `json:"goal,omitempty"`
}

// FlatBoard is a flattened representation of a Jira board.
type FlatBoard struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	ProjectKey  string `json:"projectKey,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
}

// FilterFields filters a FlatIssue to only the specified field names.
// Field names are case-insensitive. Returns a map with only the requested fields.
func FilterFields(issue FlatIssue, fieldNames []string) map[string]any {
	m := IssueToMap(issue)
	if len(fieldNames) == 0 {
		return m
	}
	result := make(map[string]any, len(fieldNames))
	for _, name := range fieldNames {
		key := strings.TrimSpace(strings.ToLower(name))
		if v, ok := m[key]; ok {
			result[key] = v
		}
	}
	return result
}

// IssueToMap converts a FlatIssue to a map for field filtering.
func IssueToMap(issue FlatIssue) map[string]any {
	m := map[string]any{
		"key":     issue.Key,
		"summary": issue.Summary,
		"status":  issue.Status,
		"type":    issue.Type,
	}
	if issue.Assignee != "" {
		m["assignee"] = issue.Assignee
	}
	if issue.Reporter != "" {
		m["reporter"] = issue.Reporter
	}
	if issue.Priority != "" {
		m["priority"] = issue.Priority
	}
	if issue.Created != "" {
		m["created"] = issue.Created
	}
	if issue.Updated != "" {
		m["updated"] = issue.Updated
	}
	if issue.Labels != "" {
		m["labels"] = issue.Labels
	}
	if issue.Component != "" {
		m["component"] = issue.Component
	}
	if issue.Parent != "" {
		m["parent"] = issue.Parent
	}
	return m
}

// SprintToMap converts a FlatSprint to a map for field filtering.
func SprintToMap(s FlatSprint) map[string]any {
	m := map[string]any{
		"id":    s.ID,
		"name":  s.Name,
		"state": s.State,
	}
	if s.StartDate != "" {
		m["startDate"] = s.StartDate
	}
	if s.EndDate != "" {
		m["endDate"] = s.EndDate
	}
	if s.Goal != "" {
		m["goal"] = s.Goal
	}
	return m
}

// sprintCanonicalKey maps a normalized (lowercase) sprint field name to its
// canonical output key (camelCase where applicable, matching FlatSprint JSON tags).
func sprintCanonicalKey(normalized string) string {
	switch normalized {
	case "startdate":
		return "startDate"
	case "enddate":
		return "endDate"
	default:
		return normalized
	}
}

// lookupSprintField resolves a requested field name against a SprintToMap result.
// Lookup is case-insensitive; returned keys use canonical camelCase.
func lookupSprintField(m map[string]any, name string) (canonicalKey string, value any, ok bool) {
	normalized := strings.TrimSpace(strings.ToLower(name))
	if normalized == "" {
		return "", nil, false
	}
	canon := sprintCanonicalKey(normalized)
	if v, found := m[canon]; found {
		return canon, v, true
	}
	return "", nil, false
}

// FilterSprintFields filters a FlatSprint to only the specified field names.
// Field names are case-insensitive. Output keys use canonical camelCase.
func FilterSprintFields(s FlatSprint, fieldNames []string) map[string]any {
	m := SprintToMap(s)
	if len(fieldNames) == 0 {
		return m
	}
	result := make(map[string]any, len(fieldNames))
	for _, name := range fieldNames {
		if key, v, ok := lookupSprintField(m, name); ok {
			result[key] = v
		}
	}
	return result
}
