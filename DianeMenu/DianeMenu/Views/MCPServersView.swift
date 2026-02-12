import SwiftUI

struct MCPServersView: View {
    @State private var viewModel: MCPServersViewModel
    
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
                    
                    Text("•")
                        .font(.caption)
                        .foregroundStyle(.tertiary)
                    
                    Text(server.enabled ? "Enabled" : "Disabled")
                        .font(.caption)
                        .foregroundStyle(server.enabled ? .green : .secondary)
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
                
                Divider()
                
                // Configuration details
                Group {
                    DetailSection(title: "Type") {
                        InfoRow(label: "Type", value: serverTypeDisplayName(server.type))
                        InfoRow(label: "Description", value: serverTypeDescription(server.type))
                    }
                    
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
                
                // Metadata
                DetailSection(title: "Metadata") {
                    InfoRow(label: "Created", value: formatDate(server.createdAt))
                    InfoRow(label: "Updated", value: formatDate(server.updatedAt))
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

