package api

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// GetReport returns the velocity/completion summary for a sprint. It first tries
// the Greenhopper sprintreport chart (committed vs completed counts + story
// points); if that endpoint is missing or unparseable (it varies across DC
// versions), it falls back to computing committed/completed counts from the
// sprint's issues by status category. boardID is the rapid view the chart is
// scoped to; it is required only for the chart path. The returned report's
// Source field records which path produced the figures.
func (a *SprintAPI) GetReport(sprintID, boardID int) (*SprintReport, error) {
	if boardID > 0 {
		if report, ok := a.reportFromGreenhopper(sprintID, boardID); ok {
			return report, nil
		}
	}
	return a.reportFromIssues(sprintID)
}

// reportFromGreenhopper queries the chart endpoint. It returns ok=false (rather
// than an error) on any failure so the caller can fall back gracefully; a hard
// error only surfaces from the fallback path.
func (a *SprintAPI) reportFromGreenhopper(sprintID, boardID int) (*SprintReport, bool) {
	path := fmt.Sprintf("/rest/greenhopper/1.0/rapid/charts/sprintreport?rapidViewId=%d&sprintId=%d", boardID, sprintID)
	data, err := a.client.Get(path)
	if err != nil {
		return nil, false
	}
	var gh greenhopperSprintReport
	if err := json.Unmarshal(data, &gh); err != nil {
		return nil, false
	}

	completed := len(gh.Contents.CompletedIssues)
	notCompleted := len(gh.Contents.IssuesNotCompletedInCurrentSprint)
	punted := len(gh.Contents.PuntedIssues)
	// Committed = work in the sprint at start = completed + still-open. Punted
	// (removed mid-sprint) issues are reported separately, not as committed.
	committed := completed + notCompleted

	report := &SprintReport{
		SprintID:           sprintID,
		SprintName:         gh.Sprint.Name,
		State:              gh.Sprint.State,
		CommittedIssues:    committed,
		CompletedIssues:    completed,
		NotCompletedIssues: notCompleted,
		PunctedIssues:      punted,
		CompletedPoints:    gh.Contents.CompletedIssuesEstimateSum.Value,
		NotCompletedPoints: gh.Contents.IssuesNotCompletedEstimateSum.Value,
		CommittedPoints:    gh.Contents.CompletedIssuesEstimateSum.Value + gh.Contents.IssuesNotCompletedEstimateSum.Value,
		Source:             "greenhopper",
	}
	return report, true
}

// reportFromIssues derives committed/completed counts from the sprint's issues
// by status category. Story points are left at zero because the sprint-issue
// payload doesn't expose a portable estimate field across DC configs; the
// Source field flags the figures as computed so callers don't over-trust them.
func (a *SprintAPI) reportFromIssues(sprintID int) (*SprintReport, error) {
	sprint, err := a.Get(sprintID)
	if err != nil {
		return nil, err
	}
	issues, err := a.GetIssues(sprintID)
	if err != nil {
		return nil, err
	}
	completed := 0
	for _, issue := range issues {
		if issue.Fields.Status.StatusCategory.Key == "done" {
			completed++
		}
	}
	return &SprintReport{
		SprintID:           sprintID,
		SprintName:         sprint.Name,
		State:              sprint.State,
		CommittedIssues:    len(issues),
		CompletedIssues:    completed,
		NotCompletedIssues: len(issues) - completed,
		Source:             "computed",
	}, nil
}

// List lists all sprints for a given board, with optional state filter (active/future/closed).
func (a *SprintAPI) List(boardID int, state string) ([]Sprint, error) {
	basePath := fmt.Sprintf("/rest/agile/1.0/board/%d/sprint", boardID)
	params := url.Values{}
	if state != "" {
		params.Set("state", state)
	}
	return a.client.fetchAllSprintPages(basePath, params)
}

// Get retrieves a single sprint by ID.
func (a *SprintAPI) Get(sprintID int) (*Sprint, error) {
	path := fmt.Sprintf("/rest/agile/1.0/sprint/%d", sprintID)
	data, err := a.client.Get(path)
	if err != nil {
		return nil, err
	}
	var sprint Sprint
	if err := json.Unmarshal(data, &sprint); err != nil {
		return nil, fmt.Errorf("parsing sprint: %w", err)
	}
	return &sprint, nil
}

// GetIssues retrieves all issues in a given sprint.
func (a *SprintAPI) GetIssues(sprintID int) ([]Issue, error) {
	basePath := fmt.Sprintf("/rest/agile/1.0/sprint/%d/issue", sprintID)
	return a.client.fetchAllIssuePages(basePath, nil)
}

// Create creates a new sprint on the given board.
func (a *SprintAPI) Create(req CreateSprintRequest) (*Sprint, error) {
	data, err := a.client.Post("/rest/agile/1.0/sprint", req)
	if err != nil {
		return nil, err
	}
	var sprint Sprint
	if err := json.Unmarshal(data, &sprint); err != nil {
		return nil, fmt.Errorf("parsing created sprint: %w", err)
	}
	return &sprint, nil
}

// Update updates sprint information.
func (a *SprintAPI) Update(sprintID int, req UpdateSprintRequest) (*Sprint, error) {
	path := fmt.Sprintf("/rest/agile/1.0/sprint/%d", sprintID)
	data, err := a.client.Put(path, req)
	if err != nil {
		return nil, err
	}
	var sprint Sprint
	if err := json.Unmarshal(data, &sprint); err != nil {
		return nil, fmt.Errorf("parsing updated sprint: %w", err)
	}
	return &sprint, nil
}

// Close sets the sprint state to closed.
func (a *SprintAPI) Close(sprintID int) error {
	path := fmt.Sprintf("/rest/agile/1.0/sprint/%d", sprintID)
	req := UpdateSprintRequest{State: "closed"}
	_, err := a.client.Put(path, req)
	return err
}

// MoveIssues moves the specified issues into the given sprint.
func (a *SprintAPI) MoveIssues(sprintID int, issueKeys []string) error {
	path := fmt.Sprintf("/rest/agile/1.0/sprint/%d/issue", sprintID)
	req := MoveIssuesToSprintRequest{Issues: issueKeys}
	_, err := a.client.Post(path, req)
	return err
}
