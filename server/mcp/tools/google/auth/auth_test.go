// Package auth provides shared OAuth authentication for Google services
package auth

import (
	"os"
	"path/filepath"
	"testing"
)

// =============================================================================
// Unit Tests for Token Loading
// =============================================================================

func TestLoadToken_MissingAccount(t *testing.T) {
	// Test with default account when none specified
	_, err := LoadToken("")
	// This will fail unless there's a token file, which is expected
	if err == nil {
		t.Log("Token found for default account (integration environment)")
	} else {
		// Expected in test environments without tokens
		t.Logf("Expected error for missing token: %v", err)
	}
}

func TestLoadToken_NonExistentAccount(t *testing.T) {
	_, err := LoadToken("nonexistent-test-account-12345")
	if err == nil {
		t.Error("LoadToken() expected error for non-existent account, got nil")
	}
}

func TestLoadCredentials_FileNotFound(t *testing.T) {
	// Save original home and set a temp one
	origHome := os.Getenv("HOME")
	tempDir := t.TempDir()
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	_, err := LoadCredentials()
	if err == nil {
		t.Error("LoadCredentials() expected error when no credentials file exists, got nil")
	}
}

func TestLoadCredentials_ValidFile(t *testing.T) {
	// Save original home and set a temp one
	origHome := os.Getenv("HOME")
	tempDir := t.TempDir()
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	// Create the credentials directory and file
	credDir := filepath.Join(tempDir, ".diane", "secrets", "google")
	if err := os.MkdirAll(credDir, 0755); err != nil {
		t.Fatalf("Failed to create credentials directory: %v", err)
	}

	credContent := `{
		"installed": {
			"client_id": "test-client-id.apps.googleusercontent.com",
			"client_secret": "test-client-secret",
			"auth_uri": "https://accounts.google.com/o/oauth2/auth",
			"token_uri": "https://oauth2.googleapis.com/token"
		}
	}`

	credPath := filepath.Join(credDir, "credentials.json")
	if err := os.WriteFile(credPath, []byte(credContent), 0644); err != nil {
		t.Fatalf("Failed to write credentials file: %v", err)
	}

	data, err := LoadCredentials()
	if err != nil {
		t.Errorf("LoadCredentials() unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Error("LoadCredentials() returned empty data")
	}
}

func TestLoadToken_ValidFile(t *testing.T) {
	// Save original home and set a temp one
	origHome := os.Getenv("HOME")
	tempDir := t.TempDir()
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	// Create the token directory and file
	tokenDir := filepath.Join(tempDir, ".diane", "secrets", "google")
	if err := os.MkdirAll(tokenDir, 0755); err != nil {
		t.Fatalf("Failed to create token directory: %v", err)
	}

	tokenContent := `{
		"access_token": "test-access-token",
		"token_type": "Bearer",
		"refresh_token": "test-refresh-token",
		"expiry": "2026-12-31T23:59:59Z"
	}`

	tokenPath := filepath.Join(tokenDir, "token_testaccount.json")
	if err := os.WriteFile(tokenPath, []byte(tokenContent), 0644); err != nil {
		t.Fatalf("Failed to write token file: %v", err)
	}

	token, err := LoadToken("testaccount")
	if err != nil {
		t.Errorf("LoadToken() unexpected error: %v", err)
	}
	if token == nil {
		t.Fatal("LoadToken() returned nil token")
	}
	if token.AccessToken != "test-access-token" {
		t.Errorf("LoadToken() access token = %q, want %q", token.AccessToken, "test-access-token")
	}
	if token.RefreshToken != "test-refresh-token" {
		t.Errorf("LoadToken() refresh token = %q, want %q", token.RefreshToken, "test-refresh-token")
	}
	if token.TokenType != "Bearer" {
		t.Errorf("LoadToken() token type = %q, want %q", token.TokenType, "Bearer")
	}
}

func TestLoadToken_GogFallback(t *testing.T) {
	// Save original home and set a temp one
	origHome := os.Getenv("HOME")
	tempDir := t.TempDir()
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	// Create the gog token directory (fallback location)
	tokenDir := filepath.Join(tempDir, ".config", "gog", "tokens")
	if err := os.MkdirAll(tokenDir, 0755); err != nil {
		t.Fatalf("Failed to create gog token directory: %v", err)
	}

	tokenContent := `{
		"access_token": "gog-access-token",
		"token_type": "Bearer",
		"refresh_token": "gog-refresh-token"
	}`

	tokenPath := filepath.Join(tokenDir, "fallbackaccount.json")
	if err := os.WriteFile(tokenPath, []byte(tokenContent), 0644); err != nil {
		t.Fatalf("Failed to write gog token file: %v", err)
	}

	token, err := LoadToken("fallbackaccount")
	if err != nil {
		t.Errorf("LoadToken() unexpected error for gog fallback: %v", err)
	}
	if token == nil {
		t.Fatal("LoadToken() returned nil token for gog fallback")
	}
	if token.AccessToken != "gog-access-token" {
		t.Errorf("LoadToken() access token = %q, want %q", token.AccessToken, "gog-access-token")
	}
}

func TestNewOAuthConfig_NoCredentials(t *testing.T) {
	// Save original home and set a temp one (with no credentials)
	origHome := os.Getenv("HOME")
	tempDir := t.TempDir()
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	_, err := NewOAuthConfig("https://www.googleapis.com/auth/gmail.readonly")
	if err == nil {
		t.Error("NewOAuthConfig() expected error when no credentials file exists, got nil")
	}
}

func TestNewOAuthConfig_ValidCredentials(t *testing.T) {
	// Save original home and set a temp one
	origHome := os.Getenv("HOME")
	tempDir := t.TempDir()
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	// Create credentials file
	credDir := filepath.Join(tempDir, ".diane", "secrets", "google")
	if err := os.MkdirAll(credDir, 0755); err != nil {
		t.Fatalf("Failed to create credentials directory: %v", err)
	}

	credContent := `{
		"installed": {
			"client_id": "test-client-id.apps.googleusercontent.com",
			"client_secret": "test-client-secret",
			"auth_uri": "https://accounts.google.com/o/oauth2/auth",
			"token_uri": "https://oauth2.googleapis.com/token",
			"redirect_uris": ["http://localhost"]
		}
	}`

	credPath := filepath.Join(credDir, "credentials.json")
	if err := os.WriteFile(credPath, []byte(credContent), 0644); err != nil {
		t.Fatalf("Failed to write credentials file: %v", err)
	}

	config, err := NewOAuthConfig("https://www.googleapis.com/auth/gmail.readonly")
	if err != nil {
		t.Errorf("NewOAuthConfig() unexpected error: %v", err)
	}
	if config == nil {
		t.Fatal("NewOAuthConfig() returned nil config")
	}
	if config.ClientID != "test-client-id.apps.googleusercontent.com" {
		t.Errorf("NewOAuthConfig() client ID = %q, want %q", config.ClientID, "test-client-id.apps.googleusercontent.com")
	}
}

// =============================================================================
// Integration Tests (require actual credentials)
// =============================================================================

func TestGetTokenSource_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test requires actual credentials to be set up
	// It will be skipped in most test environments
	t.Log("Integration test for GetTokenSource - requires actual Google credentials")
}
