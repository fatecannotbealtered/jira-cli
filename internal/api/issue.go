package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

// parseTimeSpent parses a time string (e.g. "1w2d3h30m", "2h", "30m", "1d") into seconds.
// Supports w (weeks), d (days), h (hours), m (minutes).
func parseTimeSpent(timeStr string) (int, error) {
	timeStr = strings.TrimSpace(timeStr)
	if timeStr == "" {
		return 0, fmt.Errorf("empty time string")
	}

	var weeks, days, hours, minutes int
	remaining := timeStr

	if wIdx := strings.Index(remaining, "w"); wIdx >= 0 {
		wPart := remaining[:wIdx]
		if _, err := fmt.Sscanf(wPart, "%d", &weeks); err != nil {
			return 0, fmt.Errorf("invalid weeks in %q: %w", timeStr, err)
		}
		remaining = remaining[wIdx+1:]
	}

	if dIdx := strings.Index(remaining, "d"); dIdx >= 0 {
		dPart := remaining[:dIdx]
		if _, err := fmt.Sscanf(dPart, "%d", &days); err != nil {
			return 0, fmt.Errorf("invalid days in %q: %w", timeStr, err)
		}
		remaining = remaining[dIdx+1:]
	}

	if hIdx := strings.Index(remaining, "h"); hIdx >= 0 {
		hPart := remaining[:hIdx]
		if _, err := fmt.Sscanf(hPart, "%d", &hours); err != nil {
			return 0, fmt.Errorf("invalid hours in %q: %w", timeStr, err)
		}
		remaining = remaining[hIdx+1:]
	}

	if mIdx := strings.Index(remaining, "m"); mIdx >= 0 {
		mPart := remaining[:mIdx]
		if _, err := fmt.Sscanf(mPart, "%d", &minutes); err != nil {
			return 0, fmt.Errorf("invalid minutes in %q: %w", timeStr, err)
		}
	}

	total := weeks*5*8*3600 + days*8*3600 + hours*3600 + minutes*60
	if total == 0 {
		return 0, fmt.Errorf("could not parse time string %q", timeStr)
	}

	return total, nil
}

// Get retrieves issue details.
func (a *IssueAPI) Get(key string, expand []string) (*Issue, error) {
	path := a.client.restPath(fmt.Sprintf("/issue/%s", url.PathEscape(key)))
	if len(expand) > 0 {
		path += "?expand=" + url.QueryEscape(strings.Join(expand, ","))
	}
	data, err := a.client.Get(path)
	if err != nil {
		return nil, err
	}
	var issue Issue
	if err := json.Unmarshal(data, &issue); err != nil {
		return nil, fmt.Errorf("parsing issue: %w", err)
	}
	return &issue, nil
}

// List lists issues based on the given options (internally builds JQL).
func (a *IssueAPI) List(opts IssueListOptions) ([]Issue, error) {
	parts := []string{fmt.Sprintf("project = %q", opts.Project)}
	if opts.Status != "" {
		parts = append(parts, fmt.Sprintf("status = %q", opts.Status))
	}
	if opts.Assignee != "" {
		if opts.Assignee == "me" {
			parts = append(parts, "assignee = currentUser()")
		} else {
			parts = append(parts, fmt.Sprintf("assignee = %q", opts.Assignee))
		}
	}
	if opts.Type != "" {
		parts = append(parts, fmt.Sprintf("issuetype = %q", opts.Type))
	}
	if opts.Label != "" {
		parts = append(parts, fmt.Sprintf("labels = %q", opts.Label))
	}
	if opts.Priority != "" {
		parts = append(parts, fmt.Sprintf("priority = %q", opts.Priority))
	}
	jql := strings.Join(parts, " AND ")
	if opts.OrderBy != "" {
		jql += " ORDER BY " + opts.OrderBy
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	result, err := a.Search(jql, SearchOptions{MaxResults: limit})
	if err != nil {
		return nil, err
	}
	return result.Issues, nil
}

// Create creates a new issue.
func (a *IssueAPI) Create(req CreateIssueRequest) (*IssueRef, error) {
	// Jira Data Center / Server REST API v2 takes a plain-string description;
	// ADF (the doc-node object) is Cloud/API-v3 only and v2 rejects it with
	// "description: must be a string". jira-cli targets Data Center, so the
	// string is passed through unchanged.
	data, err := a.client.Post("/rest/api/2/issue", req)
	if err != nil {
		return nil, err
	}
	var ref IssueRef
	if err := json.Unmarshal(data, &ref); err != nil {
		return nil, fmt.Errorf("parsing create response: %w", err)
	}
	if ref.Key == "" {
		return nil, fmt.Errorf("parsing create response: missing issue key")
	}
	return &ref, nil
}

// Edit updates issue fields.
func (a *IssueAPI) Edit(key string, req EditIssueRequest) error {
	// API v2 (Data Center/Server) takes a plain-string description, not ADF.
	path := a.client.restPath(fmt.Sprintf("/issue/%s", url.PathEscape(key)))
	_, err := a.client.Put(path, req)
	return err
}

// EditRaw updates issue fields using a raw map (for custom fields).
func (a *IssueAPI) EditRaw(key string, body map[string]any) error {
	path := a.client.restPath(fmt.Sprintf("/issue/%s", url.PathEscape(key)))
	_, err := a.client.Put(path, body)
	return err
}

// Delete deletes an issue.
func (a *IssueAPI) Delete(key string) error {
	path := a.client.restPath(fmt.Sprintf("/issue/%s", url.PathEscape(key)))
	return a.client.Delete(path)
}

// GetTransitions retrieves the list of available status transitions.
func (a *IssueAPI) GetTransitions(key string) ([]Transition, error) {
	path := a.client.restPath(fmt.Sprintf("/issue/%s/transitions", url.PathEscape(key)))
	data, err := a.client.Get(path)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Transitions []Transition `json:"transitions"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing transitions: %w", err)
	}
	return resp.Transitions, nil
}

// DoTransition performs a status transition.
func (a *IssueAPI) DoTransition(key, transitionID string) error {
	path := a.client.restPath(fmt.Sprintf("/issue/%s/transitions", url.PathEscape(key)))
	req := TransitionRequest{}
	req.Transition.ID = transitionID
	_, err := a.client.Post(path, req)
	return err
}

// Assign assigns an issue to a user by username (empty username clears the assignee).
func (a *IssueAPI) Assign(key, username string) error {
	path := a.client.restPath(fmt.Sprintf("/issue/%s/assignee", url.PathEscape(key)))
	var body any
	if username == "" {
		body = map[string]any{"name": nil}
	} else {
		body = map[string]any{"name": username}
	}
	_, err := a.client.Put(path, body)
	return err
}

// AddWatcher adds a watcher.
func (a *IssueAPI) AddWatcher(key, username string) error {
	path := a.client.restPath(fmt.Sprintf("/issue/%s/watchers", url.PathEscape(key)))
	_, err := a.client.Post(path, username)
	return err
}

// RemoveWatcher removes a watcher.
func (a *IssueAPI) RemoveWatcher(key, username string) error {
	path := a.client.restPath(fmt.Sprintf("/issue/%s/watchers?username=%s",
		url.PathEscape(key), url.QueryEscape(username)))
	return a.client.Delete(path)
}

// GetWatchers retrieves the watcher list.
func (a *IssueAPI) GetWatchers(key string) ([]User, error) {
	path := a.client.restPath(fmt.Sprintf("/issue/%s/watchers", url.PathEscape(key)))
	data, err := a.client.Get(path)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Watchers []User `json:"watchers"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing watchers: %w", err)
	}
	return resp.Watchers, nil
}

// AddVote adds a vote.
func (a *IssueAPI) AddVote(key string) error {
	path := a.client.restPath(fmt.Sprintf("/issue/%s/votes", url.PathEscape(key)))
	_, err := a.client.Post(path, nil)
	return err
}

// RemoveVote removes a vote.
func (a *IssueAPI) RemoveVote(key string) error {
	path := a.client.restPath(fmt.Sprintf("/issue/%s/votes", url.PathEscape(key)))
	return a.client.Delete(path)
}

// AddComment adds a comment.
func (a *IssueAPI) AddComment(key, body string) (*Comment, error) {
	path := a.client.restPath(fmt.Sprintf("/issue/%s/comment", url.PathEscape(key)))
	req := map[string]any{"body": body}
	data, err := a.client.Post(path, req)
	if err != nil {
		return nil, err
	}
	var comment Comment
	if err := json.Unmarshal(data, &comment); err != nil {
		return nil, fmt.Errorf("parsing comment: %w", err)
	}
	return &comment, nil
}

// ListComments lists comments.
func (a *IssueAPI) ListComments(key string, limit int) ([]Comment, error) {
	if limit <= 0 {
		limit = 50
	}
	path := a.client.restPath(fmt.Sprintf("/issue/%s/comment?maxResults=%d&orderBy=-created",
		url.PathEscape(key), limit))
	data, err := a.client.Get(path)
	if err != nil {
		return nil, err
	}
	var page CommentPage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("parsing comments: %w", err)
	}
	return page.Comments, nil
}

// DeleteComment deletes a comment.
func (a *IssueAPI) DeleteComment(issueKey, commentID string) error {
	path := a.client.restPath(fmt.Sprintf("/issue/%s/comment/%s",
		url.PathEscape(issueKey), url.PathEscape(commentID)))
	return a.client.Delete(path)
}

// AddWorklog adds a worklog entry.
func (a *IssueAPI) AddWorklog(key, timeStr, started, comment string) (*Worklog, error) {
	seconds, err := parseTimeSpent(timeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid time format %q: %w", timeStr, err)
	}
	if started == "" {
		started = time.Now().UTC().Format("2006-01-02T15:04:05.000+0000")
	}
	req := WorklogRequest{
		TimeSpentSeconds: seconds,
		Started:          started,
	}
	if comment != "" {
		req.Comment = comment
	}
	path := a.client.restPath(fmt.Sprintf("/issue/%s/worklog", url.PathEscape(key)))
	data, err := a.client.Post(path, req)
	if err != nil {
		return nil, err
	}
	var worklog Worklog
	if err := json.Unmarshal(data, &worklog); err != nil {
		return nil, fmt.Errorf("parsing worklog: %w", err)
	}
	return &worklog, nil
}

// ListWorklogs lists worklog entries.
func (a *IssueAPI) ListWorklogs(key string) ([]Worklog, error) {
	const pageSize = 100
	var allWorklogs []Worklog
	startAt := 0
	for {
		path := a.client.restPath(fmt.Sprintf("/issue/%s/worklog?startAt=%d&maxResults=%d",
			url.PathEscape(key), startAt, pageSize))
		data, err := a.client.Get(path)
		if err != nil {
			return nil, err
		}
		var page WorklogPage
		if err := json.Unmarshal(data, &page); err != nil {
			return nil, fmt.Errorf("parsing worklogs: %w", err)
		}
		allWorklogs = append(allWorklogs, page.Worklogs...)
		if len(page.Worklogs) == 0 || startAt+len(page.Worklogs) >= page.Total {
			break
		}
		startAt += len(page.Worklogs)
	}
	return allWorklogs, nil
}

// GetLinks retrieves only the issuelinks field of an issue. Used by the
// first-class `issue link list` command so callers don't have to parse the full
// `issue get` payload just to inspect links.
func (a *IssueAPI) GetLinks(key string) ([]IssueLink, error) {
	path := a.client.restPath(fmt.Sprintf("/issue/%s?fields=issuelinks", url.PathEscape(key)))
	data, err := a.client.Get(path)
	if err != nil {
		return nil, err
	}
	var issue Issue
	if err := json.Unmarshal(data, &issue); err != nil {
		return nil, fmt.Errorf("parsing issue links: %w", err)
	}
	return issue.Fields.IssueLinks, nil
}

// CreateLink creates an issue link.
func (a *IssueAPI) CreateLink(req IssueLinkRequest) error {
	_, err := a.client.Post(a.client.restPath("/issueLink"), req)
	return err
}

// DeleteLink deletes an issue link.
func (a *IssueAPI) DeleteLink(linkID string) error {
	path := a.client.restPath(fmt.Sprintf("/issueLink/%s", url.PathEscape(linkID)))
	return a.client.Delete(path)
}

// GetLinkTypes retrieves all issue link types.
func (a *IssueAPI) GetLinkTypes() ([]LinkType, error) {
	data, err := a.client.Get(a.client.restPath("/issueLinkType"))
	if err != nil {
		return nil, err
	}
	var resp struct {
		IssueLinkTypes []LinkType `json:"issueLinkTypes"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing link types: %w", err)
	}
	return resp.IssueLinkTypes, nil
}

// AddRemoteLink adds a remote link.
func (a *IssueAPI) AddRemoteLink(key string, req RemoteLinkRequest) (*RemoteLink, error) {
	path := a.client.restPath(fmt.Sprintf("/issue/%s/remotelink", url.PathEscape(key)))
	data, err := a.client.Post(path, req)
	if err != nil {
		return nil, err
	}
	var link RemoteLink
	if err := json.Unmarshal(data, &link); err != nil {
		return nil, fmt.Errorf("parsing remote link: %w", err)
	}
	return &link, nil
}

// ListRemoteLinks lists remote links.
func (a *IssueAPI) ListRemoteLinks(key string) ([]RemoteLink, error) {
	path := a.client.restPath(fmt.Sprintf("/issue/%s/remotelink", url.PathEscape(key)))
	data, err := a.client.Get(path)
	if err != nil {
		return nil, err
	}
	var links []RemoteLink
	if err := json.Unmarshal(data, &links); err != nil {
		return nil, fmt.Errorf("parsing remote links: %w", err)
	}
	return links, nil
}

// UploadAttachment uploads an attachment.
func (a *IssueAPI) UploadAttachment(ctx context.Context, key, filePath string) ([]Attachment, error) {
	path := a.client.restPath(fmt.Sprintf("/issue/%s/attachments", url.PathEscape(key)))
	data, err := a.client.Upload(ctx, path, filePath)
	if err != nil {
		return nil, err
	}
	var attachments []Attachment
	if err := json.Unmarshal(data, &attachments); err != nil {
		return nil, fmt.Errorf("parsing attachments: %w", err)
	}
	return attachments, nil
}

// ListAttachments lists attachments.
func (a *IssueAPI) ListAttachments(key string) ([]Attachment, error) {
	issue, err := a.Get(key, nil)
	if err != nil {
		return nil, err
	}
	return issue.Fields.Attachments, nil
}

// DownloadAttachment downloads one attachment's content into dstDir and returns
// the saved file path. The server-provided filename is sanitised to its base
// name to prevent path traversal.
func (a *IssueAPI) DownloadAttachment(ctx context.Context, att Attachment, dstDir string) (string, error) {
	if att.Content == "" {
		return "", fmt.Errorf("attachment %s (%s) has no content URL", att.ID, att.Filename)
	}
	name := filepath.Base(att.Filename)
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = att.ID
	}
	dst := filepath.Join(dstDir, name)
	if err := a.client.Download(ctx, att.Content, dst); err != nil {
		return "", err
	}
	return dst, nil
}

// Search executes a JQL search (single page).
func (a *IssueAPI) Search(jql string, opts SearchOptions) (*SearchResult, error) {
	if opts.MaxResults <= 0 {
		opts.MaxResults = 50
	}
	body := map[string]any{
		"jql":        jql,
		"startAt":    opts.StartAt,
		"maxResults": opts.MaxResults,
	}
	if len(opts.Fields) > 0 {
		body["fields"] = opts.Fields
	}
	data, err := a.client.Post("/rest/api/2/search", body)
	if err != nil {
		return nil, err
	}
	var result SearchResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing search result: %w", err)
	}
	return &result, nil
}

// SearchAll executes a JQL search with automatic pagination.
// It caps at 10,000 issues to prevent unbounded memory usage.
// SearchAllCap is the maximum number of issues SearchAll will accumulate before
// stopping. When the cap is hit with more results available, SearchAll reports
// truncated=true so callers can surface it instead of silently dropping rows.
const SearchAllCap = 10000

// SearchAll pages through all results for jql, up to SearchAllCap. The returned
// bool reports whether the result was truncated at the cap (more matched than
// were returned).
func (a *IssueAPI) SearchAll(jql string, fields []string) ([]Issue, bool, error) {
	const pageSize = 100
	var allIssues []Issue
	truncated := false
	startAt := 0
	for {
		result, err := a.Search(jql, SearchOptions{
			StartAt:    startAt,
			MaxResults: pageSize,
			Fields:     fields,
		})
		if err != nil {
			return nil, false, err
		}
		allIssues = append(allIssues, result.Issues...)
		if len(result.Issues) == 0 || startAt+len(result.Issues) >= result.Total {
			break
		}
		if len(allIssues) >= SearchAllCap {
			truncated = true
			break
		}
		startAt += len(result.Issues)
	}
	return allIssues, truncated, nil
}
