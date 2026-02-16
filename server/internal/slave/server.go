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

	"github.com/diane-assistant/diane/internal/db"
	"github.com/diane-assistant/diane/internal/slavetypes"
	"github.com/gorilla/websocket"
)

// Server manages WebSocket connections from slave Diane instances
type Server struct {
	ca              *CertificateAuthority
	registry        *Registry
	db              *db.DB
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
func NewServer(ca *CertificateAuthority, registry *Registry, database *db.DB, pairing *PairingService) *Server {
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
func Start(addr string, ca *CertificateAuthority, registry *Registry, database *db.DB, pairing *PairingService) (*Server, error) {
	srv := NewServer(ca, registry, database, pairing)

	// Create TLS config with optional client certificate authentication
	// This allows pairing requests (without cert) to proceed
	tlsConfig := &tls.Config{
		ClientAuth: tls.VerifyClientCertIfGiven,
		ClientCAs:  x509.NewCertPool(),
		MinVersion: tls.VersionTLS12,
	}

	// Add CA certificate to client CA pool
	caCert, err := ca.GetCertificate()
	if err != nil {
		return nil, fmt.Errorf("failed to get CA certificate: %w", err)
	}
	tlsConfig.ClientCAs.AddCert(caCert)

	// Setup HTTP server with WebSocket handler and pairing endpoints
	mux := http.NewServeMux()
	mux.HandleFunc("/slave/connect", srv.handleConnect)
	mux.HandleFunc("/api/slaves/pair", srv.handlePair)
	mux.HandleFunc("/api/slaves/pair/", srv.handlePairStatus)

	srv.server = &http.Server{
		Addr:      addr,
		Handler:   mux,
		TLSConfig: tlsConfig,
	}

	// Start heartbeat monitor
	srv.startHeartbeatMonitor()

	// Start server in background
	srv.wg.Add(1)
	go func() {
		defer srv.wg.Done()
		slog.Info("Starting slave WebSocket server", "addr", addr)

		// Load CA cert and key for TLS
		caCertPath, caKeyPath, err := ca.GetPaths()
		if err != nil {
			slog.Error("Failed to get CA paths", "error", err)
			return
		}

		if err := srv.server.ListenAndServeTLS(caCertPath, caKeyPath); err != nil && err != http.ErrServerClosed {
			slog.Error("Slave server error", "error", err)
		}
	}()

	return srv, nil
}

// handleConnect handles WebSocket upgrade and slave connection
func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	// Verify client certificate
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		http.Error(w, "Client certificate required", http.StatusUnauthorized)
		return
	}

	clientCert := r.TLS.PeerCertificates[0]
	hostname := clientCert.Subject.CommonName

	// Get slave ID from certificate
	slave, err := s.db.GetSlaveServerByHostID(hostname)
	if err != nil {
		slog.Error("Failed to get slave server", "hostname", hostname, "error", err)
		http.Error(w, "Unknown slave", http.StatusUnauthorized)
		return
	}

	// Check if credentials are revoked
	revoked, err := s.db.IsCredentialRevoked(slave.CertSerial)
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
	if err := s.db.UpdateSlaveLastSeen(hostname); err != nil {
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
		"hostname", reg.Hostname,
		"version", reg.Version,
		"tools", len(reg.Tools))

	// Update registry
	s.registry.Connect(reg.Hostname, reg.Tools, conn.conn)

	// Send acknowledgment
	s.sendMessage(conn, slavetypes.Message{
		Type:      slavetypes.MessageTypeResponse,
		ID:        msg.ID,
		Timestamp: time.Now(),
		Data:      json.RawMessage(`{"status":"registered"}`),
	})
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
