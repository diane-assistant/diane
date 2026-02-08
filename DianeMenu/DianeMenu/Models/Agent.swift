import Foundation

/// Represents a configured ACP agent
struct AgentConfig: Codable, Identifiable {
    var id: String { name }
    
    let name: String
    let url: String?
    let type: String?
    let command: String?
    let args: [String]?
    let env: [String: String]?
    let workdir: String?
    let port: Int?
    let enabled: Bool
    let description: String?
    let tags: [String]?
    
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
}

/// Result of testing an agent connection
struct AgentTestResult: Codable {
    let name: String
    let url: String?
    let workdir: String?
    let enabled: Bool
    let status: String  // "connected", "unreachable", "error", "disabled"
    let error: String?
    let agentCount: Int?
    let agents: [String]?
    
    enum CodingKeys: String, CodingKey {
        case name, url, workdir, enabled, status, error
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
    let runId: String?
    let status: String?
    let output: String?
    let error: String?
    
    enum CodingKeys: String, CodingKey {
        case runId = "run_id"
        case status, output, error
    }
}
