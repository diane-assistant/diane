package mcpproxy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
)

// MCPClient represents a connection to an MCP server
type MCPClient struct {
	Name       string
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	stderr     io.ReadCloser
	encoder    *json.Encoder
	decoder    *json.Decoder
	mu         sync.Mutex
	notifyChan chan string // Channel for notifications (method names)
	responseCh chan MCPResponse
	nextID     int
	pendingMu  sync.Mutex
	pending    map[interface{}]chan MCPResponse
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

	// Start goroutine to log stderr output
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Printf("[%s stderr] %s", name, scanner.Text())
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
				log.Printf("[%s] Error reading message: %v", c.Name, err)
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
			c.pendingMu.Lock()
			if ch, ok := c.pending[msg.ID]; ok {
				ch <- MCPResponse{
					JSONRPC: msg.JSONRPC,
					ID:      msg.ID,
					Result:  msg.Result,
					Error:   msg.Error,
				}
				delete(c.pending, msg.ID)
			} else {
				log.Printf("[%s] Received response for unknown request ID: %v", c.Name, msg.ID)
			}
			c.pendingMu.Unlock()
		} else if msg.Method != "" {
			// It's a notification - send to notification channel
			log.Printf("[%s] Received notification: %s", c.Name, msg.Method)
			select {
			case c.notifyChan <- msg.Method:
			default:
				log.Printf("[%s] Notification channel full, dropping: %s", c.Name, msg.Method)
			}
		}
	}
}

// sendRequest sends a request and waits for response asynchronously
func (c *MCPClient) sendRequest(method string, params json.RawMessage) (json.RawMessage, error) {
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

	// Wait for response
	resp, ok := <-respCh
	if !ok {
		return nil, fmt.Errorf("connection closed while waiting for response")
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("%s error: %s", method, resp.Error.Message)
	}

	return resp.Result, nil
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
	result, err := c.sendRequest("tools/list", nil)
	if err != nil {
		return nil, err
	}

	var toolsResult struct {
		Tools []map[string]interface{} `json:"tools"`
	}
	if err := json.Unmarshal(result, &toolsResult); err != nil {
		return nil, fmt.Errorf("failed to parse tools: %w", err)
	}

	return toolsResult.Tools, nil
}

// CallTool calls a tool on the MCP server
func (c *MCPClient) CallTool(toolName string, arguments map[string]interface{}) (json.RawMessage, error) {
	params, err := json.Marshal(map[string]interface{}{
		"name":      toolName,
		"arguments": arguments,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	return c.sendRequest("tools/call", params)
}

// NotificationChan returns the channel for receiving notifications from this client
func (c *MCPClient) NotificationChan() <-chan string {
	return c.notifyChan
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
			log.Printf("Failed to kill process for %s: %v", c.Name, err)
		}
		c.cmd.Wait()
	}
	return nil
}

// getPath returns the PATH environment variable with common locations
func getPath() string {
	return "/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin:/opt/homebrew/bin"
}
