package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/fatecannotbealtered/jira-cli/internal/api"
	"github.com/fatecannotbealtered/jira-cli/internal/config"
	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Print current runtime, configuration, and credential status",
	Args:  cobra.NoArgs,
	RunE:  runContext,
}

func init() {
	rootCmd.AddCommand(contextCmd)
}

type contextDocument struct {
	Tool        string             `json:"tool"`
	Version     string             `json:"version"`
	Runtime     contextRuntime     `json:"runtime"`
	Config      contextConfig      `json:"config"`
	Credentials contextCredentials `json:"credentials"`
	Account     *contextAccount    `json:"account,omitempty"`
	Errors      []string           `json:"errors,omitempty"`
	Env         map[string]string  `json:"env"`
	Notices     []updateNotice     `json:"notices,omitempty"`
}

type contextRuntime struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
	CWD  string `json:"cwd,omitempty"`
}

type contextConfig struct {
	ConfigFile   string `json:"config_file"`
	Host         string `json:"host,omitempty"`
	HostSource   string `json:"host_source,omitempty"`
	TokenPresent bool   `json:"token_present"`
	TokenSource  string `json:"token_source,omitempty"`
	Configured   bool   `json:"configured"`
}

type contextCredentials struct {
	Configured bool   `json:"configured"`
	Present    bool   `json:"present"`
	Source     string `json:"source,omitempty"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
}

type contextAccount struct {
	Username    string `json:"username,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Email       string `json:"email,omitempty"`
}

func runContext(_ *cobra.Command, _ []string) error {
	cwd, _ := os.Getwd()
	doc := contextDocument{
		Tool:        "jira-cli",
		Version:     version,
		Runtime:     contextRuntime{OS: runtime.GOOS, Arch: runtime.GOARCH, CWD: cwd},
		Config:      contextConfig{ConfigFile: config.FilePath()},
		Credentials: contextCredentials{Status: "not_configured"},
		Notices:     readCachedUpdateNotices(),
		Env: map[string]string{
			"JIRA_HOST":  envPresence("JIRA_HOST"),
			"JIRA_TOKEN": envPresence("JIRA_TOKEN"),
		},
	}

	cfg, err := config.Load()
	if err != nil {
		doc.Errors = append(doc.Errors, err.Error())
		doc.Credentials.Status = "config_error"
		doc.Credentials.Error = err.Error()
		printContextResult(doc)
		return nil
	}

	doc.Config.Host = cfg.Host
	doc.Config.TokenPresent = cfg.Token != ""
	doc.Config.Configured = cfg.Host != "" && cfg.Token != ""
	doc.Config.HostSource = valueSource("JIRA_HOST", cfg.Host)
	doc.Config.TokenSource = valueSource("JIRA_TOKEN", cfg.Token)
	doc.Credentials.Configured = doc.Config.Configured
	doc.Credentials.Present = doc.Config.TokenPresent
	doc.Credentials.Source = doc.Config.TokenSource
	if !doc.Config.Configured {
		printContextResult(doc)
		return nil
	}

	myself, err := api.NewClient(cfg).Users.Me()
	if err != nil {
		doc.Credentials.Status = "invalid"
		doc.Credentials.Error = err.Error()
		printContextResult(doc)
		return nil
	}
	doc.Credentials.Status = "valid"
	doc.Account = &contextAccount{
		Username:    myself.Name,
		DisplayName: myself.DisplayName,
		Email:       myself.EmailAddress,
	}
	printContextResult(doc)
	return nil
}

func printContextResult(doc contextDocument) {
	if jsonMode {
		output.PrintJSON(doc)
		return
	}
	output.Bold("jira-cli context")
	output.Gray(fmt.Sprintf("  Version: %s", doc.Version))
	output.Gray(fmt.Sprintf("  Runtime: %s/%s", doc.Runtime.OS, doc.Runtime.Arch))
	if doc.Config.Host != "" {
		output.Gray("  Host: " + doc.Config.Host)
	}
	output.Gray("  Auth: " + doc.Credentials.Status)
	if doc.Account != nil {
		output.Gray(fmt.Sprintf("  Account: %s (%s)", doc.Account.DisplayName, doc.Account.Username))
	}
	printUpdateNoticeHint(os.Stdout, doc.Notices)
}

func envPresence(name string) string {
	if os.Getenv(name) == "" {
		return "unset"
	}
	return "set"
}

func valueSource(envName, value string) string {
	if value == "" {
		return ""
	}
	if os.Getenv(envName) != "" {
		return "env"
	}
	return "config_file"
}
