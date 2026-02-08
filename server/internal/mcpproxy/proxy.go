package mcpproxy

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ServerConfig represents configuration for an MCP server
type ServerConfig struct {
	Name    string            `json:"name"`
	Enabled bool              `json:"enabled"`
	Type    string            `json:"type"` // stdio, http, etc.
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

// Config represents the MCP proxy configuration
type Config struct {
	Servers []ServerConfig `json:"servers"`
}

// Proxy manages multiple MCP clients
type Proxy struct {
	clients    map[string]*MCPClient
	config     *Config
	configPath string // Store config path for reload
	mu         sync.RWMutex
	notifyChan chan string       // Aggregated notifications channel
	initErrors map[string]string // Store initialization errors per server
}

// NewProxy creates a new MCP proxy
func NewProxy(configPath string) (*Proxy, error) {
	config, err := loadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	proxy := &Proxy{
		clients:    make(map[string]*MCPClient),
		config:     config,
		configPath: configPath,
		notifyChan: make(chan string, 10), // Buffered channel for notifications
		initErrors: make(map[string]string),
	}

	// Start enabled MCP servers
	for _, server := range config.Servers {
		if server.Enabled {
			if err := proxy.startClient(server); err != nil {
				log.Printf("Failed to start MCP server %s: %v", server.Name, err)
				proxy.initErrors[server.Name] = err.Error()
			}
		}
	}

	// Start notification monitor
	go proxy.monitorNotifications()

	return proxy, nil
}

// startClient starts an MCP client
func (p *Proxy) startClient(config ServerConfig) error {
	client, err := NewMCPClient(config.Name, config.Command, config.Args, config.Env)
	if err != nil {
		return err
	}

	p.mu.Lock()
	p.clients[config.Name] = client
	p.mu.Unlock()

	log.Printf("Started MCP server: %s", config.Name)
	return nil
}

// ListAllTools aggregates tools from all MCP clients
func (p *Proxy) ListAllTools() ([]map[string]interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var allTools []map[string]interface{}

	for serverName, client := range p.clients {
		tools, err := client.ListTools()
		if err != nil {
			log.Printf("Failed to list tools from %s: %v", serverName, err)
			continue
		}

		// Prefix tool names with server name to avoid conflicts
		for _, tool := range tools {
			if name, ok := tool["name"].(string); ok {
				tool["name"] = serverName + "_" + name
				tool["_server"] = serverName // Track which server this tool belongs to
			}
			allTools = append(allTools, tool)
		}
	}

	return allTools, nil
}

// CallTool routes a tool call to the appropriate MCP client
func (p *Proxy) CallTool(toolName string, arguments map[string]interface{}) (json.RawMessage, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Parse server name from tool name (format: server_toolname)
	var serverName, actualToolName string
	for sName := range p.clients {
		prefix := sName + "_"
		if len(toolName) > len(prefix) && toolName[:len(prefix)] == prefix {
			serverName = sName
			actualToolName = toolName[len(prefix):]
			break
		}
	}

	if serverName == "" {
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}

	client, ok := p.clients[serverName]
	if !ok {
		return nil, fmt.Errorf("server not found: %s", serverName)
	}

	return client.CallTool(actualToolName, arguments)
}

// monitorNotifications watches all client notification channels
func (p *Proxy) monitorNotifications() {
	p.mu.RLock()
	clients := make([]*MCPClient, 0, len(p.clients))
	for _, client := range p.clients {
		clients = append(clients, client)
	}
	p.mu.RUnlock()

	// Start a goroutine for each client to forward notifications
	for _, client := range clients {
		go p.monitorClient(client)
	}
}

// NotificationChan returns the channel for receiving aggregated notifications
func (p *Proxy) NotificationChan() <-chan string {
	return p.notifyChan
}

// Reload reloads the MCP configuration and starts/stops servers as needed
func (p *Proxy) Reload() error {
	log.Printf("Reloading MCP configuration from %s", p.configPath)

	newConfig, err := loadConfig(p.configPath)
	if err != nil {
		return fmt.Errorf("failed to load new config: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Build map of new enabled servers
	newServers := make(map[string]ServerConfig)
	for _, s := range newConfig.Servers {
		if s.Enabled && s.Type == "stdio" {
			newServers[s.Name] = s
		}
	}

	// Stop removed servers
	for name, client := range p.clients {
		if _, exists := newServers[name]; !exists {
			log.Printf("Stopping removed MCP server: %s", name)
			client.Close()
			delete(p.clients, name)
		}
	}

	// Start new servers
	for name, serverConfig := range newServers {
		if _, exists := p.clients[name]; !exists {
			log.Printf("Starting new MCP server: %s", name)
			if err := p.startClientUnlocked(serverConfig); err != nil {
				log.Printf("Failed to start %s: %v", name, err)
			}
		}
	}

	p.config = newConfig

	// Send notification that tools changed
	select {
	case p.notifyChan <- "config-reload":
		log.Printf("Sent config-reload notification")
	default:
		log.Printf("Notification channel full, dropping config-reload notification")
	}

	log.Printf("MCP configuration reload complete")
	return nil
}

// startClientUnlocked starts a client (assumes lock is held by caller)
func (p *Proxy) startClientUnlocked(config ServerConfig) error {
	client, err := NewMCPClient(config.Name, config.Command, config.Args, config.Env)
	if err != nil {
		p.initErrors[config.Name] = err.Error()
		return err
	}

	// Clear any previous init error
	delete(p.initErrors, config.Name)

	p.clients[config.Name] = client

	// Start monitoring this client's notifications
	go p.monitorClient(client)

	log.Printf("Started MCP server: %s", config.Name)
	return nil
}

// monitorClient monitors a single client for notifications
func (p *Proxy) monitorClient(client *MCPClient) {
	for method := range client.NotificationChan() {
		if method == "notifications/tools/list_changed" {
			log.Printf("[%s] Tools changed, forwarding notification", client.Name)
			select {
			case p.notifyChan <- client.Name:
			default:
				log.Printf("Proxy notification channel full, dropping notification from %s", client.Name)
			}
		}
	}
}

// ServerStatus represents the status of an MCP server
type ServerStatus struct {
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	Connected bool   `json:"connected"`
	ToolCount int    `json:"tool_count"`
	Error     string `json:"error,omitempty"`
}

// GetServerStatuses returns the status of all configured MCP servers (non-blocking)
func (p *Proxy) GetServerStatuses() []ServerStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var statuses []ServerStatus

	for _, server := range p.config.Servers {
		status := ServerStatus{
			Name:    server.Name,
			Enabled: server.Enabled,
		}

		if client, ok := p.clients[server.Name]; ok {
			status.Connected = client.IsConnected()
			// Use cached tool count for fast response
			cachedCount := client.GetCachedToolCount()
			if cachedCount >= 0 {
				status.ToolCount = cachedCount
			}
			// If no cache, trigger async refresh (only one at a time)
			if cachedCount < 0 && status.Connected {
				client.TriggerAsyncRefresh(30 * time.Second)
			}
			// Include error info if not connected
			if !status.Connected {
				if errMsg := client.GetLastError(); errMsg != "" {
					status.Error = errMsg
				} else if stderr := client.GetStderrOutput(); stderr != "" {
					status.Error = stderr
				}
			}
		} else if server.Enabled {
			// Client not created - check if there's an init error stored
			if errMsg, ok := p.initErrors[server.Name]; ok {
				status.Error = errMsg
			}
		}

		statuses = append(statuses, status)
	}

	return statuses
}

// RestartServer restarts a specific MCP server by name
func (p *Proxy) RestartServer(name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Find the server config
	var serverConfig *ServerConfig
	for _, s := range p.config.Servers {
		if s.Name == name {
			serverConfig = &s
			break
		}
	}

	if serverConfig == nil {
		return fmt.Errorf("server not found: %s", name)
	}

	// Close existing client if running
	if client, ok := p.clients[name]; ok {
		log.Printf("Stopping MCP server for restart: %s", name)
		client.Close()
		delete(p.clients, name)
	}

	// Start fresh
	if serverConfig.Enabled && serverConfig.Type == "stdio" {
		log.Printf("Restarting MCP server: %s", name)
		if err := p.startClientUnlocked(*serverConfig); err != nil {
			return fmt.Errorf("failed to restart %s: %w", name, err)
		}
	}

	// Notify about tool changes
	select {
	case p.notifyChan <- name:
	default:
	}

	return nil
}

// GetTotalToolCount returns the total number of tools across all connected servers (uses cache)
func (p *Proxy) GetTotalToolCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	total := 0
	for _, client := range p.clients {
		if count := client.GetCachedToolCount(); count > 0 {
			total += count
		}
	}
	return total
}

// Close shuts down all MCP clients
func (p *Proxy) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for name, client := range p.clients {
		log.Printf("Shutting down MCP server: %s", name)
		if err := client.Close(); err != nil {
			log.Printf("Error closing %s: %v", name, err)
		}
	}

	p.clients = make(map[string]*MCPClient)
	return nil
}

// loadConfig loads the MCP proxy configuration
func loadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// GetDefaultConfigPath returns the default config path
func GetDefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".diane", "mcp-servers.json")
}
