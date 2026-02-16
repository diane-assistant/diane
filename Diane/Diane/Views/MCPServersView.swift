import SwiftUI

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
            
            if viewModel.isLoading {
                loadingView
            } else if let error = viewModel.error {
                errorView(error)
            } else if viewModel.servers.isEmpty {
                emptyView
            } else {
                MasterDetailView {
                    serversListView
                } detail: {
                    detailView
                }
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
        .sheet(isPresented: $viewModel.showCreateServer) {
            createServerSheet
        }
        .sheet(item: $viewModel.editingServer) { server in
            editServerSheet(for: server)
        }
        .alert("Delete Server", isPresented: $viewModel.showDeleteConfirm) {
            Button("Cancel", role: .cancel) { }
            Button("Delete", role: .destructive) {
                if let server = viewModel.serverToDelete {
                    Task { await viewModel.deleteServer(server) }
                }
            }
        } message: {
            if let server = viewModel.serverToDelete {
                Text("Are you sure you want to delete '\(server.name)'? This cannot be undone.")
            }
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
            
            // Type filter
            Picker("", selection: $viewModel.typeFilter) {
                Text("All").tag(nil as MCPServerType?)
                ForEach(MCPServerType.allCases) { type in
                    Text(type.displayName).tag(type as MCPServerType?)
                }
            }
            .pickerStyle(.menu)
            .frame(width: 150)
            
            // Create server button
            Button {
                viewModel.showCreateServer = true
            } label: {
                Label("Add Server", systemImage: "plus")
            }
            
            // Refresh button
            Button {
                Task { await viewModel.loadData() }
            } label: {
                Image(systemName: "arrow.clockwise")
            }
            .disabled(viewModel.isLoading)
        }
        .padding()
    }
    
    // MARK: - Loading View
    
    private var loadingView: some View {
        VStack(spacing: 12) {
            ProgressView()
            Text("Loading servers...")
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
            Text("Failed to load servers")
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
            title: "No MCP servers configured",
            description: "Add an MCP server to extend Diane's capabilities",
            actionLabel: "Add Server",
            action: { viewModel.showCreateServer = true }
        )
    }
    
    // MARK: - Servers List
    
    private var serversListView: some View {
        VStack(spacing: 0) {
            MasterListHeader(
                icon: "server.rack",
                title: "MCP Servers"
            )
            
            Divider()
            
            ScrollView {
                LazyVStack(spacing: 0) {
                    ForEach(viewModel.filteredServers) { server in
                        serverRow(server)
                            .contentShape(Rectangle())
                            .onTapGesture {
                                viewModel.selectedServer = server
                            }
                    }
                }
            }
        }
    }
    
    private func serverRow(_ server: MCPServer) -> some View {
        HStack(spacing: 12) {
            // Type icon
            Image(systemName: serverTypeIcon(server.type))
                .foregroundStyle(server.enabled ? .blue : .secondary)
                .frame(width: 20)
            
            VStack(alignment: .leading, spacing: 2) {
                Text(server.name)
                    .font(.system(.body, design: .default))
                    .foregroundColor(.primary)
                
                HStack(spacing: 6) {
                    Text(serverTypeDisplayName(server.type))
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    
                    if server.isBuiltin {
                        Text("Built-in")
                            .font(.system(size: 9, weight: .semibold))
                            .foregroundStyle(.white)
                            .padding(.horizontal, 5)
                            .padding(.vertical, 1)
                            .background(Color.blue.opacity(0.7))
                            .cornerRadius(3)
                    }
                    
                    Text("•")
                        .font(.caption)
                        .foregroundStyle(.tertiary)
                    
                    Text(server.enabled ? "Enabled" : "Disabled")
                        .font(.caption)
                        .foregroundStyle(server.enabled ? .green : .secondary)
                }
                
                // Capability counts from status
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
            
            Spacer()
            
            if viewModel.selectedServer?.id == server.id {
                Image(systemName: "checkmark")
                    .foregroundStyle(.blue)
            }
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 8)
        .background(viewModel.selectedServer?.id == server.id ? Color.accentColor.opacity(0.1) : Color.clear)
    }
    
    // MARK: - Detail View
    
    private var detailView: some View {
        Group {
            if let server = viewModel.selectedServer {
                serverDetailView(server)
            } else {
                noSelectionView
            }
        }
    }
    
    private var noSelectionView: some View {
        VStack(spacing: 12) {
            Image(systemName: "server.rack")
                .font(.system(size: 48))
                .foregroundStyle(.secondary)
            Text("No server selected")
                .font(.headline)
                .foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
    
    private func serverDetailView(_ server: MCPServer) -> some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                // Header with actions
                HStack {
                    VStack(alignment: .leading, spacing: 4) {
                        Text(server.name)
                            .font(.title2)
                            .fontWeight(.semibold)
                        
                        HStack(spacing: 8) {
                            Label(serverTypeDisplayName(server.type), systemImage: serverTypeIcon(server.type))
                                .font(.caption)
                                .foregroundStyle(.secondary)
                            
                            Text("•")
                                .font(.caption)
                                .foregroundStyle(.tertiary)
                            
                            Text(server.enabled ? "Enabled" : "Disabled")
                                .font(.caption)
                                .foregroundStyle(server.enabled ? .green : .secondary)
                        }
                    }
                    
                    Spacer()
                    
                    if !server.isBuiltin {
                        HStack(spacing: 8) {
                            Button {
                                Task { await viewModel.duplicateServer(server) }
                            } label: {
                                Label("Duplicate", systemImage: "doc.on.doc")
                            }
                            
                            Button {
                                viewModel.editingServer = server
                            } label: {
                                Label("Edit", systemImage: "pencil")
                            }
                            
                            Button(role: .destructive) {
                                viewModel.serverToDelete = server
                                viewModel.showDeleteConfirm = true
                            } label: {
                                Label("Delete", systemImage: "trash")
                            }
                        }
                    }
                }
                
                Divider()
                
                // Capabilities section
                CapabilitiesSection(serverName: server.name)
                
                Divider()
                
                // Configuration details
                Group {
                    // Type section (always visible)
                    DetailSection(title: "Type") {
                        InfoRow(label: "Type", value: serverTypeDisplayName(server.type))
                        InfoRow(label: "Description", value: serverTypeDescription(server.type))
                    }
                    
                    // Config sections (hidden for builtins)
                    if !server.isBuiltin {
                        if server.type == "stdio" {
                            stdioConfigSection(server)
                        } else if server.type == "sse" || server.type == "http" {
                            networkConfigSection(server)
                        }
                        
                        if let env = server.env, !env.isEmpty {
                            DetailSection(title: "Environment Variables") {
                                VStack(alignment: .leading, spacing: 4) {
                                    ForEach(Array(env.keys.sorted()), id: \.self) { key in
                                        HStack {
                                            Text(key)
                                                .font(.system(.caption, design: .monospaced))
                                                .foregroundStyle(.secondary)
                                            Text("=")
                                                .foregroundStyle(.tertiary)
                                            Text(env[key] ?? "")
                                                .font(.system(.caption, design: .monospaced))
                                        }
                                    }
                                }
                            }
                        }
                        
                        if server.oauth != nil {
                            DetailSection(title: "OAuth Configuration") {
                                InfoRow(label: "Provider", value: server.oauth?.provider ?? "Not specified")
                                if let scopes = server.oauth?.scopes, !scopes.isEmpty {
                                    InfoRow(label: "Scopes", value: scopes.joined(separator: ", "))
                                }
                            }
                        }
                    }
                }
                
                // Metadata (hidden for builtins as they are static)
                if !server.isBuiltin {
                    DetailSection(title: "Metadata") {
                        InfoRow(label: "Created", value: formatDate(server.createdAt))
                        InfoRow(label: "Updated", value: formatDate(server.updatedAt))
                    }
                }
            }
            .padding(20)
        }
    }
    
    private func stdioConfigSection(_ server: MCPServer) -> some View {
        DetailSection(title: "Standard I/O Configuration") {
            if let command = server.command {
                InfoRow(label: "Command", value: command)
            }
            if let args = server.args, !args.isEmpty {
                VStack(alignment: .leading, spacing: 4) {
                    Text("Arguments")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    ForEach(args.indices, id: \.self) { index in
                        Text("[\(index)] \(args[index])")
                            .font(.system(.caption, design: .monospaced))
                    }
                }
            }
        }
    }
    
    private func networkConfigSection(_ server: MCPServer) -> some View {
        DetailSection(title: "Network Configuration") {
            if let url = server.url {
                InfoRow(label: "URL", value: url)
            }
            if let headers = server.headers, !headers.isEmpty {
                VStack(alignment: .leading, spacing: 4) {
                    Text("Headers")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    ForEach(Array(headers.keys.sorted()), id: \.self) { key in
                        HStack {
                            Text(key)
                                .font(.system(.caption, design: .monospaced))
                                .foregroundStyle(.secondary)
                            Text(":")
                                .foregroundStyle(.tertiary)
                            Text(headers[key] ?? "")
                                .font(.system(.caption, design: .monospaced))
                        }
                    }
                }
            }
        }
    }
    
    // MARK: - Create Server Sheet
    
    private var createServerSheet: some View {
        VStack(spacing: 0) {
            // Header
            HStack {
                Text("Add MCP Server")
                    .font(.headline)
                Spacer()
                Button("Cancel") {
                    viewModel.showCreateServer = false
                }
            }
            .padding()
            
            Divider()
            
            // Form
            ScrollView {
                Form {
                    Section("Basic Information") {
                        TextField("Server Name", text: $viewModel.newServerName)
                        
                        Picker("Type", selection: $viewModel.newServerType) {
                            ForEach(MCPServerType.allCases) { type in
                                Text(type.displayName).tag(type)
                            }
                        }
                        
                        Toggle("Enabled", isOn: $viewModel.newServerEnabled)
                    }
                    
                    // Type-specific fields
                    if viewModel.newServerType == .stdio {
                        Section("Standard I/O Configuration") {
                            TextField("Command", text: $viewModel.newServerCommand)
                                .help("Path to the executable (e.g., node, python3, /usr/local/bin/mcp-server)")
                            
                            StringArrayEditor(
                                items: $viewModel.newServerArgs,
                                title: "Arguments",
                                placeholder: "Add argument"
                            )
                        }
                    } else if viewModel.newServerType == .sse || viewModel.newServerType == .http {
                        Section("Network Configuration") {
                            TextField("URL", text: $viewModel.newServerURL)
                                .help("Full URL to the MCP server endpoint")
                            
                            KeyValueEditor(
                                items: $viewModel.newServerHeaders,
                                title: "HTTP Headers",
                                keyPlaceholder: "Header name",
                                valuePlaceholder: "Header value"
                            )
                        }
                    }
                    
                    // Environment variables (for all types)
                    if viewModel.newServerType != .builtin {
                        Section("Environment Variables") {
                            KeyValueEditor(
                                items: $viewModel.newServerEnv,
                                title: "Environment",
                                keyPlaceholder: "Variable name",
                                valuePlaceholder: "Value"
                            )
                        }
                    }
                    
                    // OAuth configuration (optional for all types)
                    Section("OAuth (Optional)") {
                        OAuthConfigEditor(config: $viewModel.newServerOAuth)
                    }
                    
                    if let error = viewModel.createError {
                        Section {
                            Text(error)
                                .foregroundStyle(.red)
                                .font(.caption)
                        }
                    }
                }
                .formStyle(.grouped)
            }
            
            Divider()
            
            // Footer
            HStack {
                Spacer()
                Button("Create") {
                    Task { await viewModel.createServer() }
                }
                .disabled(!viewModel.canCreateServer)
                .buttonStyle(.borderedProminent)
            }
            .padding()
        }
        .frame(width: 600, height: 700)
    }
    
    // MARK: - Edit Server Sheet
    
    private func editServerSheet(for server: MCPServer) -> some View {
        VStack(spacing: 0) {
            // Header
            HStack {
                Text("Edit \(server.name)")
                    .font(.headline)
                Spacer()
                Button("Cancel") {
                    viewModel.editingServer = nil
                }
            }
            .padding()
            
            Divider()
            
            // Form
            ScrollView {
                Form {
                    Section("Basic Information") {
                        TextField("Server Name", text: $viewModel.editName)
                        
                        Picker("Type", selection: $viewModel.editType) {
                            ForEach(MCPServerType.allCases) { type in
                                Text(type.displayName).tag(type)
                            }
                        }
                        .disabled(true) // Don't allow changing type after creation
                        
                        Toggle("Enabled", isOn: $viewModel.editEnabled)
                    }
                    
                    // Type-specific fields
                    if viewModel.editType == .stdio {
                        Section("Standard I/O Configuration") {
                            TextField("Command", text: $viewModel.editCommand)
                                .help("Path to the executable (e.g., node, python3, /usr/local/bin/mcp-server)")
                            
                            StringArrayEditor(
                                items: $viewModel.editArgs,
                                title: "Arguments",
                                placeholder: "Add argument"
                            )
                        }
                    } else if viewModel.editType == .sse || viewModel.editType == .http {
                        Section("Network Configuration") {
                            TextField("URL", text: $viewModel.editURL)
                                .help("Full URL to the MCP server endpoint")
                            
                            KeyValueEditor(
                                items: $viewModel.editHeaders,
                                title: "HTTP Headers",
                                keyPlaceholder: "Header name",
                                valuePlaceholder: "Header value"
                            )
                        }
                    }
                    
                    // Environment variables (for all types)
                    if viewModel.editType != .builtin {
                        Section("Environment Variables") {
                            KeyValueEditor(
                                items: $viewModel.editEnv,
                                title: "Environment",
                                keyPlaceholder: "Variable name",
                                valuePlaceholder: "Value"
                            )
                        }
                    }
                    
                    // OAuth configuration (optional for all types)
                    Section("OAuth (Optional)") {
                        OAuthConfigEditor(config: $viewModel.editOAuth)
                    }
                    
                    if let error = viewModel.editError {
                        Section {
                            Text(error)
                                .foregroundStyle(.red)
                                .font(.caption)
                        }
                    }
                }
                .formStyle(.grouped)
            }
            
            Divider()
            
            // Footer
            HStack {
                Spacer()
                Button("Save") {
                    Task { await viewModel.updateServer(server) }
                }
                .disabled(!viewModel.canSaveEdit)
                .buttonStyle(.borderedProminent)
            }
            .padding()
        }
        .frame(width: 600, height: 700)
        .onAppear {
            viewModel.populateEditFields(from: server)
        }
    }
    
    // MARK: - Helpers
    
    private func serverTypeIcon(_ type: String) -> String {
        switch type {
        case "stdio": return "terminal"
        case "sse": return "bolt.horizontal"
        case "http": return "network"
        case "builtin": return "cube.box"
        default: return "server.rack"
        }
    }
    
    private func serverTypeDisplayName(_ type: String) -> String {
        MCPServerType(rawValue: type)?.displayName ?? type
    }
    
    private func serverTypeDescription(_ type: String) -> String {
        MCPServerType(rawValue: type)?.description ?? ""
    }
    
    private func formatDate(_ date: Date) -> String {
        let formatter = DateFormatter()
        formatter.dateStyle = .medium
        formatter.timeStyle = .short
        return formatter.string(from: date)
    }
}

// MARK: - Capability Badge

struct CapabilityBadge: View {
    let icon: String
    let label: String
    let count: Int
    let color: Color
    
    var body: some View {
        VStack(spacing: 6) {
            Image(systemName: icon)
                .font(.title2)
                .foregroundStyle(color)
            
            VStack(spacing: 2) {
                Text("\(count)")
                    .font(.title3.bold())
                Text(label)
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
        .frame(maxWidth: .infinity)
        .padding(.vertical, 12)
        .background(color.opacity(0.1))
        .cornerRadius(8)
    }
}

// MARK: - Capabilities Section

struct CapabilitiesSection: View {
    let serverName: String
    @EnvironmentObject var statusMonitor: StatusMonitor
    @State private var tools: [ToolInfo] = []
    @State private var prompts: [PromptInfo] = []
    @State private var resources: [ResourceInfo] = []
    @State private var isLoading = false
    
    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Capabilities")
                .font(.headline)
            
            if isLoading {
                HStack {
                    ProgressView()
                        .scaleEffect(0.7)
                    Text("Loading...")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                .padding(.vertical, 8)
            } else {
                ScrollViewReader { proxy in
                    VStack(alignment: .leading, spacing: 12) {
                        // Filter by selected server
                        let filteredPrompts = prompts.filter { $0.server == serverName }
                        let filteredResources = resources.filter { $0.server == serverName }
                        let filteredTools = tools.filter { $0.server == serverName }
                        
                        // Summary Badges
                        if !filteredPrompts.isEmpty || !filteredResources.isEmpty || !filteredTools.isEmpty {
                            HStack(spacing: 12) {
                                if !filteredTools.isEmpty {
                                    Button {
                                        withAnimation { proxy.scrollTo("tools", anchor: .top) }
                                    } label: {
                                        CapabilityBadge(icon: "wrench.fill", label: "Tools", count: filteredTools.count, color: .blue)
                                    }
                                    .buttonStyle(.plain)
                                }
                                if !filteredResources.isEmpty {
                                    Button {
                                        withAnimation { proxy.scrollTo("resources", anchor: .top) }
                                    } label: {
                                        CapabilityBadge(icon: "doc.fill", label: "Resources", count: filteredResources.count, color: .green)
                                    }
                                    .buttonStyle(.plain)
                                }
                                if !filteredPrompts.isEmpty {
                                    Button {
                                        withAnimation { proxy.scrollTo("prompts", anchor: .top) }
                                    } label: {
                                        CapabilityBadge(icon: "text.bubble.fill", label: "Prompts", count: filteredPrompts.count, color: .purple)
                                    }
                                    .buttonStyle(.plain)
                                }
                            }
                            .padding(.bottom, 8)
                        }
                        
                        if !filteredPrompts.isEmpty {
                            VStack(alignment: .leading, spacing: 8) {
                                HStack(spacing: 6) {
                                    Image(systemName: "text.bubble.fill")
                                        .foregroundStyle(.purple)
                                        .font(.caption)
                                    Text("Prompts")
                                        .font(.subheadline)
                                        .fontWeight(.medium)
                                    Text("(\(filteredPrompts.count))")
                                        .font(.caption)
                                        .foregroundStyle(.secondary)
                                }
                                
                                VStack(alignment: .leading, spacing: 4) {
                                    ForEach(filteredPrompts) { prompt in
                                        PromptItemRow(prompt: prompt, serverName: serverName)
                                    }
                                }
                                .padding(.horizontal, 12)
                                .padding(.vertical, 8)
                                .background(Color.purple.opacity(0.05))
                                .cornerRadius(6)
                            }
                            .id("prompts")
                        }
                        
                        // Resources section
                        if !filteredResources.isEmpty {
                            VStack(alignment: .leading, spacing: 8) {
                                HStack(spacing: 6) {
                                    Image(systemName: "doc.fill")
                                        .foregroundStyle(.green)
                                        .font(.caption)
                                    Text("Resources")
                                        .font(.subheadline)
                                        .fontWeight(.medium)
                                    Text("(\(filteredResources.count))")
                                        .font(.caption)
                                        .foregroundStyle(.secondary)
                                }
                                
                                VStack(alignment: .leading, spacing: 4) {
                                    ForEach(filteredResources) { resource in
                                        ResourceItemRow(resource: resource, serverName: serverName)
                                    }
                                }
                                .padding(.horizontal, 12)
                                .padding(.vertical, 8)
                                .background(Color.green.opacity(0.05))
                                .cornerRadius(6)
                            }
                            .id("resources")
                        }
                        
                        // Tools section
                        if !filteredTools.isEmpty {
                            VStack(alignment: .leading, spacing: 8) {
                                HStack(spacing: 6) {
                                    Image(systemName: "wrench.fill")
                                        .foregroundStyle(.blue)
                                        .font(.caption)
                                    Text("Tools")
                                        .font(.subheadline)
                                        .fontWeight(.medium)
                                    Text("(\(filteredTools.count))")
                                        .font(.caption)
                                        .foregroundStyle(.secondary)
                                }
                                
                                VStack(alignment: .leading, spacing: 4) {
                                    ForEach(filteredTools) { tool in
                                        ToolItemRow(tool: tool)
                                    }
                                }
                                .padding(.horizontal, 12)
                                .padding(.vertical, 8)
                                .background(Color.blue.opacity(0.05))
                                .cornerRadius(6)
                            }
                            .id("tools")
                        }
                        
                        // Show empty state if nothing is available
                        if filteredPrompts.isEmpty && filteredResources.isEmpty && filteredTools.isEmpty {
                            Text("No capabilities available for this server")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                                .padding(.vertical, 8)
                        }
                    }
                }
            }
        }
        .task {
            await loadCapabilities()
        }
    }
    
    private func loadCapabilities() async {
        guard let client = statusMonitor.configuredClient else { return }
        
        isLoading = true
        do {
            async let toolsTask = client.getTools()
            async let promptsTask = client.getPrompts()
            async let resourcesTask = client.getResources()
            
            tools = try await toolsTask
            prompts = try await promptsTask
            resources = try await resourcesTask
        } catch {
            print("Error loading capabilities: \(error)")
            // Show empty state on error
        }
        isLoading = false
    }
}

// MARK: - Prompt Item Row (with full content preview)

struct PromptItemRow: View {
    let prompt: PromptInfo
    let serverName: String
    @EnvironmentObject var statusMonitor: StatusMonitor
    @State private var isExpanded = false
    @State private var isHovering = false
    @State private var contentResponse: PromptContentResponse?
    @State private var isLoadingContent = false
    @State private var loadError: String?
    
    private func toggleExpanded() {
        withAnimation(.easeInOut(duration: 0.15)) {
            isExpanded.toggle()
        }
        if isExpanded && contentResponse == nil && !isLoadingContent {
            Task { await loadContent() }
        }
    }
    
    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                Text(prompt.name)
                    .font(.system(.caption, design: .monospaced))
                    .fontWeight(.medium)
                
                Spacer()
                
                Image(systemName: isExpanded ? "chevron.up" : "chevron.down")
                    .font(.caption2)
                    .foregroundStyle(.secondary)
            }
            .contentShape(Rectangle())
            .onTapGesture { toggleExpanded() }
            
            if isExpanded {
                VStack(alignment: .leading, spacing: 6) {
                    // Description
                    if !prompt.description.isEmpty {
                        Text(prompt.description)
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                    }
                    
                    // Arguments
                    if let args = prompt.arguments, !args.isEmpty {
                        VStack(alignment: .leading, spacing: 2) {
                            Text("Arguments:")
                                .font(.caption2)
                                .fontWeight(.medium)
                                .foregroundStyle(.secondary)
                            ForEach(args) { arg in
                                HStack(spacing: 4) {
                                    Text(arg.name)
                                        .font(.system(.caption2, design: .monospaced))
                                        .foregroundStyle(.purple)
                                    if arg.required == true {
                                        Text("(required)")
                                            .font(.system(size: 9))
                                            .foregroundStyle(.orange)
                                    }
                                    if let desc = arg.description, !desc.isEmpty {
                                        Text("- \(desc)")
                                            .font(.caption2)
                                            .foregroundStyle(.tertiary)
                                    }
                                }
                            }
                        }
                    }
                    
                    // Full content
                    if isLoadingContent {
                        HStack(spacing: 4) {
                            ProgressView()
                                .scaleEffect(0.5)
                            Text("Loading content...")
                                .font(.caption2)
                                .foregroundStyle(.secondary)
                        }
                    } else if let error = loadError {
                        Text("Error: \(error)")
                            .font(.caption2)
                            .foregroundStyle(.red)
                    } else if let response = contentResponse, let messages = response.messages {
                        Divider()
                        VStack(alignment: .leading, spacing: 6) {
                            Text("Prompt Messages:")
                                .font(.caption2)
                                .fontWeight(.medium)
                                .foregroundStyle(.secondary)
                            ForEach(Array(messages.enumerated()), id: \.offset) { _, msg in
                                VStack(alignment: .leading, spacing: 2) {
                                    Text(msg.role.uppercased())
                                        .font(.system(size: 9, weight: .bold, design: .monospaced))
                                        .foregroundStyle(msg.role == "user" ? .blue : .green)
                                        .padding(.horizontal, 4)
                                        .padding(.vertical, 1)
                                        .background(msg.role == "user" ? Color.blue.opacity(0.1) : Color.green.opacity(0.1))
                                        .cornerRadius(2)
                                    
                                    Text(msg.content.text)
                                        .font(.system(.caption2, design: .monospaced))
                                        .foregroundStyle(.primary)
                                        .textSelection(.enabled)
                                        .padding(6)
                                        .frame(maxWidth: .infinity, alignment: .leading)
                                        .background(Color(nsColor: .textBackgroundColor).opacity(0.5))
                                        .cornerRadius(4)
                                }
                            }
                        }
                    }
                }
                .padding(.top, 2)
            }
        }
        .padding(.horizontal, 8)
        .padding(.vertical, 6)
        .background(isHovering ? Color.purple.opacity(0.1) : Color.clear)
        .cornerRadius(4)
        .onHover { hovering in
            isHovering = hovering
        }
    }
    
    private func loadContent() async {
        guard let client = statusMonitor.configuredClient else { return }
        isLoadingContent = true
        loadError = nil
        do {
            // Strip server prefix from prompt name if present
            let actualName: String
            let prefix = serverName + "_"
            if prompt.name.hasPrefix(prefix) {
                actualName = String(prompt.name.dropFirst(prefix.count))
            } else {
                actualName = prompt.name
            }
            contentResponse = try await client.getPromptContent(server: serverName, name: actualName)
        } catch {
            loadError = error.localizedDescription
        }
        isLoadingContent = false
    }
}

// MARK: - Tool Item Row (with inputSchema preview)

struct ToolItemRow: View {
    let tool: ToolInfo
    @State private var isExpanded = false
    @State private var isHovering = false
    
    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                Text(tool.name)
                    .font(.system(.caption, design: .monospaced))
                    .fontWeight(.medium)
                
                Spacer()
                
                Image(systemName: isExpanded ? "chevron.up" : "chevron.down")
                    .font(.caption2)
                    .foregroundStyle(.secondary)
            }
            .contentShape(Rectangle())
            .onTapGesture {
                withAnimation(.easeInOut(duration: 0.15)) {
                    isExpanded.toggle()
                }
            }
            
            if isExpanded {
                VStack(alignment: .leading, spacing: 6) {
                    // Description
                    if !tool.description.isEmpty {
                        Text(tool.description)
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                    }
                    
                    // Input Schema
                    if let schema = tool.inputSchema {
                        Divider()
                        VStack(alignment: .leading, spacing: 4) {
                            Text("Parameters:")
                                .font(.caption2)
                                .fontWeight(.medium)
                                .foregroundStyle(.secondary)
                            
                            SchemaPropertiesView(schema: schema)
                        }
                    }
                }
                .padding(.top, 2)
            }
        }
        .padding(.horizontal, 8)
        .padding(.vertical, 6)
        .background(isHovering ? Color.blue.opacity(0.1) : Color.clear)
        .cornerRadius(4)
        .onHover { hovering in
            isHovering = hovering
        }
    }
}

// MARK: - Schema Properties View (renders inputSchema)

struct SchemaPropertiesView: View {
    let schema: JSONValue
    
    var body: some View {
        VStack(alignment: .leading, spacing: 3) {
            if case .object(let obj) = schema,
               case .object(let props)? = obj["properties"] {
                let requiredNames = extractRequired(from: obj["required"])
                ForEach(Array(props.keys.sorted()), id: \.self) { key in
                    if let propValue = props[key], case .object(let prop) = propValue {
                        HStack(alignment: .top, spacing: 4) {
                            Text(key)
                                .font(.system(.caption2, design: .monospaced))
                                .foregroundStyle(.blue)
                            
                            if let type = prop["type"] {
                                Text("(\(type.prettyDescription.replacingOccurrences(of: "\"", with: "")))")
                                    .font(.system(size: 9, design: .monospaced))
                                    .foregroundStyle(.tertiary)
                            }
                            
                            if requiredNames.contains(key) {
                                Text("required")
                                    .font(.system(size: 9))
                                    .foregroundStyle(.orange)
                            }
                            
                            if let desc = prop["description"], case .string(let d) = desc {
                                Text("- \(d)")
                                    .font(.caption2)
                                    .foregroundStyle(.tertiary)
                                    .lineLimit(2)
                            }
                        }
                    }
                }
            } else {
                // Fallback: show raw schema
                Text(schema.prettyDescription)
                    .font(.system(.caption2, design: .monospaced))
                    .foregroundStyle(.secondary)
                    .textSelection(.enabled)
            }
        }
    }
    
    private func extractRequired(from value: JSONValue?) -> Set<String> {
        guard let value = value, case .array(let arr) = value else { return [] }
        var names = Set<String>()
        for item in arr {
            if case .string(let s) = item {
                names.insert(s)
            }
        }
        return names
    }
}

struct ResourceItemRow: View {
    let resource: ResourceInfo
    let serverName: String
    @EnvironmentObject var statusMonitor: StatusMonitor
    @State private var isExpanded = false
    @State private var isHovering = false
    @State private var contentResponse: ResourceContentResponse?
    @State private var isLoadingContent = false
    @State private var loadError: String?
    
    private func toggleExpanded() {
        withAnimation(.easeInOut(duration: 0.15)) {
            isExpanded.toggle()
        }
        if isExpanded && contentResponse == nil && !isLoadingContent {
            Task { await loadContent() }
        }
    }
    
    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                Text(resource.name)
                    .font(.system(.caption, design: .monospaced))
                    .fontWeight(.medium)
                
                Spacer()
                
                Image(systemName: isExpanded ? "chevron.up" : "chevron.down")
                    .font(.caption2)
                    .foregroundStyle(.secondary)
            }
            .contentShape(Rectangle())
            .onTapGesture { toggleExpanded() }
            
            if isExpanded {
                VStack(alignment: .leading, spacing: 6) {
                    // Description
                    if !resource.description.isEmpty {
                        Text(resource.description)
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                    }
                    
                    // Metadata
                    HStack(spacing: 8) {
                        HStack(spacing: 4) {
                            Text("URI:")
                                .font(.caption2)
                                .foregroundStyle(.tertiary)
                            Text(resource.uri)
                                .font(.system(.caption2, design: .monospaced))
                                .foregroundStyle(.secondary)
                        }
                        
                        if let mimeType = resource.mimeType, !mimeType.isEmpty {
                            HStack(spacing: 4) {
                                Text("Type:")
                                    .font(.caption2)
                                    .foregroundStyle(.tertiary)
                                Text(mimeType)
                                    .font(.system(.caption2, design: .monospaced))
                                    .foregroundStyle(.secondary)
                            }
                        }
                    }
                    
                    // Full content
                    if isLoadingContent {
                        HStack(spacing: 4) {
                            ProgressView()
                                .scaleEffect(0.5)
                            Text("Loading content...")
                                .font(.caption2)
                                .foregroundStyle(.secondary)
                        }
                    } else if let error = loadError {
                        Text("Error: \(error)")
                            .font(.caption2)
                            .foregroundStyle(.red)
                    } else if let response = contentResponse, let contents = response.contents, !contents.isEmpty {
                        Divider()
                        VStack(alignment: .leading, spacing: 4) {
                            Text("Content:")
                                .font(.caption2)
                                .fontWeight(.medium)
                                .foregroundStyle(.secondary)
                            
                            ForEach(Array(contents.enumerated()), id: \.offset) { _, item in
                                if let text = item.text, !text.isEmpty {
                                    ScrollView {
                                        Text(text)
                                            .font(.system(.caption2, design: .monospaced))
                                            .foregroundStyle(.primary)
                                            .textSelection(.enabled)
                                            .frame(maxWidth: .infinity, alignment: .leading)
                                    }
                                    .frame(maxHeight: 300)
                                    .padding(6)
                                    .background(Color(nsColor: .textBackgroundColor).opacity(0.5))
                                    .cornerRadius(4)
                                } else if item.blob != nil {
                                    Text("[Binary content]")
                                        .font(.caption2)
                                        .foregroundStyle(.secondary)
                                        .italic()
                                }
                            }
                        }
                    }
                }
                .padding(.top, 2)
            }
        }
        .padding(.horizontal, 8)
        .padding(.vertical, 6)
        .background(isHovering ? Color.green.opacity(0.1) : Color.clear)
        .cornerRadius(4)
        .onHover { hovering in
            isHovering = hovering
        }
    }
    
    private func loadContent() async {
        guard let client = statusMonitor.configuredClient else { return }
        isLoadingContent = true
        loadError = nil
        do {
            contentResponse = try await client.getResourceContent(server: serverName, uri: resource.uri)
        } catch {
            loadError = error.localizedDescription
        }
        isLoadingContent = false
    }
}


