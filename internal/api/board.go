package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// List returns all boards, optionally filtered by project and type.
func (a *BoardAPI) List(project, boardType string) ([]Board, error) {
	params := url.Values{}
	if project != "" {
		params.Set("projectKeyOrId", project)
	}
	if boardType != "" {
		params.Set("type", boardType)
	}
	return a.client.fetchAllBoardPages("/rest/agile/1.0/board", params)
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
	basePath := fmt.Sprintf("/rest/agile/1.0/board/%d/backlog", boardID)
	return a.client.fetchAllIssuePages(basePath, nil)
}

// GetEpics returns all epics on a board. If done is true, only completed epics are returned.
func (a *BoardAPI) GetEpics(boardID int, done bool) ([]Issue, error) {
	basePath := fmt.Sprintf("/rest/agile/1.0/board/%d/epic", boardID)
	params := url.Values{}
	if done {
		params.Set("done", "true")
	}
	rawValues, err := a.client.fetchAllEpicValuePages(basePath, params)
	if err != nil {
		return nil, err
	}
	return parseEpicValues(rawValues)
}

func parseEpicValues(rawValues []json.RawMessage) ([]Issue, error) {
	epics := make([]Issue, 0, len(rawValues))
	for _, raw := range rawValues {
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
	basePath := fmt.Sprintf("/rest/agile/1.0/board/%d/sprint", boardID)
	params := url.Values{}
	if state != "" {
		params.Set("state", state)
	}
	return a.client.fetchAllSprintPages(basePath, params)
}

// GetEpicIssues returns issues belonging to an Agile epic ID.
func (a *BoardAPI) GetEpicIssues(epicID string) ([]Issue, error) {
	basePath := fmt.Sprintf("/rest/agile/1.0/epic/%s/issue", url.PathEscape(epicID))
	return a.client.fetchAllIssuePages(basePath, nil)
}
