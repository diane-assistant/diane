import Foundation

/// Represents a context for grouping MCP servers
struct Context: Codable, Identifiable {
    let id: Int64
    let name: String
    let description: String?
    let isDefault: Bool
    
    enum CodingKeys: String, CodingKey {
        case id
        case name
        case description
        case isDefault = "is_default"
    }
    
    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        id = try container.decode(Int64.self, forKey: .id)
        name = try container.decode(String.self, forKey: .name)
        description = try container.decodeIfPresent(String.self, forKey: .description)
        isDefault = try container.decodeIfPresent(Bool.self, forKey: .isDefault) ?? false
    }
}

/// Server in a context with its tool status
struct ContextServer: Codable, Identifiable {
    let id: Int64
    let name: String
    let type: String
    let enabled: Bool
    let toolsActive: Int
    let toolsTotal: Int
    let tools: [ContextTool]?
    
    enum CodingKeys: String, CodingKey {
        case id
        case name
        case type
        case enabled
        case toolsActive = "tools_active"
        case toolsTotal = "tools_total"
        case tools
    }
    
    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        id = try container.decode(Int64.self, forKey: .id)
        name = try container.decode(String.self, forKey: .name)
        type = try container.decodeIfPresent(String.self, forKey: .type) ?? ""
        enabled = try container.decode(Bool.self, forKey: .enabled)
        toolsActive = try container.decodeIfPresent(Int.self, forKey: .toolsActive) ?? 0
        toolsTotal = try container.decodeIfPresent(Int.self, forKey: .toolsTotal) ?? 0
        tools = try container.decodeIfPresent([ContextTool].self, forKey: .tools)
    }
}

/// Tool status within a context-server relationship
struct ContextTool: Codable, Identifiable {
    let name: String
    let description: String?
    let enabled: Bool
    
    var id: String { name }
}

/// Summary of a context's servers and tools
struct ContextSummary: Codable {
    let serversEnabled: Int
    let serversTotal: Int
    let toolsActive: Int
    let toolsTotal: Int
    
    enum CodingKeys: String, CodingKey {
        case serversEnabled = "servers_enabled"
        case serversTotal = "servers_total"
        case toolsActive = "tools_active"
        case toolsTotal = "tools_total"
    }
}

/// Full context detail response
struct ContextDetail: Codable {
    let context: Context
    let servers: [ContextServer]
    let summary: ContextSummary
}

/// Connection info for a context
struct ContextConnectInfo: Codable {
    let context: String
    let sse: ConnectionDetails
    let streamable: ConnectionDetails
    let description: String?
}

struct ConnectionDetails: Codable {
    let url: String
    let example: String?
}

/// Available server that can be added to a context
struct AvailableServer: Codable, Identifiable {
    let name: String
    let toolCount: Int
    let inContext: Bool
    let builtin: Bool?
    
    var id: String { name }
    
    enum CodingKeys: String, CodingKey {
        case name
        case toolCount = "tool_count"
        case inContext = "in_context"
        case builtin
    }
    
    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        name = try container.decode(String.self, forKey: .name)
        toolCount = try container.decodeIfPresent(Int.self, forKey: .toolCount) ?? 0
        inContext = try container.decodeIfPresent(Bool.self, forKey: .inContext) ?? false
        builtin = try container.decodeIfPresent(Bool.self, forKey: .builtin)
    }
}
