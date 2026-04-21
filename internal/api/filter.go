package api

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// ListMy lists all favourite filters for the current user.
func (a *FilterAPI) ListMy() ([]Filter, error) {
	data, err := a.client.Get("/rest/api/2/filter/favourite")
	if err != nil {
		return nil, err
	}
	var filters []Filter
	if err := json.Unmarshal(data, &filters); err != nil {
		return nil, fmt.Errorf("parsing filters: %w", err)
	}
	return filters, nil
}

// Get retrieves a single filter by ID.
func (a *FilterAPI) Get(id string) (*Filter, error) {
	path := a.client.restPath(fmt.Sprintf("/filter/%s", url.PathEscape(id)))
	data, err := a.client.Get(path)
	if err != nil {
		return nil, err
	}
	var filter Filter
	if err := json.Unmarshal(data, &filter); err != nil {
		return nil, fmt.Errorf("parsing filter: %w", err)
	}
	return &filter, nil
}

// Create creates a new filter.
func (a *FilterAPI) Create(name, jql, description string) (*Filter, error) {
	req := CreateFilterRequest{
		Name:        name,
		JQL:         jql,
		Description: description,
	}
	data, err := a.client.Post(a.client.restPath("/filter"), req)
	if err != nil {
		return nil, err
	}
	var filter Filter
	if err := json.Unmarshal(data, &filter); err != nil {
		return nil, fmt.Errorf("parsing created filter: %w", err)
	}
	return &filter, nil
}

// Delete removes a filter by ID.
func (a *FilterAPI) Delete(id string) error {
	path := a.client.restPath(fmt.Sprintf("/filter/%s", url.PathEscape(id)))
	return a.client.Delete(path)
}
