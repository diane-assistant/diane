import Foundation

/// Information about a registered slave server
struct SlaveInfo: Codable, Identifiable, Hashable {
    var id: String { hostname }
    
    let hostname: String
    let status: String
    let version: String?
    let toolCount: Int
    let lastSeen: String?
    let connectedAt: String?
    let certSerial: String
    let issuedAt: String
    let expiresAt: String
    let enabled: Bool
    let platform: String?
    
    enum CodingKeys: String, CodingKey {
        case hostname
        case status
        case version
        case toolCount = "tool_count"
        case lastSeen = "last_seen"
        case connectedAt = "connected_at"
        case certSerial = "cert_serial"
        case issuedAt = "issued_at"
        case expiresAt = "expires_at"
        case enabled
        case platform
    }
    
    /// Check if the slave is currently connected
    var isConnected: Bool {
        status == "connected"
    }
    
    /// Get platform display name
    var platformDisplay: String {
        switch platform {
        case "darwin":
            return "macOS"
        case "linux":
            return "Linux"
        case "windows":
            return "Windows"
        default:
            return platform ?? "Unknown"
        }
    }
    
    /// Get SF Symbol name for platform
    var platformIcon: String {
        switch platform {
        case "darwin":
            return "laptopcomputer"
        case "linux":
            return "terminal"
        case "windows":
            return "desktopcomputer"
        default:
            return "server.rack"
        }
    }
    
    /// Get formatted last seen time
    var lastSeenFormatted: String? {
        guard let lastSeen = lastSeen,
              let date = ISO8601DateFormatter().date(from: lastSeen) else {
            return nil
        }
        
        let formatter = RelativeDateTimeFormatter()
        formatter.unitsStyle = .abbreviated
        return formatter.localizedString(for: date, relativeTo: Date())
    }
}

/// A pending pairing request from a slave
struct PairingRequest: Codable, Identifiable, Hashable {
    var id: String { hostname }
    
    let hostname: String
    let pairingCode: String
    let status: String
    let createdAt: String
    let expiresAt: String
    let platform: String?
    
    enum CodingKeys: String, CodingKey {
        case hostname
        case pairingCode = "pairing_code"
        case status
        case createdAt = "created_at"
        case expiresAt = "expires_at"
        case platform
    }
    
    /// Get platform display name
    var platformDisplay: String {
        switch platform {
        case "darwin":
            return "macOS"
        case "linux":
            return "Linux"
        case "windows":
            return "Windows"
        default:
            return platform ?? "Unknown"
        }
    }
    
    /// Get SF Symbol name for platform
    var platformIcon: String {
        switch platform {
        case "darwin":
            return "laptopcomputer"
        case "linux":
            return "terminal"
        case "windows":
            return "desktopcomputer"
        default:
            return "server.rack"
        }
    }
    
    /// Get formatted expiry time
    var expiresIn: String? {
        guard let date = ISO8601DateFormatter().date(from: expiresAt) else {
            return nil
        }
        
        let formatter = RelativeDateTimeFormatter()
        formatter.unitsStyle = .abbreviated
        return formatter.localizedString(for: date, relativeTo: Date())
    }
    
    /// Check if the request has expired
    var isExpired: Bool {
        guard let date = ISO8601DateFormatter().date(from: expiresAt) else {
            return false
        }
        return date < Date()
    }
}
