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
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// HTTPClient represents a connection to an MCP server via HTTP Streamable transport
type HTTPClient struct {
	name                string
	baseURL             string
	headers             map[string]string
	httpClient          *http.Client
	sessionID           string
	mu                  sync.Mutex
	notifyChan          chan string
	connected           atomic.Bool
	cachedToolCount     int
	toolCountValid      bool
	cachedPromptCount   int
	promptCountValid    bool
	cachedResourceCount int
	resourceCountValid  bool
	refreshing          bool
	lastError           string
	nextID              int
	// OAuth support
	oauthConfig *OAuthConfig
}

// NewHTTPClient creates a new HTTP Streamable MCP client
func NewHTTPClient(name string, url string, headers map[string]string) (*HTTPClient, error) {
	return NewHTTPClientWithOAuth(name, url, headers, nil)
}

// NewHTTPClientWithOAuth creates a new HTTP Streamable MCP client with OAuth support
func NewHTTPClientWithOAuth(name string, url string, headers map[string]string, oauth *OAuthConfig) (*HTTPClient, error) {
	// Create HTTP client with connection pooling and keepalive
	transport := &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
		// TCP keepalive to detect dead connections
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}

	client := &HTTPClient{
		name:        name,
		baseURL:     url,
		headers:     headers,
		httpClient:  &http.Client{Timeout: 30 * time.Second, Transport: transport},
		notifyChan:  make(chan string, 10),
		nextID:      1,
		oauthConfig: oauth,
	}

	// Initialize the MCP connection
	if err := client.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize: %w", err)
	}

	client.connected.Store(true)
	slog.Info("HTTP MCP client connected", "client", name, "session", client.sessionID)

	return client, nil
}

// getAuthorizationHeader returns the Authorization header value if OAuth is configured
func (c *HTTPClient) getAuthorizationHeader() string {
	if c.oauthConfig == nil {
		return ""
	}

	oauthMgr := GetOAuthManager()
	if oauthMgr == nil {
		return ""
	}

	token := oauthMgr.GetToken(c.name)
	if token == nil {
		return ""
	}

	tokenType := token.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}

	return tokenType + " " + token.AccessToken
}

// RequiresAuth returns true if this client requires OAuth authentication
func (c *HTTPClient) RequiresAuth() bool {
	return c.oauthConfig != nil
}

// HasValidAuth returns true if this client has valid OAuth credentials
func (c *HTTPClient) HasValidAuth() bool {
	if c.oauthConfig == nil {
		return true // No auth required
	}

	oauthMgr := GetOAuthManager()
	if oauthMgr == nil {
		return false
	}

	return oauthMgr.HasValidToken(c.name)
}

// parseSSEResponse extracts JSON from SSE formatted response
// SSE format: "event: message\ndata: {json}\n\n"
func parseSSEResponse(body []byte) ([]byte, error) {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	// Increase buffer size for large SSE responses (GitHub tools/list is ~100KB)
	// Default scanner buffer is 64KB which is too small
	buf := make([]byte, 256*1024)
	scanner.Buffer(buf, 256*1024)

	var dataLines []string

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimSpace(data)
			dataLines = append(dataLines, data)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error: %w", err)
	}

	if len(dataLines) == 0 {
		// Not SSE format, return original body
		return body, nil
	}

	// Join all data lines (in case data spans multiple lines)
	return []byte(strings.Join(dataLines, "")), nil
}

// decodeResponse handles both JSON and SSE formatted responses
func decodeResponse(resp *http.Response) (*MCPResponse, error) {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check if response is SSE format
	contentType := resp.Header.Get("Content-Type")
	isSSE := strings.Contains(contentType, "text/event-stream") || bytes.HasPrefix(bodyBytes, []byte("event:"))

	slog.Info("decodeResponse", "contentType", contentType, "isSSE", isSSE, "bodyLen", len(bodyBytes))

	if isSSE {
		bodyBytes, err = parseSSEResponse(bodyBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse SSE response: %w", err)
		}
		slog.Info("decodeResponse after SSE parse", "bodyLen", len(bodyBytes))
	}

	var mcpResp MCPResponse
	if err := json.Unmarshal(bodyBytes, &mcpResp); err != nil {
		slog.Error("decodeResponse unmarshal failed", "error", err, "bodyPrefix", string(bodyBytes[:min(100, len(bodyBytes))]))
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &mcpResp, nil
}

// initialize sends the initialize request and captures session ID
func (c *HTTPClient) initialize() error {
	params := json.RawMessage(`{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"diane","version":"1.0.0"}}`)

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      0,
		Method:  "initialize",
		Params:  params,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.baseURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	httpReq.Header.Set("MCP-Protocol-Version", "2025-03-26")
	for k, v := range c.headers {
		httpReq.Header.Set(k, v)
	}

	// Add OAuth authorization header if configured
	if authHeader := c.getAuthorizationHeader(); authHeader != "" {
		httpReq.Header.Set("Authorization", authHeader)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Capture session ID from response header
	if sessionID := resp.Header.Get("Mcp-Session-Id"); sessionID != "" {
		c.sessionID = sessionID
		slog.Info("HTTPClient captured session ID", "client", c.name, "sessionID", sessionID)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		// Check if this is an auth error
		if resp.StatusCode == http.StatusUnauthorized && c.oauthConfig != nil {
			return fmt.Errorf("authentication required: run 'diane auth login %s' to authenticate", c.name)
		}
		return fmt.Errorf("initialize failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	mcpResp, err := decodeResponse(resp)
	if err != nil {
		return err
	}

	if mcpResp.Error != nil {
		return fmt.Errorf("initialize error: %s", mcpResp.Error.Message)
	}

	return nil
}

// retryRequest wraps a request with retry logic for transient failures
// Retries network errors and 5xx errors, but not 4xx errors (client errors)
func (c *HTTPClient) retryRequest(fn func() (json.RawMessage, error), method string) (json.RawMessage, error) {
	maxRetries := 3
	baseDelay := 100 * time.Millisecond

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := baseDelay * time.Duration(1<<uint(attempt-1)) // exponential backoff
			slog.Debug("Retrying HTTP request",
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
			slog.Debug("HTTP request failed, will retry",
				"client", c.name,
				"method", method,
				"error", err,
				"attempt", attempt)
		}
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", maxRetries+1, lastErr)
}

// sendRequest sends a JSON-RPC request via HTTP POST
func (c *HTTPClient) sendRequest(method string, params json.RawMessage) (json.RawMessage, error) {
	return c.sendRequestWithTimeout(method, params, 30*time.Second)
}

func (c *HTTPClient) sendRequestWithTimeout(method string, params json.RawMessage, timeout time.Duration) (json.RawMessage, error) {
	// Wrap in retry logic for network resilience
	return c.retryRequest(func() (json.RawMessage, error) {
		c.mu.Lock()
		c.nextID++
		reqID := c.nextID
		sessionID := c.sessionID
		c.mu.Unlock()

		slog.Debug("HTTPClient sendRequest", "client", c.name, "method", method, "sessionID", sessionID)

		req := MCPRequest{
			JSONRPC: "2.0",
			ID:      reqID,
			Method:  method,
			Params:  params,
		}

		body, err := json.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		httpReq, err := http.NewRequest("POST", c.baseURL, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "application/json, text/event-stream")
		httpReq.Header.Set("MCP-Protocol-Version", "2025-03-26")
		if sessionID != "" {
			httpReq.Header.Set("MCP-Session-Id", sessionID)
		}
		for k, v := range c.headers {
			httpReq.Header.Set(k, v)
		}

		// Add OAuth authorization header if configured
		if authHeader := c.getAuthorizationHeader(); authHeader != "" {
			httpReq.Header.Set("Authorization", authHeader)
		}

		// Use the shared HTTP client with connection pooling
		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			c.mu.Lock()
			c.lastError = err.Error()
			c.mu.Unlock()
			return nil, fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
			bodyBytes, _ := io.ReadAll(resp.Body)
			errMsg := fmt.Sprintf("status %d: %s", resp.StatusCode, string(bodyBytes))
			c.mu.Lock()
			c.lastError = errMsg
			c.mu.Unlock()
			return nil, fmt.Errorf("request failed with %s", errMsg)
		}

		mcpResp, err := decodeResponse(resp)
		if err != nil {
			return nil, err
		}

		if mcpResp.Error != nil {
			return nil, fmt.Errorf("%s error: %s", method, mcpResp.Error.Message)
		}

		return mcpResp.Result, nil
	}, method)
}

// GetName returns the client name
func (c *HTTPClient) GetName() string {
	return c.name
}

// ListTools returns the list of available tools
func (c *HTTPClient) ListTools() ([]map[string]interface{}, error) {
	return c.ListToolsWithTimeout(30 * time.Second)
}

// ListToolsWithTimeout returns the list of tools with a custom timeout
func (c *HTTPClient) ListToolsWithTimeout(timeout time.Duration) ([]map[string]interface{}, error) {
	slog.Debug("HTTPClient ListTools starting", "client", c.name, "timeout", timeout)
	result, err := c.sendRequestWithTimeout("tools/list", nil, timeout)
	if err != nil {
		slog.Warn("HTTPClient ListTools failed", "client", c.name, "error", err)
		return nil, err
	}

	slog.Debug("HTTPClient ListTools got result", "client", c.name, "resultLen", len(result))

	var toolsResult struct {
		Tools []map[string]interface{} `json:"tools"`
	}
	if err := json.Unmarshal(result, &toolsResult); err != nil {
		slog.Warn("HTTPClient ListTools unmarshal failed", "client", c.name, "error", err, "result", string(result[:min(200, len(result))]))
		return nil, fmt.Errorf("failed to parse tools: %w", err)
	}

	slog.Info("HTTPClient ListTools success", "client", c.name, "toolCount", len(toolsResult.Tools))

	c.mu.Lock()
	c.cachedToolCount = len(toolsResult.Tools)
	c.toolCountValid = true
	c.mu.Unlock()

	return toolsResult.Tools, nil
}

// CallTool calls a tool on the MCP server
func (c *HTTPClient) CallTool(toolName string, arguments map[string]interface{}) (json.RawMessage, error) {
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
func (c *HTTPClient) ListPrompts() ([]map[string]interface{}, error) {
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
func (c *HTTPClient) GetPrompt(name string, arguments map[string]string) (json.RawMessage, error) {
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
func (c *HTTPClient) ListResources() ([]map[string]interface{}, error) {
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
func (c *HTTPClient) ReadResource(uri string) (json.RawMessage, error) {
	params, err := json.Marshal(map[string]interface{}{
		"uri": uri,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	return c.sendRequest("resources/read", params)
}

// IsConnected returns true if the client is connected
func (c *HTTPClient) IsConnected() bool {
	return c.connected.Load()
}

// GetCachedToolCount returns the cached tool count
func (c *HTTPClient) GetCachedToolCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.toolCountValid {
		return c.cachedToolCount
	}
	return -1
}

// GetCachedPromptCount returns the cached prompt count
func (c *HTTPClient) GetCachedPromptCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.promptCountValid {
		return c.cachedPromptCount
	}
	return -1
}

// GetCachedResourceCount returns the cached resource count
func (c *HTTPClient) GetCachedResourceCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.resourceCountValid {
		return c.cachedResourceCount
	}
	return -1
}

// InvalidateToolCache marks the tool cache as invalid
func (c *HTTPClient) InvalidateToolCache() {
	c.mu.Lock()
	c.toolCountValid = false
	c.mu.Unlock()
}

// TriggerAsyncRefresh starts a background cache refresh
func (c *HTTPClient) TriggerAsyncRefresh(timeout time.Duration) bool {
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
func (c *HTTPClient) NotificationChan() <-chan string {
	return c.notifyChan
}

// GetLastError returns the last error message
func (c *HTTPClient) GetLastError() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastError
}

// SetError sets the last error message
func (c *HTTPClient) SetError(err string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastError = err
}

// GetStderrOutput returns empty string (no stderr for HTTP)
func (c *HTTPClient) GetStderrOutput() string {
	return ""
}

// Close closes the HTTP connection
func (c *HTTPClient) Close() error {
	c.connected.Store(false)
	return nil
}

// GetDisconnectChan returns a channel that never closes because HTTP is stateless
func (c *HTTPClient) GetDisconnectChan() <-chan struct{} {
	// HTTP is stateless - each request reconnects, so disconnect doesn't apply
	return make(chan struct{})
}
