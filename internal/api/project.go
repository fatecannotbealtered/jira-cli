package api

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// List lists all projects, optionally filtered by type (client-side).
func (a *ProjectAPI) List(projectType string) ([]Project, error) {
	data, err := a.client.Get("/rest/api/2/project")
	if err != nil {
		return nil, err
	}
	var projects []Project
	if err := json.Unmarshal(data, &projects); err != nil {
		return nil, fmt.Errorf("parsing projects: %w", err)
	}
	if projectType == "" {
		return projects, nil
	}
	out := make([]Project, 0, len(projects))
	for _, p := range projects {
		if p.ProjectTypeKey == projectType {
			out = append(out, p)
		}
	}
	return out, nil
}

// Get retrieves a single project by key.
func (a *ProjectAPI) Get(key string) (*Project, error) {
	path := fmt.Sprintf("/rest/api/2/project/%s", url.PathEscape(key))
	data, err := a.client.Get(path)
	if err != nil {
		return nil, err
	}
	var project Project
	if err := json.Unmarshal(data, &project); err != nil {
		return nil, fmt.Errorf("parsing project: %w", err)
	}
	return &project, nil
}

// GetComponents retrieves all components for a project.
func (a *ProjectAPI) GetComponents(key string) ([]Component, error) {
	path := fmt.Sprintf("/rest/api/2/project/%s/components", url.PathEscape(key))
	data, err := a.client.Get(path)
	if err != nil {
		return nil, err
	}
	var components []Component
	if err := json.Unmarshal(data, &components); err != nil {
		return nil, fmt.Errorf("parsing components: %w", err)
	}
	return components, nil
}

// GetVersions retrieves all versions for a project, with optional released/unreleased filtering.
func (a *ProjectAPI) GetVersions(key string, released, unreleased bool) ([]Version, error) {
	path := fmt.Sprintf("/rest/api/2/project/%s/versions", url.PathEscape(key))
	data, err := a.client.Get(path)
	if err != nil {
		return nil, err
	}
	var versions []Version
	if err := json.Unmarshal(data, &versions); err != nil {
		return nil, fmt.Errorf("parsing versions: %w", err)
	}
	return filterVersionsByReleaseFlags(versions, released, unreleased), nil
}

func filterVersionsByReleaseFlags(versions []Version, released, unreleased bool) []Version {
	if released && !unreleased {
		var filtered []Version
		for _, v := range versions {
			if v.Released {
				filtered = append(filtered, v)
			}
		}
		return filtered
	}
	if unreleased && !released {
		var filtered []Version
		for _, v := range versions {
			if !v.Released {
				filtered = append(filtered, v)
			}
		}
		return filtered
	}
	return versions
}

// GetIssueTypes retrieves all issue types supported by a project.
func (a *ProjectAPI) GetIssueTypes(key string) ([]IssueType, error) {
	project, err := a.Get(key)
	if err != nil {
		return nil, err
	}
	return project.IssueTypes, nil
}
