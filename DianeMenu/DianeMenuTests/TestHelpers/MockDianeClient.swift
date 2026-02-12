import Foundation
#if !COMPONENT_CATALOG
@testable import DianeMenu
#endif

/// Mock implementation of `DianeClientProtocol` for unit testing.
///
/// Tracks method invocations, supports configurable return values, and can
/// simulate errors on demand. Thread-safe via `@MainActor` (tests run on main).
@MainActor
final class MockDianeClient: DianeClientProtocol {

    // MARK: - Configuration

    /// Servers returned by `getMCPServerConfigs()`.
    var serverConfigs: [MCPServer] = []

    /// Jobs returned by `getJobs()`.
    var jobs: [Job] = []

    /// Executions returned by `getJobLogs()`.
    var jobExecutions: [JobExecution] = []

    /// Usage response returned by `getUsage()`.
    var usageResponse: UsageResponse?

    /// Usage summary response returned by `getUsageSummary()`.
    var usageSummaryResponse: UsageSummaryResponse?

    /// Tools returned by `getTools()`.
    var toolsList: [ToolInfo] = []

    /// When non-nil, async methods throw this error instead of returning data.
    var errorToThrow: Error?

    /// Counter for every method call, keyed by method name.
    var methodCallCounts: [String: Int] = [:]

    /// The last arguments passed to `createMCPServerConfig`.
    var lastCreateArgs: (name: String, type: String, enabled: Bool, command: String?, args: [String]?, env: [String: String]?, url: String?, headers: [String: String]?, oauth: OAuthConfig?)?

    /// The last arguments passed to `updateMCPServerConfig`.
    var lastUpdateArgs: (id: Int64, name: String?, type: String?, enabled: Bool?, command: String?, args: [String]?, env: [String: String]?, url: String?, headers: [String: String]?, oauth: OAuthConfig?)?

    /// Auto-incrementing ID for created servers.
    private var nextID: Int64 = 1000

    // MARK: - Call Tracking

    private func record(_ method: String) {
        methodCallCounts[method, default: 0] += 1
    }

    func callCount(for method: String) -> Int {
        methodCallCounts[method, default: 0]
    }

    // MARK: - Error Helper

    private func throwIfNeeded() throws {
        if let error = errorToThrow {
            throw error
        }
    }

    // MARK: - Process Management

    var socketExists: Bool { true }
    func getPID() -> Int? { record("getPID"); return 12345 }
    func isProcessRunning() -> Bool { record("isProcessRunning"); return true }
    func startDiane() throws { record("startDiane"); try throwIfNeeded() }
    func stopDiane() throws { record("stopDiane"); try throwIfNeeded() }
    func restartDiane() async throws { record("restartDiane"); try throwIfNeeded() }
    func sendReloadSignal() throws { record("sendReloadSignal"); try throwIfNeeded() }

    // MARK: - Health & Status

    func health() async -> Bool { record("health"); return true }

    func getStatus() async throws -> DianeStatus {
        record("getStatus")
        try throwIfNeeded()
        // Return a minimal status — tests that need specific status should set up their own mock
        fatalError("getStatus not stubbed — set up a specific mock if you need this")
    }

    func reloadConfig() async throws { record("reloadConfig"); try throwIfNeeded() }

    // MARK: - MCP Servers (Runtime)

    func getMCPServers() async throws -> [MCPServerStatus] {
        record("getMCPServers"); try throwIfNeeded(); return []
    }

    func restartMCPServer(name: String) async throws {
        record("restartMCPServer"); try throwIfNeeded()
    }

    // MARK: - MCP Server Configuration

    func getMCPServerConfigs() async throws -> [MCPServer] {
        record("getMCPServerConfigs")
        try throwIfNeeded()
        return serverConfigs
    }

    func getMCPServerConfig(id: Int64) async throws -> MCPServer {
        record("getMCPServerConfig")
        try throwIfNeeded()
        guard let server = serverConfigs.first(where: { $0.id == id }) else {
            throw MockError.notFound
        }
        return server
    }

    func createMCPServerConfig(
        name: String, type: String, enabled: Bool,
        command: String?, args: [String]?, env: [String: String]?,
        url: String?, headers: [String: String]?, oauth: OAuthConfig?
    ) async throws -> MCPServer {
        record("createMCPServerConfig")
        try throwIfNeeded()
        lastCreateArgs = (name, type, enabled, command, args, env, url, headers, oauth)

        let now = Date()
        let server = MCPServer(
            id: nextID,
            name: name,
            enabled: enabled,
            type: type,
            command: command,
            args: args,
            env: env,
            url: url,
            headers: headers,
            oauth: oauth,
            createdAt: now,
            updatedAt: now
        )
        nextID += 1
        serverConfigs.append(server)
        return server
    }

    func updateMCPServerConfig(
        id: Int64, name: String?, type: String?, enabled: Bool?,
        command: String?, args: [String]?, env: [String: String]?,
        url: String?, headers: [String: String]?, oauth: OAuthConfig?
    ) async throws -> MCPServer {
        record("updateMCPServerConfig")
        try throwIfNeeded()
        lastUpdateArgs = (id, name, type, enabled, command, args, env, url, headers, oauth)

        guard let index = serverConfigs.firstIndex(where: { $0.id == id }) else {
            throw MockError.notFound
        }
        var server = serverConfigs[index]
        if let name = name { server.name = name }
        if let enabled = enabled { server.enabled = enabled }
        serverConfigs[index] = server
        return server
    }

    func deleteMCPServerConfig(id: Int64) async throws {
        record("deleteMCPServerConfig")
        try throwIfNeeded()
        serverConfigs.removeAll { $0.id == id }
    }

    // MARK: - Tools

    func getTools() async throws -> [ToolInfo] {
        record("getTools"); try throwIfNeeded(); return toolsList
    }

    // MARK: - OAuth

    func startAuth(serverName: String) async throws -> DeviceCodeInfo {
        record("startAuth"); try throwIfNeeded()
        fatalError("startAuth not stubbed")
    }

    func pollAuth(serverName: String, deviceCode: String, interval: Int) async throws {
        record("pollAuth"); try throwIfNeeded()
    }

    // MARK: - Scheduler

    func getJobs() async throws -> [Job] {
        record("getJobs"); try throwIfNeeded(); return jobs
    }

    func getJobLogs(jobName: String?, limit: Int) async throws -> [JobExecution] {
        record("getJobLogs"); try throwIfNeeded()
        if let jobName = jobName {
            return jobExecutions.filter { $0.jobName == jobName }
        }
        return jobExecutions
    }

    func toggleJob(name: String, enabled: Bool) async throws {
        record("toggleJob"); try throwIfNeeded()
        if let index = jobs.firstIndex(where: { $0.name == name }) {
            // Jobs use let for enabled, so we replace the whole job
            let old = jobs[index]
            jobs[index] = Job(
                id: old.id, name: old.name, command: old.command,
                schedule: old.schedule, enabled: enabled,
                actionType: old.actionType, agentName: old.agentName,
                createdAt: old.createdAt, updatedAt: old.updatedAt
            )
        }
    }

    // MARK: - Agents

    /// Agents returned by `getAgents()`.
    var agentsList: [AgentConfig] = []

    /// Test results returned by `testAgent(name:)`, keyed by agent name.
    var agentTestResults: [String: AgentTestResult] = [:]

    /// Logs returned by `getAgentLogs()`.
    var agentLogs: [AgentLog] = []

    /// Remote agents returned by `getRemoteAgents()`.
    var remoteAgentsList: [RemoteAgentInfo] = []

    /// Gallery entries returned by `getGallery()`.
    var galleryEntriesList: [GalleryEntry] = []

    /// Install response returned by `installGalleryAgent()`.
    var installResponse: GalleryInstallResponse?

    /// Run result returned by `runAgentPrompt()`.
    var agentRunResultToReturn: AgentRunResult?

    /// Last args passed to `updateAgent`.
    var lastUpdateAgentArgs: (name: String, subAgent: String?, enabled: Bool?, description: String?, workdir: String?)?

    /// Last args passed to `installGalleryAgent`.
    var lastInstallAgentArgs: (id: String, name: String?, workdir: String?, port: Int?)?

    func getAgents() async throws -> [AgentConfig] {
        record("getAgents"); try throwIfNeeded()
        return agentsList
    }

    func getAgent(name: String) async throws -> AgentConfig {
        record("getAgent"); try throwIfNeeded()
        guard let agent = agentsList.first(where: { $0.name == name }) else {
            throw MockError.notFound
        }
        return agent
    }

    func testAgent(name: String) async throws -> AgentTestResult {
        record("testAgent"); try throwIfNeeded()
        guard let result = agentTestResults[name] else {
            throw MockError.notFound
        }
        return result
    }

    func toggleAgent(name: String, enabled: Bool) async throws {
        record("toggleAgent"); try throwIfNeeded()
        // AgentConfig uses `let enabled`, so we must replace the whole entry.
        // Since AgentConfig has no CodingKeys-free memberwise init issue,
        // we reconstruct via the auto-generated memberwise init.
        if let index = agentsList.firstIndex(where: { $0.name == name }) {
            let old = agentsList[index]
            agentsList[index] = AgentConfig(
                name: old.name, url: old.url, type: old.type,
                command: old.command, args: old.args, env: old.env,
                workdir: old.workdir, port: old.port, subAgent: old.subAgent,
                enabled: enabled, description: old.description, tags: old.tags
            )
        }
    }

    func runAgentPrompt(agentName: String, prompt: String, remoteAgentName: String?) async throws -> AgentRunResult {
        record("runAgentPrompt"); try throwIfNeeded()
        guard let result = agentRunResultToReturn else {
            throw MockError.notFound
        }
        return result
    }

    func getAgentLogs(agentName: String?, limit: Int) async throws -> [AgentLog] {
        record("getAgentLogs"); try throwIfNeeded()
        if let agentName = agentName {
            return agentLogs.filter { $0.agentName == agentName }
        }
        return agentLogs
    }

    func removeAgent(name: String) async throws {
        record("removeAgent"); try throwIfNeeded()
        agentsList.removeAll { $0.name == name }
    }

    func getRemoteAgents(agentName: String) async throws -> [RemoteAgentInfo] {
        record("getRemoteAgents"); try throwIfNeeded()
        return remoteAgentsList
    }

    func updateAgent(name: String, subAgent: String?, enabled: Bool?, description: String?, workdir: String?) async throws {
        record("updateAgent"); try throwIfNeeded()
        lastUpdateAgentArgs = (name, subAgent, enabled, description, workdir)

        if let index = agentsList.firstIndex(where: { $0.name == name }) {
            let old = agentsList[index]
            agentsList[index] = AgentConfig(
                name: old.name, url: old.url, type: old.type,
                command: old.command, args: old.args, env: old.env,
                workdir: old.workdir, port: old.port,
                subAgent: subAgent ?? old.subAgent,
                enabled: enabled ?? old.enabled,
                description: description ?? old.description,
                tags: old.tags
            )
        }
    }

    // MARK: - Gallery

    func getGallery(featured: Bool) async throws -> [GalleryEntry] {
        record("getGallery"); try throwIfNeeded()
        if featured {
            return galleryEntriesList.filter { $0.featured }
        }
        return galleryEntriesList
    }

    func installGalleryAgent(id: String, name: String?, workdir: String?, port: Int?) async throws -> GalleryInstallResponse {
        record("installGalleryAgent"); try throwIfNeeded()
        lastInstallAgentArgs = (id, name, workdir, port)
        guard let response = installResponse else {
            throw MockError.notFound
        }
        return response
    }

    // MARK: - Contexts

    /// Contexts returned by `getContexts()`.
    var contextsList: [Context] = []

    /// Context details keyed by context name.
    var contextDetails: [String: ContextDetail] = [:]

    /// Connection info keyed by context name.
    var contextConnectInfoMap: [String: ContextConnectInfo] = [:]

    /// Available servers returned by `getAvailableServers()`.
    var availableServersList: [AvailableServer] = []

    /// Context server tools keyed by "\(contextName)/\(serverName)".
    var contextServerToolsMap: [String: [ContextTool]] = [:]

    /// Number of tools synced, returned by `syncContextTools()`.
    var syncedToolsCount: Int = 0

    /// Auto-incrementing ID for created contexts.
    private var nextContextID: Int64 = 3000

    /// Last args passed to `createContext`.
    var lastCreateContextArgs: (name: String, description: String?)?

    /// Last args passed to `addServerToContext`.
    var lastAddServerToContextArgs: (contextName: String, serverName: String, enabled: Bool)?

    /// Last args passed to `setServerEnabledInContext`.
    var lastSetServerEnabledArgs: (contextName: String, serverName: String, enabled: Bool)?

    /// Last args passed to `setToolEnabledInContext`.
    var lastSetToolEnabledArgs: (contextName: String, serverName: String, toolName: String, enabled: Bool)?

    /// Last args passed to `bulkSetToolsEnabled`.
    var lastBulkSetToolsArgs: (contextName: String, serverName: String, tools: [String: Bool])?

    func getContexts() async throws -> [Context] {
        record("getContexts"); try throwIfNeeded()
        return contextsList
    }

    func getContextDetail(name: String) async throws -> ContextDetail {
        record("getContextDetail"); try throwIfNeeded()
        guard let detail = contextDetails[name] else {
            throw MockError.notFound
        }
        return detail
    }

    func createContext(name: String, description: String?) async throws -> Context {
        record("createContext"); try throwIfNeeded()
        lastCreateContextArgs = (name, description)

        let context = Context(id: nextContextID, name: name, description: description, isDefault: false)
        nextContextID += 1
        contextsList.append(context)
        return context
    }

    func updateContext(name: String, description: String) async throws {
        record("updateContext"); try throwIfNeeded()
    }

    func deleteContext(name: String) async throws {
        record("deleteContext"); try throwIfNeeded()
        contextsList.removeAll { $0.name == name }
        contextDetails.removeValue(forKey: name)
    }

    func setDefaultContext(name: String) async throws {
        record("setDefaultContext"); try throwIfNeeded()
        // Rebuild the list: clear all defaults, then set the target
        var updated: [Context] = []
        for ctx in contextsList {
            updated.append(Context(id: ctx.id, name: ctx.name, description: ctx.description, isDefault: ctx.name == name))
        }
        contextsList = updated
    }

    func getContextConnectInfo(name: String) async throws -> ContextConnectInfo {
        record("getContextConnectInfo"); try throwIfNeeded()
        guard let info = contextConnectInfoMap[name] else {
            throw MockError.notFound
        }
        return info
    }

    func getContextServers(contextName: String) async throws -> [ContextServer] {
        record("getContextServers"); try throwIfNeeded()
        return contextDetails[contextName]?.servers ?? []
    }

    func setServerEnabledInContext(contextName: String, serverName: String, enabled: Bool) async throws {
        record("setServerEnabledInContext"); try throwIfNeeded()
        lastSetServerEnabledArgs = (contextName, serverName, enabled)
    }

    func removeServerFromContext(contextName: String, serverName: String) async throws {
        record("removeServerFromContext"); try throwIfNeeded()
    }

    func getContextServerTools(contextName: String, serverName: String) async throws -> [ContextTool] {
        record("getContextServerTools"); try throwIfNeeded()
        let key = "\(contextName)/\(serverName)"
        return contextServerToolsMap[key] ?? []
    }

    func setToolEnabledInContext(contextName: String, serverName: String, toolName: String, enabled: Bool) async throws {
        record("setToolEnabledInContext"); try throwIfNeeded()
        lastSetToolEnabledArgs = (contextName, serverName, toolName, enabled)
    }

    func bulkSetToolsEnabled(contextName: String, serverName: String, tools: [String: Bool]) async throws {
        record("bulkSetToolsEnabled"); try throwIfNeeded()
        lastBulkSetToolsArgs = (contextName, serverName, tools)
    }

    func syncContextTools(contextName: String) async throws -> Int {
        record("syncContextTools"); try throwIfNeeded()
        return syncedToolsCount
    }

    func getAvailableServers(contextName: String) async throws -> [AvailableServer] {
        record("getAvailableServers"); try throwIfNeeded()
        return availableServersList
    }

    func addServerToContext(contextName: String, serverName: String, enabled: Bool) async throws {
        record("addServerToContext"); try throwIfNeeded()
        lastAddServerToContextArgs = (contextName, serverName, enabled)
    }

    // MARK: - Providers

    /// Providers returned by `getProviders()` etc.
    var providersList: [Provider] = []

    /// Templates returned by `getProviderTemplates()`.
    var providerTemplates: [ProviderTemplate] = []

    /// Test result returned by `testProvider()`.
    var providerTestResult: ProviderTestResult?

    /// Models returned by `listModels()`.
    var availableModelsList: [AvailableModel] = []

    /// Google auth status returned by `getGoogleAuthStatus()`.
    var googleAuthStatusResult: GoogleAuthStatus?

    /// Device code response returned by `startGoogleAuth()`.
    var googleDeviceCodeResponse: GoogleDeviceCodeResponse?

    /// Poll response returned by `pollGoogleAuth()`.
    var googlePollResponse: GoogleAuthPollResponse?

    /// Last args passed to `createProvider`.
    var lastCreateProviderArgs: (name: String, service: String, config: [String: Any], authConfig: [String: Any]?)?

    /// Last args passed to `updateProvider`.
    var lastUpdateProviderArgs: (id: Int64, name: String?, config: [String: Any]?, authConfig: [String: Any]?)?

    /// Auto-incrementing ID for created providers.
    private var nextProviderID: Int64 = 2000

    func getProviders(type: ProviderType?) async throws -> [Provider] {
        record("getProviders"); try throwIfNeeded()
        if let type = type {
            return providersList.filter { $0.type == type }
        }
        return providersList
    }

    func getProviderTemplates() async throws -> [ProviderTemplate] {
        record("getProviderTemplates"); try throwIfNeeded()
        return providerTemplates
    }

    func getProvider(id: Int64) async throws -> Provider {
        record("getProvider"); try throwIfNeeded()
        guard let provider = providersList.first(where: { $0.id == id }) else {
            throw MockError.notFound
        }
        return provider
    }

    func createProvider(name: String, service: String, config: [String: Any], authConfig: [String: Any]?) async throws -> Provider {
        record("createProvider"); try throwIfNeeded()
        lastCreateProviderArgs = (name, service, config, authConfig)

        let now = Date()
        // Determine type from matching template
        let templateType = providerTemplates.first(where: { $0.service == service })?.type ?? .llm
        let templateAuth = providerTemplates.first(where: { $0.service == service })?.authType ?? .none
        let provider = Provider(
            id: nextProviderID,
            name: name,
            type: templateType,
            service: service,
            enabled: true,
            isDefault: false,
            authType: templateAuth,
            config: config.mapValues { AnyCodable($0) },
            createdAt: now,
            updatedAt: now
        )
        nextProviderID += 1
        providersList.append(provider)
        return provider
    }

    func updateProvider(id: Int64, name: String?, config: [String: Any]?, authConfig: [String: Any]?) async throws -> Provider {
        record("updateProvider"); try throwIfNeeded()
        lastUpdateProviderArgs = (id, name, config, authConfig)

        guard let index = providersList.firstIndex(where: { $0.id == id }) else {
            throw MockError.notFound
        }
        var provider = providersList[index]
        if let name = name { provider.name = name }
        providersList[index] = provider
        return provider
    }

    func deleteProvider(id: Int64) async throws {
        record("deleteProvider"); try throwIfNeeded()
        providersList.removeAll { $0.id == id }
    }

    func enableProvider(id: Int64) async throws -> Provider {
        record("enableProvider"); try throwIfNeeded()
        guard let index = providersList.firstIndex(where: { $0.id == id }) else {
            throw MockError.notFound
        }
        providersList[index].enabled = true
        return providersList[index]
    }

    func disableProvider(id: Int64) async throws -> Provider {
        record("disableProvider"); try throwIfNeeded()
        guard let index = providersList.firstIndex(where: { $0.id == id }) else {
            throw MockError.notFound
        }
        providersList[index].enabled = false
        return providersList[index]
    }

    func setDefaultProvider(id: Int64) async throws -> Provider {
        record("setDefaultProvider"); try throwIfNeeded()
        // Clear all defaults first, then set the target
        for i in providersList.indices {
            providersList[i].isDefault = false
        }
        guard let index = providersList.firstIndex(where: { $0.id == id }) else {
            throw MockError.notFound
        }
        providersList[index].isDefault = true
        return providersList[index]
    }

    func testProvider(id: Int64) async throws -> ProviderTestResult {
        record("testProvider"); try throwIfNeeded()
        return providerTestResult ?? ProviderTestResult(success: true, message: "OK", responseTimeMs: 42)
    }

    func listModels(service: String, projectID: String?) async throws -> [AvailableModel] {
        record("listModels"); try throwIfNeeded()
        return availableModelsList
    }

    func getModelInfo(provider: String, modelID: String) async throws -> AvailableModel {
        record("getModelInfo"); try throwIfNeeded()
        guard let model = availableModelsList.first(where: { $0.id == modelID }) else {
            throw MockError.notFound
        }
        return model
    }

    // MARK: - Usage

    func getUsage(from: Date?, to: Date?, limit: Int, service: String?, providerID: Int64?) async throws -> UsageResponse {
        record("getUsage"); try throwIfNeeded()
        return usageResponse ?? UsageResponse(records: [], totalCost: 0, from: from ?? Date(), to: to ?? Date())
    }

    func getUsageSummary(from: Date?, to: Date?) async throws -> UsageSummaryResponse {
        record("getUsageSummary"); try throwIfNeeded()
        return usageSummaryResponse ?? UsageSummaryResponse(summary: [], totalCost: 0, from: from ?? Date(), to: to ?? Date())
    }

    // MARK: - Google OAuth

    func getGoogleAuthStatus(account: String) async throws -> GoogleAuthStatus {
        record("getGoogleAuthStatus"); try throwIfNeeded()
        guard let status = googleAuthStatusResult else {
            throw MockError.notFound
        }
        return status
    }

    func startGoogleAuth(account: String?, scopes: [String]?) async throws -> GoogleDeviceCodeResponse {
        record("startGoogleAuth"); try throwIfNeeded()
        guard let response = googleDeviceCodeResponse else {
            throw MockError.notFound
        }
        return response
    }

    func pollGoogleAuth(account: String, deviceCode: String, interval: Int) async throws -> GoogleAuthPollResponse {
        record("pollGoogleAuth"); try throwIfNeeded()
        guard let response = googlePollResponse else {
            throw MockError.notFound
        }
        return response
    }

    func deleteGoogleAuth(account: String) async throws {
        record("deleteGoogleAuth"); try throwIfNeeded()
    }
}

// MARK: - Mock Errors

enum MockError: Error, LocalizedError {
    case notFound
    case networkFailure
    case serverError(String)

    var errorDescription: String? {
        switch self {
        case .notFound: return "Not found"
        case .networkFailure: return "Network failure"
        case .serverError(let msg): return msg
        }
    }
}
