package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// List returns all boards, optionally filtered by project and type.
func (a *BoardAPI) List(project, boardType string) ([]Board, error) {
	path := "/rest/agile/1.0/board"
	params := url.Values{}
	if project != "" {
		params.Set("projectKeyOrId", project)
	}
	if boardType != "" {
		params.Set("type", boardType)
	}
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	data, err := a.client.Get(path)
	if err != nil {
		return nil, err
	}
	var page BoardPage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("parsing boards: %w", err)
	}
	return page.Values, nil
}

// Get returns a single board by ID.
func (a *BoardAPI) Get(boardID int) (*Board, error) {
	path := fmt.Sprintf("/rest/agile/1.0/board/%d", boardID)
	data, err := a.client.Get(path)
	if err != nil {
		return nil, err
	}
	var board Board
	if err := json.Unmarshal(data, &board); err != nil {
		return nil, fmt.Errorf("parsing board: %w", err)
	}
	return &board, nil
}

// GetBacklog returns all issues in the board backlog.
func (a *BoardAPI) GetBacklog(boardID int) ([]Issue, error) {
	path := fmt.Sprintf("/rest/agile/1.0/board/%d/backlog", boardID)
	data, err := a.client.Get(path)
	if err != nil {
		return nil, err
	}
	var result struct {
		Issues []Issue `json:"issues"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing backlog: %w", err)
	}
	return result.Issues, nil
}

// GetEpics returns all epics on a board. If done is true, only completed epics are returned.
func (a *BoardAPI) GetEpics(boardID int, done bool) ([]Issue, error) {
	path := fmt.Sprintf("/rest/agile/1.0/board/%d/epic", boardID)
	if done {
		path += "?done=true"
	}
	data, err := a.client.Get(path)
	if err != nil {
		return nil, err
	}
	var result struct {
		Values []json.RawMessage `json:"values"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing epics: %w", err)
	}
	epics := make([]Issue, 0, len(result.Values))
	for _, raw := range result.Values {
		var issue Issue
		if err := json.Unmarshal(raw, &issue); err == nil {
			epics = append(epics, issue)
			continue
		}
		var agileEpic struct {
			ID      int    `json:"id"`
			Key     string `json:"key"`
			Self    string `json:"self"`
			Name    string `json:"name"`
			Summary string `json:"summary"`
			Done    bool   `json:"done"`
		}
		if err := json.Unmarshal(raw, &agileEpic); err != nil {
			return nil, fmt.Errorf("parsing epics: %w", err)
		}
		statusName := "To Do"
		statusKey := "new"
		if agileEpic.Done {
			statusName = "Done"
			statusKey = "done"
		}
		epics = append(epics, Issue{
			ID:   strconv.Itoa(agileEpic.ID),
			Key:  agileEpic.Key,
			Self: agileEpic.Self,
			Fields: IssueFields{
				Summary:   firstNonEmpty(agileEpic.Summary, agileEpic.Name),
				IssueType: IssueType{Name: "Epic"},
				Status: Status{
					Name: statusName,
					StatusCategory: StatusCategory{
						Key: statusKey,
					},
				},
			},
		})
	}
	return epics, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// GetSprints returns all sprints for a board.
func (a *BoardAPI) GetSprints(boardID int, state string) ([]Sprint, error) {
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
		return nil, fmt.Errorf("parsing board sprints: %w", err)
	}
	return page.Values, nil
}

// GetEpicIssues returns issues belonging to an Agile epic ID.
func (a *BoardAPI) GetEpicIssues(epicID string) ([]Issue, error) {
	path := fmt.Sprintf("/rest/agile/1.0/epic/%s/issue", url.PathEscape(epicID))
	data, err := a.client.Get(path)
	if err != nil {
		return nil, err
	}
	var result struct {
		Issues []Issue `json:"issues"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing epic issues: %w", err)
	}
	return result.Issues, nil
}
