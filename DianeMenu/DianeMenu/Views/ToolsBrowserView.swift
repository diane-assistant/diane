import SwiftUI

struct ToolsBrowserView: View {
    @State private var tools: [ToolInfo] = []
    @State private var isLoading = true
    @State private var error: String?
    @State private var searchText = ""
    @State private var selectedServer: String?
    
    private let client = DianeClient()
    
    private var servers: [String] {
        Array(Set(tools.map { $0.server })).sorted()
    }
    
    private var filteredTools: [ToolInfo] {
        var result = tools
        
        // Filter by server
        if let server = selectedServer {
            result = result.filter { $0.server == server }
        }
        
        // Filter by search text
        if !searchText.isEmpty {
            result = result.filter {
                $0.name.localizedCaseInsensitiveContains(searchText) ||
                $0.description.localizedCaseInsensitiveContains(searchText)
            }
        }
        
        return result
    }
    
    private var groupedTools: [(server: String, tools: [ToolInfo])] {
        let grouped = Dictionary(grouping: filteredTools) { $0.server }
        return grouped.keys.sorted().map { server in
            (server: server, tools: grouped[server]!.sorted { $0.name < $1.name })
        }
    }
    
    var body: some View {
        VStack(spacing: 0) {
            // Header with search and filter
            headerView
            
            Divider()
            
            if isLoading {
                loadingView
            } else if let error = error {
                errorView(error)
            } else if tools.isEmpty {
                emptyView
            } else {
                toolsListView
            }
        }
        .frame(minWidth: 500, idealWidth: 600, maxWidth: .infinity,
               minHeight: 400, idealHeight: 500, maxHeight: .infinity)
        .task {
            await loadTools()
        }
    }
    
    // MARK: - Header
    
    private var headerView: some View {
        HStack(spacing: 12) {
            // Search field
            HStack(spacing: 6) {
                Image(systemName: "magnifyingglass")
                    .foregroundStyle(.secondary)
                TextField("Search tools...", text: $searchText)
                    .textFieldStyle(.plain)
                if !searchText.isEmpty {
                    Button {
                        searchText = ""
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
            Picker("Server", selection: $selectedServer) {
                Text("All Servers").tag(nil as String?)
                Divider()
                ForEach(servers, id: \.self) { server in
                    Text(server).tag(server as String?)
                }
            }
            .pickerStyle(.menu)
            .frame(width: 150)
            
            // Refresh button
            Button {
                Task { await loadTools() }
            } label: {
                Image(systemName: "arrow.clockwise")
            }
            .disabled(isLoading)
            
            // Stats
            Text("\(filteredTools.count) tools")
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
                Task { await loadTools() }
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
    
    // MARK: - Empty View
    
    private var emptyView: some View {
        VStack(spacing: 12) {
            Image(systemName: "wrench.and.screwdriver")
                .font(.largeTitle)
                .foregroundStyle(.secondary)
            Text("No tools found")
                .font(.headline)
            if !searchText.isEmpty || selectedServer != nil {
                Text("Try adjusting your filters")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
    
    // MARK: - Tools List
    
    private var toolsListView: some View {
        ScrollView {
            LazyVStack(alignment: .leading, spacing: 16, pinnedViews: .sectionHeaders) {
                ForEach(groupedTools, id: \.server) { group in
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
    
    // MARK: - Actions
    
    private func loadTools() async {
        isLoading = true
        error = nil
        
        do {
            tools = try await client.getTools()
        } catch {
            self.error = error.localizedDescription
        }
        
        isLoading = false
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
