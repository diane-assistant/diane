package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/diane-assistant/diane/internal/acp"
)

// MCPServerStatus represents the status of an MCP server
type MCPServerStatus struct {
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	Connected bool   `json:"connected"`
	ToolCount int    `json:"tool_count"`
	Error     string `json:"error,omitempty"`
	Builtin   bool   `json:"builtin,omitempty"`
}

// Status represents the overall Diane status
type Status struct {
	Running       bool              `json:"running"`
	PID           int               `json:"pid"`
	Version       string            `json:"version"`
	Uptime        string            `json:"uptime"`
	UptimeSeconds int64             `json:"uptime_seconds"`
	StartedAt     time.Time         `json:"started_at"`
	TotalTools    int               `json:"total_tools"`
	MCPServers    []MCPServerStatus `json:"mcp_servers"`
}

// ToolInfo represents information about a tool
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
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

// StatusProvider is an interface for getting status from the main server
type StatusProvider interface {
	GetStatus() Status
	GetMCPServers() []MCPServerStatus
	GetAllTools() []ToolInfo
	RestartMCPServer(name string) error
	ReloadConfig() error
	GetJobs() ([]Job, error)
	GetJobLogs(jobName string, limit int) ([]JobExecution, error)
	ToggleJob(name string, enabled bool) error
	GetAgentLogs(agentName string, limit int) ([]AgentLog, error)
	CreateAgentLog(agentName, direction, messageType string, content, errMsg *string, durationMs *int) error
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
	statusProvider StatusProvider
	acpManager     *acp.Manager
	gallery        *acp.Gallery
}

// NewServer creates a new API server
func NewServer(statusProvider StatusProvider) (*Server, error) {
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
		log.Printf("Warning: failed to initialize ACP manager: %v", err)
	}

	// Initialize Gallery
	gallery, err := acp.NewGallery()
	if err != nil {
		log.Printf("Warning: failed to initialize ACP gallery: %v", err)
	}

	return &Server{
		socketPath:     socketPath,
		statusProvider: statusProvider,
		acpManager:     acpManager,
		gallery:        gallery,
	}, nil
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
		log.Printf("Warning: failed to set socket permissions: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/tools", s.handleTools)
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

	s.server = &http.Server{Handler: mux}

	go func() {
		log.Printf("API server listening on %s", s.socketPath)
		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("API server error: %v", err)
		}
	}()

	return nil
}

// Stop stops the API server
func (s *Server) Stop() error {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
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
