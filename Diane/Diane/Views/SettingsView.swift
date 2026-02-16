import SwiftUI
import ServiceManagement

struct SettingsView: View {
    @EnvironmentObject var statusMonitor: StatusMonitor
    @Environment(ServerConfiguration.self) private var serverConfig
    @AppStorage("launchAtLogin") private var launchAtLogin = false
    @AppStorage("autoStartDiane") private var autoStartDiane = true
    @AppStorage("pollInterval") private var pollInterval = 5.0
    @State private var showingChangeServerAlert = false
    
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
