import SwiftUI

/// View for managing MCP server definitions (the "registry").
///
/// This view allows users to:
/// - Browse all MCP server definitions
/// - Create new MCP servers
/// - Edit server configuration (type, command, URL, etc.)
/// - Delete server definitions
/// - Duplicate servers
///
/// Does NOT handle per-node deployment - that's MCPServersView's job.
struct MCPRegistryView: View {
    @EnvironmentObject var statusMonitor: StatusMonitor
    @State private var viewModel: MCPRegistryViewModel
    @State private var clientInitialized = false
    
    init(viewModel: MCPRegistryViewModel = MCPRegistryViewModel()) {
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
                viewModel = MCPRegistryViewModel(client: configuredClient)
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
            Text("Loading server definitions...")
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
            Text("Failed to load registry")
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
            icon: "book.closed",
            title: "No MCP servers defined",
            description: "Add an MCP server definition to the registry",
            actionLabel: "Add Server",
            action: { viewModel.showCreateServer = true }
        )
    }
    
    // MARK: - Servers List
    
    private var serversListView: some View {
        VStack(spacing: 0) {
            MasterListHeader(
                icon: "server.rack",
                title: "All Servers"
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
        let isEnabled = viewModel.isServerEnabled(server)
        let status = statusMonitor.status.mcpServers.first(where: { $0.name == server.name })
        
        return HStack(spacing: 12) {
            // Type icon (colored blue when enabled)
            Image(systemName: serverTypeIcon(server.type))
                .foregroundStyle(isEnabled ? .blue : .secondary)
                .frame(width: 20)
            
            VStack(alignment: .leading, spacing: 4) {
                // Server name
                Text(server.name)
                    .font(.system(.body, design: .default))
                    .foregroundColor(.primary)
                
                // Type and badges
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
                }
                
                // Runtime capabilities (tools/prompts/resources) - only show when enabled
                if isEnabled, let status = status, status.connected {
                    HStack(spacing: 12) {
                        if status.toolCount > 0 {
                            HStack(spacing: 4) {
                                Image(systemName: "wrench.fill")
                                    .font(.system(size: 9))
                                    .foregroundStyle(.blue)
                                Text("\(status.toolCount)")
                                    .font(.system(size: 10))
                                    .foregroundStyle(.secondary)
                            }
                        }
                        if status.promptCount > 0 {
                            HStack(spacing: 4) {
                                Image(systemName: "text.bubble.fill")
                                    .font(.system(size: 9))
                                    .foregroundStyle(.purple)
                                Text("\(status.promptCount)")
                                    .font(.system(size: 10))
                                    .foregroundStyle(.secondary)
                            }
                        }
                        if status.resourceCount > 0 {
                            HStack(spacing: 4) {
                                Image(systemName: "doc.fill")
                                    .font(.system(size: 9))
                                    .foregroundStyle(.green)
                                Text("\(status.resourceCount)")
                                    .font(.system(size: 10))
                                    .foregroundStyle(.secondary)
                            }
                        }
                    }
                }
            }
            
            Spacer()
            
            // Master Node Toggle
            Toggle("", isOn: Binding(
                get: { isEnabled },
                set: { newValue in
                    Task { await viewModel.toggleServer(server, enabled: newValue) }
                }
            ))
            .toggleStyle(.switch)
            .labelsHidden()
            .scaleEffect(0.7)
            .frame(width: 30)
            .padding(.trailing, 8)
            
            if viewModel.selectedServer?.id == server.id {
                Image(systemName: "chevron.right")
                    .font(.caption)
                    .foregroundStyle(.secondary)
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
            Text("Select a server to view details or configure it.")
                .font(.caption)
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
                        
                        Label(serverTypeDisplayName(server.type), systemImage: serverTypeIcon(server.type))
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                    
                    Spacer()
                    
                    Menu {
                        if !server.isBuiltin {
                            Button {
                                viewModel.editingServer = server
                                viewModel.populateEditFields(from: server)
                            } label: {
                                Label("Edit", systemImage: "pencil")
                            }
                        }
                        
                        Button {
                            Task { await viewModel.duplicateServer(server) }
                        } label: {
                            Label("Duplicate", systemImage: "doc.on.doc")
                        }
                        
                        Divider()
                        
                        if !server.isBuiltin {
                            Button(role: .destructive) {
                                viewModel.serverToDelete = server
                                viewModel.showDeleteConfirm = true
                            } label: {
                                Label("Delete", systemImage: "trash")
                            }
                        }
                    } label: {
                        Image(systemName: "ellipsis.circle")
                            .font(.title2)
                    }
                    .menuIndicator(.hidden)
                }
                
                // Basic Info
                DetailSection(title: "Configuration") {
                    InfoRow(label: "Type", value: serverTypeDisplayName(server.type))
                    
                    if server.isBuiltin {
                        InfoRow(label: "Built-in", value: "Yes")
                    }
                }
                
                // Type-specific config
                switch MCPServerType(rawValue: server.type) {
                case .stdio:
                    if let command = server.command {
                        DetailSection(title: "STDIO Configuration") {
                            InfoRow(label: "Command", value: command)
                            
                            if let args = server.args, !args.isEmpty {
                                VStack(alignment: .leading, spacing: 4) {
                                    Text("Arguments")
                                        .font(.caption)
                                        .foregroundStyle(.secondary)
                                    ForEach(args, id: \.self) { arg in
                                        Text(arg)
                                            .font(.system(.caption, design: .monospaced))
                                            .foregroundColor(.primary)
                                    }
                                }
                            }
                            
                            if let env = server.env, !env.isEmpty {
                                VStack(alignment: .leading, spacing: 4) {
                                    Text("Environment Variables")
                                        .font(.caption)
                                        .foregroundStyle(.secondary)
                                    ForEach(Array(env.keys.sorted()), id: \.self) { key in
                                        Text("\(key)=\(env[key] ?? "")")
                                            .font(.system(.caption, design: .monospaced))
                                            .foregroundColor(.primary)
                                    }
                                }
                            }
                        }
                    }
                    
                case .sse, .http:
                    if let url = server.url {
                        DetailSection(title: "HTTP Configuration") {
                            InfoRow(label: "URL", value: url)
                            
                            if let headers = server.headers, !headers.isEmpty {
                                VStack(alignment: .leading, spacing: 4) {
                                    Text("Headers")
                                        .font(.caption)
                                        .foregroundStyle(.secondary)
                                    ForEach(Array(headers.keys.sorted()), id: \.self) { key in
                                        Text("\(key): \(headers[key] ?? "")")
                                            .font(.system(.caption, design: .monospaced))
                                            .foregroundColor(.primary)
                                    }
                                }
                            }
                            
                            if server.oauth != nil {
                                InfoRow(label: "OAuth", value: "Configured")
                            }
                        }
                    }
                    
                case .builtin:
                    DetailSection(title: "Built-in Server") {
                        InfoRow(label: "Managed by", value: "Diane")
                    }
                    
                case .none:
                    EmptyView()
                }
                
                // Runtime status from statusMonitor (shows tools/prompts/resources)
                if let status = statusMonitor.status.mcpServers.first(where: { $0.name == server.name }) {
                    DetailSection(title: "Runtime Status") {
                        HStack(spacing: 6) {
                            Circle()
                                .fill(status.connected ? Color.green : Color.red)
                                .frame(width: 8, height: 8)
                            Text(status.connected ? "Connected" : "Disconnected")
                                .font(.caption)
                                .foregroundColor(.primary)
                        }
                        
                        if status.toolCount > 0 || status.promptCount > 0 || status.resourceCount > 0 {
                            HStack(spacing: 16) {
                                if status.toolCount > 0 {
                                    HStack(spacing: 4) {
                                        Image(systemName: "wrench.fill")
                                            .font(.system(size: 10))
                                            .foregroundStyle(.blue)
                                        Text("\(status.toolCount) tools")
                                            .font(.caption)
                                    }
                                }
                                if status.promptCount > 0 {
                                    HStack(spacing: 4) {
                                        Image(systemName: "text.bubble.fill")
                                            .font(.system(size: 10))
                                            .foregroundStyle(.purple)
                                        Text("\(status.promptCount) prompts")
                                            .font(.caption)
                                    }
                                }
                                if status.resourceCount > 0 {
                                    HStack(spacing: 4) {
                                        Image(systemName: "doc.fill")
                                            .font(.system(size: 10))
                                            .foregroundStyle(.green)
                                        Text("\(status.resourceCount) resources")
                                            .font(.caption)
                                    }
                                }
                            }
                        }
                        
                        if let error = status.error, !error.isEmpty {
                            HStack(spacing: 4) {
                                Image(systemName: "exclamationmark.triangle.fill")
                                    .font(.system(size: 10))
                                    .foregroundStyle(.orange)
                                Text(error)
                                    .font(.caption)
                                    .foregroundStyle(.red)
                            }
                        }
                    }
                }
                
                // Available in Contexts
                DetailSection(title: "Available in Contexts") {
                    VStack(alignment: .leading, spacing: 8) {
                        Text("Contexts where AI clients can access this server")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                        
                        let contexts = viewModel.contextsForServer(server)
                        if contexts.isEmpty {
                            Text("Not in any context")
                                .font(.caption2)
                                .foregroundStyle(.tertiary)
                        } else {
                            HStack(spacing: 6) {
                                ForEach(contexts, id: \.self) { contextName in
                                    Text(contextName)
                                        .font(.caption2)
                                        .fontWeight(.medium)
                                        .foregroundStyle(.white)
                                        .padding(.horizontal, 8)
                                        .padding(.vertical, 3)
                                        .background(Color.blue)
                                        .cornerRadius(4)
                                }
                            }
                        }
                        
                        Text("Go to Contexts tab (Cmd+2) to manage")
                            .font(.caption2)
                            .foregroundStyle(.tertiary)
                            .padding(.top, 4)
                    }
                }
                
                // Deployed on Nodes
                DetailSection(title: "Deployed on Nodes") {
                    VStack(alignment: .leading, spacing: 8) {
                        Text("Nodes running this server")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                        
                        let nodes = viewModel.nodesForServer(server)
                        if nodes.isEmpty {
                            Text("Not deployed on any node")
                                .font(.caption2)
                                .foregroundStyle(.tertiary)
                        } else {
                            HStack(spacing: 6) {
                                ForEach(nodes, id: \.self) { nodeName in
                                    Text(nodeName)
                                        .font(.caption2)
                                        .fontWeight(.medium)
                                        .foregroundStyle(.white)
                                        .padding(.horizontal, 8)
                                        .padding(.vertical, 3)
                                        .background(Color.green)
                                        .cornerRadius(4)
                                }
                            }
                        }
                        
                        Text("Multi-node deployment coming soon")
                            .font(.caption2)
                            .foregroundStyle(.tertiary)
                            .padding(.top, 4)
                    }
                }
            }
            .padding()
        }
    }
    
    // MARK: - Create Server Sheet
    
    private var createServerSheet: some View {
        MCPServerFormSheet(
            title: "Add MCP Server",
            isPresented: $viewModel.showCreateServer,
            name: $viewModel.newServerName,
            type: $viewModel.newServerType,
            command: $viewModel.newServerCommand,
            args: $viewModel.newServerArgs,
            env: $viewModel.newServerEnv,
            url: $viewModel.newServerURL,
            headers: $viewModel.newServerHeaders,
            isProcessing: viewModel.isCreating,
            error: viewModel.createError,
            canSubmit: viewModel.canCreateServer,
            onSubmit: {
                Task { await viewModel.createServer() }
            }
        )
    }
    
    // MARK: - Edit Server Sheet
    
    private func editServerSheet(for server: MCPServer) -> some View {
        MCPServerFormSheet(
            title: "Edit \(server.name)",
            isPresented: Binding(
                get: { viewModel.editingServer != nil },
                set: { if !$0 { viewModel.editingServer = nil } }
            ),
            name: $viewModel.editName,
            type: $viewModel.editType,
            command: $viewModel.editCommand,
            args: $viewModel.editArgs,
            env: $viewModel.editEnv,
            url: $viewModel.editURL,
            headers: $viewModel.editHeaders,
            isProcessing: viewModel.isEditing,
            error: viewModel.editError,
            canSubmit: viewModel.canSaveEdit,
            isEditing: true,
            onSubmit: {
                Task { await viewModel.updateServer(server) }
            }
        )
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

// MARK: - Info Box

private struct InfoBox: View {
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
        }
        .padding()
        .background(color.opacity(0.1))
        .cornerRadius(8)
    }
}

// MARK: - MCP Server Form Sheet

/// A reusable sheet for creating or editing an MCP server definition.
struct MCPServerFormSheet: View {
    let title: String
    @Binding var isPresented: Bool
    @Binding var name: String
    @Binding var type: MCPServerType
    @Binding var command: String
    @Binding var args: [String]
    @Binding var env: [String: String]
    @Binding var url: String
    @Binding var headers: [String: String]
    let isProcessing: Bool
    let error: String?
    let canSubmit: Bool
    var isEditing: Bool = false
    let onSubmit: () -> Void
    
    var body: some View {
        VStack(spacing: 0) {
            // Header
            HStack {
                Text(title)
                    .font(.headline)
                Spacer()
                Button {
                    isPresented = false
                } label: {
                    Image(systemName: "xmark.circle.fill")
                        .foregroundStyle(.secondary)
                }
                .buttonStyle(.plain)
            }
            .padding()
            
            Divider()
            
            // Form content
            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
                    // Name field
                    VStack(alignment: .leading, spacing: 4) {
                        Text("Name")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                        TextField("Server name", text: $name)
                            .textFieldStyle(.roundedBorder)
                    }
                    
                    // Type picker (only for create)
                    if !isEditing {
                        VStack(alignment: .leading, spacing: 4) {
                            Text("Type")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                            Picker("", selection: $type) {
                                ForEach(MCPServerType.allCases) { serverType in
                                    Text(serverType.displayName).tag(serverType)
                                }
                            }
                            .pickerStyle(.segmented)
                            
                            Text(type.description)
                                .font(.caption2)
                                .foregroundStyle(.tertiary)
                        }
                    }
                    
                    // Type-specific fields
                    switch type {
                    case .stdio:
                        VStack(alignment: .leading, spacing: 4) {
                            Text("Command")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                            TextField("e.g. /usr/local/bin/my-server", text: $command)
                                .textFieldStyle(.roundedBorder)
                                .font(.system(.body, design: .monospaced))
                        }
                        
                        StringArrayEditor(items: $args, title: "Arguments", placeholder: "e.g. --port 8080")
                        KeyValueEditor(items: $env, title: "Environment Variables", keyPlaceholder: "KEY", valuePlaceholder: "value")
                        
                    case .sse, .http:
                        VStack(alignment: .leading, spacing: 4) {
                            Text("URL")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                            TextField("e.g. https://api.example.com/mcp", text: $url)
                                .textFieldStyle(.roundedBorder)
                                .font(.system(.body, design: .monospaced))
                        }
                        
                        KeyValueEditor(items: $headers, title: "Headers", keyPlaceholder: "Header-Name", valuePlaceholder: "value")
                        
                    case .builtin:
                        Text("Built-in servers are managed by Diane.")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                    
                    // Error message
                    if let error = error {
                        HStack(spacing: 6) {
                            Image(systemName: "exclamationmark.triangle.fill")
                                .foregroundStyle(.orange)
                            Text(error)
                                .font(.caption)
                                .foregroundStyle(.red)
                        }
                    }
                }
                .padding()
            }
            
            Divider()
            
            // Footer with buttons
            HStack {
                Spacer()
                Button("Cancel") {
                    isPresented = false
                }
                .keyboardShortcut(.cancelAction)
                
                Button(isEditing ? "Save" : "Create") {
                    onSubmit()
                }
                .keyboardShortcut(.defaultAction)
                .disabled(!canSubmit)
                
                if isProcessing {
                    ProgressView()
                        .scaleEffect(0.6)
                }
            }
            .padding()
        }
        .frame(minWidth: 450, idealWidth: 500, maxWidth: 600,
               minHeight: 350, idealHeight: 450, maxHeight: 600)
    }
}
