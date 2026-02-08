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
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Command   string    `json:"command"`
	Schedule  string    `json:"schedule"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
}

// Server is the Unix socket HTTP API server
type Server struct {
	socketPath     string
	listener       net.Listener
	server         *http.Server
	statusProvider StatusProvider
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

	return &Server{
		socketPath:     socketPath,
		statusProvider: statusProvider,
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
