package cmd

import (
	"fmt"
	"time"

	"github.com/fatecannotbealtered/jira-cli/internal/api"
	"github.com/fatecannotbealtered/jira-cli/internal/config"
	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check configuration and connectivity",
	RunE:  runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(_ *cobra.Command, _ []string) error {
	type doctorResult struct {
		ConfigExists bool   `json:"configExists"`
		AuthValid    bool   `json:"authValid"`
		LatencyMs    int64  `json:"latencyMs"`
		Host         string `json:"host,omitempty"`
		Username     string `json:"username,omitempty"`
		DisplayName  string `json:"displayName,omitempty"`
		Error        string `json:"error,omitempty"`
	}

	result := doctorResult{}

	cfg, err := config.Load()
	if err != nil {
		result.Error = err.Error()
		if jsonMode {
			output.PrintJSON(result)
		} else {
			output.Error("Reading config: " + err.Error())
		}
		return ErrSilent
	}

	if cfg.Host == "" || cfg.Token == "" {
		result.ConfigExists = false
		result.Error = "not configured: run 'jira-cli login' or set JIRA_HOST and JIRA_TOKEN"
		if jsonMode {
			output.PrintJSON(result)
		} else {
			fmt.Println()
			output.Bold("  jira-cli Doctor")
			output.Gray("  ────────────────────────────────────────")
			fmt.Println()
			output.Error("Not configured. Run 'jira-cli login' or set JIRA_HOST and JIRA_TOKEN env vars.")
			fmt.Println()
		}
		return ErrSilent
	}
	result.ConfigExists = true
	result.Host = cfg.Host

	client := api.NewClient(cfg)
	start := time.Now()
	myself, err := client.Users.Me()
	latency := time.Since(start).Milliseconds()
	result.LatencyMs = latency

	if err != nil {
		result.AuthValid = false
		result.Error = err.Error()
		if jsonMode {
			output.PrintJSON(result)
		} else {
			fmt.Println()
			output.Bold("  jira-cli Doctor")
			output.Gray("  ────────────────────────────────────────")
			fmt.Println()
			output.Error("Connection failed: " + err.Error())
			fmt.Println()
		}
		return ErrSilent
	}

	result.AuthValid = true
	result.Username = myself.Name
	result.DisplayName = myself.DisplayName

	if jsonMode {
		output.PrintJSON(result)
		return nil
	}

	fmt.Println()
	output.Bold("  jira-cli Doctor")
	output.Gray("  ────────────────────────────────────────")
	fmt.Println()
	output.Success("Config found")
	output.Success("PAT valid (Bearer authentication)")
	output.Success(fmt.Sprintf("Connected to %s", cfg.Host))
	output.Success(fmt.Sprintf("Authenticated as %s (%s)", myself.DisplayName, myself.Name))
	output.Gray(fmt.Sprintf("  Latency: %dms", latency))
	fmt.Println()
	return nil
}
