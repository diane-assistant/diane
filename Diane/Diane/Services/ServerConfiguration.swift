import SwiftUI

/// Manages the server connection configuration, persisted via @AppStorage (UserDefaults).
@MainActor
@Observable
class ServerConfiguration {
    /// The host to connect to (e.g., "my-mac.tail12345.ts.net" or "192.168.1.100")
    var host: String {
        didSet { UserDefaults.standard.set(host, forKey: "diane_server_host") }
    }

    /// The port to connect to (default 8080)
    var port: Int {
        didSet { UserDefaults.standard.set(port, forKey: "diane_server_port") }
    }

    /// Whether a server address has been configured at least once
    var isConfigured: Bool {
        !host.isEmpty
    }

    /// Construct the base URL from host and port
    var baseURL: URL? {
        guard !host.isEmpty else { return nil }
        return URL(string: "http://\(host):\(port)")
    }

    init() {
        self.host = UserDefaults.standard.string(forKey: "diane_server_host") ?? ""
        let storedPort = UserDefaults.standard.integer(forKey: "diane_server_port")
        self.port = storedPort > 0 ? storedPort : 8080
    }

    /// Reset the configuration (for disconnect/change server)
    func reset() {
        host = ""
        port = 8080
    }
}
