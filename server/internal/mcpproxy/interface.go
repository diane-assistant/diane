package mcpproxy

import (
	"encoding/json"
	"time"
)

// Client is the interface for MCP clients (stdio, SSE, HTTP)
type Client interface {
	// Name returns the client name
	GetName() string

	// ListTools returns the list of available tools
	ListTools() ([]map[string]interface{}, error)

	// ListToolsWithTimeout returns the list of tools with a custom timeout
	ListToolsWithTimeout(timeout time.Duration) ([]map[string]interface{}, error)

	// CallTool calls a tool on the MCP server
	CallTool(toolName string, arguments map[string]interface{}) (json.RawMessage, error)

	// ListPrompts returns the list of available prompts
	ListPrompts() ([]map[string]interface{}, error)

	// GetPrompt retrieves a specific prompt and fills in its template
	GetPrompt(name string, arguments map[string]string) (json.RawMessage, error)

	// ListResources returns the list of available resources
	ListResources() ([]map[string]interface{}, error)

	// ReadResource reads the contents of a specific resource
	ReadResource(uri string) (json.RawMessage, error)

	// IsConnected returns true if the client is connected
	IsConnected() bool

	// GetCachedToolCount returns the cached tool count (-1 if not cached)
	GetCachedToolCount() int

	// GetCachedPromptCount returns the cached prompt count (-1 if not cached)
	GetCachedPromptCount() int

	// GetCachedResourceCount returns the cached resource count (-1 if not cached)
	GetCachedResourceCount() int

	// InvalidateToolCache marks the tool cache as invalid
	InvalidateToolCache()

	// TriggerAsyncRefresh starts a background cache refresh
	TriggerAsyncRefresh(timeout time.Duration) bool

	// NotificationChan returns the channel for receiving notifications
	NotificationChan() <-chan string

	// GetLastError returns the last error message
	GetLastError() string

	// SetError sets the last error message
	SetError(err string)

	// GetStderrOutput returns stderr output (for stdio clients)
	GetStderrOutput() string

	// Close closes the client connection
	Close() error
}
