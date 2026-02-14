import SwiftUI

/// List of contexts (MCP server groups).
struct ContextsListView: View {
    @Environment(\.dianeClient) private var client

    @State private var contexts: [Context] = []
    @State private var isLoading = true

    var body: some View {
        Group {
            if isLoading && contexts.isEmpty {
                ProgressView("Loading contexts...")
            } else if contexts.isEmpty {
                EmptyStateView(
                    icon: "rectangle.connected.to.line.below",
                    title: "No Contexts",
                    description: "No contexts are configured"
                )
            } else {
                List(contexts) { context in
                    NavigationLink {
                        ContextDetailView(context: context)
                    } label: {
                        ContextRow(context: context)
                    }
                }
            }
        }
        .navigationTitle("Contexts")
        .refreshable { await refresh() }
        .task { await refresh() }
    }

    private func refresh() async {
        guard let client else { return }
        contexts = (try? await client.getContexts()) ?? []
        isLoading = false
    }
}

// MARK: - Context Row

private struct ContextRow: View {
    let context: Context

    var body: some View {
        HStack {
            VStack(alignment: .leading, spacing: 2) {
                HStack(spacing: 6) {
                    Text(context.name)
                        .font(.body)
                    if context.isDefault {
                        Text("Default")
                            .font(.caption2)
                            .padding(.horizontal, 6)
                            .padding(.vertical, 1)
                            .background(Color.blue.opacity(0.1))
                            .foregroundStyle(.blue)
                            .clipShape(Capsule())
                    }
                }
                if let desc = context.description, !desc.isEmpty {
                    Text(desc)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                }
            }
            Spacer()
        }
    }
}

// MARK: - Context Detail

struct ContextDetailView: View {
    let context: Context

    @Environment(\.dianeClient) private var client
    @State private var detail: ContextDetail?
    @State private var connectInfo: ContextConnectInfo?
    @State private var isLoading = true

    var body: some View {
        List {
            DetailSection(title: "Overview") {
                InfoRow(label: "Name", value: context.name)
                if let desc = context.description, !desc.isEmpty {
                    InfoRow(label: "Description", value: desc)
                }
                InfoRow(label: "Default", value: context.isDefault ? "Yes" : "No")
            }

            if let detail = detail {
                DetailSection(title: "Summary") {
                    InfoRow(label: "Servers", value: "\(detail.summary.serversEnabled)/\(detail.summary.serversTotal) enabled")
                    InfoRow(label: "Tools", value: "\(detail.summary.toolsActive)/\(detail.summary.toolsTotal) active")
                }

                if !detail.servers.isEmpty {
                    DetailSection(title: "Servers (\(detail.servers.count))") {
                        ForEach(detail.servers) { server in
                            VStack(alignment: .leading, spacing: 4) {
                                HStack {
                                    Text(server.name)
                                        .font(.body)
                                    Spacer()
                                    if server.enabled {
                                        Text("\(server.toolsActive)/\(server.toolsTotal) tools")
                                            .font(.caption)
                                            .foregroundStyle(.secondary)
                                    } else {
                                        Text("disabled")
                                            .font(.caption)
                                            .foregroundStyle(.secondary)
                                    }
                                }
                                if let tools = server.tools, !tools.isEmpty {
                                    VStack(alignment: .leading, spacing: 2) {
                                        ForEach(tools) { tool in
                                            HStack(spacing: 4) {
                                                Image(systemName: tool.enabled ? "checkmark.circle.fill" : "circle")
                                                    .font(.caption2)
                                                    .foregroundStyle(tool.enabled ? .green : .secondary)
                                                Text(tool.name)
                                                    .font(.caption)
                                            }
                                        }
                                    }
                                    .padding(.leading, 4)
                                }
                            }
                        }
                    }
                }
            } else if isLoading {
                ProgressView()
            }

            if let info = connectInfo {
                DetailSection(title: "Connection") {
                    InfoRow(label: "SSE URL", value: info.sse.url)
                    InfoRow(label: "Streamable URL", value: info.streamable.url)
                }
            }
        }
        .navigationTitle(context.name)
        .task { await loadDetail() }
        .refreshable { await loadDetail() }
    }

    private func loadDetail() async {
        guard let client else { return }
        async let detailResult = try? client.getContextDetail(name: context.name)
        async let connectResult = try? client.getContextConnectInfo(name: context.name)
        detail = await detailResult
        connectInfo = await connectResult
        isLoading = false
    }
}
