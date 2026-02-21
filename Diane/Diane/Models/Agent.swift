import Foundation

/// Represents a configured ACP agent
struct AgentConfig: Codable, Identifiable, Equatable {
    var id: String { name }
    
    let name: String
    let url: String?
    let type: String?
    let command: String?
    let args: [String]?
    let env: [String: String]?
    let workdir: String?
    let port: Int?
    let subAgent: String?
    let enabled: Bool
    let description: String?
    let tags: [String]?
    let workspaceConfig: WorkspaceConfig?
    
    enum CodingKeys: String, CodingKey {
        case name, url, type, command, args, env, workdir, port
        case subAgent = "sub_agent"
        case enabled, description, tags
        case workspaceConfig = "workspace_config"
    }
    
    /// Display name (without workspace suffix)
    var displayName: String {
        if let atIndex = name.firstIndex(of: "@") {
            return String(name[..<atIndex])
        }
        return name
    }
    
    /// Workspace name (if any)
    var workspaceName: String? {
        if let atIndex = name.firstIndex(of: "@") {
            return String(name[name.index(after: atIndex)...])
        }
        return nil
    }
    
    static func == (lhs: AgentConfig, rhs: AgentConfig) -> Bool {
        lhs.name == rhs.name && lhs.subAgent == rhs.subAgent && lhs.enabled == rhs.enabled
    }
}

/// Result of testing an agent connection
struct AgentTestResult: Codable {
    let name: String
    let url: String?
    let workdir: String?
    let enabled: Bool
    let status: String  // "connected", "unreachable", "error", "disabled"
    let error: String?
    let version: String?
    let agentCount: Int?
    let agents: [String]?
    
    enum CodingKeys: String, CodingKey {
        case name, url, workdir, enabled, status, error, version
        case agentCount = "agent_count"
        case agents
    }
    
    var isConnected: Bool {
        status == "connected"
    }
    
    var statusColor: String {
        switch status {
        case "connected": return "green"
        case "unreachable": return "red"
        case "error": return "orange"
        case "disabled": return "gray"
        default: return "gray"
        }
    }
    
    /// Formatted version string (trimmed)
    var displayVersion: String? {
        version?.trimmingCharacters(in: .whitespacesAndNewlines)
    }
}

/// Represents a log entry for agent communication
struct AgentLog: Codable, Identifiable {
    let id: Int64
    let agentName: String
    let direction: String  // "request" or "response"
    let messageType: String  // "ping", "run", "list", etc.
    let content: String?
    let error: String?
    let timestamp: Date
    let durationMs: Int?
    
    enum CodingKeys: String, CodingKey {
        case id
        case agentName = "agent_name"
        case direction
        case messageType = "message_type"
        case content
        case error
        case timestamp
        case durationMs = "duration_ms"
    }
    
    var isRequest: Bool {
        direction == "request"
    }
    
    var isError: Bool {
        error != nil
    }
    
    var formattedDuration: String? {
        guard let ms = durationMs else { return nil }
        if ms < 1000 {
            return "\(ms)ms"
        } else {
            return String(format: "%.1fs", Double(ms) / 1000.0)
        }
    }
}

/// Response from running a prompt on an agent
struct AgentRunResult: Codable {
    let agentName: String
    let sessionId: String?
    let runId: String
    let status: String
    let awaitRequest: String?
    let output: [AgentMessage]
    let error: AgentError?
    let createdAt: Date
    let finishedAt: Date?
    
    enum CodingKeys: String, CodingKey {
        case agentName = "agent_name"
        case sessionId = "session_id"
        case runId = "run_id"
        case status
        case awaitRequest = "await_request"
        case output
        case error
        case createdAt = "created_at"
        case finishedAt = "finished_at"
    }
    
    /// Get the text output from the run
    var textOutput: String {
        output.compactMap { message in
            message.parts.compactMap { part in
                if part.contentType == "text/plain" || part.contentType == nil || part.contentType == "" {
                    return part.content
                }
                return nil
            }.joined()
        }.joined()
    }
    
    /// Check if the run completed successfully
    var isSuccess: Bool {
        status == "completed" && error == nil
    }
    
    /// Check if the run failed
    var isFailed: Bool {
        status == "failed" || error != nil
    }
}

/// Represents a message in the agent output
struct AgentMessage: Codable {
    let role: String
    let parts: [AgentMessagePart]
    let createdAt: Date?
    let completedAt: Date?
    
    enum CodingKeys: String, CodingKey {
        case role, parts
        case createdAt = "created_at"
        case completedAt = "completed_at"
    }
}

/// Represents a part of a message
struct AgentMessagePart: Codable {
    let name: String?
    let contentType: String?
    let content: String?
    let contentEncoding: String?
    let contentUrl: String?
    
    enum CodingKeys: String, CodingKey {
        case name
        case contentType = "content_type"
        case content
        case contentEncoding = "content_encoding"
        case contentUrl = "content_url"
    }
}

/// Represents an error from the agent
struct AgentError: Codable {
    let code: String
    let message: String
    let data: AnyCodable?
    
    /// Format the error for display
    var displayMessage: String {
        if code.isEmpty {
            return message
        }
        return "[\(code)] \(message)"
    }
}

/// A type-erased Codable wrapper for arbitrary JSON data
struct AnyCodable: Codable {
    let value: Any
    
    init(_ value: Any) {
        self.value = value
    }
    
    init(from decoder: Decoder) throws {
        let container = try decoder.singleValueContainer()
        
        if container.decodeNil() {
            self.value = NSNull()
        } else if let bool = try? container.decode(Bool.self) {
            self.value = bool
        } else if let int = try? container.decode(Int.self) {
            self.value = int
        } else if let double = try? container.decode(Double.self) {
            self.value = double
        } else if let string = try? container.decode(String.self) {
            self.value = string
        } else if let array = try? container.decode([AnyCodable].self) {
            self.value = array.map { $0.value }
        } else if let dict = try? container.decode([String: AnyCodable].self) {
            self.value = dict.mapValues { $0.value }
        } else {
            throw DecodingError.dataCorruptedError(in: container, debugDescription: "Cannot decode value")
        }
    }
    
    func encode(to encoder: Encoder) throws {
        var container = encoder.singleValueContainer()
        
        switch value {
        case is NSNull:
            try container.encodeNil()
        case let bool as Bool:
            try container.encode(bool)
        case let int as Int:
            try container.encode(int)
        case let double as Double:
            try container.encode(double)
        case let string as String:
            try container.encode(string)
        case let array as [Any]:
            try container.encode(array.map { AnyCodable($0) })
        case let dict as [String: Any]:
            try container.encode(dict.mapValues { AnyCodable($0) })
        default:
            throw EncodingError.invalidValue(value, EncodingError.Context(codingPath: encoder.codingPath, debugDescription: "Cannot encode value"))
        }
    }
}

/// Represents an agent available in the gallery
struct GalleryEntry: Codable, Identifiable, Hashable {
    var id: String { entryId }
    
    let entryId: String
    let name: String
    let description: String
    let icon: String?
    let category: String
    let provider: String
    let installType: String
    let tags: [String]?
    let featured: Bool
    
    enum CodingKeys: String, CodingKey {
        case entryId = "id"
        case name, description, icon, category, provider
        case installType = "install_type"
        case tags, featured
    }
    
    // Hashable conformance
    func hash(into hasher: inout Hasher) {
        hasher.combine(entryId)
    }
    
    static func == (lhs: GalleryEntry, rhs: GalleryEntry) -> Bool {
        lhs.entryId == rhs.entryId
    }
    
    /// Provider display name with proper capitalization
    var providerDisplayName: String {
        switch provider.lowercased() {
        case "anthropic": return "Anthropic"
        case "google": return "Google"
        case "microsoft": return "Microsoft"
        case "openai": return "OpenAI"
        case "sst": return "SST"
        default: return provider.capitalized
        }
    }
}

/// Request to install an agent from the gallery
struct GalleryInstallRequest: Codable {
    let name: String?
    let workdir: String?
}

/// Response from installing an agent
struct GalleryInstallResponse: Codable {
    let status: String
    let agent: String
    let installCmd: String?
    
    enum CodingKeys: String, CodingKey {
        case status, agent
        case installCmd = "install_cmd"
    }
}

/// Represents a remote sub-agent available from an ACP server
struct RemoteAgentInfo: Codable, Identifiable {
    var id: String { configId }
    
    let configId: String
    let name: String
    let description: String?
    let options: [String]?
    
    enum CodingKeys: String, CodingKey {
        case configId = "id"
        case name, description, options
    }
}

/// Request to update an agent's configuration
struct AgentUpdateRequest: Codable {
    let subAgent: String?
    let enabled: Bool?
    let description: String?
    let workdir: String?
    
    enum CodingKeys: String, CodingKey {
        case subAgent = "sub_agent"
        case enabled, description, workdir
    }
}
