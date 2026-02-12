import SwiftUI
import ServiceManagement

struct SettingsView: View {
    @EnvironmentObject var statusMonitor: StatusMonitor
    @AppStorage("launchAtLogin") private var launchAtLogin = false
    @AppStorage("autoStartDiane") private var autoStartDiane = true
    @AppStorage("pollInterval") private var pollInterval = 5.0
    
    var body: some View {
        Form {
            Section("General") {
                Toggle("Launch at Login", isOn: $launchAtLogin)
                    .onChange(of: launchAtLogin) { _, newValue in
                        updateLaunchAtLogin(newValue)
                    }
                
                Toggle("Auto-start Diane", isOn: $autoStartDiane)
                    .help("Automatically start Diane when this app launches")
                
                Picker("Status Poll Interval", selection: $pollInterval) {
                    Text("1 second").tag(1.0)
                    Text("5 seconds").tag(5.0)
                    Text("10 seconds").tag(10.0)
                    Text("30 seconds").tag(30.0)
                }
            }
            
            Section("Diane") {
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
                
                LabeledContent("Config Path") {
                    Text("~/.diane/mcp-servers.json")
                        .font(.system(.body, design: .monospaced))
                        .foregroundStyle(.secondary)
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
                LabeledContent("DianeMenu Version") {
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
}

#Preview {
    SettingsView()
        .environmentObject(StatusMonitor())
}
