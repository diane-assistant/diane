import SwiftUI

/// Popover showing master server, connected slaves, and pending pairing requests
struct ServerListPopover: View {
    @ObservedObject var statusMonitor: StatusMonitor
    let onClose: () -> Void
    
    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            // Header
            Text("Servers")
                .font(.caption)
                .foregroundStyle(.secondary)
                .padding(.horizontal, 12)
                .padding(.top, 12)
                .padding(.bottom, 8)
            
            Divider()
            
            ScrollView {
                VStack(alignment: .leading, spacing: 4) {
                    // Master server (local/current)
                    MasterServerRow(
                        displayName: statusMonitor.serverDisplayName,
                        isConnected: statusMonitor.connectionState == .connected,
                        isRemoteMode: statusMonitor.isRemoteMode
                    )
                    .padding(.horizontal, 8)
                    .padding(.vertical, 4)
                    
                    // Connected slaves
                    if !statusMonitor.slaves.isEmpty {
                        Divider()
                            .padding(.vertical, 4)
                        
                        ForEach(statusMonitor.slaves) { slave in
                            SlaveServerRow(slave: slave)
                                .padding(.horizontal, 8)
                                .padding(.vertical, 4)
                        }
                    }
                    
                    // Pending pairing requests
                    if !statusMonitor.pendingPairingRequests.isEmpty {
                        Divider()
                            .padding(.vertical, 4)
                        
                        Text("Pending Requests")
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                            .padding(.horizontal, 12)
                            .padding(.top, 4)
                        
                        ForEach(statusMonitor.pendingPairingRequests) { request in
                            PairingRequestRow(
                                request: request,
                                onAccept: {
                                    Task {
                                        await statusMonitor.approvePairing(
                                            hostname: request.hostname,
                                            pairingCode: request.pairingCode
                                        )
                                    }
                                },
                                onDeny: {
                                    Task {
                                        await statusMonitor.denyPairing(
                                            hostname: request.hostname,
                                            pairingCode: request.pairingCode
                                        )
                                    }
                                }
                            )
                            .padding(.horizontal, 8)
                            .padding(.vertical, 4)
                        }
                    }
                }
                .padding(.vertical, 8)
            }
            .frame(maxHeight: 400)
        }
        .frame(width: 300)
    }
}

/// Row showing the master server
struct MasterServerRow: View {
    let displayName: String
    let isConnected: Bool
    let isRemoteMode: Bool
    
    var body: some View {
        HStack(spacing: 10) {
            // Platform icon
            Image(systemName: isRemoteMode ? "network" : "laptopcomputer")
                .font(.system(size: 16))
                .foregroundStyle(.secondary)
                .frame(width: 20)
            
            VStack(alignment: .leading, spacing: 2) {
                HStack(spacing: 4) {
                    Text(displayName)
                        .font(.system(size: 13, weight: .medium))
                    
                    Text("(Master)")
                        .font(.system(size: 11))
                        .foregroundStyle(.secondary)
                }
                
                Text(isRemoteMode ? "Remote" : "Local")
                    .font(.system(size: 11))
                    .foregroundStyle(.secondary)
            }
            
            Spacer()
            
            // Status indicator
            Circle()
                .fill(isConnected ? Color.green : Color.gray)
                .frame(width: 8, height: 8)
        }
        .padding(.horizontal, 4)
        .padding(.vertical, 6)
        .background(Color.primary.opacity(0.03))
        .cornerRadius(6)
    }
}

/// Row showing a connected slave server
struct SlaveServerRow: View {
    let slave: SlaveInfo
    
    var body: some View {
        HStack(spacing: 10) {
            // Platform icon
            Image(systemName: slave.platformIcon)
                .font(.system(size: 16))
                .foregroundStyle(.secondary)
                .frame(width: 20)
            
            VStack(alignment: .leading, spacing: 2) {
                Text(slave.hostname)
                    .font(.system(size: 13))
                
                HStack(spacing: 4) {
                    Text(slave.platformDisplay)
                        .font(.system(size: 11))
                        .foregroundStyle(.secondary)
                    
                    if slave.toolCount > 0 {
                        Text("•")
                            .font(.system(size: 11))
                            .foregroundStyle(.secondary)
                        
                        Text("\(slave.toolCount) tools")
                            .font(.system(size: 11))
                            .foregroundStyle(.secondary)
                    }
                }
            }
            
            Spacer()
            
            // Status indicator
            Circle()
                .fill(slave.isConnected ? Color.green : Color.gray)
                .frame(width: 8, height: 8)
        }
        .padding(.horizontal, 4)
        .padding(.vertical, 6)
        .background(Color.primary.opacity(0.02))
        .cornerRadius(6)
    }
}

/// Row showing a pending pairing request with Accept button
struct PairingRequestRow: View {
    let request: PairingRequest
    let onAccept: () -> Void
    let onDeny: () -> Void
    
    @State private var showingDenyConfirmation = false
    
    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 10) {
                // Platform icon
                Image(systemName: request.platformIcon)
                    .font(.system(size: 16))
                    .foregroundStyle(.orange)
                    .frame(width: 20)
                
                VStack(alignment: .leading, spacing: 2) {
                    Text(request.hostname)
                        .font(.system(size: 13, weight: .medium))
                    
                    Text(request.platformDisplay)
                        .font(.system(size: 11))
                        .foregroundStyle(.secondary)
                }
                
                Spacer()
            }
            
            // Pairing code
            HStack {
                Text("Code:")
                    .font(.system(size: 11))
                    .foregroundStyle(.secondary)
                
                Text(request.pairingCode)
                    .font(.system(size: 13, weight: .semibold, design: .monospaced))
                    .foregroundStyle(.primary)
                
                Spacer()
                
                if let expiresIn = request.expiresIn {
                    Text("expires \(expiresIn)")
                        .font(.system(size: 10))
                        .foregroundStyle(.secondary)
                }
            }
            
            // Action buttons
            HStack(spacing: 8) {
                Button(action: onAccept) {
                    HStack(spacing: 4) {
                        Image(systemName: "checkmark")
                            .font(.system(size: 10, weight: .semibold))
                        Text("Accept")
                            .font(.system(size: 12, weight: .medium))
                    }
                    .foregroundStyle(.white)
                    .padding(.horizontal, 12)
                    .padding(.vertical, 6)
                    .background(Color.green)
                    .cornerRadius(6)
                }
                .buttonStyle(.plain)
                .help("Accept pairing request")
                
                Button(action: {
                    showingDenyConfirmation = true
                }) {
                    Text("Deny")
                        .font(.system(size: 12))
                        .foregroundStyle(.secondary)
                        .padding(.horizontal, 12)
                        .padding(.vertical, 6)
                }
                .buttonStyle(.plain)
                .help("Deny pairing request")
                .confirmationDialog(
                    "Deny pairing request from \(request.hostname)?",
                    isPresented: $showingDenyConfirmation,
                    titleVisibility: .visible
                ) {
                    Button("Deny", role: .destructive) {
                        onDeny()
                    }
                    Button("Cancel", role: .cancel) {}
                }
            }
        }
        .padding(.horizontal, 8)
        .padding(.vertical, 10)
        .background(Color.orange.opacity(0.08))
        .cornerRadius(8)
    }
}

#Preview("With Slaves and Requests") {
    let monitor = StatusMonitor()
    monitor.serverDisplayName = "Local"
    monitor.connectionState = .connected
    monitor.slaves = [
        SlaveInfo(
            hostname: "macbook-pro",
            status: "connected",
            toolCount: 12,
            lastSeen: nil,
            connectedAt: ISO8601DateFormatter().string(from: Date()),
            certSerial: "123456",
            issuedAt: ISO8601DateFormatter().string(from: Date()),
            expiresAt: ISO8601DateFormatter().string(from: Date().addingTimeInterval(86400 * 365)),
            enabled: true,
            platform: "darwin"
        ),
        SlaveInfo(
            hostname: "linux-server",
            status: "connected",
            toolCount: 8,
            lastSeen: nil,
            connectedAt: ISO8601DateFormatter().string(from: Date()),
            certSerial: "789012",
            issuedAt: ISO8601DateFormatter().string(from: Date()),
            expiresAt: ISO8601DateFormatter().string(from: Date().addingTimeInterval(86400 * 365)),
            enabled: true,
            platform: "linux"
        )
    ]
    monitor.pendingPairingRequests = [
        PairingRequest(
            hostname: "new-macbook",
            pairingCode: "123-456",
            status: "pending",
            createdAt: ISO8601DateFormatter().string(from: Date()),
            expiresAt: ISO8601DateFormatter().string(from: Date().addingTimeInterval(600)),
            platform: "darwin"
        )
    ]
    
    return ServerListPopover(statusMonitor: monitor, onClose: {})
        .frame(width: 300, height: 500)
}

#Preview("Master Only") {
    let monitor = StatusMonitor()
    monitor.serverDisplayName = "Local"
    monitor.connectionState = .connected
    
    return ServerListPopover(statusMonitor: monitor, onClose: {})
        .frame(width: 300, height: 200)
}
