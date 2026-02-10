// Package acp provides configuration and management for ACP agents.
package acp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// AgentConfig represents a configured ACP agent
type AgentConfig struct {
	Name        string            `json:"name"`
	URL         string            `json:"url"`
	Type        string            `json:"type,omitempty"`      // "acp" (default), "stdio" for local agents
	Command     string            `json:"command,omitempty"`   // For stdio agents
	Args        []string          `json:"args,omitempty"`      // For stdio agents
	Env         map[string]string `json:"env,omitempty"`       // Environment variables
	WorkDir     string            `json:"workdir,omitempty"`   // Working directory/project path for the agent
	Port        int               `json:"port,omitempty"`      // Port for ACP server (auto-assigned if 0)
	SubAgent    string            `json:"sub_agent,omitempty"` // Sub-agent name to use (for servers with multiple agents)
	Enabled     bool              `json:"enabled"`
	Description string            `json:"description,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
}

// UniqueKey returns a unique identifier for this agent instance (name + workdir)
func (c *AgentConfig) UniqueKey() string {
	if c.WorkDir == "" {
		return c.Name
	}
	return fmt.Sprintf("%s@%s", c.Name, c.WorkDir)
}

// Config holds the full ACP configuration
type Config struct {
	Agents []AgentConfig `json:"agents"`
}

// Manager handles ACP agent configuration and lifecycle
type Manager struct {
	mu           sync.RWMutex
	config       *Config
	configPath   string
	clients      map[string]*Client      // Legacy HTTP clients (deprecated)
	stdioClients map[string]*StdioClient // ACP stdio clients
}

// NewManager creates a new ACP manager
func NewManager() (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(home, ".diane", "acp-agents.json")
	m := &Manager{
		configPath:   configPath,
		clients:      make(map[string]*Client),
		stdioClients: make(map[string]*StdioClient),
	}

	if err := m.loadConfig(); err != nil {
		// Initialize with empty config if file doesn't exist
		m.config = &Config{Agents: []AgentConfig{}}
	}

	return m, nil
}

// loadConfig loads the configuration from disk
func (m *Manager) loadConfig() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	m.config = &config
	return nil
}

// saveConfig saves the configuration to disk
func (m *Manager) saveConfig() error {
	// Ensure directory exists
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// AddAgent adds a new ACP agent
func (m *Manager) AddAgent(agent AgentConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for duplicate name
	for _, a := range m.config.Agents {
		if a.Name == agent.Name {
			return fmt.Errorf("agent '%s' already exists", agent.Name)
		}
	}

	// Set defaults
	if agent.Type == "" {
		agent.Type = "acp"
	}

	m.config.Agents = append(m.config.Agents, agent)
	return m.saveConfig()
}

// RemoveAgent removes an ACP agent by name
func (m *Manager) RemoveAgent(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, a := range m.config.Agents {
		if a.Name == name {
			// Remove HTTP client if exists
			if client, ok := m.clients[name]; ok {
				_ = client // No cleanup needed for HTTP client
				delete(m.clients, name)
			}
			// Remove stdio client if exists
			if client, ok := m.stdioClients[name]; ok {
				client.Close()
				delete(m.stdioClients, name)
			}

			m.config.Agents = append(m.config.Agents[:i], m.config.Agents[i+1:]...)
			return m.saveConfig()
		}
	}

	return fmt.Errorf("agent '%s' not found", name)
}

// GetAgent returns an agent by name
func (m *Manager) GetAgent(name string) (*AgentConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, a := range m.config.Agents {
		if a.Name == name {
			return &a, nil
		}
	}

	return nil, fmt.Errorf("agent '%s' not found", name)
}

// ListAgents returns all configured agents
func (m *Manager) ListAgents() []AgentConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]AgentConfig, len(m.config.Agents))
	copy(result, m.config.Agents)
	return result
}

// EnableAgent enables or disables an agent
func (m *Manager) EnableAgent(name string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, a := range m.config.Agents {
		if a.Name == name {
			m.config.Agents[i].Enabled = enabled
			return m.saveConfig()
		}
	}

	return fmt.Errorf("agent '%s' not found", name)
}

// UpdateAgent updates an agent's configuration (partial update)
func (m *Manager) UpdateAgent(name string, updates map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, a := range m.config.Agents {
		if a.Name == name {
			// Apply updates
			if subAgent, ok := updates["sub_agent"].(string); ok {
				m.config.Agents[i].SubAgent = subAgent
			}
			if enabled, ok := updates["enabled"].(bool); ok {
				m.config.Agents[i].Enabled = enabled
			}
			if description, ok := updates["description"].(string); ok {
				m.config.Agents[i].Description = description
			}
			if workDir, ok := updates["workdir"].(string); ok {
				m.config.Agents[i].WorkDir = workDir
			}
			return m.saveConfig()
		}
	}

	return fmt.Errorf("agent '%s' not found", name)
}

// GetClient returns an ACP client for the specified agent
func (m *Manager) GetClient(name string) (*Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Return cached client if exists
	if client, ok := m.clients[name]; ok {
		return client, nil
	}

	// Find agent config
	var agentConfig *AgentConfig
	for _, a := range m.config.Agents {
		if a.Name == name {
			agentConfig = &a
			break
		}
	}

	if agentConfig == nil {
		return nil, fmt.Errorf("agent '%s' not found", name)
	}

	if !agentConfig.Enabled {
		return nil, fmt.Errorf("agent '%s' is disabled", name)
	}

	if agentConfig.Type != "acp" && agentConfig.Type != "" {
		return nil, fmt.Errorf("agent '%s' is not an ACP agent (type: %s)", name, agentConfig.Type)
	}

	client := NewClient(agentConfig.URL)
	m.clients[name] = client

	return client, nil
}

// TestAgent tests connectivity to an agent
func (m *Manager) TestAgent(name string) (*AgentTestResult, error) {
	agent, err := m.GetAgent(name)
	if err != nil {
		return nil, err
	}

	result := &AgentTestResult{
		Name:    name,
		URL:     agent.URL,
		WorkDir: agent.WorkDir,
		Enabled: agent.Enabled,
	}

	if !agent.Enabled {
		result.Status = "disabled"
		return result, nil
	}

	// Handle stdio agents (simple command execution agents)
	if agent.Type == "stdio" {
		return m.testSimpleStdioAgent(agent, result)
	}

	// Handle ACP agents (proper ACP protocol over stdio)
	if agent.Type == "acp" || agent.Type == "" {
		return m.testACPAgent(agent, result)
	}

	result.Status = "error"
	result.Error = fmt.Sprintf("unknown agent type: %s", agent.Type)
	return result, nil
}

// testACPAgent tests an ACP agent by initializing a connection
func (m *Manager) testACPAgent(agent *AgentConfig, result *AgentTestResult) (*AgentTestResult, error) {
	// ACP agents use the stdio transport with JSON-RPC
	// We need to spawn the agent process and do the initialize handshake

	// Determine command and args
	command := agent.Command
	args := agent.Args

	// If no command specified, try to infer from name
	if command == "" {
		// For gallery agents like "opencode", the command is the name with "acp" arg
		command = agent.Name
		args = []string{"acp"}
	}

	// Check if command exists
	cmdPath, err := exec.LookPath(command)
	if err != nil {
		result.Status = "unreachable"
		result.Error = fmt.Sprintf("command not found: %s", command)
		return result, nil
	}

	// Create a temporary stdio client to test connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := NewStdioClient(cmdPath, args, agent.WorkDir, agent.Env)
	if err != nil {
		result.Status = "unreachable"
		result.Error = fmt.Sprintf("failed to start agent: %v", err)
		return result, nil
	}
	defer client.Close()

	// Try to initialize
	if err := client.Initialize(ctx); err != nil {
		result.Status = "error"
		result.Error = fmt.Sprintf("initialization failed: %v", err)
		return result, nil
	}

	result.Status = "connected"
	result.AgentCount = 1
	result.Agents = []string{agent.Name}

	// Add version info from agent
	if info := client.GetAgentInfo(); info != nil {
		result.Version = fmt.Sprintf("%s %s", info.Name, info.Version)
	}

	return result, nil
}

// testSimpleStdioAgent tests a simple stdio-based agent by verifying the command exists
// and optionally running a quick version/help check
func (m *Manager) testSimpleStdioAgent(agent *AgentConfig, result *AgentTestResult) (*AgentTestResult, error) {
	// Check if command exists
	cmdPath, err := exec.LookPath(agent.Command)
	if err != nil {
		result.Status = "unreachable"
		result.Error = fmt.Sprintf("command not found: %s", agent.Command)
		return result, nil
	}

	// Try running the command with --version or --help to verify it works
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try --version first (most CLI tools support this)
	cmd := exec.CommandContext(ctx, cmdPath, "--version")
	if agent.WorkDir != "" {
		cmd.Dir = agent.WorkDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try -h as fallback
		ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel2()

		cmd2 := exec.CommandContext(ctx2, cmdPath, "-h")
		if agent.WorkDir != "" {
			cmd2.Dir = agent.WorkDir
		}

		output2, err2 := cmd2.CombinedOutput()
		if err2 != nil {
			// Command exists but doesn't respond to version/help
			// This is still a valid state for some tools
			result.Status = "connected"
			result.AgentCount = 1
			result.Agents = []string{agent.Name}
			return result, nil
		}
		output = output2
	}

	result.Status = "connected"
	result.AgentCount = 1
	result.Agents = []string{agent.Name}

	// Store version info in first 100 chars of output
	versionInfo := string(output)
	if len(versionInfo) > 100 {
		versionInfo = versionInfo[:100] + "..."
	}
	if versionInfo != "" {
		result.Version = versionInfo
	}

	return result, nil
}

// TestAllAgents tests connectivity to all enabled agents
func (m *Manager) TestAllAgents() []*AgentTestResult {
	agents := m.ListAgents()
	results := make([]*AgentTestResult, 0, len(agents))

	for _, agent := range agents {
		result, _ := m.TestAgent(agent.Name)
		if result != nil {
			results = append(results, result)
		}
	}

	return results
}

// AgentTestResult represents the result of testing an agent
type AgentTestResult struct {
	Name       string   `json:"name"`
	URL        string   `json:"url"`
	WorkDir    string   `json:"workdir,omitempty"`
	Enabled    bool     `json:"enabled"`
	Status     string   `json:"status"` // "connected", "unreachable", "error", "disabled"
	Error      string   `json:"error,omitempty"`
	Version    string   `json:"version,omitempty"`
	AgentCount int      `json:"agent_count,omitempty"`
	Agents     []string `json:"agents,omitempty"`
}

// RemoteAgentInfo represents a sub-agent available from an ACP server
type RemoteAgentInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Options     []string `json:"options,omitempty"`
}

// GetRemoteAgents fetches available sub-agents/config options from an ACP agent
func (m *Manager) GetRemoteAgents(name string) ([]RemoteAgentInfo, error) {
	agent, err := m.GetAgent(name)
	if err != nil {
		return nil, err
	}

	if !agent.Enabled {
		return nil, fmt.Errorf("agent '%s' is disabled", name)
	}

	// Only ACP agents support remote agent discovery
	if agent.Type != "acp" && agent.Type != "" {
		return nil, fmt.Errorf("agent '%s' is not an ACP agent", name)
	}

	// Determine command and args
	command := agent.Command
	args := agent.Args

	if command == "" {
		command = agent.Name
		args = getACPArgsForAgent(agent.Name)
	}

	// Check if command exists
	cmdPath, err := exec.LookPath(command)
	if err != nil {
		return nil, fmt.Errorf("command not found: %s", command)
	}

	// Create a stdio client to query config options
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := NewStdioClient(cmdPath, args, agent.WorkDir, agent.Env)
	if err != nil {
		return nil, fmt.Errorf("failed to start agent: %v", err)
	}
	defer client.Close()

	// Initialize
	if err := client.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("initialization failed: %v", err)
	}

	// Create session to get models and config options
	cwd := agent.WorkDir
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	sessionInfo, err := client.NewSessionWithInfo(ctx, cwd)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %v", err)
	}

	var remoteAgents []RemoteAgentInfo

	// Extract models from session info (OpenCode style)
	if sessionInfo.Models != nil && len(sessionInfo.Models.AvailableModels) > 0 {
		var modelOptions []string
		for _, model := range sessionInfo.Models.AvailableModels {
			modelOptions = append(modelOptions, model.ModelID)
		}
		remoteAgents = append(remoteAgents, RemoteAgentInfo{
			ID:          "model",
			Name:        "Model",
			Description: fmt.Sprintf("Current: %s", sessionInfo.Models.CurrentModelID),
			Options:     modelOptions,
		})
	}

	// Extract modes from session info
	if sessionInfo.Modes != nil && len(sessionInfo.Modes.AvailableModes) > 0 {
		var modeOptions []string
		for _, mode := range sessionInfo.Modes.AvailableModes {
			modeOptions = append(modeOptions, mode.ID)
		}
		remoteAgents = append(remoteAgents, RemoteAgentInfo{
			ID:          "mode",
			Name:        "Mode",
			Description: fmt.Sprintf("Current: %s", sessionInfo.Modes.CurrentModeID),
			Options:     modeOptions,
		})
	}

	// If we got models/modes from session info, return them
	if len(remoteAgents) > 0 {
		return remoteAgents, nil
	}

	// Fallback: Try to get config options (for agents that support this method)
	configOptions, err := client.GetConfigOptions(ctx, sessionInfo.SessionID)
	if err != nil {
		// Check if the agent doesn't support config options (Method not found error)
		// This is expected for agents that don't implement session/get_config_options
		errStr := err.Error()
		if strings.Contains(errStr, "Method not found") || strings.Contains(errStr, "-32601") {
			// Agent doesn't support config options, try known models fallback
			if knownModels := getKnownModelsForAgent(agent.Name); knownModels != nil {
				return knownModels, nil
			}
			return []RemoteAgentInfo{}, nil
		}
		return nil, fmt.Errorf("failed to get config options: %v", err)
	}

	// Convert config options to remote agent info
	// Look for config options that represent agent/model selection
	for _, opt := range configOptions {
		// Look for agent-related config options (typically named "agent", "subagent", "model", etc.)
		if opt.ID == "agent" || opt.ID == "subagent" || opt.ID == "sub_agent" || opt.ID == "model" || opt.Category == "agent" {
			info := RemoteAgentInfo{
				ID:          opt.ID,
				Name:        opt.Name,
				Description: opt.Description,
			}
			for _, optVal := range opt.Options {
				info.Options = append(info.Options, optVal.Value)
			}
			remoteAgents = append(remoteAgents, info)
		}
	}

	// If no agent-specific options found, return all config options as potential agents
	if len(remoteAgents) == 0 {
		for _, opt := range configOptions {
			if len(opt.Options) > 0 {
				info := RemoteAgentInfo{
					ID:          opt.ID,
					Name:        opt.Name,
					Description: opt.Description,
				}
				for _, optVal := range opt.Options {
					info.Options = append(info.Options, optVal.Value)
				}
				remoteAgents = append(remoteAgents, info)
			}
		}
	}

	return remoteAgents, nil
}

// getKnownModelsForAgent returns known models for popular agents that don't expose them via ACP
func getKnownModelsForAgent(agentName string) []RemoteAgentInfo {
	knownModels := map[string][]string{
		"gemini": {
			"gemini-3-pro",
			"gemini-3-flash",
			"gemini-2.5-pro",
			"gemini-2.5-flash",
			"gemini-2.5-flash-lite",
			"gemini-2.0-flash",
			"gemini-2.0-flash-lite",
			"gemini-1.5-pro",
			"gemini-1.5-flash",
		},
		"claude-code-acp": {
			"claude-sonnet-4-5-20250514",
			"claude-sonnet-4-20250514",
			"claude-opus-4-20250514",
		},
	}

	if models, ok := knownModels[agentName]; ok {
		return []RemoteAgentInfo{
			{
				ID:          "model",
				Name:        "Model",
				Description: "Use --model flag or GEMINI_MODEL env var",
				Options:     models,
			},
		}
	}
	return nil
}

// getACPArgsForAgent returns the ACP command-line args for known agents
func getACPArgsForAgent(agentName string) []string {
	acpArgs := map[string][]string{
		"opencode":        {"acp"},
		"gemini":          {"--experimental-acp"},
		"claude-code-acp": {"--acp"},
		"codex-acp":       {"acp"},
		"auggie":          {"acp"},
	}

	if args, ok := acpArgs[agentName]; ok {
		return args
	}
	// Default to "acp" subcommand
	return []string{"acp"}
}

// Reload reloads the configuration from disk
func (m *Manager) Reload() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Close and clear stdio clients
	for _, client := range m.stdioClients {
		client.Close()
	}
	m.stdioClients = make(map[string]*StdioClient)

	// Clear cached HTTP clients
	m.clients = make(map[string]*Client)

	return m.loadConfig()
}

// GetEnabledAgents returns all enabled agents
func (m *Manager) GetEnabledAgents() []AgentConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]AgentConfig, 0)
	for _, a := range m.config.Agents {
		if a.Enabled {
			result = append(result, a)
		}
	}
	return result
}

// GetAgentsForWorkDir returns all agents configured for a specific working directory
func (m *Manager) GetAgentsForWorkDir(workDir string) []AgentConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]AgentConfig, 0)
	for _, a := range m.config.Agents {
		if a.WorkDir == workDir {
			result = append(result, a)
		}
	}
	return result
}

// GetAgentByKey returns an agent by its unique key (name@workdir or just name)
func (m *Manager) GetAgentByKey(key string) (*AgentConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, a := range m.config.Agents {
		if a.UniqueKey() == key {
			return &a, nil
		}
	}

	return nil, fmt.Errorf("agent '%s' not found", key)
}

// AddAgentForWorkDir adds an agent configured for a specific workspace
func (m *Manager) AddAgentForWorkDir(agent AgentConfig, workDir string) error {
	agent.WorkDir = workDir

	// Check for duplicate key
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, a := range m.config.Agents {
		if a.UniqueKey() == agent.UniqueKey() {
			return fmt.Errorf("agent '%s' already exists for workspace '%s'", agent.Name, workDir)
		}
	}

	// Set defaults
	if agent.Type == "" {
		agent.Type = "acp"
	}

	m.config.Agents = append(m.config.Agents, agent)
	return m.saveConfig()
}

// GetAvailablePorts returns the next available port for a new agent instance
func (m *Manager) GetAvailablePort(startPort int) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	usedPorts := make(map[int]bool)
	for _, a := range m.config.Agents {
		if a.Port > 0 {
			usedPorts[a.Port] = true
		}
	}

	for port := startPort; port < startPort+100; port++ {
		if !usedPorts[port] {
			return port
		}
	}
	return startPort + 100
}

// RunAgent runs a prompt against an agent (supports both ACP and stdio types)
func (m *Manager) RunAgent(name, prompt string) (*Run, error) {
	agent, err := m.GetAgent(name)
	if err != nil {
		return nil, err
	}

	if !agent.Enabled {
		return nil, fmt.Errorf("agent '%s' is disabled", name)
	}

	// Handle simple stdio agents (just run command with prompt as arg)
	if agent.Type == "stdio" {
		return m.runSimpleStdioAgent(agent, prompt)
	}

	// Handle ACP agents (proper ACP protocol over stdio)
	if agent.Type == "acp" || agent.Type == "" {
		return m.runACPAgent(agent, prompt)
	}

	return nil, fmt.Errorf("unknown agent type: %s", agent.Type)
}

// runACPAgent runs a prompt against an ACP agent using the proper protocol
func (m *Manager) runACPAgent(agent *AgentConfig, prompt string) (*Run, error) {
	// Create run record
	runID := make([]byte, 16)
	rand.Read(runID)
	run := &Run{
		AgentName: agent.Name,
		RunID:     hex.EncodeToString(runID),
		Status:    RunStatusInProgress,
		Output:    []Message{},
		CreatedAt: time.Now(),
	}

	// Determine command and args
	command := agent.Command
	args := agent.Args

	// If no command specified, try to infer from name
	if command == "" {
		command = agent.Name
		args = []string{"acp"}
	}

	// Check if command exists
	cmdPath, err := exec.LookPath(command)
	if err != nil {
		now := time.Now()
		run.FinishedAt = &now
		run.Status = RunStatusFailed
		run.Error = &Error{
			Code:    "command_not_found",
			Message: fmt.Sprintf("command not found: %s", command),
		}
		return run, nil
	}

	// Create stdio client
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	client, err := NewStdioClient(cmdPath, args, agent.WorkDir, agent.Env)
	if err != nil {
		now := time.Now()
		run.FinishedAt = &now
		run.Status = RunStatusFailed
		run.Error = &Error{
			Code:    "start_error",
			Message: fmt.Sprintf("failed to start agent: %v", err),
		}
		return run, nil
	}
	defer client.Close()

	// Initialize
	if err := client.Initialize(ctx); err != nil {
		now := time.Now()
		run.FinishedAt = &now
		run.Status = RunStatusFailed
		run.Error = &Error{
			Code:    "init_error",
			Message: fmt.Sprintf("initialization failed: %v", err),
		}
		return run, nil
	}

	// Create session
	cwd := agent.WorkDir
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	sessionID, err := client.NewSession(ctx, cwd)
	if err != nil {
		now := time.Now()
		run.FinishedAt = &now
		run.Status = RunStatusFailed
		run.Error = &Error{
			Code:    "session_error",
			Message: fmt.Sprintf("failed to create session: %v", err),
		}
		return run, nil
	}

	// Collect output from updates
	var outputText string

	// Send prompt
	result, err := client.Prompt(ctx, sessionID, prompt, func(update *SessionUpdateParams) {
		if update.Update.SessionUpdate == "agent_message_chunk" && update.Update.Content != nil {
			if update.Update.Content.Type == "text" {
				outputText += update.Update.Content.Text
			}
		}
	})

	now := time.Now()
	run.FinishedAt = &now

	if err != nil {
		run.Status = RunStatusFailed
		run.Error = &Error{
			Code:    "prompt_error",
			Message: fmt.Sprintf("prompt failed: %v", err),
		}
		return run, nil
	}

	// Check stop reason
	switch result.StopReason {
	case "end_turn":
		run.Status = RunStatusCompleted
	case "cancelled":
		run.Status = RunStatusCancelled
	default:
		run.Status = RunStatusCompleted
	}

	run.Output = []Message{
		NewTextMessage("agent", outputText),
	}

	return run, nil
}

// runSimpleStdioAgent runs a prompt against a simple stdio-based agent
func (m *Manager) runSimpleStdioAgent(agent *AgentConfig, prompt string) (*Run, error) {
	// Build command with args and prompt
	args := append([]string{}, agent.Args...)
	args = append(args, prompt)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, agent.Command, args...)
	if agent.WorkDir != "" {
		cmd.Dir = agent.WorkDir
	}

	// Set environment variables
	if len(agent.Env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range agent.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Create run record
	runID := make([]byte, 16)
	rand.Read(runID)
	run := &Run{
		AgentName: agent.Name,
		RunID:     hex.EncodeToString(runID),
		Status:    RunStatusInProgress,
		Output:    []Message{},
		CreatedAt: time.Now(),
	}

	// Execute and capture output
	output, err := cmd.CombinedOutput()
	now := time.Now()
	run.FinishedAt = &now

	if err != nil {
		run.Status = RunStatusFailed
		run.Error = &Error{
			Code:    "execution_error",
			Message: fmt.Sprintf("%v: %s", err, string(output)),
		}
		return run, nil
	}

	run.Status = RunStatusCompleted
	run.Output = []Message{
		NewTextMessage("agent", string(output)),
	}

	return run, nil
}
