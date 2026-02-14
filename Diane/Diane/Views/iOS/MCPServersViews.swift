import SwiftUI

/// List of MCP servers with status indicators.
struct MCPServersListView: View {
    @Environment(\.dianeClient) private var client

    @State private var servers: [MCPServerStatus] = []
    @State private var configs: [MCPServer] = []
    @State private var searchText = ""
    @State private var isLoading = true

    private var filteredServers: [MCPServerStatus] {
        if searchText.isEmpty { return servers }
        return servers.filter { $0.name.localizedCaseInsensitiveContains(searchText) }
    }

    var body: some View {
        Group {
            if isLoading && servers.isEmpty {
                ProgressView("Loading servers...")
            } else if servers.isEmpty {
                EmptyStateView(
                    icon: "server.rack",
                    title: "No MCP Servers",
                    description: "No MCP servers are configured"
                )
            } else {
                List(filteredServers, id: \.name) { server in
                    NavigationLink {
                        MCPServerDetailView(server: server, config: configFor(server.name))
                    } label: {
                        MCPServerRow(server: server)
                    }
                }
                .searchable(text: $searchText, prompt: "Search servers")
            }
        }
        .navigationTitle("MCP Servers")
        .refreshable { await refresh() }
        .task { await refresh() }
    }

    private func configFor(_ name: String) -> MCPServer? {
        configs.first { $0.name == name }
    }

    private func refresh() async {
        guard let client else { return }
        async let serversResult = try? client.getMCPServers()
        async let configsResult = try? client.getMCPServerConfigs()
        servers = (await serversResult) ?? []
        configs = (await configsResult) ?? []
        isLoading = false
    }
}

// MARK: - Server Row

private struct MCPServerRow: View {
    let server: MCPServerStatus

    var body: some View {
        HStack {
            VStack(alignment: .leading, spacing: 2) {
                Text(server.name)
                    .font(.body)
                if server.builtin {
                    Text("Built-in")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            }
            Spacer()
            StatusBadge(connected: server.connected, enabled: server.enabled)
        }
    }
}

// MARK: - Status Badge

struct StatusBadge: View {
    let connected: Bool
    let enabled: Bool

    var body: some View {
        Text(statusText)
            .font(.caption2.bold())
            .padding(.horizontal, 8)
            .padding(.vertical, 3)
            .background(backgroundColor)
            .foregroundStyle(foregroundColor)
            .clipShape(Capsule())
    }

    private var statusText: String {
        if !enabled { return "disabled" }
        return connected ? "connected" : "disconnected"
    }

    private var backgroundColor: Color {
        if !enabled { return Color.gray.opacity(0.15) }
        return connected ? Color.green.opacity(0.15) : Color.red.opacity(0.15)
    }

    private var foregroundColor: Color {
        if !enabled { return .secondary }
        return connected ? .green : .red
    }
}

// MARK: - Server Detail

struct MCPServerDetailView: View {
    let server: MCPServerStatus
    let config: MCPServer?

    @Environment(\.dianeClient) private var client
    @State private var tools: [ToolInfo] = []

    var body: some View {
        List {
            DetailSection(title: "Status") {
                InfoRow(label: "Connected", value: server.connected ? "Yes" : "No")
                InfoRow(label: "Enabled", value: server.enabled ? "Yes" : "No")
                InfoRow(label: "Tools", value: "\(server.toolCount)")
                if server.builtin {
                    InfoRow(label: "Type", value: "Built-in")
                }
                if let error = server.error, !error.isEmpty {
                    InfoRow(label: "Error", value: error)
                }
                if server.requiresAuth {
                    InfoRow(label: "Auth Required", value: server.authenticated ? "Authenticated" : "Not Authenticated")
                }
            }

            if let config = config {
                DetailSection(title: "Configuration") {
                    InfoRow(label: "Type", value: config.type)
                    if let command = config.command, !command.isEmpty {
                        InfoRow(label: "Command", value: command)
                    }
                    if let args = config.args, !args.isEmpty {
                        InfoRow(label: "Arguments", value: args.joined(separator: " "))
                    }
                    if let url = config.url, !url.isEmpty {
                        InfoRow(label: "URL", value: url)
                    }
                }

                if let env = config.env, !env.isEmpty {
                    DetailSection(title: "Environment Variables") {
                        ForEach(env.sorted(by: { $0.key < $1.key }), id: \.key) { key, value in
                            InfoRow(label: key, value: value)
                        }
                    }
                }
            }

            if !tools.isEmpty {
                DetailSection(title: "Tools (\(tools.count))") {
                    ForEach(tools) { tool in
                        VStack(alignment: .leading, spacing: 2) {
                            Text(tool.name)
                                .font(.body)
                            if !tool.description.isEmpty {
                                Text(tool.description)
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                            }
                        }
                    }
                }
            }
        }
        .navigationTitle(server.name)
        .task {
            guard let client else { return }
            tools = (try? await client.getTools()) ?? []
            // Filter to only tools from this server
            tools = tools.filter { $0.server == server.name }
        }
    }
}
