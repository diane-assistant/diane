import SwiftUI

struct ToolsBrowserView: View {
    @EnvironmentObject var statusMonitor: StatusMonitor
    @State private var viewModel: ToolsBrowserViewModel

    init(viewModel: ToolsBrowserViewModel = ToolsBrowserViewModel()) {
        _viewModel = State(initialValue: viewModel)
    }

    var body: some View {
        VStack(spacing: 0) {
            // Header with search and filter
            headerView
            
            Divider()
            
            if viewModel.isLoading {
                loadingView
            } else if let error = viewModel.error {
                errorView(error)
            } else if viewModel.tools.isEmpty {
                emptyView
            } else {
                toolsListView
            }
        }
        .frame(minWidth: 500, idealWidth: 600, maxWidth: .infinity,
               minHeight: 400, idealHeight: 500, maxHeight: .infinity)
        .onAppear {
            // Only load tools when view appears AND we're connected
            FileLogger.shared.info("onAppear, connectionState=\(statusMonitor.connectionState)", category: "ToolsBrowserView")
            if case .connected = statusMonitor.connectionState {
                Task {
                    await viewModel.loadTools()
                }
            }
        }
        .onChange(of: statusMonitor.connectionState) {
            // Load tools when connection is established
            FileLogger.shared.info("connectionState changed to \(statusMonitor.connectionState)", category: "ToolsBrowserView")
            if case .connected = statusMonitor.connectionState, viewModel.tools.isEmpty {
                Task {
                    await viewModel.loadTools()
                }
            }
        }
    }
    
    // MARK: - Header
    
    private var headerView: some View {
        HStack(spacing: 12) {
            Image(systemName: "wrench.and.screwdriver")
                .foregroundStyle(.secondary)
            
            Text("Tools")
                .font(.headline)
            
            Spacer()
            
            // Search field
            HStack(spacing: 6) {
                Image(systemName: "magnifyingglass")
                    .foregroundStyle(.secondary)
                TextField("Search tools...", text: $viewModel.searchText)
                    .textFieldStyle(.plain)
                if !viewModel.searchText.isEmpty {
                    Button {
                        viewModel.searchText = ""
                    } label: {
                        Image(systemName: "xmark.circle.fill")
                            .foregroundStyle(.secondary)
                    }
                    .buttonStyle(.plain)
                }
            }
            .padding(8)
            .background(Color(nsColor: .textBackgroundColor))
            .cornerRadius(8)
            
            // Server filter
            Picker("Server", selection: $viewModel.selectedServer) {
                Text("All Servers").tag(nil as String?)
                Divider()
                ForEach(viewModel.servers, id: \.self) { server in
                    Text(server).tag(server as String?)
                }
            }
            .pickerStyle(.menu)
            .frame(width: 150)
            
            // Refresh button
            Button {
                Task { await viewModel.loadTools() }
            } label: {
                Image(systemName: "arrow.clockwise")
            }
            .disabled(viewModel.isLoading)
            
            // Stats
            Text("\(viewModel.filteredTools.count) tools")
                .font(.caption)
                .foregroundStyle(.secondary)
        }
        .padding()
    }
    
    // MARK: - Loading View
    
    private var loadingView: some View {
        VStack(spacing: 12) {
            ProgressView()
            Text("Loading tools...")
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
            Text("Failed to load tools")
                .font(.headline)
            Text(message)
                .font(.caption)
                .foregroundStyle(.secondary)
            Button("Retry") {
                Task { await viewModel.loadTools() }
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
    
    // MARK: - Empty View
    
    private var emptyView: some View {
        EmptyStateView(
            icon: "wrench.and.screwdriver",
            title: "No tools found",
            description: (!viewModel.searchText.isEmpty || viewModel.selectedServer != nil)
                ? "Try adjusting your filters"
                : nil
        )
    }
    
    // MARK: - Tools List
    
    private var toolsListView: some View {
        ScrollView {
            LazyVStack(alignment: .leading, spacing: 16, pinnedViews: .sectionHeaders) {
                ForEach(viewModel.groupedTools, id: \.server) { group in
                    Section {
                        ForEach(group.tools) { tool in
                            ToolRow(tool: tool)
                        }
                    } header: {
                        ServerSectionHeader(
                            name: group.server,
                            toolCount: group.tools.count,
                            isBuiltin: group.tools.first?.builtin ?? false
                        )
                    }
                }
            }
            .padding()
        }
    }
}

// MARK: - Server Section Header

struct ServerSectionHeader: View {
    let name: String
    let toolCount: Int
    let isBuiltin: Bool
    
    var body: some View {
        HStack(spacing: 8) {
            Image(systemName: isBuiltin ? "building.2.fill" : "server.rack")
                .font(.caption)
                .foregroundStyle(.secondary)
            
            Text(name)
                .font(.subheadline.weight(.semibold))
            
            if isBuiltin {
                Text("builtin")
                    .font(.caption2)
                    .foregroundStyle(.secondary)
                    .padding(.horizontal, 4)
                    .padding(.vertical, 2)
                    .background(Color.secondary.opacity(0.15))
                    .cornerRadius(4)
            }
            
            Text("\(toolCount)")
                .font(.caption)
                .foregroundStyle(.secondary)
            
            Spacer()
        }
        .padding(.vertical, 6)
        .padding(.horizontal, 8)
        .background(Color(nsColor: .windowBackgroundColor).opacity(0.95))
    }
}

// MARK: - Tool Row

struct ToolRow: View {
    let tool: ToolInfo
    @State private var isHovering = false
    @State private var isExpanded = false
    
    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack(spacing: 8) {
                // Tool icon
                Image(systemName: "wrench.fill")
                    .font(.caption)
                    .foregroundStyle(.blue)
                    .frame(width: 16)
                
                // Tool name
                Text(tool.name)
                    .font(.system(.body, design: .monospaced))
                    .fontWeight(.medium)
                
                Spacer()
                
                // Copy button (shown on hover)
                if isHovering {
                    Button {
                        NSPasteboard.general.clearContents()
                        NSPasteboard.general.setString(tool.name, forType: .string)
                    } label: {
                        Image(systemName: "doc.on.doc")
                            .font(.caption)
                    }
                    .buttonStyle(.plain)
                    .help("Copy tool name")
                }
            }
            
            // Description
            Text(tool.description)
                .font(.caption)
                .foregroundStyle(.secondary)
                .lineLimit(isExpanded ? nil : 2)
                .onTapGesture {
                    withAnimation(.easeInOut(duration: 0.15)) {
                        isExpanded.toggle()
                    }
                }
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 8)
        .background(isHovering ? Color.blue.opacity(0.05) : Color.clear)
        .cornerRadius(6)
        .onHover { hovering in
            isHovering = hovering
        }
    }
}

// MARK: - Preview

#Preview {
    ToolsBrowserView()
}
