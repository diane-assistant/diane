import SwiftUI

/// Dashboard showing connection state and summary metrics.
struct StatusDashboardView: View {
    @Environment(IOSStatusMonitor.self) private var monitor
    @Environment(\.dianeClient) private var client

    @State private var mcpServers: [MCPServerStatus] = []
    @State private var agents: [AgentConfig] = []
    @State private var providers: [Provider] = []

    var body: some View {
        ScrollView {
            VStack(spacing: 16) {
                // Connection state
                connectionBanner

                if let status = monitor.status {
                    // Summary cards
                    LazyVGrid(columns: [
                        GridItem(.flexible()),
                        GridItem(.flexible())
                    ], spacing: 12) {
                        SummaryCard(
                            title: "MCP Servers",
                            value: "\(mcpServers.count)",
                            icon: "server.rack",
                            color: .blue
                        )

                        SummaryCard(
                            title: "Agents",
                            value: "\(agents.count)",
                            icon: "cpu",
                            color: .purple
                        )

                        SummaryCard(
                            title: "Providers",
                            value: "\(providers.count)",
                            icon: "cloud",
                            color: .orange
                        )

                        SummaryCard(
                            title: "Version",
                            value: status.version,
                            icon: "info.circle",
                            color: .green
                        )
                    }
                    .padding(.horizontal)

                    // Server status summary
                    if !mcpServers.isEmpty {
                        HStack(spacing: 16) {
                            Label("\(mcpServers.filter { $0.connected }.count) connected", systemImage: "circle.fill")
                                .foregroundStyle(.green)
                            Label("\(agents.filter { $0.enabled }.count) agents enabled", systemImage: "circle.fill")
                                .foregroundStyle(.blue)
                        }
                        .font(.caption)
                        .padding(.horizontal)
                    }
                } else if case .connecting = monitor.connectionState {
                    ProgressView("Connecting...")
                        .padding(.top, 40)
                } else {
                    EmptyStateView(
                        icon: "wifi.slash",
                        title: "Not Connected",
                        description: "Unable to reach the Diane server"
                    )
                }
            }
            .padding(.vertical)
        }
        .navigationTitle("Status")
        .refreshable {
            await refresh()
        }
        .task {
            await refresh()
        }
    }

    // MARK: - Connection Banner

    private var connectionBanner: some View {
        HStack(spacing: 8) {
            Circle()
                .fill(connectionColor)
                .frame(width: 8, height: 8)
            Text(connectionText)
                .font(.subheadline)
                .foregroundStyle(.secondary)
            Spacer()
        }
        .padding(.horizontal)
    }

    private var connectionColor: Color {
        switch monitor.connectionState {
        case .connected: .green
        case .connecting: .yellow
        case .disconnected: .red
        case .error: .red
        }
    }

    private var connectionText: String {
        switch monitor.connectionState {
        case .connected: "Connected"
        case .connecting: "Connecting..."
        case .disconnected: "Disconnected"
        case .error(let msg): "Error: \(msg)"
        }
    }

    // MARK: - Data Loading

    private func refresh() async {
        guard let client else { return }
        async let serversResult = try? client.getMCPServers()
        async let agentsResult = try? client.getAgents()
        async let providersResult = try? client.getProviders(type: nil)

        mcpServers = (await serversResult) ?? []
        agents = (await agentsResult) ?? []
        providers = (await providersResult) ?? []

        await monitor.poll()
    }
}
