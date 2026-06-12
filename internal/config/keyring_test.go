package config

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestMain(m *testing.M) {
	// Tests must never touch the real OS keyring.
	keyring.MockInit()
	os.Exit(m.Run())
}

// TestSaveUsesKeyringAndKeepsConfigSecretFree covers the keyring three-part
// pattern (SEC-SPEC §4): secret in the keyring, zero-secret config file with
// a visible storage marker, and Load round-tripping through the keyring.
func TestSaveUsesKeyringAndKeepsConfigSecretFree(t *testing.T) {
	_, restore := overrideHome(t)
	defer restore()

	if err := Save(&Config{Host: "https://jira.example.com", Token: "s3cret-token"}); err != nil {
		t.Fatalf("save: %v", err)
	}

	raw, err := os.ReadFile(FilePath())
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if strings.Contains(string(raw), "s3cret-token") {
		t.Fatalf("config file must not contain the secret: %s", raw)
	}
	var disk map[string]any
	if err := json.Unmarshal(raw, &disk); err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if disk["token_storage"] != "keyring" {
		t.Fatalf("token_storage = %v, want keyring", disk["token_storage"])
	}
	if _, ok := disk["token_enc"]; ok {
		t.Fatal("keyring-backed config must not carry token_enc")
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Token != "s3cret-token" {
		t.Fatalf("token = %q, want round-trip through keyring", cfg.Token)
	}
	if cfg.Storage != "keyring" {
		t.Fatalf("storage = %q, want keyring", cfg.Storage)
	}
}

// TestSaveFallsBackToEncryptedFile covers the degradation path: no keyring
// service available -> machine-bound encrypted file, marker visible.
func TestSaveFallsBackToEncryptedFile(t *testing.T) {
	_, restore := overrideHome(t)
	defer restore()

	origSet := keyringSet
	keyringSet = func(service, account, secret string) error {
		return errors.New("no keyring service")
	}
	defer func() { keyringSet = origSet }()

	if err := Save(&Config{Host: "https://jira.example.com", Token: "fallback-token"}); err != nil {
		t.Fatalf("save: %v", err)
	}

	raw, _ := os.ReadFile(FilePath())
	var disk map[string]any
	if err := json.Unmarshal(raw, &disk); err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if disk["token_storage"] != "encrypted-file" {
		t.Fatalf("token_storage = %v, want encrypted-file", disk["token_storage"])
	}
	if enc, _ := disk["token_enc"].(string); enc == "" || strings.Contains(enc, "fallback-token") {
		t.Fatalf("token_enc should hold ciphertext, got %q", enc)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Token != "fallback-token" || cfg.Storage != "encrypted-file" {
		t.Fatalf("cfg = %+v, want decrypted fallback token", cfg)
	}
}

// TestDeleteRemovesKeyringEntry: logout must clear the keyring secret too.
func TestDeleteRemovesKeyringEntry(t *testing.T) {
	_, restore := overrideHome(t)
	defer restore()

	if err := Save(&Config{Host: "https://jira.example.com", Token: "bye-token"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := Delete(); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := keyringGet(keyringService, keyringAccount); err == nil {
		t.Fatal("keyring entry should be gone after Delete")
	}
}
