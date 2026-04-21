package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// ListAll retrieves all fields (system + custom).
func (a *FieldAPI) ListAll() ([]FieldMeta, error) {
	data, err := a.client.Get("/rest/api/2/field")
	if err != nil {
		return nil, err
	}
	var fields []FieldMeta
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, fmt.Errorf("parsing fields: %w", err)
	}
	return fields, nil
}

// ListCustom retrieves only custom fields.
func (a *FieldAPI) ListCustom() ([]FieldMeta, error) {
	all, err := a.ListAll()
	if err != nil {
		return nil, err
	}
	var custom []FieldMeta
	for _, f := range all {
		if f.Custom {
			custom = append(custom, f)
		}
	}
	return custom, nil
}

// ResolveFieldID resolves a field name to its field ID.
// Supports passing a customfield_xxxxx format ID directly.
func (a *FieldAPI) ResolveFieldID(nameOrID string) (string, error) {
	if strings.HasPrefix(nameOrID, "customfield_") {
		return nameOrID, nil
	}

	fields, err := a.ListAll()
	if err != nil {
		return "", err
	}

	// Exact match (case-insensitive)
	for _, f := range fields {
		if strings.EqualFold(f.Name, nameOrID) {
			return f.ID, nil
		}
	}

	// Fuzzy match
	var candidates []FieldMeta
	lower := strings.ToLower(nameOrID)
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f.Name), lower) {
			candidates = append(candidates, f)
		}
	}

	if len(candidates) == 1 {
		return candidates[0].ID, nil
	}
	if len(candidates) > 1 {
		names := make([]string, len(candidates))
		for i, c := range candidates {
			names[i] = fmt.Sprintf("%s (%s)", c.Name, c.ID)
		}
		return "", fmt.Errorf("ambiguous field name %q, matches: %s", nameOrID, strings.Join(names, ", "))
	}

	return "", fmt.Errorf("field %q not found", nameOrID)
}

// SetCustomFields sets custom fields in an issue create/edit request body.
// fields is a slice of "FieldName=Value" pairs.
func (a *FieldAPI) SetCustomFields(body map[string]any, fieldPairs []string) error {
	fieldsMap, ok := body["fields"].(map[string]any)
	if !ok {
		fieldsMap = make(map[string]any)
		body["fields"] = fieldsMap
	}

	for _, pair := range fieldPairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid field format %q, expected 'FieldName=Value'", pair)
		}
		name := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		fieldID, err := a.ResolveFieldID(name)
		if err != nil {
			return err
		}

		var jsonValue any
		if err := json.Unmarshal([]byte(value), &jsonValue); err == nil {
			fieldsMap[fieldID] = jsonValue
		} else {
			fieldsMap[fieldID] = value
		}
	}
	return nil
}

// GetCustomFieldValues extracts custom field values from an issue's raw JSON.
func (a *FieldAPI) GetCustomFieldValues(issueKey string) (map[string]any, error) {
	path := fmt.Sprintf("/rest/api/2/issue/%s", url.PathEscape(issueKey))
	data, err := a.client.Get(path)
	if err != nil {
		return nil, err
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing issue: %w", err)
	}

	fields, _ := raw["fields"].(map[string]any)
	if fields == nil {
		return nil, fmt.Errorf("no fields in issue response")
	}

	allFields, err := a.ListAll()
	if err != nil {
		return nil, err
	}

	nameMap := make(map[string]string)
	for _, f := range allFields {
		if f.Custom {
			nameMap[f.ID] = f.Name
		}
	}

	result := make(map[string]any)
	for id, val := range fields {
		if strings.HasPrefix(id, "customfield_") && val != nil {
			name := nameMap[id]
			if name == "" {
				name = id
			}
			result[name] = val
		}
	}
	return result, nil
}
