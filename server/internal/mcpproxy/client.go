package mcpproxy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// MCPClient represents a connection to an MCP server
type MCPClient struct {
	Name                string
	cmd                 *exec.Cmd
	stdin               io.WriteCloser
	stdout              io.ReadCloser
	stderr              io.ReadCloser
	encoder             *json.Encoder
	decoder             *json.Decoder
	mu                  sync.Mutex
	notifyChan          chan string // Channel for notifications (method names)
	responseCh          chan MCPResponse
	nextID              int
	pendingMu           sync.Mutex
	pending             map[interface{}]chan MCPResponse
	cachedToolCount     int    // Cached tool count for fast status queries
	toolCountValid      bool   // Whether cached count is valid
	cachedPromptCount   int    // Cached prompt count for fast status queries
	promptCountValid    bool   // Whether cached prompt count is valid
	cachedResourceCount int    // Cached resource count for fast status queries
	resourceCountValid  bool   // Whether cached resource count is valid
	refreshing          bool   // Whether a cache refresh is in progress
	lastError           string // Last error message
	stderrOutput        string // Last stderr output (truncated)
}

// MCPRequest represents a JSON-RPC request
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// MCPResponse represents a JSON-RPC response
type MCPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

// MCPError represents a JSON-RPC error
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCPNotification represents a JSON-RPC notification (no ID field)
type MCPNotification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// MCPMessage is a generic JSON-RPC message that could be response or notification
type MCPMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`     // Present for responses
	Method  string          `json:"method,omitempty"` // Present for notifications
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// NewMCPClient creates a new MCP client and starts the server process
func NewMCPClient(name string, command string, args []string, env map[string]string) (*MCPClient, error) {
	cmd := exec.Command(command, args...)

	// Set environment variables
	cmd.Env = append(cmd.Env, "PATH="+getPath())
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdin: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	client := &MCPClient{
		Name:       name,
		cmd:        cmd,
		stdin:      stdin,
		stdout:     stdout,
		stderr:     stderr,
		encoder:    json.NewEncoder(stdin),
		decoder:    json.NewDecoder(bufio.NewReader(stdout)),
		notifyChan: make(chan string, 10), // Buffered channel for notifications
		nextID:     1,                     // Start at 1 (0 is used by initialize)
		pending:    make(map[interface{}]chan MCPResponse),
	}

	// Start goroutine to log stderr output and capture it
	go func() {
		scanner := bufio.NewScanner(stderr)
		var stderrLines []string
		for scanner.Scan() {
			line := scanner.Text()
			slog.Debug("MCP server stderr", "server", name, "line", line)
			// Keep last few lines for error display
			stderrLines = append(stderrLines, line)
			if len(stderrLines) > 10 {
				stderrLines = stderrLines[1:]
			}
			client.mu.Lock()
			client.stderrOutput = ""
			for _, l := range stderrLines {
				if client.stderrOutput != "" {
					client.stderrOutput += "\n"
				}
				client.stderrOutput += l
			}
			client.mu.Unlock()
		}
	}()

	// Initialize the MCP connection
	if err := client.initialize(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to initialize: %w", err)
	}

	// Start background goroutine to read all messages from stdout
	go client.messageLoop()

	return client, nil
}

// messageLoop reads all messages from stdout and routes them appropriately
func (c *MCPClient) messageLoop() {
	for {
		var msg MCPMessage
		if err := c.decoder.Decode(&msg); err != nil {
			if err != io.EOF {
				slog.Debug("Error reading message from MCP server", "server", c.Name, "error", err)
			}
			// Connection closed, cleanup pending requests
			c.pendingMu.Lock()
			for id, ch := range c.pending {
				close(ch)
				delete(c.pending, id)
			}
			c.pendingMu.Unlock()
			return
		}

		// Check if it's a response (has ID) or notification (has method, no ID)
		if msg.ID != nil {
			// It's a response - route to pending request
			// Normalize ID to int (JSON decodes numbers as float64)
			var reqID interface{} = msg.ID
			if f, ok := msg.ID.(float64); ok {
				reqID = int(f)
			}
			c.pendingMu.Lock()
			if ch, ok := c.pending[reqID]; ok {
				ch <- MCPResponse{
					JSONRPC: msg.JSONRPC,
					ID:      msg.ID,
					Result:  msg.Result,
					Error:   msg.Error,
				}
				delete(c.pending, reqID)
			} else {
				slog.Warn("Received response for unknown request ID", "server", c.Name, "id", msg.ID)
			}
			c.pendingMu.Unlock()
		} else if msg.Method != "" {
			// It's a notification - send to notification channel
			slog.Debug("Received notification from MCP server", "server", c.Name, "method", msg.Method)
			select {
			case c.notifyChan <- msg.Method:
			default:
				slog.Warn("Notification channel full, dropping notification", "server", c.Name, "method", msg.Method)
			}
		}
	}
}

// sendRequest sends a request and waits for response with timeout
func (c *MCPClient) sendRequest(method string, params json.RawMessage) (json.RawMessage, error) {
	return c.sendRequestWithTimeout(method, params, 5*time.Second)
}

// sendRequestWithTimeout sends a request and waits for response with a specific timeout
func (c *MCPClient) sendRequestWithTimeout(method string, params json.RawMessage, timeout time.Duration) (json.RawMessage, error) {
	// Generate unique request ID
	c.mu.Lock()
	c.nextID++
	reqID := c.nextID
	c.mu.Unlock()

	// Create response channel
	respCh := make(chan MCPResponse, 1)
	c.pendingMu.Lock()
	c.pending[reqID] = respCh
	c.pendingMu.Unlock()

	// Send request
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      reqID,
		Method:  method,
		Params:  params,
	}

	c.mu.Lock()
	err := c.encoder.Encode(req)
	c.mu.Unlock()

	if err != nil {
		c.pendingMu.Lock()
		delete(c.pending, reqID)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("failed to send %s: %w", method, err)
	}

	// Wait for response with timeout
	select {
	case resp, ok := <-respCh:
		if !ok {
			return nil, fmt.Errorf("connection closed while waiting for response")
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("%s error: %s", method, resp.Error.Message)
		}
		return resp.Result, nil
	case <-time.After(timeout):
		c.pendingMu.Lock()
		delete(c.pending, reqID)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("%s timed out after %v", method, timeout)
	}
}

// initialize sends the initialize request to the MCP server
func (c *MCPClient) initialize() error {
	params := json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"diane","version":"1.0.0"}}`)

	// For initialize, we can't use the async messageLoop yet (it's not started)
	// So we do a synchronous request here
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      0,
		Method:  "initialize",
		Params:  params,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.encoder.Encode(req); err != nil {
		return fmt.Errorf("failed to send initialize: %w", err)
	}

	var resp MCPResponse
	if err := c.decoder.Decode(&resp); err != nil {
		return fmt.Errorf("failed to read initialize response: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("initialize error: %s", resp.Error.Message)
	}

	return nil
}

// ListTools requests the list of tools from the MCP server
func (c *MCPClient) ListTools() ([]map[string]interface{}, error) {
	return c.ListToolsWithTimeout(5 * time.Second)
}

// ListToolsWithTimeout requests the list of tools with a custom timeout
func (c *MCPClient) ListToolsWithTimeout(timeout time.Duration) ([]map[string]interface{}, error) {
	result, err := c.sendRequestWithTimeout("tools/list", nil, timeout)
	if err != nil {
		return nil, err
	}

	var toolsResult struct {
		Tools []map[string]interface{} `json:"tools"`
	}
	if err := json.Unmarshal(result, &toolsResult); err != nil {
		return nil, fmt.Errorf("failed to parse tools: %w", err)
	}

	// Cache the tool count
	c.mu.Lock()
	c.cachedToolCount = len(toolsResult.Tools)
	c.toolCountValid = true
	c.mu.Unlock()

	return toolsResult.Tools, nil
}

// GetCachedToolCount returns the cached tool count (fast, non-blocking)
// Returns -1 if no cached value is available
func (c *MCPClient) GetCachedToolCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.toolCountValid {
		return c.cachedToolCount
	}
	return -1
}

// InvalidateToolCache marks the tool cache as invalid
func (c *MCPClient) InvalidateToolCache() {
	c.mu.Lock()
	c.toolCountValid = false
	c.mu.Unlock()
}

// TriggerAsyncRefresh starts a background cache refresh if one isn't already running
// Returns true if a refresh was started, false if one was already in progress
func (c *MCPClient) TriggerAsyncRefresh(timeout time.Duration) bool {
	c.mu.Lock()
	if c.refreshing {
		c.mu.Unlock()
		return false
	}
	c.refreshing = true
	c.mu.Unlock()

	go func() {
		_, _ = c.ListToolsWithTimeout(timeout)
		_, _ = c.ListPrompts()
		_, _ = c.ListResources()
		c.mu.Lock()
		c.refreshing = false
		c.mu.Unlock()
	}()
	return true
}

// CallTool calls a tool on the MCP server.
// Uses a 30s timeout (matching SSE/HTTP clients) since tool execution can
// involve multiple network calls (e.g., batch artifact creation).
func (c *MCPClient) CallTool(toolName string, arguments map[string]interface{}) (json.RawMessage, error) {
	params, err := json.Marshal(map[string]interface{}{
		"name":      toolName,
		"arguments": arguments,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	return c.sendRequestWithTimeout("tools/call", params, 30*time.Second)
}

// ListPrompts requests the list of prompts from the MCP server
func (c *MCPClient) ListPrompts() ([]map[string]interface{}, error) {
	result, err := c.sendRequestWithTimeout("prompts/list", nil, 5*time.Second)
	if err != nil {
		return nil, err
	}

	var promptsResult struct {
		Prompts []map[string]interface{} `json:"prompts"`
	}
	if err := json.Unmarshal(result, &promptsResult); err != nil {
		return nil, fmt.Errorf("failed to parse prompts: %w", err)
	}

	// Cache the prompt count
	c.mu.Lock()
	c.cachedPromptCount = len(promptsResult.Prompts)
	c.promptCountValid = true
	c.mu.Unlock()

	return promptsResult.Prompts, nil
}

// GetPrompt retrieves a specific prompt and fills in its template
func (c *MCPClient) GetPrompt(name string, arguments map[string]string) (json.RawMessage, error) {
	params, err := json.Marshal(map[string]interface{}{
		"name":      name,
		"arguments": arguments,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	return c.sendRequestWithTimeout("prompts/get", params, 5*time.Second)
}

// ListResources requests the list of resources from the MCP server
func (c *MCPClient) ListResources() ([]map[string]interface{}, error) {
	result, err := c.sendRequestWithTimeout("resources/list", nil, 5*time.Second)
	if err != nil {
		return nil, err
	}

	var resourcesResult struct {
		Resources []map[string]interface{} `json:"resources"`
	}
	if err := json.Unmarshal(result, &resourcesResult); err != nil {
		return nil, fmt.Errorf("failed to parse resources: %w", err)
	}

	// Cache the resource count
	c.mu.Lock()
	c.cachedResourceCount = len(resourcesResult.Resources)
	c.resourceCountValid = true
	c.mu.Unlock()

	return resourcesResult.Resources, nil
}

// ReadResource reads the contents of a specific resource
func (c *MCPClient) ReadResource(uri string) (json.RawMessage, error) {
	params, err := json.Marshal(map[string]interface{}{
		"uri": uri,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	return c.sendRequestWithTimeout("resources/read", params, 5*time.Second)
}

// GetCachedPromptCount returns the cached prompt count (fast, non-blocking)
// Returns -1 if no cached value is available
func (c *MCPClient) GetCachedPromptCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.promptCountValid {
		return c.cachedPromptCount
	}
	return -1
}

// GetCachedResourceCount returns the cached resource count (fast, non-blocking)
// Returns -1 if no cached value is available
func (c *MCPClient) GetCachedResourceCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.resourceCountValid {
		return c.cachedResourceCount
	}
	return -1
}

// NotificationChan returns the channel for receiving notifications from this client
func (c *MCPClient) NotificationChan() <-chan string {
	return c.notifyChan
}

// GetName returns the client name
func (c *MCPClient) GetName() string {
	return c.Name
}

// IsConnected returns true if the MCP client process is still running
func (c *MCPClient) IsConnected() bool {
	if c.cmd == nil || c.cmd.Process == nil {
		return false
	}
	// Check if process has exited by sending signal 0
	// This is the standard Unix way to check if a process is alive
	err := c.cmd.Process.Signal(syscall.Signal(0))
	return err == nil
}

// GetLastError returns the last error message for this client
func (c *MCPClient) GetLastError() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastError
}

// SetError sets the last error message
func (c *MCPClient) SetError(err string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastError = err
}

// GetStderrOutput returns the last stderr output
func (c *MCPClient) GetStderrOutput() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stderrOutput
}

// Close terminates the MCP server process
func (c *MCPClient) Close() error {
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.stdout != nil {
		c.stdout.Close()
	}
	if c.stderr != nil {
		c.stderr.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		if err := c.cmd.Process.Kill(); err != nil {
			slog.Warn("Failed to kill process", "server", c.Name, "error", err)
		}
		c.cmd.Wait()
	}
	return nil
}

// getPath returns the PATH environment variable with common locations
func getPath() string {
	return "/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin:/opt/homebrew/bin"
}
