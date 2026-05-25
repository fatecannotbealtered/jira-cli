package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDir_UserHomeDirError(t *testing.T) {
	origHome := os.Getenv("HOME")
	origUserProfile := os.Getenv("USERPROFILE")
	_ = os.Unsetenv("HOME")
	_ = os.Unsetenv("USERPROFILE")
	t.Cleanup(func() {
		_ = os.Setenv("HOME", origHome)
		_ = os.Setenv("USERPROFILE", origUserProfile)
	})

	if got := Dir(); got != ".jira-cli" {
		t.Errorf("Dir() = %q, want .jira-cli", got)
	}
}

func TestSave_MkdirAllError(t *testing.T) {
	tmpDir := t.TempDir()
	fileAsHome := filepath.Join(tmpDir, "not-a-dir")
	if err := os.WriteFile(fileAsHome, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	origHome := os.Getenv("HOME")
	origUserProfile := os.Getenv("USERPROFILE")
	_ = os.Setenv("HOME", fileAsHome)
	_ = os.Setenv("USERPROFILE", fileAsHome)
	t.Cleanup(func() {
		_ = os.Setenv("HOME", origHome)
		_ = os.Setenv("USERPROFILE", origUserProfile)
	})

	err := Save(&Config{Host: "https://jira.example.com", Token: "tok"})
	if err == nil {
		t.Fatal("Save() should fail when config dir cannot be created")
	}
	if !strings.Contains(err.Error(), "creating config dir") {
		t.Errorf("error = %q, want creating config dir message", err.Error())
	}
}

func TestSave_WriteFileError(t *testing.T) {
	tmpDir, restore := overrideHome(t)
	defer restore()

	cfgDir := filepath.Join(tmpDir, ".jira-cli")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.Mkdir(filepath.Join(cfgDir, "config.json"), 0700); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}

	err := Save(&Config{Host: "https://jira.example.com", Token: "tok"})
	if err == nil {
		t.Fatal("Save() should fail when config path is a directory")
	}
	if !strings.Contains(err.Error(), "writing config") {
		t.Errorf("error = %q, want writing config message", err.Error())
	}
}

func TestSave_WriteFileUsesRestrictedMode(t *testing.T) {
	_, restore := overrideHome(t)
	defer restore()

	if err := Save(&Config{Host: "https://jira.example.com", Token: "tok"}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	if _, err := os.Stat(FilePath()); err != nil {
		t.Fatalf("Stat(config file) error: %v", err)
	}
}

func TestLoad_ReadError(t *testing.T) {
	tmpDir, restore := overrideHome(t)
	defer restore()

	cfgDir := filepath.Join(tmpDir, ".jira-cli")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.Mkdir(filepath.Join(cfgDir, "config.json"), 0700); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load() should return error when config path is a directory")
	}
	if !strings.Contains(err.Error(), "reading config") {
		t.Errorf("error = %q, want reading config message", err.Error())
	}
}

func TestDelete_RemoveError(t *testing.T) {
	_, restore := overrideHome(t)
	defer restore()

	if err := Save(&Config{Host: "https://jira.example.com", Token: "tok"}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	orig := configRemove
	configRemove = func(string) error {
		return errors.New("remove failed")
	}
	t.Cleanup(func() { configRemove = orig })

	err := Delete()
	if err == nil {
		t.Fatal("Delete() should fail when remove returns an error")
	}
	if !strings.Contains(err.Error(), "deleting config") {
		t.Errorf("error = %q, want deleting config message", err.Error())
	}
}

func TestIsConfigured_LoadError(t *testing.T) {
	tmpDir, restore := overrideHome(t)
	defer restore()

	cfgDir := filepath.Join(tmpDir, ".jira-cli")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.Mkdir(filepath.Join(cfgDir, "config.json"), 0700); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}

	if IsConfigured() {
		t.Error("IsConfigured() should be false when Load() fails")
	}
}

func TestIsConfigured_PartialCredentials(t *testing.T) {
	_, restore := overrideHome(t)
	defer restore()

	t.Setenv("JIRA_HOST", "https://jira.example.com")
	t.Setenv("JIRA_TOKEN", "   ")

	if IsConfigured() {
		t.Error("IsConfigured() should be false when token is blank")
	}
}

func TestSave_MarshalIndentError(t *testing.T) {
	orig := configMarshalIndent
	configMarshalIndent = func(v any, prefix string, indent string) ([]byte, error) {
		return nil, errors.New("marshal failed")
	}
	t.Cleanup(func() { configMarshalIndent = orig })

	_, restore := overrideHome(t)
	defer restore()

	err := Save(&Config{Host: "https://jira.example.com", Token: "tok"})
	if err == nil {
		t.Fatal("Save() should fail when encoding config fails")
	}
	if !strings.Contains(err.Error(), "encoding config") {
		t.Errorf("error = %q, want encoding config message", err.Error())
	}
}
