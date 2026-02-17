import SwiftUI

/// View for managing node-centric MCP server deployment (not definition).
///
/// This view allows users to:
/// - Select a node (master or slave)
/// - See all available MCP servers
/// - Toggle which servers are enabled on the selected node
///
/// Does NOT handle server definition/configuration - that's MCPRegistryView's job.
struct MCPServersView: View {
    @EnvironmentObject var statusMonitor: StatusMonitor
    @State private var viewModel: MCPServersViewModel
    @State private var clientInitialized = false
    
    init(viewModel: MCPServersViewModel = MCPServersViewModel()) {
        _viewModel = State(initialValue: viewModel)
    }
    
    var body: some View {
        VStack(spacing: 0) {
            headerView
            
            Divider()
            
            if viewModel.isLoadingHosts || viewModel.isLoadingServers {
                loadingView
            } else if let error = viewModel.hostsError ?? viewModel.serversError {
                errorView(error)
            } else if viewModel.allServers.isEmpty {
                emptyView
            } else {
                contentView
            }
        }
        .frame(minWidth: 700, idealWidth: 800, maxWidth: .infinity,
               minHeight: 400, idealHeight: 500, maxHeight: .infinity)
        .task {
            // Initialize with the correct client from StatusMonitor if available
            if !clientInitialized, let configuredClient = statusMonitor.configuredClient {
                viewModel = MCPServersViewModel(client: configuredClient)
                clientInitialized = true
            }
            await viewModel.loadData()
        }
    }
    
    // MARK: - Header
    
    private var headerView: some View {
        HStack(spacing: 12) {
            Image(systemName: "server.rack")
                .foregroundStyle(.secondary)
            
            Text("MCP Servers")
                .font(.headline)
            
            Spacer()
            
            // Node picker (only show if slaves exist)
            if !viewModel.isMasterOnly {
                Menu {
                    ForEach(viewModel.hosts) { host in
                        Button {
                            Task { await viewModel.selectHost(host) }
                        } label: {
                            HStack {
                                Label(host.displayName, systemImage: host.id == "master" ? "server.rack" : "externaldrive.connected.to.line.below")
                                
                                if !host.online {
                                    Text("(Offline)")
                                        .foregroundStyle(.secondary)
                                }
                                
                                if viewModel.selectedHost?.id == host.id {
                                    Image(systemName: "checkmark")
                                }
                            }
                        }
                        // Offline nodes are still selectable for pre-configuration
                    }
                } label: {
                    HStack(spacing: 6) {
                        Image(systemName: viewModel.selectedHost?.id == "master" ? "server.rack" : "externaldrive.connected.to.line.below")
                        Text(viewModel.selectedHost?.displayName ?? "Select Node")
                        Image(systemName: "chevron.down")
                            .font(.caption)
                    }
                }
                .frame(width: 200)
            }
            
            // Refresh button
            Button {
                Task { await viewModel.loadData() }
            } label: {
                Image(systemName: "arrow.clockwise")
            }
            .disabled(viewModel.isLoadingHosts || viewModel.isLoadingServers)
        }
        .padding()
    }
    
    // MARK: - Loading View
    
    private var loadingView: some View {
        VStack(spacing: 12) {
            ProgressView()
            Text("Loading server placements...")
                .font(.caption)
                .foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
    
    // MARK: - Error View
    
    private func errorView(_ message: String) -> some View {
        VStack(spacing: 12) {
            Image(systemName: "exclamationmark.triangle.fill")
                .font(.largeTitle)
                .foregroundStyle(.orange)
            Text("Failed to load placements")
                .font(.headline)
            Text(message)
                .font(.caption)
                .foregroundStyle(.secondary)
            Button("Retry") {
                Task { await viewModel.loadData() }
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
    
    // MARK: - Empty View
    
    private var emptyView: some View {
        EmptyStateView(
            icon: "server.rack",
            title: "No MCP servers defined",
            description: "Add MCP server definitions in the MCP Registry first",
            actionLabel: nil,
            action: nil
        )
    }
    
    // MARK: - Content View
    
    private var contentView: some View {
        VStack(spacing: 0) {
            // Info banner if master-only mode
            if viewModel.isMasterOnly {
                InfoBanner(
                    icon: "info.circle",
                    text: "Master-only mode. Pair slaves to enable multi-node deployment.",
                    color: .blue
                )
                Divider()
            }
            
            // Server list with toggle switches
            ScrollView {
                LazyVStack(spacing: 0) {
                    ForEach(viewModel.serversWithPlacementStatus, id: \.server.id) { item in
                        serverRow(server: item.server, isEnabled: item.isEnabledOnNode)
                        Divider()
                    }
                }
            }
        }
    }
    
    // MARK: - Server Row
    
    private func serverRow(server: MCPServer, isEnabled: Bool) -> some View {
        HStack(spacing: 12) {
            // Type icon
            Image(systemName: serverTypeIcon(server.type))
                .foregroundStyle(isEnabled ? .blue : .secondary)
                .frame(width: 24)
            
            VStack(alignment: .leading, spacing: 4) {
                HStack(spacing: 8) {
                    Text(server.name)
                        .font(.system(.body, design: .default))
                        .foregroundColor(.primary)
                    
                    if server.isBuiltin {
                        Text("Built-in")
                            .font(.system(size: 9, weight: .semibold))
                            .foregroundStyle(.white)
                            .padding(.horizontal, 5)
                            .padding(.vertical, 1)
                            .background(Color.blue.opacity(0.7))
                            .cornerRadius(3)
                    }
                }
                
                Text(serverTypeDisplayName(server.type))
                    .font(.caption)
                    .foregroundStyle(.secondary)
                
                // Show runtime capability counts if enabled and running
                if isEnabled {
                    if let status = statusMonitor.status.mcpServers.first(where: { $0.name == server.name }) {
                        HStack(spacing: 8) {
                            if status.toolCount > 0 {
                                HStack(spacing: 2) {
                                    Image(systemName: "wrench.fill")
                                        .font(.system(size: 8))
                                    Text("\(status.toolCount)")
                                        .font(.caption2)
                                }
                                .foregroundStyle(.blue)
                            }
                            if status.promptCount > 0 {
                                HStack(spacing: 2) {
                                    Image(systemName: "text.bubble.fill")
                                        .font(.system(size: 8))
                                    Text("\(status.promptCount)")
                                        .font(.caption2)
                                }
                                .foregroundStyle(.purple)
                            }
                            if status.resourceCount > 0 {
                                HStack(spacing: 2) {
                                    Image(systemName: "doc.fill")
                                        .font(.system(size: 8))
                                    Text("\(status.resourceCount)")
                                        .font(.caption2)
                                }
                                .foregroundStyle(.green)
                            }
                        }
                    }
                }
            }
            
            Spacer()
            
            // Toggle switch
            Toggle("", isOn: Binding(
                get: { isEnabled },
                set: { newValue in
                    Task {
                        await viewModel.toggleServerOnCurrentHost(server: server, enabled: newValue)
                    }
                }
            ))
            .labelsHidden()
            .disabled(viewModel.isLoadingPlacements)
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 12)
        .contentShape(Rectangle())
    }
    
    // MARK: - Helpers
    
    private func serverTypeIcon(_ type: String) -> String {
        switch MCPServerType(rawValue: type) {
        case .stdio: return "terminal"
        case .sse: return "antenna.radiowaves.left.and.right"
        case .http: return "network"
        case .builtin: return "cube.fill"
        case .none: return "server.rack"
        }
    }
    
    private func serverTypeDisplayName(_ type: String) -> String {
        MCPServerType(rawValue: type)?.displayName ?? type
    }
}

// MARK: - Info Banner

private struct InfoBanner: View {
    let icon: String
    let text: String
    let color: Color
    
    var body: some View {
        HStack(spacing: 12) {
            Image(systemName: icon)
                .foregroundStyle(color)
            Text(text)
                .font(.caption)
                .foregroundStyle(.secondary)
            Spacer()
        }
        .padding()
        .background(color.opacity(0.1))
    }
}
