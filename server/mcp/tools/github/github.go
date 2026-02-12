// Package github provides GitHub App-based tools for commenting on issues
// as the Diane bot with a separate identity from the user.
package github

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// getConfigPath returns the path to the GitHub bot config file
func getConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".diane", "secrets", "github-bot-token.json")
}

const userAgent = "diane-assistant-bot"

// Config holds GitHub App configuration
type Config struct {
	AppID          string `json:"appId"`
	InstallationID string `json:"installationId"`
	PrivateKeyPath string `json:"privateKeyPath"`
	Owner          string `json:"owner"`
	Repo           string `json:"repo"`
}

// Provider implements GitHub bot tools
type Provider struct {
	config      *Config
	privateKey  *rsa.PrivateKey
	cachedToken string
	tokenExpiry time.Time
	mu          sync.Mutex
}

// NewProvider creates a new GitHub provider
func NewProvider() (*Provider, error) {
	// Read config
	configPath := getConfigPath()
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Read private key
	keyPath := config.PrivateKeyPath
	if !filepath.IsAbs(keyPath) {
		keyPath = filepath.Join(filepath.Dir(configPath), keyPath)
	}

	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	// Parse PEM block
	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	// Parse private key (PKCS8 format)
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS1 as fallback
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
	}

	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not RSA")
	}

	return &Provider{
		config:     &config,
		privateKey: rsaKey,
	}, nil
}

// CheckDependencies verifies GitHub App configuration exists
func (p *Provider) CheckDependencies() error {
	configPath := getConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("GitHub App config not found at %s", configPath)
	}
	return nil
}

// Tool represents an MCP tool definition
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// Tools returns the list of GitHub bot tools
func (p *Provider) Tools() []Tool {
	return []Tool{
		{
			Name:        "github_bot_comment_as_bot",
			Description: "Post comment on GitHub issue as Diane bot (separate identity from user). Use this to respond to issues so Diane has her own GitHub identity.",
			InputSchema: map[string]interface{}{
				"type":     "object",
				"required": []string{"issue_number", "body"},
				"properties": map[string]interface{}{
					"issue_number": map[string]interface{}{
						"type":        "number",
						"description": "Issue number",
					},
					"body": map[string]interface{}{
						"type":        "string",
						"description": "Comment markdown body",
					},
				},
			},
		},
		{
			Name:        "github_bot_react_as_bot",
			Description: "Add emoji reaction to comment as Diane bot. Use to acknowledge reading comments with separate bot identity.",
			InputSchema: map[string]interface{}{
				"type":     "object",
				"required": []string{"comment_id", "emoji"},
				"properties": map[string]interface{}{
					"comment_id": map[string]interface{}{
						"type":        "string",
						"description": "Comment ID from GitHub (numeric ID)",
					},
					"emoji": map[string]interface{}{
						"type":        "string",
						"description": "Reaction emoji: +1, eyes, hooray, rocket, confused, heart, laugh, tada",
					},
				},
			},
		},
		{
			Name:        "github_bot_manage_labels",
			Description: "Add or remove labels to organize and tidy up GitHub issues. Use to ensure correct categorization (purchase, finance, urgent, etc.) and remove incorrect labels.",
			InputSchema: map[string]interface{}{
				"type":     "object",
				"required": []string{"issue_number"},
				"properties": map[string]interface{}{
					"issue_number": map[string]interface{}{
						"type":        "number",
						"description": "Issue number",
					},
					"add_labels": map[string]interface{}{
						"type":        "string",
						"description": "Comma-separated labels to add (e.g., 'purchase,high-priority')",
					},
					"remove_labels": map[string]interface{}{
						"type":        "string",
						"description": "Comma-separated labels to remove (e.g., 'low-priority,stale')",
					},
				},
			},
		},
	}
}

// HasTool checks if a tool name belongs to this provider
func (p *Provider) HasTool(name string) bool {
	switch name {
	case "github_bot_comment_as_bot", "github_bot_react_as_bot", "github_bot_manage_labels":
		return true
	}
	return false
}

// Call executes a GitHub bot tool
func (p *Provider) Call(name string, args map[string]interface{}) (interface{}, error) {
	switch name {
	case "github_bot_comment_as_bot":
		return p.commentAsBot(args)
	case "github_bot_react_as_bot":
		return p.reactAsBot(args)
	case "github_bot_manage_labels":
		return p.manageLabels(args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// getInstallationToken gets or refreshes the GitHub installation token
func (p *Provider) getInstallationToken() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check cache
	if p.cachedToken != "" && time.Now().Before(p.tokenExpiry) {
		return p.cachedToken, nil
	}

	// Generate JWT
	now := time.Now()
	claims := jwt.MapClaims{
		"iat": now.Add(-60 * time.Second).Unix(), // Clock skew tolerance
		"exp": now.Add(10 * time.Minute).Unix(),
		"iss": p.config.AppID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	jwtStr, err := token.SignedString(p.privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	// Request installation access token
	url := fmt.Sprintf("https://api.github.com/app/installations/%s/access_tokens", p.config.InstallationID)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+jwtStr)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", userAgent)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get installation token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get installation token: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	p.cachedToken = result.Token
	p.tokenExpiry = time.Now().Add(55 * time.Minute) // Refresh 5 minutes early

	return p.cachedToken, nil
}

// commentAsBot posts a comment on an issue as the bot
func (p *Provider) commentAsBot(args map[string]interface{}) (interface{}, error) {
	issueNumber, ok := args["issue_number"].(float64)
	if !ok {
		return nil, fmt.Errorf("issue_number is required")
	}
	body, ok := args["body"].(string)
	if !ok || body == "" {
		return nil, fmt.Errorf("body is required")
	}

	token, err := p.getInstallationToken()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d/comments",
		p.config.Owner, p.config.Repo, int(issueNumber))

	payload, _ := json.Marshal(map[string]string{"body": body})
	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", userAgent)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to post comment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error: %s - %s", resp.Status, string(respBody))
	}

	var result struct {
		HTMLURL string `json:"html_url"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	return textContent(fmt.Sprintf("✅ Comment posted as diane-assistant[bot]: %s", result.HTMLURL)), nil
}

// reactAsBot adds a reaction to a comment
func (p *Provider) reactAsBot(args map[string]interface{}) (interface{}, error) {
	commentID, ok := args["comment_id"].(string)
	if !ok || commentID == "" {
		return nil, fmt.Errorf("comment_id is required")
	}
	emoji, ok := args["emoji"].(string)
	if !ok || emoji == "" {
		return nil, fmt.Errorf("emoji is required")
	}

	token, err := p.getInstallationToken()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/comments/%s/reactions",
		p.config.Owner, p.config.Repo, commentID)

	payload, _ := json.Marshal(map[string]string{"content": emoji})
	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", userAgent)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to add reaction: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error: %s - %s", resp.Status, string(respBody))
	}

	return textContent(fmt.Sprintf("✅ Added %s reaction as diane-assistant[bot]", emoji)), nil
}

// manageLabels adds or removes labels from an issue
func (p *Provider) manageLabels(args map[string]interface{}) (interface{}, error) {
	issueNumber, ok := args["issue_number"].(float64)
	if !ok {
		return nil, fmt.Errorf("issue_number is required")
	}

	token, err := p.getInstallationToken()
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	var added, removed []string

	// Add labels
	if addLabels, ok := args["add_labels"].(string); ok && addLabels != "" {
		labels := splitLabels(addLabels)
		url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d/labels",
			p.config.Owner, p.config.Repo, int(issueNumber))

		payload, _ := json.Marshal(map[string][]string{"labels": labels})
		req, _ := http.NewRequest("POST", url, bytes.NewReader(payload))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", userAgent)

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to add labels: %w", err)
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
			added = labels
		}
	}

	// Remove labels
	if removeLabels, ok := args["remove_labels"].(string); ok && removeLabels != "" {
		labels := splitLabels(removeLabels)
		for _, label := range labels {
			url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d/labels/%s",
				p.config.Owner, p.config.Repo, int(issueNumber), label)

			req, _ := http.NewRequest("DELETE", url, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Accept", "application/vnd.github+json")
			req.Header.Set("User-Agent", userAgent)

			resp, err := client.Do(req)
			if err != nil {
				continue
			}
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
				removed = append(removed, label)
			}
		}
	}

	return textContent(fmt.Sprintf("✅ Labels updated - Added: [%s], Removed: [%s]",
		joinLabels(added), joinLabels(removed))), nil
}

// splitLabels splits comma-separated labels and trims whitespace
func splitLabels(s string) []string {
	var labels []string
	for _, l := range bytes.Split([]byte(s), []byte(",")) {
		label := string(bytes.TrimSpace(l))
		if label != "" {
			labels = append(labels, label)
		}
	}
	return labels
}

// joinLabels joins labels with commas
func joinLabels(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	result := labels[0]
	for _, l := range labels[1:] {
		result += ", " + l
	}
	return result
}

// textContent formats result as MCP text content
func textContent(text string) map[string]interface{} {
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": text,
			},
		},
	}
}
