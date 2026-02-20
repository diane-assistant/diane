package mcpproxy

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// MasterToolCallFunc is a function that sends a tool call to the master via WebSocket
// and returns the result. The WSClient provides this function.
type MasterToolCallFunc func(serverName, toolName string, arguments map[string]interface{}) (json.RawMessage, error)

// MasterProxyClient implements the Client interface for tools that live on the master.
// It stores a cached list of tools received from the master via WebSocket and
// routes CallTool requests back through the WebSocket connection.
type MasterProxyClient struct {
	serverName     string // The name of the MCP server on the master (e.g. "specmcp-diane")
	tools          []map[string]interface{}
	mu             sync.RWMutex
	callFunc       MasterToolCallFunc
	notifyChan     chan string
	lastError      string
	disconnected   chan struct{}
	disconnectMu   sync.Mutex
	isDisconnected bool
}

// NewMasterProxyClient creates a new client that proxies tools from the master.
func NewMasterProxyClient(serverName string, tools []map[string]interface{}, callFunc MasterToolCallFunc) *MasterProxyClient {
	return &MasterProxyClient{
		serverName:   serverName,
		tools:        tools,
		callFunc:     callFunc,
		notifyChan:   make(chan string, 10),
		disconnected: make(chan struct{}),
	}
}

// GetName returns the server name (as known on the master).
func (c *MasterProxyClient) GetName() string {
	return c.serverName
}

// ListTools returns the cached tool list from the master.
func (c *MasterProxyClient) ListTools() ([]map[string]interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.tools, nil
}

// ListToolsWithTimeout returns the cached tool list (timeout unused since it's local cache).
func (c *MasterProxyClient) ListToolsWithTimeout(timeout time.Duration) ([]map[string]interface{}, error) {
	return c.ListTools()
}

// CallTool routes the call to the master via WebSocket.
func (c *MasterProxyClient) CallTool(toolName string, arguments map[string]interface{}) (json.RawMessage, error) {
	if c.callFunc == nil {
		return nil, fmt.Errorf("master tool call function not set")
	}
	return c.callFunc(c.serverName, toolName, arguments)
}

// UpdateTools replaces the cached tool list (called when master sends an update).
func (c *MasterProxyClient) UpdateTools(tools []map[string]interface{}) {
	c.mu.Lock()
	c.tools = tools
	c.mu.Unlock()
}

// ListPrompts returns empty (prompts not proxied from master for now).
func (c *MasterProxyClient) ListPrompts() ([]map[string]interface{}, error) {
	return []map[string]interface{}{}, nil
}

// GetPrompt is not supported for master-proxied servers.
func (c *MasterProxyClient) GetPrompt(name string, arguments map[string]string) (json.RawMessage, error) {
	return nil, fmt.Errorf("prompts not supported on master-proxied servers")
}

// ListResources returns empty (resources not proxied from master for now).
func (c *MasterProxyClient) ListResources() ([]map[string]interface{}, error) {
	return []map[string]interface{}{}, nil
}

// ReadResource is not supported for master-proxied servers.
func (c *MasterProxyClient) ReadResource(uri string) (json.RawMessage, error) {
	return nil, fmt.Errorf("resources not supported on master-proxied servers")
}

// IsConnected returns true (we consider master tools available while the WS is up).
func (c *MasterProxyClient) IsConnected() bool {
	c.disconnectMu.Lock()
	defer c.disconnectMu.Unlock()
	return !c.isDisconnected
}

// GetCachedToolCount returns the number of cached tools.
func (c *MasterProxyClient) GetCachedToolCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.tools)
}

// GetCachedPromptCount returns 0 (prompts not supported).
func (c *MasterProxyClient) GetCachedPromptCount() int {
	return 0
}

// GetCachedResourceCount returns 0 (resources not supported).
func (c *MasterProxyClient) GetCachedResourceCount() int {
	return 0
}

// InvalidateToolCache is a no-op; tools are pushed by the master.
func (c *MasterProxyClient) InvalidateToolCache() {}

// TriggerAsyncRefresh is a no-op; tools are pushed by the master.
func (c *MasterProxyClient) TriggerAsyncRefresh(timeout time.Duration) bool {
	return false
}

// NotificationChan returns the notification channel.
func (c *MasterProxyClient) NotificationChan() <-chan string {
	return c.notifyChan
}

// GetLastError returns the last error message.
func (c *MasterProxyClient) GetLastError() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastError
}

// SetError sets the last error message.
func (c *MasterProxyClient) SetError(err string) {
	c.mu.Lock()
	c.lastError = err
	c.mu.Unlock()
}

// GetStderrOutput returns empty (no stderr for WebSocket proxied tools).
func (c *MasterProxyClient) GetStderrOutput() string {
	return ""
}

// Close marks the client as disconnected.
func (c *MasterProxyClient) Close() error {
	c.disconnectMu.Lock()
	defer c.disconnectMu.Unlock()
	if !c.isDisconnected {
		c.isDisconnected = true
		close(c.disconnected)
	}
	return nil
}

// GetDisconnectChan returns a channel closed when the master connection drops.
func (c *MasterProxyClient) GetDisconnectChan() <-chan struct{} {
	return c.disconnected
}
