import Foundation

/// Information about a tool
struct ToolInfo: Codable, Identifiable {
    let name: String
    let description: String
    let server: String
    let builtin: Bool
    
    var id: String { name }
}

/// Status of an MCP server (includes both builtin providers and external MCP servers)
struct MCPServerStatus: Codable, Identifiable {
    let name: String
    let enabled: Bool
    let connected: Bool
    let toolCount: Int
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
        error = try container.decodeIfPresent(String.self, forKey: .error)
        builtin = try container.decodeIfPresent(Bool.self, forKey: .builtin) ?? false
        requiresAuth = try container.decodeIfPresent(Bool.self, forKey: .requiresAuth) ?? false
        authenticated = try container.decodeIfPresent(Bool.self, forKey: .authenticated) ?? false
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
enum ConnectionState {
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
