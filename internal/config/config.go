package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var (
	configMarshalIndent = json.MarshalIndent
	configRemove        = os.Remove
)

// Config stores Jira Data Center authentication information.
type Config struct {
	Host  string `json:"host"`
	Token string `json:"token"`
	// Storage reports which at-rest backend served the token:
	// "keyring", "encrypted-file", or "" (env-only / legacy plaintext).
	Storage string `json:"-"`
}

type diskConfig struct {
	Version      int    `json:"version"`
	Host         string `json:"host"`
	Token        string `json:"token,omitempty"`
	TokenEnc     string `json:"token_enc,omitempty"`
	TokenStorage string `json:"token_storage,omitempty"`
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
		var disk diskConfig
		if err := json.Unmarshal(data, &disk); err != nil {
			return nil, fmt.Errorf("parsing config %s: %w", FilePath(), err)
		}
		cfg.Host = disk.Host
		switch {
		case disk.TokenStorage == storageKeyring:
			token, err := keyringGet(keyringService, keyringAccount)
			if err != nil {
				return nil, fmt.Errorf("reading token from OS keyring (re-run 'jira-cli login'): %w", err)
			}
			cfg.Token = token
			cfg.Storage = storageKeyring
		case disk.TokenEnc != "":
			token, err := decryptToken(disk.TokenEnc)
			if err != nil {
				return nil, fmt.Errorf("decrypting config token: %w", err)
			}
			cfg.Token = token
			cfg.Storage = storageEncryptedFile
		default:
			// Legacy plaintext config; the next successful login/save rewrites it encrypted.
			cfg.Token = disk.Token
		}
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

	// Keyring three-part pattern (SEC-SPEC §4): secret to the OS keyring,
	// zero-secret config. Machine-bound file encryption only as fallback when
	// no keyring service is available; the chosen backend stays visible.
	disk := diskConfig{Version: 2, Host: cfg.Host}
	if err := keyringSet(keyringService, keyringAccount, cfg.Token); err == nil {
		disk.TokenStorage = storageKeyring
		cfg.Storage = storageKeyring
	} else {
		tokenEnc, encErr := encryptToken(cfg.Token)
		if encErr != nil {
			return fmt.Errorf("encrypting config token: %w", encErr)
		}
		disk.TokenEnc = tokenEnc
		disk.TokenStorage = storageEncryptedFile
		cfg.Storage = storageEncryptedFile
	}
	data, err := configMarshalIndent(disk, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	if err := os.WriteFile(FilePath(), data, 0600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

func machineKey() ([]byte, error) {
	home, _ := os.UserHomeDir()
	host, _ := os.Hostname()
	material := "jira-cli-config-v2|" + host + "|" + home
	sum := sha256.Sum256([]byte(material))
	return sum[:], nil
}

func encryptToken(token string) (string, error) {
	key, err := machineKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(token), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func decryptToken(encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	key, err := machineKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", fmt.Errorf("encrypted token is too short")
	}
	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
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

// Delete removes the configuration file and the keyring entry (used for logout).
func Delete() error {
	// Best-effort: the keyring entry may not exist (env-only or encrypted-file setups).
	_ = keyringDelete(keyringService, keyringAccount)
	err := configRemove(FilePath())
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
