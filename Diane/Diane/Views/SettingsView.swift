import SwiftUI
import ServiceManagement

struct SettingsView: View {
    @EnvironmentObject var statusMonitor: StatusMonitor
    @Environment(ServerConfiguration.self) private var serverConfig
    @AppStorage("launchAtLogin") private var launchAtLogin = false
    @AppStorage("autoStartDiane") private var autoStartDiane = true
    @AppStorage("pollInterval") private var pollInterval = 5.0
    @State private var showingChangeServerAlert = false
    @State private var showingRevokeAlert = false
    @State private var slaveToRevoke: SlaveInfo?
    
    var body: some View {
        Form {
            Section("General") {
                Toggle("Launch at Login", isOn: $launchAtLogin)
                    .onChange(of: launchAtLogin) { _, newValue in
                        updateLaunchAtLogin(newValue)
                    }
                
                if !statusMonitor.isRemoteMode {
                    Toggle("Auto-start Diane", isOn: $autoStartDiane)
                        .help("Automatically start Diane when this app launches")
                }
                
                Picker("Status Poll Interval", selection: $pollInterval) {
                    Text("1 second").tag(1.0)
                    Text("5 seconds").tag(5.0)
                    Text("10 seconds").tag(10.0)
                    Text("30 seconds").tag(30.0)
                }
            }
            
            Section("Server") {
                if statusMonitor.isRemoteMode {
                    LabeledContent("Mode") {
                        Label("Remote", systemImage: "network")
                            .font(.system(.body))
                            .foregroundStyle(.secondary)
                    }
                    
                    LabeledContent("Host") {
                        Text(serverConfig.host)
                            .font(.system(.body, design: .monospaced))
                            .foregroundStyle(.secondary)
                    }
                    
                    LabeledContent("Port") {
                        Text("\(serverConfig.port)")
                            .font(.system(.body, design: .monospaced))
                            .foregroundStyle(.secondary)
                    }
                    
                    LabeledContent("API Key") {
                        Text(serverConfig.apiKey.isEmpty ? "None" : "Configured")
                            .font(.system(.body))
                            .foregroundStyle(.secondary)
                    }
                } else {
                    LabeledContent("Mode") {
                        Label("Local", systemImage: "desktopcomputer")
                            .font(.system(.body))
                            .foregroundStyle(.secondary)
                    }
                    
                    LabeledContent("Binary Path") {
                        Text("~/.diane/bin/diane")
                            .font(.system(.body, design: .monospaced))
                            .foregroundStyle(.secondary)
                    }
                    
                    LabeledContent("Socket Path") {
                        Text("~/.diane/diane.sock")
                            .font(.system(.body, design: .monospaced))
                            .foregroundStyle(.secondary)
                    }
                    
                    LabeledContent("Database Path") {
                        Text("~/.diane/cron.db")
                            .font(.system(.body, design: .monospaced))
                            .foregroundStyle(.secondary)
                    }
                }
                
                Button("Change Server...", role: .destructive) {
                    showingChangeServerAlert = true
                }
                .alert("Change Server", isPresented: $showingChangeServerAlert) {
                    Button("Change", role: .destructive) {
                        changeServer()
                    }
                    Button("Cancel", role: .cancel) { }
                } message: {
                    Text("This will disconnect from the current server and return to the setup screen.")
                }
            }
            
            Section("Status") {
                if case .connected = statusMonitor.connectionState {
                    LabeledContent("Version") {
                        Text(statusMonitor.status.version)
                    }
                    LabeledContent("PID") {
                        Text("\(statusMonitor.status.pid)")
                    }
                    LabeledContent("Uptime") {
                        Text(statusMonitor.status.uptime)
                    }
                    LabeledContent("Total Tools") {
                        Text("\(statusMonitor.status.totalTools)")
                    }
                } else {
                    Text("Diane is not running")
                        .foregroundStyle(.secondary)
                }
            }
            
            // Slave Management
            if case .connected = statusMonitor.connectionState {
                // Pending Pairing Requests
                if !statusMonitor.pendingPairingRequests.isEmpty {
                    Section("Pending Pairing Requests") {
                        ForEach(statusMonitor.pendingPairingRequests) { request in
                            HStack {
                                Image(systemName: request.platformIcon)
                                    .foregroundStyle(.orange)
                                    .frame(width: 20)
                                
                                VStack(alignment: .leading, spacing: 2) {
                                    Text(request.hostname)
                                        .font(.system(.body, weight: .medium))
                                    HStack(spacing: 4) {
                                        Text(request.platformDisplay)
                                            .font(.caption)
                                            .foregroundStyle(.secondary)
                                        Text("Code: \(request.pairingCode)")
                                            .font(.system(.caption, design: .monospaced))
                                            .foregroundStyle(.secondary)
                                        if let expiresIn = request.expiresIn {
                                            Text("(\(expiresIn))")
                                                .font(.caption)
                                                .foregroundStyle(.secondary)
                                        }
                                    }
                                }
                                
                                Spacer()
                                
                                Button {
                                    Task {
                                        await statusMonitor.approvePairing(
                                            hostname: request.hostname,
                                            pairingCode: request.pairingCode
                                        )
                                    }
                                } label: {
                                    Text("Accept")
                                        .font(.caption)
                                }
                                .buttonStyle(.borderedProminent)
                                .tint(.green)
                                .controlSize(.small)
                                
                                Button(role: .destructive) {
                                    Task {
                                        await statusMonitor.denyPairing(
                                            hostname: request.hostname,
                                            pairingCode: request.pairingCode
                                        )
                                    }
                                } label: {
                                    Text("Deny")
                                        .font(.caption)
                                }
                                .controlSize(.small)
                            }
                            .padding(.vertical, 2)
                        }
                    }
                }
                
                // Connected/Registered Slaves
                Section {
                    if statusMonitor.slaves.isEmpty {
                        Text("No slave servers registered")
                            .foregroundStyle(.secondary)
                    } else {
                        ForEach(statusMonitor.slaves) { slave in
                            HStack {
                                // Status dot
                                Circle()
                                    .fill(slave.isConnected ? Color.green : Color.gray)
                                    .frame(width: 8, height: 8)
                                
                                Image(systemName: slave.platformIcon)
                                    .foregroundStyle(.secondary)
                                    .frame(width: 20)
                                
                                VStack(alignment: .leading, spacing: 2) {
                                    HStack(spacing: 4) {
                                        Text(slave.hostname)
                                            .font(.system(.body, weight: .medium))
                                        
                                        if !slave.enabled {
                                            Text("DISABLED")
                                                .font(.system(size: 9, weight: .semibold))
                                                .foregroundStyle(.white)
                                                .padding(.horizontal, 4)
                                                .padding(.vertical, 1)
                                                .background(Color.red.opacity(0.7))
                                                .cornerRadius(3)
                                        }
                                    }
                                    
                                    HStack(spacing: 4) {
                                        Text(slave.isConnected ? "Connected" : "Disconnected")
                                            .font(.caption)
                                            .foregroundStyle(slave.isConnected ? .green : .secondary)
                                        
                                        Text(slave.platformDisplay)
                                            .font(.caption)
                                            .foregroundStyle(.secondary)
                                        
                                        if slave.toolCount > 0 {
                                            Text("\(slave.toolCount) tools")
                                                .font(.caption)
                                                .foregroundStyle(.secondary)
                                        }
                                        
                                        if let lastSeen = slave.lastSeenFormatted, !slave.isConnected {
                                            Text("last seen \(lastSeen)")
                                                .font(.caption)
                                                .foregroundStyle(.secondary)
                                        }
                                    }
                                }
                                
                                Spacer()
                                
                                Button(role: .destructive) {
                                    slaveToRevoke = slave
                                    showingRevokeAlert = true
                                } label: {
                                    Image(systemName: "trash")
                                        .font(.caption)
                                }
                                .buttonStyle(.borderless)
                                .help("Revoke credentials for \(slave.hostname)")
                            }
                            .padding(.vertical, 2)
                        }
                    }
                } header: {
                    Text("Slave Servers")
                } footer: {
                    Text("Slave servers extend Diane's capabilities by connecting remote machines.")
                }
                .alert("Revoke Slave", isPresented: $showingRevokeAlert) {
                    Button("Revoke", role: .destructive) {
                        if let slave = slaveToRevoke {
                            Task {
                                await statusMonitor.revokeSlaveCredentials(hostname: slave.hostname)
                            }
                        }
                    }
                    Button("Cancel", role: .cancel) { }
                } message: {
                    Text("Revoke credentials for \(slaveToRevoke?.hostname ?? "")? This will disconnect the slave and require re-pairing.")
                }
            }
            
            Section("About") {
                LabeledContent("Diane Version") {
                    Text("1.0.0")
                }
                
                Link("Diane Documentation", destination: URL(string: "https://github.com/diane-assistant/diane")!)
            }
        }
        .formStyle(.grouped)
        .navigationTitle("Settings")
    }
    
    private func updateLaunchAtLogin(_ enabled: Bool) {
        do {
            if enabled {
                try SMAppService.mainApp.register()
            } else {
                try SMAppService.mainApp.unregister()
            }
        } catch {
            print("Failed to update launch at login: \(error)")
        }
    }
    
    /// Reset server config to trigger the setup flow again
    private func changeServer() {
        statusMonitor.stopPolling()
        serverConfig.reset()
    }
}

#Preview {
    SettingsView()
        .environmentObject(StatusMonitor())
        .environment(ServerConfiguration())
}
