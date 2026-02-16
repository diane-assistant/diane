import Foundation

/// Protocol abstracting DianeClient for dependency injection and testing.
///
/// Extracts all public methods from `DianeClient` into a protocol so that
/// views and ViewModels can depend on the abstraction rather than the
/// concrete class. This enables mock implementations in tests.
///
/// Marked `@MainActor` because all ViewModels that use this protocol are
/// `@MainActor`-isolated, and the concrete `DianeClient` performs its work
/// on background queues internally via continuation-based bridging.
@MainActor
protocol DianeClientProtocol {

    // MARK: - Process Management

    /// Check if the socket file exists
    var socketExists: Bool { get }

    /// Read PID from file
    func getPID() -> Int?

    /// Check if process is running
    func isProcessRunning() -> Bool

    /// Start Diane (launches the process)
    func startDiane() throws

    /// Stop Diane (sends SIGTERM)
    func stopDiane() throws

    /// Restart Diane
    func restartDiane() async throws

    /// Send reload signal (SIGUSR1)
    func sendReloadSignal() throws

    // MARK: - Health & Status

    /// Health check
    func health() async -> Bool

    /// Get full status
    func getStatus() async throws -> DianeStatus

    /// Reload configuration
    func reloadConfig() async throws

    // MARK: - MCP Servers (Runtime)

    /// Get MCP servers runtime status
    func getMCPServers() async throws -> [MCPServerStatus]

    /// Restart an MCP server
    func restartMCPServer(name: String) async throws

    // MARK: - MCP Server Configuration

    /// Get all MCP server configurations
    func getMCPServerConfigs() async throws -> [MCPServer]

    /// Get a specific MCP server configuration by ID
    func getMCPServerConfig(id: Int64) async throws -> MCPServer

    /// Create a new MCP server configuration
    func createMCPServerConfig(name: String, type: String, enabled: Bool, command: String?, args: [String]?, env: [String: String]?, url: String?, headers: [String: String]?, oauth: OAuthConfig?) async throws -> MCPServer

    /// Update an MCP server configuration
    func updateMCPServerConfig(id: Int64, name: String?, type: String?, enabled: Bool?, command: String?, args: [String]?, env: [String: String]?, url: String?, headers: [String: String]?, oauth: OAuthConfig?) async throws -> MCPServer

    /// Delete an MCP server configuration
    func deleteMCPServerConfig(id: Int64) async throws

    // MARK: - Tools

    /// Get all tools
    func getTools() async throws -> [ToolInfo]
    
    /// Get all prompts
    func getPrompts() async throws -> [PromptInfo]
    
    /// Get all resources
    func getResources() async throws -> [ResourceInfo]

    // MARK: - OAuth

    /// Start OAuth login for an MCP server (device flow)
    func startAuth(serverName: String) async throws -> DeviceCodeInfo

    /// Poll for OAuth token completion
    func pollAuth(serverName: String, deviceCode: String, interval: Int) async throws

    // MARK: - Scheduler

    /// Get all scheduled jobs
    func getJobs() async throws -> [Job]

    /// Get job execution logs
    func getJobLogs(jobName: String?, limit: Int) async throws -> [JobExecution]

    /// Toggle a job's enabled status
    func toggleJob(name: String, enabled: Bool) async throws

    // MARK: - Agents

    /// Get all configured agents
    func getAgents() async throws -> [AgentConfig]

    /// Get a specific agent by name
    func getAgent(name: String) async throws -> AgentConfig

    /// Test an agent's connectivity
    func testAgent(name: String) async throws -> AgentTestResult

    /// Toggle an agent's enabled status
    func toggleAgent(name: String, enabled: Bool) async throws

    /// Run a prompt on an agent
    func runAgentPrompt(agentName: String, prompt: String, remoteAgentName: String?) async throws -> AgentRunResult

    /// Get agent communication logs
    func getAgentLogs(agentName: String?, limit: Int) async throws -> [AgentLog]

    /// Remove an agent
    func removeAgent(name: String) async throws

    /// Get remote sub-agents from an ACP agent
    func getRemoteAgents(agentName: String) async throws -> [RemoteAgentInfo]

    /// Update an agent's configuration
    func updateAgent(name: String, subAgent: String?, enabled: Bool?, description: String?, workdir: String?) async throws

    // MARK: - Gallery

    /// Get all available agents from the gallery
    func getGallery(featured: Bool) async throws -> [GalleryEntry]

    /// Install an agent from the gallery
    func installGalleryAgent(id: String, name: String?, workdir: String?, port: Int?) async throws -> GalleryInstallResponse

    // MARK: - Contexts

    /// Get all contexts
    func getContexts() async throws -> [Context]

    /// Get context details including servers and tools
    func getContextDetail(name: String) async throws -> ContextDetail

    /// Create a new context
    func createContext(name: String, description: String?) async throws -> Context

    /// Update a context's description
    func updateContext(name: String, description: String) async throws

    /// Delete a context
    func deleteContext(name: String) async throws

    /// Set a context as the default
    func setDefaultContext(name: String) async throws

    /// Get connection info for a context
    func getContextConnectInfo(name: String) async throws -> ContextConnectInfo

    /// Get servers in a context
    func getContextServers(contextName: String) async throws -> [ContextServer]

    /// Enable/disable a server in a context
    func setServerEnabledInContext(contextName: String, serverName: String, enabled: Bool) async throws

    /// Remove a server from a context
    func removeServerFromContext(contextName: String, serverName: String) async throws

    /// Get tools for a server in a context
    func getContextServerTools(contextName: String, serverName: String) async throws -> [ContextTool]

    /// Enable/disable a specific tool in a context
    func setToolEnabledInContext(contextName: String, serverName: String, toolName: String, enabled: Bool) async throws

    /// Bulk update tool enabled states
    func bulkSetToolsEnabled(contextName: String, serverName: String, tools: [String: Bool]) async throws

    /// Sync tools from running MCP servers to a context
    func syncContextTools(contextName: String) async throws -> Int

    /// Get available servers that can be added to a context
    func getAvailableServers(contextName: String) async throws -> [AvailableServer]

    /// Add a server to a context
    func addServerToContext(contextName: String, serverName: String, enabled: Bool) async throws

    // MARK: - Providers

    /// Get all providers, optionally filtered by type
    func getProviders(type: ProviderType?) async throws -> [Provider]

    /// Get provider templates
    func getProviderTemplates() async throws -> [ProviderTemplate]

    /// Get a specific provider by ID
    func getProvider(id: Int64) async throws -> Provider

    /// Create a new provider
    func createProvider(name: String, service: String, config: [String: Any], authConfig: [String: Any]?) async throws -> Provider

    /// Update a provider
    func updateProvider(id: Int64, name: String?, config: [String: Any]?, authConfig: [String: Any]?) async throws -> Provider

    /// Delete a provider
    func deleteProvider(id: Int64) async throws

    /// Enable a provider
    func enableProvider(id: Int64) async throws -> Provider

    /// Disable a provider
    func disableProvider(id: Int64) async throws -> Provider

    /// Set a provider as the default for its type
    func setDefaultProvider(id: Int64) async throws -> Provider

    /// Test a provider's connectivity
    func testProvider(id: Int64) async throws -> ProviderTestResult

    /// List available models for a service
    func listModels(service: String, projectID: String?) async throws -> [AvailableModel]

    /// Get model info by provider and model ID
    func getModelInfo(provider: String, modelID: String) async throws -> AvailableModel

    // MARK: - Usage

    /// Get usage records with optional filtering
    func getUsage(from: Date?, to: Date?, limit: Int, service: String?, providerID: Int64?) async throws -> UsageResponse

    /// Get usage summary (aggregated by provider/model)
    func getUsageSummary(from: Date?, to: Date?) async throws -> UsageSummaryResponse

    // MARK: - Google OAuth

    /// Get Google authentication status
    func getGoogleAuthStatus(account: String) async throws -> GoogleAuthStatus

    /// Start Google OAuth device flow
    func startGoogleAuth(account: String?, scopes: [String]?) async throws -> GoogleDeviceCodeResponse

    /// Poll for Google OAuth token
    func pollGoogleAuth(account: String, deviceCode: String, interval: Int) async throws -> GoogleAuthPollResponse

    /// Delete Google OAuth token
    func deleteGoogleAuth(account: String) async throws
}
