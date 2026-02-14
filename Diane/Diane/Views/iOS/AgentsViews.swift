import SwiftUI

/// List of configured agents with status info.
struct AgentsListView: View {
    @Environment(\.dianeClient) private var client

    @State private var agents: [AgentConfig] = []
    @State private var searchText = ""
    @State private var isLoading = true

    private var filteredAgents: [AgentConfig] {
        if searchText.isEmpty { return agents }
        return agents.filter { $0.name.localizedCaseInsensitiveContains(searchText) }
    }

    var body: some View {
        Group {
            if isLoading && agents.isEmpty {
                ProgressView("Loading agents...")
            } else if agents.isEmpty {
                EmptyStateView(
                    icon: "cpu",
                    title: "No Agents",
                    description: "No agents are configured"
                )
            } else {
                List(filteredAgents) { agent in
                    NavigationLink {
                        AgentDetailView(agent: agent)
                    } label: {
                        AgentRow(agent: agent)
                    }
                }
                .searchable(text: $searchText, prompt: "Search agents")
            }
        }
        .navigationTitle("Agents")
        .refreshable { await refresh() }
        .task { await refresh() }
    }

    private func refresh() async {
        guard let client else { return }
        agents = (try? await client.getAgents()) ?? []
        isLoading = false
    }
}

// MARK: - Agent Row

private struct AgentRow: View {
    let agent: AgentConfig

    var body: some View {
        HStack {
            VStack(alignment: .leading, spacing: 2) {
                Text(agent.displayName)
                    .font(.body)
                HStack(spacing: 6) {
                    if let type = agent.type {
                        Text(type)
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                    if let workspace = agent.workspaceName {
                        Text("@\(workspace)")
                            .font(.caption)
                            .foregroundStyle(.tertiary)
                    }
                }
            }
            Spacer()
            if agent.enabled {
                Image(systemName: "circle.fill")
                    .font(.caption2)
                    .foregroundStyle(.green)
            } else {
                Image(systemName: "circle.slash")
                    .font(.caption2)
                    .foregroundStyle(.secondary)
            }
        }
    }
}

// MARK: - Agent Detail

struct AgentDetailView: View {
    let agent: AgentConfig

    var body: some View {
        List {
            DetailSection(title: "Overview") {
                InfoRow(label: "Name", value: agent.name)
                InfoRow(label: "Enabled", value: agent.enabled ? "Yes" : "No")
                if let type = agent.type {
                    InfoRow(label: "Type", value: type)
                }
                if let desc = agent.description, !desc.isEmpty {
                    InfoRow(label: "Description", value: desc)
                }
            }

            DetailSection(title: "Connection") {
                if let url = agent.url, !url.isEmpty {
                    InfoRow(label: "URL", value: url)
                }
                if let command = agent.command, !command.isEmpty {
                    InfoRow(label: "Command", value: command)
                }
                if let args = agent.args, !args.isEmpty {
                    InfoRow(label: "Arguments", value: args.joined(separator: " "))
                }
                if let port = agent.port {
                    InfoRow(label: "Port", value: "\(port)")
                }
                if let workdir = agent.workdir, !workdir.isEmpty {
                    InfoRow(label: "Working Directory", value: workdir)
                }
            }

            if let subAgent = agent.subAgent, !subAgent.isEmpty {
                DetailSection(title: "Sub-Agent") {
                    InfoRow(label: "Sub-Agent", value: subAgent)
                }
            }

            if let env = agent.env, !env.isEmpty {
                DetailSection(title: "Environment Variables") {
                    ForEach(env.sorted(by: { $0.key < $1.key }), id: \.key) { key, value in
                        InfoRow(label: key, value: value)
                    }
                }
            }

            if let tags = agent.tags, !tags.isEmpty {
                DetailSection(title: "Tags") {
                    Text(tags.joined(separator: ", "))
                        .font(.body)
                        .foregroundStyle(.secondary)
                }
            }
        }
        .navigationTitle(agent.displayName)
    }
}
