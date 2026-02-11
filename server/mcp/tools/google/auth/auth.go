// Package auth provides shared OAuth authentication for Google services
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Google OAuth endpoints for device flow
const (
	GoogleDeviceAuthURL = "https://oauth2.googleapis.com/device/code"
	GoogleTokenURL      = "https://oauth2.googleapis.com/token"
)

// DeviceCodeResponse represents the response from the device authorization endpoint
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURL string `json:"verification_url"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// DeviceTokenResponse represents the token response from polling
type DeviceTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	Error        string `json:"error,omitempty"`
}

// AuthStatus represents the current authentication status
type AuthStatus struct {
	Authenticated  bool   `json:"authenticated"`
	Account        string `json:"account"`
	HasToken       bool   `json:"has_token"`
	HasCredentials bool   `json:"has_credentials"`
	HasADC         bool   `json:"has_adc"`
	UsingADC       bool   `json:"using_adc"`
	TokenPath      string `json:"token_path,omitempty"`
}

// tokenFile represents the OAuth token file format (compatible with gog)
type tokenFile struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	Expiry       string `json:"expiry"`
}

// adcFile represents the Application Default Credentials file format
type adcFile struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RefreshToken string `json:"refresh_token"`
	Type         string `json:"type"`
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

// GetADCPath returns the path to Application Default Credentials
func GetADCPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "gcloud", "application_default_credentials.json"), nil
}

// LoadADC loads Application Default Credentials from ~/.config/gcloud/application_default_credentials.json
func LoadADC() (*adcFile, error) {
	adcPath, err := GetADCPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(adcPath)
	if err != nil {
		return nil, fmt.Errorf("no Application Default Credentials found. Run 'gcloud auth application-default login'")
	}

	var adc adcFile
	if err := json.Unmarshal(data, &adc); err != nil {
		return nil, fmt.Errorf("failed to parse ADC: %w", err)
	}

	return &adc, nil
}

// HasADC checks if Application Default Credentials exist
func HasADC() bool {
	_, err := LoadADC()
	return err == nil
}

// GetADCTokenSource returns a token source using Application Default Credentials.
// This is a simpler alternative when no custom credentials.json is configured.
func GetADCTokenSource(ctx context.Context, scopes ...string) (oauth2.TokenSource, error) {
	adc, err := LoadADC()
	if err != nil {
		return nil, err
	}

	// Create OAuth config from ADC
	config := &oauth2.Config{
		ClientID:     adc.ClientID,
		ClientSecret: adc.ClientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       scopes,
	}

	// Create token from ADC refresh token
	token := &oauth2.Token{
		RefreshToken: adc.RefreshToken,
	}

	return config.TokenSource(ctx, token), nil
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
// It tries in order:
// 1. Custom credentials.json + token file
// 2. Application Default Credentials (ADC) as fallback
func GetTokenSource(ctx context.Context, account string, scopes ...string) (oauth2.TokenSource, error) {
	// Try custom credentials + token first
	config, err := NewOAuthConfig(scopes...)
	if err == nil {
		token, err := LoadToken(account)
		if err == nil {
			return config.TokenSource(ctx, token), nil
		}
	}

	// Fallback to ADC
	adcTs, err := GetADCTokenSource(ctx, scopes...)
	if err == nil {
		return adcTs, nil
	}

	return nil, fmt.Errorf("no valid credentials found: configure credentials.json or run 'gcloud auth application-default login'")
}

// GetTokenPath returns the path where a token would be saved for an account
func GetTokenPath(account string) (string, error) {
	if account == "" {
		account = "default"
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".diane", "secrets", "google", fmt.Sprintf("token_%s.json", account)), nil
}

// SaveToken saves an OAuth token for an account to ~/.diane/secrets/google/token_{account}.json
func SaveToken(account string, token *oauth2.Token) error {
	if account == "" {
		account = "default"
	}

	tokenPath, err := GetTokenPath(account)
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(tokenPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create token directory: %w", err)
	}

	// Convert to tokenFile format
	tf := tokenFile{
		AccessToken:  token.AccessToken,
		TokenType:    token.TokenType,
		RefreshToken: token.RefreshToken,
		Expiry:       token.Expiry.Format(time.RFC3339),
	}

	data, err := json.MarshalIndent(tf, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	if err := os.WriteFile(tokenPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	return nil
}

// DeleteToken removes the OAuth token for an account
func DeleteToken(account string) error {
	if account == "" {
		account = "default"
	}

	tokenPath, err := GetTokenPath(account)
	if err != nil {
		return err
	}

	if err := os.Remove(tokenPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete token: %w", err)
	}

	return nil
}

// GetAuthStatus checks the current authentication status for an account
func GetAuthStatus(account string) (*AuthStatus, error) {
	if account == "" {
		account = "default"
	}

	status := &AuthStatus{
		Account: account,
	}

	// Check if custom credentials exist
	_, err := LoadCredentials()
	status.HasCredentials = err == nil

	// Check if ADC exists
	status.HasADC = HasADC()

	// Check if token exists (for custom credentials flow)
	tokenPath, err := GetTokenPath(account)
	if err != nil {
		return nil, err
	}

	token, err := LoadToken(account)
	if err == nil {
		status.HasToken = true
		status.TokenPath = tokenPath
		// Token is considered valid if it has a refresh token (can be refreshed)
		status.Authenticated = token.RefreshToken != ""
	}

	// If no custom token but ADC exists, user is authenticated via ADC
	if !status.HasToken && status.HasADC {
		status.Authenticated = true
		status.UsingADC = true
	}

	return status, nil
}

// StartDeviceFlow initiates the Google OAuth device flow
func StartDeviceFlow(scopes ...string) (*DeviceCodeResponse, error) {
	config, err := NewOAuthConfig(scopes...)
	if err != nil {
		return nil, err
	}

	// Make request to device authorization endpoint
	data := url.Values{
		"client_id": {config.ClientID},
		"scope":     {strings.Join(scopes, " ")},
	}

	resp, err := http.PostForm(GoogleDeviceAuthURL, data)
	if err != nil {
		return nil, fmt.Errorf("failed to start device flow: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device flow request failed: %s", string(body))
	}

	var dcr DeviceCodeResponse
	if err := json.Unmarshal(body, &dcr); err != nil {
		return nil, fmt.Errorf("failed to parse device code response: %w", err)
	}

	// Default interval to 5 seconds if not provided
	if dcr.Interval == 0 {
		dcr.Interval = 5
	}

	return &dcr, nil
}

// PollForToken polls the token endpoint until the user authorizes or an error occurs
// Returns the token on success, or an error with the error code from the OAuth response
func PollForToken(account string, deviceCode string, interval int) (*oauth2.Token, error) {
	config, err := NewOAuthConfig()
	if err != nil {
		return nil, err
	}

	data := url.Values{
		"client_id":     {config.ClientID},
		"client_secret": {config.ClientSecret},
		"device_code":   {deviceCode},
		"grant_type":    {"urn:ietf:params:oauth:grant_type:device_code"},
	}

	resp, err := http.PostForm(GoogleTokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("failed to poll for token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var tokenResp DeviceTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	// Check for errors
	if tokenResp.Error != "" {
		return nil, fmt.Errorf("%s", tokenResp.Error)
	}

	// Success - create token and save it
	token := &oauth2.Token{
		AccessToken:  tokenResp.AccessToken,
		TokenType:    tokenResp.TokenType,
		RefreshToken: tokenResp.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}

	if err := SaveToken(account, token); err != nil {
		return nil, fmt.Errorf("failed to save token: %w", err)
	}

	return token, nil
}
