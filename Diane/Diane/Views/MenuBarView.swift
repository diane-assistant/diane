import SwiftUI
import AppKit

struct MenuBarView: View {
    @EnvironmentObject var statusMonitor: StatusMonitor
    @EnvironmentObject var updateChecker: UpdateChecker
    @Environment(\.dismiss) private var dismiss
    
    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            // Update available banner
            if updateChecker.updateAvailable, let version = updateChecker.latestVersion {
                updateBanner(version: version)
                
                Divider()
                    .padding(.vertical, 8)
            }
            
            // Master/Slave status section
            if case .connected = statusMonitor.connectionState {
                serverStatusSection
            } else {
                // Disconnected state
                disconnectedSection
            }
            
            Divider()
                .padding(.vertical, 8)
            
            // Footer
            footerSection
                .padding(.vertical, 4)
        }
        .padding(12)
        .frame(width: 300)
    }
    
    // MARK: - Server Status Section
    
    private var serverStatusSection: some View {
        VStack(alignment: .leading, spacing: 8) {
            // Master server (local/current)
            MasterServerRowWithRestart(
                displayName: statusMonitor.status.hostname ?? statusMonitor.serverDisplayName,
                isConnected: statusMonitor.connectionState == .connected,
                isRemoteMode: statusMonitor.isRemoteMode,
                remoteURL: statusMonitor.remoteURL,
                platform: statusMonitor.status.platformDisplay,
                architecture: statusMonitor.status.architecture,
                version: statusMonitor.status.version,
                isRestarting: statusMonitor.restartingMaster,
                onRestart: {
                    Task { await statusMonitor.restartDiane() }
                }
            )
            
            // Connected slaves
            if !statusMonitor.slaves.isEmpty {
                ForEach(statusMonitor.slaves) { slave in
                    SlaveServerRowWithActions(
                        slave: slave,
                        isRestarting: statusMonitor.restartingSlaves.contains(slave.hostname),
                        isUpgrading: statusMonitor.upgradingSlaves.contains(slave.hostname),
                        onRestart: {
                            Task {
                                do {
                                    try await statusMonitor.restartSlave(hostname: slave.hostname)
                                } catch {
                                    print("Failed to restart slave \(slave.hostname): \(error)")
                                }
                            }
                        },
                        onUpgrade: {
                            Task {
                                do {
                                    try await statusMonitor.upgradeSlave(hostname: slave.hostname)
                                } catch {
                                    print("Failed to upgrade slave \(slave.hostname): \(error)")
                                }
                            }
                        }
                    )
                }
            }
            
            // Pending pairing requests indicator
            if !statusMonitor.pendingPairingRequests.isEmpty {
                HStack(spacing: 8) {
                    Image(systemName: "exclamationmark.circle.fill")
                        .font(.system(size: 14))
                        .foregroundStyle(.orange)
                    
                    Text("\(statusMonitor.pendingPairingRequests.count) pending pairing request(s)")
                        .font(.caption)
                        .foregroundStyle(.orange)
                    
                    Spacer()
                }
                .padding(.vertical, 6)
                .padding(.horizontal, 8)
                .background(Color.orange.opacity(0.1))
                .cornerRadius(6)
            }
        }
    }
    
    // MARK: - Disconnected Section
    
    private var disconnectedSection: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 10) {
                Circle()
                    .fill(statusColor)
                    .frame(width: 10, height: 10)
                
                Text(statusMonitor.connectionState.description)
                    .font(.headline)
                
                Spacer()
                
                // Control buttons
                controlButtons
            }
        }
    }
    
    private var controlButtons: some View {
        HStack(spacing: 2) {
            if statusMonitor.isRemoteMode {
                // Remote mode: no start/stop/restart controls
                if case .connected = statusMonitor.connectionState {
                    Image(systemName: "network")
                        .font(.system(size: 12))
                        .foregroundStyle(.secondary)
                        .frame(width: 24, height: 24)
                        .help("Connected to remote server")
                }
            } else if statusMonitor.isLoading {
                ProgressView()
                    .scaleEffect(0.6)
                    .frame(width: 24, height: 24)
            } else if case .connected = statusMonitor.connectionState {
                // Restart button
                Button {
                    Task { await statusMonitor.restartDiane() }
                } label: {
                    Image(systemName: "arrow.clockwise")
                        .font(.system(size: 12))
                        .frame(width: 24, height: 24)
                }
                .buttonStyle(.plain)
                .help("Restart Diane")
                
                // Stop button
                Button {
                    Task { await statusMonitor.stopDiane() }
                } label: {
                    Image(systemName: "stop.fill")
                        .font(.system(size: 10))
                        .frame(width: 24, height: 24)
                }
                .buttonStyle(.plain)
                .help("Stop Diane")
            } else {
                // Start button
                Button {
                    Task { await statusMonitor.startDiane() }
                } label: {
                    Image(systemName: "play.fill")
                        .font(.system(size: 12))
                        .frame(width: 24, height: 24)
                }
                .buttonStyle(.plain)
                .help("Start Diane")
            }
        }
    }
    
    private var statusColor: Color {
        switch statusMonitor.connectionState {
        case .unknown:
            return .gray
        case .connected:
            return .green
        case .disconnected:
            return .gray
        case .error:
            return .orange
        }
    }
    
    // MARK: - Update Banner
    
    private func updateBanner(version: String) -> some View {
        VStack(spacing: 8) {
            if updateChecker.isUpdating {
                // Show update progress
                VStack(spacing: 6) {
                    HStack(spacing: 8) {
                        ProgressView()
                            .scaleEffect(0.7)
                        
                        Text(updateChecker.updateStatus)
                            .font(.subheadline.weight(.medium))
                        
                        Spacer()
                    }
                    
                    ProgressView(value: updateChecker.updateProgress)
                        .progressViewStyle(.linear)
                    
                    if let error = updateChecker.updateError {
                        Text(error)
                            .font(.caption)
                            .foregroundStyle(.red)
                    }
                }
                .padding(8)
                .background(Color.blue.opacity(0.1))
                .cornerRadius(6)
            } else {
                // Show update available button
                HStack(spacing: 8) {
                    Image(systemName: "arrow.up.circle.fill")
                        .foregroundStyle(.orange)
                    
                    VStack(alignment: .leading, spacing: 1) {
                        Text("Update Available")
                            .font(.subheadline.weight(.medium))
                        
                        // Show current -> new version
                        HStack(spacing: 4) {
                            Text(statusMonitor.status.version)
                                .font(.caption.monospaced())
                            Image(systemName: "arrow.right")
                                .font(.caption2)
                            Text(version)
                                .font(.caption.monospaced().weight(.medium))
                        }
                        .foregroundStyle(.secondary)
                    }
                    
                    Spacer()
                    
                    Text("Install")
                        .font(.caption.weight(.medium))
                        .foregroundStyle(.white)
                        .padding(.horizontal, 8)
                        .padding(.vertical, 4)
                        .background(Color.orange)
                        .cornerRadius(4)
                }
                .padding(8)
                .background(Color.orange.opacity(0.1))
                .cornerRadius(6)
                .contentShape(Rectangle())
                .onTapGesture {
                    Task { await updateChecker.performUpdate() }
                }
            }
        }
    }
    
    // MARK: - Footer
    
    private var footerSection: some View {
        HStack {
            // Quit button (power icon)
            Button {
                exit(0)
            } label: {
                Image(systemName: "power")
                    .font(.system(size: 13))
                    .foregroundStyle(.secondary)
            }
            .buttonStyle(.plain)
            .keyboardShortcut("q", modifiers: [.command, .option])
            .help("Quit Diane")
            
            Spacer()
            
            // Open Diane main window
            Button {
                dismiss()
                DispatchQueue.main.asyncAfter(deadline: .now() + 0.15) {
                    MainWindowView.openMainWindow()
                }
            } label: {
                HStack(spacing: 4) {
                    Text("Open Diane")
                        .font(.subheadline.weight(.medium))
                    Image(systemName: "arrow.up.right")
                        .font(.caption2)
                }
                .foregroundStyle(.blue)
            }
            .buttonStyle(.plain)
            .help("Open Diane main window")
        }
    }
}

// MARK: - Server Rows with Restart on Hover

/// Master server row with restart button on hover
struct MasterServerRowWithRestart: View {
    let displayName: String
    let isConnected: Bool
    let isRemoteMode: Bool
    let remoteURL: String?
    let platform: String
    let architecture: String?
    let version: String
    let isRestarting: Bool
    let onRestart: () -> Void
    
    @State private var isHovered = false
    @State private var blinkOpacity: Double = 1.0
    
    var body: some View {
        HStack(spacing: 10) {
            // Platform icon
            Image(systemName: isRemoteMode ? "server.rack" : platformIcon)
                .font(.system(size: 16))
                .foregroundStyle(.secondary)
                .frame(width: 20)
            
            VStack(alignment: .leading, spacing: 2) {
                HStack(spacing: 6) {
                    Text(displayName)
                        .font(.system(size: 13, weight: .medium))
                    
                    // Master badge
                    Text("Master")
                        .font(.system(size: 9, weight: .semibold))
                        .foregroundStyle(.white)
                        .padding(.horizontal, 5)
                        .padding(.vertical, 1)
                        .background(Color.blue.opacity(0.7))
                        .cornerRadius(3)
                }
                
                HStack(spacing: 4) {
                    Text(platform)
                        .font(.system(size: 11))
                        .foregroundStyle(.secondary)
                    
                    if let arch = architecture, !arch.isEmpty {
                        Text("•")
                            .font(.system(size: 11))
                            .foregroundStyle(.secondary)
                        
                        Text(arch)
                            .font(.system(size: 11))
                            .foregroundStyle(.secondary)
                    }
                    
                    if version != "unknown" {
                        Text("•")
                            .font(.system(size: 11))
                            .foregroundStyle(.secondary)
                        
                        Text(version)
                            .font(.system(size: 11))
                            .foregroundStyle(.secondary)
                    }
                    
                    if isRemoteMode {
                        Text("•")
                            .font(.system(size: 11))
                            .foregroundStyle(.secondary)
                        
                        Text("Remote")
                            .font(.system(size: 11))
                            .foregroundStyle(.secondary)
                    }
                }
            }
            
            Spacer()
            
            // Restart button (visible on hover, local mode only)
            if isHovered && isConnected && !isRemoteMode && !isRestarting {
                Button(action: onRestart) {
                    Image(systemName: "arrow.clockwise")
                        .font(.system(size: 12))
                        .foregroundStyle(.secondary)
                }
                .buttonStyle(.plain)
                .help("Restart Diane")
            }
            
            // Status indicator
            Circle()
                .fill(statusColor)
                .frame(width: 8, height: 8)
                .opacity(isRestarting ? blinkOpacity : 1.0)
                .onAppear {
                    if isRestarting {
                        startBlinking()
                    }
                }
                .onChange(of: isRestarting) { _, newValue in
                    if newValue {
                        startBlinking()
                    } else {
                        blinkOpacity = 1.0
                    }
                }
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 8)
        .background(Color.primary.opacity(0.03))
        .cornerRadius(6)
        .onHover { hovering in
            isHovered = hovering
        }
        .help(isRemoteMode ? (remoteURL ?? "Remote server") : "Local server")
    }
    
    private var platformIcon: String {
        switch platform {
        case "macOS": return "laptopcomputer"
        case "Linux": return "server.rack"
        case "Windows": return "desktopcomputer"
        default: return "questionmark.circle"
        }
    }
    
    private var statusColor: Color {
        if isRestarting {
            return .orange
        }
        return isConnected ? .green : .gray
    }
    
    private func startBlinking() {
        withAnimation(.easeInOut(duration: 0.6).repeatForever(autoreverses: true)) {
            blinkOpacity = 0.3
        }
    }
}

/// Slave server row with restart and upgrade buttons on hover
struct SlaveServerRowWithActions: View {
    let slave: SlaveInfo
    let isRestarting: Bool
    let isUpgrading: Bool
    let onRestart: () -> Void
    let onUpgrade: () -> Void
    
    @State private var isHovered = false
    @State private var blinkOpacity: Double = 1.0
    
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
                    
                    if let version = slave.version, !version.isEmpty {
                        Text("•")
                            .font(.system(size: 11))
                            .foregroundStyle(.secondary)
                        
                        Text(version)
                            .font(.system(size: 11))
                            .foregroundStyle(.secondary)
                    }
                    
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
            
            // Action buttons (visible on hover)
            if isHovered && slave.isConnected && !isRestarting && !isUpgrading {
                HStack(spacing: 6) {
                    // Upgrade button
                    Button(action: onUpgrade) {
                        Image(systemName: "arrow.up.circle")
                            .font(.system(size: 12))
                            .foregroundStyle(.secondary)
                    }
                    .buttonStyle(.plain)
                    .help("Upgrade \(slave.hostname) to latest version")
                    
                    // Restart button
                    Button(action: onRestart) {
                        Image(systemName: "arrow.clockwise")
                            .font(.system(size: 12))
                            .foregroundStyle(.secondary)
                    }
                    .buttonStyle(.plain)
                    .help("Restart \(slave.hostname)")
                }
            }
            
            // Status indicator
            Circle()
                .fill(statusColor)
                .frame(width: 8, height: 8)
                .opacity(isBlinking ? blinkOpacity : 1.0)
                .onAppear {
                    if isBlinking {
                        startBlinking()
                    }
                }
                .onChange(of: isBlinking) { _, newValue in
                    if newValue {
                        startBlinking()
                    } else {
                        blinkOpacity = 1.0
                    }
                }
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 8)
        .background(Color.primary.opacity(0.02))
        .cornerRadius(6)
        .onHover { hovering in
            isHovered = hovering
        }
    }
    
    private var isBlinking: Bool {
        isRestarting || isUpgrading
    }
    
    private var statusColor: Color {
        if isRestarting {
            return .orange
        }
        if isUpgrading {
            return .blue
        }
        return slave.isConnected ? .green : .gray
    }
    
    private func startBlinking() {
        withAnimation(.easeInOut(duration: 0.6).repeatForever(autoreverses: true)) {
            blinkOpacity = 0.3
        }
    }
}

#Preview {
    MenuBarView()
        .environmentObject(StatusMonitor())
        .environmentObject(UpdateChecker())
}
