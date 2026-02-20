package slave

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/diane-assistant/diane/internal/mcpproxy"
	"github.com/diane-assistant/diane/internal/slavetypes"
	"github.com/diane-assistant/diane/internal/store"
	"github.com/gorilla/websocket"
)

// Server manages WebSocket connections from slave Diane instances
type Server struct {
	ca              *CertificateAuthority
	registry        *Registry
	db              store.SlaveStore
	pairing         *PairingService
	upgrader        websocket.Upgrader
	server          *http.Server
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	connMu          sync.RWMutex
	connections     map[string]*slaveConnection // hostname -> connection
	pendingCalls    map[string]chan slavetypes.Message
	responseMu      sync.RWMutex
	heartbeatTicker *time.Ticker
	proxy           *mcpproxy.Proxy    // Master's MCP proxy, for collecting tools to send to slaves
	contextStore    store.ContextStore // Master's context store, for building context mappings
}

// slaveConnection represents an active WebSocket connection to a slave
type slaveConnection struct {
	conn          *websocket.Conn
	hostname      string
	writeMu       sync.Mutex
	lastHeartbeat time.Time
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewServer creates a new slave WebSocket server
func NewServer(ca *CertificateAuthority, registry *Registry, database store.SlaveStore, pairing *PairingService) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		ca:       ca,
		registry: registry,
		db:       database,
		pairing:  pairing,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Client cert auth handles authorization
				return true
			},
		},
		ctx:          ctx,
		cancel:       cancel,
		connections:  make(map[string]*slaveConnection),
		pendingCalls: make(map[string]chan slavetypes.Message),
	}
}

// Start begins listening for slave connections
// DEPRECATED: This starts its own server which conflicts with the MCP HTTP server
// Use RegisterHandlers and GetTLSConfig instead
func Start(addr string, ca *CertificateAuthority, registry *Registry, database store.SlaveStore, pairing *PairingService) (*Server, error) {
	srv := NewServer(ca, registry, database, pairing)

	// Start heartbeat monitor
	srv.startHeartbeatMonitor()

	// Note: No longer starting a separate HTTP server
	// Handlers should be registered with the main MCP HTTP server
	slog.Info("Slave server initialized (handlers not bound)", "addr", addr)

	return srv, nil
}

// RegisterHandlers registers the slave WebSocket and pairing handlers with the given mux
func (s *Server) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/slave/connect", s.handleConnect)
	mux.HandleFunc("/api/slaves/pair", s.handlePair)
	mux.HandleFunc("/api/slaves/pair/", s.handlePairStatus)
}

// GetTLSConfig returns the TLS configuration for slave connections
func (s *Server) GetTLSConfig() (*tls.Config, error) {
	// Create TLS config with optional client certificate authentication
	// This allows pairing requests (without cert) to proceed
	tlsConfig := &tls.Config{
		ClientAuth: tls.VerifyClientCertIfGiven,
		ClientCAs:  x509.NewCertPool(),
		MinVersion: tls.VersionTLS12,
	}

	// Add CA certificate to client CA pool
	caCert, err := s.ca.GetCertificate()
	if err != nil {
		return nil, fmt.Errorf("failed to get CA certificate: %w", err)
	}
	tlsConfig.ClientCAs.AddCert(caCert)

	return tlsConfig, nil
}

// GetCertPaths returns the paths to the CA certificate and key files
func (s *Server) GetCertPaths() (certPath, keyPath string, err error) {
	return s.ca.GetPaths()
}

// handleConnect handles WebSocket upgrade and slave connection
func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	// Check if database is available
	if s.db == nil {
		slog.Error("Slave connection rejected: database not available")
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Verify client certificate
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		http.Error(w, "Client certificate required", http.StatusUnauthorized)
		return
	}

	clientCert := r.TLS.PeerCertificates[0]
	hostname := clientCert.Subject.CommonName

	// Get slave ID from certificate
	slave, err := s.db.GetSlaveServerByHostID(context.Background(), hostname)
	if err != nil {
		slog.Error("Failed to get slave server", "hostname", hostname, "error", err)
		http.Error(w, "Unknown slave", http.StatusUnauthorized)
		return
	}
	if slave == nil {
		slog.Error("Unknown slave hostname", "hostname", hostname)
		http.Error(w, "Unknown slave", http.StatusUnauthorized)
		return
	}

	// Check if credentials are revoked
	revoked, err := s.db.IsCredentialRevoked(context.Background(), slave.CertSerial)
	if err != nil {
		slog.Error("Failed to check revocation", "hostname", hostname, "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if revoked {
		slog.Warn("Rejected connection from revoked slave", "hostname", hostname)
		http.Error(w, "Credentials revoked", http.StatusForbidden)
		return
	}

	// Upgrade to WebSocket
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WebSocket upgrade failed", "hostname", hostname, "error", err)
		return
	}

	slog.Info("Slave connected", "hostname", hostname, "cert_serial", slave.CertSerial)

	// Create connection context
	ctx, cancel := context.WithCancel(s.ctx)
	slaveConn := &slaveConnection{
		conn:          conn,
		hostname:      hostname,
		lastHeartbeat: time.Now(),
		ctx:           ctx,
		cancel:        cancel,
	}

	// Store connection
	s.connMu.Lock()
	s.connections[hostname] = slaveConn
	s.connMu.Unlock()

	// Update last seen
	if err := s.db.UpdateSlaveLastSeen(context.Background(), hostname); err != nil {
		slog.Error("Failed to update last seen", "hostname", hostname, "error", err)
	}

	// Handle connection
	s.handleConnection(slaveConn)

	// Cleanup on disconnect
	s.connMu.Lock()
	delete(s.connections, hostname)
	s.connMu.Unlock()

	s.registry.Disconnect(hostname)
	slog.Info("Slave disconnected", "hostname", hostname)
}

// handlePair handles pairing requests
func (s *Server) handlePair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Hostname string `json:"hostname"`
		CSR      string `json:"csr"`
		Platform string `json:"platform"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Hostname == "" || req.CSR == "" {
		http.Error(w, "hostname and csr are required", http.StatusBadRequest)
		return
	}

	code, err := s.pairing.CreatePairingRequest(req.Hostname, []byte(req.CSR), req.Platform)
	if err != nil {
		slog.Error("Failed to create pairing request", "error", err)
		http.Error(w, "Failed to create pairing request", http.StatusInternalServerError)
		return
	}

	slog.Info("Pairing initiated via public port", "hostname", req.Hostname, "code", code)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"pairing_code": code,
	})
}

// handlePairStatus handles checking pairing status
func (s *Server) handlePairStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/slaves/pair/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Pairing code required", http.StatusBadRequest)
		return
	}
	code := parts[0]

	status, cert, err := s.pairing.GetPairingStatus(code)
	if err != nil {
		slog.Error("Failed to get pairing status", "error", err)
		http.Error(w, "Failed to get pairing status", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status": status,
	}

	if status == "approved" && cert != "" {
		response["certificate"] = cert
		caCertPEM, err := s.pairing.GetCACertPEM()
		if err == nil {
			response["ca_cert"] = string(caCertPEM)
		}
	}

	json.NewEncoder(w).Encode(response)
}

// handleConnection processes messages from a slave connection
func (s *Server) handleConnection(conn *slaveConnection) {
	defer conn.conn.Close()
	defer conn.cancel()

	for {
		select {
		case <-conn.ctx.Done():
			return
		default:
		}

		var msg slavetypes.Message
		if err := conn.conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Error("WebSocket read error", "hostname", conn.hostname, "error", err)
			}
			return
		}

		msg.Timestamp = time.Now()

		switch msg.Type {
		case slavetypes.MessageTypeRegister:
			s.handleRegister(conn, msg)
		case slavetypes.MessageTypeHeartbeat:
			s.handleHeartbeat(conn, msg)
		case slavetypes.MessageTypeToolUpdate:
			s.handleToolUpdate(conn, msg)
		case slavetypes.MessageTypeMasterToolCall:
			s.handleMasterToolCall(conn, msg)
		case slavetypes.MessageTypeResponse, slavetypes.MessageTypeError:
			// Route response to pending call if any
			s.responseMu.RLock()
			ch, ok := s.pendingCalls[msg.ID]
			s.responseMu.RUnlock()

			if ok {
				select {
				case ch <- msg:
					// Response delivered
				default:
					slog.Warn("Response channel blocked or closed", "id", msg.ID)
				}
			} else {
				// No pending call, might be stale or handled by registry
				slog.Debug("Received response for unknown call ID", "id", msg.ID)
			}
		default:
			slog.Warn("Unknown message type", "type", msg.Type, "hostname", conn.hostname)
		}
	}
}

// handleRegister processes registration from slave
func (s *Server) handleRegister(conn *slaveConnection, msg slavetypes.Message) {
	var reg slavetypes.RegisterMessage
	if err := json.Unmarshal(msg.Data, &reg); err != nil {
		slog.Error("Failed to unmarshal register message", "hostname", conn.hostname, "error", err)
		s.sendError(conn, msg.ID, "Invalid registration data")
		return
	}

	slog.Info("Slave registered",
		"cert_hostname", conn.hostname,
		"reported_hostname", reg.Hostname,
		"version", reg.Version,
		"tools", len(reg.Tools))

	// Use certificate CN as authoritative hostname (not the reported hostname)
	// This ensures consistency even if the system hostname changes
	s.registry.Connect(conn.hostname, reg.Tools, conn.conn)

	// Update slave version in database
	if reg.Version != "" {
		if err := s.db.UpdateSlaveVersion(context.Background(), conn.hostname, reg.Version); err != nil {
			slog.Error("Failed to update slave version", "hostname", conn.hostname, "error", err)
		}
	}

	// Send acknowledgment
	s.sendMessage(conn, slavetypes.Message{
		Type:      slavetypes.MessageTypeResponse,
		ID:        msg.ID,
		Timestamp: time.Now(),
		Data:      json.RawMessage(`{"status":"registered"}`),
	})

	// After registration, send master tools to the slave
	go s.sendMasterToolsToSlave(conn)
}

// handleHeartbeat processes heartbeat from slave
func (s *Server) handleHeartbeat(conn *slaveConnection, msg slavetypes.Message) {
	conn.lastHeartbeat = time.Now()
	s.registry.Heartbeat(conn.hostname)
}

// handleToolUpdate processes tool list updates from slave
func (s *Server) handleToolUpdate(conn *slaveConnection, msg slavetypes.Message) {
	var tools []map[string]interface{}
	if err := json.Unmarshal(msg.Data, &tools); err != nil {
		slog.Error("Failed to unmarshal tool update", "hostname", conn.hostname, "error", err)
		return
	}

	slog.Info("Slave tools updated", "hostname", conn.hostname, "tools", len(tools))
	s.registry.UpdateTools(conn.hostname, tools)
}

// SendToolCall sends a tool call request to a slave and waits for the response
func (s *Server) SendToolCall(hostname, callID, tool string, arguments map[string]interface{}) (json.RawMessage, error) {
	s.connMu.RLock()
	conn, ok := s.connections[hostname]
	s.connMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("slave not connected: %s", hostname)
	}

	callMsg := slavetypes.ToolCallMessage{
		Tool:      tool,
		Arguments: arguments,
	}

	data, err := json.Marshal(callMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tool call: %w", err)
	}

	msg := slavetypes.Message{
		Type:      slavetypes.MessageTypeToolCall,
		ID:        callID,
		Timestamp: time.Now(),
		Data:      data,
	}

	// Create response channel
	respChan := make(chan slavetypes.Message, 1)

	// Register pending call
	s.responseMu.Lock()
	s.pendingCalls[callID] = respChan
	s.responseMu.Unlock()

	// Ensure cleanup
	defer func() {
		s.responseMu.Lock()
		delete(s.pendingCalls, callID)
		s.responseMu.Unlock()
	}()

	// Send message
	if err := s.sendMessage(conn, msg); err != nil {
		return nil, err
	}

	// Wait for response or timeout
	select {
	case resp := <-respChan:
		if resp.Type == slavetypes.MessageTypeError {
			// Extract error message
			var errorResp struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(resp.Data, &errorResp); err != nil {
				return nil, fmt.Errorf("tool call failed: %s", string(resp.Data))
			}
			return nil, fmt.Errorf("tool call failed: %s", errorResp.Error)
		}

		// Parse ToolCallResponse to get the result
		var toolResp slavetypes.ToolCallResponse
		if err := json.Unmarshal(resp.Data, &toolResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool response: %w", err)
		}

		if toolResp.Error != "" {
			return nil, fmt.Errorf("tool execution error: %s", toolResp.Error)
		}

		return toolResp.Result, nil

	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("tool call timed out")
	}
}

// SendRestartCommand sends a restart command to a slave
func (s *Server) SendRestartCommand(hostname string) error {
	s.connMu.RLock()
	conn, ok := s.connections[hostname]
	s.connMu.RUnlock()

	if !ok {
		return fmt.Errorf("slave not connected: %s", hostname)
	}

	msg := slavetypes.Message{
		Type:      slavetypes.MessageTypeRestart,
		ID:        fmt.Sprintf("restart-%s-%d", hostname, time.Now().UnixNano()),
		Timestamp: time.Now(),
	}

	// Send restart message (fire and forget - slave will restart and reconnect)
	if err := s.sendMessage(conn, msg); err != nil {
		return fmt.Errorf("failed to send restart command: %w", err)
	}

	slog.Info("Restart command sent to slave", "hostname", hostname)
	return nil
}

// SendUpgradeCommand sends an upgrade command to a slave
func (s *Server) SendUpgradeCommand(hostname string) error {
	s.connMu.RLock()
	conn, ok := s.connections[hostname]
	s.connMu.RUnlock()

	if !ok {
		return fmt.Errorf("slave not connected: %s", hostname)
	}

	msg := slavetypes.Message{
		Type:      slavetypes.MessageTypeUpgrade,
		ID:        fmt.Sprintf("upgrade-%s-%d", hostname, time.Now().UnixNano()),
		Timestamp: time.Now(),
	}

	// Send upgrade message (fire and forget - slave will upgrade and restart)
	if err := s.sendMessage(conn, msg); err != nil {
		return fmt.Errorf("failed to send upgrade command: %w", err)
	}

	slog.Info("Upgrade command sent to slave", "hostname", hostname)
	return nil
}

// sendMessage sends a message to a slave connection
func (s *Server) sendMessage(conn *slaveConnection, msg slavetypes.Message) error {
	conn.writeMu.Lock()
	defer conn.writeMu.Unlock()

	if err := conn.conn.WriteJSON(msg); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

// sendError sends an error message to a slave
func (s *Server) sendError(conn *slaveConnection, id, errMsg string) {
	msg := slavetypes.Message{
		Type:      slavetypes.MessageTypeError,
		ID:        id,
		Timestamp: time.Now(),
		Data:      json.RawMessage(fmt.Sprintf(`{"error":%q}`, errMsg)),
	}

	if err := s.sendMessage(conn, msg); err != nil {
		slog.Error("Failed to send error message", "hostname", conn.hostname, "error", err)
	}
}

// SetProxy sets the master's MCP proxy so the server can collect tools to send to slaves.
func (s *Server) SetProxy(proxy *mcpproxy.Proxy) {
	s.proxy = proxy
}

// SetContextStore sets the master's context store so the server can include
// context-to-server mappings when broadcasting tools to slaves.
func (s *Server) SetContextStore(cs store.ContextStore) {
	s.contextStore = cs
}

// sendMasterToolsToSlave collects all non-slave tools from the master's proxy
// and sends them to the specified slave connection, along with context-to-server
// mappings so the slave can filter tools by context.
func (s *Server) sendMasterToolsToSlave(conn *slaveConnection) {
	if s.proxy == nil {
		slog.Debug("No proxy set, skipping master tools broadcast", "hostname", conn.hostname)
		return
	}

	// Get all tools from the master's proxy, grouped by server name.
	// We call ListAllTools which returns tools prefixed with server name.
	// We need to group them by server.
	allTools, err := s.proxy.ListAllTools()
	if err != nil {
		slog.Error("Failed to list master tools for slave", "hostname", conn.hostname, "error", err)
		return
	}

	// Group tools by server name, excluding tools from this slave itself
	serverTools := make(map[string][]map[string]interface{})
	for _, tool := range allTools {
		serverName, _ := tool["_server"].(string)
		if serverName == "" {
			continue
		}
		// Don't send slave's own tools back to it
		if serverName == conn.hostname {
			continue
		}
		// Create a clean copy of the tool (without the server prefix and _server metadata)
		cleanTool := make(map[string]interface{})
		for k, v := range tool {
			cleanTool[k] = v
		}
		// Restore original tool name (strip server prefix)
		if prefixedName, ok := cleanTool["name"].(string); ok {
			prefix := serverName + "_"
			if strings.HasPrefix(prefixedName, prefix) {
				cleanTool["name"] = strings.TrimPrefix(prefixedName, prefix)
			}
		}
		delete(cleanTool, "_server")
		serverTools[serverName] = append(serverTools[serverName], cleanTool)
	}

	if len(serverTools) == 0 {
		slog.Debug("No master tools to send to slave", "hostname", conn.hostname)
		return
	}

	// Build context-to-server mappings from the master's context store.
	// For each context, we record which servers (that we're sending tools for)
	// are enabled, so the slave can filter master-proxied tools by context.
	var contextMappings map[string][]string
	if s.contextStore != nil {
		contextMappings = s.buildContextMappings(serverTools)
	}

	toolsMsg := slavetypes.MasterToolsMessage{
		Servers:         serverTools,
		ContextMappings: contextMappings,
	}

	data, err := json.Marshal(toolsMsg)
	if err != nil {
		slog.Error("Failed to marshal master tools", "hostname", conn.hostname, "error", err)
		return
	}

	msg := slavetypes.Message{
		Type:      slavetypes.MessageTypeMasterTools,
		ID:        fmt.Sprintf("master-tools-%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		Data:      data,
	}

	if err := s.sendMessage(conn, msg); err != nil {
		slog.Error("Failed to send master tools to slave", "hostname", conn.hostname, "error", err)
		return
	}

	totalTools := 0
	for _, tools := range serverTools {
		totalTools += len(tools)
	}
	slog.Info("Sent master tools to slave",
		"hostname", conn.hostname,
		"servers", len(serverTools),
		"total_tools", totalTools,
		"context_mappings", len(contextMappings))
}

// buildContextMappings queries the context store for all contexts and builds a
// mapping of contextName -> []serverName for servers that are in serverTools.
func (s *Server) buildContextMappings(serverTools map[string][]map[string]interface{}) map[string][]string {
	ctx := context.Background()

	contexts, err := s.contextStore.ListContexts(ctx)
	if err != nil {
		slog.Warn("Failed to list contexts for master tool mappings", "error", err)
		return nil
	}

	mappings := make(map[string][]string, len(contexts))
	for _, c := range contexts {
		servers, err := s.contextStore.GetServersForContext(ctx, c.Name)
		if err != nil {
			slog.Warn("Failed to get servers for context", "context", c.Name, "error", err)
			continue
		}

		var enabledServers []string
		for _, srv := range servers {
			if srv.Enabled {
				// Only include servers whose tools we're actually sending
				if _, hasTool := serverTools[srv.ServerName]; hasTool {
					enabledServers = append(enabledServers, srv.ServerName)
				}
			}
		}

		if len(enabledServers) > 0 {
			mappings[c.Name] = enabledServers
		}
	}

	return mappings
}

// handleMasterToolCall processes a master_tool_call from a slave.
// It routes the call to the appropriate MCP server via the master's proxy.
func (s *Server) handleMasterToolCall(conn *slaveConnection, msg slavetypes.Message) {
	var callMsg slavetypes.MasterToolCallMessage
	if err := json.Unmarshal(msg.Data, &callMsg); err != nil {
		slog.Error("Failed to unmarshal master tool call", "hostname", conn.hostname, "error", err)
		s.sendError(conn, msg.ID, "Invalid master tool call message")
		return
	}

	if s.proxy == nil {
		s.sendError(conn, msg.ID, "Master proxy not available")
		return
	}

	slog.Debug("Handling master tool call from slave",
		"hostname", conn.hostname,
		"server", callMsg.Server,
		"tool", callMsg.Tool)

	// Call the tool on the master's proxy using the prefixed name (server_tool)
	prefixedName := callMsg.Server + "_" + callMsg.Tool
	result, err := s.proxy.CallTool(prefixedName, callMsg.Arguments)

	response := slavetypes.ToolCallResponse{
		Success: err == nil,
		Result:  result,
	}
	if err != nil {
		response.Error = err.Error()
	}

	respData, _ := json.Marshal(response)

	// Wrap in an MCPResponse-like structure so the slave's handleResponse can parse it
	mcpResult, _ := json.Marshal(response)
	mcpResp := struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      string          `json:"id"`
		Result  json.RawMessage `json:"result,omitempty"`
	}{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  mcpResult,
	}
	respData, _ = json.Marshal(mcpResp)

	respMsg := slavetypes.Message{
		Type:      slavetypes.MessageTypeResponse,
		ID:        msg.ID,
		Timestamp: time.Now(),
		Data:      respData,
	}

	if err := s.sendMessage(conn, respMsg); err != nil {
		slog.Error("Failed to send master tool call response",
			"hostname", conn.hostname, "error", err)
	}
}

// BroadcastMasterTools sends the current master tools to all connected slaves.
// Call this when a new MCP server starts on the master.
func (s *Server) BroadcastMasterTools() {
	s.connMu.RLock()
	conns := make([]*slaveConnection, 0, len(s.connections))
	for _, conn := range s.connections {
		conns = append(conns, conn)
	}
	s.connMu.RUnlock()

	for _, conn := range conns {
		go s.sendMasterToolsToSlave(conn)
	}
}

// startHeartbeatMonitor monitors slave connections for heartbeat timeout
func (s *Server) startHeartbeatMonitor() {
	s.heartbeatTicker = time.NewTicker(30 * time.Second)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		for {
			select {
			case <-s.ctx.Done():
				s.heartbeatTicker.Stop()
				return
			case <-s.heartbeatTicker.C:
				s.checkHeartbeats()
			}
		}
	}()
}

// checkHeartbeats checks for stale connections
func (s *Server) checkHeartbeats() {
	timeout := 2 * time.Minute
	now := time.Now()

	s.connMu.RLock()
	staleConns := make([]*slaveConnection, 0)
	for _, conn := range s.connections {
		if now.Sub(conn.lastHeartbeat) > timeout {
			staleConns = append(staleConns, conn)
		}
	}
	s.connMu.RUnlock()

	for _, conn := range staleConns {
		slog.Warn("Slave heartbeat timeout, disconnecting", "hostname", conn.hostname)
		conn.conn.Close()

		s.connMu.Lock()
		delete(s.connections, conn.hostname)
		s.connMu.Unlock()

		s.registry.Disconnect(conn.hostname)
	}
}

// Stop stops the server
func (s *Server) Stop() error {
	s.cancel()
	if s.server != nil {
		return s.server.Close()
	}
	s.wg.Wait()
	return nil
}
