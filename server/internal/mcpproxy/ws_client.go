package mcpproxy

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/diane-assistant/diane/internal/slavetypes"
	"github.com/gorilla/websocket"
)

// ToolProvider interface for retrieving local tools
type ToolProvider interface {
	ListTools() ([]map[string]interface{}, error)
	CallTool(name string, arguments map[string]interface{}) (map[string]interface{}, error)
}

// WSClient is a WebSocket-based MCP client for remote slave connections
type WSClient struct {
	name         string
	hostname     string
	masterAddr   string
	version      string // Version to send in registration
	toolProvider ToolProvider
	conn         *websocket.Conn
	connMu       sync.RWMutex
	connected    bool
	encoder      *json.Encoder
	decoder      *json.Decoder
	mu           sync.Mutex
	notifyChan   chan string
	nextID       int
	pendingMu    sync.Mutex
	pending      map[interface{}]chan MCPResponse

	// Cache
	cachedToolCount     int
	toolCountValid      bool
	cachedPromptCount   int
	promptCountValid    bool
	cachedResourceCount int
	resourceCountValid  bool
	refreshing          bool
	lastError           string

	// TLS config
	certPath string
	keyPath  string
	caPath   string

	// Reverse proxy: master tools flowing to this slave
	proxy          *Proxy                        // The slave's local proxy, for registering master tool clients
	masterClients  map[string]*MasterProxyClient // serverName -> client
	masterClientMu sync.Mutex
}

// NewWSClient creates a new WebSocket MCP client
func NewWSClient(name, hostname, masterAddr, certPath, keyPath, caPath, version string, toolProvider ToolProvider) (*WSClient, error) {
	client := &WSClient{
		name:         name,
		hostname:     hostname,
		masterAddr:   masterAddr,
		version:      version,
		toolProvider: toolProvider,
		notifyChan:   make(chan string, 10),
		pending:      make(map[interface{}]chan MCPResponse),
		certPath:     certPath,
		keyPath:      keyPath,
		caPath:       caPath,
	}

	// Connect to master
	if err := client.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to master: %w", err)
	}

	// Start message reader
	go client.readLoop()

	// Start heartbeat
	go client.heartbeatLoop()

	return client, nil
}

// connect establishes WebSocket connection to master
func (c *WSClient) connect() error {
	// Load client certificate
	cert, err := tls.LoadX509KeyPair(c.certPath, c.keyPath)
	if err != nil {
		return fmt.Errorf("failed to load client cert: %w", err)
	}

	// Load CA certificate
	caCert, err := os.ReadFile(c.caPath)
	if err != nil {
		return fmt.Errorf("failed to read CA cert: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return fmt.Errorf("failed to parse CA cert")
	}

	// Create TLS config
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            caCertPool,
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true, // TODO: Fix master cert to include proper SANs
	}

	// Create WebSocket dialer with TLS
	dialer := websocket.Dialer{
		TLSClientConfig: tlsConfig,
	}

	// Connect to master
	url := fmt.Sprintf("wss://%s/slave/connect", c.masterAddr)
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("failed to dial master: %w", err)
	}

	c.connMu.Lock()
	c.conn = conn
	c.connected = true
	c.connMu.Unlock()

	// Send registration message
	if err := c.register(); err != nil {
		c.Close()
		return fmt.Errorf("failed to register: %w", err)
	}

	slog.Info("Connected to master", "master", c.masterAddr, "hostname", c.hostname)
	return nil
}

// register sends registration message to master
func (c *WSClient) register() error {
	// Get local tools
	tools, err := c.getLocalTools()
	if err != nil {
		return fmt.Errorf("failed to get local tools: %w", err)
	}

	regMsg := slavetypes.RegisterMessage{
		Hostname: c.hostname,
		Version:  c.version,
		Tools:    tools,
	}

	data, err := json.Marshal(regMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal registration: %w", err)
	}

	msg := slavetypes.Message{
		Type:      slavetypes.MessageTypeRegister,
		ID:        c.getNextID(),
		Timestamp: time.Now(),
		Data:      data,
	}

	return c.sendMessage(msg)
}

// getLocalTools queries local MCP servers for tools
func (c *WSClient) getLocalTools() ([]map[string]interface{}, error) {
	if c.toolProvider == nil {
		return []map[string]interface{}{}, nil
	}
	return c.toolProvider.ListTools()
}

// readLoop reads messages from WebSocket connection
func (c *WSClient) readLoop() {
	for {
		c.connMu.RLock()
		conn := c.conn
		connected := c.connected
		c.connMu.RUnlock()

		if !connected || conn == nil {
			// Not connected, exit this readLoop instance
			// reconnectLoop will start a new one after reconnecting
			return
		}

		var msg slavetypes.Message
		if err := conn.ReadJSON(&msg); err != nil {
			// Any read error means the connection is broken
			// Always trigger reconnection regardless of error type
			slog.Error("WebSocket read error, triggering reconnect",
				"error", err,
				"master", c.masterAddr)
			c.handleDisconnect()
			return
		}

		c.handleMessage(msg)
	}
}

// handleMessage processes incoming messages from master
func (c *WSClient) handleMessage(msg slavetypes.Message) {
	switch msg.Type {
	case slavetypes.MessageTypeToolCall:
		c.handleToolCall(msg)
	case slavetypes.MessageTypeResponse:
		c.handleResponse(msg)
	case slavetypes.MessageTypeError:
		c.handleError(msg)
	case slavetypes.MessageTypeRestart:
		c.handleRestart(msg)
	case slavetypes.MessageTypeUpgrade:
		c.handleUpgrade(msg)
	case slavetypes.MessageTypeMasterTools:
		c.handleMasterTools(msg)
	default:
		slog.Warn("Unknown message type", "type", msg.Type)
	}
}

// handleToolCall processes tool call request from master
func (c *WSClient) handleToolCall(msg slavetypes.Message) {
	var callMsg slavetypes.ToolCallMessage
	if err := json.Unmarshal(msg.Data, &callMsg); err != nil {
		slog.Error("Failed to unmarshal tool call", "error", err)
		c.sendErrorResponse(msg.ID, "Invalid tool call message")
		return
	}

	// Execute tool locally
	result, err := c.executeLocalTool(callMsg.Tool, callMsg.Arguments)

	// Send response
	response := slavetypes.ToolCallResponse{
		Success: err == nil,
		Result:  result,
	}

	if err != nil {
		response.Error = err.Error()
	}

	data, _ := json.Marshal(response)
	respMsg := slavetypes.Message{
		Type:      slavetypes.MessageTypeResponse,
		ID:        msg.ID,
		Timestamp: time.Now(),
		Data:      data,
	}

	if err := c.sendMessage(respMsg); err != nil {
		slog.Error("Failed to send tool response", "error", err)
	}
}

// executeLocalTool executes a tool on the local Diane instance
func (c *WSClient) executeLocalTool(tool string, arguments map[string]interface{}) (json.RawMessage, error) {
	if c.toolProvider == nil {
		return nil, fmt.Errorf("tool provider not initialized")
	}

	result, err := c.toolProvider.CallTool(tool, arguments)
	if err != nil {
		return nil, err
	}

	return json.Marshal(result)
}

// handleResponse processes response to a request we sent
func (c *WSClient) handleResponse(msg slavetypes.Message) {
	c.pendingMu.Lock()
	ch, ok := c.pending[msg.ID]
	delete(c.pending, msg.ID)
	c.pendingMu.Unlock()

	if !ok {
		slog.Warn("Received response for unknown request", "id", msg.ID)
		return
	}

	// Parse as MCP response
	var mcpResp MCPResponse
	if err := json.Unmarshal(msg.Data, &mcpResp); err != nil {
		mcpResp = MCPResponse{
			Error: &MCPError{
				Code:    -32700,
				Message: "Failed to parse response",
			},
		}
	}

	ch <- mcpResp
	close(ch)
}

// handleError processes error message from master
func (c *WSClient) handleError(msg slavetypes.Message) {
	var errData struct {
		Error string `json:"error"`
	}

	if err := json.Unmarshal(msg.Data, &errData); err != nil {
		slog.Error("Failed to unmarshal error message", "error", err)
		return
	}

	slog.Error("Received error from master", "error", errData.Error)
	c.SetError(errData.Error)
}

// handleRestart handles restart command from master
func (c *WSClient) handleRestart(msg slavetypes.Message) {
	slog.Info("Received restart command from master")

	// Perform graceful restart using the platform-specific restart script
	// This will restart the Diane process which will reconnect to master
	go func() {
		// Give time for any pending operations to complete
		time.Sleep(1 * time.Second)

		// Close connection gracefully
		c.connMu.Lock()
		if c.conn != nil {
			c.conn.Close()
		}
		c.connMu.Unlock()

		// Execute restart - the process will exit and systemd/launchd will restart it
		// On most systems, sending SIGUSR1 triggers a graceful restart
		// Or we can use os.Exit() and rely on process manager
		slog.Info("Initiating restart...")
		os.Exit(0) // Exit cleanly, process manager will restart
	}()
}

// handleUpgrade handles upgrade command from master
func (c *WSClient) handleUpgrade(msg slavetypes.Message) {
	slog.Info("Received upgrade command from master")

	// Run diane upgrade command
	go func() {
		// Give time to send response
		time.Sleep(500 * time.Millisecond)

		// Close connection gracefully
		c.connMu.Lock()
		if c.conn != nil {
			c.conn.Close()
		}
		c.connMu.Unlock()

		// Get the diane binary path
		dianePath := filepath.Join(os.Getenv("HOME"), ".diane", "bin", "diane")

		slog.Info("Starting upgrade process", "diane_path", dianePath)

		// Execute upgrade command
		// The upgrade command will replace the binary and restart the service
		cmd := exec.Command(dianePath, "upgrade")
		output, err := cmd.CombinedOutput()

		if err != nil {
			slog.Error("Upgrade failed", "error", err, "output", string(output))
		} else {
			slog.Info("Upgrade completed", "output", string(output))
		}

		// Exit to allow process manager to restart with new binary
		os.Exit(0)
	}()
}

// SetProxy sets the slave's local proxy so that master tools can be registered as clients.
// This must be called after creating the WSClient but before master tools arrive.
func (c *WSClient) SetProxy(proxy *Proxy) {
	c.masterClientMu.Lock()
	defer c.masterClientMu.Unlock()
	c.proxy = proxy
	if c.masterClients == nil {
		c.masterClients = make(map[string]*MasterProxyClient)
	}
}

// handleMasterTools processes a master_tools message, registering each master
// MCP server's tools as a MasterProxyClient with the slave's local proxy.
// Also stores context-to-server mappings so the slave can filter by context.
func (c *WSClient) handleMasterTools(msg slavetypes.Message) {
	var masterTools slavetypes.MasterToolsMessage
	if err := json.Unmarshal(msg.Data, &masterTools); err != nil {
		slog.Error("Failed to unmarshal master tools", "error", err)
		return
	}

	c.masterClientMu.Lock()
	proxy := c.proxy
	c.masterClientMu.Unlock()

	if proxy == nil {
		slog.Warn("Received master tools but proxy not set, ignoring")
		return
	}

	// Store context-to-server mappings on the proxy (if provided by master)
	if masterTools.ContextMappings != nil {
		proxy.SetMasterContextMappings(masterTools.ContextMappings)
	}

	totalTools := 0
	for serverName, tools := range masterTools.Servers {
		totalTools += len(tools)

		c.masterClientMu.Lock()
		existing, exists := c.masterClients[serverName]
		c.masterClientMu.Unlock()

		if exists {
			// Update existing client's tools
			existing.UpdateTools(tools)
			slog.Info("Updated master tools", "server", serverName, "tools", len(tools))
		} else {
			// Create new MasterProxyClient and register with proxy
			client := NewMasterProxyClient(serverName, tools, c.sendMasterToolCall)

			if err := proxy.RegisterSlaveClient(serverName, client); err != nil {
				slog.Error("Failed to register master tool client",
					"server", serverName, "error", err)
				continue
			}

			c.masterClientMu.Lock()
			c.masterClients[serverName] = client
			c.masterClientMu.Unlock()

			slog.Info("Registered master tools", "server", serverName, "tools", len(tools))
		}
	}

	// Remove clients for servers no longer present in the update
	c.masterClientMu.Lock()
	for serverName, client := range c.masterClients {
		if _, stillExists := masterTools.Servers[serverName]; !stillExists {
			client.Close()
			proxy.UnregisterSlaveClient(serverName)
			delete(c.masterClients, serverName)
			slog.Info("Removed master tools (server no longer present)", "server", serverName)
		}
	}
	c.masterClientMu.Unlock()

	slog.Info("Master tools synchronized", "servers", len(masterTools.Servers), "total_tools", totalTools)
}

// sendMasterToolCall sends a tool call request to the master via WebSocket
// and waits for the response. This is used as the MasterToolCallFunc.
func (c *WSClient) sendMasterToolCall(serverName, toolName string, arguments map[string]interface{}) (json.RawMessage, error) {
	callMsg := slavetypes.MasterToolCallMessage{
		Server:    serverName,
		Tool:      toolName,
		Arguments: arguments,
	}

	data, err := json.Marshal(callMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal master tool call: %w", err)
	}

	msgID := c.getNextID()
	msg := slavetypes.Message{
		Type:      slavetypes.MessageTypeMasterToolCall,
		ID:        msgID,
		Timestamp: time.Now(),
		Data:      data,
	}

	// Create response channel
	respChan := make(chan MCPResponse, 1)
	c.pendingMu.Lock()
	c.pending[msgID] = respChan
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, msgID)
		c.pendingMu.Unlock()
	}()

	// Send message
	if err := c.sendMessage(msg); err != nil {
		return nil, fmt.Errorf("failed to send master tool call: %w", err)
	}

	// Wait for response with timeout
	select {
	case resp := <-respChan:
		if resp.Error != nil {
			return nil, fmt.Errorf("master tool call failed: %s (code %d)", resp.Error.Message, resp.Error.Code)
		}
		// The response wraps a ToolCallResponse
		var toolResp slavetypes.ToolCallResponse
		if err := json.Unmarshal(resp.Result, &toolResp); err != nil {
			// If it doesn't parse as ToolCallResponse, return raw result
			return resp.Result, nil
		}
		if toolResp.Error != "" {
			return nil, fmt.Errorf("master tool execution error: %s", toolResp.Error)
		}
		return toolResp.Result, nil
	case <-time.After(60 * time.Second):
		return nil, fmt.Errorf("master tool call timed out after 60s")
	}
}

// handleDisconnect handles connection loss
func (c *WSClient) handleDisconnect() {
	c.connMu.Lock()
	c.connected = false
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.connMu.Unlock()

	slog.Warn("Disconnected from master, will attempt reconnect")

	// Attempt reconnection with exponential backoff
	go c.reconnectLoop()
}

// reconnectLoop attempts to reconnect to master forever
// Will keep trying with exponential backoff, capped at 2 minutes
func (c *WSClient) reconnectLoop() {
	backoff := time.Second
	maxBackoff := 2 * time.Minute
	attemptCount := 0

	for {
		attemptCount++
		time.Sleep(backoff)

		slog.Info("Attempting to reconnect to master",
			"master", c.masterAddr,
			"attempt", attemptCount,
			"backoff", backoff)

		if err := c.connect(); err != nil {
			slog.Error("Reconnection failed",
				"error", err,
				"attempt", attemptCount,
				"next_retry_in", backoff*2)

			c.SetError(fmt.Sprintf("reconnect failed (attempt %d): %v", attemptCount, err))

			// Increase backoff for next attempt, capped at maxBackoff
			// We will keep trying forever
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		// Reconnected successfully - reset counters
		backoff = time.Second
		attemptCount = 0
		slog.Info("Reconnected to master successfully", "master", c.masterAddr)
		c.SetError("")

		// Restart heartbeat and message reading
		go c.readLoop()
		go c.heartbeatLoop()
		return
	}
}

// heartbeatLoop sends periodic heartbeats to master
func (c *WSClient) heartbeatLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		c.connMu.RLock()
		connected := c.connected
		c.connMu.RUnlock()

		if !connected {
			continue
		}

		msg := slavetypes.Message{
			Type:      slavetypes.MessageTypeHeartbeat,
			Timestamp: time.Now(),
		}

		if err := c.sendMessage(msg); err != nil {
			slog.Error("Failed to send heartbeat", "error", err)
		}
	}
}

// sendMessage sends a message to the master
func (c *WSClient) sendMessage(msg slavetypes.Message) error {
	c.connMu.RLock()
	conn := c.conn
	connected := c.connected
	c.connMu.RUnlock()

	if !connected || conn == nil {
		return fmt.Errorf("not connected to master")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return conn.WriteJSON(msg)
}

// sendErrorResponse sends an error response to master
func (c *WSClient) sendErrorResponse(id, errMsg string) {
	msg := slavetypes.Message{
		Type:      slavetypes.MessageTypeError,
		ID:        id,
		Timestamp: time.Now(),
		Data:      json.RawMessage(fmt.Sprintf(`{"error":%q}`, errMsg)),
	}

	if err := c.sendMessage(msg); err != nil {
		slog.Error("Failed to send error response", "error", err)
	}
}

// getNextID generates next message ID
func (c *WSClient) getNextID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextID++
	return fmt.Sprintf("%d", c.nextID)
}

// --- Client interface implementation ---

// GetName returns the client name
func (c *WSClient) GetName() string {
	return c.name
}

// ListTools returns the list of available tools
func (c *WSClient) ListTools() ([]map[string]interface{}, error) {
	return c.ListToolsWithTimeout(10 * time.Second)
}

// ListToolsWithTimeout returns the list of tools with a custom timeout
func (c *WSClient) ListToolsWithTimeout(timeout time.Duration) ([]map[string]interface{}, error) {
	// This is called by the master - we don't actually need to query the slave
	// The tools are already registered in the registry
	// Return empty for now, actual tools come from registry
	return []map[string]interface{}{}, nil
}

// CallTool calls a tool on the MCP server
func (c *WSClient) CallTool(toolName string, arguments map[string]interface{}) (json.RawMessage, error) {
	// This method is called when a tool needs to be executed on the slave
	// The slave is registered with the master and the master sends tool calls here
	// For now, return not implemented
	return nil, fmt.Errorf("CallTool should not be called on WSClient directly")
}

// ListPrompts returns the list of available prompts
func (c *WSClient) ListPrompts() ([]map[string]interface{}, error) {
	return []map[string]interface{}{}, nil
}

// GetPrompt retrieves a specific prompt and fills in its template
func (c *WSClient) GetPrompt(name string, arguments map[string]string) (json.RawMessage, error) {
	return nil, fmt.Errorf("prompts not supported on remote slaves")
}

// ListResources returns the list of available resources
func (c *WSClient) ListResources() ([]map[string]interface{}, error) {
	return []map[string]interface{}{}, nil
}

// ReadResource reads the contents of a specific resource
func (c *WSClient) ReadResource(uri string) (json.RawMessage, error) {
	return nil, fmt.Errorf("resources not supported on remote slaves")
}

// IsConnected returns true if the client is connected
func (c *WSClient) IsConnected() bool {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	return c.connected
}

// GetCachedToolCount returns the cached tool count (-1 if not cached)
func (c *WSClient) GetCachedToolCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.toolCountValid {
		return -1
	}
	return c.cachedToolCount
}

// GetCachedPromptCount returns the cached prompt count (-1 if not cached)
func (c *WSClient) GetCachedPromptCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.promptCountValid {
		return -1
	}
	return c.cachedPromptCount
}

// GetCachedResourceCount returns the cached resource count (-1 if not cached)
func (c *WSClient) GetCachedResourceCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.resourceCountValid {
		return -1
	}
	return c.cachedResourceCount
}

// InvalidateToolCache marks the tool cache as invalid
func (c *WSClient) InvalidateToolCache() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.toolCountValid = false
	c.promptCountValid = false
	c.resourceCountValid = false
}

// TriggerAsyncRefresh starts a background cache refresh
func (c *WSClient) TriggerAsyncRefresh(timeout time.Duration) bool {
	c.mu.Lock()
	if c.refreshing {
		c.mu.Unlock()
		return false
	}
	c.refreshing = true
	c.mu.Unlock()

	go func() {
		defer func() {
			c.mu.Lock()
			c.refreshing = false
			c.mu.Unlock()
		}()

		// Refresh cache
		// For remote slaves, we don't need to do anything
		// Tools are managed by the registry
	}()

	return true
}

// NotificationChan returns the channel for receiving notifications
func (c *WSClient) NotificationChan() <-chan string {
	return c.notifyChan
}

// GetLastError returns the last error message
func (c *WSClient) GetLastError() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastError
}

// SetError sets the last error message
func (c *WSClient) SetError(err string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastError = err
}

// GetStderrOutput returns stderr output (for stdio clients)
func (c *WSClient) GetStderrOutput() string {
	return ""
}

// Close closes the client connection
func (c *WSClient) Close() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	c.connected = false

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}

	return nil
}

// GetDisconnectChan returns a channel that never closes because WS reconnects forever
func (c *WSClient) GetDisconnectChan() <-chan struct{} {
	// WebSocket handles reconnection internally via reconnectLoop, so this never fires
	return make(chan struct{})
}

// GetCertPaths returns the paths to the slave's certificates
func GetCertPaths(dataDir, hostname string) (certPath, keyPath, caPath string) {
	certPath = filepath.Join(dataDir, fmt.Sprintf("slave-%s-cert.pem", hostname))
	keyPath = filepath.Join(dataDir, fmt.Sprintf("slave-%s-key.pem", hostname))
	caPath = filepath.Join(dataDir, "slave-ca-cert.pem")
	return
}
