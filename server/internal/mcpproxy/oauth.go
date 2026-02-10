package mcpproxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Well-known OAuth providers - convenience shortcuts for common providers
// Users can also specify full OAuth config manually in mcp-servers.json
var oauthProviders = map[string]OAuthProviderConfig{
	// "github" provider should be configured in mcp-servers.json with your OAuth App client ID
	// See: https://github.com/settings/developers to create an OAuth App
}

// OAuthProviderConfig holds provider-specific OAuth settings
type OAuthProviderConfig struct {
	ClientID      string
	ClientSecret  string
	DeviceAuthURL string
	TokenURL      string
	Scopes        []string
}

// OAuthToken represents a stored OAuth token
type OAuthToken struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	Scope        string    `json:"scope,omitempty"`
}

// IsExpired checks if the token is expired (with 5 minute buffer)
func (t *OAuthToken) IsExpired() bool {
	if t.ExpiresAt.IsZero() {
		return false // No expiry set
	}
	return time.Now().Add(5 * time.Minute).After(t.ExpiresAt)
}

// DeviceCodeResponse is the response from the device authorization endpoint
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// TokenResponse is the response from the token endpoint
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Error        string `json:"error,omitempty"`
	ErrorDesc    string `json:"error_description,omitempty"`
}

// OAuthManager manages OAuth tokens for MCP servers
type OAuthManager struct {
	tokensDir string
	tokens    map[string]*OAuthToken
	mu        sync.RWMutex
}

// NewOAuthManager creates a new OAuth manager
func NewOAuthManager() (*OAuthManager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	tokensDir := filepath.Join(home, ".diane", "oauth-tokens")
	if err := os.MkdirAll(tokensDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create tokens directory: %w", err)
	}

	manager := &OAuthManager{
		tokensDir: tokensDir,
		tokens:    make(map[string]*OAuthToken),
	}

	// Load existing tokens
	manager.loadTokens()

	return manager, nil
}

// loadTokens loads all stored tokens from disk
func (m *OAuthManager) loadTokens() {
	entries, err := os.ReadDir(m.tokensDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		serverName := entry.Name()[:len(entry.Name())-5] // Remove .json
		token, err := m.loadToken(serverName)
		if err == nil {
			m.tokens[serverName] = token
		}
	}
}

// loadToken loads a token for a specific server
func (m *OAuthManager) loadToken(serverName string) (*OAuthToken, error) {
	path := filepath.Join(m.tokensDir, serverName+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var token OAuthToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}

	return &token, nil
}

// saveToken saves a token to disk
func (m *OAuthManager) saveToken(serverName string, token *OAuthToken) error {
	path := filepath.Join(m.tokensDir, serverName+".json")
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// GetToken returns a valid token for a server, or nil if not available
func (m *OAuthManager) GetToken(serverName string) *OAuthToken {
	m.mu.RLock()
	defer m.mu.RUnlock()

	token, ok := m.tokens[serverName]
	if !ok {
		return nil
	}

	if token.IsExpired() {
		return nil // Token expired, need to re-auth
	}

	return token
}

// HasValidToken checks if a valid token exists for a server
func (m *OAuthManager) HasValidToken(serverName string) bool {
	return m.GetToken(serverName) != nil
}

// SetToken stores a token for a server
func (m *OAuthManager) SetToken(serverName string, token *OAuthToken) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tokens[serverName] = token
	return m.saveToken(serverName, token)
}

// DeleteToken removes a token for a server
func (m *OAuthManager) DeleteToken(serverName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.tokens, serverName)
	path := filepath.Join(m.tokensDir, serverName+".json")
	return os.Remove(path)
}

// GetTokenStatus returns the status of a token for a server
func (m *OAuthManager) GetTokenStatus(serverName string) map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	token, ok := m.tokens[serverName]
	if !ok {
		return map[string]interface{}{
			"authenticated": false,
			"status":        "not_authenticated",
		}
	}

	status := map[string]interface{}{
		"authenticated": true,
		"token_type":    token.TokenType,
		"scope":         token.Scope,
	}

	if !token.ExpiresAt.IsZero() {
		status["expires_at"] = token.ExpiresAt
		if token.IsExpired() {
			status["status"] = "expired"
			status["authenticated"] = false
		} else {
			status["status"] = "valid"
			status["expires_in"] = time.Until(token.ExpiresAt).String()
		}
	} else {
		status["status"] = "valid"
	}

	return status
}

// ListTokenStatuses returns status for all servers with OAuth config
func (m *OAuthManager) ListTokenStatuses() map[string]map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]map[string]interface{})
	for name := range m.tokens {
		result[name] = m.GetTokenStatus(name)
	}
	return result
}

// GetProviderConfig returns OAuth config for a server, resolving provider presets
func GetProviderConfig(oauth *OAuthConfig) *OAuthProviderConfig {
	if oauth == nil {
		return nil
	}

	// Check for well-known provider
	if oauth.Provider != "" {
		if provider, ok := oauthProviders[oauth.Provider]; ok {
			return &provider
		}
	}

	// Use manual configuration
	if oauth.ClientID == "" || oauth.TokenURL == "" {
		return nil
	}

	return &OAuthProviderConfig{
		ClientID:      oauth.ClientID,
		ClientSecret:  oauth.ClientSecret,
		DeviceAuthURL: oauth.DeviceAuthURL,
		TokenURL:      oauth.TokenURL,
		Scopes:        oauth.Scopes,
	}
}

// StartDeviceFlow initiates the OAuth device authorization flow
// Returns the DeviceCodeResponse containing the user code and verification URL
func (m *OAuthManager) StartDeviceFlow(serverName string, config *OAuthProviderConfig) (*DeviceCodeResponse, error) {
	if config.DeviceAuthURL == "" {
		return nil, fmt.Errorf("device authorization URL not configured")
	}

	data := url.Values{
		"client_id": {config.ClientID},
	}
	if len(config.Scopes) > 0 {
		for _, scope := range config.Scopes {
			data.Add("scope", scope)
		}
	}

	req, err := http.NewRequest("POST", config.DeviceAuthURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("device auth request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device auth failed with status %d: %s", resp.StatusCode, string(body))
	}

	var deviceResp DeviceCodeResponse
	if err := json.Unmarshal(body, &deviceResp); err != nil {
		return nil, fmt.Errorf("failed to parse device auth response: %w", err)
	}

	return &deviceResp, nil
}

// PollForToken polls the token endpoint until the user completes authorization
// Returns the token when successful, or an error if authorization fails/expires
func (m *OAuthManager) PollForToken(serverName string, config *OAuthProviderConfig, deviceCode string, interval int) (*OAuthToken, error) {
	if interval < 5 {
		interval = 5 // Minimum polling interval
	}

	client := &http.Client{Timeout: 30 * time.Second}

	for {
		time.Sleep(time.Duration(interval) * time.Second)

		data := url.Values{
			"client_id":   {config.ClientID},
			"device_code": {deviceCode},
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		}
		if config.ClientSecret != "" {
			data.Set("client_secret", config.ClientSecret)
		}

		req, err := http.NewRequest("POST", config.TokenURL, bytes.NewBufferString(data.Encode()))
		if err != nil {
			return nil, fmt.Errorf("failed to create token request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("token request failed: %w", err)
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var tokenResp TokenResponse
		if err := json.Unmarshal(body, &tokenResp); err != nil {
			return nil, fmt.Errorf("failed to parse token response: %w", err)
		}

		switch tokenResp.Error {
		case "":
			// Success!
			token := &OAuthToken{
				AccessToken:  tokenResp.AccessToken,
				TokenType:    tokenResp.TokenType,
				RefreshToken: tokenResp.RefreshToken,
				Scope:        tokenResp.Scope,
			}
			if tokenResp.ExpiresIn > 0 {
				token.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
			}

			// Save the token
			if err := m.SetToken(serverName, token); err != nil {
				return nil, fmt.Errorf("failed to save token: %w", err)
			}

			return token, nil

		case "authorization_pending":
			// User hasn't authorized yet, continue polling
			continue

		case "slow_down":
			// We need to slow down, increase interval
			interval += 5
			continue

		case "expired_token":
			return nil, fmt.Errorf("device code expired, please try again")

		case "access_denied":
			return nil, fmt.Errorf("user denied authorization")

		default:
			return nil, fmt.Errorf("token error: %s - %s", tokenResp.Error, tokenResp.ErrorDesc)
		}
	}
}

// global OAuth manager instance
var globalOAuthManager *OAuthManager
var oauthManagerOnce sync.Once

// GetOAuthManager returns the global OAuth manager instance
func GetOAuthManager() *OAuthManager {
	oauthManagerOnce.Do(func() {
		var err error
		globalOAuthManager, err = NewOAuthManager()
		if err != nil {
			// Log error but don't fail - OAuth just won't work
			fmt.Fprintf(os.Stderr, "Warning: failed to initialize OAuth manager: %v\n", err)
		}
	})
	return globalOAuthManager
}
