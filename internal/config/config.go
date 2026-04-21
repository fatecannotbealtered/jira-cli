package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config stores Jira Data Center authentication information.
type Config struct {
	Host  string `json:"host"`
	Token string `json:"token"`
}

// Dir returns the configuration directory path ~/.jira-cli/
func Dir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".jira-cli"
	}
	return filepath.Join(home, ".jira-cli")
}

// FilePath returns the configuration file path ~/.jira-cli/config.json
func FilePath() string {
	return filepath.Join(Dir(), "config.json")
}

// Load reads the configuration file.
// Environment variables JIRA_HOST and JIRA_TOKEN take precedence over the config file.
// Returns an empty Config (no error) if neither source has values.
func Load() (*Config, error) {
	cfg := &Config{}

	// 1. Try config file
	data, err := os.ReadFile(FilePath())
	if err == nil {
		_ = json.Unmarshal(data, cfg)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	// 2. Environment variables override file values
	if envHost := strings.TrimSpace(os.Getenv("JIRA_HOST")); envHost != "" {
		cfg.Host = envHost
	}
	if envToken := strings.TrimSpace(os.Getenv("JIRA_TOKEN")); envToken != "" {
		cfg.Token = envToken
	}

	return cfg, nil
}

// Save writes the configuration to file, automatically creating the directory (mode 0700). File mode is 0600.
func Save(cfg *Config) error {
	dir := Dir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	if err := os.WriteFile(FilePath(), data, 0600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

// MustLoad reads the configuration and validates required fields.
func MustLoad() (*Config, error) {
	cfg, err := Load()
	if err != nil {
		return nil, err
	}

	if cfg.Host == "" || strings.TrimSpace(cfg.Token) == "" {
		return nil, errors.New("not logged in: run 'jira-cli login' or set JIRA_HOST and JIRA_TOKEN environment variables")
	}

	// Enforce HTTPS
	if !strings.HasPrefix(cfg.Host, "https://") {
		return nil, errors.New("host must start with https://")
	}

	return cfg, nil
}

// Delete removes the configuration file (used for logout).
func Delete() error {
	err := os.Remove(FilePath())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("deleting config: %w", err)
	}
	return nil
}

// IsConfigured reports whether credentials are available (file or env vars).
func IsConfigured() bool {
	cfg, err := Load()
	if err != nil {
		return false
	}
	return cfg.Host != "" && strings.TrimSpace(cfg.Token) != ""
}
