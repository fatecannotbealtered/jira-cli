package cmd

import "testing"

func TestChangelog_JSON(t *testing.T) {
	stdout, _ := runRootOK(t, "--json", "changelog")
	var data struct {
		CurrentVersion string `json:"current_version"`
		Entries        []struct {
			Version string `json:"version"`
		} `json:"entries"`
	}
	decodeEnvelopeData(t, stdout, &data)
	if len(data.Entries) == 0 {
		t.Fatal("changelog should expose at least one version entry")
	}
}

func TestChangelog_Since(t *testing.T) {
	stdout, _ := runRootOK(t, "--json", "changelog", "--since", "0.0.0")
	var data struct {
		Since   string           `json:"since"`
		Entries []map[string]any `json:"entries"`
	}
	decodeEnvelopeData(t, stdout, &data)
	if len(data.Entries) == 0 {
		t.Fatal("--since 0.0.0 should include every released entry")
	}
}
