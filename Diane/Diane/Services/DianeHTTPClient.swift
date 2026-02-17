import Foundation
import os.log

private let logger = Logger(subsystem: "com.diane.ios", category: "DianeHTTPClient")

/// Creates a JSONDecoder configured for Go's time.Time format
/// (Duplicated from DianeClient.swift since that file is macOS-only)
private func makeGoCompatibleDecoderHTTP() -> JSONDecoder {
    let decoder = JSONDecoder()
    decoder.dateDecodingStrategy = .custom { decoder in
        let container = try decoder.singleValueContainer()
        let dateString = try container.decode(String.self)

        // Try ISO8601 with fractional seconds first (Go's default format)
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        if let date = formatter.date(from: dateString) {
            return date
        }

        // Fallback to without fractional seconds
        formatter.formatOptions = [.withInternetDateTime]
        if let date = formatter.date(from: dateString) {
            return date
        }

        throw DecodingError.dataCorruptedError(in: container, debugDescription: "Cannot decode date: \(dateString)")
    }
    return decoder
}

// MARK: - Error Types

enum DianeHTTPClientError: LocalizedError {
    case readOnlyMode
    case connectionFailed(underlying: Error)
    case decodingFailed(underlying: Error)
    case serverError(statusCode: Int, body: String)
    case invalidURL(path: String)
    case notAvailableOnIOS

    var errorDescription: String? {
        switch self {
        case .readOnlyMode:
            return "This operation is not available in read-only mode"
        case .connectionFailed(let error):
            return "Connection failed: \(error.localizedDescription)"
        case .decodingFailed(let error):
            return "Failed to decode response: \(error.localizedDescription)"
        case .serverError(let code, let body):
            return "Server error (\(code)): \(body)"
        case .invalidURL(let path):
            return "Invalid URL path: \(path)"
        case .notAvailableOnIOS:
            return "This operation is not available on iOS"
        }
    }
}

// MARK: - HTTP Client

/// URLSession-based HTTP client that connects to Diane server over TCP.
/// Supports optional API key authentication via Bearer token.
@MainActor
class DianeHTTPClient: DianeClientProtocol {
    private(set) var baseURL: URL
    private(set) var apiKey: String?
    private let session: URLSession

    init(baseURL: URL, apiKey: String? = nil) {
        self.baseURL = baseURL
        self.apiKey = apiKey

        let config = URLSessionConfiguration.default
        config.timeoutIntervalForRequest = 10
        config.timeoutIntervalForResource = 30
        self.session = URLSession(configuration: config)

        logger.info("DianeHTTPClient initialized with base URL: \(baseURL.absoluteString), apiKey: \(apiKey != nil ? "set" : "none")")
    }

    /// Update the base URL for server address changes
    func updateBaseURL(_ url: URL) {
        self.baseURL = url
        logger.info("Base URL updated to: \(url.absoluteString)")
    }
    
    /// Update the API key
    func updateAPIKey(_ key: String?) {
        self.apiKey = key
        logger.info("API key updated: \(key != nil ? "set" : "cleared")")
    }

    // MARK: - Private Helpers

    /// Make an HTTP GET request and return the response data
    private func request(_ path: String) async throws -> Data {
        guard let url = URL(string: path, relativeTo: baseURL) else {
            throw DianeHTTPClientError.invalidURL(path: path)
        }

        var urlRequest = URLRequest(url: url)
        urlRequest.httpMethod = "GET"
        
        // Add API key auth header if configured
        if let apiKey, !apiKey.isEmpty {
            urlRequest.setValue("Bearer \(apiKey)", forHTTPHeaderField: "Authorization")
        }

        logger.info("GET \(url.absoluteString)")

        let data: Data
        let response: URLResponse
        do {
            (data, response) = try await session.data(for: urlRequest)
        } catch {
            logger.error("Connection failed for \(path): \(error.localizedDescription)")
            throw DianeHTTPClientError.connectionFailed(underlying: error)
        }

        guard let httpResponse = response as? HTTPURLResponse else {
            throw DianeHTTPClientError.connectionFailed(underlying: URLError(.badServerResponse))
        }

        guard (200...299).contains(httpResponse.statusCode) else {
            let body = String(data: data, encoding: .utf8) ?? ""
            logger.error("Server error \(httpResponse.statusCode) for \(path): \(body)")
            throw DianeHTTPClientError.serverError(statusCode: httpResponse.statusCode, body: body)
        }

        logger.info("GET \(path) succeeded, \(data.count) bytes")
        return data
    }
    
    /// Make an HTTP POST/PUT/DELETE request with a body and return the response data
    private func requestWithBody(_ path: String, method: String, body: Data? = nil) async throws -> Data {
        guard let url = URL(string: path, relativeTo: baseURL) else {
            throw DianeHTTPClientError.invalidURL(path: path)
        }

        var urlRequest = URLRequest(url: url)
        urlRequest.httpMethod = method
        
        // Add API key auth header if configured
        if let apiKey, !apiKey.isEmpty {
            urlRequest.setValue("Bearer \(apiKey)", forHTTPHeaderField: "Authorization")
        }
        
        // Add body if provided
        if let body = body {
            urlRequest.httpBody = body
            urlRequest.setValue("application/json", forHTTPHeaderField: "Content-Type")
        }

        logger.info("\(method) \(url.absoluteString)")

        let data: Data
        let response: URLResponse
        do {
            (data, response) = try await session.data(for: urlRequest)
        } catch {
            logger.error("Connection failed for \(path): \(error.localizedDescription)")
            throw DianeHTTPClientError.connectionFailed(underlying: error)
        }

        guard let httpResponse = response as? HTTPURLResponse else {
            throw DianeHTTPClientError.connectionFailed(underlying: URLError(.badServerResponse))
        }

        guard (200...299).contains(httpResponse.statusCode) else {
            let body = String(data: data, encoding: .utf8) ?? ""
            logger.error("Server error \(httpResponse.statusCode) for \(path): \(body)")
            throw DianeHTTPClientError.serverError(statusCode: httpResponse.statusCode, body: body)
        }

        logger.info("\(method) \(path) succeeded, \(data.count) bytes")
        return data
    }

    /// Decode data using the Go-compatible decoder, wrapping errors
    private func decodeGo<T: Decodable>(_ type: T.Type, from data: Data) throws -> T {
        do {
            return try makeGoCompatibleDecoderHTTP().decode(type, from: data)
        } catch {
            throw DianeHTTPClientError.decodingFailed(underlying: error)
        }
    }

    /// Decode data using a plain JSONDecoder, wrapping errors
    private func decode<T: Decodable>(_ type: T.Type, from data: Data) throws -> T {
        do {
            return try JSONDecoder().decode(type, from: data)
        } catch {
            throw DianeHTTPClientError.decodingFailed(underlying: error)
        }
    }

    // MARK: - Process Management (not available on iOS)

    var socketExists: Bool { false }

    func getPID() -> Int? { nil }

    func isProcessRunning() -> Bool { false }

    func startDiane() throws {
        throw DianeHTTPClientError.notAvailableOnIOS
    }

    func stopDiane() throws {
        throw DianeHTTPClientError.notAvailableOnIOS
    }

    func restartDiane() async throws {
        throw DianeHTTPClientError.notAvailableOnIOS
    }

    func sendReloadSignal() throws {
        throw DianeHTTPClientError.notAvailableOnIOS
    }

    // MARK: - Health & Status

    func health() async -> Bool {
        do {
            _ = try await request("/health")
            return true
        } catch {
            return false
        }
    }

    func getStatus() async throws -> DianeStatus {
        let data = try await request("/status")
        return try decodeGo(DianeStatus.self, from: data)
    }

    func reloadConfig() async throws {
        throw DianeHTTPClientError.readOnlyMode
    }

    // MARK: - MCP Servers (Runtime)

    func getMCPServers() async throws -> [MCPServerStatus] {
        let data = try await request("/mcp-servers")
        return try decode([MCPServerStatus].self, from: data)
    }

    func restartMCPServer(name: String) async throws {
        throw DianeHTTPClientError.readOnlyMode
    }

    // MARK: - MCP Server Configuration

    func getMCPServerConfigs() async throws -> [MCPServer] {
        let data = try await request("/mcp-servers-config")
        return try decodeGo([MCPServer].self, from: data)
    }

    func getMCPServerConfig(id: Int64) async throws -> MCPServer {
        let data = try await request("/mcp-servers-config/\(id)")
        return try decodeGo(MCPServer.self, from: data)
    }

    func createMCPServerConfig(name: String, type: String, enabled: Bool, command: String?, args: [String]?, env: [String: String]?, url: String?, headers: [String: String]?, oauth: OAuthConfig?, nodeID: String?, nodeMode: String?) async throws -> MCPServer {
        throw DianeHTTPClientError.readOnlyMode
    }

    func updateMCPServerConfig(id: Int64, name: String?, type: String?, enabled: Bool?, command: String?, args: [String]?, env: [String: String]?, url: String?, headers: [String: String]?, oauth: OAuthConfig?, nodeID: String?, nodeMode: String?) async throws -> MCPServer {
        throw DianeHTTPClientError.readOnlyMode
    }

    func deleteMCPServerConfig(id: Int64) async throws {
        throw DianeHTTPClientError.readOnlyMode
    }

    // MARK: - Tools

    func getTools() async throws -> [ToolInfo] {
        let data = try await request("/tools")
        return try decode([ToolInfo].self, from: data)
    }
    
    func getPrompts() async throws -> [PromptInfo] {
        let data = try await request("/prompts")
        return try decode([PromptInfo].self, from: data)
    }
    
    func getResources() async throws -> [ResourceInfo] {
        let data = try await request("/resources")
        return try decode([ResourceInfo].self, from: data)
    }
    
    func getPromptContent(server: String, name: String) async throws -> PromptContentResponse {
        let encodedServer = server.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? server
        let encodedName = name.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? name
        let data = try await request("/prompts/get?server=\(encodedServer)&name=\(encodedName)")
        return try decode(PromptContentResponse.self, from: data)
    }
    
    func getResourceContent(server: String, uri: String) async throws -> ResourceContentResponse {
        let encodedServer = server.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? server
        let encodedURI = uri.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? uri
        let data = try await request("/resources/read?server=\(encodedServer)&uri=\(encodedURI)")
        return try decode(ResourceContentResponse.self, from: data)
    }

    // MARK: - OAuth (read-only: no login/poll)

    func startAuth(serverName: String) async throws -> DeviceCodeInfo {
        throw DianeHTTPClientError.readOnlyMode
    }

    func pollAuth(serverName: String, deviceCode: String, interval: Int) async throws {
        throw DianeHTTPClientError.readOnlyMode
    }

    // MARK: - Scheduler

    func getJobs() async throws -> [Job] {
        let data = try await request("/jobs")
        return try decodeGo([Job].self, from: data)
    }

    func getJobLogs(jobName: String?, limit: Int) async throws -> [JobExecution] {
        var path = "/jobs/logs?limit=\(limit)"
        if let jobName = jobName {
            let encoded = jobName.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? jobName
            path += "&job_name=\(encoded)"
        }
        let data = try await request(path)
        return try decodeGo([JobExecution].self, from: data)
    }

    func toggleJob(name: String, enabled: Bool) async throws {
        throw DianeHTTPClientError.readOnlyMode
    }

    // MARK: - Agents

    func getAgents() async throws -> [AgentConfig] {
        let data = try await request("/agents")
        return try decode([AgentConfig].self, from: data)
    }

    func getAgent(name: String) async throws -> AgentConfig {
        let encodedName = name.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? name
        let data = try await request("/agents/\(encodedName)")
        return try decode(AgentConfig.self, from: data)
    }

    func testAgent(name: String) async throws -> AgentTestResult {
        throw DianeHTTPClientError.readOnlyMode
    }

    func toggleAgent(name: String, enabled: Bool) async throws {
        throw DianeHTTPClientError.readOnlyMode
    }

    func runAgentPrompt(agentName: String, prompt: String, remoteAgentName: String?) async throws -> AgentRunResult {
        throw DianeHTTPClientError.readOnlyMode
    }

    func getAgentLogs(agentName: String?, limit: Int) async throws -> [AgentLog] {
        var path = "/agents/logs?limit=\(limit)"
        if let agentName = agentName {
            let encoded = agentName.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? agentName
            path += "&agent_name=\(encoded)"
        }
        let data = try await request(path)
        return try decodeGo([AgentLog].self, from: data)
    }

    func removeAgent(name: String) async throws {
        throw DianeHTTPClientError.readOnlyMode
    }

    func getRemoteAgents(agentName: String) async throws -> [RemoteAgentInfo] {
        let encodedName = agentName.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? agentName
        let data = try await request("/agents/\(encodedName)/remote-agents")
        return try decode([RemoteAgentInfo].self, from: data)
    }

    func updateAgent(name: String, subAgent: String?, enabled: Bool?, description: String?, workdir: String?) async throws {
        throw DianeHTTPClientError.readOnlyMode
    }

    // MARK: - Gallery (read-only: no install)

    func getGallery(featured: Bool) async throws -> [GalleryEntry] {
        let path = featured ? "/gallery?featured=true" : "/gallery"
        let data = try await request(path)
        return try decode([GalleryEntry].self, from: data)
    }

    func installGalleryAgent(id: String, name: String?, workdir: String?, port: Int?) async throws -> GalleryInstallResponse {
        throw DianeHTTPClientError.readOnlyMode
    }

    // MARK: - Contexts

    func getContexts() async throws -> [Context] {
        let data = try await request("/contexts")
        return try decode([Context].self, from: data)
    }

    func getContextDetail(name: String) async throws -> ContextDetail {
        let encodedName = name.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? name
        let data = try await request("/contexts/\(encodedName)")
        return try decode(ContextDetail.self, from: data)
    }

    func createContext(name: String, description: String?) async throws -> Context {
        throw DianeHTTPClientError.readOnlyMode
    }

    func updateContext(name: String, description: String) async throws {
        throw DianeHTTPClientError.readOnlyMode
    }

    func deleteContext(name: String) async throws {
        throw DianeHTTPClientError.readOnlyMode
    }

    func setDefaultContext(name: String) async throws {
        throw DianeHTTPClientError.readOnlyMode
    }

    func getContextConnectInfo(name: String) async throws -> ContextConnectInfo {
        let encodedName = name.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? name
        let data = try await request("/contexts/\(encodedName)/connect")
        return try decode(ContextConnectInfo.self, from: data)
    }

    func getContextServers(contextName: String) async throws -> [ContextServer] {
        let encodedName = contextName.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? contextName
        let data = try await request("/contexts/\(encodedName)/servers")
        return try decode([ContextServer].self, from: data)
    }

    func setServerEnabledInContext(contextName: String, serverName: String, enabled: Bool) async throws {
        throw DianeHTTPClientError.readOnlyMode
    }

    func removeServerFromContext(contextName: String, serverName: String) async throws {
        throw DianeHTTPClientError.readOnlyMode
    }

    func getContextServerTools(contextName: String, serverName: String) async throws -> [ContextTool] {
        let encodedContext = contextName.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? contextName
        let encodedServer = serverName.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? serverName
        let data = try await request("/contexts/\(encodedContext)/servers/\(encodedServer)/tools")
        return try decode([ContextTool].self, from: data)
    }

    func setToolEnabledInContext(contextName: String, serverName: String, toolName: String, enabled: Bool) async throws {
        throw DianeHTTPClientError.readOnlyMode
    }

    func bulkSetToolsEnabled(contextName: String, serverName: String, tools: [String: Bool]) async throws {
        throw DianeHTTPClientError.readOnlyMode
    }

    func syncContextTools(contextName: String) async throws -> Int {
        throw DianeHTTPClientError.readOnlyMode
    }

    func getAvailableServers(contextName: String) async throws -> [AvailableServer] {
        let encodedName = contextName.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? contextName
        let data = try await request("/contexts/\(encodedName)/available-servers")
        return try decodeGo([AvailableServer].self, from: data)
    }

    func addServerToContext(contextName: String, serverName: String, enabled: Bool) async throws {
        throw DianeHTTPClientError.readOnlyMode
    }

    // MARK: - Providers

    func getProviders(type: ProviderType?) async throws -> [Provider] {
        var path = "/providers"
        if let type = type {
            path += "?type=\(type.rawValue)"
        }
        let data = try await request(path)
        return try decodeGo([Provider].self, from: data)
    }

    func getProviderTemplates() async throws -> [ProviderTemplate] {
        let data = try await request("/providers/templates")
        return try decode([ProviderTemplate].self, from: data)
    }

    func getProvider(id: Int64) async throws -> Provider {
        let data = try await request("/providers/\(id)")
        return try decodeGo(Provider.self, from: data)
    }

    func createProvider(name: String, service: String, config: [String: Any], authConfig: [String: Any]?) async throws -> Provider {
        throw DianeHTTPClientError.readOnlyMode
    }

    func updateProvider(id: Int64, name: String?, config: [String: Any]?, authConfig: [String: Any]?) async throws -> Provider {
        throw DianeHTTPClientError.readOnlyMode
    }

    func deleteProvider(id: Int64) async throws {
        throw DianeHTTPClientError.readOnlyMode
    }

    func enableProvider(id: Int64) async throws -> Provider {
        throw DianeHTTPClientError.readOnlyMode
    }

    func disableProvider(id: Int64) async throws -> Provider {
        throw DianeHTTPClientError.readOnlyMode
    }

    func setDefaultProvider(id: Int64) async throws -> Provider {
        throw DianeHTTPClientError.readOnlyMode
    }

    func testProvider(id: Int64) async throws -> ProviderTestResult {
        throw DianeHTTPClientError.readOnlyMode
    }

    func listModels(service: String, projectID: String?) async throws -> [AvailableModel] {
        throw DianeHTTPClientError.readOnlyMode
    }

    func getModelInfo(provider: String, modelID: String) async throws -> AvailableModel {
        let data = try await request("/models/\(provider)/\(modelID)")
        return try decodeGo(AvailableModel.self, from: data)
    }

    // MARK: - Usage

    func getUsage(from: Date?, to: Date?, limit: Int, service: String?, providerID: Int64?) async throws -> UsageResponse {
        var path = "/usage?limit=\(limit)"

        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime]

        if let from = from {
            path += "&from=\(formatter.string(from: from))"
        }
        if let to = to {
            path += "&to=\(formatter.string(from: to))"
        }
        if let service = service {
            path += "&service=\(service)"
        }
        if let providerID = providerID {
            path += "&provider_id=\(providerID)"
        }

        let data = try await request(path)
        return try decodeGo(UsageResponse.self, from: data)
    }

    func getUsageSummary(from: Date?, to: Date?) async throws -> UsageSummaryResponse {
        var path = "/usage/summary"
        var queryParams: [String] = []

        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime]

        if let from = from {
            queryParams.append("from=\(formatter.string(from: from))")
        }
        if let to = to {
            queryParams.append("to=\(formatter.string(from: to))")
        }

        if !queryParams.isEmpty {
            path += "?" + queryParams.joined(separator: "&")
        }

        let data = try await request(path)
        return try decodeGo(UsageSummaryResponse.self, from: data)
    }

    // MARK: - Google OAuth (read-only: no auth actions)

    func getGoogleAuthStatus(account: String) async throws -> GoogleAuthStatus {
        let data = try await request("/google/auth?account=\(account)")
        return try decode(GoogleAuthStatus.self, from: data)
    }

    func startGoogleAuth(account: String?, scopes: [String]?) async throws -> GoogleDeviceCodeResponse {
        throw DianeHTTPClientError.readOnlyMode
    }

    func pollGoogleAuth(account: String, deviceCode: String, interval: Int) async throws -> GoogleAuthPollResponse {
        throw DianeHTTPClientError.readOnlyMode
    }

    func deleteGoogleAuth(account: String) async throws {
        throw DianeHTTPClientError.readOnlyMode
    }
    
    // MARK: - Slave Management
    
    func getSlaves() async throws -> [SlaveInfo] {
        let data = try await request("/slaves")
        return try JSONDecoder().decode([SlaveInfo].self, from: data)
    }
    
    func getPendingPairingRequests() async throws -> [PairingRequest] {
        let data = try await request("/slaves/pending")
        return try JSONDecoder().decode([PairingRequest].self, from: data)
    }
    
    func approvePairingRequest(hostname: String, pairingCode: String) async throws {
        let body: [String: Any] = [
            "hostname": hostname,
            "pairing_code": pairingCode
        ]
        let bodyData = try JSONSerialization.data(withJSONObject: body)
        _ = try await requestWithBody("/slaves/approve", method: "POST", body: bodyData)
    }
    
    func denyPairingRequest(hostname: String, pairingCode: String) async throws {
        let body: [String: Any] = [
            "hostname": hostname,
            "pairing_code": pairingCode
        ]
        let bodyData = try JSONSerialization.data(withJSONObject: body)
        _ = try await requestWithBody("/slaves/deny", method: "POST", body: bodyData)
    }
    
    func revokeSlaveCredentials(hostname: String, reason: String?) async throws {
        var body: [String: Any] = ["hostname": hostname]
        if let reason = reason {
            body["reason"] = reason
        }
        let bodyData = try JSONSerialization.data(withJSONObject: body)
        _ = try await requestWithBody("/slaves/revoke", method: "POST", body: bodyData)
    }
    
    func restartSlave(hostname: String) async throws {
        _ = try await requestWithBody("/slaves/restart/\(hostname)", method: "POST", body: Data())
    }
    
    func upgradeSlave(hostname: String) async throws {
        _ = try await requestWithBody("/slaves/upgrade/\(hostname)", method: "POST", body: Data())
    }
}

// MARK: - SwiftUI Environment Support

import SwiftUI

private struct DianeHTTPClientKey: EnvironmentKey {
    static let defaultValue: DianeHTTPClient? = nil
}

extension EnvironmentValues {
    var dianeClient: DianeHTTPClient? {
        get { self[DianeHTTPClientKey.self] }
        set { self[DianeHTTPClientKey.self] = newValue }
    }
}
