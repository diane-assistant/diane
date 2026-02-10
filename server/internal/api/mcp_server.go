package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MCPHTTPServer provides an HTTP/SSE endpoint for MCP clients to connect to Diane
type MCPHTTPServer struct {
	statusProvider StatusProvider
	mcpHandler     MCPHandler
	sessions       map[string]*mcpSession
	sessionsMu     sync.RWMutex
	server         *http.Server
	port           int
}

// MCPHandler interface for handling MCP requests
type MCPHandler interface {
	HandleRequest(req MCPRequest) MCPResponse
	GetTools() ([]ToolInfo, error)
}

// mcpSession represents an active MCP session
type mcpSession struct {
	id          string
	initialized bool
	createdAt   time.Time
	eventChan   chan []byte
	closeChan   chan struct{}
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
	ID      interface{}     `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

// MCPError represents a JSON-RPC error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// NewMCPHTTPServer creates a new MCP HTTP server
func NewMCPHTTPServer(statusProvider StatusProvider, mcpHandler MCPHandler, port int) *MCPHTTPServer {
	return &MCPHTTPServer{
		statusProvider: statusProvider,
		mcpHandler:     mcpHandler,
		sessions:       make(map[string]*mcpSession),
		port:           port,
	}
}

// Start starts the MCP HTTP server
func (s *MCPHTTPServer) Start() error {
	mux := http.NewServeMux()

	// MCP endpoints
	mux.HandleFunc("/mcp", s.handleMCP)             // HTTP Streamable transport
	mux.HandleFunc("/mcp/sse", s.handleSSE)         // SSE transport
	mux.HandleFunc("/mcp/message", s.handleMessage) // SSE message endpoint

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: s.corsMiddleware(mux),
	}

	go func() {
		slog.Info("MCP HTTP server listening", "port", s.port)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("MCP HTTP server error", "error", err)
		}
	}()

	// Start session cleanup goroutine
	go s.cleanupSessions()

	return nil
}

// Stop stops the MCP HTTP server
func (s *MCPHTTPServer) Stop() error {
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

// corsMiddleware adds CORS headers
func (s *MCPHTTPServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, MCP-Protocol-Version, MCP-Session-Id")
		w.Header().Set("Access-Control-Expose-Headers", "MCP-Session-Id")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handleMCP handles HTTP Streamable MCP transport
func (s *MCPHTTPServer) handleMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check Accept header
	accept := r.Header.Get("Accept")
	if accept != "application/json" && accept != "text/event-stream" && accept != "*/*" && accept != "" {
		s.writeError(w, -32600, "Accept header must be application/json or text/event-stream", nil)
		return
	}

	// Parse request
	var req MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, -32700, "Parse error", nil)
		return
	}

	// Get or create session
	sessionID := r.Header.Get("MCP-Session-Id")
	session := s.getSession(sessionID)

	// Handle initialize specially
	if req.Method == "initialize" {
		if session == nil {
			session = s.createSession()
		}
		session.initialized = true
		w.Header().Set("MCP-Session-Id", session.id)
	} else if session == nil || !session.initialized {
		s.writeError(w, -32600, "Session not found or not initialized", map[string]string{
			"hint": "Call initialize method first",
		})
		return
	}

	// Handle the request
	resp := s.mcpHandler.HandleRequest(req)
	resp.JSONRPC = "2.0"
	resp.ID = req.ID

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleSSE handles SSE transport connection
func (s *MCPHTTPServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Create session
	session := s.createSession()

	// Send endpoint event
	messageURL := fmt.Sprintf("/mcp/message?session=%s", session.id)
	fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", messageURL)

	flusher, ok := w.(http.Flusher)
	if ok {
		flusher.Flush()
	}

	// Keep connection alive and forward events
	for {
		select {
		case event := <-session.eventChan:
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(event))
			if ok {
				flusher.Flush()
			}
		case <-session.closeChan:
			return
		case <-r.Context().Done():
			s.removeSession(session.id)
			return
		case <-time.After(30 * time.Second):
			// Send keepalive
			fmt.Fprintf(w, ": keepalive\n\n")
			if ok {
				flusher.Flush()
			}
		}
	}
}

// handleMessage handles SSE message endpoint
func (s *MCPHTTPServer) handleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.URL.Query().Get("session")
	session := s.getSession(sessionID)
	if session == nil {
		s.writeError(w, -32600, "Session not found", nil)
		return
	}

	var req MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, -32700, "Parse error", nil)
		return
	}

	// Handle initialize
	if req.Method == "initialize" {
		session.initialized = true
	} else if !session.initialized {
		s.writeError(w, -32600, "Session not initialized", nil)
		return
	}

	// Handle the request
	resp := s.mcpHandler.HandleRequest(req)
	resp.JSONRPC = "2.0"
	resp.ID = req.ID

	// Send response via SSE
	respBytes, _ := json.Marshal(resp)
	select {
	case session.eventChan <- respBytes:
	default:
		slog.Warn("Session event channel full", "session", sessionID)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
}

// writeError writes a JSON-RPC error response
func (s *MCPHTTPServer) writeError(w http.ResponseWriter, code int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(MCPResponse{
		JSONRPC: "2.0",
		Error: &MCPError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	})
}

// createSession creates a new session
func (s *MCPHTTPServer) createSession() *mcpSession {
	session := &mcpSession{
		id:        uuid.New().String(),
		createdAt: time.Now(),
		eventChan: make(chan []byte, 100),
		closeChan: make(chan struct{}),
	}

	s.sessionsMu.Lock()
	s.sessions[session.id] = session
	s.sessionsMu.Unlock()

	return session
}

// getSession retrieves a session by ID
func (s *MCPHTTPServer) getSession(id string) *mcpSession {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()
	return s.sessions[id]
}

// removeSession removes a session
func (s *MCPHTTPServer) removeSession(id string) {
	s.sessionsMu.Lock()
	if session, ok := s.sessions[id]; ok {
		close(session.closeChan)
		delete(s.sessions, id)
	}
	s.sessionsMu.Unlock()
}

// cleanupSessions removes expired sessions
func (s *MCPHTTPServer) cleanupSessions() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.sessionsMu.Lock()
		now := time.Now()
		for id, session := range s.sessions {
			// Remove sessions older than 1 hour that aren't active
			if now.Sub(session.createdAt) > time.Hour {
				close(session.closeChan)
				delete(s.sessions, id)
			}
		}
		s.sessionsMu.Unlock()
	}
}

// SendNotification sends a notification to all active sessions
func (s *MCPHTTPServer) SendNotification(method string, params interface{}) {
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}
	notifBytes, _ := json.Marshal(notification)

	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	for _, session := range s.sessions {
		if session.initialized {
			select {
			case session.eventChan <- notifBytes:
			default:
				// Channel full, skip
			}
		}
	}
}

// StreamMCP handles a streaming MCP connection (for terminal/interactive use)
func (s *MCPHTTPServer) StreamMCP(w http.ResponseWriter, r *http.Request) {
	// For testing via curl - reads JSON-RPC from request body line by line
	w.Header().Set("Content-Type", "application/x-ndjson")

	flusher, _ := w.(http.Flusher)
	scanner := bufio.NewScanner(r.Body)

	for scanner.Scan() {
		var req MCPRequest
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue
		}

		resp := s.mcpHandler.HandleRequest(req)
		resp.JSONRPC = "2.0"
		resp.ID = req.ID

		respBytes, _ := json.Marshal(resp)
		w.Write(respBytes)
		w.Write([]byte("\n"))
		if flusher != nil {
			flusher.Flush()
		}
	}
}
