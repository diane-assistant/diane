import Foundation

/// Information about a tool
struct ToolInfo: Codable, Identifiable {
    let name: String
    let description: String
    let server: String
    let builtin: Bool
    let inputSchema: JSONValue?
    
    var id: String { name }
    
    enum CodingKeys: String, CodingKey {
        case name, description, server, builtin
        case inputSchema = "input_schema"
    }
}

/// A prompt argument
struct PromptArgument: Codable, Identifiable {
    let name: String
    let description: String?
    let required: Bool?
    
    var id: String { name }
}

/// Information about a prompt
struct PromptInfo: Codable, Identifiable {
    let name: String
    let description: String
    let server: String
    let builtin: Bool
    let arguments: [PromptArgument]?
    
    var id: String { name }
}

/// A message in a prompt response
struct PromptMessage: Codable {
    let role: String
    let content: PromptMessageContent
}

/// Content of a prompt message
struct PromptMessageContent: Codable {
    let type: String
    let text: String
}

/// Response from /prompts/get endpoint
struct PromptContentResponse: Codable {
    let description: String?
    let messages: [PromptMessage]?
}

/// A resource content item from /resources/read
struct ResourceContentItem: Codable {
    let uri: String?
    let mimeType: String?
    let text: String?
    let blob: String?
}

/// Response from /resources/read endpoint
struct ResourceContentResponse: Codable {
    let contents: [ResourceContentItem]?
}

/// A type-erased JSON value for handling arbitrary JSON (like inputSchema)
enum JSONValue: Codable, Equatable {
    case string(String)
    case number(Double)
    case bool(Bool)
    case object([String: JSONValue])
    case array([JSONValue])
    case null
    
    init(from decoder: Decoder) throws {
        let container = try decoder.singleValueContainer()
        if let str = try? container.decode(String.self) {
            self = .string(str)
        } else if let num = try? container.decode(Double.self) {
            self = .number(num)
        } else if let b = try? container.decode(Bool.self) {
            self = .bool(b)
        } else if let obj = try? container.decode([String: JSONValue].self) {
            self = .object(obj)
        } else if let arr = try? container.decode([JSONValue].self) {
            self = .array(arr)
        } else if container.decodeNil() {
            self = .null
        } else {
            throw DecodingError.dataCorruptedError(in: container, debugDescription: "Cannot decode JSONValue")
        }
    }
    
    func encode(to encoder: Encoder) throws {
        var container = encoder.singleValueContainer()
        switch self {
        case .string(let s): try container.encode(s)
        case .number(let n): try container.encode(n)
        case .bool(let b): try container.encode(b)
        case .object(let o): try container.encode(o)
        case .array(let a): try container.encode(a)
        case .null: try container.encodeNil()
        }
    }
    
    /// Pretty-print JSON value for display
    var prettyDescription: String {
        switch self {
        case .string(let s): return "\"\(s)\""
        case .number(let n):
            if n == n.rounded() && n < 1e15 { return String(Int(n)) }
            return String(n)
        case .bool(let b): return b ? "true" : "false"
        case .null: return "null"
        case .array(let arr): return "[\(arr.map { $0.prettyDescription }.joined(separator: ", "))]"
        case .object(let obj):
            let pairs = obj.sorted(by: { $0.key < $1.key }).map { "\"\($0.key)\": \($0.value.prettyDescription)" }
            return "{\(pairs.joined(separator: ", "))}"
        }
    }
}

/// Information about a resource
struct ResourceInfo: Codable, Identifiable {
    let name: String
    let description: String
    let uri: String
    let mimeType: String?
    let server: String
    let builtin: Bool
    
    var id: String { name }
    
    enum CodingKeys: String, CodingKey {
        case name
        case description
        case uri
        case mimeType = "mime_type"
        case server
        case builtin
    }
}

/// Status of an MCP server (includes both builtin providers and external MCP servers)
struct MCPServerStatus: Codable, Identifiable {
    let name: String
    let enabled: Bool
    let connected: Bool
    let toolCount: Int
    let promptCount: Int
    let resourceCount: Int
    let error: String?
    let builtin: Bool
    let requiresAuth: Bool
    let authenticated: Bool
    
    var id: String { name }
    
    enum CodingKeys: String, CodingKey {
        case name
        case enabled
        case connected
        case toolCount = "tool_count"
        case promptCount = "prompt_count"
        case resourceCount = "resource_count"
        case error
        case builtin
        case requiresAuth = "requires_auth"
        case authenticated
    }
    
    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        name = try container.decode(String.self, forKey: .name)
        enabled = try container.decode(Bool.self, forKey: .enabled)
        connected = try container.decode(Bool.self, forKey: .connected)
        toolCount = try container.decode(Int.self, forKey: .toolCount)
        promptCount = try container.decodeIfPresent(Int.self, forKey: .promptCount) ?? 0
        resourceCount = try container.decodeIfPresent(Int.self, forKey: .resourceCount) ?? 0
        error = try container.decodeIfPresent(String.self, forKey: .error)
        builtin = try container.decodeIfPresent(Bool.self, forKey: .builtin) ?? false
        requiresAuth = try container.decodeIfPresent(Bool.self, forKey: .requiresAuth) ?? false
        authenticated = try container.decodeIfPresent(Bool.self, forKey: .authenticated) ?? false
    }
    
    init(
        name: String,
        enabled: Bool = true,
        connected: Bool = true,
        toolCount: Int = 0,
        promptCount: Int = 0,
        resourceCount: Int = 0,
        error: String? = nil,
        builtin: Bool = false,
        requiresAuth: Bool = false,
        authenticated: Bool = false
    ) {
        self.name = name
        self.enabled = enabled
        self.connected = connected
        self.toolCount = toolCount
        self.promptCount = promptCount
        self.resourceCount = resourceCount
        self.error = error
        self.builtin = builtin
        self.requiresAuth = requiresAuth
        self.authenticated = authenticated
    }
    
    /// Returns true if this server needs authentication but isn't authenticated yet
    var needsAuthentication: Bool {
        requiresAuth && !authenticated
    }
    
    var statusIcon: String {
        if !enabled {
            return "circle.slash"
        }
        if needsAuthentication {
            return "person.badge.key"
        }
        if connected {
            return "circle.fill"
        }
        return "exclamationmark.circle.fill"
    }
    
    var statusColor: String {
        if !enabled {
            return "secondary"
        }
        if needsAuthentication {
            return "orange"
        }
        if connected {
            return "green"
        }
        return "red"
    }
}

/// Full Diane status
struct DianeStatus: Codable {
    let running: Bool
    let pid: Int
    let version: String
    let uptime: String
    let uptimeSeconds: Int64
    let startedAt: Date
    let totalTools: Int
    let mcpServers: [MCPServerStatus]
    
    enum CodingKeys: String, CodingKey {
        case running
        case pid
        case version
        case uptime
        case uptimeSeconds = "uptime_seconds"
        case startedAt = "started_at"
        case totalTools = "total_tools"
        case mcpServers = "mcp_servers"
    }
    
    static let empty = DianeStatus(
        running: false,
        pid: 0,
        version: "unknown",
        uptime: "0s",
        uptimeSeconds: 0,
        startedAt: Date(),
        totalTools: 0,
        mcpServers: []
    )
}

/// Device code info for OAuth device flow
struct DeviceCodeInfo: Codable {
    let userCode: String
    let verificationUri: String
    let expiresIn: Int
    let interval: Int
    let deviceCode: String
    
    enum CodingKeys: String, CodingKey {
        case userCode = "user_code"
        case verificationUri = "verification_uri"
        case expiresIn = "expires_in"
        case interval
        case deviceCode = "device_code"
    }
}

/// Connection state for the UI
enum ConnectionState: Equatable {
    case unknown
    case connected
    case disconnected
    case error(String)
    
    var icon: String {
        switch self {
        case .unknown:
            return "questionmark.circle"
        case .connected:
            return "checkmark.circle.fill"
        case .disconnected:
            return "xmark.circle.fill"
        case .error:
            return "exclamationmark.triangle.fill"
        }
    }
    
    var color: String {
        switch self {
        case .unknown:
            return "secondary"
        case .connected:
            return "green"
        case .disconnected:
            return "secondary"
        case .error:
            return "red"
        }
    }
    
    var description: String {
        switch self {
        case .unknown:
            return "Checking..."
        case .connected:
            return "Running"
        case .disconnected:
            return "Not Running"
        case .error(let message):
            return "Error: \(message)"
        }
    }
}
