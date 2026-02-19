package emergent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_FromFile(t *testing.T) {
	// Create a temp config file
	dir := t.TempDir()
	configPath := filepath.Join(dir, "emergent-config.json")

	cfg := configFile{
		BaseURL: "https://emergent.example.com",
		APIKey:  "emt_test_key_123",
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatal(err)
	}

	// Clear env vars that could interfere
	t.Setenv("TEST_EMERGENT_BASE_URL", "")
	t.Setenv("TEST_EMERGENT_API_KEY", "")

	result, err := loadConfigFrom(configPath, "TEST_EMERGENT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.BaseURL != "https://emergent.example.com" {
		t.Errorf("BaseURL = %q, want %q", result.BaseURL, "https://emergent.example.com")
	}
	if result.APIKey != "emt_test_key_123" {
		t.Errorf("APIKey = %q, want %q", result.APIKey, "emt_test_key_123")
	}
}

func TestLoadConfig_FromEnvVars(t *testing.T) {
	// No config file
	nonexistent := filepath.Join(t.TempDir(), "does-not-exist.json")

	t.Setenv("TEST_EMERGENT_BASE_URL", "https://env.example.com")
	t.Setenv("TEST_EMERGENT_API_KEY", "emt_env_key_456")

	result, err := loadConfigFrom(nonexistent, "TEST_EMERGENT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.BaseURL != "https://env.example.com" {
		t.Errorf("BaseURL = %q, want %q", result.BaseURL, "https://env.example.com")
	}
	if result.APIKey != "emt_env_key_456" {
		t.Errorf("APIKey = %q, want %q", result.APIKey, "emt_env_key_456")
	}
}

func TestLoadConfig_FileTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "emergent-config.json")

	cfg := configFile{
		BaseURL: "https://file.example.com",
		APIKey:  "emt_file_key",
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(configPath, data, 0600)

	// Set env vars too
	t.Setenv("TEST_EMERGENT_BASE_URL", "https://env.example.com")
	t.Setenv("TEST_EMERGENT_API_KEY", "emt_env_key")

	result, err := loadConfigFrom(configPath, "TEST_EMERGENT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File values should win
	if result.BaseURL != "https://file.example.com" {
		t.Errorf("BaseURL = %q, want file value %q", result.BaseURL, "https://file.example.com")
	}
	if result.APIKey != "emt_file_key" {
		t.Errorf("APIKey = %q, want file value %q", result.APIKey, "emt_file_key")
	}
}

func TestLoadConfig_EnvFillsMissingFileValues(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "emergent-config.json")

	// File has only BaseURL
	cfg := configFile{
		BaseURL: "https://file.example.com",
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(configPath, data, 0600)

	// Env has API key
	t.Setenv("TEST_EMERGENT_BASE_URL", "")
	t.Setenv("TEST_EMERGENT_API_KEY", "emt_env_key")

	result, err := loadConfigFrom(configPath, "TEST_EMERGENT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.BaseURL != "https://file.example.com" {
		t.Errorf("BaseURL = %q, want %q", result.BaseURL, "https://file.example.com")
	}
	if result.APIKey != "emt_env_key" {
		t.Errorf("APIKey = %q, want %q", result.APIKey, "emt_env_key")
	}
}

func TestLoadConfig_DefaultBaseURL(t *testing.T) {
	nonexistent := filepath.Join(t.TempDir(), "does-not-exist.json")

	t.Setenv("TEST_EMERGENT_BASE_URL", "")
	t.Setenv("TEST_EMERGENT_API_KEY", "emt_key")

	result, err := loadConfigFrom(nonexistent, "TEST_EMERGENT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.BaseURL != DefaultBaseURL {
		t.Errorf("BaseURL = %q, want default %q", result.BaseURL, DefaultBaseURL)
	}
}

func TestLoadConfig_MissingAPIKey(t *testing.T) {
	nonexistent := filepath.Join(t.TempDir(), "does-not-exist.json")

	t.Setenv("TEST_EMERGENT_BASE_URL", "")
	t.Setenv("TEST_EMERGENT_API_KEY", "")

	_, err := loadConfigFrom(nonexistent, "TEST_EMERGENT")
	if err == nil {
		t.Fatal("expected error for missing API key, got nil")
	}
}

func TestLoadConfig_IgnoresLegacyProjectID(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "emergent-config.json")

	// Write a config with the legacy project_id field
	data := []byte(`{
		"base_url": "https://emergent.example.com",
		"api_key": "emt_key_123",
		"project_id": "proj_legacy_id"
	}`)
	os.WriteFile(configPath, data, 0600)

	t.Setenv("TEST_EMERGENT_BASE_URL", "")
	t.Setenv("TEST_EMERGENT_API_KEY", "")

	result, err := loadConfigFrom(configPath, "TEST_EMERGENT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Config should load without error, project_id ignored
	if result.BaseURL != "https://emergent.example.com" {
		t.Errorf("BaseURL = %q, want %q", result.BaseURL, "https://emergent.example.com")
	}
	if result.APIKey != "emt_key_123" {
		t.Errorf("APIKey = %q, want %q", result.APIKey, "emt_key_123")
	}
}

func TestNewClient(t *testing.T) {
	cfg := &Config{
		BaseURL: "http://localhost:3002",
		APIKey:  "emt_test_key",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestGetClient_Singleton(t *testing.T) {
	// Reset any existing global state
	ResetClient()
	defer ResetClient()

	// Set env vars for config
	t.Setenv("EMERGENT_API_KEY", "emt_singleton_test")
	t.Setenv("EMERGENT_BASE_URL", "http://localhost:9999")

	c1, err := GetClient()
	if err != nil {
		t.Fatalf("first GetClient() failed: %v", err)
	}

	c2, err := GetClient()
	if err != nil {
		t.Fatalf("second GetClient() failed: %v", err)
	}

	if c1 != c2 {
		t.Error("GetClient() returned different instances — expected singleton")
	}
}

func TestResetClient(t *testing.T) {
	ResetClient()
	defer ResetClient()

	t.Setenv("EMERGENT_API_KEY", "emt_reset_test")
	t.Setenv("EMERGENT_BASE_URL", "http://localhost:9999")

	c1, err := GetClient()
	if err != nil {
		t.Fatalf("first GetClient() failed: %v", err)
	}

	ResetClient()

	c2, err := GetClient()
	if err != nil {
		t.Fatalf("GetClient() after reset failed: %v", err)
	}

	if c1 == c2 {
		t.Error("GetClient() after ResetClient() returned same instance — expected new one")
	}
}
