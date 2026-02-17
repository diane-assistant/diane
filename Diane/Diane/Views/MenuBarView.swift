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
                
                Divider()
                    .padding(.vertical, 8)
                
                // Open Diane button
                openDianeButton
                
                Divider()
                    .padding(.vertical, 8)
            } else {
                // Disconnected state
                disconnectedSection
                
                Divider()
                    .padding(.vertical, 8)
            }
            
            // Footer
            footerSection
        }
        .padding(12)
        .frame(width: 300)
    }
    
    // MARK: - Server Status Section
    
    private var serverStatusSection: some View {
        VStack(alignment: .leading, spacing: 8) {
            // Master server (local/current)
            MasterServerRowWithRestart(
                displayName: statusMonitor.serverDisplayName,
                isConnected: statusMonitor.connectionState == .connected,
                isRemoteMode: statusMonitor.isRemoteMode,
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
    
    // MARK: - Open Diane Button
    
    private var openDianeButton: some View {
        Button {
            // Properly dismiss the menu bar popover
            dismiss()
            
            // Small delay to ensure popover closes before opening window
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.15) {
                MainWindowView.openMainWindow()
            }
        } label: {
            HStack(spacing: 8) {
                Image(systemName: "macwindow")
                    .font(.subheadline)
                    .foregroundStyle(.blue)
                Text("Open Diane")
                    .font(.subheadline.weight(.medium))
                
                Spacer()
                
                Image(systemName: "arrow.up.right")
                    .font(.caption2)
                    .foregroundStyle(.secondary)
            }
            .padding(.vertical, 8)
            .padding(.horizontal, 10)
            .background(Color.blue.opacity(0.1))
            .cornerRadius(6)
        }
        .buttonStyle(.plain)
        .help("Open Diane main window")
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
            Spacer()
            
            Button("Quit Diane") {
                // Force quit by calling exit() since terminate is intercepted
                // This is the only way for users to fully quit the app
                exit(0)
            }
            .keyboardShortcut("q", modifiers: [.command, .option])
        }
        .font(.subheadline)
    }
}

// MARK: - Server Rows with Restart on Hover

/// Master server row with restart button on hover
struct MasterServerRowWithRestart: View {
    let displayName: String
    let isConnected: Bool
    let isRemoteMode: Bool
    let isRestarting: Bool
    let onRestart: () -> Void
    
    @State private var isHovered = false
    @State private var blinkOpacity: Double = 1.0
    
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
            
            // Restart button (visible on hover)
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
        .padding(.horizontal, 4)
        .padding(.vertical, 6)
        .background(Color.primary.opacity(0.03))
        .cornerRadius(6)
        .onHover { hovering in
            isHovered = hovering
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
        .padding(.horizontal, 4)
        .padding(.vertical, 6)
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
