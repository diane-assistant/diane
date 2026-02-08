import Foundation
import Network
import os.log

private let logger = Logger(subsystem: "com.diane.DianeMenu", category: "DianeClient")

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
class DianeClient {
    private let socketPath: String
    private let session: URLSession
    
    init() {
        let homeDir = FileManager.default.homeDirectoryForCurrentUser
        self.socketPath = homeDir.appendingPathComponent(".diane/diane.sock").path
        logger.info("DianeClient initialized with socket path: \(self.socketPath)")
        
        // Create a custom URLSession configuration for Unix socket
        let config = URLSessionConfiguration.default
        config.timeoutIntervalForRequest = 5
        config.timeoutIntervalForResource = 10
        
        self.session = URLSession(configuration: config)
    }
    
    /// Check if the socket file exists
    var socketExists: Bool {
        let exists = FileManager.default.fileExists(atPath: socketPath)
        logger.debug("Socket exists check: \(exists)")
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
                    
                    if exitCode != 0 {
                        let errorData = errorPipe.fileHandleForReading.readDataToEndOfFile()
                        let errorStr = String(data: errorData, encoding: .utf8) ?? "unknown"
                        logger.error("curl failed with exit code \(exitCode): \(errorStr)")
                        continuation.resume(throwing: DianeClientError.requestFailed(path: path, exitCode: exitCode, stderr: errorStr))
                        return
                    }
                    
                    let data = pipe.fileHandleForReading.readDataToEndOfFile()
                    logger.info("Request to \(path) succeeded, got \(data.count) bytes")
                    continuation.resume(returning: data)
                } catch {
                    logger.error("Failed to run curl: \(error.localizedDescription)")
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
        let data = try await request("/status")
        
        do {
            let status = try makeGoCompatibleDecoder().decode(DianeStatus.self, from: data)
            logger.info("Status decoded successfully: running=\(status.running), version=\(status.version)")
            return status
        } catch {
            let dataStr = String(data: data, encoding: .utf8) ?? "invalid utf8"
            logger.error("Failed to decode status: \(error.localizedDescription), data: \(dataStr)")
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
    
    /// Reload configuration
    func reloadConfig() async throws {
        _ = try await request("/reload", method: "POST")
    }
    
    /// Restart an MCP server
    func restartMCPServer(name: String) async throws {
        _ = try await request("/mcp-servers/\(name)/restart", method: "POST")
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
        let homeDir = FileManager.default.homeDirectoryForCurrentUser
        let binaryPath = homeDir.appendingPathComponent(".diane/bin/diane").path
        
        guard FileManager.default.fileExists(atPath: binaryPath) else {
            throw DianeClientError.binaryNotFound
        }
        
        let process = Process()
        process.executableURL = URL(fileURLWithPath: binaryPath)
        
        // Detach from terminal
        process.standardInput = FileHandle.nullDevice
        process.standardOutput = FileHandle.nullDevice
        process.standardError = FileHandle.nullDevice
        
        try process.run()
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
            return "Diane binary not found at ~/.diane/bin/diane"
        case .notRunning:
            return "Diane is not running"
        case .stopFailed:
            return "Failed to stop Diane"
        case .signalFailed:
            return "Failed to send signal to Diane"
        }
    }
}
