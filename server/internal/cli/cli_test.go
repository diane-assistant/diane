package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/diane-assistant/diane/internal/acp"
	"github.com/diane-assistant/diane/internal/api"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Test infrastructure
// ---------------------------------------------------------------------------

// unixRedirectTransport intercepts http://unix/... URLs and rewrites them
// to the test server URL, allowing api.Client methods to hit the mock server.
type unixRedirectTransport struct {
	targetURL *url.URL
	inner     http.RoundTripper
}

func (t *unixRedirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == "unix" {
		req2 := req.Clone(req.Context())
		req2.URL.Scheme = t.targetURL.Scheme
		req2.URL.Host = t.targetURL.Host
		return t.inner.RoundTrip(req2)
	}
	return t.inner.RoundTrip(req)
}

// newTestClient creates an api.Client that routes all http://unix/... requests
// to the provided httptest.Server.
func newTestClient(ts *httptest.Server) *api.Client {
	u, _ := url.Parse(ts.URL)
	httpClient := &http.Client{
		Transport: &unixRedirectTransport{
			targetURL: u,
			inner:     http.DefaultTransport,
		},
	}
	return api.NewClientWithHTTPClient(httpClient)
}

// captureStdout captures everything written to os.Stdout during f().
func captureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// executeCmd runs the root command with the given args, capturing stdout.
// Returns the captured output and any error from Execute().
func executeCmd(root *cobra.Command, args ...string) (string, error) {
	var execErr error
	output := captureStdout(func() {
		root.SetArgs(args)
		execErr = root.Execute()
	})
	return output, execErr
}

// newTestRootCmd is a helper that creates a root command pointing at a test server.
func newTestRootCmd(ts *httptest.Server) *cobra.Command {
	client := newTestClient(ts)
	return NewRootCmd(client, "v1.0.0-test")
}

// jsonOK writes a JSON 200 response.
func jsonOK(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// jsonStatus writes a JSON response with the given status code.
func jsonStatus(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

// ---------------------------------------------------------------------------
// Fixture data
// ---------------------------------------------------------------------------

func fixtureStatus() api.Status {
	return api.Status{
		Running:       true,
		PID:           12345,
		Version:       "v1.0.0-test",
		Platform:      "darwin",
		Architecture:  "arm64",
		Hostname:      "test-host",
		Uptime:        "1h 30m",
		UptimeSeconds: 5400,
		StartedAt:     time.Now().Add(-5400 * time.Second),
		TotalTools:    42,
		MCPServers: []api.MCPServerStatus{
			{Name: "filesystem", Enabled: true, Connected: true, ToolCount: 10, Builtin: true},
			{Name: "brave-search", Enabled: true, Connected: true, ToolCount: 3, PromptCount: 1},
			{Name: "broken-server", Enabled: true, Connected: false, Error: "connection refused"},
			{Name: "disabled-srv", Enabled: false},
		},
	}
}

func fixtureDoctorHealthy() api.DoctorReport {
	return api.DoctorReport{
		Healthy: true,
		Checks: []api.DoctorCheck{
			{Name: "Socket", Status: "ok", Message: "Unix socket is accessible"},
			{Name: "Database", Status: "ok", Message: "Database is healthy"},
			{Name: "Config", Status: "ok", Message: "Configuration is valid"},
		},
	}
}

func fixtureDoctorUnhealthy() api.DoctorReport {
	return api.DoctorReport{
		Healthy: false,
		Checks: []api.DoctorCheck{
			{Name: "Socket", Status: "ok", Message: "Unix socket is accessible"},
			{Name: "Database", Status: "fail", Message: "Database locked"},
			{Name: "Network", Status: "warn", Message: "DNS resolution slow"},
		},
	}
}

func fixtureMCPServers() []api.MCPServerStatus {
	return fixtureStatus().MCPServers
}

func fixtureMCPConfigs() []api.MCPServerResponse {
	return []api.MCPServerResponse{
		{ID: 1, Name: "filesystem", Enabled: true, Type: "builtin"},
		{ID: 2, Name: "brave-search", Enabled: true, Type: "http", URL: "http://localhost:3000"},
		{ID: 3, Name: "broken-server", Enabled: true, Type: "stdio", Command: "broken-bin"},
		{ID: 4, Name: "disabled-srv", Enabled: false, Type: "sse"},
	}
}

func fixtureAgents() []acp.AgentConfig {
	return []acp.AgentConfig{
		{Name: "codey", URL: "http://localhost:8000", Enabled: true, Description: "Coding assistant"},
		{Name: "researcher", URL: "http://localhost:8001", Enabled: true, Description: "Research agent"},
		{Name: "disabled-agent", URL: "http://localhost:8002", Enabled: false},
	}
}

func fixtureAgent(name string) *acp.AgentConfig {
	for _, a := range fixtureAgents() {
		if a.Name == name {
			agent := a
			agent.Type = "acp"
			agent.Tags = []string{"coding", "ai"}
			agent.Port = 8000
			return &agent
		}
	}
	return &acp.AgentConfig{Name: name, URL: "http://localhost:9999", Enabled: true}
}

func fixtureAgentTestResult(name string) acp.AgentTestResult {
	return acp.AgentTestResult{
		Name:       name,
		URL:        "http://localhost:8000",
		Enabled:    true,
		Status:     "connected",
		Version:    "1.0.0",
		AgentCount: 2,
		Agents:     []string{"codey", "helper"},
	}
}

func fixtureAgentLogs() []api.AgentLog {
	dur := 150
	errMsg := "timeout"
	return []api.AgentLog{
		{ID: 1, AgentName: "codey", Direction: "request", MessageType: "run", Timestamp: time.Now().Add(-10 * time.Minute)},
		{ID: 2, AgentName: "codey", Direction: "response", MessageType: "run", DurationMs: &dur, Timestamp: time.Now().Add(-9 * time.Minute)},
		{ID: 3, AgentName: "researcher", Direction: "request", MessageType: "run", Error: &errMsg, Timestamp: time.Now().Add(-5 * time.Minute)},
	}
}

func fixtureGallery() []acp.GalleryEntry {
	return []acp.GalleryEntry{
		{ID: "codey", Name: "Codey", Description: "Coding assistant by Google", Provider: "google", Category: "coding", Featured: true},
		{ID: "claude-code", Name: "Claude Code", Description: "Anthropic coding agent", Provider: "anthropic", Category: "coding", Featured: true},
		{ID: "research-bot", Name: "Research Bot", Description: "Web research tool", Provider: "openai", Category: "general", Featured: false},
	}
}

func fixtureInstallInfo(id string) acp.InstallInfo {
	return acp.InstallInfo{
		ID:          id,
		Name:        "Codey",
		Version:     "2.0.0",
		Description: "Coding assistant by Google",
		InstallType: "npx",
		InstallCmd:  "npx @google/codey",
		Available:   true,
	}
}

func fixtureContexts() []api.ContextInfo {
	return []api.ContextInfo{
		{ID: 1, Name: "personal", Description: "Personal tools", IsDefault: true},
		{ID: 2, Name: "work", Description: "Work tools", IsDefault: false},
		{ID: 3, Name: "testing", Description: "", IsDefault: false},
	}
}

func fixtureOAuthServers() []api.OAuthServerInfo {
	return []api.OAuthServerInfo{
		{Name: "linear", Provider: "Linear", Authenticated: true, Status: "authenticated"},
		{Name: "github-server", Provider: "GitHub", Authenticated: false, Status: "not authenticated"},
	}
}

func fixtureSlaves() []api.SlaveInfo {
	return []api.SlaveInfo{
		{Hostname: "linux-box", Status: "connected", ToolCount: 15, LastSeen: time.Now().Format(time.RFC3339), Enabled: true, Platform: "linux"},
		{Hostname: "pi-node", Status: "disconnected", ToolCount: 5, LastSeen: time.Now().Add(-1 * time.Hour).Format(time.RFC3339), Enabled: true, Platform: "linux"},
	}
}

func fixturePairingRequests() []api.PairingRequest {
	return []api.PairingRequest{
		{Hostname: "new-node", PairingCode: "123-456", Status: "pending", CreatedAt: time.Now().Format(time.RFC3339)},
	}
}

func fixtureRevokedCredentials() []api.RevokedCredentialInfo {
	return []api.RevokedCredentialInfo{
		{Hostname: "old-node", CertSerial: "ABCDEF123456", RevokedAt: time.Now().Add(-24 * time.Hour).Format(time.RFC3339), Reason: "decommissioned"},
	}
}

func fixtureJobs() []api.Job {
	return []api.Job{
		{ID: 1, Name: "nightly-backup", Schedule: "@daily", Enabled: true, Command: "backup.sh"},
		{ID: 2, Name: "cleanup", Schedule: "0 0 * * *", Enabled: false, Command: "rm -rf /tmp/*"},
	}
}

func fixtureJobLogs() []api.JobExecution {
	start := time.Now().Add(-1 * time.Hour)
	end := start.Add(5 * time.Minute)
	code := 0
	return []api.JobExecution{
		{ID: 1, JobID: 1, JobName: "nightly-backup", StartedAt: start, EndedAt: &end, ExitCode: &code, Stdout: "Backup completed"},
	}
}

func fixtureProviders() []api.ProviderResponse {
	return []api.ProviderResponse{
		{ID: 1, Name: "openai", Type: "llm", Service: "openai", Enabled: true, IsDefault: true},
		{ID: 2, Name: "local-embedding", Type: "embedding", Service: "ollama", Enabled: true},
	}
}

func fixtureProviderModels() []api.ModelInfo {
	return []api.ModelInfo{
		{ID: "gpt-4", Name: "GPT-4", DisplayName: "GPT-4"},
		{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", DisplayName: "GPT-3.5 Turbo"},
	}
}

func fixtureUsageSummary() api.UsageSummaryResponse {
	return api.UsageSummaryResponse{
		TotalCost: 1.234,
		Summary: []api.UsageSummaryRecord{
			{ProviderName: "openai", Service: "openai", Model: "gpt-4", TotalRequests: 100, TotalCost: 1.0},
			{ProviderName: "anthropic", Service: "anthropic", Model: "claude-3", TotalRequests: 50, TotalCost: 0.234},
		},
	}
}

// ---------------------------------------------------------------------------
// Mock API server
// ---------------------------------------------------------------------------

// newMockServer returns an httptest.Server that simulates the full Diane API.
// The optional overrides map lets individual tests replace specific route handlers.
func newMockServer(overrides map[string]http.HandlerFunc) *httptest.Server {
	mux := http.NewServeMux()

	// Default handlers
	defaults := map[string]http.HandlerFunc{
		"/health": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		},
		"/status": func(w http.ResponseWriter, r *http.Request) {
			jsonOK(w, fixtureStatus())
		},
		"/doctor": func(w http.ResponseWriter, r *http.Request) {
			jsonOK(w, fixtureDoctorHealthy())
		},
		"/mcp-servers": func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if strings.HasSuffix(path, "/restart") {
				// POST /mcp-servers/{name}/restart
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]string{"status": "restarted"})
				return
			}
			jsonOK(w, fixtureMCPServers())
		},
		"/mcp-servers/": func(w http.ResponseWriter, r *http.Request) {
			// Catch /mcp-servers/{name}/restart
			if strings.HasSuffix(r.URL.Path, "/restart") {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]string{"status": "restarted"})
				return
			}
			w.WriteHeader(http.StatusNotFound)
		},
		"/mcp-servers-config": func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				jsonOK(w, fixtureMCPConfigs())
			case http.MethodPost:
				var req api.CreateMCPServerRequest
				json.NewDecoder(r.Body).Decode(&req)
				resp := api.MCPServerResponse{
					ID:      99,
					Name:    req.Name,
					Type:    req.Type,
					Enabled: true,
					URL:     req.URL,
					Command: req.Command,
				}
				jsonStatus(w, http.StatusCreated, resp)
			}
		},
		"/mcp-servers-config/": func(w http.ResponseWriter, r *http.Request) {
			// PUT/DELETE /mcp-servers-config/{id}
			if r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusOK) // or NoContent
				return
			}
			if r.Method == http.MethodPut {
				jsonOK(w, api.MCPServerResponse{ID: 1, Name: "updated-srv", Enabled: true})
				return
			}
		},
		"/reload": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "reloaded"})
		},
		"/agents": func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				jsonOK(w, fixtureAgents())
			case http.MethodPost:
				jsonStatus(w, http.StatusCreated, map[string]string{"status": "created"})
			}
		},
		"/agents/": func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if strings.HasSuffix(path, "/toggle") {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]bool{"ok": true})
				return
			}
			if strings.HasSuffix(path, "/test") {
				name := strings.TrimSuffix(strings.TrimPrefix(path, "/agents/"), "/test")
				jsonOK(w, fixtureAgentTestResult(name))
				return
			}
			if strings.HasSuffix(path, "/run") {
				run := acp.Run{
					AgentName: "codey",
					RunID:     "run-001",
					Status:    acp.RunStatusCompleted,
					Output: []acp.Message{
						acp.NewTextMessage("agent", "Hello from the agent!"),
					},
				}
				jsonOK(w, run)
				return
			}
			if strings.HasPrefix(path, "/agents/logs") {
				jsonOK(w, fixtureAgentLogs())
				return
			}
			// GET /agents/{name}
			if r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
				return
			}
			name := strings.TrimPrefix(path, "/agents/")
			jsonOK(w, fixtureAgent(name))
		},
		"/gallery": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				// RefreshGallery
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]string{"status": "refreshed"})
				return
			}
			featured := r.URL.Query().Get("featured") == "true"
			entries := fixtureGallery()
			if featured {
				var filtered []acp.GalleryEntry
				for _, e := range entries {
					if e.Featured {
						filtered = append(filtered, e)
					}
				}
				entries = filtered
			}
			jsonOK(w, entries)
		},
		"/gallery/": func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if strings.HasSuffix(path, "/install") {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]string{"status": "installed"})
				return
			}
			id := strings.TrimPrefix(path, "/gallery/")
			jsonOK(w, fixtureInstallInfo(id))
		},
		"/contexts": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				var req map[string]string
				json.NewDecoder(r.Body).Decode(&req)
				jsonStatus(w, http.StatusCreated, api.ContextResponse{ID: 99, Name: req["name"], Description: req["description"]})
				return
			}
			jsonOK(w, fixtureContexts())
		},
		"/contexts/": func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if strings.HasSuffix(path, "/default") {
				jsonOK(w, map[string]string{"status": "ok"})
				return
			}
			if strings.HasSuffix(path, "/available-servers") {
				jsonOK(w, []api.AvailableServer{
					{Name: "srv1", ToolCount: 5, InContext: true},
					{Name: "srv2", ToolCount: 3, InContext: false},
				})
				return
			}
			if strings.HasSuffix(path, "/sync") {
				jsonOK(w, map[string]int{"tools_synced": 5})
				return
			}
			if r.Method == http.MethodDelete {
				jsonOK(w, map[string]string{"status": "deleted"})
				return
			}
			// Context Detail
			jsonOK(w, api.ContextDetailResponse{
				Context: api.ContextResponse{ID: 1, Name: "personal", IsDefault: true},
				Servers: []api.ContextServerResponse{
					{Name: "srv1", Enabled: true, ToolsActive: 5, ToolsTotal: 5},
				},
				Summary: api.ContextSummary{ServersEnabled: 1, ServersTotal: 1, ToolsActive: 5, ToolsTotal: 5},
			})
		},
		"/auth": func(w http.ResponseWriter, r *http.Request) {
			jsonOK(w, fixtureOAuthServers())
		},
		"/auth/": func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if strings.HasSuffix(path, "/logout") {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]string{"status": "logged_out"})
				return
			}
			// GET /auth/{server} - OAuth status
			jsonOK(w, map[string]interface{}{
				"authenticated": true,
				"provider":      "github",
				"token_type":    "Bearer",
			})
		},
		"/slaves": func(w http.ResponseWriter, r *http.Request) {
			jsonOK(w, fixtureSlaves())
		},
		"/slaves/": func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			switch {
			case strings.HasSuffix(path, "/pending"):
				jsonOK(w, fixturePairingRequests())
			case strings.HasSuffix(path, "/approve"):
				jsonOK(w, api.ApprovePairingResponse{Success: true, Message: "Approved"})
			case strings.HasSuffix(path, "/deny"):
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]string{"status": "denied"})
			case strings.HasSuffix(path, "/revoke"):
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]string{"status": "revoked"})
			case strings.HasSuffix(path, "/revoked"):
				jsonOK(w, fixtureRevokedCredentials())
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		},
		"/hosts": func(w http.ResponseWriter, r *http.Request) {
			jsonOK(w, []api.HostInfo{
				{ID: "master", Name: "Master", Type: "master", Online: true},
				{ID: "slave-1", Name: "linux-box", Type: "slave", Online: true},
			})
		},
		"/usage/summary": func(w http.ResponseWriter, r *http.Request) {
			jsonOK(w, fixtureUsageSummary())
		},
		"/providers": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				jsonStatus(w, http.StatusCreated, api.ProviderResponse{ID: 3, Name: "new-provider", Enabled: true})
				return
			}
			jsonOK(w, fixtureProviders())
		},
		"/providers/models": func(w http.ResponseWriter, r *http.Request) {
			jsonOK(w, api.ListModelsResponse{Models: fixtureProviderModels()})
		},
		"/providers/": func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if strings.HasSuffix(path, "/enable") {
				jsonOK(w, api.ProviderResponse{ID: 1, Name: "p1", Enabled: true})
				return
			}
			if strings.HasSuffix(path, "/disable") {
				jsonOK(w, api.ProviderResponse{ID: 1, Name: "p1", Enabled: false})
				return
			}
			if strings.HasSuffix(path, "/set-default") {
				jsonOK(w, api.ProviderResponse{ID: 1, Name: "p1", IsDefault: true})
				return
			}
			if strings.HasSuffix(path, "/test") {
				jsonOK(w, api.ProviderTestResult{Success: true, Message: "Connected", ResponseTime: 123})
				return
			}
			if r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			if r.Method == http.MethodPut {
				jsonOK(w, api.ProviderResponse{ID: 1, Name: "updated", Enabled: true})
				return
			}
		},
		"/jobs": func(w http.ResponseWriter, r *http.Request) {
			jsonOK(w, fixtureJobs())
		},
		"/jobs/logs": func(w http.ResponseWriter, r *http.Request) {
			jsonOK(w, fixtureJobLogs())
		},
		"/jobs/": func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/toggle") {
				w.WriteHeader(http.StatusOK) // toggle returns void/error
				return
			}
		},
	}

	// Apply defaults, then overrides
	for pattern, handler := range defaults {
		if override, ok := overrides[pattern]; ok {
			mux.HandleFunc(pattern, override)
		} else {
			mux.HandleFunc(pattern, handler)
		}
	}
	// Apply any overrides that aren't in defaults
	for pattern, handler := range overrides {
		if _, exists := defaults[pattern]; !exists {
			mux.HandleFunc(pattern, handler)
		}
	}

	return httptest.NewServer(mux)
}

// ---------------------------------------------------------------------------
// Tests: Version command
// ---------------------------------------------------------------------------

func TestVersionCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "v1.0.0-test") {
		t.Errorf("expected version output to contain 'v1.0.0-test', got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// Tests: Health command
// ---------------------------------------------------------------------------

func TestHealthCommand_Success(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "health")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "running") {
		t.Errorf("expected health output to contain 'running', got: %q", out)
	}
}

func TestHealthCommand_ServerDown(t *testing.T) {
	// Create a server and immediately close it to simulate "server down"
	ts := newMockServer(nil)
	ts.Close() // intentionally close

	client := newTestClient(ts)
	// Note: We don't call NewRootCmd here because the health command calls
	// os.Exit(1) on failure, which would kill the test process.
	// Instead, test that the client's Health() returns an error.
	if err := client.Health(); err == nil {
		t.Error("expected Health() to fail when server is down")
	}
}

// ---------------------------------------------------------------------------
// Tests: Status command
// ---------------------------------------------------------------------------

func TestStatusCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "status")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain server info
	if !strings.Contains(out, "Diane") {
		t.Errorf("expected status output to contain 'Diane', got: %q", out)
	}
	if !strings.Contains(out, "PID") {
		t.Errorf("expected status output to contain 'PID', got: %q", out)
	}
	if !strings.Contains(out, "filesystem") {
		t.Errorf("expected status output to contain 'filesystem' server, got: %q", out)
	}
	if !strings.Contains(out, "brave-search") {
		t.Errorf("expected status output to contain 'brave-search' server, got: %q", out)
	}
	if !strings.Contains(out, "connection refused") {
		t.Errorf("expected status output to contain error for broken-server, got: %q", out)
	}
	if !strings.Contains(out, "42") {
		t.Errorf("expected status output to contain total tools count '42', got: %q", out)
	}
}

func TestStatusCommand_JSON(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "status", "--json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be valid JSON
	var status api.Status
	if err := json.Unmarshal([]byte(out), &status); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\nOutput: %q", err, out)
	}
	if status.PID != 12345 {
		t.Errorf("expected PID=12345, got %d", status.PID)
	}
	if len(status.MCPServers) != 4 {
		t.Errorf("expected 4 MCP servers, got %d", len(status.MCPServers))
	}
}

func TestStatusCommand_SlaveMode(t *testing.T) {
	ts := newMockServer(map[string]http.HandlerFunc{
		"/status": func(w http.ResponseWriter, r *http.Request) {
			s := fixtureStatus()
			s.SlaveMode = true
			s.SlaveConnected = true
			s.MasterURL = "https://master:8766"
			jsonOK(w, s)
		},
	})
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "status")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "connected to") {
		t.Errorf("expected slave status output to mention 'connected to', got: %q", out)
	}
}

func TestStatusCommand_NoServers(t *testing.T) {
	ts := newMockServer(map[string]http.HandlerFunc{
		"/status": func(w http.ResponseWriter, r *http.Request) {
			s := fixtureStatus()
			s.MCPServers = nil
			s.TotalTools = 0
			jsonOK(w, s)
		},
	})
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "status")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "none configured") {
		t.Errorf("expected output to say 'none configured', got: %q", out)
	}
}

func TestStatusCommand_ServerDown(t *testing.T) {
	ts := newMockServer(map[string]http.HandlerFunc{
		"/status": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
	})
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "status")
	if err != nil {
		t.Fatalf("unexpected error: %v (status should handle errors gracefully)", err)
	}
	if !strings.Contains(out, "Could not reach") {
		t.Errorf("expected error message about not reaching daemon, got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// Tests: Doctor command
// ---------------------------------------------------------------------------

func TestDoctorCommand_AllPass(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "doctor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Doctor") {
		t.Errorf("expected output to contain 'Doctor', got: %q", out)
	}
	if !strings.Contains(out, "All checks passed") {
		t.Errorf("expected output to contain 'All checks passed', got: %q", out)
	}
	// Each check should appear
	for _, name := range []string{"Socket", "Database", "Config"} {
		if !strings.Contains(out, name) {
			t.Errorf("expected output to contain check '%s', got: %q", name, out)
		}
	}
}

func TestDoctorCommand_SomeFail(t *testing.T) {
	ts := newMockServer(map[string]http.HandlerFunc{
		"/doctor": func(w http.ResponseWriter, r *http.Request) {
			jsonOK(w, fixtureDoctorUnhealthy())
		},
	})
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "doctor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Issues found") {
		t.Errorf("expected output to contain 'Issues found', got: %q", out)
	}
	if !strings.Contains(out, "1 failed") {
		t.Errorf("expected output to contain '1 failed', got: %q", out)
	}
	if !strings.Contains(out, "1 warnings") {
		t.Errorf("expected output to contain '1 warnings', got: %q", out)
	}
}

func TestDoctorCommand_JSON(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "doctor", "--json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var report api.DoctorReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("expected valid JSON, got parse error: %v\nOutput: %q", err, out)
	}
	if !report.Healthy {
		t.Errorf("expected Healthy=true, got false")
	}
	if len(report.Checks) != 3 {
		t.Errorf("expected 3 checks, got %d", len(report.Checks))
	}
}

// ---------------------------------------------------------------------------
// Tests: MCP Servers command
// ---------------------------------------------------------------------------

func TestMCPServersCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "mcp-servers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "MCP Servers") {
		t.Errorf("expected output to contain 'MCP Servers', got: %q", out)
	}
	for _, name := range []string{"filesystem", "brave-search", "broken-server", "disabled-srv"} {
		if !strings.Contains(out, name) {
			t.Errorf("expected output to contain server '%s', got: %q", name, out)
		}
	}
	if !strings.Contains(out, "connection refused") {
		t.Errorf("expected output to contain error message 'connection refused', got: %q", out)
	}
}

func TestMCPServersCommand_JSON(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "mcp-servers", "--json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var servers []api.MCPServerStatus
	if err := json.Unmarshal([]byte(out), &servers); err != nil {
		t.Fatalf("expected valid JSON, got parse error: %v\nOutput: %q", err, out)
	}
	if len(servers) != 4 {
		t.Errorf("expected 4 servers, got %d", len(servers))
	}
}

func TestMCPServersCommand_Empty(t *testing.T) {
	ts := newMockServer(map[string]http.HandlerFunc{
		"/mcp-servers": func(w http.ResponseWriter, r *http.Request) {
			jsonOK(w, []api.MCPServerStatus{})
		},
	})
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "mcp-servers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No MCP servers") {
		t.Errorf("expected empty message, got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// Tests: MCP add / add-stdio commands
// ---------------------------------------------------------------------------

func TestMCPAddCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "mcp", "add", "my-server", "http://localhost:3000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Added") {
		t.Errorf("expected success message, got: %q", out)
	}
	if !strings.Contains(out, "my-server") {
		t.Errorf("expected output to contain server name 'my-server', got: %q", out)
	}
}

func TestMCPAddCommand_WithHeaders(t *testing.T) {
	var receivedReq api.CreateMCPServerRequest
	ts := newMockServer(map[string]http.HandlerFunc{
		"/mcp-servers-config": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				json.NewDecoder(r.Body).Decode(&receivedReq)
				jsonStatus(w, http.StatusCreated, api.MCPServerResponse{
					ID: 99, Name: receivedReq.Name, Type: receivedReq.Type, Enabled: true,
				})
			} else {
				jsonOK(w, fixtureMCPConfigs())
			}
		},
	})
	defer ts.Close()

	root := newTestRootCmd(ts)
	_, err := executeCmd(root, "mcp", "add", "secure-srv", "https://api.example.com",
		"--type", "sse",
		"--header", "Authorization=Bearer token123",
		"--header", "X-Custom=value")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedReq.Type != "sse" {
		t.Errorf("expected type 'sse', got: %q", receivedReq.Type)
	}
	if receivedReq.Headers["Authorization"] != "Bearer token123" {
		t.Errorf("expected Authorization header, got: %v", receivedReq.Headers)
	}
}

func TestMCPAddStdioCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "mcp", "add-stdio", "my-stdio-srv", "npx", "--arg", "-y", "--arg", "@modelcontextprotocol/server-filesystem")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Added stdio server") {
		t.Errorf("expected stdio success message, got: %q", out)
	}
}

func TestMCPAddCommand_MissingArgs(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	_, err := executeCmd(root, "mcp", "add", "only-name")
	if err == nil {
		t.Error("expected error for missing URL argument")
	}
}

func TestMCPEditCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "mcp", "edit", "1", "--name", "updated-srv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "updated") {
		t.Errorf("expected success message, got: %q", out)
	}
}

func TestMCPDeleteCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "mcp", "delete", "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "deleted") {
		t.Errorf("expected success message, got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// Tests: Reload / Restart commands
// ---------------------------------------------------------------------------

func TestReloadCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "reload")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "reloaded") {
		t.Errorf("expected reload success message, got: %q", out)
	}
}

func TestReloadCommand_Failure(t *testing.T) {
	ts := newMockServer(map[string]http.HandlerFunc{
		"/reload": func(w http.ResponseWriter, r *http.Request) {
			jsonStatus(w, http.StatusInternalServerError, map[string]string{"error": "config parse error"})
		},
	})
	defer ts.Close()

	root := newTestRootCmd(ts)
	_, err := executeCmd(root, "reload")
	if err == nil {
		t.Error("expected error for failed reload")
	}
}

func TestRestartCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "restart", "brave-search")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "restarted") {
		t.Errorf("expected restart success message, got: %q", out)
	}
	if !strings.Contains(out, "brave-search") {
		t.Errorf("expected server name in output, got: %q", out)
	}
}

func TestRestartCommand_MissingArg(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	_, err := executeCmd(root, "restart")
	if err == nil {
		t.Error("expected error for missing server name argument")
	}
}

// ---------------------------------------------------------------------------
// Tests: Agents command
// ---------------------------------------------------------------------------

func TestAgentsCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "agents")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "ACP Agents") {
		t.Errorf("expected title 'ACP Agents', got: %q", out)
	}
	for _, name := range []string{"codey", "researcher", "disabled-agent"} {
		if !strings.Contains(out, name) {
			t.Errorf("expected agent '%s' in output, got: %q", name, out)
		}
	}
}

func TestAgentsCommand_JSON(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "agents", "--json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var agents []acp.AgentConfig
	if err := json.Unmarshal([]byte(out), &agents); err != nil {
		t.Fatalf("expected valid JSON, got parse error: %v\nOutput: %q", err, out)
	}
	if len(agents) != 3 {
		t.Errorf("expected 3 agents, got %d", len(agents))
	}
}

func TestAgentsCommand_Empty(t *testing.T) {
	ts := newMockServer(map[string]http.HandlerFunc{
		"/agents": func(w http.ResponseWriter, r *http.Request) {
			jsonOK(w, []acp.AgentConfig{})
		},
	})
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "agents")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No agents configured") {
		t.Errorf("expected empty agents message, got: %q", out)
	}
}

func TestAgentAddCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "agent", "add", "new-agent", "http://localhost:9000", "My new agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "added") {
		t.Errorf("expected add success message, got: %q", out)
	}
}

func TestAgentAddCommand_MissingArgs(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	_, err := executeCmd(root, "agent", "add", "only-name")
	if err == nil {
		t.Error("expected error for missing URL argument")
	}
}

func TestAgentRemoveCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "agent", "remove", "codey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "removed") {
		t.Errorf("expected remove success message, got: %q", out)
	}
}

func TestAgentRemoveCommand_Alias(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "agent", "rm", "codey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "removed") {
		t.Errorf("expected remove success via alias, got: %q", out)
	}
}

func TestAgentEnableCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "agent", "enable", "disabled-agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "enabled") {
		t.Errorf("expected enable success message, got: %q", out)
	}
}

func TestAgentDisableCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "agent", "disable", "codey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "disabled") {
		t.Errorf("expected disable success message, got: %q", out)
	}
}

func TestAgentTestCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "agent", "test", "codey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Agent Test") {
		t.Errorf("expected 'Agent Test' title, got: %q", out)
	}
	if !strings.Contains(out, "connected") {
		t.Errorf("expected status 'connected', got: %q", out)
	}
	if !strings.Contains(out, "1.0.0") {
		t.Errorf("expected version '1.0.0', got: %q", out)
	}
}

func TestAgentTestCommand_JSON(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "agent", "test", "codey", "--json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result acp.AgentTestResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("expected valid JSON, got: %v\nOutput: %q", err, out)
	}
	if result.Status != "connected" {
		t.Errorf("expected connected status, got: %q", result.Status)
	}
}

func TestAgentInfoCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "agent", "info", "codey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Agent: codey") {
		t.Errorf("expected agent title, got: %q", out)
	}
	if !strings.Contains(out, "enabled") {
		t.Errorf("expected enabled status, got: %q", out)
	}
}

func TestAgentInfoCommand_JSON(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "agent", "info", "codey", "--json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var agent acp.AgentConfig
	if err := json.Unmarshal([]byte(out), &agent); err != nil {
		t.Fatalf("expected valid JSON, got: %v\nOutput: %q", err, out)
	}
	if agent.Name != "codey" {
		t.Errorf("expected agent name 'codey', got: %q", agent.Name)
	}
}

func TestAgentLogsCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "agent", "logs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Agent Logs") {
		t.Errorf("expected 'Agent Logs' title, got: %q", out)
	}
	if !strings.Contains(out, "codey") {
		t.Errorf("expected agent name 'codey' in logs, got: %q", out)
	}
}

func TestAgentLogsCommand_Empty(t *testing.T) {
	ts := newMockServer(map[string]http.HandlerFunc{
		"/agents/": func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/agents/logs") {
				jsonOK(w, []api.AgentLog{})
				return
			}
		},
	})
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "agent", "logs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No agent logs") {
		t.Errorf("expected empty logs message, got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// Tests: Gallery command
// ---------------------------------------------------------------------------

func TestGalleryCommand_DefaultsList(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "gallery")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Agent Gallery") {
		t.Errorf("expected 'Agent Gallery' title, got: %q", out)
	}
	for _, name := range []string{"codey", "claude-code", "research-bot"} {
		if !strings.Contains(out, name) {
			t.Errorf("expected gallery entry '%s', got: %q", name, out)
		}
	}
}

func TestGalleryListCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "gallery", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Agent Gallery") {
		t.Errorf("expected gallery title, got: %q", out)
	}
}

func TestGalleryFeaturedCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "gallery", "featured")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Featured Agents") {
		t.Errorf("expected 'Featured Agents' title, got: %q", out)
	}
	// research-bot is not featured, should NOT appear
	if strings.Contains(out, "research-bot") {
		t.Errorf("expected 'research-bot' to NOT appear in featured list, got: %q", out)
	}
}

func TestGalleryInfoCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "gallery", "info", "codey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Gallery Agent") {
		t.Errorf("expected gallery info title, got: %q", out)
	}
	if !strings.Contains(out, "2.0.0") {
		t.Errorf("expected version '2.0.0', got: %q", out)
	}
	if !strings.Contains(out, "npx") {
		t.Errorf("expected install type 'npx', got: %q", out)
	}
	if !strings.Contains(out, "Available") {
		t.Errorf("expected 'Available' status, got: %q", out)
	}
}

func TestGalleryInfoCommand_JSON(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "gallery", "info", "codey", "--json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var info acp.InstallInfo
	if err := json.Unmarshal([]byte(out), &info); err != nil {
		t.Fatalf("expected valid JSON, got: %v\nOutput: %q", err, out)
	}
	if info.ID != "codey" {
		t.Errorf("expected id 'codey', got: %q", info.ID)
	}
}

func TestGalleryInstallCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "gallery", "install", "codey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "configured") {
		t.Errorf("expected install success message, got: %q", out)
	}
	if !strings.Contains(out, "npx @google/codey") {
		t.Errorf("expected install command hint, got: %q", out)
	}
}

func TestGalleryInstallCommand_WithOptions(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "gallery", "install", "codey", "--name", "my-codey", "--port", "9000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "my-codey") {
		t.Errorf("expected custom agent name 'my-codey', got: %q", out)
	}
}

func TestGalleryRefreshCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "gallery", "refresh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "refreshed") {
		t.Errorf("expected refresh success message, got: %q", out)
	}
}

func TestGalleryCommand_Empty(t *testing.T) {
	ts := newMockServer(map[string]http.HandlerFunc{
		"/gallery": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				jsonOK(w, []acp.GalleryEntry{})
			}
		},
	})
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "gallery", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No gallery entries") {
		t.Errorf("expected empty gallery message, got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// Tests: Context command
// ---------------------------------------------------------------------------

func TestContextCommand_DefaultsList(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "context")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Contexts") {
		t.Errorf("expected 'Contexts' title, got: %q", out)
	}
	for _, name := range []string{"personal", "work", "testing"} {
		if !strings.Contains(out, name) {
			t.Errorf("expected context '%s' in output, got: %q", name, out)
		}
	}
}

func TestContextListCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "context", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "personal") {
		t.Errorf("expected 'personal' context, got: %q", out)
	}
}

func TestContextListCommand_JSON(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "context", "list", "--json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var contexts []api.ContextInfo
	if err := json.Unmarshal([]byte(out), &contexts); err != nil {
		t.Fatalf("expected valid JSON, got: %v\nOutput: %q", err, out)
	}
	if len(contexts) != 3 {
		t.Errorf("expected 3 contexts, got %d", len(contexts))
	}
}

func TestContextListCommand_Empty(t *testing.T) {
	ts := newMockServer(map[string]http.HandlerFunc{
		"/contexts": func(w http.ResponseWriter, r *http.Request) {
			jsonOK(w, []api.ContextInfo{})
		},
	})
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "context", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No contexts") {
		t.Errorf("expected empty contexts message, got: %q", out)
	}
}

func TestContextCreateCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "context", "create", "new-ctx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "created") {
		t.Errorf("expected success message, got: %q", out)
	}
}

func TestContextDeleteCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "context", "delete", "old-ctx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "deleted") {
		t.Errorf("expected success message, got: %q", out)
	}
}

func TestContextSetDefaultCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "context", "set-default", "work")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "default") {
		t.Errorf("expected success message, got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// Tests: Auth command
// ---------------------------------------------------------------------------

func TestAuthCommand_DefaultsList(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "auth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "OAuth Servers") {
		t.Errorf("expected 'OAuth Servers' title, got: %q", out)
	}
	if !strings.Contains(out, "linear") {
		t.Errorf("expected 'linear' server, got: %q", out)
	}
	if !strings.Contains(out, "github-server") {
		t.Errorf("expected 'github-server', got: %q", out)
	}
}

func TestAuthCommand_JSON(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "auth", "--json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var servers []api.OAuthServerInfo
	if err := json.Unmarshal([]byte(out), &servers); err != nil {
		t.Fatalf("expected valid JSON, got: %v\nOutput: %q", err, out)
	}
	if len(servers) != 2 {
		t.Errorf("expected 2 OAuth servers, got %d", len(servers))
	}
}

func TestAuthCommand_Empty(t *testing.T) {
	ts := newMockServer(map[string]http.HandlerFunc{
		"/auth": func(w http.ResponseWriter, r *http.Request) {
			jsonOK(w, []api.OAuthServerInfo{})
		},
	})
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "auth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No OAuth") {
		t.Errorf("expected empty OAuth message, got: %q", out)
	}
}

func TestAuthStatusCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "auth", "status", "linear")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// auth status outputs raw JSON
	if !strings.Contains(out, "authenticated") {
		t.Errorf("expected 'authenticated' in output, got: %q", out)
	}
}

func TestAuthLogoutCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "auth", "logout", "linear")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Logged out") {
		t.Errorf("expected logout success message, got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// Tests: Tools / Prompts / Resources commands
// ---------------------------------------------------------------------------

func TestToolsCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "tools")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Tools") {
		t.Errorf("expected 'Tools' title, got: %q", out)
	}
	if !strings.Contains(out, "filesystem") {
		t.Errorf("expected 'filesystem' server, got: %q", out)
	}
}

func TestToolsCommand_ServerFilter(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "tools", "--server", "filesystem")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "filesystem") {
		t.Errorf("expected filtered server, got: %q", out)
	}
	// Other servers should not appear (brave-search etc.)
	if strings.Contains(out, "brave-search") {
		t.Errorf("expected 'brave-search' to be filtered out, got: %q", out)
	}
}

func TestToolsCommand_NoMatch(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "tools", "--server", "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No server found") {
		t.Errorf("expected no-match message, got: %q", out)
	}
}

func TestPromptsCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "prompts")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Prompts") {
		t.Errorf("expected 'Prompts' title, got: %q", out)
	}
}

func TestPromptsCommand_None(t *testing.T) {
	ts := newMockServer(map[string]http.HandlerFunc{
		"/mcp-servers": func(w http.ResponseWriter, r *http.Request) {
			servers := []api.MCPServerStatus{
				{Name: "srv1", Enabled: true, Connected: true, ToolCount: 5},
			}
			jsonOK(w, servers)
		},
	})
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "prompts")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No prompts") {
		t.Errorf("expected no-prompts message, got: %q", out)
	}
}

func TestResourcesCommand_None(t *testing.T) {
	ts := newMockServer(map[string]http.HandlerFunc{
		"/mcp-servers": func(w http.ResponseWriter, r *http.Request) {
			servers := []api.MCPServerStatus{
				{Name: "srv1", Enabled: true, Connected: true, ToolCount: 5},
			}
			jsonOK(w, servers)
		},
	})
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "resources")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No resources") {
		t.Errorf("expected no-resources message, got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// Tests: Hosts / Usage / Provider / Jobs
// ---------------------------------------------------------------------------

func TestHostsCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "hosts")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Connected Hosts") {
		t.Errorf("expected title 'Connected Hosts', got: %q", out)
	}
	if !strings.Contains(out, "master") {
		t.Errorf("expected 'master' host, got: %q", out)
	}
	if !strings.Contains(out, "slave-1") {
		t.Errorf("expected 'slave-1' host, got: %q", out)
	}
}

func TestUsageCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "usage")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Usage Summary") {
		t.Errorf("expected 'Usage Summary' title, got: %q", out)
	}
	if !strings.Contains(out, "openai") {
		t.Errorf("expected 'openai' provider, got: %q", out)
	}
	if !strings.Contains(out, "1.234") {
		t.Errorf("expected total cost '1.234', got: %q", out)
	}
}

func TestProviderCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "provider")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Default is list
	if !strings.Contains(out, "Providers") {
		t.Errorf("expected 'Providers' title, got: %q", out)
	}
}

func TestProviderListCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "provider", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Providers") {
		t.Errorf("expected 'Providers' title, got: %q", out)
	}
	if !strings.Contains(out, "openai") {
		t.Errorf("expected 'openai' provider, got: %q", out)
	}
}

func TestProviderModelsCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "provider", "models", "--service", "openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Available Models") {
		t.Errorf("expected 'Available Models' title, got: %q", out)
	}
	if !strings.Contains(out, "GPT-4") {
		t.Errorf("expected 'GPT-4' model, got: %q", out)
	}
}

func TestJobsCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "jobs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Default is list
	if !strings.Contains(out, "Scheduled Jobs") {
		t.Errorf("expected 'Scheduled Jobs' title, got: %q", out)
	}
}

func TestJobsListCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "jobs", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Scheduled Jobs") {
		t.Errorf("expected 'Scheduled Jobs' title, got: %q", out)
	}
	if !strings.Contains(out, "nightly-backup") {
		t.Errorf("expected 'nightly-backup' job, got: %q", out)
	}
}

func TestJobsLogsCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "jobs", "logs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Job Logs") {
		t.Errorf("expected 'Job Logs' title, got: %q", out)
	}
	if !strings.Contains(out, "Backup completed") {
		t.Errorf("expected log output, got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// Tests: Slave commands (master-side)
// ---------------------------------------------------------------------------

func TestSlaveListCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "slave", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "linux-box") {
		t.Errorf("expected slave hostname 'linux-box', got: %q", out)
	}
	if !strings.Contains(out, "pi-node") {
		t.Errorf("expected slave hostname 'pi-node', got: %q", out)
	}
}

func TestSlaveListCommand_JSON(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "slave", "list", "--json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var slaves []api.SlaveInfo
	if err := json.Unmarshal([]byte(out), &slaves); err != nil {
		t.Fatalf("expected valid JSON, got: %v\nOutput: %q", err, out)
	}
	if len(slaves) != 2 {
		t.Errorf("expected 2 slaves, got %d", len(slaves))
	}
}

func TestSlaveListCommand_Empty(t *testing.T) {
	ts := newMockServer(map[string]http.HandlerFunc{
		"/slaves": func(w http.ResponseWriter, r *http.Request) {
			jsonOK(w, []api.SlaveInfo{})
		},
	})
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "slave", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No slaves") {
		t.Errorf("expected empty slaves message, got: %q", out)
	}
}

func TestSlavePendingCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "slave", "pending")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "new-node") {
		t.Errorf("expected pending request hostname 'new-node', got: %q", out)
	}
	if !strings.Contains(out, "123-456") {
		t.Errorf("expected pairing code '123-456', got: %q", out)
	}
}

func TestSlavePendingCommand_Empty(t *testing.T) {
	ts := newMockServer(map[string]http.HandlerFunc{
		"/slaves/": func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/pending") {
				jsonOK(w, []api.PairingRequest{})
				return
			}
		},
	})
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "slave", "pending")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No pending") {
		t.Errorf("expected no-pending message, got: %q", out)
	}
}

func TestSlaveApproveCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "slave", "approve", "new-node", "123-456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "approved") {
		t.Errorf("expected approval success message, got: %q", out)
	}
}

func TestSlaveDenyCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "slave", "deny", "new-node", "123-456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "denied") {
		t.Errorf("expected deny success message, got: %q", out)
	}
}

func TestSlaveRevokeCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "slave", "revoke", "linux-box")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "revoked") {
		t.Errorf("expected revoke success message, got: %q", out)
	}
}

func TestSlaveRevokedCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "slave", "revoked")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "old-node") {
		t.Errorf("expected revoked hostname 'old-node', got: %q", out)
	}
	if !strings.Contains(out, "ABCDEF123456") {
		t.Errorf("expected cert serial, got: %q", out)
	}
}

func TestSlaveRevokedCommand_Empty(t *testing.T) {
	ts := newMockServer(map[string]http.HandlerFunc{
		"/slaves/": func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/revoked") {
				jsonOK(w, []api.RevokedCredentialInfo{})
				return
			}
		},
	})
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "slave", "revoked")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No revoked") {
		t.Errorf("expected empty revoked message, got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// Tests: Info command
// ---------------------------------------------------------------------------

func TestInfoCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "info")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "DIANE MCP SERVER") {
		t.Errorf("expected info header, got: %q", out)
	}
	if !strings.Contains(out, "CONNECTING TO DIANE") {
		t.Errorf("expected connecting section, got: %q", out)
	}
	if !strings.Contains(out, "OpenCode") {
		t.Errorf("expected OpenCode section, got: %q", out)
	}
	if !strings.Contains(out, "Claude Desktop") {
		t.Errorf("expected Claude Desktop section, got: %q", out)
	}
	if !strings.Contains(out, "localhost:8765") {
		t.Errorf("expected HTTP endpoint URL, got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// Tests: Logs command
// ---------------------------------------------------------------------------

func TestLogsCommand(t *testing.T) {
	// Create a temp log file
	tmpDir := t.TempDir()
	dianeDir := tmpDir + "/.diane"
	os.MkdirAll(dianeDir, 0755)
	logPath := dianeDir + "/server.log"

	var lines []string
	for i := 1; i <= 200; i++ {
		lines = append(lines, fmt.Sprintf("log line %d", i))
	}
	os.WriteFile(logPath, []byte(strings.Join(lines, "\n")+"\n"), 0644)

	// Override HOME so the logs command finds our temp file
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "logs", "-n", "10")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should show last 10 lines
	if !strings.Contains(out, "log line 200") {
		t.Errorf("expected last line 'log line 200', got: %q", out)
	}
	if !strings.Contains(out, "log line 191") {
		t.Errorf("expected line 191 in last 10, got: %q", out)
	}
	// Should NOT contain earlier lines
	if strings.Contains(out, "log line 100\n") {
		t.Errorf("expected line 100 to NOT appear, got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// Tests: Global flags
// ---------------------------------------------------------------------------

func TestGlobalFlag_NoColor(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	// --no-color should not cause an error
	out, err := executeCmd(root, "version", "--no-color")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "v1.0.0-test") {
		t.Errorf("expected version with --no-color, got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// Tests: Error handling
// ---------------------------------------------------------------------------

func TestUnknownCommand(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	_, err := executeCmd(root, "nonexistent-command")
	if err == nil {
		t.Error("expected error for unknown command")
	}
}

func TestAPIError_500(t *testing.T) {
	ts := newMockServer(map[string]http.HandlerFunc{
		"/agents": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
	})
	defer ts.Close()

	root := newTestRootCmd(ts)
	_, err := executeCmd(root, "agents")
	if err == nil {
		t.Error("expected error for 500 response on agents list")
	}
}

func TestAPIError_MCPServers(t *testing.T) {
	ts := newMockServer(map[string]http.HandlerFunc{
		"/mcp-servers": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
	})
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "mcp-servers")
	// mcp-servers handles errors gracefully (prints error, returns nil)
	if err != nil {
		t.Fatalf("mcp-servers should handle errors gracefully: %v", err)
	}
	if !strings.Contains(out, "Failed") {
		t.Errorf("expected error message in output, got: %q", out)
	}
}

func TestAPIError_AgentRemove_404(t *testing.T) {
	ts := newMockServer(map[string]http.HandlerFunc{
		"/agents/": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodDelete {
				jsonStatus(w, http.StatusNotFound, map[string]string{"error": "agent not found"})
				return
			}
		},
	})
	defer ts.Close()

	root := newTestRootCmd(ts)
	_, err := executeCmd(root, "agent", "remove", "nonexistent")
	if err == nil {
		t.Error("expected error for removing nonexistent agent")
	}
}

func TestAPIError_CreateMCPServer_Conflict(t *testing.T) {
	ts := newMockServer(map[string]http.HandlerFunc{
		"/mcp-servers-config": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				jsonStatus(w, http.StatusConflict, map[string]string{"error": "already exists"})
				return
			}
			jsonOK(w, fixtureMCPConfigs())
		},
	})
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "mcp", "add", "existing-srv", "http://localhost:3000")
	if err != nil {
		t.Fatalf("mcp add should handle errors gracefully: %v", err)
	}
	if !strings.Contains(out, "Failed") || !strings.Contains(out, "already exists") {
		t.Errorf("expected conflict error message, got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// Tests: Cobra argument validation
// ---------------------------------------------------------------------------

func TestArgValidation_RestartRequiresName(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	_, err := executeCmd(root, "restart")
	if err == nil {
		t.Error("expected error: restart requires exactly 1 arg")
	}
}

func TestArgValidation_AgentInfoRequiresName(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	_, err := executeCmd(root, "agent", "info")
	if err == nil {
		t.Error("expected error: agent info requires exactly 1 arg")
	}
}

func TestArgValidation_AgentTestRequiresName(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	_, err := executeCmd(root, "agent", "test")
	if err == nil {
		t.Error("expected error: agent test requires exactly 1 arg")
	}
}

func TestArgValidation_MCPAddRequiresNameAndURL(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	_, err := executeCmd(root, "mcp", "add")
	if err == nil {
		t.Error("expected error: mcp add requires exactly 2 args")
	}
}

func TestArgValidation_SlaveDenyRequiresHostAndCode(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	_, err := executeCmd(root, "slave", "deny", "only-host")
	if err == nil {
		t.Error("expected error: slave deny requires at least 2 args")
	}
}

func TestArgValidation_GalleryInfoRequiresID(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	_, err := executeCmd(root, "gallery", "info")
	if err == nil {
		t.Error("expected error: gallery info requires exactly 1 arg")
	}
}

func TestArgValidation_SlaveRevokeRequiresHostname(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	_, err := executeCmd(root, "slave", "revoke")
	if err == nil {
		t.Error("expected error: slave revoke requires exactly 1 arg")
	}
}

// ---------------------------------------------------------------------------
// Tests: Helper functions
// ---------------------------------------------------------------------------

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m 30s"},
		{3600 * time.Second, "1h 0m"},
		{90 * time.Minute, "1h 30m"},
		{25 * time.Hour, "1d 1h"},
		{49*time.Hour + 30*time.Minute, "2d 1h"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := formatDuration(tt.input)
			if got != tt.expected {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNormalizePairingCode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"123-456", "123-456"},
		{"123 456", "123-456"},
		{"123456", "123-456"},
		{"abc-def", "abc-def"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizePairingCode(tt.input)
			if got != tt.expected {
				t.Errorf("normalizePairingCode(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestGetTypeBadge(t *testing.T) {
	// Just ensure they don't panic and return non-empty strings
	types := []string{"http", "sse", "stdio", "builtin", "unknown"}
	for _, tp := range types {
		badge := GetTypeBadge(tp)
		if badge == "" {
			t.Errorf("GetTypeBadge(%q) returned empty string", tp)
		}
	}
}

func TestGetStatusDot(t *testing.T) {
	tests := []struct {
		connected bool
		hasError  bool
	}{
		{true, false},
		{false, false},
		{false, true},
		{true, true},
	}
	for _, tt := range tests {
		dot := GetStatusDot(tt.connected, tt.hasError)
		if dot == "" {
			t.Errorf("GetStatusDot(%v, %v) returned empty string", tt.connected, tt.hasError)
		}
	}
}

func TestTryJSON_Enabled(t *testing.T) {
	root := &cobra.Command{Use: "test"}
	root.PersistentFlags().Bool("json", false, "JSON output")

	child := &cobra.Command{
		Use: "child",
		RunE: func(cmd *cobra.Command, args []string) error {
			data := map[string]string{"key": "value"}
			if tryJSON(cmd, data) {
				return nil
			}
			fmt.Println("not json")
			return nil
		},
	}
	root.AddCommand(child)

	out := captureStdout(func() {
		root.SetArgs([]string{"child", "--json"})
		root.Execute()
	})

	if !strings.Contains(out, `"key"`) || !strings.Contains(out, `"value"`) {
		t.Errorf("expected JSON output with key/value, got: %q", out)
	}
	if strings.Contains(out, "not json") {
		t.Errorf("expected tryJSON to short-circuit, but 'not json' appeared")
	}
}

func TestTryJSON_Disabled(t *testing.T) {
	root := &cobra.Command{Use: "test"}
	root.PersistentFlags().Bool("json", false, "JSON output")

	child := &cobra.Command{
		Use: "child",
		RunE: func(cmd *cobra.Command, args []string) error {
			data := map[string]string{"key": "value"}
			if tryJSON(cmd, data) {
				return nil
			}
			fmt.Println("not json")
			return nil
		},
	}
	root.AddCommand(child)

	out := captureStdout(func() {
		root.SetArgs([]string{"child"})
		root.Execute()
	})

	if !strings.Contains(out, "not json") {
		t.Errorf("expected normal output 'not json', got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// Tests: Help output
// ---------------------------------------------------------------------------

func TestHelpOutput_Root(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should list all subcommands
	for _, cmd := range []string{"status", "health", "doctor", "agents", "agent",
		"mcp-servers", "mcp", "gallery", "context", "auth", "tools", "prompts",
		"resources", "hosts", "usage", "provider", "jobs", "info", "logs",
		"slave", "upgrade", "version", "reload", "restart"} {
		if !strings.Contains(out, cmd) {
			t.Errorf("expected help to list '%s' command, got: %q", cmd, out)
		}
	}
}

func TestHelpOutput_AgentSubcommands(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "agent", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, sub := range []string{"add", "remove", "enable", "disable", "test", "run", "info", "logs"} {
		if !strings.Contains(out, sub) {
			t.Errorf("expected agent help to list '%s', got: %q", sub, out)
		}
	}
}

func TestHelpOutput_MCPSubcommands(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "mcp", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, sub := range []string{"add", "add-stdio", "edit", "delete", "install"} {
		if !strings.Contains(out, sub) {
			t.Errorf("expected mcp help to list '%s', got: %q", sub, out)
		}
	}
}

func TestHelpOutput_GallerySubcommands(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "gallery", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, sub := range []string{"list", "featured", "info", "install", "refresh"} {
		if !strings.Contains(out, sub) {
			t.Errorf("expected gallery help to list '%s', got: %q", sub, out)
		}
	}
}

func TestHelpOutput_SlaveSubcommands(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "slave", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, sub := range []string{"pair", "start", "pending", "approve", "deny", "list", "revoke", "revoked"} {
		if !strings.Contains(out, sub) {
			t.Errorf("expected slave help to list '%s', got: %q", sub, out)
		}
	}
}

func TestHelpOutput_ContextSubcommands(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "context", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, sub := range []string{"list", "create", "delete", "set-default", "info", "servers", "tools"} {
		if !strings.Contains(out, sub) {
			t.Errorf("expected context help to list '%s', got: %q", sub, out)
		}
	}
}

func TestHelpOutput_ProviderSubcommands(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "provider", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, sub := range []string{"list", "create", "edit", "delete", "test", "enable", "disable", "set-default", "models"} {
		if !strings.Contains(out, sub) {
			t.Errorf("expected provider help to list '%s', got: %q", sub, out)
		}
	}
}

func TestHelpOutput_JobsSubcommands(t *testing.T) {
	ts := newMockServer(nil)
	defer ts.Close()

	root := newTestRootCmd(ts)
	out, err := executeCmd(root, "jobs", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, sub := range []string{"list", "logs", "enable", "disable"} {
		if !strings.Contains(out, sub) {
			t.Errorf("expected jobs help to list '%s', got: %q", sub, out)
		}
	}
}
