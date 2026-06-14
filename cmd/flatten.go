package cmd

import (
	"strconv"
	"strings"
	"time"

	"github.com/fatecannotbealtered/jira-cli/internal/api"
	"github.com/fatecannotbealtered/jira-cli/internal/output"
)

// descriptionText renders a description field: DC returns a plain string,
// Cloud/v3 returns an ADF object. Returns "" for nil/other.
func descriptionText(desc any) string {
	switch v := desc.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		return api.ADFToText(v)
	}
}

// toFlatIssue converts an API Issue to a token-efficient FlatIssue.
func toFlatIssue(issue *api.Issue) output.FlatIssue {
	f := issue.Fields
	fi := output.FlatIssue{
		Key:     issue.Key,
		Summary: f.Summary,
		Status:  f.Status.Name,
		Type:    f.IssueType.Name,
		Created: isoUTC(f.Created),
		Updated: isoUTC(f.Updated),
		Untrusted: []string{
			"summary",
		},
	}
	if f.Assignee != nil {
		fi.Assignee = f.Assignee.Name
	}
	if f.Reporter != nil {
		fi.Reporter = f.Reporter.Name
	}
	if f.Priority != nil {
		fi.Priority = f.Priority.Name
	}
	if len(f.Labels) > 0 {
		fi.Labels = strings.Join(f.Labels, ",")
	}
	if len(f.Components) > 0 {
		fi.Component = f.Components[0].Name
	}
	if f.Parent != nil {
		fi.Parent = f.Parent.Key
	}
	return fi
}

// toFlatIssues converts a slice of API Issues to FlatIssues.
func toFlatIssues(issues []api.Issue) []output.FlatIssue {
	result := make([]output.FlatIssue, len(issues))
	for i := range issues {
		result[i] = toFlatIssue(&issues[i])
	}
	return result
}

// printIssueJSON outputs an issue as JSON (flat by default, raw with --raw).
func printIssueJSON(issue *api.Issue, raw bool, fields []string) {
	if raw {
		output.PrintJSON(issue)
		return
	}
	fi := toFlatIssue(issue)
	fi.Description = descriptionText(issue.Fields.Description)
	if fi.Description != "" {
		fi.Untrusted = append(fi.Untrusted, "description")
	}
	if len(fields) > 0 {
		output.PrintJSON(output.FilterFields(fi, fields))
		return
	}
	output.PrintJSON(fi)
}

// printIssuesJSON outputs a list of issues as JSON (flat by default, raw with --raw).
func printIssuesJSON(issues []api.Issue, raw bool, fields []string) {
	if raw {
		output.PrintJSON(issues)
		return
	}
	flat := toFlatIssues(issues)
	if len(fields) > 0 {
		result := make([]map[string]any, len(flat))
		for i := range flat {
			result[i] = output.FilterFields(flat[i], fields)
		}
		output.PrintJSON(result)
		return
	}
	output.PrintJSON(flat)
}

// toFlatIssueLink flattens one API IssueLink into a row. A link points either
// inward or outward; the direction name (e.g. "blocks"/"is blocked by") comes
// from the matching side of the link type. The linked issue's summary is
// external content and tagged _untrusted.
func toFlatIssueLink(link *api.IssueLink) output.FlatIssueLink {
	row := output.FlatIssueLink{ID: link.ID}
	linked := link.OutwardIssue
	if linked != nil {
		row.Direction = "outward"
		row.Type = link.Type.Outward
	} else {
		linked = link.InwardIssue
		row.Direction = "inward"
		row.Type = link.Type.Inward
	}
	// Fall back to the type's base name when a direction phrase is absent.
	if row.Type == "" {
		row.Type = link.Type.Name
	}
	if linked != nil {
		row.LinkedKey = linked.Key
		if linked.Fields != nil {
			row.LinkedSummary = linked.Fields.Summary
			row.LinkedStatus = linked.Fields.Status.Name
		}
	}
	if row.LinkedSummary != "" {
		row.Untrusted = []string{"linkedSummary"}
	}
	return row
}

// printIssueLinksJSON outputs issue links as flattened JSON rows.
func printIssueLinksJSON(links []api.IssueLink) {
	rows := make([]output.FlatIssueLink, len(links))
	for i := range links {
		rows[i] = toFlatIssueLink(&links[i])
	}
	output.PrintJSON(rows)
}

// toFlatSprint converts an API Sprint to a token-efficient FlatSprint.
func toFlatSprint(s *api.Sprint) output.FlatSprint {
	return output.FlatSprint{
		ID:        strconv.Itoa(s.ID),
		Name:      s.Name,
		State:     s.State,
		StartDate: s.StartDate,
		EndDate:   s.EndDate,
		Goal:      s.Goal,
	}
}

// toFlatSprints converts a slice of API Sprints to FlatSprints.
func toFlatSprints(sprints []api.Sprint) []output.FlatSprint {
	result := make([]output.FlatSprint, len(sprints))
	for i := range sprints {
		result[i] = toFlatSprint(&sprints[i])
	}
	return result
}

// printSprintsJSON outputs sprints as JSON (flat by default, raw with --raw).
func printSprintsJSON(sprints []api.Sprint, raw bool, fields []string) {
	if raw {
		output.PrintJSON(sprints)
		return
	}
	flat := toFlatSprints(sprints)
	if len(fields) > 0 {
		result := make([]map[string]any, len(flat))
		for i := range flat {
			result[i] = output.FilterSprintFields(flat[i], fields)
		}
		output.PrintJSON(result)
		return
	}
	output.PrintJSON(flat)
}

// toFlatBoard converts an API Board to a token-efficient FlatBoard.
func toFlatBoard(b *api.Board) output.FlatBoard {
	return output.FlatBoard{
		ID:          strconv.Itoa(b.ID),
		Name:        b.Name,
		Type:        b.Type,
		ProjectKey:  b.Location.ProjectKey,
		DisplayName: b.Location.DisplayName,
	}
}

func toFlatBoards(boards []api.Board) []output.FlatBoard {
	result := make([]output.FlatBoard, len(boards))
	for i := range boards {
		result[i] = toFlatBoard(&boards[i])
	}
	return result
}

func isoUTC(value string) string {
	if value == "" {
		return ""
	}
	layouts := []string{
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02T15:04:05.000Z0700",
		time.RFC3339Nano,
		time.RFC3339,
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC().Format(time.RFC3339)
		}
	}
	return value
}
