import SwiftUI

/// iOS settings view — server connection management and app info.
struct IOSSettingsView: View {
    @Environment(ServerConfiguration.self) private var config
    @Environment(IOSStatusMonitor.self) private var monitor

    @State private var showDisconnectConfirmation = false

    var body: some View {
        List {
            // Server connection
            Section("Server Connection") {
                if let url = config.baseURL {
                    InfoRow(label: "Server", value: url.absoluteString)
                }
                InfoRow(label: "Host", value: config.host)
                InfoRow(label: "Port", value: "\(config.port)")
                InfoRow(label: "Auth", value: config.apiKey.isEmpty ? "None" : "API Key")

                HStack {
                    Text("Status")
                    Spacer()
                    HStack(spacing: 6) {
                        Circle()
                            .fill(statusColor)
                            .frame(width: 8, height: 8)
                        Text(statusText)
                            .foregroundStyle(.secondary)
                    }
                }
            }

            // Server info
            if let status = monitor.status {
                Section("Server Info") {
                    InfoRow(label: "Version", value: status.version)
                    InfoRow(label: "Uptime", value: status.uptime)
                    InfoRow(label: "PID", value: "\(status.pid)")
                    InfoRow(label: "Total Tools", value: "\(status.totalTools)")
                    InfoRow(label: "MCP Servers", value: "\(status.mcpServers.count)")
                }
            }

            // Disconnect
            Section {
                Button(role: .destructive) {
                    showDisconnectConfirmation = true
                } label: {
                    HStack {
                        Spacer()
                        Text("Disconnect from Server")
                        Spacer()
                    }
                }
            }

            // About
            Section("About") {
                InfoRow(label: "App", value: "Diane iOS")
                InfoRow(label: "Mode", value: config.apiKey.isEmpty ? "Read-Only" : "Full Access")
                InfoRow(label: "Connection", value: "HTTP")
            }
        }
        .navigationTitle("Settings")
        .confirmationDialog(
            "Disconnect from Server",
            isPresented: $showDisconnectConfirmation,
            titleVisibility: .visible
        ) {
            Button("Disconnect", role: .destructive) {
                monitor.stopPolling()
                config.reset()
            }
            Button("Cancel", role: .cancel) {}
        } message: {
            Text("This will clear the server address. You can reconnect by entering the address again.")
        }
    }

    private var statusColor: Color {
        switch monitor.connectionState {
        case .connected: .green
        case .connecting: .yellow
        case .disconnected, .error: .red
        }
    }

    private var statusText: String {
        switch monitor.connectionState {
        case .connected: "Connected"
        case .connecting: "Connecting..."
        case .disconnected: "Disconnected"
        case .error(let msg): "Error: \(msg)"
        }
    }
}
