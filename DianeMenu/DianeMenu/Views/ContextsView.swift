import SwiftUI
import AppKit

struct ContextsView: View {
    @State private var viewModel: ContextsViewModel
    
    init(viewModel: ContextsViewModel = ContextsViewModel()) {
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
            } else if viewModel.contexts.isEmpty {
                emptyView
            } else {
                MasterDetailView {
                    contextsListView
                } detail: {
                    detailView
                }
            }
        }
        .frame(minWidth: 750, idealWidth: 900, maxWidth: .infinity,
               minHeight: 450, idealHeight: 600, maxHeight: .infinity)
        .task {
            await viewModel.loadContexts()
        }
        .sheet(isPresented: $viewModel.showCreateContext) {
            createContextSheet
        }
        .sheet(isPresented: $viewModel.showConnectionInfo) {
            connectionInfoSheet
        }
        .sheet(isPresented: $viewModel.showAddServer) {
            addServerSheet
        }
        .alert("Delete Context", isPresented: $viewModel.showDeleteConfirm) {
            Button("Cancel", role: .cancel) { }
            Button("Delete", role: .destructive) {
                if let context = viewModel.contextToDelete {
                    Task { await viewModel.deleteContext(context) }
                }
            }
        } message: {
            if let context = viewModel.contextToDelete {
                Text("Are you sure you want to delete '\(context.name)'? This cannot be undone.")
            }
        }
    }
    
    // MARK: - Header
    
    private var headerView: some View {
        HStack(spacing: 12) {
            Image(systemName: "square.stack.3d.up")
                .foregroundStyle(.secondary)
            
            Text("Contexts")
                .font(.headline)
            
            Spacer()
            
            // Create context button
            Button {
                viewModel.showCreateContext = true
            } label: {
                Label("New Context", systemImage: "plus")
            }
            
            // Refresh button
            Button {
                Task { await viewModel.loadContexts() }
            } label: {
                Image(systemName: "arrow.clockwise")
            }
            .disabled(viewModel.isLoading)
            
            // Stats
            let defaultContext = viewModel.contexts.first { $0.isDefault }
            if let defaultName = defaultContext?.name {
                Text("Default: \(defaultName)")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
        .padding()
    }
    
    // MARK: - Loading View
    
    private var loadingView: some View {
        VStack(spacing: 12) {
            ProgressView()
            Text("Loading contexts...")
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
            Text("Failed to load contexts")
                .font(.headline)
            Text(message)
                .font(.caption)
                .foregroundStyle(.secondary)
            Button("Retry") {
                Task { await viewModel.loadContexts() }
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
    
    // MARK: - Empty View
    
    private var emptyView: some View {
        EmptyStateView(
            icon: "square.stack.3d.up",
            title: "No contexts configured",
            description: "Create a context to group MCP servers and control tool access",
            actionLabel: "Create Context",
            action: { viewModel.showCreateContext = true }
        )
    }
    
    // MARK: - Contexts List
    
    private var contextsListView: some View {
        VStack(alignment: .leading, spacing: 0) {
            MasterListHeader(icon: "list.bullet", title: "Contexts")
            
            Divider()
            
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 0) {
                    ForEach(viewModel.contexts) { context in
                        ContextRow(
                            context: context,
                            isSelected: viewModel.selectedContext?.id == context.id,
                            onSelect: {
                                viewModel.onSelectContext(context)
                                Task { await viewModel.loadContextDetail(context.name) }
                            },
                            onSetDefault: {
                                Task { await viewModel.setDefaultContext(context) }
                            },
                            onDelete: {
                                viewModel.prepareDeleteContext(context)
                            }
                        )
                        Divider()
                            .padding(.leading, 16)
                    }
                }
            }
        }
    }
    
    // MARK: - Detail View
    
    private var detailView: some View {
        VStack(alignment: .leading, spacing: 0) {
            if let context = viewModel.selectedContext {
                // Context detail header
                VStack(alignment: .leading, spacing: 8) {
                    HStack {
                        Image(systemName: "square.stack.3d.up.fill")
                            .font(.title)
                            .foregroundStyle(.blue)
                        
                        VStack(alignment: .leading, spacing: 2) {
                            HStack(spacing: 8) {
                                Text(context.name)
                                    .font(.headline)
                                
                                if context.isDefault {
                                    Text("Default")
                                        .font(.caption2)
                                        .padding(.horizontal, 6)
                                        .padding(.vertical, 2)
                                        .background(Color.green.opacity(0.2))
                                        .foregroundStyle(.green)
                                        .cornerRadius(4)
                                }
                            }
                            
                            if let desc = context.description, !desc.isEmpty {
                                Text(desc)
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                            }
                        }
                        
                        Spacer()
                        
                        // Connection info button
                        Button {
                            Task {
                                await viewModel.loadConnectionInfo(context.name)
                                viewModel.showConnectionInfo = true
                            }
                        } label: {
                            Label("Connect", systemImage: "link")
                        }
                        .buttonStyle(.bordered)
                        
                        // Sync tools button
                        Button {
                            Task { await viewModel.syncTools(context.name) }
                        } label: {
                            Label("Sync Tools", systemImage: "arrow.triangle.2.circlepath")
                        }
                        .buttonStyle(.bordered)
                        .help("Sync tools from running MCP servers")
                    }
                    
                    // Summary stats
                    if let detail = viewModel.contextDetail {
                        HStack(spacing: 16) {
                            HStack(spacing: 4) {
                                Image(systemName: "server.rack")
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                                Text("\(detail.summary.serversEnabled)/\(detail.summary.serversTotal) servers")
                                    .font(.caption)
                            }
                            
                            HStack(spacing: 4) {
                                Image(systemName: "wrench.and.screwdriver")
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                                Text("\(detail.summary.toolsActive)/\(detail.summary.toolsTotal) tools")
                                    .font(.caption)
                            }
                        }
                        .foregroundStyle(.secondary)
                    }
                }
                .padding()
                .background(Color(nsColor: .windowBackgroundColor))
                
                Divider()
                
                // Servers list
                if let detail = viewModel.contextDetail {
                    serversListView(detail: detail)
                } else {
                    VStack(spacing: 12) {
                        ProgressView()
                        Text("Loading context details...")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
                }
                
            } else {
                // No context selected
                VStack(spacing: 12) {
                    Image(systemName: "arrow.left")
                        .font(.title)
                        .foregroundStyle(.secondary)
                    Text("Select a context")
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            }
        }
    }
    
    // MARK: - Servers List View
    
    private func serversListView(detail: ContextDetail) -> some View {
        VStack(alignment: .leading, spacing: 0) {
            // Section header
            HStack {
                Image(systemName: "server.rack")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                Text("Servers & Tools")
                    .font(.subheadline.weight(.semibold))
                Spacer()
                
                Button {
                    viewModel.prepareAddServer()
                    Task {
                        await viewModel.loadAvailableServers()
                    }
                } label: {
                    Label("Add Server", systemImage: "plus")
                        .font(.caption)
                }
                .buttonStyle(.bordered)
            }
            .padding(.horizontal)
            .padding(.vertical, 8)
            .background(Color(nsColor: .windowBackgroundColor))
            
            Divider()
            
            if detail.servers.isEmpty {
                VStack(spacing: 12) {
                    Image(systemName: "server.rack")
                        .font(.title)
                        .foregroundStyle(.secondary)
                    Text("No servers in this context")
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                    Text("Click 'Add Server' to add MCP servers to this context")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    
                    Button {
                        viewModel.prepareAddServer()
                        Task {
                            await viewModel.loadAvailableServers()
                        }
                    } label: {
                        Label("Add Server", systemImage: "plus")
                    }
                    .buttonStyle(.borderedProminent)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else {
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 0) {
                        ForEach(detail.servers) { server in
                            ContextServerRow(
                                server: server,
                                contextName: viewModel.selectedContext?.name ?? "",
                                client: viewModel.client,
                                onUpdate: {
                                    if let name = viewModel.selectedContext?.name {
                                        Task { await viewModel.loadContextDetail(name) }
                                    }
                                }
                            )
                            Divider()
                                .padding(.leading, 16)
                        }
                    }
                }
            }
        }
    }
    
    // MARK: - Create Context Sheet
    
    private var createContextSheet: some View {
        VStack(spacing: 0) {
            // Header
            HStack {
                Text("Create New Context")
                    .font(.headline)
                Spacer()
                Button {
                    viewModel.showCreateContext = false
                    viewModel.resetCreateForm()
                } label: {
                    Image(systemName: "xmark.circle.fill")
                        .foregroundStyle(.secondary)
                }
                .buttonStyle(.plain)
            }
            .padding()
            
            Divider()
            
            VStack(alignment: .leading, spacing: 16) {
                VStack(alignment: .leading, spacing: 4) {
                    Text("Name")
                        .font(.subheadline.weight(.medium))
                    TextField("e.g., work, personal, dev", text: $viewModel.newContextName)
                        .textFieldStyle(.roundedBorder)
                    Text("A unique identifier for this context (lowercase, no spaces)")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                
                VStack(alignment: .leading, spacing: 4) {
                    Text("Description")
                        .font(.subheadline.weight(.medium))
                    TextField("Optional description", text: $viewModel.newContextDescription)
                        .textFieldStyle(.roundedBorder)
                }
                
                if let error = viewModel.createError {
                    HStack(spacing: 4) {
                        Image(systemName: "exclamationmark.triangle.fill")
                            .foregroundStyle(.red)
                        Text(error)
                            .foregroundStyle(.red)
                    }
                    .font(.caption)
                }
            }
            .padding()
            
            Spacer()
            
            Divider()
            
            // Footer
            HStack {
                Button("Cancel") {
                    viewModel.showCreateContext = false
                    viewModel.resetCreateForm()
                }
                .keyboardShortcut(.escape)
                
                Spacer()
                
                Button {
                    Task { await viewModel.createContext() }
                } label: {
                    if viewModel.isCreating {
                        ProgressView()
                            .scaleEffect(0.7)
                    } else {
                        Text("Create")
                    }
                }
                .buttonStyle(.borderedProminent)
                .disabled(viewModel.newContextName.isEmpty || viewModel.isCreating)
                .keyboardShortcut(.return)
            }
            .padding()
        }
        .frame(width: 400, height: 300)
    }
    
    // MARK: - Connection Info Sheet
    
    private var connectionInfoSheet: some View {
        VStack(spacing: 0) {
            // Header
            HStack {
                Text("Connection Info")
                    .font(.headline)
                Spacer()
                Button {
                    viewModel.showConnectionInfo = false
                } label: {
                    Image(systemName: "xmark.circle.fill")
                        .foregroundStyle(.secondary)
                }
                .buttonStyle(.plain)
            }
            .padding()
            
            Divider()
            
            if let info = viewModel.connectionInfo {
                ScrollView {
                    VStack(alignment: .leading, spacing: 16) {
                        // Context name
                        VStack(alignment: .leading, spacing: 4) {
                            Text("Context")
                                .font(.subheadline.weight(.medium))
                            Text(info.context)
                                .font(.system(.body, design: .monospaced))
                        }
                        
                        if let desc = info.description, !desc.isEmpty {
                            Text(desc)
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        }
                        
                        // SSE connection
                        VStack(alignment: .leading, spacing: 4) {
                            Text("SSE Endpoint")
                                .font(.subheadline.weight(.medium))
                            HStack {
                                Text(info.sse.url)
                                    .font(.system(.caption, design: .monospaced))
                                    .textSelection(.enabled)
                                Spacer()
                                Button {
                                    copyToClipboard(info.sse.url)
                                } label: {
                                    Image(systemName: "doc.on.doc")
                                }
                                .buttonStyle(.plain)
                                .help("Copy URL")
                            }
                            .padding(8)
                            .background(Color(nsColor: .textBackgroundColor))
                            .cornerRadius(6)
                            
                            if let example = info.sse.example, !example.isEmpty {
                                Text("Example:")
                                    .font(.caption.weight(.medium))
                                Text(example)
                                    .font(.system(.caption2, design: .monospaced))
                                    .textSelection(.enabled)
                                    .padding(6)
                                    .background(Color(nsColor: .textBackgroundColor))
                                    .cornerRadius(4)
                            }
                        }
                        
                        // Streamable HTTP connection
                        VStack(alignment: .leading, spacing: 4) {
                            Text("Streamable HTTP Endpoint")
                                .font(.subheadline.weight(.medium))
                            HStack {
                                Text(info.streamable.url)
                                    .font(.system(.caption, design: .monospaced))
                                    .textSelection(.enabled)
                                Spacer()
                                Button {
                                    copyToClipboard(info.streamable.url)
                                } label: {
                                    Image(systemName: "doc.on.doc")
                                }
                                .buttonStyle(.plain)
                                .help("Copy URL")
                            }
                            .padding(8)
                            .background(Color(nsColor: .textBackgroundColor))
                            .cornerRadius(6)
                            
                            if let example = info.streamable.example, !example.isEmpty {
                                Text("Example:")
                                    .font(.caption.weight(.medium))
                                Text(example)
                                    .font(.system(.caption2, design: .monospaced))
                                    .textSelection(.enabled)
                                    .padding(6)
                                    .background(Color(nsColor: .textBackgroundColor))
                                    .cornerRadius(4)
                            }
                        }
                    }
                    .padding()
                }
            } else {
                VStack(spacing: 12) {
                    ProgressView()
                    Text("Loading connection info...")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            }
            
            Divider()
            
            // Footer
            HStack {
                Spacer()
                Button("Done") {
                    viewModel.showConnectionInfo = false
                }
                .keyboardShortcut(.escape)
            }
            .padding()
        }
        .frame(width: 500, height: 400)
    }
    
    // MARK: - Actions
    
    private func copyToClipboard(_ text: String) {
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString(text, forType: .string)
    }
    
    // MARK: - Add Server Sheet
    
    private var addServerSheet: some View {
        VStack(spacing: 0) {
            // Header
            HStack {
                Text("Add Server to Context")
                    .font(.headline)
                Spacer()
                Button {
                    viewModel.showAddServer = false
                } label: {
                    Image(systemName: "xmark.circle.fill")
                        .foregroundStyle(.secondary)
                }
                .buttonStyle(.plain)
            }
            .padding()
            
            Divider()
            
            if viewModel.isLoadingServers {
                VStack(spacing: 12) {
                    ProgressView()
                    Text("Loading available servers...")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else if viewModel.availableServers.isEmpty {
                VStack(spacing: 12) {
                    Image(systemName: "server.rack")
                        .font(.largeTitle)
                        .foregroundStyle(.secondary)
                    Text("No servers available")
                        .font(.headline)
                    Text("Make sure Diane is running and MCP servers are connected")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else {
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 0) {
                        ForEach(viewModel.availableServers) { server in
                            HStack(spacing: 12) {
                                Image(systemName: server.builtin == true ? "cpu" : "server.rack")
                                    .font(.body)
                                    .foregroundStyle(server.inContext ? .green : .secondary)
                                
                                VStack(alignment: .leading, spacing: 2) {
                                    HStack(spacing: 8) {
                                        Text(server.name)
                                            .font(.system(.body, design: .default))
                                            .fontWeight(.medium)
                                        
                                        if server.builtin == true {
                                            Text("builtin")
                                                .font(.caption2)
                                                .foregroundStyle(.blue)
                                                .padding(.horizontal, 4)
                                                .padding(.vertical, 2)
                                                .background(Color.blue.opacity(0.15))
                                                .cornerRadius(4)
                                        }
                                        
                                        if server.inContext {
                                            Text("added")
                                                .font(.caption2)
                                                .foregroundStyle(.green)
                                                .padding(.horizontal, 4)
                                                .padding(.vertical, 2)
                                                .background(Color.green.opacity(0.15))
                                                .cornerRadius(4)
                                        }
                                    }
                                    
                                    Text("\(server.toolCount) tools")
                                        .font(.caption)
                                        .foregroundStyle(.secondary)
                                }
                                
                                Spacer()
                                
                                if !server.inContext {
                                    Button {
                                        Task { await viewModel.addServer(server.name) }
                                    } label: {
                                        Text("Add")
                                    }
                                    .buttonStyle(.borderedProminent)
                                }
                            }
                            .padding(.horizontal)
                            .padding(.vertical, 10)
                            
                            Divider()
                                .padding(.leading, 16)
                        }
                    }
                }
            }
            
            Divider()
            
            // Footer
            HStack {
                Text("\(viewModel.availableServers.filter { !$0.inContext }.count) servers available")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                Spacer()
                Button("Done") {
                    viewModel.showAddServer = false
                }
                .keyboardShortcut(.escape)
            }
            .padding()
        }
        .frame(width: 450, height: 400)
    }
}

// MARK: - Context Row

struct ContextRow: View {
    let context: Context
    let isSelected: Bool
    let onSelect: () -> Void
    let onSetDefault: () -> Void
    let onDelete: () -> Void
    
    @State private var isHovering = false
    
    var body: some View {
        Button(action: onSelect) {
            HStack(spacing: 12) {
                // Icon
                Image(systemName: context.isDefault ? "square.stack.3d.up.fill" : "square.stack.3d.up")
                    .font(.body)
                    .foregroundStyle(context.isDefault ? .green : .secondary)
                
                VStack(alignment: .leading, spacing: 2) {
                    HStack(spacing: 8) {
                        Text(context.name)
                            .font(.system(.body, design: .default))
                            .fontWeight(.medium)
                        
                        if context.isDefault {
                            Text("default")
                                .font(.caption2)
                                .foregroundStyle(.green)
                                .padding(.horizontal, 4)
                                .padding(.vertical, 2)
                                .background(Color.green.opacity(0.15))
                                .cornerRadius(4)
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
                
                // Actions
                if isHovering && !context.isDefault {
                    Button {
                        onSetDefault()
                    } label: {
                        Image(systemName: "star")
                            .font(.caption)
                    }
                    .buttonStyle(.plain)
                    .help("Set as default")
                }
                
                Button {
                    onDelete()
                } label: {
                    Image(systemName: "trash")
                        .font(.caption)
                        .foregroundStyle(.red.opacity(0.7))
                }
                .buttonStyle(.plain)
                .help("Delete context")
            }
            .padding(.horizontal, 16)
            .padding(.vertical, 10)
            .background(isSelected ? Color.accentColor.opacity(0.1) : (isHovering ? Color.primary.opacity(0.05) : Color.clear))
        }
        .buttonStyle(.plain)
        .onHover { hovering in
            isHovering = hovering
        }
    }
}

// MARK: - Context Server Row

struct ContextServerRow: View {
    let server: ContextServer
    let contextName: String
    let client: DianeClientProtocol
    let onUpdate: () -> Void
    
    @State private var isExpanded = false
    @State private var isHovering = false
    @State private var tools: [ContextTool] = []
    @State private var isLoadingTools = false
    
    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            // Server header
            Button {
                withAnimation(.easeInOut(duration: 0.2)) {
                    isExpanded.toggle()
                }
                if isExpanded && tools.isEmpty {
                    Task { await loadTools() }
                }
            } label: {
                HStack(spacing: 12) {
                    Image(systemName: isExpanded ? "chevron.down" : "chevron.right")
                        .font(.caption2)
                        .frame(width: 10)
                    
                    Circle()
                        .fill(server.enabled ? .green : .secondary)
                        .frame(width: 8, height: 8)
                    
                    VStack(alignment: .leading, spacing: 2) {
                        Text(server.name)
                            .font(.system(.body, design: .default))
                            .fontWeight(.medium)
                        
                        HStack(spacing: 8) {
                            if !server.type.isEmpty {
                                Text(server.type)
                                    .font(.caption2)
                                    .foregroundStyle(.secondary)
                            }
                            
                            Text("\(server.toolsActive)/\(server.toolsTotal) tools")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        }
                    }
                    
                    Spacer()
                    
                    // Enable/disable toggle
                    Toggle("", isOn: Binding(
                        get: { server.enabled },
                        set: { enabled in
                            Task { await toggleServer(enabled: enabled) }
                        }
                    ))
                    .toggleStyle(.switch)
                    .labelsHidden()
                    .scaleEffect(0.8)
                }
                .padding(.horizontal, 16)
                .padding(.vertical, 10)
                .background(isHovering ? Color.primary.opacity(0.05) : Color.clear)
            }
            .buttonStyle(.plain)
            .onHover { hovering in
                isHovering = hovering
            }
            
            // Expanded tools list
            if isExpanded {
                if isLoadingTools {
                    HStack {
                        Spacer()
                        ProgressView()
                            .scaleEffect(0.7)
                        Text("Loading tools...")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                        Spacer()
                    }
                    .padding()
                } else if tools.isEmpty {
                    Text("No tools available")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .padding(.leading, 40)
                        .padding(.vertical, 8)
                } else {
                    VStack(alignment: .leading, spacing: 0) {
                        // Bulk actions
                        HStack(spacing: 8) {
                            Button("Enable All") {
                                Task { await setAllTools(enabled: true) }
                            }
                            .font(.caption)
                            .buttonStyle(.bordered)
                            
                            Button("Disable All") {
                                Task { await setAllTools(enabled: false) }
                            }
                            .font(.caption)
                            .buttonStyle(.bordered)
                            
                            Spacer()
                        }
                        .padding(.horizontal, 40)
                        .padding(.vertical, 8)
                        
                        ForEach(tools) { tool in
                            ContextToolRow(
                                tool: tool,
                                onToggle: { enabled in
                                    Task { await toggleTool(tool.name, enabled: enabled) }
                                }
                            )
                        }
                    }
                    .padding(.bottom, 8)
                }
            }
        }
    }
    
    private func loadTools() async {
        isLoadingTools = true
        do {
            tools = try await client.getContextServerTools(contextName: contextName, serverName: server.name)
        } catch {
            tools = []
        }
        isLoadingTools = false
    }
    
    private func toggleServer(enabled: Bool) async {
        do {
            try await client.setServerEnabledInContext(contextName: contextName, serverName: server.name, enabled: enabled)
            onUpdate()
        } catch {
            // Show error somehow
        }
    }
    
    private func toggleTool(_ toolName: String, enabled: Bool) async {
        do {
            try await client.setToolEnabledInContext(contextName: contextName, serverName: server.name, toolName: toolName, enabled: enabled)
            // Update local state
            if let index = tools.firstIndex(where: { $0.name == toolName }) {
                tools[index] = ContextTool(name: toolName, description: tools[index].description, enabled: enabled)
            }
            onUpdate()
        } catch {
            // Show error somehow
        }
    }
    
    private func setAllTools(enabled: Bool) async {
        do {
            var updates: [String: Bool] = [:]
            for tool in tools {
                updates[tool.name] = enabled
            }
            try await client.bulkSetToolsEnabled(contextName: contextName, serverName: server.name, tools: updates)
            await loadTools()
            onUpdate()
        } catch {
            // Show error somehow
        }
    }
}

// MARK: - Context Tool Row

struct ContextToolRow: View {
    let tool: ContextTool
    let onToggle: (Bool) -> Void
    
    var body: some View {
        HStack(spacing: 12) {
            Image(systemName: "wrench.and.screwdriver")
                .font(.caption)
                .foregroundStyle(.secondary)
            
            VStack(alignment: .leading, spacing: 2) {
                Text(tool.name)
                    .font(.caption)
                    .fontWeight(.medium)
                
                if let desc = tool.description, !desc.isEmpty {
                    Text(desc)
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                }
            }
            
            Spacer()
            
            Toggle("", isOn: Binding(
                get: { tool.enabled },
                set: { onToggle($0) }
            ))
            .toggleStyle(.switch)
            .labelsHidden()
            .scaleEffect(0.7)
        }
        .padding(.horizontal, 40)
        .padding(.vertical, 4)
    }
}

// MARK: - Preview

#Preview {
    ContextsView()
}
