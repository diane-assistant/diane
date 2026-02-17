package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/diane-assistant/diane/internal/acp"
	"github.com/diane-assistant/diane/internal/config"
	"github.com/diane-assistant/diane/internal/db"
	"github.com/diane-assistant/diane/internal/models"
	"github.com/diane-assistant/diane/internal/pairing"
	"github.com/diane-assistant/diane/internal/slave"
)

// MCPServerStatus represents the status of an MCP server
type MCPServerStatus struct {
	Name          string `json:"name"`
	Enabled       bool   `json:"enabled"`
	Connected     bool   `json:"connected"`
	ToolCount     int    `json:"tool_count"`
	PromptCount   int    `json:"prompt_count"`
	ResourceCount int    `json:"resource_count"`
	Error         string `json:"error,omitempty"`
	Builtin       bool   `json:"builtin,omitempty"`
	RequiresAuth  bool   `json:"requires_auth,omitempty"`
	Authenticated bool   `json:"authenticated,omitempty"`
}

// Status represents the overall Diane status
type Status struct {
	Running        bool              `json:"running"`
	PID            int               `json:"pid"`
	Version        string            `json:"version"`
	Platform       string            `json:"platform"`
	Architecture   string            `json:"architecture"`
	Hostname       string            `json:"hostname"`
	Uptime         string            `json:"uptime"`
	UptimeSeconds  int64             `json:"uptime_seconds"`
	StartedAt      time.Time         `json:"started_at"`
	TotalTools     int               `json:"total_tools"`
	MCPServers     []MCPServerStatus `json:"mcp_servers"`
	LogFile        string            `json:"log_file,omitempty"`
	SlaveMode      bool              `json:"slave_mode,omitempty"`
	MasterURL      string            `json:"master_url,omitempty"`
	SlaveConnected bool              `json:"slave_connected,omitempty"`
	SlaveError     string            `json:"slave_error,omitempty"`
}

// ToolInfo represents information about a tool
type ToolInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Server      string                 `json:"server"`
	Builtin     bool                   `json:"builtin"`
	InputSchema map[string]interface{} `json:"input_schema,omitempty"`
}

// PromptArgument represents an argument for a prompt
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// PromptInfo represents information about a prompt
type PromptInfo struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Server      string           `json:"server"`
	Builtin     bool             `json:"builtin"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// ResourceInfo represents information about a resource
type ResourceInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	URI         string `json:"uri"`
	MimeType    string `json:"mime_type,omitempty"`
	Server      string `json:"server"`
	Builtin     bool   `json:"builtin"`
}

// Job represents a scheduled job
type Job struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Command    string    `json:"command"`
	Schedule   string    `json:"schedule"`
	Enabled    bool      `json:"enabled"`
	ActionType string    `json:"action_type,omitempty"`
	AgentName  *string   `json:"agent_name,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// JobExecution represents a job execution log entry
type JobExecution struct {
	ID        int64      `json:"id"`
	JobID     int64      `json:"job_id"`
	JobName   string     `json:"job_name,omitempty"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	ExitCode  *int       `json:"exit_code,omitempty"`
	Stdout    string     `json:"stdout,omitempty"`
	Stderr    string     `json:"stderr,omitempty"`
	Error     *string    `json:"error,omitempty"`
}

// DoctorCheck represents a single diagnostic check result
type DoctorCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "ok", "warn", "fail"
	Message string `json:"message"`
}

// DoctorReport is the full diagnostic report
type DoctorReport struct {
	Healthy bool          `json:"healthy"`
	Checks  []DoctorCheck `json:"checks"`
}

// StatusProvider is an interface for getting status from the main server
type StatusProvider interface {
	GetStatus() Status
	GetMCPServers() []MCPServerStatus
	GetAllTools() []ToolInfo
	GetAllPrompts() []PromptInfo
	GetAllResources() []ResourceInfo
	GetPromptContent(serverName string, promptName string) (json.RawMessage, error)
	ReadResourceContent(serverName string, uri string) (json.RawMessage, error)
	RestartMCPServer(name string) error
	ReloadConfig() error
	GetJobs() ([]Job, error)
	GetJobLogs(jobName string, limit int) ([]JobExecution, error)
	ToggleJob(name string, enabled bool) error
	GetAgentLogs(agentName string, limit int) ([]AgentLog, error)
	CreateAgentLog(agentName, direction, messageType string, content, errMsg *string, durationMs *int) error
	// OAuth methods
	GetOAuthServers() []OAuthServerInfo
	GetOAuthStatus(serverName string) (map[string]interface{}, error)
	StartOAuthLogin(serverName string) (*DeviceCodeInfo, error)
	PollOAuthToken(serverName string, deviceCode string, interval int) error
	DeleteOAuthToken(serverName string) error
}

// OAuthServerInfo represents an MCP server with OAuth configuration
type OAuthServerInfo struct {
	Name          string `json:"name"`
	Provider      string `json:"provider,omitempty"`
	Authenticated bool   `json:"authenticated"`
	Status        string `json:"status"`
}

// DeviceCodeInfo contains the device code flow information for the user
type DeviceCodeInfo struct {
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	DeviceCode      string `json:"device_code"`
}

// AgentLog represents an agent communication log entry
type AgentLog struct {
	ID          int64     `json:"id"`
	AgentName   string    `json:"agent_name"`
	Direction   string    `json:"direction"`
	MessageType string    `json:"message_type"`
	Content     *string   `json:"content,omitempty"`
	Error       *string   `json:"error,omitempty"`
	DurationMs  *int      `json:"duration_ms,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// Server is the Unix socket HTTP API server
type Server struct {
	socketPath     string
	listener       net.Listener
	server         *http.Server
	httpAddr       string       // optional TCP address for HTTP listener (e.g., ":8080")
	httpAPIKey     string       // API key for authenticating TCP HTTP requests
	tcpListener    net.Listener // TCP listener for HTTP access (nil if disabled)
	httpServer     *http.Server // separate http.Server for TCP listener
	statusProvider StatusProvider
	acpManager     *acp.Manager
	gallery        *acp.Gallery
	contextsAPI    *ContextsAPI
	providersAPI   *ProvidersAPI
	mcpServersAPI  *MCPServersAPI
	hostsAPI       *HostsAPI
	slaveManager   *slave.Manager
	pairLimiter    *pairing.RateLimiter // rate limiter for pairing attempts
}

// NewServer creates a new API server
func NewServer(statusProvider StatusProvider, database *db.DB, cfg config.Config, slaveManager *slave.Manager) (*Server, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	socketPath := filepath.Join(home, ".diane", "diane.sock")

	// Remove existing socket if it exists
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to remove existing socket: %w", err)
	}

	// Initialize ACP manager
	acpManager, err := acp.NewManager()
	if err != nil {
		slog.Warn("Failed to initialize ACP manager", "error", err)
	}

	// Initialize Gallery
	gallery, err := acp.NewGallery()
	if err != nil {
		slog.Warn("Failed to initialize ACP gallery", "error", err)
	}

	// Initialize Contexts API with database
	var contextsAPI *ContextsAPI
	if database != nil {
		contextsAPI = NewContextsAPI(database)
		contextsAPI.SetToolProvider(statusProvider)
	}

	// Initialize Providers API with models registry (uses same database)
	var providersAPI *ProvidersAPI
	if database != nil {
		// Initialize models registry
		modelsRegistry := models.NewRegistry(filepath.Join(home, ".diane"))
		// Load registry in background (don't block startup)
		go func() {
			if err := modelsRegistry.Load(); err != nil {
				slog.Warn("Failed to load models registry", "error", err)
			} else {
				slog.Info("Models registry loaded successfully")
			}
		}()
		providersAPI = NewProvidersAPI(database, modelsRegistry)
	}

	// Initialize MCP Servers API (uses same database)
	var mcpServersAPI *MCPServersAPI
	if database != nil {
		mcpServersAPI = NewMCPServersAPI(database, statusProvider)
	}

	// Initialize Hosts API (shows master + slaves)
	var hostsAPI *HostsAPI
	slog.Info("NewServer: About to initialize Hosts API", "database_nil", database == nil, "slaveManager_nil", slaveManager == nil)
	if database != nil {
		hostsAPI = NewHostsAPI(database, slaveManager)
		slog.Info("NewServer: Hosts API initialized", "hostsAPI_nil", hostsAPI == nil)
	} else {
		slog.Warn("NewServer: Database is nil, Hosts API not initialized")
	}

	return &Server{
		socketPath:     socketPath,
		httpAddr:       cfg.HTTPAddr(),
		httpAPIKey:     cfg.HTTP.APIKey,
		statusProvider: statusProvider,
		acpManager:     acpManager,
		gallery:        gallery,
		contextsAPI:    contextsAPI,
		providersAPI:   providersAPI,
		mcpServersAPI:  mcpServersAPI,
		hostsAPI:       hostsAPI,
		slaveManager:   slaveManager,
		pairLimiter:    pairing.NewRateLimiter(),
	}, nil
}

// readOnlyMiddleware wraps a handler to reject non-GET/HEAD requests with 405.
// This is used on the TCP HTTP listener when no API key is configured,
// enforcing read-only access from remote clients.
// The /pair endpoint is exempt to allow pairing without auth.
func readOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow pairing endpoints without auth
		if r.URL.Path == "/pair" || r.URL.Path == "/slaves/pair" || strings.HasPrefix(r.URL.Path, "/slaves/pair/") {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed (read-only listener)", http.StatusMethodNotAllowed)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// apiKeyAuthMiddleware wraps a handler to require a valid API key.
// The key must be provided via the Authorization header as "Bearer <key>".
// When an API key is configured, all HTTP methods are allowed (full access).
// The /pair endpoint is exempt from auth to allow pairing code exchange.
func apiKeyAuthMiddleware(apiKey string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow pairing endpoints without auth
		if r.URL.Path == "/pair" || r.URL.Path == "/slaves/pair" || strings.HasPrefix(r.URL.Path, "/slaves/pair/") {
			next.ServeHTTP(w, r)
			return
		}

		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="diane"`)
			http.Error(w, "Authorization required", http.StatusUnauthorized)
			return
		}

		// Accept "Bearer <key>" format
		const prefix = "Bearer "
		if !strings.HasPrefix(auth, prefix) {
			http.Error(w, "Invalid authorization format (expected: Bearer <api-key>)", http.StatusUnauthorized)
			return
		}

		providedKey := strings.TrimPrefix(auth, prefix)
		if providedKey != apiKey {
			http.Error(w, "Invalid API key", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Start starts the API server
func (s *Server) Start() error {
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on socket: %w", err)
	}
	s.listener = listener

	// Set socket permissions to be readable/writable by owner only
	if err := os.Chmod(s.socketPath, 0600); err != nil {
		slog.Warn("Failed to set socket permissions", "error", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/doctor", s.handleDoctor)
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/tools", s.handleTools)
	mux.HandleFunc("/prompts", s.handlePrompts)
	mux.HandleFunc("/prompts/get", s.handlePromptGet)
	mux.HandleFunc("/resources", s.handleResources)
	mux.HandleFunc("/resources/read", s.handleResourceRead)
	mux.HandleFunc("/mcp-servers", s.handleMCPServers)
	mux.HandleFunc("/mcp-servers/", s.handleMCPServerAction)
	mux.HandleFunc("/reload", s.handleReload)
	mux.HandleFunc("/jobs", s.handleJobs)
	mux.HandleFunc("/jobs/logs", s.handleJobLogs)
	mux.HandleFunc("/jobs/", s.handleJobAction)
	mux.HandleFunc("/agents", s.handleAgents)
	mux.HandleFunc("/agents/logs", s.handleAgentLogs)
	mux.HandleFunc("/agents/", s.handleAgentAction)
	mux.HandleFunc("/gallery", s.handleGallery)
	mux.HandleFunc("/gallery/", s.handleGalleryAction)
	mux.HandleFunc("/auth", s.handleAuth)
	mux.HandleFunc("/auth/", s.handleAuthAction)
	mux.HandleFunc("/pair", s.handlePair)

	// Register Contexts API routes
	if s.contextsAPI != nil {
		s.contextsAPI.RegisterRoutes(mux)
	}

	// Register Providers API routes
	if s.providersAPI != nil {
		s.providersAPI.RegisterRoutes(mux)
	}

	// Register MCP Servers API routes
	if s.mcpServersAPI != nil {
		s.mcpServersAPI.RegisterRoutes(mux)
	}

	// Register Hosts API routes
	if s.hostsAPI != nil {
		slog.Info("Registering Hosts API routes")
		s.hostsAPI.RegisterRoutes(mux)
	} else {
		slog.Warn("Hosts API is nil, routes not registered")
	}

	// Register Slave Management API routes
	if s.slaveManager != nil {
		RegisterSlaveRoutes(mux, s)
	}

	s.server = &http.Server{Handler: mux}

	go func() {
		slog.Info("API server listening", "socket", s.socketPath)
		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			slog.Error("API server error", "error", err)
		}
	}()

	// Optionally start TCP HTTP listener for remote access
	// If API key is configured: full read/write access with API key authentication
	// If API key is not configured: read-only access (GET/HEAD only, no auth required)
	if s.httpAddr != "" {
		tcpListener, err := net.Listen("tcp", s.httpAddr)
		if err != nil {
			slog.Error("Failed to start HTTP listener", "addr", s.httpAddr, "error", err)
			// Non-fatal: Unix socket still works
		} else {
			s.tcpListener = tcpListener
			if s.httpAPIKey != "" {
				s.httpServer = &http.Server{Handler: apiKeyAuthMiddleware(s.httpAPIKey, mux)}
				go func() {
					slog.Info("HTTP listener started (API key auth, full access)", "addr", s.httpAddr)
					if err := s.httpServer.Serve(tcpListener); err != nil && err != http.ErrServerClosed {
						slog.Error("HTTP listener error", "error", err)
					}
				}()
			} else {
				s.httpServer = &http.Server{Handler: readOnlyMiddleware(mux)}
				go func() {
					slog.Info("HTTP listener started (read-only, no auth)", "addr", s.httpAddr)
					if err := s.httpServer.Serve(tcpListener); err != nil && err != http.ErrServerClosed {
						slog.Error("HTTP listener error", "error", err)
					}
				}()
			}
		}
	}

	return nil
}

// Stop stops the API server
func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shut down TCP HTTP listener first (if running)
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			slog.Error("Failed to shut down HTTP listener", "error", err)
		}
	}
	if s.tcpListener != nil {
		s.tcpListener.Close()
	}

	// Shut down Unix socket server
	if s.server != nil {
		if err := s.server.Shutdown(ctx); err != nil {
			return err
		}
	}
	if s.listener != nil {
		s.listener.Close()
	}
	os.Remove(s.socketPath)
	return nil
}

// handleHealth returns a simple health check response
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleDoctor runs diagnostic checks and returns a report
func (s *Server) handleDoctor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var checks []DoctorCheck
	healthy := true

	// 1. Daemon running (trivially true if we're responding)
	checks = append(checks, DoctorCheck{
		Name:    "daemon",
		Status:  "ok",
		Message: "Diane daemon is running",
	})

	// 2. Socket file
	socketPath := GetSocketPath()
	if _, err := os.Stat(socketPath); err != nil {
		healthy = false
		checks = append(checks, DoctorCheck{
			Name:    "socket",
			Status:  "fail",
			Message: fmt.Sprintf("Unix socket missing: %s", socketPath),
		})
	} else {
		checks = append(checks, DoctorCheck{
			Name:    "socket",
			Status:  "ok",
			Message: fmt.Sprintf("Unix socket: %s", socketPath),
		})
	}

	// 3. MCP HTTP server (port 8765)
	httpClient := &http.Client{Timeout: 3 * time.Second}
	resp, err := httpClient.Get("http://localhost:8765/health")
	if err != nil {
		healthy = false
		checks = append(checks, DoctorCheck{
			Name:    "mcp_http",
			Status:  "fail",
			Message: fmt.Sprintf("MCP HTTP server not reachable: %s", err),
		})
	} else {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			checks = append(checks, DoctorCheck{
				Name:    "mcp_http",
				Status:  "ok",
				Message: "MCP HTTP server listening on :8765",
			})
		} else {
			healthy = false
			checks = append(checks, DoctorCheck{
				Name:    "mcp_http",
				Status:  "fail",
				Message: fmt.Sprintf("MCP HTTP /health returned %d", resp.StatusCode),
			})
		}
	}

	// 4. MCP SSE endpoint
	sseReq, _ := http.NewRequest(http.MethodGet, "http://localhost:8765/mcp/sse", nil)
	sseCtx, sseCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer sseCancel()
	sseReq = sseReq.WithContext(sseCtx)
	sseResp, err := httpClient.Do(sseReq)
	if err != nil {
		// context deadline exceeded is expected since SSE stays open — check if we got headers
		healthy = false
		checks = append(checks, DoctorCheck{
			Name:    "mcp_sse",
			Status:  "fail",
			Message: fmt.Sprintf("MCP SSE endpoint not reachable: %s", err),
		})
	} else {
		sseResp.Body.Close()
		ct := sseResp.Header.Get("Content-Type")
		if sseResp.StatusCode == http.StatusOK && strings.Contains(ct, "text/event-stream") {
			checks = append(checks, DoctorCheck{
				Name:    "mcp_sse",
				Status:  "ok",
				Message: "MCP SSE endpoint responding at /mcp/sse",
			})
		} else {
			healthy = false
			checks = append(checks, DoctorCheck{
				Name:    "mcp_sse",
				Status:  "fail",
				Message: fmt.Sprintf("MCP SSE returned status %d, content-type: %s", sseResp.StatusCode, ct),
			})
		}
	}

	// 5. MCP Streamable endpoint
	initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"diane-doctor","version":"1.0"}}}`
	streamResp, err := httpClient.Post("http://localhost:8765/mcp", "application/json", strings.NewReader(initBody))
	if err != nil {
		healthy = false
		checks = append(checks, DoctorCheck{
			Name:    "mcp_streamable",
			Status:  "fail",
			Message: fmt.Sprintf("MCP Streamable endpoint not reachable: %s", err),
		})
	} else {
		streamResp.Body.Close()
		if streamResp.StatusCode == http.StatusOK {
			checks = append(checks, DoctorCheck{
				Name:    "mcp_streamable",
				Status:  "ok",
				Message: "MCP Streamable endpoint responding at /mcp",
			})
		} else {
			healthy = false
			checks = append(checks, DoctorCheck{
				Name:    "mcp_streamable",
				Status:  "fail",
				Message: fmt.Sprintf("MCP Streamable /mcp returned %d", streamResp.StatusCode),
			})
		}
	}

	// 6. Built-in MCP servers
	servers := s.statusProvider.GetMCPServers()
	activeCount := 0
	failedServers := []string{}
	for _, srv := range servers {
		if srv.Connected {
			activeCount++
		} else if srv.Enabled && srv.Error != "" {
			failedServers = append(failedServers, fmt.Sprintf("%s (%s)", srv.Name, srv.Error))
		}
	}
	if len(failedServers) > 0 {
		healthy = false
		checks = append(checks, DoctorCheck{
			Name:    "mcp_servers",
			Status:  "fail",
			Message: fmt.Sprintf("%d/%d servers active, failed: %s", activeCount, len(servers), strings.Join(failedServers, ", ")),
		})
	} else {
		checks = append(checks, DoctorCheck{
			Name:    "mcp_servers",
			Status:  "ok",
			Message: fmt.Sprintf("%d/%d MCP servers active", activeCount, len(servers)),
		})
	}

	// 7. Database
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".diane", "cron.db")
	if info, err := os.Stat(dbPath); err != nil {
		healthy = false
		checks = append(checks, DoctorCheck{
			Name:    "database",
			Status:  "fail",
			Message: fmt.Sprintf("Database not found: %s", dbPath),
		})
	} else {
		checks = append(checks, DoctorCheck{
			Name:    "database",
			Status:  "ok",
			Message: fmt.Sprintf("Database: %s (%.1f KB)", dbPath, float64(info.Size())/1024),
		})
	}

	// 8. PID file
	pidPath := filepath.Join(home, ".diane", "mcp.pid")
	if _, err := os.Stat(pidPath); err != nil {
		checks = append(checks, DoctorCheck{
			Name:    "pid_file",
			Status:  "warn",
			Message: "PID file missing (daemon may have been started differently)",
		})
	} else {
		checks = append(checks, DoctorCheck{
			Name:    "pid_file",
			Status:  "ok",
			Message: fmt.Sprintf("PID file: %s", pidPath),
		})
	}

	report := DoctorReport{
		Healthy: healthy,
		Checks:  checks,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}

// handleStatus returns the full status
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := s.statusProvider.GetStatus()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleTools returns the list of all available tools
func (s *Server) handleTools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tools := s.statusProvider.GetAllTools()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tools)
}

// handlePrompts returns the list of all available prompts
func (s *Server) handlePrompts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	prompts := s.statusProvider.GetAllPrompts()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(prompts)
}

// handleResources returns the list of all available resources
func (s *Server) handleResources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resources := s.statusProvider.GetAllResources()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resources)
}

// handlePromptGet returns the full content of a specific prompt
func (s *Server) handlePromptGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	serverName := r.URL.Query().Get("server")
	promptName := r.URL.Query().Get("name")
	if serverName == "" || promptName == "" {
		http.Error(w, "server and name query parameters are required", http.StatusBadRequest)
		return
	}

	result, err := s.statusProvider.GetPromptContent(serverName, promptName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(result)
}

// handleResourceRead returns the full content of a specific resource
func (s *Server) handleResourceRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	serverName := r.URL.Query().Get("server")
	uri := r.URL.Query().Get("uri")
	if serverName == "" || uri == "" {
		http.Error(w, "server and uri query parameters are required", http.StatusBadRequest)
		return
	}

	result, err := s.statusProvider.ReadResourceContent(serverName, uri)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(result)
}

// handleMCPServers returns the list of MCP servers
func (s *Server) handleMCPServers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	servers := s.statusProvider.GetMCPServers()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(servers)
}

// handleMCPServerAction handles actions on specific MCP servers
func (s *Server) handleMCPServerAction(w http.ResponseWriter, r *http.Request) {
	// Parse the path: /mcp-servers/{name}/restart
	path := strings.TrimPrefix(r.URL.Path, "/mcp-servers/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	serverName := parts[0]
	action := parts[1]

	switch action {
	case "restart":
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := s.statusProvider.RestartMCPServer(serverName); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "restarted", "server": serverName})
	default:
		http.Error(w, "Unknown action", http.StatusBadRequest)
	}
}

// handleReload reloads the MCP configuration
func (s *Server) handleReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.statusProvider.ReloadConfig(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "reloaded"})
}

// handleJobs returns the list of scheduled jobs
func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobs, err := s.statusProvider.GetJobs()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

// handleJobLogs returns job execution logs
func (s *Server) handleJobLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobName := r.URL.Query().Get("job_name")
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	logs, err := s.statusProvider.GetJobLogs(jobName, limit)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

// handleJobAction handles actions on specific jobs
func (s *Server) handleJobAction(w http.ResponseWriter, r *http.Request) {
	// Parse the path: /jobs/{name}/toggle
	path := strings.TrimPrefix(r.URL.Path, "/jobs/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	jobName := parts[0]
	action := parts[1]

	switch action {
	case "toggle":
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if err := s.statusProvider.ToggleJob(jobName, body.Enabled); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "job": jobName, "enabled": body.Enabled})
	default:
		http.Error(w, "Unknown action", http.StatusBadRequest)
	}
}

// GetSocketPath returns the socket path for clients
func GetSocketPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".diane", "diane.sock")
}

// CheckProcessRunning checks if a process with the given PID is running
func CheckProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, sending signal 0 checks if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// GetPIDFromFile reads the PID from a file
func GetPIDFromFile(pidPath string) (int, error) {
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// handleAgents handles listing and adding ACP agents
func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.acpManager == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "ACP manager not initialized"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		agents := s.acpManager.ListAgents()
		json.NewEncoder(w).Encode(agents)

	case http.MethodPost:
		var agent acp.AgentConfig
		if err := json.NewDecoder(r.Body).Decode(&agent); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body: " + err.Error()})
			return
		}

		if agent.Name == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Agent name is required"})
			return
		}

		if agent.URL == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Agent URL is required"})
			return
		}

		if err := s.acpManager.AddAgent(agent); err != nil {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "created", "agent": agent.Name})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
	}
}

// handleAgentAction handles actions on specific ACP agents
func (s *Server) handleAgentAction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.acpManager == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "ACP manager not initialized"})
		return
	}

	// Parse the path: /agents/{name} or /agents/{name}/action
	path := strings.TrimPrefix(r.URL.Path, "/agents/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Agent name required"})
		return
	}

	agentName := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch action {
	case "": // GET /agents/{name} or DELETE /agents/{name}
		switch r.Method {
		case http.MethodGet:
			agent, err := s.acpManager.GetAgent(agentName)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			json.NewEncoder(w).Encode(agent)

		case http.MethodDelete:
			if err := s.acpManager.RemoveAgent(agentName); err != nil {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "agent": agentName})

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		}

	case "toggle":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
			return
		}

		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		if err := s.acpManager.EnableAgent(agentName, body.Enabled); err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "agent": agentName, "enabled": body.Enabled})

	case "test":
		if r.Method != http.MethodPost && r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
			return
		}

		startTime := time.Now()
		result, err := s.acpManager.TestAgent(agentName)
		durationMs := int(time.Since(startTime).Milliseconds())

		if err != nil {
			errMsg := err.Error()
			s.statusProvider.CreateAgentLog(agentName, "response", "test", nil, &errMsg, &durationMs)
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		// Log the test result
		var errMsg *string
		if result.Error != "" {
			errMsg = &result.Error
		}
		s.statusProvider.CreateAgentLog(agentName, "response", "test", &result.Status, errMsg, &durationMs)

		json.NewEncoder(w).Encode(result)

	case "run":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
			return
		}

		var body struct {
			Prompt string `json:"prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		if body.Prompt == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Prompt is required"})
			return
		}

		// Log the request
		promptContent := body.Prompt
		s.statusProvider.CreateAgentLog(agentName, "request", "run", &promptContent, nil, nil)

		startTime := time.Now()
		run, err := s.acpManager.RunAgent(agentName, body.Prompt)
		durationMs := int(time.Since(startTime).Milliseconds())

		if err != nil {
			errMsg := err.Error()
			s.statusProvider.CreateAgentLog(agentName, "response", "run", nil, &errMsg, &durationMs)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		// Log the response
		responseContent := run.GetTextOutput()
		var errMsg *string
		if run.Error != nil {
			e := run.Error.Message
			errMsg = &e
		}
		s.statusProvider.CreateAgentLog(agentName, "response", "run", &responseContent, errMsg, &durationMs)

		json.NewEncoder(w).Encode(run)

	case "remote-agents":
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
			return
		}

		remoteAgents, err := s.acpManager.GetRemoteAgents(agentName)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		json.NewEncoder(w).Encode(remoteAgents)

	case "update":
		if r.Method != http.MethodPost && r.Method != http.MethodPatch {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
			return
		}

		var updates map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		if err := s.acpManager.UpdateAgent(agentName, updates); err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"status": "updated", "agent": agentName})

	default:
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unknown action: " + action})
	}
}

// handleAgentLogs handles agent communication logs
func (s *Server) handleAgentLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	agentName := r.URL.Query().Get("agent_name")
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	logs, err := s.statusProvider.GetAgentLogs(agentName, limit)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(logs)
}

// handleGallery handles the agent gallery endpoints
func (s *Server) handleGallery(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.gallery == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "Gallery not initialized"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		// Check for ?featured=true query param
		if r.URL.Query().Get("featured") == "true" {
			entries, err := s.gallery.ListFeatured()
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			json.NewEncoder(w).Encode(entries)
		} else {
			entries, err := s.gallery.ListGallery()
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			json.NewEncoder(w).Encode(entries)
		}

	case http.MethodPost:
		// POST /gallery refreshes the registry
		if err := s.gallery.RefreshRegistry(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "refreshed"})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
	}
}

// handleGalleryAction handles actions on gallery entries
func (s *Server) handleGalleryAction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.gallery == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "Gallery not initialized"})
		return
	}

	// Parse path: /gallery/{id} or /gallery/{id}/install
	path := strings.TrimPrefix(r.URL.Path, "/gallery/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Agent ID required"})
		return
	}

	agentID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch action {
	case "": // GET /gallery/{id}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
			return
		}

		info, err := s.gallery.GetInstallInfo(agentID)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(info)

	case "install":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
			return
		}

		// Parse optional request body for custom name, workdir, and port
		var body struct {
			Name    string `json:"name"`
			WorkDir string `json:"workdir"`
			Port    int    `json:"port"`
			Type    string `json:"type"` // "stdio" or "acp"
		}
		if r.Body != nil {
			json.NewDecoder(r.Body).Decode(&body)
		}

		// Get install info
		info, err := s.gallery.GetInstallInfo(agentID)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		// Add to configured agents (doesn't actually install, just configures)
		if s.acpManager != nil {
			// Determine the agent name
			agentName := body.Name
			if agentName == "" {
				agentName = agentID
			}

			// Build args with workdir if provided
			args := info.Args
			if body.WorkDir != "" {
				args = acp.BuildArgsWithWorkDir(agentID, info.Args, body.WorkDir)
			}

			// Determine agent type - use ACP if port is specified, otherwise stdio
			agentType := body.Type
			if agentType == "" {
				if body.Port > 0 {
					agentType = "acp"
				} else {
					agentType = "stdio"
				}
			}

			agentConfig := acp.AgentConfig{
				Name:        agentName,
				Type:        agentType,
				Command:     info.Command,
				Args:        args,
				Env:         info.Env,
				WorkDir:     body.WorkDir,
				Port:        body.Port,
				Enabled:     true,
				Description: info.Description,
			}

			// Set URL for ACP agents
			if agentType == "acp" && body.Port > 0 {
				agentConfig.URL = fmt.Sprintf("http://localhost:%d", body.Port)
			}

			if err := s.acpManager.AddAgent(agentConfig); err != nil {
				// Agent might already exist
				if !strings.Contains(err.Error(), "already exists") {
					w.WriteHeader(http.StatusConflict)
					json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				}
			}

			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":      "configured",
				"agent":       agentName,
				"install_cmd": info.InstallCmd,
				"info":        info,
			})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":      "configured",
				"agent":       agentID,
				"install_cmd": info.InstallCmd,
				"info":        info,
			})
		}

	default:
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unknown action: " + action})
	}
}

// handleAuth handles listing OAuth servers and their status
func (s *Server) handleAuth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	servers := s.statusProvider.GetOAuthServers()
	json.NewEncoder(w).Encode(servers)
}

// handleAuthAction handles OAuth actions for specific servers
func (s *Server) handleAuthAction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse path: /auth/{server} or /auth/{server}/action
	path := strings.TrimPrefix(r.URL.Path, "/auth/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Server name required"})
		return
	}

	serverName := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch action {
	case "", "status": // GET /auth/{server} or /auth/{server}/status
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
			return
		}

		status, err := s.statusProvider.GetOAuthStatus(serverName)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(status)

	case "login":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
			return
		}

		deviceInfo, err := s.statusProvider.StartOAuthLogin(serverName)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(deviceInfo)

	case "poll":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
			return
		}

		var body struct {
			DeviceCode string `json:"device_code"`
			Interval   int    `json:"interval"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		if err := s.statusProvider.PollOAuthToken(serverName, body.DeviceCode, body.Interval); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"status": "authenticated"})

	case "logout":
		if r.Method != http.MethodPost && r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
			return
		}

		if err := s.statusProvider.DeleteOAuthToken(serverName); err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

	default:
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unknown action: " + action})
	}
}

// handlePair handles pairing code exchange.
// POST /pair with {"code": "123456"} validates the time-based pairing code
// and returns the API key if valid. This endpoint bypasses auth middleware
// and is rate-limited to prevent brute-force attacks.
func (s *Server) handlePair(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	// Pairing requires an API key to be configured on the server
	if s.httpAPIKey == "" {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "Pairing not available (no API key configured)"})
		return
	}

	// Rate limiting
	clientIP := pairing.ClientIP(r)
	if !s.pairLimiter.Allow(clientIP) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]string{"error": "Too many pairing attempts. Try again later."})
		return
	}

	// Parse request body
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	if body.Code == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Pairing code is required"})
		return
	}

	// Validate the code
	if !pairing.ValidateCode(s.httpAPIKey, body.Code) {
		// Record failed attempt for rate limiting
		s.pairLimiter.Record(clientIP)
		slog.Info("Pairing code rejected", "ip", clientIP)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid or expired pairing code"})
		return
	}

	// Success — return the API key
	slog.Info("Pairing successful", "ip", clientIP)
	json.NewEncoder(w).Encode(map[string]string{
		"api_key": s.httpAPIKey,
	})
}
