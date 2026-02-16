import SwiftUI

/// Connection mode: local Unix socket or remote HTTP server
enum ConnectionMode: String, CaseIterable {
    case local = "local"
    case remote = "remote"
    
    /// Not yet chosen (first launch)
    static let unconfiguredKey = "diane_connection_mode"
}

/// Manages the server connection configuration, persisted via UserDefaults.
@MainActor
@Observable
class ServerConfiguration {
    /// The connection mode (local or remote). Empty string means unconfigured (first launch).
    var connectionModeRaw: String {
        didSet { UserDefaults.standard.set(connectionModeRaw, forKey: ConnectionMode.unconfiguredKey) }
    }
    
    /// The host to connect to (e.g., "my-mac.tail12345.ts.net" or "192.168.1.100")
    var host: String {
        didSet { UserDefaults.standard.set(host, forKey: "diane_server_host") }
    }

    /// The port to connect to (default 9090)
    var port: Int {
        didSet { UserDefaults.standard.set(port, forKey: "diane_server_port") }
    }
    
    /// API key for authenticating with the remote server (optional)
    var apiKey: String {
        didSet { UserDefaults.standard.set(apiKey, forKey: "diane_server_api_key") }
    }
    
    /// Whether a connection mode has been chosen at all (false on first launch)
    var isConfigured: Bool {
        !connectionModeRaw.isEmpty
    }
    
    /// The active connection mode, or nil if not yet configured
    var connectionMode: ConnectionMode? {
        ConnectionMode(rawValue: connectionModeRaw)
    }
    
    /// Whether we're in local mode
    var isLocal: Bool {
        connectionMode == .local
    }
    
    /// Whether we're in remote mode
    var isRemote: Bool {
        connectionMode == .remote
    }
    
    /// Whether the remote server config is complete (host is set)
    var isRemoteConfigured: Bool {
        isRemote && !host.isEmpty
    }

    /// Construct the base URL from host and port
    var baseURL: URL? {
        guard !host.isEmpty else { return nil }
        return URL(string: "http://\(host):\(port)")
    }

    init() {
        self.connectionModeRaw = UserDefaults.standard.string(forKey: ConnectionMode.unconfiguredKey) ?? ""
        self.host = UserDefaults.standard.string(forKey: "diane_server_host") ?? ""
        let storedPort = UserDefaults.standard.integer(forKey: "diane_server_port")
        self.port = storedPort > 0 ? storedPort : 9090
        self.apiKey = UserDefaults.standard.string(forKey: "diane_server_api_key") ?? ""
    }
    
    /// Set to local mode
    func setLocal() {
        connectionModeRaw = ConnectionMode.local.rawValue
    }
    
    /// Set to remote mode with the given server details
    func setRemote(host: String, port: Int, apiKey: String = "") {
        self.host = host
        self.port = port
        self.apiKey = apiKey
        self.connectionModeRaw = ConnectionMode.remote.rawValue
    }

    /// Reset the configuration (for disconnect/change server)
    func reset() {
        connectionModeRaw = ""
        host = ""
        port = 9090
        apiKey = ""
    }
}
