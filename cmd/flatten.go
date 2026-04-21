package cmd

import (
	"strings"

	"github.com/fatecannotbealtered/jira-cli/internal/api"
	"github.com/fatecannotbealtered/jira-cli/internal/output"
)

// toFlatIssue converts an API Issue to a token-efficient FlatIssue.
func toFlatIssue(issue *api.Issue) output.FlatIssue {
	f := issue.Fields
	fi := output.FlatIssue{
		Key:     issue.Key,
		Summary: f.Summary,
		Status:  f.Status.Name,
		Type:    f.IssueType.Name,
		Created: f.Created,
		Updated: f.Updated,
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

// toFlatSprint converts an API Sprint to a token-efficient FlatSprint.
func toFlatSprint(s *api.Sprint) output.FlatSprint {
	return output.FlatSprint{
		ID:        s.ID,
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
