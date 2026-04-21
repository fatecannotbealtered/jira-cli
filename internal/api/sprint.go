package api

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// List lists all sprints for a given board, with optional state filter (active/future/closed).
func (a *SprintAPI) List(boardID int, state string) ([]Sprint, error) {
	path := fmt.Sprintf("/rest/agile/1.0/board/%d/sprint", boardID)
	if state != "" {
		path += "?state=" + url.QueryEscape(state)
	}
	data, err := a.client.Get(path)
	if err != nil {
		return nil, err
	}
	var page SprintPage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("parsing sprints: %w", err)
	}
	return page.Values, nil
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
	path := fmt.Sprintf("/rest/agile/1.0/sprint/%d/issue", sprintID)
	data, err := a.client.Get(path)
	if err != nil {
		return nil, err
	}
	var result struct {
		Issues []Issue `json:"issues"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing sprint issues: %w", err)
	}
	return result.Issues, nil
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
