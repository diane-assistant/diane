package mcpproxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SSEClient represents a connection to an MCP server via SSE transport
type SSEClient struct {
	name                string
	baseURL             string
	headers             map[string]string
	httpClient          *http.Client
	sseTransport        *http.Transport
	sessionID           string
	messageEndpoint     string
	mu                  sync.Mutex
	notifyChan          chan string
	stopChan            chan struct{}
	reconnectChan       chan struct{}
	connected           atomic.Bool
	closing             atomic.Bool
	cachedToolCount     int
	toolCountValid      bool
	cachedPromptCount   int
	promptCountValid    bool
	cachedResourceCount int
	resourceCountValid  bool
	refreshing          bool
	lastError           string
	lastActivity        time.Time
	nextID              int
	pendingMu           sync.Mutex
	pending             map[interface{}]chan MCPResponse
}

// NewSSEClient creates a new SSE MCP client
func NewSSEClient(name string, url string, headers map[string]string) (*SSEClient, error) {
	// Transport with TCP keep-alive for long-lived SSE connections
	sseTransport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 15 * time.Second,
		}).DialContext,
		IdleConnTimeout:       0, // no idle timeout for SSE
		ResponseHeaderTimeout: 10 * time.Second,
	}

	client := &SSEClient{
		name:          name,
		baseURL:       strings.TrimSuffix(url, "/"),
		headers:       headers,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		sseTransport:  sseTransport,
		notifyChan:    make(chan string, 10),
		stopChan:      make(chan struct{}),
		reconnectChan: make(chan struct{}, 1),
		lastActivity:  time.Now(),
		nextID:        1,
		pending:       make(map[interface{}]chan MCPResponse),
	}

	// Connect to SSE endpoint to get session ID
	if err := client.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	// Initialize the MCP connection
	if err := client.initialize(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to initialize: %w", err)
	}

	// Start reconnect loop
	go client.reconnectLoop()

	return client, nil
}

// connect establishes SSE connection and gets session info
func (c *SSEClient) connect() error {
	sseURL := c.baseURL

	req, err := http.NewRequest("GET", sseURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create SSE request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	// Use the SSE transport with TCP keep-alive (no overall timeout — stream is long-lived)
	sseClient := &http.Client{Transport: c.sseTransport}
	resp, err := sseClient.Do(req)
	if err != nil {
		return fmt.Errorf("SSE connection failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("SSE connection failed with status %d", resp.StatusCode)
	}

	c.connected.Store(true)
	c.mu.Lock()
	c.lastActivity = time.Now()
	c.mu.Unlock()

	// Start SSE event loop in background
	go c.eventLoop(resp.Body)

	// Start liveness monitor
	go c.livenessMonitor()

	// Wait briefly for the endpoint event
	time.Sleep(100 * time.Millisecond)

	if c.messageEndpoint == "" {
		slog.Warn("No endpoint event received from SSE server", "client", c.name)
	}

	slog.Info("SSE connected", "client", c.name, "endpoint", c.messageEndpoint)
	return nil
}

// eventLoop reads SSE events
func (c *SSEClient) eventLoop(body io.ReadCloser) {
	defer body.Close()
	defer func() {
		c.connected.Store(false)
		// Trigger reconnection if not closing
		if !c.closing.Load() {
			select {
			case c.reconnectChan <- struct{}{}:
			default:
			}
		}
	}()

	scanner := bufio.NewScanner(body)
	var eventType, eventData string

	for scanner.Scan() {
		select {
		case <-c.stopChan:
			return
		default:
		}

		line := scanner.Text()

		// Any data from the server means the connection is alive
		c.mu.Lock()
		c.lastActivity = time.Now()
		c.mu.Unlock()

		if line == "" {
			// Empty line = end of event
			if eventType != "" || eventData != "" {
				c.handleEvent(eventType, eventData)
				eventType = ""
				eventData = ""
			}
			continue
		}

		// SSE comment lines (e.g. ": keepalive") — just track activity, no processing needed
		if strings.HasPrefix(line, ":") {
			continue
		}

		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if eventData != "" {
				eventData += "\n"
			}
			eventData += data
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Warn("SSE connection error", "client", c.name, "error", err)
		c.mu.Lock()
		c.lastError = err.Error()
		c.mu.Unlock()
	} else {
		slog.Warn("SSE connection closed", "client", c.name)
		c.mu.Lock()
		c.lastError = "connection closed by server"
		c.mu.Unlock()
	}
}

// livenessMonitor checks that we've received data recently and forces reconnect if stale
func (c *SSEClient) livenessMonitor() {
	// Most SSE servers send keepalives every 15-30s. If we haven't seen anything
	// in 90s the connection is likely dead (TCP half-open).
	const staleDuration = 90 * time.Second
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			if !c.connected.Load() {
				return // eventLoop already triggered reconnect
			}
			c.mu.Lock()
			sinceActivity := time.Since(c.lastActivity)
			c.mu.Unlock()

			if sinceActivity > staleDuration {
				slog.Warn("SSE connection stale, forcing reconnect",
					"client", c.name,
					"last_activity", sinceActivity.Round(time.Second))
				c.connected.Store(false)
				c.mu.Lock()
				c.lastError = fmt.Sprintf("no data received for %s", sinceActivity.Round(time.Second))
				c.mu.Unlock()
				// Trigger reconnect
				select {
				case c.reconnectChan <- struct{}{}:
				default:
				}
				return
			}
		}
	}
}

// reconnectLoop handles automatic reconnection with exponential backoff
// This will reconnect forever, with a reasonable maximum backoff of 60 seconds
func (c *SSEClient) reconnectLoop() {
	backoff := time.Second
	maxBackoff := 60 * time.Second
	attemptCount := 0

	for {
		select {
		case <-c.stopChan:
			return
		case <-c.reconnectChan:
			attemptCount++

			// Wait before reconnecting
			select {
			case <-c.stopChan:
				return
			case <-time.After(backoff):
			}

			slog.Info("SSE reconnecting",
				"client", c.name,
				"backoff", backoff,
				"attempt", attemptCount)

			if err := c.reconnect(); err != nil {
				slog.Warn("SSE reconnect failed",
					"client", c.name,
					"error", err,
					"attempt", attemptCount)
				c.mu.Lock()
				c.lastError = fmt.Sprintf("reconnect failed (attempt %d): %v", attemptCount, err)
				c.mu.Unlock()

				// Increase backoff for next attempt, capped at maxBackoff
				// We will keep trying forever, just with a longer delay
				backoff = backoff * 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}

				// Trigger another reconnect attempt immediately
				select {
				case c.reconnectChan <- struct{}{}:
				default:
				}
			} else {
				// Reset backoff and attempt count on successful reconnection
				backoff = time.Second
				attemptCount = 0
				slog.Info("SSE reconnected successfully", "client", c.name)
			}
		}
	}
}

// reconnect re-establishes the SSE connection and MCP session
func (c *SSEClient) reconnect() error {
	sseURL := c.baseURL

	req, err := http.NewRequest("GET", sseURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create SSE request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	sseClient := &http.Client{Transport: c.sseTransport}
	resp, err := sseClient.Do(req)
	if err != nil {
		return fmt.Errorf("SSE connection failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("SSE connection failed with status %d", resp.StatusCode)
	}

	c.connected.Store(true)
	c.mu.Lock()
	c.lastError = ""
	c.lastActivity = time.Now()
	c.mu.Unlock()

	// Start new event loop
	go c.eventLoop(resp.Body)

	// Start new liveness monitor
	go c.livenessMonitor()

	// Wait briefly for the endpoint event
	time.Sleep(100 * time.Millisecond)

	// Re-initialize the MCP session — the server requires this after a new SSE connection
	if err := c.initialize(); err != nil {
		slog.Warn("SSE re-initialize failed after reconnect", "client", c.name, "error", err)
		return fmt.Errorf("re-initialize failed: %w", err)
	}

	return nil
}

// handleEvent processes an SSE event
func (c *SSEClient) handleEvent(eventType, data string) {
	slog.Debug("SSE event received", "client", c.name, "type", eventType)

	switch eventType {
	case "endpoint":
		// Server sends the message endpoint URL (can be relative or absolute)
		c.mu.Lock()
		if strings.HasPrefix(data, "http://") || strings.HasPrefix(data, "https://") {
			// Absolute URL - use as-is
			c.messageEndpoint = data
		} else {
			// Relative URL - resolve against the base URL
			baseURL, err := url.Parse(c.baseURL)
			if err != nil {
				slog.Warn("Failed to parse base URL", "client", c.name, "error", err)
				c.messageEndpoint = data
			} else {
				refURL, err := url.Parse(data)
				if err != nil {
					slog.Warn("Failed to parse endpoint URL", "client", c.name, "error", err)
					c.messageEndpoint = data
				} else {
					c.messageEndpoint = baseURL.ResolveReference(refURL).String()
				}
			}
		}
		slog.Debug("Message endpoint set", "client", c.name, "endpoint", c.messageEndpoint)
		c.mu.Unlock()

	case "message":
		// Parse JSON-RPC message
		var msg MCPMessage
		if err := json.Unmarshal([]byte(data), &msg); err != nil {
			slog.Warn("Failed to parse SSE message", "client", c.name, "error", err)
			return
		}

		if msg.ID != nil {
			// Response to a request
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
			}
			c.pendingMu.Unlock()
		} else if msg.Method != "" {
			// Notification
			select {
			case c.notifyChan <- msg.Method:
			default:
			}
		}

	default:
		slog.Debug("Unknown SSE event type", "client", c.name, "type", eventType)
	}
}

// retryRequest wraps a request with retry logic for transient failures
// Retries network errors and 5xx errors, but not 4xx errors (client errors)
func (c *SSEClient) retryRequest(fn func() (json.RawMessage, error), method string) (json.RawMessage, error) {
	maxRetries := 3
	baseDelay := 100 * time.Millisecond

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := baseDelay * time.Duration(1<<uint(attempt-1)) // exponential backoff
			slog.Debug("Retrying request",
				"client", c.name,
				"method", method,
				"attempt", attempt,
				"delay", delay)
			time.Sleep(delay)
		}

		result, err := fn()
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Check if error is retryable
		errStr := err.Error()
		isNetworkError := strings.Contains(errStr, "connection refused") ||
			strings.Contains(errStr, "connection reset") ||
			strings.Contains(errStr, "timeout") ||
			strings.Contains(errStr, "EOF") ||
			strings.Contains(errStr, "no such host")

		isServerError := strings.Contains(errStr, "status 5")

		// Don't retry on non-retryable errors (4xx, parse errors, etc.)
		if !isNetworkError && !isServerError {
			return nil, err
		}

		if attempt < maxRetries {
			slog.Debug("Request failed, will retry",
				"client", c.name,
				"method", method,
				"error", err,
				"attempt", attempt)
		}
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", maxRetries+1, lastErr)
}

// sendRequest sends a JSON-RPC request via HTTP POST
func (c *SSEClient) sendRequest(method string, params json.RawMessage) (json.RawMessage, error) {
	return c.sendRequestWithTimeout(method, params, 30*time.Second)
}

func (c *SSEClient) sendRequestWithTimeout(method string, params json.RawMessage, timeout time.Duration) (json.RawMessage, error) {
	// Wrap in retry logic for network resilience
	return c.retryRequest(func() (json.RawMessage, error) {
		c.mu.Lock()
		c.nextID++
		reqID := c.nextID
		endpoint := c.messageEndpoint
		c.mu.Unlock()

		if endpoint == "" {
			return nil, fmt.Errorf("no message endpoint available")
		}

		// Create response channel
		respCh := make(chan MCPResponse, 1)
		c.pendingMu.Lock()
		c.pending[reqID] = respCh
		c.pendingMu.Unlock()

		req := MCPRequest{
			JSONRPC: "2.0",
			ID:      reqID,
			Method:  method,
			Params:  params,
		}

		body, err := json.Marshal(req)
		if err != nil {
			c.pendingMu.Lock()
			delete(c.pending, reqID)
			c.pendingMu.Unlock()
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		httpReq, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
		if err != nil {
			c.pendingMu.Lock()
			delete(c.pending, reqID)
			c.pendingMu.Unlock()
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")
		for k, v := range c.headers {
			httpReq.Header.Set(k, v)
		}

		client := &http.Client{Timeout: timeout}
		resp, err := client.Do(httpReq)
		if err != nil {
			c.pendingMu.Lock()
			delete(c.pending, reqID)
			c.pendingMu.Unlock()
			return nil, fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
			c.pendingMu.Lock()
			delete(c.pending, reqID)
			c.pendingMu.Unlock()
			bodyBytes, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
		}

		// Check if response is in body (synchronous) or via SSE (async)
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.pendingMu.Lock()
			delete(c.pending, reqID)
			c.pendingMu.Unlock()
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		if len(bodyBytes) > 0 {
			// Synchronous response in body
			var mcpResp MCPResponse
			if err := json.Unmarshal(bodyBytes, &mcpResp); err == nil && mcpResp.ID != nil {
				c.pendingMu.Lock()
				delete(c.pending, reqID)
				c.pendingMu.Unlock()
				if mcpResp.Error != nil {
					return nil, fmt.Errorf("%s error: %s", method, mcpResp.Error.Message)
				}
				return mcpResp.Result, nil
			}
		}

		// Wait for async response via SSE
		select {
		case result, ok := <-respCh:
			if !ok {
				return nil, fmt.Errorf("connection closed")
			}
			if result.Error != nil {
				return nil, fmt.Errorf("%s error: %s", method, result.Error.Message)
			}
			return result.Result, nil
		case <-time.After(timeout):
			c.pendingMu.Lock()
			delete(c.pending, reqID)
			c.pendingMu.Unlock()
			return nil, fmt.Errorf("%s timed out", method)
		}
	}, method)
}

// initialize sends the initialize request
func (c *SSEClient) initialize() error {
	params := json.RawMessage(`{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"diane","version":"1.0.0"}}`)
	_, err := c.sendRequestWithTimeout("initialize", params, 10*time.Second)
	return err
}

// GetName returns the client name
func (c *SSEClient) GetName() string {
	return c.name
}

// ListTools returns the list of available tools
func (c *SSEClient) ListTools() ([]map[string]interface{}, error) {
	return c.ListToolsWithTimeout(5 * time.Second)
}

// ListToolsWithTimeout returns the list of tools with a custom timeout
func (c *SSEClient) ListToolsWithTimeout(timeout time.Duration) ([]map[string]interface{}, error) {
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

	c.mu.Lock()
	c.cachedToolCount = len(toolsResult.Tools)
	c.toolCountValid = true
	c.mu.Unlock()

	return toolsResult.Tools, nil
}

// CallTool calls a tool on the MCP server
func (c *SSEClient) CallTool(toolName string, arguments map[string]interface{}) (json.RawMessage, error) {
	params, err := json.Marshal(map[string]interface{}{
		"name":      toolName,
		"arguments": arguments,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	return c.sendRequest("tools/call", params)
}

// ListPrompts requests the list of prompts from the MCP server
func (c *SSEClient) ListPrompts() ([]map[string]interface{}, error) {
	result, err := c.sendRequest("prompts/list", nil)
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
func (c *SSEClient) GetPrompt(name string, arguments map[string]string) (json.RawMessage, error) {
	params, err := json.Marshal(map[string]interface{}{
		"name":      name,
		"arguments": arguments,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	return c.sendRequest("prompts/get", params)
}

// ListResources requests the list of resources from the MCP server
func (c *SSEClient) ListResources() ([]map[string]interface{}, error) {
	result, err := c.sendRequest("resources/list", nil)
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
func (c *SSEClient) ReadResource(uri string) (json.RawMessage, error) {
	params, err := json.Marshal(map[string]interface{}{
		"uri": uri,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	return c.sendRequest("resources/read", params)
}

// IsConnected returns true if the client is connected
func (c *SSEClient) IsConnected() bool {
	return c.connected.Load()
}

// GetCachedToolCount returns the cached tool count
func (c *SSEClient) GetCachedToolCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.toolCountValid {
		return c.cachedToolCount
	}
	return -1
}

// GetCachedPromptCount returns the cached prompt count
func (c *SSEClient) GetCachedPromptCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.promptCountValid {
		return c.cachedPromptCount
	}
	return -1
}

// GetCachedResourceCount returns the cached resource count
func (c *SSEClient) GetCachedResourceCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.resourceCountValid {
		return c.cachedResourceCount
	}
	return -1
}

// InvalidateToolCache marks the tool cache as invalid
func (c *SSEClient) InvalidateToolCache() {
	c.mu.Lock()
	c.toolCountValid = false
	c.mu.Unlock()
}

// TriggerAsyncRefresh starts a background cache refresh
func (c *SSEClient) TriggerAsyncRefresh(timeout time.Duration) bool {
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

// NotificationChan returns the channel for receiving notifications
func (c *SSEClient) NotificationChan() <-chan string {
	return c.notifyChan
}

// GetLastError returns the last error message
func (c *SSEClient) GetLastError() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastError
}

// SetError sets the last error message
func (c *SSEClient) SetError(err string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastError = err
}

// GetStderrOutput returns empty string (no stderr for SSE)
func (c *SSEClient) GetStderrOutput() string {
	return ""
}

// Close closes the SSE connection
func (c *SSEClient) Close() error {
	c.closing.Store(true)
	close(c.stopChan)
	c.connected.Store(false)
	return nil
}

// GetDisconnectChan returns a channel that never closes because SSE reconnects forever
func (c *SSEClient) GetDisconnectChan() <-chan struct{} {
	// SSE handles reconnection internally via reconnectLoop, so this never fires
	return make(chan struct{})
}
