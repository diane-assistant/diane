package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/diane-assistant/diane/internal/acp"
)

// Client is a client for the Diane API
type Client struct {
	httpClient *http.Client
	socketPath string
}

// NewClient creates a new API client
func NewClient() *Client {
	return NewClientWithTimeout(10 * time.Second)
}

// NewClientWithTimeout creates a new API client with a custom timeout
func NewClientWithTimeout(timeout time.Duration) *Client {
	socketPath := GetSocketPath()

	return &Client{
		socketPath: socketPath,
		httpClient: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return net.Dial("unix", socketPath)
				},
			},
			Timeout: timeout,
		},
	}
}

// Health checks if the API server is responding
func (c *Client) Health() error {
	resp, err := c.httpClient.Get("http://unix/health")
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: status %d", resp.StatusCode)
	}
	return nil
}

// GetStatus returns the full Diane status
func (c *Client) GetStatus() (*Status, error) {
	resp, err := c.httpClient.Get("http://unix/status")
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status request failed: %d", resp.StatusCode)
	}

	var status Status
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode status: %w", err)
	}

	return &status, nil
}

// GetMCPServers returns the list of MCP servers
func (c *Client) GetMCPServers() ([]MCPServerStatus, error) {
	resp, err := c.httpClient.Get("http://unix/mcp-servers")
	if err != nil {
		return nil, fmt.Errorf("failed to get MCP servers: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MCP servers request failed: %d", resp.StatusCode)
	}

	var servers []MCPServerStatus
	if err := json.NewDecoder(resp.Body).Decode(&servers); err != nil {
		return nil, fmt.Errorf("failed to decode servers: %w", err)
	}

	return servers, nil
}

// RestartMCPServer restarts a specific MCP server
func (c *Client) RestartMCPServer(name string) error {
	url := fmt.Sprintf("http://unix/mcp-servers/%s/restart", name)
	resp, err := c.httpClient.Post(url, "application/json", nil)
	if err != nil {
		return fmt.Errorf("failed to restart server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("restart failed: %s", errResp.Error)
	}

	return nil
}

// ReloadConfig reloads the MCP configuration
func (c *Client) ReloadConfig() error {
	resp, err := c.httpClient.Post("http://unix/reload", "application/json", nil)
	if err != nil {
		return fmt.Errorf("failed to reload config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("reload failed: %s", errResp.Error)
	}

	return nil
}

// IsRunning checks if Diane is running by attempting to connect to the socket
func (c *Client) IsRunning() bool {
	return c.Health() == nil
}

// ListAgents returns the list of configured ACP agents
func (c *Client) ListAgents() ([]acp.AgentConfig, error) {
	resp, err := c.httpClient.Get("http://unix/agents")
	if err != nil {
		return nil, fmt.Errorf("failed to get agents: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("agents request failed: %d", resp.StatusCode)
	}

	var agents []acp.AgentConfig
	if err := json.NewDecoder(resp.Body).Decode(&agents); err != nil {
		return nil, fmt.Errorf("failed to decode agents: %w", err)
	}

	return agents, nil
}

// GetAgent returns a specific ACP agent
func (c *Client) GetAgent(name string) (*acp.AgentConfig, error) {
	url := fmt.Sprintf("http://unix/agents/%s", name)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("get agent failed: %s", errResp.Error)
	}

	var agent acp.AgentConfig
	if err := json.NewDecoder(resp.Body).Decode(&agent); err != nil {
		return nil, fmt.Errorf("failed to decode agent: %w", err)
	}

	return &agent, nil
}

// AddAgent adds a new ACP agent
func (c *Client) AddAgent(agent acp.AgentConfig) error {
	body, err := json.Marshal(agent)
	if err != nil {
		return fmt.Errorf("failed to marshal agent: %w", err)
	}

	resp, err := c.httpClient.Post("http://unix/agents", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to add agent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("add agent failed: %s", errResp.Error)
	}

	return nil
}

// RemoveAgent removes an ACP agent
func (c *Client) RemoveAgent(name string) error {
	url := fmt.Sprintf("http://unix/agents/%s", name)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to remove agent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("remove agent failed: %s", errResp.Error)
	}

	return nil
}

// ToggleAgent enables or disables an ACP agent
func (c *Client) ToggleAgent(name string, enabled bool) error {
	url := fmt.Sprintf("http://unix/agents/%s/toggle", name)
	body, _ := json.Marshal(map[string]bool{"enabled": enabled})
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to toggle agent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("toggle agent failed: %s", errResp.Error)
	}

	return nil
}

// TestAgent tests connectivity to an ACP agent
func (c *Client) TestAgent(name string) (*acp.AgentTestResult, error) {
	url := fmt.Sprintf("http://unix/agents/%s/test", name)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to test agent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("test agent failed: %s", errResp.Error)
	}

	var result acp.AgentTestResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode test result: %w", err)
	}

	return &result, nil
}

// RunAgent runs a prompt against an ACP agent
func (c *Client) RunAgent(name, prompt, remoteAgentName string) (*acp.Run, error) {
	url := fmt.Sprintf("http://unix/agents/%s/run", name)
	body, _ := json.Marshal(map[string]string{"prompt": prompt, "agent_name": remoteAgentName})
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to run agent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("run agent failed: %s", errResp.Error)
	}

	var run acp.Run
	if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
		return nil, fmt.Errorf("failed to decode run result: %w", err)
	}

	return &run, nil
}

// GetAgentLogs returns communication logs for an agent
func (c *Client) GetAgentLogs(agentName string, limit int) ([]AgentLog, error) {
	url := fmt.Sprintf("http://unix/agents/logs?limit=%d", limit)
	if agentName != "" {
		url += fmt.Sprintf("&agent_name=%s", agentName)
	}

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent logs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("agent logs request failed: %d", resp.StatusCode)
	}

	var logs []AgentLog
	if err := json.NewDecoder(resp.Body).Decode(&logs); err != nil {
		return nil, fmt.Errorf("failed to decode agent logs: %w", err)
	}

	return logs, nil
}

// ListGallery returns all agents from the gallery
func (c *Client) ListGallery(featured bool) ([]acp.GalleryEntry, error) {
	url := "http://unix/gallery"
	if featured {
		url += "?featured=true"
	}

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get gallery: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gallery request failed: %d", resp.StatusCode)
	}

	var entries []acp.GalleryEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("failed to decode gallery: %w", err)
	}

	return entries, nil
}

// GetGalleryAgent returns installation info for a gallery agent
func (c *Client) GetGalleryAgent(id string) (*acp.InstallInfo, error) {
	url := fmt.Sprintf("http://unix/gallery/%s", id)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get gallery agent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("get gallery agent failed: %s", errResp.Error)
	}

	var info acp.InstallInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode install info: %w", err)
	}

	return &info, nil
}

// InstallGalleryAgent configures an agent from the gallery
func (c *Client) InstallGalleryAgent(id string) error {
	return c.InstallGalleryAgentWithOptions(id, "", "", 0)
}

// InstallGalleryAgentWithOptions configures an agent from the gallery with custom name, workdir, and port
func (c *Client) InstallGalleryAgentWithOptions(id, name, workdir string, port int) error {
	url := fmt.Sprintf("http://unix/gallery/%s/install", id)

	body := map[string]interface{}{}
	if name != "" {
		body["name"] = name
	}
	if workdir != "" {
		body["workdir"] = workdir
	}
	if port > 0 {
		body["port"] = port
		body["type"] = "acp"
	}

	var reqBody io.Reader
	if len(body) > 0 {
		jsonBody, _ := json.Marshal(body)
		reqBody = bytes.NewReader(jsonBody)
	}

	resp, err := c.httpClient.Post(url, "application/json", reqBody)
	if err != nil {
		return fmt.Errorf("failed to install agent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("install agent failed: %s", errResp.Error)
	}

	return nil
}

// RefreshGallery refreshes the agent registry
func (c *Client) RefreshGallery() error {
	resp, err := c.httpClient.Post("http://unix/gallery", "application/json", nil)
	if err != nil {
		return fmt.Errorf("failed to refresh gallery: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("refresh gallery failed: %s", errResp.Error)
	}

	return nil
}

// ListOAuthServers returns all MCP servers with OAuth configuration
func (c *Client) ListOAuthServers() ([]OAuthServerInfo, error) {
	resp, err := c.httpClient.Get("http://unix/auth")
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuth servers: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OAuth servers request failed: %d", resp.StatusCode)
	}

	var servers []OAuthServerInfo
	if err := json.NewDecoder(resp.Body).Decode(&servers); err != nil {
		return nil, fmt.Errorf("failed to decode OAuth servers: %w", err)
	}

	return servers, nil
}

// GetOAuthStatus returns the OAuth status for a specific server
func (c *Client) GetOAuthStatus(serverName string) (map[string]interface{}, error) {
	url := fmt.Sprintf("http://unix/auth/%s", serverName)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuth status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("OAuth status failed: %s", errResp.Error)
	}

	var status map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode OAuth status: %w", err)
	}

	return status, nil
}

// StartOAuthLogin initiates the OAuth device flow for a server
func (c *Client) StartOAuthLogin(serverName string) (*DeviceCodeInfo, error) {
	url := fmt.Sprintf("http://unix/auth/%s/login", serverName)
	resp, err := c.httpClient.Post(url, "application/json", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start OAuth login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("OAuth login failed: %s", errResp.Error)
	}

	var deviceInfo DeviceCodeInfo
	if err := json.NewDecoder(resp.Body).Decode(&deviceInfo); err != nil {
		return nil, fmt.Errorf("failed to decode device info: %w", err)
	}

	return &deviceInfo, nil
}

// PollOAuthToken polls for the OAuth token after user authorization
func (c *Client) PollOAuthToken(serverName string, deviceCode string, interval int) error {
	url := fmt.Sprintf("http://unix/auth/%s/poll", serverName)
	body, _ := json.Marshal(map[string]interface{}{
		"device_code": deviceCode,
		"interval":    interval,
	})
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to poll OAuth token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("OAuth poll failed: %s", errResp.Error)
	}

	return nil
}

// LogoutOAuth removes the OAuth token for a server
func (c *Client) LogoutOAuth(serverName string) error {
	url := fmt.Sprintf("http://unix/auth/%s/logout", serverName)
	resp, err := c.httpClient.Post(url, "application/json", nil)
	if err != nil {
		return fmt.Errorf("failed to logout: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("logout failed: %s", errResp.Error)
	}

	return nil
}
