package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/fatecannotbealtered/jira-cli/internal/api"
	"github.com/fatecannotbealtered/jira-cli/internal/config"
	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	loginIsTerminal   = term.IsTerminal
	loginReadPassword = term.ReadPassword
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Configure Jira Data Center credentials",
	Long: `Configure Jira Data Center host and Personal Access Token (PAT).

  Interactive:   jira-cli login
  Non-interactive: jira-cli login --host https://jira.company.com --token <PAT>

  Credentials are stored at ~/.jira-cli/config.json (mode 0600).
  Environment variables JIRA_HOST and JIRA_TOKEN override the config file.`,
	RunE: runLogin,
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove saved Jira credentials",
	RunE:  runLogout,
}

var loginHostFlag string
var loginTokenFlag string

func init() {
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
	loginCmd.Flags().StringVar(&loginHostFlag, "host", "", "Jira host URL (e.g. https://jira.company.com)")
	loginCmd.Flags().StringVar(&loginTokenFlag, "token", "", "Personal Access Token (PAT)")
}

func runLogin(_ *cobra.Command, _ []string) error {
	// Non-interactive mode: both --host and --token provided
	if loginHostFlag != "" && loginTokenFlag != "" {
		host := strings.TrimSpace(loginHostFlag)
		if !strings.HasPrefix(host, "https://") {
			output.Error("host must start with https://")
			return SilentErr(ExitBadArgs)
		}
		token := strings.TrimSpace(loginTokenFlag)
		if token == "" {
			output.Error("token cannot be empty")
			return SilentErr(ExitBadArgs)
		}

		cfg := &config.Config{Host: host, Token: token}
		client := api.NewClient(cfg)
		myself, err := client.Users.Me()
		if err != nil {
			output.Error("invalid credentials: " + err.Error())
			return SilentErr(ExitAuth)
		}

		if err := config.Save(cfg); err != nil {
			output.Error("failed to save config: " + err.Error())
			return SilentErr(ExitAuth)
		}

		if jsonMode {
			output.PrintJSON(map[string]string{
				"status":      "ok",
				"displayName": myself.DisplayName,
				"username":    myself.Name,
			})
			return nil
		}
		output.Success(fmt.Sprintf("Logged in as %s (%s)", myself.DisplayName, myself.Name))
		output.Info("Config saved to " + config.FilePath())
		return nil
	}

	// Interactive mode
	reader := bufio.NewReader(os.Stdin)

	if !jsonMode {
		fmt.Println()
		output.Bold("  jira-cli Login (Data Center)")
		output.Gray("  ────────────────────────────────────────")
		fmt.Println()
	}

	host := loginHostFlag
	if host == "" {
		loginPrompt("  Jira host (e.g. https://jira.company.com): ")
		host, _ = reader.ReadString('\n')
		host = strings.TrimSpace(host)
	}
	if !strings.HasPrefix(host, "https://") {
		output.Error("host must start with https://")
		return SilentErr(ExitBadArgs)
	}

	token := loginTokenFlag
	if token == "" {
		loginPrompt("  Personal Access Token (PAT): ")
		var tokenBytes []byte
		var err error
		if loginIsTerminal(int(syscall.Stdin)) {
			tokenBytes, err = loginReadPassword(int(syscall.Stdin))
			loginPrompt("\n")
			if err != nil {
				output.Error("failed to read token: " + err.Error())
				return SilentErr(ExitBadArgs)
			}
		} else {
			line, _ := reader.ReadString('\n')
			tokenBytes = []byte(strings.TrimSpace(line))
		}
		token = strings.TrimSpace(string(tokenBytes))
	}
	if token == "" {
		output.Error("token cannot be empty")
		return SilentErr(ExitBadArgs)
	}

	output.Gray("  Verifying credentials...")
	cfg := &config.Config{Host: host, Token: token}
	client := api.NewClient(cfg)
	myself, err := client.Users.Me()
	if err != nil {
		output.Error("invalid credentials: " + err.Error())
		return SilentErr(ExitAuth)
	}

	if err := config.Save(cfg); err != nil {
		output.Error("failed to save config: " + err.Error())
		return SilentErr(ExitAuth)
	}

	if jsonMode {
		output.PrintJSON(map[string]string{
			"status":      "ok",
			"displayName": myself.DisplayName,
			"username":    myself.Name,
		})
		return nil
	}

	fmt.Println()
	output.Success(fmt.Sprintf("Logged in as %s (%s)", myself.DisplayName, myself.Name))
	output.Info("Config saved to " + config.FilePath())
	fmt.Println()
	output.Gray("  Try: jira-cli doctor")
	fmt.Println()
	return nil
}

func loginPrompt(msg string) {
	if jsonMode {
		fmt.Fprint(os.Stderr, msg)
		return
	}
	fmt.Print(msg)
}

func runLogout(_ *cobra.Command, _ []string) error {
	if err := config.Delete(); err != nil {
		output.Error("failed to remove config: " + err.Error())
		return SilentErr(ExitAuth)
	}
	if jsonMode {
		output.PrintJSON(map[string]string{"status": "loggedOut"})
		return nil
	}
	output.Success("Logged out. Config removed.")
	return nil
}
