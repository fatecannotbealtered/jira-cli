package api

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// Search searches for users by username.
func (a *UserAPI) Search(query string) ([]User, error) {
	path := "/rest/api/2/user/search?username=" + url.QueryEscape(query)
	data, err := a.client.Get(path)
	if err != nil {
		return nil, err
	}
	var users []User
	if err := json.Unmarshal(data, &users); err != nil {
		return nil, fmt.Errorf("parsing users: %w", err)
	}
	return users, nil
}

// SearchAssignable searches for users assignable to a given project.
func (a *UserAPI) SearchAssignable(query, project string) ([]User, error) {
	path := fmt.Sprintf("/rest/api/2/user/assignable/search?project=%s&username=%s",
		url.QueryEscape(project), url.QueryEscape(query))
	data, err := a.client.Get(path)
	if err != nil {
		return nil, err
	}
	var users []User
	if err := json.Unmarshal(data, &users); err != nil {
		return nil, fmt.Errorf("parsing assignable users: %w", err)
	}
	return users, nil
}

// Me retrieves the currently logged-in user's information.
func (a *UserAPI) Me() (*Myself, error) {
	data, err := a.client.Get("/rest/api/2/myself")
	if err != nil {
		return nil, err
	}
	var myself Myself
	if err := json.Unmarshal(data, &myself); err != nil {
		return nil, fmt.Errorf("parsing myself: %w", err)
	}
	return &myself, nil
}
