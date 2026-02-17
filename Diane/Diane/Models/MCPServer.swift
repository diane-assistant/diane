import Foundation

/// Represents an MCP server configuration
struct MCPServer: Codable, Identifiable {
    let id: Int64
    var name: String
    var enabled: Bool
    var type: String  // stdio, sse, http, builtin
    var command: String?
    var args: [String]?
    var env: [String: String]?
    var url: String?
    var headers: [String: String]?
    var oauth: OAuthConfig?
    var nodeID: String?
    var nodeMode: String?
    let createdAt: Date
    let updatedAt: Date
    
    enum CodingKeys: String, CodingKey {
        case id
        case name
        case enabled
        case type
        case command
        case args
        case env
        case url
        case headers
        case oauth
        case nodeID = "node_id"
        case nodeMode = "node_mode"
        case createdAt = "created_at"
        case updatedAt = "updated_at"
    }
    
    /// Whether this server is a built-in provider
    var isBuiltin: Bool {
        type == "builtin"
    }
}

/// OAuth configuration for an MCP server
struct OAuthConfig: Codable {
    var provider: String?
    var clientID: String?
    var clientSecret: String?
    var scopes: [String]?
    var deviceAuthURL: String?
    var tokenURL: String?
    
    enum CodingKeys: String, CodingKey {
        case provider
        case clientID = "client_id"
        case clientSecret = "client_secret"
        case scopes
        case deviceAuthURL = "device_auth_url"
        case tokenURL = "token_url"
    }
}

/// Server type enum for validation
enum MCPServerType: String, CaseIterable, Identifiable {
    case stdio
    case sse
    case http
    case builtin
    
    var id: String { rawValue }
    
    var displayName: String {
        switch self {
        case .stdio: return "Standard I/O"
        case .sse: return "Server-Sent Events"
        case .http: return "HTTP"
        case .builtin: return "Built-in"
        }
    }
    
    var description: String {
        switch self {
        case .stdio: return "Communicate via stdin/stdout (local process)"
        case .sse: return "Connect via Server-Sent Events"
        case .http: return "Connect via HTTP"
        case .builtin: return "Built-in server (managed by Diane)"
        }
    }
}

/// Node mode enum for MCP server deployment
enum MCPNodeMode: String, CaseIterable, Identifiable {
    case master
    case specific
    case any
    
    var id: String { rawValue }
    
    var displayName: String {
        switch self {
        case .master: return "Master Node"
        case .specific: return "Specific Node"
        case .any: return "Any Available Node"
        }
    }
    
    var description: String {
        switch self {
        case .master: return "Run on the master/main node"
        case .specific: return "Run on a specific slave node"
        case .any: return "Run on any available slave node"
        }
    }
}

/// Request body for creating an MCP server
struct CreateMCPServerRequest: Codable {
    let name: String
    let enabled: Bool
    let type: String
    let command: String?
    let args: [String]?
    let env: [String: String]?
    let url: String?
    let headers: [String: String]?
    let oauth: OAuthConfig?
    let nodeID: String?
    let nodeMode: String?
    
    enum CodingKeys: String, CodingKey {
        case name, enabled, type, command, args, env, url, headers, oauth
        case nodeID = "node_id"
        case nodeMode = "node_mode"
    }
}

/// Request body for updating an MCP server
struct UpdateMCPServerRequest: Codable {
    let name: String?
    let enabled: Bool?
    let type: String?
    let command: String?
    let args: [String]?
    let env: [String: String]?
    let url: String?
    let headers: [String: String]?
    let oauth: OAuthConfig?
    let nodeID: String?
    let nodeMode: String?
    
    enum CodingKeys: String, CodingKey {
        case name, enabled, type, command, args, env, url, headers, oauth
        case nodeID = "node_id"
        case nodeMode = "node_mode"
    }
}
