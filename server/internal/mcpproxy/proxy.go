package mcpproxy

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// OAuthConfig represents OAuth configuration for an MCP server
type OAuthConfig struct {
	// Provider is a well-known OAuth provider (e.g., "github-copilot")
	// If set, other fields are auto-configured
	Provider string `json:"provider,omitempty"`

	// Manual OAuth configuration
	ClientID     string   `json:"client_id,omitempty"`
	ClientSecret string   `json:"client_secret,omitempty"`
	Scopes       []string `json:"scopes,omitempty"`

	// Device flow endpoints
	DeviceAuthURL string `json:"device_auth_url,omitempty"`
	TokenURL      string `json:"token_url,omitempty"`
}

// ServerConfig represents configuration for an MCP server
type ServerConfig struct {
	Name    string            `json:"name"`
	Enabled bool              `json:"enabled"`
	Type    string            `json:"type"` // stdio, sse, http, remote
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	// SSE/HTTP fields
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	// OAuth configuration
	OAuth *OAuthConfig `json:"oauth,omitempty"`
	// Remote slave fields
	Hostname string `json:"hostname,omitempty"`  // For remote slaves
	CertPath string `json:"cert_path,omitempty"` // Client cert path
	KeyPath  string `json:"key_path,omitempty"`  // Client key path
	CAPath   string `json:"ca_path,omitempty"`   // CA cert path
}

// Config represents the MCP proxy configuration
type Config struct {
	Servers []ServerConfig `json:"servers"`
}

// ConfigProvider is an interface for loading MCP server configurations.
// This allows the proxy to load configs from any source (database, file, etc.)
type ConfigProvider interface {
	// LoadMCPServerConfigs returns all MCP server configurations
	LoadMCPServerConfigs() ([]ServerConfig, error)
}

// Proxy manages multiple MCP clients
type Proxy struct {
	clients        map[string]Client
	config         *Config
	configProvider ConfigProvider // Provider for loading server configs
	mu             sync.RWMutex
	notifyChan     chan string       // Aggregated notifications channel
	initErrors     map[string]string // Store initialization errors per server
	initializing   map[string]bool   // Track servers currently initializing
	initWg         sync.WaitGroup    // Wait group for initial startup
}

// NewProxy creates a new MCP proxy that loads config from the given provider
func NewProxy(provider ConfigProvider) (*Proxy, error) {
	servers, err := provider.LoadMCPServerConfigs()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	config := &Config{Servers: servers}

	proxy := &Proxy{
		clients:        make(map[string]Client),
		config:         config,
		configProvider: provider,
		notifyChan:     make(chan string, 10), // Buffered channel for notifications
		initErrors:     make(map[string]string),
		initializing:   make(map[string]bool),
	}

	// Start enabled MCP servers concurrently in background
	for _, server := range config.Servers {
		if server.Enabled {
			proxy.initWg.Add(1)
			proxy.mu.Lock()
			proxy.initializing[server.Name] = true
			proxy.mu.Unlock()

			go func(srv ServerConfig) {
				defer proxy.initWg.Done()
				if err := proxy.startClient(srv); err != nil {
					slog.Warn("Failed to start MCP server", "server", srv.Name, "error", err)
					proxy.mu.Lock()
					proxy.initErrors[srv.Name] = err.Error()
					proxy.mu.Unlock()
				}
				proxy.mu.Lock()
				delete(proxy.initializing, srv.Name)
				proxy.mu.Unlock()
			}(server)
		}
	}

	// Start notification monitor
	go proxy.monitorNotifications()

	return proxy, nil
}

// WaitForInit waits for all initial servers to finish starting (optional)
func (p *Proxy) WaitForInit() {
	p.initWg.Wait()
}

// IsInitializing returns true if any servers are still initializing
func (p *Proxy) IsInitializing() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.initializing) > 0
}

// GetInitializingServers returns the names of servers still initializing
func (p *Proxy) GetInitializingServers() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	servers := make([]string, 0, len(p.initializing))
	for name := range p.initializing {
		servers = append(servers, name)
	}
	return servers
}

// startClient starts an MCP client based on transport type
func (p *Proxy) startClient(config ServerConfig) error {
	var client Client
	var err error

	switch config.Type {
	case "sse":
		client, err = NewSSEClient(config.Name, config.URL, config.Headers)
	case "http":
		client, err = NewHTTPClientWithOAuth(config.Name, config.URL, config.Headers, config.OAuth)
	case "remote":
		// Remote slave connection via WebSocket
		if config.Hostname == "" || config.URL == "" {
			return fmt.Errorf("remote slaves require hostname and URL (master address)")
		}
		client, err = NewWSClient(config.Name, config.Hostname, config.URL,
			config.CertPath, config.KeyPath, config.CAPath)
	case "stdio", "":
		// Default to stdio
		client, err = NewMCPClient(config.Name, config.Command, config.Args, config.Env)
	default:
		return fmt.Errorf("unsupported transport type: %s", config.Type)
	}

	if err != nil {
		return err
	}

	p.mu.Lock()
	p.clients[config.Name] = client
	p.mu.Unlock()

	slog.Info("Started MCP server", "server", config.Name, "type", config.Type)
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
			slog.Warn("Failed to list tools from server", "server", serverName, "error", err)
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

// ListPromptsForContext returns prompts from servers enabled in the context
func (p *Proxy) ListPromptsForContext(contextName string, contextFilter ContextFilter) ([]map[string]interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var allPrompts []map[string]interface{}

	// Get enabled servers for this context
	enabledServers, err := contextFilter.GetEnabledServersForContext(contextName)
	if err != nil {
		return nil, fmt.Errorf("failed to get enabled servers for context: %w", err)
	}

	// Create a map for faster lookup
	enabledMap := make(map[string]bool)
	for _, name := range enabledServers {
		enabledMap[name] = true
	}

	for serverName, client := range p.clients {
		// Skip if server is not enabled in this context
		if !enabledMap[serverName] {
			continue
		}

		prompts, err := client.ListPrompts()
		if err != nil {
			slog.Warn("Failed to list prompts from server", "server", serverName, "error", err)
			continue
		}

		// Prefix prompt names with server name to avoid conflicts
		for _, prompt := range prompts {
			if name, ok := prompt["name"].(string); ok {
				prompt["name"] = serverName + "_" + name
				prompt["_server"] = serverName // Track which server this prompt belongs to
			}
			allPrompts = append(allPrompts, prompt)
		}
	}

	return allPrompts, nil
}

// GetPromptForContext routes a prompt request to the appropriate MCP client after validating context access
func (p *Proxy) GetPromptForContext(contextName, promptName string, arguments map[string]string, contextFilter ContextFilter) (json.RawMessage, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Parse server name from prompt name (format: server_promptname)
	var serverName, actualPromptName string
	for sName := range p.clients {
		prefix := sName + "_"
		if len(promptName) > len(prefix) && promptName[:len(prefix)] == prefix {
			serverName = sName
			actualPromptName = promptName[len(prefix):]
			break
		}
	}

	if serverName == "" {
		return nil, fmt.Errorf("unknown prompt: %s", promptName)
	}

	// Check if server is enabled in the context
	enabledServers, err := contextFilter.GetEnabledServersForContext(contextName)
	if err != nil {
		return nil, fmt.Errorf("failed to check context access: %w", err)
	}

	isEnabled := false
	for _, name := range enabledServers {
		if name == serverName {
			isEnabled = true
			break
		}
	}

	if !isEnabled {
		return nil, fmt.Errorf("prompt %s is not enabled in context %s", promptName, contextName)
	}

	client, ok := p.clients[serverName]
	if !ok {
		return nil, fmt.Errorf("server not found: %s", serverName)
	}

	return client.GetPrompt(actualPromptName, arguments)
}

// ListAllPrompts aggregates prompts from all MCP clients
func (p *Proxy) ListAllPrompts() ([]map[string]interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var allPrompts []map[string]interface{}

	for serverName, client := range p.clients {
		prompts, err := client.ListPrompts()
		if err != nil {
			slog.Warn("Failed to list prompts from server", "server", serverName, "error", err)
			continue
		}

		// Prefix prompt names with server name to avoid conflicts
		for _, prompt := range prompts {
			if name, ok := prompt["name"].(string); ok {
				prompt["name"] = serverName + "_" + name
				prompt["_server"] = serverName // Track which server this prompt belongs to
			}
			allPrompts = append(allPrompts, prompt)
		}
	}

	return allPrompts, nil
}

// GetPrompt routes a prompt request to the appropriate MCP client
func (p *Proxy) GetPrompt(promptName string, arguments map[string]string) (json.RawMessage, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Parse server name from prompt name (format: server_promptname)
	var serverName, actualPromptName string
	for sName := range p.clients {
		prefix := sName + "_"
		if len(promptName) > len(prefix) && promptName[:len(prefix)] == prefix {
			serverName = sName
			actualPromptName = promptName[len(prefix):]
			break
		}
	}

	if serverName == "" {
		return nil, fmt.Errorf("unknown prompt: %s", promptName)
	}

	client, ok := p.clients[serverName]
	if !ok {
		return nil, fmt.Errorf("server not found: %s", serverName)
	}

	return client.GetPrompt(actualPromptName, arguments)
}

// ListAllResources aggregates resources from all MCP clients
func (p *Proxy) ListAllResources() ([]map[string]interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var allResources []map[string]interface{}

	for serverName, client := range p.clients {
		resources, err := client.ListResources()
		if err != nil {
			slog.Warn("Failed to list resources from server", "server", serverName, "error", err)
			continue
		}

		// Add server metadata to track which server this resource belongs to
		for _, resource := range resources {
			resource["_server"] = serverName
			allResources = append(allResources, resource)
		}
	}

	return allResources, nil
}

// ReadResource routes a resource read request to the appropriate MCP client
// The URI should be prefixed with the server name (format: server_name://uri)
func (p *Proxy) ReadResource(uri string) (json.RawMessage, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Parse server name from URI prefix (format: server_name://actualuri)
	var serverName, actualURI string
	for sName := range p.clients {
		prefix := sName + "://"
		if len(uri) > len(prefix) && uri[:len(prefix)] == prefix {
			serverName = sName
			actualURI = uri[len(prefix):]
			break
		}
	}

	if serverName == "" {
		return nil, fmt.Errorf("unknown resource server for URI: %s", uri)
	}

	client, ok := p.clients[serverName]
	if !ok {
		return nil, fmt.Errorf("server not found: %s", serverName)
	}

	return client.ReadResource(actualURI)
}

// monitorNotifications watches all client notification channels
func (p *Proxy) monitorNotifications() {
	p.mu.RLock()
	clients := make([]Client, 0, len(p.clients))
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
	slog.Info("Reloading MCP configuration")

	servers, err := p.configProvider.LoadMCPServerConfigs()
	if err != nil {
		return fmt.Errorf("failed to load new config: %w", err)
	}
	newConfig := &Config{Servers: servers}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Build map of new enabled servers (all supported types)
	newServers := make(map[string]ServerConfig)
	for _, s := range newConfig.Servers {
		if s.Enabled && (s.Type == "stdio" || s.Type == "sse" || s.Type == "http" || s.Type == "") {
			newServers[s.Name] = s
		}
	}

	// Stop removed servers
	for name, client := range p.clients {
		if _, exists := newServers[name]; !exists {
			slog.Info("Stopping removed MCP server", "server", name)
			client.Close()
			delete(p.clients, name)
		}
	}

	// Start new servers
	for name, serverConfig := range newServers {
		if _, exists := p.clients[name]; !exists {
			slog.Info("Starting new MCP server", "server", name)
			if err := p.startClientUnlocked(serverConfig); err != nil {
				slog.Warn("Failed to start MCP server", "server", name, "error", err)
			}
		}
	}

	p.config = newConfig

	// Send notification that tools changed
	select {
	case p.notifyChan <- "config-reload":
		slog.Debug("Sent config-reload notification")
	default:
		slog.Warn("Notification channel full, dropping config-reload notification")
	}

	slog.Info("MCP configuration reload complete")
	return nil
}

// startClientUnlocked starts a client (assumes lock is held by caller)
func (p *Proxy) startClientUnlocked(config ServerConfig) error {
	var client Client
	var err error

	switch config.Type {
	case "sse":
		client, err = NewSSEClient(config.Name, config.URL, config.Headers)
	case "http":
		client, err = NewHTTPClientWithOAuth(config.Name, config.URL, config.Headers, config.OAuth)
	case "stdio", "":
		client, err = NewMCPClient(config.Name, config.Command, config.Args, config.Env)
	default:
		err = fmt.Errorf("unsupported transport type: %s", config.Type)
	}

	if err != nil {
		p.initErrors[config.Name] = err.Error()
		return err
	}

	// Clear any previous init error
	delete(p.initErrors, config.Name)

	p.clients[config.Name] = client

	// Start monitoring this client's notifications
	go p.monitorClient(client)

	slog.Info("Started MCP server", "server", config.Name, "type", config.Type)
	return nil
}

// monitorClient monitors a single client for notifications
func (p *Proxy) monitorClient(client Client) {
	for method := range client.NotificationChan() {
		if method == "notifications/tools/list_changed" {
			slog.Debug("Tools changed, forwarding notification", "server", client.GetName())
			select {
			case p.notifyChan <- client.GetName():
			default:
				slog.Warn("Proxy notification channel full, dropping notification", "server", client.GetName())
			}
		}
	}
}

// ServerStatus represents the status of an MCP server
type ServerStatus struct {
	Name          string `json:"name"`
	Enabled       bool   `json:"enabled"`
	Connected     bool   `json:"connected"`
	ToolCount     int    `json:"tool_count"`
	PromptCount   int    `json:"prompt_count"`
	ResourceCount int    `json:"resource_count"`
	Error         string `json:"error,omitempty"`
	RequiresAuth  bool   `json:"requires_auth,omitempty"`
	Authenticated bool   `json:"authenticated,omitempty"`
}

// GetServerStatuses returns the status of all configured MCP servers (non-blocking)
func (p *Proxy) GetServerStatuses() []ServerStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	oauthMgr := GetOAuthManager()
	var statuses []ServerStatus

	for _, server := range p.config.Servers {
		status := ServerStatus{
			Name:    server.Name,
			Enabled: server.Enabled,
		}

		// Check if this server requires OAuth authentication
		if server.OAuth != nil {
			status.RequiresAuth = true
			if oauthMgr != nil {
				status.Authenticated = oauthMgr.HasValidToken(server.Name)
			}
		}

		if client, ok := p.clients[server.Name]; ok {
			status.Connected = client.IsConnected()
			// Use cached tool count for fast response
			cachedCount := client.GetCachedToolCount()
			if cachedCount >= 0 {
				status.ToolCount = cachedCount
			}
			// Use cached prompt count for fast response
			cachedPromptCount := client.GetCachedPromptCount()
			if cachedPromptCount >= 0 {
				status.PromptCount = cachedPromptCount
			}
			// Use cached resource count for fast response
			cachedResourceCount := client.GetCachedResourceCount()
			if cachedResourceCount >= 0 {
				status.ResourceCount = cachedResourceCount
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
		slog.Info("Stopping MCP server for restart", "server", name)
		client.Close()
		delete(p.clients, name)
	}

	// Start fresh
	if serverConfig.Enabled && (serverConfig.Type == "stdio" || serverConfig.Type == "sse" || serverConfig.Type == "http" || serverConfig.Type == "") {
		slog.Info("Restarting MCP server", "server", name)
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
		slog.Info("Shutting down MCP server", "server", name)
		if err := client.Close(); err != nil {
			slog.Warn("Error closing MCP server", "server", name, "error", err)
		}
	}

	p.clients = make(map[string]Client)
	return nil
}

// RegisterSlaveClient dynamically registers a remote slave client
// This is used by the slave registry to add slaves as they connect
func (p *Proxy) RegisterSlaveClient(name string, client Client) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.clients[name]; exists {
		return fmt.Errorf("client with name %s already exists", name)
	}

	p.clients[name] = client
	slog.Info("Registered slave client", "name", name)

	// Start monitoring the client
	go p.monitorClient(client)

	return nil
}

// UnregisterSlaveClient dynamically removes a remote slave client
// This is used by the slave registry when slaves disconnect
func (p *Proxy) UnregisterSlaveClient(name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	client, exists := p.clients[name]
	if !exists {
		return fmt.Errorf("client with name %s not found", name)
	}

	// Close the client
	if err := client.Close(); err != nil {
		slog.Warn("Error closing slave client", "name", name, "error", err)
	}

	delete(p.clients, name)
	slog.Info("Unregistered slave client", "name", name)

	return nil
}

// GetClient returns a client by name (useful for testing and debugging)
func (p *Proxy) GetClient(name string) (Client, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	client, ok := p.clients[name]
	return client, ok
}

// GetServerConfig returns the configuration for a specific server
func (p *Proxy) GetServerConfig(name string) *ServerConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, s := range p.config.Servers {
		if s.Name == name {
			return &s
		}
	}
	return nil
}

// GetServersWithOAuth returns all servers that have OAuth configured
func (p *Proxy) GetServersWithOAuth() []ServerConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var result []ServerConfig
	for _, s := range p.config.Servers {
		if s.OAuth != nil {
			result = append(result, s)
		}
	}
	return result
}

// ContextFilter provides methods to filter tools by context
// This interface is implemented by the db package
type ContextFilter interface {
	// IsToolEnabledInContext checks if a tool is enabled for a server in a context
	IsToolEnabledInContext(contextName, serverName, toolName string) (bool, error)
	// GetEnabledServersForContext returns servers that are enabled in a context
	GetEnabledServersForContext(contextName string) ([]string, error)
	// GetDefaultContext returns the default context name
	GetDefaultContext() (string, error)
}

// ListToolsForContext returns tools filtered by context settings
// If contextFilter is nil or contextName is empty, returns all tools (no filtering)
func (p *Proxy) ListToolsForContext(contextName string, contextFilter ContextFilter) ([]map[string]interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var allTools []map[string]interface{}

	for serverName, client := range p.clients {
		tools, err := client.ListTools()
		if err != nil {
			slog.Warn("Failed to list tools from server", "server", serverName, "error", err)
			continue
		}

		for _, tool := range tools {
			name, ok := tool["name"].(string)
			if !ok {
				continue
			}

			// If context filtering is enabled, check if tool is enabled
			if contextFilter != nil && contextName != "" {
				enabled, err := contextFilter.IsToolEnabledInContext(contextName, serverName, name)
				if err != nil {
					slog.Warn("Failed to check tool context", "context", contextName, "server", serverName, "tool", name, "error", err)
					// On error, exclude the tool (fail closed)
					enabled = false
				}
				if !enabled {
					continue
				}
			}

			// Prefix tool names with server name to avoid conflicts
			tool["name"] = serverName + "_" + name
			tool["_server"] = serverName
			allTools = append(allTools, tool)
		}
	}

	return allTools, nil
}

// CallToolForContext routes a tool call to the appropriate MCP client after validating context access
// If contextFilter is nil or contextName is empty, no context validation is performed
func (p *Proxy) CallToolForContext(contextName, toolName string, arguments map[string]interface{}, contextFilter ContextFilter) (json.RawMessage, error) {
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

	// Validate context access if filtering is enabled
	if contextFilter != nil && contextName != "" {
		enabled, err := contextFilter.IsToolEnabledInContext(contextName, serverName, actualToolName)
		if err != nil {
			slog.Warn("Failed to check tool context access", "context", contextName, "server", serverName, "tool", actualToolName, "error", err)
			return nil, fmt.Errorf("failed to verify context access for tool %s", toolName)
		} else if !enabled {
			return nil, fmt.Errorf("tool %s is not enabled in context %s", toolName, contextName)
		}
	}

	client, ok := p.clients[serverName]
	if !ok {
		return nil, fmt.Errorf("server not found: %s", serverName)
	}

	return client.CallTool(actualToolName, arguments)
}
