package slave

import (
	"encoding/json"
	"fmt"
	"time"
)

// SlaveProxyClient implements the mcpproxy.Client interface for a slave connection
// This allows slaves to be registered with the MCP proxy
type SlaveProxyClient struct {
	hostname   string
	conn       *SlaveConnection
	server     *Server
	notifyChan chan string
	lastError  string
}

// NewSlaveProxyClient creates a new slave proxy client
func NewSlaveProxyClient(hostname string, conn *SlaveConnection, server *Server) *SlaveProxyClient {
	return &SlaveProxyClient{
		hostname:   hostname,
		conn:       conn,
		server:     server,
		notifyChan: make(chan string, 10),
	}
}

// GetName returns the client name
func (c *SlaveProxyClient) GetName() string {
	return c.hostname
}

// ListTools returns the list of available tools
func (c *SlaveProxyClient) ListTools() ([]map[string]interface{}, error) {
	return c.ListToolsWithTimeout(10 * time.Second)
}

// ListToolsWithTimeout returns the list of tools with a custom timeout
func (c *SlaveProxyClient) ListToolsWithTimeout(timeout time.Duration) ([]map[string]interface{}, error) {
	c.conn.mu.RLock()
	tools := c.conn.Tools
	c.conn.mu.RUnlock()

	return tools, nil
}

// CallTool calls a tool on the slave
func (c *SlaveProxyClient) CallTool(toolName string, arguments map[string]interface{}) (json.RawMessage, error) {
	// Generate call ID
	callID := fmt.Sprintf("%s-%d", c.hostname, time.Now().UnixNano())

	// Send tool call to slave via server and wait for response
	result, err := c.server.SendToolCall(c.hostname, callID, toolName, arguments)
	if err != nil {
		return nil, fmt.Errorf("failed to call tool: %w", err)
	}

	return result, nil
}

// ListPrompts returns the list of available prompts
func (c *SlaveProxyClient) ListPrompts() ([]map[string]interface{}, error) {
	// Prompts not supported on slaves for now
	return []map[string]interface{}{}, nil
}

// GetPrompt retrieves a specific prompt and fills in its template
func (c *SlaveProxyClient) GetPrompt(name string, arguments map[string]string) (json.RawMessage, error) {
	return nil, fmt.Errorf("prompts not supported on remote slaves")
}

// ListResources returns the list of available resources
func (c *SlaveProxyClient) ListResources() ([]map[string]interface{}, error) {
	// Resources not supported on slaves for now
	return []map[string]interface{}{}, nil
}

// ReadResource reads the contents of a specific resource
func (c *SlaveProxyClient) ReadResource(uri string) (json.RawMessage, error) {
	return nil, fmt.Errorf("resources not supported on remote slaves")
}

// IsConnected returns true if the client is connected
func (c *SlaveProxyClient) IsConnected() bool {
	return c.conn.Status == StatusConnected
}

// GetCachedToolCount returns the cached tool count (-1 if not cached)
func (c *SlaveProxyClient) GetCachedToolCount() int {
	c.conn.mu.RLock()
	defer c.conn.mu.RUnlock()
	return len(c.conn.Tools)
}

// GetCachedPromptCount returns the cached prompt count (-1 if not cached)
func (c *SlaveProxyClient) GetCachedPromptCount() int {
	return 0 // Prompts not supported
}

// GetCachedResourceCount returns the cached resource count (-1 if not cached)
func (c *SlaveProxyClient) GetCachedResourceCount() int {
	return 0 // Resources not supported
}

// InvalidateToolCache marks the tool cache as invalid
func (c *SlaveProxyClient) InvalidateToolCache() {
	// Tools are managed by the registry, cache is always fresh
}

// TriggerAsyncRefresh starts a background cache refresh
func (c *SlaveProxyClient) TriggerAsyncRefresh(timeout time.Duration) bool {
	// Tools are managed by the registry, no need to refresh
	return false
}

// NotificationChan returns the channel for receiving notifications
func (c *SlaveProxyClient) NotificationChan() <-chan string {
	return c.notifyChan
}

// GetLastError returns the last error message
func (c *SlaveProxyClient) GetLastError() string {
	return c.lastError
}

// SetError sets the last error message
func (c *SlaveProxyClient) SetError(err string) {
	c.lastError = err
}

// GetStderrOutput returns stderr output (for stdio clients)
func (c *SlaveProxyClient) GetStderrOutput() string {
	return ""
}

// Close closes the client connection
func (c *SlaveProxyClient) Close() error {
	// Don't close the actual connection, just clean up the proxy client
	// The connection is managed by the registry
	return nil
}

// GetDisconnectChan returns a channel that is closed when the slave disconnects
func (c *SlaveProxyClient) GetDisconnectChan() <-chan struct{} {
	// Slave connections are managed by the registry and reconnect automatically
	// Return a channel that never closes
	return make(chan struct{})
}
