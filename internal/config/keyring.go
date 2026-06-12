package config

import "github.com/zalando/go-keyring"

// Keyring three-part pattern (SEC-SPEC §4): the API token lives in the OS
// keyring (Windows Credential Manager / macOS Keychain / Linux Secret
// Service) and config.json keeps zero secrets — only the marker saying which
// backend is in use. Machine-bound file encryption remains as the fallback
// for environments without a keyring service.
const (
	keyringService = "jira-cli"
	keyringAccount = "api-token"
)

// Seams: tests override these to avoid touching the real OS keyring.
var (
	keyringSet    = keyring.Set
	keyringGet    = keyring.Get
	keyringDelete = keyring.Delete
)

// Storage backend markers persisted in config.json.
const (
	storageKeyring       = "keyring"
	storageEncryptedFile = "encrypted-file"
)
