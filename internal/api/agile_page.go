package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const (
	agilePageSize = 100
	agileMaxTotal = 10000
)

func buildAgilePagePath(basePath string, params url.Values, startAt, maxResults int) string {
	u, err := url.Parse(basePath)
	if err != nil {
		q := url.Values{}
		for k, vs := range params {
			for _, v := range vs {
				q.Add(k, v)
			}
		}
		q.Set("startAt", strconv.Itoa(startAt))
		q.Set("maxResults", strconv.Itoa(maxResults))
		sep := "?"
		if strings.Contains(basePath, "?") {
			sep = "&"
		}
		return basePath + sep + q.Encode()
	}
	q := u.Query()
	for k, vs := range params {
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	q.Set("startAt", strconv.Itoa(startAt))
	q.Set("maxResults", strconv.Itoa(maxResults))
	u.RawQuery = q.Encode()
	return u.String()
}

func agilePageFinished(pr PagedResponse, itemCount, maxResults int) bool {
	if pr.IsLast {
		return true
	}
	if itemCount == 0 {
		return true
	}
	if pr.Total > 0 && pr.StartAt+itemCount >= pr.Total {
		return true
	}
	// Responses without total/isLast (e.g. single-page mocks) stop when the page is not full.
	if pr.Total == 0 && itemCount < maxResults {
		return true
	}
	return false
}

func parseAgilePageMeta(data []byte) (PagedResponse, error) {
	var meta PagedResponse
	if err := json.Unmarshal(data, &meta); err != nil {
		return PagedResponse{}, fmt.Errorf("parsing agile page metadata: %w", err)
	}
	return meta, nil
}

// getAgilePageAll fetches all pages from a Jira Agile API endpoint.
// collect receives each page body and returns the number of items appended.
func (c *Client) getAgilePageAll(basePath string, params url.Values, collect func([]byte) (items int, err error)) error {
	startAt := 0
	collected := 0
	for {
		path := buildAgilePagePath(basePath, params, startAt, agilePageSize)
		data, err := c.Get(path)
		if err != nil {
			return err
		}
		meta, err := parseAgilePageMeta(data)
		if err != nil {
			return err
		}
		count, err := collect(data)
		if err != nil {
			return err
		}
		collected += count
		if agilePageFinished(meta, count, agilePageSize) || collected >= agileMaxTotal {
			break
		}
		startAt += count
	}
	return nil
}

func (c *Client) fetchAllSprintPages(basePath string, params url.Values) ([]Sprint, error) {
	var all []Sprint
	err := c.getAgilePageAll(basePath, params, func(data []byte) (int, error) {
		var page SprintPage
		if err := json.Unmarshal(data, &page); err != nil {
			return 0, fmt.Errorf("parsing sprints: %w", err)
		}
		all = append(all, page.Values...)
		return len(page.Values), nil
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}

func (c *Client) fetchAllBoardPages(basePath string, params url.Values) ([]Board, error) {
	var all []Board
	err := c.getAgilePageAll(basePath, params, func(data []byte) (int, error) {
		var page BoardPage
		if err := json.Unmarshal(data, &page); err != nil {
			return 0, fmt.Errorf("parsing boards: %w", err)
		}
		all = append(all, page.Values...)
		return len(page.Values), nil
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}

func (c *Client) fetchAllIssuePages(basePath string, params url.Values) ([]Issue, error) {
	var all []Issue
	err := c.getAgilePageAll(basePath, params, func(data []byte) (int, error) {
		var page SearchResult
		if err := json.Unmarshal(data, &page); err != nil {
			return 0, fmt.Errorf("parsing issues: %w", err)
		}
		all = append(all, page.Issues...)
		return len(page.Issues), nil
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}

func (c *Client) fetchAllEpicValuePages(basePath string, params url.Values) ([]json.RawMessage, error) {
	var all []json.RawMessage
	err := c.getAgilePageAll(basePath, params, func(data []byte) (int, error) {
		var page struct {
			PagedResponse
			Values []json.RawMessage `json:"values"`
		}
		if err := json.Unmarshal(data, &page); err != nil {
			return 0, fmt.Errorf("parsing epics: %w", err)
		}
		all = append(all, page.Values...)
		return len(page.Values), nil
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}
