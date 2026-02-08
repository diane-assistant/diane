// Package acp provides an ACP server that wraps local CLI agents.
// This allows tools like OpenCode and Gemini CLI to be exposed via the
// Agent Communication Protocol REST API.
package acp

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// LocalAgentServer wraps a local CLI agent with an ACP-compliant HTTP server
type LocalAgentServer struct {
	config LocalAgentConfig
	server *http.Server
	runs   map[string]*Run
	runsMu sync.RWMutex
}

// LocalAgentConfig configures a local agent to be exposed via ACP
type LocalAgentConfig struct {
	Name        string            `json:"name"`
	Command     string            `json:"command"`
	Args        []string          `json:"args"`
	Env         map[string]string `json:"env"`
	Description string            `json:"description"`
	Port        int               `json:"port"`
	WorkDir     string            `json:"work_dir"`
}

// NewLocalAgentServer creates a new ACP server for a local agent
func NewLocalAgentServer(config LocalAgentConfig) *LocalAgentServer {
	return &LocalAgentServer{
		config: config,
		runs:   make(map[string]*Run),
	}
}

// generateRunID generates a unique run ID
func generateRunID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Start starts the ACP server
func (s *LocalAgentServer) Start() error {
	mux := http.NewServeMux()

	// ACP endpoints
	mux.HandleFunc("/ping", s.handlePing)
	mux.HandleFunc("/agents", s.handleAgents)
	mux.HandleFunc("/agents/", s.handleAgent)
	mux.HandleFunc("/runs", s.handleRuns)
	mux.HandleFunc("/runs/", s.handleRun)

	addr := fmt.Sprintf(":%d", s.config.Port)
	s.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	log.Printf("ACP server for '%s' listening on %s", s.config.Name, addr)
	return s.server.ListenAndServe()
}

// Stop stops the ACP server
func (s *LocalAgentServer) Stop() error {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(ctx)
	}
	return nil
}

// handlePing handles GET /ping
func (s *LocalAgentServer) handlePing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleAgents handles GET /agents
func (s *LocalAgentServer) handleAgents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	agents := AgentsListResponse{
		Agents: []AgentManifest{
			{
				Name:               s.config.Name,
				Description:        s.config.Description,
				InputContentTypes:  []string{"text/plain"},
				OutputContentTypes: []string{"text/plain"},
			},
		},
	}

	json.NewEncoder(w).Encode(agents)
}

// handleAgent handles GET /agents/{name}
func (s *LocalAgentServer) handleAgent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	name := strings.TrimPrefix(r.URL.Path, "/agents/")
	if name != s.config.Name {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(&Error{
			Code:    "not_found",
			Message: fmt.Sprintf("Agent '%s' not found", name),
		})
		return
	}

	agent := AgentManifest{
		Name:               s.config.Name,
		Description:        s.config.Description,
		InputContentTypes:  []string{"text/plain"},
		OutputContentTypes: []string{"text/plain"},
	}

	json.NewEncoder(w).Encode(agent)
}

// handleRuns handles POST /runs
func (s *LocalAgentServer) handleRuns(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req RunCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(&Error{
			Code:    "invalid_input",
			Message: err.Error(),
		})
		return
	}

	// Extract text prompt from input
	var prompt string
	for _, msg := range req.Input {
		for _, part := range msg.Parts {
			if part.ContentType == "text/plain" || part.ContentType == "" {
				prompt += part.Content
			}
		}
	}

	// Create run
	run := &Run{
		AgentName: req.AgentName,
		SessionID: req.SessionID,
		RunID:     generateRunID(),
		Status:    RunStatusCreated,
		Output:    []Message{},
		CreatedAt: time.Now(),
	}

	s.runsMu.Lock()
	s.runs[run.RunID] = run
	s.runsMu.Unlock()

	// Execute based on mode
	if req.Mode == RunModeAsync {
		// Async mode - return immediately and run in background
		go s.executeRun(run, prompt)
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(run)
	} else {
		// Sync mode - wait for completion
		s.executeRun(run, prompt)
		json.NewEncoder(w).Encode(run)
	}
}

// handleRun handles GET/POST /runs/{run_id}
func (s *LocalAgentServer) handleRun(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	path := strings.TrimPrefix(r.URL.Path, "/runs/")
	parts := strings.Split(path, "/")
	runID := parts[0]

	s.runsMu.RLock()
	run, ok := s.runs[runID]
	s.runsMu.RUnlock()

	if !ok {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(&Error{
			Code:    "not_found",
			Message: fmt.Sprintf("Run '%s' not found", runID),
		})
		return
	}

	// Handle cancel
	if len(parts) > 1 && parts[1] == "cancel" {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		s.runsMu.Lock()
		run.Status = RunStatusCancelled
		now := time.Now()
		run.FinishedAt = &now
		s.runsMu.Unlock()

		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(run)
		return
	}

	json.NewEncoder(w).Encode(run)
}

// executeRun executes the local agent command
func (s *LocalAgentServer) executeRun(run *Run, prompt string) {
	s.runsMu.Lock()
	run.Status = RunStatusInProgress
	s.runsMu.Unlock()

	// Build command
	args := append([]string{}, s.config.Args...)
	args = append(args, prompt)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, s.config.Command, args...)
	if s.config.WorkDir != "" {
		cmd.Dir = s.config.WorkDir
	}

	// Set environment
	for k, v := range s.config.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Capture output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.failRun(run, err)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		s.failRun(run, err)
		return
	}

	if err := cmd.Start(); err != nil {
		s.failRun(run, err)
		return
	}

	// Read output
	var output strings.Builder
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		output.WriteString(scanner.Text())
		output.WriteString("\n")
	}

	// Read stderr
	var errOutput strings.Builder
	errScanner := bufio.NewScanner(stderr)
	for errScanner.Scan() {
		errOutput.WriteString(errScanner.Text())
		errOutput.WriteString("\n")
	}

	if err := cmd.Wait(); err != nil {
		s.failRun(run, fmt.Errorf("%v: %s", err, errOutput.String()))
		return
	}

	// Complete run
	s.runsMu.Lock()
	run.Status = RunStatusCompleted
	run.Output = []Message{
		NewTextMessage("agent", output.String()),
	}
	now := time.Now()
	run.FinishedAt = &now
	s.runsMu.Unlock()
}

// failRun marks a run as failed
func (s *LocalAgentServer) failRun(run *Run, err error) {
	s.runsMu.Lock()
	run.Status = RunStatusFailed
	run.Error = &Error{
		Code:    "server_error",
		Message: err.Error(),
	}
	now := time.Now()
	run.FinishedAt = &now
	s.runsMu.Unlock()
}

// OpenCodeConfig returns a LocalAgentConfig for OpenCode
func OpenCodeConfig(port int, workDir string) LocalAgentConfig {
	return LocalAgentConfig{
		Name:        "opencode",
		Command:     "opencode",
		Args:        []string{"run"},
		Description: "OpenCode AI Coding Agent",
		Port:        port,
		WorkDir:     workDir,
	}
}

// GeminiCLIConfig returns a LocalAgentConfig for Gemini CLI
func GeminiCLIConfig(port int, workDir string) LocalAgentConfig {
	return LocalAgentConfig{
		Name:        "gemini",
		Command:     "gemini",
		Args:        []string{"-y", "--output-format", "text"},
		Description: "Gemini CLI AI Assistant",
		Port:        port,
		WorkDir:     workDir,
	}
}
