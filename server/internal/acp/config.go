// Package acp provides configuration and management for ACP agents.
package acp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// AgentConfig represents a configured ACP agent
type AgentConfig struct {
	Name        string            `json:"name"`
	URL         string            `json:"url"`
	Type        string            `json:"type,omitempty"`    // "acp" (default), "stdio" for local agents
	Command     string            `json:"command,omitempty"` // For stdio agents
	Args        []string          `json:"args,omitempty"`    // For stdio agents
	Env         map[string]string `json:"env,omitempty"`     // Environment variables
	WorkDir     string            `json:"workdir,omitempty"` // Working directory/project path for the agent
	Port        int               `json:"port,omitempty"`    // Port for ACP server (auto-assigned if 0)
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
	mu         sync.RWMutex
	config     *Config
	configPath string
	clients    map[string]*Client
}

// NewManager creates a new ACP manager
func NewManager() (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(home, ".diane", "acp-agents.json")
	m := &Manager{
		configPath: configPath,
		clients:    make(map[string]*Client),
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
			// Remove client if exists
			if client, ok := m.clients[name]; ok {
				_ = client // No cleanup needed for HTTP client
				delete(m.clients, name)
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

	client := NewClient(agent.URL)

	// Test ping
	if err := client.Ping(); err != nil {
		result.Status = "unreachable"
		result.Error = err.Error()
		return result, nil
	}

	// Get agent manifest
	agents, err := client.ListAgents(10, 0)
	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result, nil
	}

	result.Status = "connected"
	result.AgentCount = len(agents)
	result.Agents = make([]string, 0, len(agents))
	for _, a := range agents {
		result.Agents = append(result.Agents, a.Name)
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
	AgentCount int      `json:"agent_count,omitempty"`
	Agents     []string `json:"agents,omitempty"`
}

// Reload reloads the configuration from disk
func (m *Manager) Reload() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear cached clients
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
