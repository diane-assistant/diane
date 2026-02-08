package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Client is a client for the Diane API
type Client struct {
	httpClient *http.Client
	socketPath string
}

// NewClient creates a new API client
func NewClient() *Client {
	socketPath := GetSocketPath()

	return &Client{
		socketPath: socketPath,
		httpClient: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return net.Dial("unix", socketPath)
				},
			},
			Timeout: 10 * time.Second,
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
