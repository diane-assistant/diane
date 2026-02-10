import SwiftUI
import AppKit

struct MenuBarView: View {
    @EnvironmentObject var statusMonitor: StatusMonitor
    @EnvironmentObject var updateChecker: UpdateChecker
    @Environment(\.dismiss) private var dismiss
    @State private var isMCPServersExpanded = true
    @State private var authInProgress: String? = nil  // Server name being authenticated
    @State private var showingAuthAlert = false
    @State private var authUserCode: String = ""
    @State private var currentAuthServer: String? = nil
    
    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            // Update available banner
            if updateChecker.updateAvailable, let version = updateChecker.latestVersion {
                updateBanner(version: version)
                
                Divider()
                    .padding(.vertical, 8)
            }
            
            // Header with status and controls
            headerSection
            
            Divider()
                .padding(.vertical, 8)
            
            // Stats (only when connected)
            if case .connected = statusMonitor.connectionState {
                statsSection
                
                Divider()
                    .padding(.vertical, 8)
                
                // MCP Servers
                mcpServersSection
                
                Divider()
                    .padding(.vertical, 8)
            } else {
                // Debug: show connection state
                Text("Connection: \(String(describing: statusMonitor.connectionState))")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .padding(.vertical, 4)
            }
            
            // Footer
            footerSection
        }
        .padding(12)
        .frame(width: 280)
        .alert("Enter this code", isPresented: $showingAuthAlert) {
            Button("OK") { }
        } message: {
            Text("Code copied to clipboard:\n\n\(authUserCode)")
                .font(.body.monospaced())
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
    
    // MARK: - Header
    
    private var headerSection: some View {
        HStack(spacing: 10) {
            // Status indicator
            Circle()
                .fill(statusColor)
                .frame(width: 10, height: 10)
            
            // Status text and info
            VStack(alignment: .leading, spacing: 2) {
                Text(statusMonitor.connectionState.description)
                    .font(.headline)
                
                if case .connected = statusMonitor.connectionState {
                    Text("\(statusMonitor.status.version) \u{2022} up \(statusMonitor.status.uptime)")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            }
            
            Spacer()
            
            // Control buttons
            controlButtons
        }
    }
    
    private var controlButtons: some View {
        HStack(spacing: 2) {
            if statusMonitor.isLoading {
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
    
    // MARK: - Stats
    
    private var statsSection: some View {
        VStack(spacing: 4) {
            // Tools row
            Button {
                // Dismiss the menu bar popover first
                NSApp.keyWindow?.close()
                // Then open the tools browser
                WindowManager.shared.openToolsBrowser()
            } label: {
                HStack(spacing: 16) {
                    // Tools count
                    HStack(spacing: 4) {
                        Image(systemName: "wrench.and.screwdriver")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                        Text("\(statusMonitor.status.totalTools) tools")
                            .font(.subheadline)
                    }
                    
                    // MCP servers count
                    HStack(spacing: 4) {
                        Image(systemName: "server.rack")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                        let connectedCount = statusMonitor.status.mcpServers.filter { $0.connected }.count
                        let totalCount = statusMonitor.status.mcpServers.count
                        Text("\(connectedCount)/\(totalCount) MCP")
                            .font(.subheadline)
                    }
                    
                    Spacer()
                    
                    Image(systemName: "arrow.up.right")
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                }
                .padding(.vertical, 4)
                .padding(.horizontal, 6)
                .background(Color.primary.opacity(0.05))
                .cornerRadius(6)
            }
            .buttonStyle(.plain)
            .help("Browse all tools")
            
            // Scheduler row
            Button {
                NSApp.keyWindow?.close()
                WindowManager.shared.openScheduler()
            } label: {
                HStack(spacing: 16) {
                    HStack(spacing: 4) {
                        Image(systemName: "calendar.badge.clock")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                        Text("Scheduler")
                            .font(.subheadline)
                    }
                    
                    Spacer()
                    
                    Image(systemName: "arrow.up.right")
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                }
                .padding(.vertical, 4)
                .padding(.horizontal, 6)
                .background(Color.primary.opacity(0.05))
                .cornerRadius(6)
            }
            .buttonStyle(.plain)
            .help("View scheduled jobs")
            
            // Agents row
            Button {
                NSApp.keyWindow?.close()
                WindowManager.shared.openAgents()
            } label: {
                HStack(spacing: 16) {
                    HStack(spacing: 4) {
                        Image(systemName: "person.3.fill")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                        Text("ACP Agents")
                            .font(.subheadline)
                    }
                    
                    Spacer()
                    
                    Image(systemName: "arrow.up.right")
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                }
                .padding(.vertical, 4)
                .padding(.horizontal, 6)
                .background(Color.primary.opacity(0.05))
                .cornerRadius(6)
            }
            .buttonStyle(.plain)
            .help("Manage ACP agents")
            
            HStack {
                Spacer()
                
                // Reload config button
                Button {
                    Task { await statusMonitor.reloadConfig() }
                } label: {
                    Image(systemName: "arrow.triangle.2.circlepath")
                        .font(.caption)
                }
                .buttonStyle(.plain)
                .help("Reload Configuration")
                .disabled(statusMonitor.isLoading)
            }
        }
    }
    
    // MARK: - MCP Servers
    
    private var mcpServersSection: some View {
        VStack(alignment: .leading, spacing: 6) {
            Button {
                withAnimation(.easeInOut(duration: 0.2)) {
                    isMCPServersExpanded.toggle()
                }
            } label: {
                HStack {
                    Image(systemName: isMCPServersExpanded ? "chevron.down" : "chevron.right")
                        .font(.caption2)
                        .frame(width: 10)
                    Text("MCP Servers")
                        .font(.subheadline.weight(.medium))
                    Spacer()
                }
            }
            .buttonStyle(.plain)
            
            if isMCPServersExpanded {
                if statusMonitor.status.mcpServers.isEmpty {
                    Text("No MCP servers configured")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .padding(.leading, 14)
                } else {
                    ForEach(statusMonitor.status.mcpServers) { server in
                        MCPServerRow(
                            server: server,
                            onRestart: {
                                Task {
                                    await statusMonitor.restartMCPServer(name: server.name)
                                }
                            },
                            onSignIn: {
                                startAuthFlow(serverName: server.name)
                            }
                        )
                    }
                    .padding(.leading, 14)
                }
            }
        }
    }
    
    // MARK: - Footer
    
    private var footerSection: some View {
        HStack {
            SettingsLink {
                Label("Settings", systemImage: "gear")
            }
            
            Spacer()
            
            // Check for updates button
            Button {
                Task { await updateChecker.checkForUpdates() }
            } label: {
                if updateChecker.isChecking {
                    ProgressView()
                        .scaleEffect(0.5)
                        .frame(width: 14, height: 14)
                } else {
                    Image(systemName: "arrow.triangle.2.circlepath")
                }
            }
            .buttonStyle(.plain)
            .help("Check for updates")
            .disabled(updateChecker.isChecking)
            
            Button("Quit") {
                NSApplication.shared.terminate(nil)
            }
        }
        .font(.subheadline)
    }
    
    // MARK: - Auth Flow
    
    private func startAuthFlow(serverName: String) {
        Task {
            authInProgress = serverName
            if let deviceInfo = await statusMonitor.startAuth(serverName: serverName) {
                currentAuthServer = serverName
                authUserCode = deviceInfo.userCode
                
                // Copy the user code to clipboard for convenience
                NSPasteboard.general.clearContents()
                NSPasteboard.general.setString(deviceInfo.userCode, forType: .string)
                
                // Show the alert with the code
                showingAuthAlert = true
                
                // Open the verification URL in the browser
                if let url = URL(string: deviceInfo.verificationUri) {
                    NSWorkspace.shared.open(url)
                }
                
                // Start polling for token in background
                Task {
                    let success = await statusMonitor.pollAuthAndRefresh(
                        serverName: serverName,
                        deviceCode: deviceInfo.deviceCode,
                        interval: deviceInfo.interval
                    )
                    authInProgress = nil
                    showingAuthAlert = false
                    authUserCode = ""
                    currentAuthServer = nil
                    
                    if !success {
                        // Auth failed or timed out - status monitor already set lastError
                    }
                }
            } else {
                authInProgress = nil
            }
        }
    }
}

// MARK: - MCP Server Row

struct MCPServerRow: View {
    let server: MCPServerStatus
    let onRestart: () -> Void
    let onSignIn: () -> Void
    
    var body: some View {
        VStack(alignment: .leading, spacing: 2) {
            HStack(spacing: 8) {
                Circle()
                    .fill(serverColor)
                    .frame(width: 6, height: 6)
                
                Text(server.name)
                    .font(.subheadline)
                
                if server.builtin {
                    Text("builtin")
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                        .padding(.horizontal, 4)
                        .padding(.vertical, 1)
                        .background(Color.secondary.opacity(0.15))
                        .cornerRadius(3)
                }
                
                if server.connected {
                    Text("\(server.toolCount)")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                
                Spacer()
                
                if !server.enabled {
                    Text("off")
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                } else if server.needsAuthentication {
                    // Show Sign In button for servers that need auth
                    Button {
                        onSignIn()
                    } label: {
                        Text("Sign In")
                            .font(.caption2)
                            .foregroundStyle(.white)
                            .padding(.horizontal, 6)
                            .padding(.vertical, 2)
                            .background(Color.orange)
                            .cornerRadius(3)
                    }
                    .buttonStyle(.plain)
                    .help("Sign in to \(server.name)")
                } else if server.error != nil && !server.connected {
                    Image(systemName: "exclamationmark.triangle.fill")
                        .font(.caption2)
                        .foregroundStyle(.orange)
                }
                
                // Only show restart button for non-builtin servers
                if server.enabled && !server.builtin && !server.needsAuthentication {
                    Button {
                        onRestart()
                    } label: {
                        Image(systemName: "arrow.clockwise")
                            .font(.caption2)
                    }
                    .buttonStyle(.plain)
                    .help("Restart \(server.name)")
                }
            }
            
            // Show error message below the server row (but not for auth errors)
            if let error = server.error, !server.connected, server.enabled, !server.needsAuthentication {
                Text(error)
                    .font(.caption2)
                    .foregroundStyle(.orange)
                    .lineLimit(2)
                    .padding(.leading, 14)
            }
        }
        .padding(.vertical, 1)
    }
    
    private var serverColor: Color {
        if !server.enabled {
            return .secondary
        }
        if server.needsAuthentication {
            return .orange
        }
        return server.connected ? .green : .red
    }
}

#Preview {
    MenuBarView()
        .environmentObject(StatusMonitor())
        .environmentObject(UpdateChecker())
}
