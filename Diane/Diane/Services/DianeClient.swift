import Foundation
import Network
import os.log

private let logger = Logger(subsystem: "com.diane.Diane", category: "DianeClient")

/// Creates a JSONDecoder configured for Go's time.Time format
private func makeGoCompatibleDecoder() -> JSONDecoder {
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

/// Client for communicating with Diane via Unix socket
@MainActor
class DianeClient: DianeClientProtocol {
    private let socketPath: String
    private let session: URLSession
    
    /// Path to the bundled diane binary in the app bundle
    static var bundledBinaryPath: String? {
        Bundle.main.executableURL?.deletingLastPathComponent().appendingPathComponent("diane-server").path
    }
    
    /// Path to the symlink location (~/.diane/bin/diane)
    static var symlinkPath: String {
        FileManager.default.homeDirectoryForCurrentUser
            .appendingPathComponent(".diane/bin/diane").path
    }
    
    /// Get the best available binary path (bundled preferred, then symlink/installed)
    static var binaryPath: String {
        // First check if bundled binary exists
        if let bundled = bundledBinaryPath,
           FileManager.default.fileExists(atPath: bundled) {
            return bundled
        }
        // Fall back to ~/.diane/bin/diane
        return symlinkPath
    }
    
    init() {
        let homeDir = FileManager.default.homeDirectoryForCurrentUser
        self.socketPath = homeDir.appendingPathComponent(".diane/diane.sock").path
        logger.info("DianeClient initialized with socket path: \(self.socketPath)")
        FileLogger.shared.info("DianeClient initialized with socket path: \(self.socketPath)", category: "DianeClient")
        
        // Create a custom URLSession configuration for Unix socket
        let config = URLSessionConfiguration.default
        config.timeoutIntervalForRequest = 5
        config.timeoutIntervalForResource = 10
        
        self.session = URLSession(configuration: config)
        
        // Ensure symlink to bundled binary exists
        Self.ensureSymlink()
    }
    
    /// Ensures ~/.diane/bin/diane is a symlink to the bundled binary
    static func ensureSymlink() {
        guard let bundledPath = bundledBinaryPath,
              FileManager.default.fileExists(atPath: bundledPath) else {
            logger.info("No bundled binary found, skipping symlink setup")
            return
        }
        
        let fm = FileManager.default
        let binDir = fm.homeDirectoryForCurrentUser.appendingPathComponent(".diane/bin")
        
        // Create ~/.diane/bin if it doesn't exist
        if !fm.fileExists(atPath: binDir.path) {
            do {
                try fm.createDirectory(at: binDir, withIntermediateDirectories: true)
                logger.info("Created directory: \(binDir.path)")
            } catch {
                logger.error("Failed to create bin directory: \(error.localizedDescription)")
                return
            }
        }
        
        // Check current state of symlink path
        var isDirectory: ObjCBool = false
        let exists = fm.fileExists(atPath: symlinkPath, isDirectory: &isDirectory)
        
        if exists {
            // Check if it's already the correct symlink
            do {
                let attrs = try fm.attributesOfItem(atPath: symlinkPath)
                if let type = attrs[.type] as? FileAttributeType, type == .typeSymbolicLink {
                    let destination = try fm.destinationOfSymbolicLink(atPath: symlinkPath)
                    if destination == bundledPath {
                        logger.debug("Symlink already points to bundled binary")
                        return
                    }
                }
                // Remove existing file/symlink to replace it
                try fm.removeItem(atPath: symlinkPath)
                logger.info("Removed existing file at symlink path")
            } catch {
                logger.error("Failed to check/remove existing symlink: \(error.localizedDescription)")
                return
            }
        }
        
        // Create symlink
        do {
            try fm.createSymbolicLink(atPath: symlinkPath, withDestinationPath: bundledPath)
            logger.info("Created symlink: \(symlinkPath) -> \(bundledPath)")
            FileLogger.shared.info("Created symlink: \(symlinkPath) -> \(bundledPath)", category: "DianeClient")
        } catch {
            logger.error("Failed to create symlink: \(error.localizedDescription)")
            FileLogger.shared.error("Failed to create symlink: \(error.localizedDescription)", category: "DianeClient")
        }
    }
    
    /// Check if the socket file exists
    var socketExists: Bool {
        let exists = FileManager.default.fileExists(atPath: socketPath)
        logger.debug("Socket exists check: \(exists)")
        FileLogger.shared.debug("Socket exists check: \(exists)", category: "DianeClient")
        return exists
    }
    
    /// Get the PID file path
    private var pidFilePath: String {
        let homeDir = FileManager.default.homeDirectoryForCurrentUser
        return homeDir.appendingPathComponent(".diane/mcp.pid").path
    }
    
    /// Read PID from file
    func getPID() -> Int? {
        guard let content = try? String(contentsOfFile: pidFilePath, encoding: .utf8) else {
            return nil
        }
        return Int(content.trimmingCharacters(in: .whitespacesAndNewlines))
    }
    
    /// Check if process is running
    func isProcessRunning() -> Bool {
        guard let pid = getPID() else { return false }
        // Send signal 0 to check if process exists
        return kill(Int32(pid), 0) == 0
    }
    
    /// Make a request to the Unix socket (non-blocking with timeout)
    private func request(_ path: String, method: String = "GET", timeout: Int = 3, body: Data? = nil) async throws -> Data {
        logger.info("Making \(method) request to \(path)")
        FileLogger.shared.info("Making \(method) request to \(path)", category: "DianeClient")
        
        // Use curl to communicate with Unix socket (simplest approach for macOS)
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/bin/curl")
        
        var args = [
            "--unix-socket", socketPath,
            "-s", // silent
            "-f", // fail on HTTP errors
            "--max-time", "\(timeout)", // timeout in seconds
            "--connect-timeout", "2", // connection timeout
            "http://localhost\(path)"
        ]
        
        if method != "GET" {
            args.insert(contentsOf: ["-X", method], at: 0)
            if let body = body {
                args.insert(contentsOf: ["-H", "Content-Type: application/json"], at: 0)
                args.insert(contentsOf: ["-d", String(data: body, encoding: .utf8) ?? "{}"], at: 0)
            }
        }
        
        process.arguments = args
        
        let pipe = Pipe()
        let errorPipe = Pipe()
        process.standardOutput = pipe
        process.standardError = errorPipe
        
        // Run process in background to avoid blocking the main thread
        return try await withCheckedThrowingContinuation { continuation in
            DispatchQueue.global(qos: .userInitiated).async {
                do {
                    try process.run()
                    process.waitUntilExit()
                    
                    let exitCode = process.terminationStatus
                    logger.info("curl exit code: \(exitCode) for \(path)")
                    FileLogger.shared.info("curl exit code: \(exitCode) for \(path)", category: "DianeClient")
                    
                    if exitCode != 0 {
                        let errorData = errorPipe.fileHandleForReading.readDataToEndOfFile()
                        let errorStr = String(data: errorData, encoding: .utf8) ?? "unknown"
                        logger.error("curl failed with exit code \(exitCode): \(errorStr)")
                        FileLogger.shared.error("curl failed with exit code \(exitCode): \(errorStr)", category: "DianeClient")
                        continuation.resume(throwing: DianeClientError.requestFailed(path: path, exitCode: exitCode, stderr: errorStr))
                        return
                    }
                    
                    let data = pipe.fileHandleForReading.readDataToEndOfFile()
                    logger.info("Request to \(path) succeeded, got \(data.count) bytes")
                    FileLogger.shared.info("Request to \(path) succeeded, got \(data.count) bytes", category: "DianeClient")
                    continuation.resume(returning: data)
                } catch {
                    logger.error("Failed to run curl: \(error.localizedDescription)")
                    FileLogger.shared.error("Failed to run curl: \(error.localizedDescription)", category: "DianeClient")
                    continuation.resume(throwing: error)
                }
            }
        }
    }
    
    /// Health check
    func health() async -> Bool {
        do {
            _ = try await request("/health")
            return true
        } catch {
            return false
        }
    }
    
    /// Get full status
    func getStatus() async throws -> DianeStatus {
        logger.info("Getting status...")
        FileLogger.shared.info("Getting status...", category: "DianeClient")
        let data = try await request("/status")
        
        do {
            let status = try makeGoCompatibleDecoder().decode(DianeStatus.self, from: data)
            logger.info("Status decoded successfully: running=\(status.running), version=\(status.version)")
            FileLogger.shared.info("Status decoded successfully: running=\(status.running), version=\(status.version)", category: "DianeClient")
            return status
        } catch {
            let dataStr = String(data: data, encoding: .utf8) ?? "invalid utf8"
            logger.error("Failed to decode status: \(error.localizedDescription), data: \(dataStr)")
            FileLogger.shared.error("Failed to decode status: \(error.localizedDescription), data: \(dataStr)", category: "DianeClient")
            throw error
        }
    }
    
    /// Get MCP servers
    func getMCPServers() async throws -> [MCPServerStatus] {
        let data = try await request("/mcp-servers")
        return try JSONDecoder().decode([MCPServerStatus].self, from: data)
    }
    
    /// Get all tools
    func getTools() async throws -> [ToolInfo] {
        let data = try await request("/tools")
        return try JSONDecoder().decode([ToolInfo].self, from: data)
    }
    
    func getPrompts() async throws -> [PromptInfo] {
        let data = try await request("/prompts")
        return try JSONDecoder().decode([PromptInfo].self, from: data)
    }
    
    func getResources() async throws -> [ResourceInfo] {
        let data = try await request("/resources")
        return try JSONDecoder().decode([ResourceInfo].self, from: data)
    }
    
    func getPromptContent(server: String, name: String) async throws -> PromptContentResponse {
        let encodedServer = server.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? server
        let encodedName = name.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? name
        let data = try await request("/prompts/get?server=\(encodedServer)&name=\(encodedName)")
        return try JSONDecoder().decode(PromptContentResponse.self, from: data)
    }
    
    func getResourceContent(server: String, uri: String) async throws -> ResourceContentResponse {
        let encodedServer = server.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? server
        let encodedURI = uri.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? uri
        let data = try await request("/resources/read?server=\(encodedServer)&uri=\(encodedURI)")
        return try JSONDecoder().decode(ResourceContentResponse.self, from: data)
    }
    
    /// Reload configuration
    func reloadConfig() async throws {
        _ = try await request("/reload", method: "POST")
    }
    
    /// Restart an MCP server
    func restartMCPServer(name: String) async throws {
        _ = try await request("/mcp-servers/\(name)/restart", method: "POST")
    }
    
    // MARK: - OAuth API
    
    /// Start OAuth login for an MCP server (device flow)
    func startAuth(serverName: String) async throws -> DeviceCodeInfo {
        let encodedName = serverName.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? serverName
        let data = try await request("/auth/\(encodedName)/login", method: "POST", timeout: 10)
        return try JSONDecoder().decode(DeviceCodeInfo.self, from: data)
    }
    
    /// Poll for OAuth token completion
    func pollAuth(serverName: String, deviceCode: String, interval: Int) async throws {
        let encodedName = serverName.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? serverName
        let body: [String: Any] = ["device_code": deviceCode, "interval": interval]
        let bodyData = try JSONSerialization.data(withJSONObject: body)
        _ = try await request("/auth/\(encodedName)/poll", method: "POST", timeout: 120, body: bodyData)
    }
    
    // MARK: - Scheduler API
    
    /// Get all scheduled jobs
    func getJobs() async throws -> [Job] {
        let data = try await request("/jobs")
        return try makeGoCompatibleDecoder().decode([Job].self, from: data)
    }
    
    /// Get job execution logs
    func getJobLogs(jobName: String? = nil, limit: Int = 50) async throws -> [JobExecution] {
        var path = "/jobs/logs?limit=\(limit)"
        if let jobName = jobName {
            path += "&job_name=\(jobName)"
        }
        let data = try await request(path)
        return try makeGoCompatibleDecoder().decode([JobExecution].self, from: data)
    }
    
    /// Toggle a job's enabled status
    func toggleJob(name: String, enabled: Bool) async throws {
        let body = ["enabled": enabled]
        let bodyData = try JSONEncoder().encode(body)
        _ = try await request("/jobs/\(name)/toggle", method: "POST", body: bodyData)
    }
    
    /// Start Diane (launches the process)
    func startDiane() throws {
        // Use bundled binary if available, otherwise fall back to ~/.diane/bin/diane
        let binaryPath = Self.binaryPath
        
        guard FileManager.default.fileExists(atPath: binaryPath) else {
            throw DianeClientError.binaryNotFound
        }
        
        let process = Process()
        process.executableURL = URL(fileURLWithPath: binaryPath)
        process.arguments = ["serve"]
        
        // Detach from terminal
        process.standardInput = FileHandle.nullDevice
        process.standardOutput = FileHandle.nullDevice
        process.standardError = FileHandle.nullDevice
        
        try process.run()
        logger.info("Started diane from: \(binaryPath) with 'serve' argument")
    }
    
    /// Stop Diane (sends SIGTERM)
    func stopDiane() throws {
        guard let pid = getPID() else {
            throw DianeClientError.notRunning
        }
        
        let result = kill(Int32(pid), SIGTERM)
        if result != 0 {
            throw DianeClientError.stopFailed
        }
    }
    
    /// Restart Diane
    func restartDiane() async throws {
        if isProcessRunning() {
            try stopDiane()
            // Wait for process to stop
            try await Task.sleep(nanoseconds: 1_000_000_000) // 1 second
        }
        try startDiane()
    }
    
    /// Send reload signal (SIGUSR1)
    func sendReloadSignal() throws {
        guard let pid = getPID() else {
            throw DianeClientError.notRunning
        }
        
        let result = kill(Int32(pid), SIGUSR1)
        if result != 0 {
            throw DianeClientError.signalFailed
        }
    }
    
    // MARK: - Agents API
    
    /// Get all configured agents
    func getAgents() async throws -> [AgentConfig] {
        let data = try await request("/agents")
        return try JSONDecoder().decode([AgentConfig].self, from: data)
    }
    
    /// Get a specific agent by name
    func getAgent(name: String) async throws -> AgentConfig {
        let encodedName = name.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? name
        let data = try await request("/agents/\(encodedName)")
        return try JSONDecoder().decode(AgentConfig.self, from: data)
    }
    
    /// Test an agent's connectivity
    func testAgent(name: String) async throws -> AgentTestResult {
        let encodedName = name.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? name
        let data = try await request("/agents/\(encodedName)/test", method: "POST", timeout: 10)
        return try JSONDecoder().decode(AgentTestResult.self, from: data)
    }
    
    /// Toggle an agent's enabled status
    func toggleAgent(name: String, enabled: Bool) async throws {
        let encodedName = name.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? name
        let body = ["enabled": enabled]
        let bodyData = try JSONEncoder().encode(body)
        _ = try await request("/agents/\(encodedName)/toggle", method: "POST", body: bodyData)
    }
    
    /// Run a prompt on an agent
    func runAgentPrompt(agentName: String, prompt: String, remoteAgentName: String? = nil) async throws -> AgentRunResult {
        let encodedName = agentName.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? agentName
        var body: [String: String] = ["prompt": prompt]
        if let remoteAgent = remoteAgentName {
            body["agent_name"] = remoteAgent
        }
        let bodyData = try JSONEncoder().encode(body)
        let data = try await request("/agents/\(encodedName)/run", method: "POST", timeout: 60, body: bodyData)
        return try makeGoCompatibleDecoder().decode(AgentRunResult.self, from: data)
    }
    
    /// Get agent communication logs
    func getAgentLogs(agentName: String? = nil, limit: Int = 50) async throws -> [AgentLog] {
        var path = "/agents/logs?limit=\(limit)"
        if let agentName = agentName {
            let encodedName = agentName.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? agentName
            path += "&agent_name=\(encodedName)"
        }
        let data = try await request(path)
        return try makeGoCompatibleDecoder().decode([AgentLog].self, from: data)
    }
    
    /// Remove an agent
    func removeAgent(name: String) async throws {
        let encodedName = name.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? name
        _ = try await request("/agents/\(encodedName)", method: "DELETE")
    }
    
    /// Get remote sub-agents/config options from an ACP agent
    func getRemoteAgents(agentName: String) async throws -> [RemoteAgentInfo] {
        let encodedName = agentName.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? agentName
        let data = try await request("/agents/\(encodedName)/remote-agents", method: "GET", timeout: 35)
        return try JSONDecoder().decode([RemoteAgentInfo].self, from: data)
    }
    
    /// Update an agent's configuration
    func updateAgent(name: String, subAgent: String? = nil, enabled: Bool? = nil, description: String? = nil, workdir: String? = nil) async throws {
        let encodedName = name.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? name
        var body: [String: Any] = [:]
        if let subAgent = subAgent {
            body["sub_agent"] = subAgent
        }
        if let enabled = enabled {
            body["enabled"] = enabled
        }
        if let description = description {
            body["description"] = description
        }
        if let workdir = workdir {
            body["workdir"] = workdir
        }
        
        guard !body.isEmpty else { return }
        
        let bodyData = try JSONSerialization.data(withJSONObject: body)
        _ = try await request("/agents/\(encodedName)/update", method: "POST", body: bodyData)
    }
    
    // MARK: - Gallery API
    
    /// Get all available agents from the gallery
    func getGallery(featured: Bool = false) async throws -> [GalleryEntry] {
        let path = featured ? "/gallery?featured=true" : "/gallery"
        let data = try await request(path)
        return try JSONDecoder().decode([GalleryEntry].self, from: data)
    }
    
    /// Install an agent from the gallery
    func installGalleryAgent(id: String, name: String? = nil, workdir: String? = nil, port: Int? = nil) async throws -> GalleryInstallResponse {
        var body: [String: Any] = [:]
        if let name = name {
            body["name"] = name
        }
        if let workdir = workdir {
            body["workdir"] = workdir
        }
        if let port = port, port > 0 {
            body["port"] = port
            body["type"] = "acp"
        }
        
        let bodyData = body.isEmpty ? nil : try JSONSerialization.data(withJSONObject: body)
        let data = try await request("/gallery/\(id)/install", method: "POST", body: bodyData)
        return try JSONDecoder().decode(GalleryInstallResponse.self, from: data)
    }
    
    // MARK: - Contexts API
    
    /// Get all contexts
    func getContexts() async throws -> [Context] {
        let data = try await request("/contexts")
        return try JSONDecoder().decode([Context].self, from: data)
    }
    
    /// Get context details including servers and tools
    func getContextDetail(name: String) async throws -> ContextDetail {
        let encodedName = name.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? name
        let data = try await request("/contexts/\(encodedName)")
        return try JSONDecoder().decode(ContextDetail.self, from: data)
    }
    
    /// Create a new context
    func createContext(name: String, description: String? = nil) async throws -> Context {
        var body: [String: String] = ["name": name]
        if let description = description {
            body["description"] = description
        }
        let bodyData = try JSONEncoder().encode(body)
        let data = try await request("/contexts", method: "POST", body: bodyData)
        return try JSONDecoder().decode(Context.self, from: data)
    }
    
    /// Update a context's description
    func updateContext(name: String, description: String) async throws {
        let encodedName = name.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? name
        let body = ["description": description]
        let bodyData = try JSONEncoder().encode(body)
        _ = try await request("/contexts/\(encodedName)", method: "PUT", body: bodyData)
    }
    
    /// Delete a context
    func deleteContext(name: String) async throws {
        let encodedName = name.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? name
        _ = try await request("/contexts/\(encodedName)", method: "DELETE")
    }
    
    /// Set a context as the default
    func setDefaultContext(name: String) async throws {
        let encodedName = name.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? name
        _ = try await request("/contexts/\(encodedName)/default", method: "POST")
    }
    
    /// Get connection info for a context
    func getContextConnectInfo(name: String) async throws -> ContextConnectInfo {
        let encodedName = name.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? name
        let data = try await request("/contexts/\(encodedName)/connect")
        return try JSONDecoder().decode(ContextConnectInfo.self, from: data)
    }
    
    /// Get servers in a context
    func getContextServers(contextName: String) async throws -> [ContextServer] {
        let encodedName = contextName.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? contextName
        let data = try await request("/contexts/\(encodedName)/servers")
        return try JSONDecoder().decode([ContextServer].self, from: data)
    }
    
    /// Enable/disable a server in a context
    func setServerEnabledInContext(contextName: String, serverName: String, enabled: Bool) async throws {
        let encodedContext = contextName.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? contextName
        let encodedServer = serverName.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? serverName
        let body = ["enabled": enabled]
        let bodyData = try JSONEncoder().encode(body)
        _ = try await request("/contexts/\(encodedContext)/servers/\(encodedServer)", method: "PUT", body: bodyData)
    }
    
    /// Remove a server from a context
    func removeServerFromContext(contextName: String, serverName: String) async throws {
        let encodedContext = contextName.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? contextName
        let encodedServer = serverName.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? serverName
        _ = try await request("/contexts/\(encodedContext)/servers/\(encodedServer)", method: "DELETE")
    }
    
    /// Get tools for a server in a context
    func getContextServerTools(contextName: String, serverName: String) async throws -> [ContextTool] {
        let encodedContext = contextName.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? contextName
        let encodedServer = serverName.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? serverName
        let data = try await request("/contexts/\(encodedContext)/servers/\(encodedServer)/tools")
        return try JSONDecoder().decode([ContextTool].self, from: data)
    }
    
    /// Enable/disable a specific tool in a context
    func setToolEnabledInContext(contextName: String, serverName: String, toolName: String, enabled: Bool) async throws {
        let encodedContext = contextName.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? contextName
        let encodedServer = serverName.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? serverName
        let encodedTool = toolName.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? toolName
        let body = ["enabled": enabled]
        let bodyData = try JSONEncoder().encode(body)
        _ = try await request("/contexts/\(encodedContext)/servers/\(encodedServer)/tools/\(encodedTool)", method: "PUT", body: bodyData)
    }
    
    /// Bulk update tool enabled states
    func bulkSetToolsEnabled(contextName: String, serverName: String, tools: [String: Bool]) async throws {
        let encodedContext = contextName.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? contextName
        let encodedServer = serverName.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? serverName
        let bodyData = try JSONEncoder().encode(tools)
        _ = try await request("/contexts/\(encodedContext)/servers/\(encodedServer)/tools", method: "PUT", body: bodyData)
    }
    
    /// Sync tools from running MCP servers to a context
    func syncContextTools(contextName: String) async throws -> Int {
        let encodedContext = contextName.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? contextName
        let data = try await request("/contexts/\(encodedContext)/sync", method: "POST")
        let decoder = makeGoCompatibleDecoder()
        let result = try decoder.decode(SyncResult.self, from: data)
        return result.toolsSynced
    }
    
    /// Get available servers that can be added to a context
    func getAvailableServers(contextName: String) async throws -> [AvailableServer] {
        let encodedContext = contextName.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? contextName
        let data = try await request("/contexts/\(encodedContext)/available-servers")
        let decoder = makeGoCompatibleDecoder()
        return try decoder.decode([AvailableServer].self, from: data)
    }
    
    /// Add a server to a context
    func addServerToContext(contextName: String, serverName: String, enabled: Bool = true) async throws {
        let encodedContext = contextName.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? contextName
        let body: [String: Any] = ["server_name": serverName, "enabled": enabled]
        let bodyData = try JSONSerialization.data(withJSONObject: body)
        _ = try await request("/contexts/\(encodedContext)/servers", method: "POST", body: bodyData)
    }
    
    // MARK: - MCP Server Configuration API
    
    /// Get all MCP server configurations
    func getMCPServerConfigs() async throws -> [MCPServer] {
        let data = try await request("/mcp-servers-config")
        return try makeGoCompatibleDecoder().decode([MCPServer].self, from: data)
    }
    
    /// Get a specific MCP server configuration by ID
    func getMCPServerConfig(id: Int64) async throws -> MCPServer {
        let data = try await request("/mcp-servers-config/\(id)")
        return try makeGoCompatibleDecoder().decode(MCPServer.self, from: data)
    }
    
    /// Create a new MCP server configuration
    func createMCPServerConfig(name: String, type: String, enabled: Bool = true, command: String? = nil, args: [String]? = nil, env: [String: String]? = nil, url: String? = nil, headers: [String: String]? = nil, oauth: OAuthConfig? = nil, nodeID: String? = nil, nodeMode: String? = nil) async throws -> MCPServer {
        var body: [String: Any] = [
            "name": name,
            "type": type,
            "enabled": enabled
        ]
        if let command = command {
            body["command"] = command
        }
        if let args = args {
            body["args"] = args
        }
        if let env = env {
            body["env"] = env
        }
        if let url = url {
            body["url"] = url
        }
        if let headers = headers {
            body["headers"] = headers
        }
        if let oauth = oauth {
            body["oauth"] = try? JSONEncoder().encode(oauth)
        }
        if let nodeID = nodeID {
            body["node_id"] = nodeID
        }
        if let nodeMode = nodeMode {
            body["node_mode"] = nodeMode
        }
        
        let bodyData = try JSONSerialization.data(withJSONObject: body)
        let data = try await request("/mcp-servers-config", method: "POST", body: bodyData)
        return try makeGoCompatibleDecoder().decode(MCPServer.self, from: data)
    }
    
    /// Update an MCP server configuration
    func updateMCPServerConfig(id: Int64, name: String? = nil, type: String? = nil, enabled: Bool? = nil, command: String? = nil, args: [String]? = nil, env: [String: String]? = nil, url: String? = nil, headers: [String: String]? = nil, oauth: OAuthConfig? = nil, nodeID: String? = nil, nodeMode: String? = nil) async throws -> MCPServer {
        var body: [String: Any] = [:]
        if let name = name {
            body["name"] = name
        }
        if let type = type {
            body["type"] = type
        }
        if let enabled = enabled {
            body["enabled"] = enabled
        }
        if let command = command {
            body["command"] = command
        }
        if let args = args {
            body["args"] = args
        }
        if let env = env {
            body["env"] = env
        }
        if let url = url {
            body["url"] = url
        }
        if let headers = headers {
            body["headers"] = headers
        }
        if let oauth = oauth {
            body["oauth"] = try? JSONEncoder().encode(oauth)
        }
        if let nodeID = nodeID {
            body["node_id"] = nodeID
        }
        if let nodeMode = nodeMode {
            body["node_mode"] = nodeMode
        }
        
        guard !body.isEmpty else {
            // Nothing to update, return current state
            return try await getMCPServerConfig(id: id)
        }
        
        let bodyData = try JSONSerialization.data(withJSONObject: body)
        let data = try await request("/mcp-servers-config/\(id)", method: "PUT", body: bodyData)
        return try makeGoCompatibleDecoder().decode(MCPServer.self, from: data)
    }
    
    /// Delete an MCP server configuration
    func deleteMCPServerConfig(id: Int64) async throws {
        _ = try await request("/mcp-servers-config/\(id)", method: "DELETE")
    }
    
    // MARK: - Providers API
    
    /// Get all providers, optionally filtered by type
    func getProviders(type: ProviderType? = nil) async throws -> [Provider] {
        var path = "/providers"
        if let type = type {
            path += "?type=\(type.rawValue)"
        }
        let data = try await request(path)
        return try makeGoCompatibleDecoder().decode([Provider].self, from: data)
    }
    
    /// Get provider templates
    func getProviderTemplates() async throws -> [ProviderTemplate] {
        let data = try await request("/providers/templates")
        return try JSONDecoder().decode([ProviderTemplate].self, from: data)
    }
    
    /// Get a specific provider by ID
    func getProvider(id: Int64) async throws -> Provider {
        let data = try await request("/providers/\(id)")
        return try makeGoCompatibleDecoder().decode(Provider.self, from: data)
    }
    
    /// Create a new provider
    func createProvider(name: String, service: String, config: [String: Any], authConfig: [String: Any]? = nil) async throws -> Provider {
        var body: [String: Any] = [
            "name": name,
            "service": service,
            "config": config
        ]
        if let authConfig = authConfig {
            body["auth_config"] = authConfig
        }
        let bodyData = try JSONSerialization.data(withJSONObject: body)
        let data = try await request("/providers", method: "POST", body: bodyData)
        return try makeGoCompatibleDecoder().decode(Provider.self, from: data)
    }
    
    /// Update a provider
    func updateProvider(id: Int64, name: String? = nil, config: [String: Any]? = nil, authConfig: [String: Any]? = nil) async throws -> Provider {
        var body: [String: Any] = [:]
        if let name = name {
            body["name"] = name
        }
        if let config = config {
            body["config"] = config
        }
        if let authConfig = authConfig {
            body["auth_config"] = authConfig
        }
        
        guard !body.isEmpty else {
            // Nothing to update, just return current state
            return try await getProvider(id: id)
        }
        
        let bodyData = try JSONSerialization.data(withJSONObject: body)
        let data = try await request("/providers/\(id)", method: "PUT", body: bodyData)
        return try makeGoCompatibleDecoder().decode(Provider.self, from: data)
    }
    
    /// Delete a provider
    func deleteProvider(id: Int64) async throws {
        _ = try await request("/providers/\(id)", method: "DELETE")
    }
    
    /// Enable a provider
    func enableProvider(id: Int64) async throws -> Provider {
        let data = try await request("/providers/\(id)/enable", method: "POST")
        return try makeGoCompatibleDecoder().decode(Provider.self, from: data)
    }
    
    /// Disable a provider
    func disableProvider(id: Int64) async throws -> Provider {
        let data = try await request("/providers/\(id)/disable", method: "POST")
        return try makeGoCompatibleDecoder().decode(Provider.self, from: data)
    }
    
    /// Set a provider as the default for its type
    func setDefaultProvider(id: Int64) async throws -> Provider {
        let data = try await request("/providers/\(id)/set-default", method: "POST")
        return try makeGoCompatibleDecoder().decode(Provider.self, from: data)
    }
    
    /// Test a provider's connectivity
    func testProvider(id: Int64) async throws -> ProviderTestResult {
        let data = try await request("/providers/\(id)/test", method: "POST", timeout: 35)
        return try makeGoCompatibleDecoder().decode(ProviderTestResult.self, from: data)
    }
    
    /// List available models for a service
    func listModels(service: String, projectID: String? = nil) async throws -> [AvailableModel] {
        var requestBody: [String: Any] = ["service": service]
        if let projectID = projectID {
            requestBody["project_id"] = projectID
        }
        let bodyData = try JSONSerialization.data(withJSONObject: requestBody)
        let data = try await request("/providers/models", method: "POST", timeout: 35, body: bodyData)
        let response = try makeGoCompatibleDecoder().decode(ListModelsResponse.self, from: data)
        return response.models
    }
    
    /// Get model info by provider and model ID (e.g., "google-vertex", "gemini-2.0-flash")
    func getModelInfo(provider: String, modelID: String) async throws -> AvailableModel {
        let data = try await request("/models/\(provider)/\(modelID)")
        return try makeGoCompatibleDecoder().decode(AvailableModel.self, from: data)
    }
    
    // MARK: - Usage API
    
    /// Get usage records with optional filtering
    func getUsage(from: Date? = nil, to: Date? = nil, limit: Int = 100, service: String? = nil, providerID: Int64? = nil) async throws -> UsageResponse {
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
        return try makeGoCompatibleDecoder().decode(UsageResponse.self, from: data)
    }
    
    /// Get usage summary (aggregated by provider/model)
    func getUsageSummary(from: Date? = nil, to: Date? = nil) async throws -> UsageSummaryResponse {
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
        return try makeGoCompatibleDecoder().decode(UsageSummaryResponse.self, from: data)
    }
    
    // MARK: - Google OAuth API
    
    /// Get Google authentication status
    func getGoogleAuthStatus(account: String = "default") async throws -> GoogleAuthStatus {
        let data = try await request("/google/auth?account=\(account)")
        return try JSONDecoder().decode(GoogleAuthStatus.self, from: data)
    }
    
    /// Start Google OAuth device flow
    func startGoogleAuth(account: String? = nil, scopes: [String]? = nil) async throws -> GoogleDeviceCodeResponse {
        var body: [String: Any] = [:]
        if let account = account {
            body["account"] = account
        }
        if let scopes = scopes {
            body["scopes"] = scopes
        }
        
        let bodyData = body.isEmpty ? nil : try JSONSerialization.data(withJSONObject: body)
        let data = try await request("/google/auth/start", method: "POST", timeout: 10, body: bodyData)
        return try JSONDecoder().decode(GoogleDeviceCodeResponse.self, from: data)
    }
    
    /// Poll for Google OAuth token
    /// Returns the poll response. Check isPending, isSuccess, etc. to determine status.
    func pollGoogleAuth(account: String = "default", deviceCode: String, interval: Int) async throws -> GoogleAuthPollResponse {
        let body: [String: Any] = [
            "account": account,
            "device_code": deviceCode,
            "interval": interval
        ]
        let bodyData = try JSONSerialization.data(withJSONObject: body)
        
        // This endpoint may return 202 (pending), 429 (slow down), 410 (expired), 403 (denied), or 200 (success)
        // We need to handle non-200 status codes specially
        let data = try await requestWithStatus("/google/auth/poll", method: "POST", timeout: 30, body: bodyData)
        return try makeGoCompatibleDecoder().decode(GoogleAuthPollResponse.self, from: data)
    }
    
    /// Delete Google OAuth token
    func deleteGoogleAuth(account: String = "default") async throws {
        _ = try await request("/google/auth?account=\(account)", method: "DELETE")
    }
    
    // MARK: - Slave Management
    
    /// Get all registered slaves
    func getSlaves() async throws -> [SlaveInfo] {
        let data = try await request("/slaves", timeout: 5)
        return try JSONDecoder().decode([SlaveInfo].self, from: data)
    }
    
    /// Get pending pairing requests
    func getPendingPairingRequests() async throws -> [PairingRequest] {
        logger.info("DEBUG: Requesting GET /slaves/pending")
        let data = try await request("/slaves/pending", timeout: 5)
        if let str = String(data: data, encoding: .utf8) {
            logger.info("DEBUG: Response from /slaves/pending: \(str)")
        }
        return try JSONDecoder().decode([PairingRequest].self, from: data)
    }
    
    /// Approve a pairing request
    func approvePairingRequest(hostname: String, pairingCode: String) async throws {
        let body: [String: Any] = [
            "hostname": hostname,
            "pairing_code": pairingCode
        ]
        let bodyData = try JSONSerialization.data(withJSONObject: body)
        _ = try await request("/slaves/approve", method: "POST", timeout: 10, body: bodyData)
    }
    
    /// Deny a pairing request
    func denyPairingRequest(hostname: String, pairingCode: String) async throws {
        let body: [String: Any] = [
            "hostname": hostname,
            "pairing_code": pairingCode
        ]
        let bodyData = try JSONSerialization.data(withJSONObject: body)
        _ = try await request("/slaves/deny", method: "POST", timeout: 10, body: bodyData)
    }
    
    /// Revoke slave credentials
    func revokeSlaveCredentials(hostname: String, reason: String?) async throws {
        var body: [String: Any] = ["hostname": hostname]
        if let reason = reason {
            body["reason"] = reason
        }
        let bodyData = try JSONSerialization.data(withJSONObject: body)
        _ = try await request("/slaves/revoke", method: "POST", timeout: 10, body: bodyData)
    }
    
    func restartSlave(hostname: String) async throws {
        _ = try await request("/slaves/restart/\(hostname)", method: "POST", timeout: 10)
    }
    
    func upgradeSlave(hostname: String) async throws {
        _ = try await request("/slaves/upgrade/\(hostname)", method: "POST", timeout: 10)
    }
    
    /// Make a request that accepts non-200 status codes (for polling)
    private func requestWithStatus(_ path: String, method: String = "GET", timeout: Int = 3, body: Data? = nil) async throws -> Data {
        logger.info("Making \(method) request to \(path) (with status)")
        FileLogger.shared.info("Making \(method) request to \(path) (with status)", category: "DianeClient")
        
        // Use curl to communicate with Unix socket - don't fail on non-200
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/bin/curl")
        
        var args = [
            "--unix-socket", socketPath,
            "-s", // silent
            // Note: no -f flag, so we get the body even on non-200
            "--max-time", "\(timeout)",
            "--connect-timeout", "2",
            "http://localhost\(path)"
        ]
        
        if method != "GET" {
            args.insert(contentsOf: ["-X", method], at: 0)
            if let body = body {
                args.insert(contentsOf: ["-H", "Content-Type: application/json"], at: 0)
                args.insert(contentsOf: ["-d", String(data: body, encoding: .utf8) ?? "{}"], at: 0)
            }
        }
        
        process.arguments = args
        
        let pipe = Pipe()
        let errorPipe = Pipe()
        process.standardOutput = pipe
        process.standardError = errorPipe
        
        return try await withCheckedThrowingContinuation { continuation in
            DispatchQueue.global(qos: .userInitiated).async {
                do {
                    try process.run()
                    process.waitUntilExit()
                    
                    let exitCode = process.terminationStatus
                    logger.info("curl exit code: \(exitCode) for \(path)")
                    
                    // For this method, we only fail on curl errors, not HTTP errors
                    if exitCode != 0 && exitCode != 22 { // 22 is HTTP error, but we still get the body
                        let errorData = errorPipe.fileHandleForReading.readDataToEndOfFile()
                        let errorStr = String(data: errorData, encoding: .utf8) ?? "unknown"
                        continuation.resume(throwing: DianeClientError.requestFailed(path: path, exitCode: exitCode, stderr: errorStr))
                        return
                    }
                    
                    let data = pipe.fileHandleForReading.readDataToEndOfFile()
                    continuation.resume(returning: data)
                } catch {
                    continuation.resume(throwing: error)
                }
            }
        }
    }
}

/// Result of syncing tools
struct SyncResult: Codable {
    let status: String
    let toolsSynced: Int
    
    enum CodingKeys: String, CodingKey {
        case status
        case toolsSynced = "tools_synced"
    }
}

enum DianeClientError: LocalizedError {
    case requestFailed(path: String, exitCode: Int32, stderr: String)
    case binaryNotFound
    case notRunning
    case stopFailed
    case signalFailed
    
    var errorDescription: String? {
        switch self {
        case .requestFailed(let path, let exitCode, let stderr):
            if stderr.isEmpty {
                return "Request to \(path) failed (exit code: \(exitCode))"
            } else {
                return "Request to \(path) failed: \(stderr)"
            }
        case .binaryNotFound:
            return "Diane binary not found. Expected at bundled location or ~/.diane/bin/diane"
        case .notRunning:
            return "Diane is not running"
        case .stopFailed:
            return "Failed to stop Diane"
        case .signalFailed:
            return "Failed to send signal to Diane"
        }
    }
}
