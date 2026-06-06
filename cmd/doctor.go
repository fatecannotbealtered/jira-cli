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
	type doctorCheck struct {
		Check   string `json:"check"`
		Status  string `json:"status"`
		Message string `json:"message,omitempty"`
		Fix     string `json:"fix,omitempty"`
	}
	type doctorResult struct {
		Checks      []doctorCheck `json:"checks"`
		Host        string        `json:"host,omitempty"`
		Username    string        `json:"username,omitempty"`
		DisplayName string        `json:"displayName,omitempty"`
		LatencyMs   int64         `json:"latency_ms,omitempty"`
	}
	addCheck := func(result *doctorResult, check, status, msg, fix string) {
		result.Checks = append(result.Checks, doctorCheck{Check: check, Status: status, Message: msg, Fix: fix})
	}

	result := doctorResult{}

	cfg, err := config.Load()
	if err != nil {
		addCheck(&result, "config", "fail", err.Error(), "fix or remove "+config.FilePath())
		if jsonMode {
			output.PrintJSON(result)
		} else {
			output.Error("Reading config: " + err.Error())
		}
		return SilentErr(ExitAuth)
	}

	if cfg.Host == "" || cfg.Token == "" {
		addCheck(&result, "config", "fail", "Jira host/token not configured", "run 'jira-cli login --host <url> --token <pat>' or set JIRA_HOST and JIRA_TOKEN")
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
		return SilentErr(ExitAuth)
	}
	addCheck(&result, "config", "pass", "configuration found", "")
	result.Host = cfg.Host

	client := api.NewClient(cfg)
	start := time.Now()
	myself, err := client.Users.Me()
	latency := time.Since(start).Milliseconds()
	result.LatencyMs = latency

	if err != nil {
		addCheck(&result, "auth", "fail", err.Error(), "check PAT validity, Jira permissions, proxy, or VPN")
		addCheck(&result, "network", "fail", err.Error(), "set HTTP_PROXY/HTTPS_PROXY if required and verify Jira is reachable")
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
		return SilentErr(ExitAuth)
	}

	addCheck(&result, "auth", "pass", "PAT valid", "")
	addCheck(&result, "network", "pass", fmt.Sprintf("connected in %dms", latency), "")
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
