// Package auth provides shared OAuth authentication for Google services
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// tokenFile represents the OAuth token file format (compatible with gog)
type tokenFile struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	Expiry       string `json:"expiry"`
}

// LoadToken loads OAuth token for an account from:
// 1. ~/.diane/secrets/google/token_{account}.json
// 2. ~/.config/gog/tokens/{account}.json (fallback for backward compatibility)
func LoadToken(account string) (*oauth2.Token, error) {
	if account == "" {
		account = "default"
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	// Try diane secrets location first
	tokenPath := filepath.Join(home, ".diane", "secrets", "google", fmt.Sprintf("token_%s.json", account))
	tokenData, err := os.ReadFile(tokenPath)
	if err != nil {
		// Try gog tokens location (backward compatibility)
		tokenPath = filepath.Join(home, ".config", "gog", "tokens", fmt.Sprintf("%s.json", account))
		tokenData, err = os.ReadFile(tokenPath)
		if err != nil {
			return nil, fmt.Errorf("no token found for account %s. Run 'gog auth' first", account)
		}
	}

	var tf tokenFile
	if err := json.Unmarshal(tokenData, &tf); err != nil {
		return nil, fmt.Errorf("failed to parse token file: %w", err)
	}

	token := &oauth2.Token{
		AccessToken:  tf.AccessToken,
		TokenType:    tf.TokenType,
		RefreshToken: tf.RefreshToken,
	}

	return token, nil
}

// LoadCredentials loads OAuth client credentials from:
// 1. ~/.diane/secrets/google/credentials.json
// 2. ~/.config/gog/credentials.json (fallback for backward compatibility)
func LoadCredentials() ([]byte, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	// Try diane secrets location first
	credPath := filepath.Join(home, ".diane", "secrets", "google", "credentials.json")
	credData, err := os.ReadFile(credPath)
	if err != nil {
		// Try gog config location
		credPath = filepath.Join(home, ".config", "gog", "credentials.json")
		credData, err = os.ReadFile(credPath)
		if err != nil {
			return nil, fmt.Errorf("no credentials found. Place credentials.json in ~/.diane/secrets/google/")
		}
	}

	return credData, nil
}

// NewOAuthConfig creates an OAuth config for the given scopes
func NewOAuthConfig(scopes ...string) (*oauth2.Config, error) {
	credData, err := LoadCredentials()
	if err != nil {
		return nil, err
	}

	config, err := google.ConfigFromJSON(credData, scopes...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse credentials: %w", err)
	}

	return config, nil
}

// GetTokenSource returns a token source for the given account and scopes.
// The token source automatically refreshes expired tokens.
func GetTokenSource(ctx context.Context, account string, scopes ...string) (oauth2.TokenSource, error) {
	config, err := NewOAuthConfig(scopes...)
	if err != nil {
		return nil, err
	}

	token, err := LoadToken(account)
	if err != nil {
		return nil, err
	}

	return config.TokenSource(ctx, token), nil
}
