// Package acp provides a client for the Agent Client Protocol (ACP).
// ACP is an open protocol for standardized communication between code editors and AI coding agents.
// Spec: https://agentclientprotocol.com
//
// This file implements the stdio transport, where the agent runs as a subprocess
// and communicates via JSON-RPC over stdin/stdout.
package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// JSON-RPC 2.0 message types

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCNotification represents a JSON-RPC 2.0 notification (no ID, no response expected)
type JSONRPCNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (e *JSONRPCError) Error() string {
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

// ACP Protocol Types

// ClientInfo describes the client implementation
type ClientInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version"`
}

// AgentInfo describes the agent implementation
type AgentInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version"`
}

// ClientCapabilities describes what the client supports
type ClientCapabilities struct {
	FS       *FSCapabilities `json:"fs,omitempty"`
	Terminal bool            `json:"terminal,omitempty"`
}

// FSCapabilities describes file system capabilities
type FSCapabilities struct {
	ReadTextFile  bool `json:"readTextFile,omitempty"`
	WriteTextFile bool `json:"writeTextFile,omitempty"`
}

// AgentCapabilities describes what the agent supports
type AgentCapabilities struct {
	LoadSession        bool                `json:"loadSession,omitempty"`
	PromptCapabilities *PromptCapabilities `json:"promptCapabilities,omitempty"`
	MCPCapabilities    *MCPCapabilities    `json:"mcp,omitempty"`
}

// PromptCapabilities describes what content types can be in prompts
type PromptCapabilities struct {
	Image           bool `json:"image,omitempty"`
	Audio           bool `json:"audio,omitempty"`
	EmbeddedContext bool `json:"embeddedContext,omitempty"`
}

// MCPCapabilities describes MCP transport support
type MCPCapabilities struct {
	HTTP bool `json:"http,omitempty"`
	SSE  bool `json:"sse,omitempty"`
}

// InitializeParams are the parameters for the initialize method
type InitializeParams struct {
	ProtocolVersion    int                `json:"protocolVersion"`
	ClientCapabilities ClientCapabilities `json:"clientCapabilities"`
	ClientInfo         ClientInfo         `json:"clientInfo"`
}

// InitializeResult is the response from initialize
type InitializeResult struct {
	ProtocolVersion   int               `json:"protocolVersion"`
	AgentCapabilities AgentCapabilities `json:"agentCapabilities"`
	AgentInfo         AgentInfo         `json:"agentInfo"`
	AuthMethods       []interface{}     `json:"authMethods,omitempty"`
}

// SessionNewParams are the parameters for session/new
type SessionNewParams struct {
	CWD        string      `json:"cwd"`
	MCPServers []MCPServer `json:"mcpServers"` // Required, must always be present
}

// MCPServer describes an MCP server to connect to
type MCPServer struct {
	Name    string        `json:"name"`
	Command string        `json:"command,omitempty"`
	Args    []string      `json:"args,omitempty"`
	Env     []EnvVariable `json:"env,omitempty"`
	// For HTTP/SSE transports
	Type    string       `json:"type,omitempty"`
	URL     string       `json:"url,omitempty"`
	Headers []HTTPHeader `json:"headers,omitempty"`
}

// EnvVariable is an environment variable
type EnvVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// HTTPHeader is an HTTP header
type HTTPHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// SessionNewResult is the response from session/new
type SessionNewResult struct {
	SessionID string `json:"sessionId"`
}

// ContentBlock represents content in a message
type ContentBlock struct {
	Type     string    `json:"type"` // "text", "image", "resource", etc.
	Text     string    `json:"text,omitempty"`
	Resource *Resource `json:"resource,omitempty"`
}

// Resource represents an embedded resource
type Resource struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
}

// SessionPromptParams are the parameters for session/prompt
type SessionPromptParams struct {
	SessionID string         `json:"sessionId"`
	Prompt    []ContentBlock `json:"prompt"`
}

// SessionPromptResult is the response from session/prompt
type SessionPromptResult struct {
	StopReason string `json:"stopReason"` // "end_turn", "max_tokens", "cancelled", etc.
}

// SessionUpdateParams represents a session/update notification
type SessionUpdateParams struct {
	SessionID string        `json:"sessionId"`
	Update    SessionUpdate `json:"update"`
}

// SessionUpdate represents different types of updates
type SessionUpdate struct {
	SessionUpdate string        `json:"sessionUpdate"` // "agent_message_chunk", "tool_call", etc.
	Content       *ContentBlock `json:"content,omitempty"`
	ToolCallID    string        `json:"toolCallId,omitempty"`
	Title         string        `json:"title,omitempty"`
	Kind          string        `json:"kind,omitempty"`
	Status        string        `json:"status,omitempty"`
	Entries       []PlanEntry   `json:"entries,omitempty"`
}

// PlanEntry represents a plan entry
type PlanEntry struct {
	Content  string `json:"content"`
	Priority string `json:"priority"`
	Status   string `json:"status"`
}

// SetConfigOptionParams are the parameters for session/set_config_option
type SetConfigOptionParams struct {
	SessionID string `json:"sessionId"`
	ConfigID  string `json:"configId"`
	Value     string `json:"value"`
}

// ConfigOption represents a configuration option for a session
type ConfigOption struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	Description  string              `json:"description,omitempty"`
	Category     string              `json:"category,omitempty"`
	Type         string              `json:"type"`
	CurrentValue string              `json:"currentValue"`
	Options      []ConfigOptionValue `json:"options"`
}

// ConfigOptionValue represents a value choice for a config option
type ConfigOptionValue struct {
	Value       string `json:"value"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// SetConfigOptionResult is the response from session/set_config_option
type SetConfigOptionResult struct {
	ConfigOptions []ConfigOption `json:"configOptions"`
}

// StdioClient implements the ACP protocol over stdio
type StdioClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	scanner *bufio.Scanner
	encoder *json.Encoder

	nextID    int64
	pending   map[int64]chan *JSONRPCResponse
	pendingMu sync.Mutex

	notifications chan *JSONRPCNotification

	initialized bool
	sessionID   string
	agentInfo   *AgentInfo
	agentCaps   *AgentCapabilities

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu sync.Mutex
}

// NewStdioClient creates a new ACP client that communicates via stdio
func NewStdioClient(command string, args []string, workDir string, env map[string]string) (*StdioClient, error) {
	ctx, cancel := context.WithCancel(context.Background())

	cmd := exec.CommandContext(ctx, command, args...)
	if workDir != "" {
		cmd.Dir = workDir
	}

	// Set environment
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	client := &StdioClient{
		cmd:           cmd,
		stdin:         stdin,
		stdout:        stdout,
		stderr:        stderr,
		scanner:       bufio.NewScanner(stdout),
		encoder:       json.NewEncoder(stdin),
		pending:       make(map[int64]chan *JSONRPCResponse),
		notifications: make(chan *JSONRPCNotification, 100),
		ctx:           ctx,
		cancel:        cancel,
	}

	// Start the subprocess
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start agent process: %w", err)
	}

	// Start reading responses
	client.wg.Add(1)
	go client.readLoop()

	// Start reading stderr for logging
	client.wg.Add(1)
	go client.readStderr()

	return client, nil
}

// readLoop reads JSON-RPC messages from stdout
func (c *StdioClient) readLoop() {
	defer c.wg.Done()

	for c.scanner.Scan() {
		line := c.scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Debug: log raw line (uncomment for debugging)
		// fmt.Fprintf(os.Stderr, "ACP RAW: %s\n", string(line))

		// Try to parse as response first (has "id" field)
		var msg struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      *int64          `json:"id"`
			Method  string          `json:"method"`
			Result  json.RawMessage `json:"result"`
			Error   *JSONRPCError   `json:"error"`
			Params  json.RawMessage `json:"params"`
		}

		if err := json.Unmarshal(line, &msg); err != nil {
			// Log parse error but continue
			fmt.Fprintf(os.Stderr, "ACP: failed to parse message: %v\n", err)
			continue
		}

		if msg.ID != nil {
			// This is a response to a request we made
			c.pendingMu.Lock()
			ch, ok := c.pending[*msg.ID]
			if ok {
				delete(c.pending, *msg.ID)
			}
			c.pendingMu.Unlock()

			if ok {
				resp := &JSONRPCResponse{
					JSONRPC: msg.JSONRPC,
					ID:      *msg.ID,
					Result:  msg.Result,
					Error:   msg.Error,
				}
				select {
				case ch <- resp:
				default:
				}
			}
		} else if msg.Method != "" {
			// This is a notification from the agent
			notif := &JSONRPCNotification{
				JSONRPC: msg.JSONRPC,
				Method:  msg.Method,
			}
			if len(msg.Params) > 0 {
				var params interface{}
				json.Unmarshal(msg.Params, &params)
				notif.Params = params
			}

			select {
			case c.notifications <- notif:
			default:
				// Channel full - log this as it indicates a problem
				fmt.Fprintf(os.Stderr, "ACP: notification channel full, dropping: %s\n", msg.Method)
			}
		}
	}

	if err := c.scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "ACP: scanner error: %v\n", err)
	}
}

// readStderr reads and logs stderr output from the agent
func (c *StdioClient) readStderr() {
	defer c.wg.Done()

	scanner := bufio.NewScanner(c.stderr)
	for scanner.Scan() {
		fmt.Fprintf(os.Stderr, "ACP agent stderr: %s\n", scanner.Text())
	}
}

// call sends a JSON-RPC request and waits for a response
func (c *StdioClient) call(ctx context.Context, method string, params interface{}) (*JSONRPCResponse, error) {
	c.mu.Lock()
	id := atomic.AddInt64(&c.nextID, 1)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	// Create response channel
	respCh := make(chan *JSONRPCResponse, 1)
	c.pendingMu.Lock()
	c.pending[id] = respCh
	c.pendingMu.Unlock()

	// Send request
	if err := c.encoder.Encode(req); err != nil {
		c.mu.Unlock()
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	c.mu.Unlock()

	// Wait for response
	select {
	case resp := <-respCh:
		return resp, nil
	case <-ctx.Done():
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, ctx.Err()
	}
}

// Initialize performs the ACP initialization handshake
func (c *StdioClient) Initialize(ctx context.Context) error {
	params := InitializeParams{
		ProtocolVersion: 1,
		ClientCapabilities: ClientCapabilities{
			FS: &FSCapabilities{
				ReadTextFile:  true,
				WriteTextFile: true,
			},
			Terminal: true,
		},
		ClientInfo: ClientInfo{
			Name:    "diane",
			Title:   "Diane Personal Assistant",
			Version: "1.0.0",
		},
	}

	resp, err := c.call(ctx, "initialize", params)
	if err != nil {
		return fmt.Errorf("initialize failed: %w", err)
	}

	if resp.Error != nil {
		return resp.Error
	}

	var result InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("failed to parse initialize result: %w", err)
	}

	c.agentInfo = &result.AgentInfo
	c.agentCaps = &result.AgentCapabilities
	c.initialized = true

	return nil
}

// NewSession creates a new ACP session
func (c *StdioClient) NewSession(ctx context.Context, cwd string) (string, error) {
	if !c.initialized {
		return "", fmt.Errorf("client not initialized")
	}

	params := SessionNewParams{
		CWD:        cwd,
		MCPServers: []MCPServer{}, // Empty array, required by ACP spec
	}

	resp, err := c.call(ctx, "session/new", params)
	if err != nil {
		return "", fmt.Errorf("session/new failed: %w", err)
	}

	if resp.Error != nil {
		return "", resp.Error
	}

	var result SessionNewResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("failed to parse session/new result: %w", err)
	}

	c.sessionID = result.SessionID
	return result.SessionID, nil
}

// SetConfigOption sets a configuration option for a session
func (c *StdioClient) SetConfigOption(ctx context.Context, sessionID, configID, value string) error {
	params := SetConfigOptionParams{
		SessionID: sessionID,
		ConfigID:  configID,
		Value:     value,
	}

	resp, err := c.call(ctx, "session/set_config_option", params)
	if err != nil {
		return fmt.Errorf("session/set_config_option failed: %w", err)
	}

	if resp.Error != nil {
		return resp.Error
	}

	return nil
}

// Prompt sends a prompt to the agent and returns when the turn is complete
// The updateHandler is called for each session/update notification received
func (c *StdioClient) Prompt(ctx context.Context, sessionID, text string, updateHandler func(*SessionUpdateParams)) (*SessionPromptResult, error) {
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	params := SessionPromptParams{
		SessionID: sessionID,
		Prompt: []ContentBlock{
			{
				Type: "text",
				Text: text,
			},
		},
	}

	// Start a goroutine to handle notifications during this prompt
	done := make(chan struct{})
	notifDone := make(chan struct{})
	go func() {
		defer close(notifDone)
		for {
			select {
			case notif := <-c.notifications:
				if notif.Method == "session/update" && updateHandler != nil {
					// Parse the update params
					paramsBytes, _ := json.Marshal(notif.Params)
					var updateParams SessionUpdateParams
					if err := json.Unmarshal(paramsBytes, &updateParams); err == nil {
						updateHandler(&updateParams)
					}
				}
			case <-done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	resp, err := c.call(ctx, "session/prompt", params)
	if err != nil {
		close(done)
		<-notifDone
		return nil, fmt.Errorf("session/prompt failed: %w", err)
	}

	if resp.Error != nil {
		close(done)
		<-notifDone
		return nil, resp.Error
	}

	var result SessionPromptResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		close(done)
		<-notifDone
		return nil, fmt.Errorf("failed to parse session/prompt result: %w", err)
	}

	// Drain any remaining notifications for a short period
	// Notifications may still be arriving after the response
	drainCtx, drainCancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer drainCancel()

drainLoop:
	for {
		select {
		case notif := <-c.notifications:
			if notif.Method == "session/update" && updateHandler != nil {
				paramsBytes, _ := json.Marshal(notif.Params)
				var updateParams SessionUpdateParams
				if err := json.Unmarshal(paramsBytes, &updateParams); err == nil {
					updateHandler(&updateParams)
				}
			}
		case <-drainCtx.Done():
			break drainLoop
		case <-time.After(100 * time.Millisecond):
			// No notifications for 100ms, assume we're done
			break drainLoop
		}
	}

	close(done)
	<-notifDone

	return &result, nil
}

// Cancel sends a session/cancel notification
func (c *StdioClient) Cancel(sessionID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	notif := JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  "session/cancel",
		Params: map[string]string{
			"sessionId": sessionID,
		},
	}

	return c.encoder.Encode(notif)
}

// Close shuts down the client and terminates the agent process
func (c *StdioClient) Close() error {
	c.cancel()

	// Close stdin to signal the agent to exit
	if c.stdin != nil {
		c.stdin.Close()
	}

	// Wait for the process to exit with a timeout
	done := make(chan error, 1)
	go func() {
		done <- c.cmd.Wait()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		// Force kill if it doesn't exit gracefully
		c.cmd.Process.Kill()
	}

	c.wg.Wait()
	return nil
}

// Ping checks if the agent process is still running and responsive
func (c *StdioClient) Ping(ctx context.Context) error {
	// If not initialized, try to initialize
	if !c.initialized {
		return c.Initialize(ctx)
	}

	// Check if process is still running
	if c.cmd.ProcessState != nil && c.cmd.ProcessState.Exited() {
		return fmt.Errorf("agent process has exited")
	}

	return nil
}

// GetAgentInfo returns information about the connected agent
func (c *StdioClient) GetAgentInfo() *AgentInfo {
	return c.agentInfo
}

// GetAgentCapabilities returns the capabilities of the connected agent
func (c *StdioClient) GetAgentCapabilities() *AgentCapabilities {
	return c.agentCaps
}

// IsInitialized returns whether the client has completed initialization
func (c *StdioClient) IsInitialized() bool {
	return c.initialized
}

// GetSessionID returns the current session ID
func (c *StdioClient) GetSessionID() string {
	return c.sessionID
}
