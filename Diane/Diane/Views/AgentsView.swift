import SwiftUI
import AppKit

struct AgentsView: View {
    @State private var viewModel: AgentsViewModel
    
    init(viewModel: AgentsViewModel = AgentsViewModel()) {
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
            } else if viewModel.agents.isEmpty {
                emptyView
            } else {
                MasterDetailView {
                    agentsListView
                } detail: {
                    detailView
                }
            }
        }
        .frame(minWidth: 750, idealWidth: 950, maxWidth: .infinity,
               minHeight: 450, idealHeight: 650, maxHeight: .infinity)
        .task {
            await viewModel.loadData()
        }
    }
    
    // MARK: - Header
    
    private var headerView: some View {
        HStack(spacing: 12) {
            Image(systemName: "person.3.fill")
                .foregroundStyle(.secondary)
            
            Text("ACP Agents")
                .font(.headline)
            
            Spacer()
            
            // Add Agent button
            Button {
                viewModel.showAddAgent = true
                Task { await viewModel.loadGallery() }
            } label: {
                Label("Add Agent", systemImage: "plus")
            }
            
            // Refresh button
            Button {
                Task { await viewModel.loadData() }
            } label: {
                Image(systemName: "arrow.clockwise")
            }
            .disabled(viewModel.isLoading)
            
            // Stats
            let enabledCount = viewModel.agents.filter { $0.enabled }.count
            Text("\(enabledCount)/\(viewModel.agents.count) enabled")
                .font(.caption)
                .foregroundStyle(.secondary)
        }
        .padding()
        .sheet(isPresented: $viewModel.showAddAgent) {
            addAgentSheet
        }
    }
    
    // MARK: - Loading View
    
    private var loadingView: some View {
        VStack(spacing: 12) {
            ProgressView()
            Text("Loading agents...")
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
            Text("Failed to load agents")
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
            icon: "person.3",
            title: "No agents configured",
            description: "Use the gallery to add agents:"
        ) {
            Text("./scripts/acp-gallery.sh install gemini")
                .font(.system(.caption, design: .monospaced))
                .padding(Padding.small)
                .background(Color(nsColor: .textBackgroundColor))
                .cornerRadius(CornerRadius.medium)
        }
    }
    
    // MARK: - Agents List
    
    private var agentsListView: some View {
        VStack(alignment: .leading, spacing: 0) {
            MasterListHeader(icon: "list.bullet", title: "Configured Agents")
            
            Divider()
            
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 0) {
                    ForEach(viewModel.agents) { agent in
                        AgentRow(
                            agent: agent,
                            testResult: viewModel.testResults[agent.name],
                            isSelected: viewModel.selectedAgent?.name == agent.name,
                            onTest: {
                                Task { await viewModel.testAgent(agent) }
                            },
                            onToggle: { enabled in
                                Task { await viewModel.toggleAgent(agent, enabled: enabled) }
                            },
                            onSelect: {
                                viewModel.onSelectAgent(agent)
                                Task { await viewModel.loadLogs(forAgent: agent.name) }
                            },
                            onRemove: {
                                Task { await viewModel.removeAgent(name: agent.name) }
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
            if let agent = viewModel.selectedAgent {
                // Agent detail header
                VStack(alignment: .leading, spacing: 8) {
                    HStack {
                        Image(systemName: "person.circle.fill")
                            .font(.title)
                            .foregroundStyle(.blue)
                        
                        VStack(alignment: .leading, spacing: 2) {
                            Text(agent.displayName)
                                .font(.headline)
                            if let workspace = agent.workspaceName {
                                HStack(spacing: 4) {
                                    Image(systemName: "folder")
                                        .font(.caption2)
                                    Text(workspace)
                                        .font(.caption)
                                }
                                .foregroundStyle(.secondary)
                            }
                        }
                        
                        Spacer()
                        
                        // Status badge
                        if let result = viewModel.testResults[agent.name] {
                            statusBadge(for: result)
                        }
                    }
                    
                    if let desc = agent.description {
                        Text(desc)
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                    
                    // Workdir info
                    if let workdir = agent.workdir {
                        HStack(spacing: 4) {
                            Image(systemName: "folder.fill")
                                .font(.caption)
                            Text(workdir)
                                .font(.system(.caption, design: .monospaced))
                        }
                        .foregroundStyle(.secondary)
                    }
                    
                    // Current sub-agent info
                    if let subAgent = agent.subAgent, !subAgent.isEmpty {
                        HStack(spacing: 4) {
                            Image(systemName: "person.2.fill")
                                .font(.caption)
                            Text("Sub-agent: \(subAgent)")
                                .font(.caption)
                        }
                        .foregroundStyle(.blue)
                    }
                }
                .padding()
                .background(Color(nsColor: .windowBackgroundColor))
                
                // Sub-agent configuration section (for ACP agents)
                if agent.type == "acp" || agent.type == nil {
                    subAgentConfigSection(agent: agent)
                }
                
                // Connection error section
                if let result = viewModel.testResults[agent.name], let error = result.error {
                    VStack(alignment: .leading, spacing: 8) {
                        HStack(spacing: 6) {
                            Image(systemName: "exclamationmark.triangle.fill")
                                .foregroundStyle(.orange)
                            Text("Connection Error")
                                .font(.subheadline.weight(.semibold))
                            Spacer()
                            Button {
                                Task { await viewModel.testAgent(agent) }
                            } label: {
                                Label("Retry", systemImage: "arrow.clockwise")
                                    .font(.caption)
                            }
                            .buttonStyle(.bordered)
                        }
                        
                        Text(error)
                            .font(.system(.caption, design: .monospaced))
                            .foregroundStyle(.secondary)
                            .textSelection(.enabled)
                            .padding(8)
                            .frame(maxWidth: .infinity, alignment: .leading)
                            .background(Color.red.opacity(0.1))
                            .cornerRadius(6)
                    }
                    .padding()
                    .background(Color.orange.opacity(0.05))
                }
                
                Divider()
                
                // Test message section
                testMessageSection(agent: agent)
                
                Divider()
                
                // Logs section
                logsSection(agent: agent)
                
            } else {
                // No agent selected
                VStack(spacing: 12) {
                    Image(systemName: "arrow.left")
                        .font(.title)
                        .foregroundStyle(.secondary)
                    Text("Select an agent")
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            }
        }
    }
    
    // MARK: - Sub-Agent Configuration Section
    
    private func subAgentConfigSection(agent: AgentConfig) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Image(systemName: "person.2.fill")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                Text("Sub-Agent Configuration")
                    .font(.subheadline.weight(.semibold))
                Spacer()
                
                Button {
                    Task { await viewModel.loadRemoteAgents(for: agent) }
                } label: {
                    if viewModel.isLoadingRemoteAgents {
                        ProgressView()
                            .scaleEffect(0.6)
                    } else {
                        Label("Discover", systemImage: "arrow.triangle.2.circlepath")
                            .font(.caption)
                    }
                }
                .buttonStyle(.bordered)
                .disabled(viewModel.isLoadingRemoteAgents || !agent.enabled)
            }
            
            if let error = viewModel.remoteAgentsError {
                HStack(spacing: 4) {
                    Image(systemName: "exclamationmark.triangle.fill")
                        .foregroundStyle(.orange)
                    Text(error)
                        .font(.caption)
                        .foregroundStyle(.orange)
                }
            }
            
            if viewModel.remoteAgents.isEmpty {
                Text("Click 'Discover' to find available sub-agents from this ACP server")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            } else {
                // Show config options as pickers
                ForEach(viewModel.remoteAgents) { remoteAgent in
                    VStack(alignment: .leading, spacing: 4) {
                        Text(remoteAgent.name)
                            .font(.caption.weight(.medium))
                        
                        if let desc = remoteAgent.description, !desc.isEmpty {
                            Text(desc)
                                .font(.caption2)
                                .foregroundStyle(.secondary)
                        }
                        
                        if let options = remoteAgent.options, !options.isEmpty {
                            HStack {
                                Picker("", selection: $viewModel.selectedSubAgent) {
                                    Text("Default").tag("")
                                    ForEach(options, id: \.self) { option in
                                        Text(option).tag(option)
                                    }
                                }
                                .pickerStyle(.menu)
                                .frame(maxWidth: 200)
                                .labelsHidden()
                                
                                if viewModel.isSavingSubAgent {
                                    ProgressView()
                                        .scaleEffect(0.6)
                                } else {
                                    Button("Save") {
                                        Task { await viewModel.saveSubAgent(for: agent) }
                                    }
                                    .buttonStyle(.borderedProminent)
                                    .disabled(viewModel.selectedSubAgent == (agent.subAgent ?? ""))
                                }
                            }
                        }
                    }
                    .padding(8)
                    .background(Color(nsColor: .textBackgroundColor))
                    .cornerRadius(6)
                }
            }
        }
        .padding()
        .background(Color.blue.opacity(0.03))
        .onAppear {
            viewModel.selectedSubAgent = agent.subAgent ?? ""
        }
        .onChange(of: viewModel.selectedAgent) {
            viewModel.onSelectedAgentChanged()
        }
    }
    
    // MARK: - Test Message Section
    
    private func testMessageSection(agent: AgentConfig) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Image(systemName: "bubble.left.and.bubble.right")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                Text("Send Test Message")
                    .font(.subheadline.weight(.semibold))
                Spacer()
            }
            
            HStack(spacing: 8) {
                TextField("Enter a prompt...", text: $viewModel.testPrompt)
                    .textFieldStyle(.roundedBorder)
                
                Button {
                    Task { await viewModel.runPrompt(agent: agent) }
                } label: {
                    if viewModel.isRunningPrompt {
                        ProgressView()
                            .scaleEffect(0.7)
                    } else {
                        Image(systemName: "paperplane.fill")
                    }
                }
                .disabled(viewModel.testPrompt.isEmpty || viewModel.isRunningPrompt || !agent.enabled)
                .buttonStyle(.borderedProminent)
            }
            
            // Result display
            if let result = viewModel.promptResult {
                VStack(alignment: .leading, spacing: 4) {
                    if let error = result.error {
                        HStack(spacing: 4) {
                            Image(systemName: "xmark.circle.fill")
                                .foregroundStyle(.red)
                            Text("Error")
                                .font(.caption.weight(.semibold))
                        }
                        Text(error.displayMessage)
                            .font(.system(.caption, design: .monospaced))
                            .foregroundStyle(.red)
                    } else {
                        let output = result.textOutput
                        if !output.isEmpty {
                            HStack(spacing: 4) {
                                Image(systemName: "checkmark.circle.fill")
                                    .foregroundStyle(.green)
                                Text("Response")
                                    .font(.caption.weight(.semibold))
                            }
                            ScrollView {
                                Text(output)
                                    .font(.system(.caption, design: .monospaced))
                                    .textSelection(.enabled)
                                    .frame(maxWidth: .infinity, alignment: .leading)
                            }
                            .frame(maxHeight: 150)
                            .padding(8)
                            .background(Color(nsColor: .textBackgroundColor))
                            .cornerRadius(6)
                        }
                    }
                }
            }
        }
        .padding()
    }
    
    // MARK: - Logs Section
    
    private func logsSection(agent: AgentConfig) -> some View {
        VStack(alignment: .leading, spacing: 0) {
            HStack {
                Image(systemName: "doc.text")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                Text("Communication Logs")
                    .font(.subheadline.weight(.semibold))
                Spacer()
                
                Button {
                    Task { await viewModel.loadLogs(forAgent: agent.name) }
                } label: {
                    Image(systemName: "arrow.clockwise")
                        .font(.caption)
                }
                .buttonStyle(.plain)
            }
            .padding(.horizontal)
            .padding(.vertical, 8)
            .background(Color(nsColor: .windowBackgroundColor))
            
            Divider()
            
            if viewModel.logs.isEmpty {
                VStack(spacing: 8) {
                    Image(systemName: "doc.text.magnifyingglass")
                        .font(.title2)
                        .foregroundStyle(.secondary)
                    Text("No communication logs")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else {
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 0) {
                        ForEach(viewModel.logs) { log in
                            LogRow(log: log)
                            Divider()
                                .padding(.leading, 16)
                        }
                    }
                }
            }
        }
    }
    
    // MARK: - Status Badge
    
    private func statusBadge(for result: AgentTestResult) -> some View {
        HStack(spacing: 4) {
            Circle()
                .fill(statusColor(for: result.status))
                .frame(width: 8, height: 8)
            Text(result.status)
                .font(.caption)
        }
        .padding(.horizontal, 8)
        .padding(.vertical, 4)
        .background(statusColor(for: result.status).opacity(0.15))
        .cornerRadius(12)
    }
    
    private func statusColor(for status: String) -> Color {
        switch status {
        case "connected": return .green
        case "unreachable": return .red
        case "error": return .orange
        case "disabled": return .gray
        default: return .gray
        }
    }
    
    // MARK: - Add Agent Sheet
    
    private var addAgentSheet: some View {
        VStack(spacing: 0) {
            // Header
            HStack {
                Text("Add Agent from Gallery")
                    .font(.headline)
                Spacer()
                Button {
                    viewModel.showAddAgent = false
                } label: {
                    Image(systemName: "xmark.circle.fill")
                        .foregroundStyle(.secondary)
                }
                .buttonStyle(.plain)
            }
            .padding()
            
            Divider()
            
            if viewModel.isLoadingGallery {
                VStack(spacing: 12) {
                    ProgressView()
                    Text("Loading gallery...")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else if viewModel.galleryEntries.isEmpty {
                VStack(spacing: 12) {
                    Image(systemName: "tray")
                        .font(.largeTitle)
                        .foregroundStyle(.secondary)
                    Text("No agents available")
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else {
                // Gallery list
                List(viewModel.galleryEntries, selection: $viewModel.selectedGalleryEntry) { entry in
                    GalleryEntryRow(entry: entry, isSelected: viewModel.selectedGalleryEntry?.id == entry.id)
                        .tag(entry)
                }
                .listStyle(.inset)
                
                Divider()
                
                // Configuration section
                if let entry = viewModel.selectedGalleryEntry {
                    VStack(alignment: .leading, spacing: 12) {
                        Text("Configuration")
                            .font(.subheadline.weight(.semibold))
                        
                        HStack {
                            Text("Name:")
                                .frame(width: 70, alignment: .trailing)
                            TextField("Leave empty for default", text: $viewModel.newAgentName)
                                .textFieldStyle(.roundedBorder)
                        }
                        
                        HStack {
                            Text("Workdir:")
                                .frame(width: 70, alignment: .trailing)
                            TextField("Optional working directory", text: $viewModel.newAgentWorkdir)
                                .textFieldStyle(.roundedBorder)
                        }
                        
                        HStack {
                            Text("Port:")
                                .frame(width: 70, alignment: .trailing)
                            TextField("e.g. 4322 for ACP agents", text: $viewModel.newAgentPort)
                                .textFieldStyle(.roundedBorder)
                                .frame(width: 150)
                            Spacer()
                        }
                        
                        if entry.id == "opencode" {
                            Text("OpenCode uses port 4322 by default when running in ACP mode")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        } else {
                            Text("Set a port to connect to an already-running ACP agent")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        }
                        
                        // Show install error if any
                        if let error = viewModel.installError {
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
                }
            }
            
            Divider()
            
            // Footer
            HStack {
                Button("Cancel") {
                    viewModel.showAddAgent = false
                }
                .keyboardShortcut(.escape)
                
                Spacer()
                
                Button {
                    Task { await viewModel.installAgent() }
                } label: {
                    if viewModel.isInstalling {
                        ProgressView()
                            .scaleEffect(0.7)
                    } else {
                        Text("Install")
                    }
                }
                .buttonStyle(.borderedProminent)
                .disabled(viewModel.selectedGalleryEntry == nil || viewModel.isInstalling)
                .keyboardShortcut(.return)
            }
            .padding()
        }
        .frame(width: 500, height: 450)
    }
}

// MARK: - Gallery Entry Row

struct GalleryEntryRow: View {
    let entry: GalleryEntry
    let isSelected: Bool
    
    var body: some View {
        HStack(spacing: 12) {
            VStack(alignment: .leading, spacing: 4) {
                HStack(spacing: 8) {
                    Text(entry.id)
                        .font(.system(.body, design: .default))
                        .fontWeight(.medium)
                    
                    if entry.featured {
                        Text("Featured")
                            .font(.caption2)
                            .padding(.horizontal, 6)
                            .padding(.vertical, 2)
                            .background(Color.orange.opacity(0.2))
                            .foregroundStyle(.orange)
                            .cornerRadius(4)
                    }
                }
                
                if !entry.description.isEmpty {
                    Text(entry.description)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .lineLimit(2)
                }
                
                HStack(spacing: 8) {
                    Text(entry.provider)
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                    
                    Text(entry.installType)
                        .font(.system(.caption2, design: .monospaced))
                        .foregroundStyle(.secondary)
                }
            }
            
            Spacer()
        }
        .padding(.vertical, 4)
        .contentShape(Rectangle())
    }
}

// MARK: - Agent Row

struct AgentRow: View {
    let agent: AgentConfig
    let testResult: AgentTestResult?
    let isSelected: Bool
    let onTest: () -> Void
    let onToggle: (Bool) -> Void
    let onSelect: () -> Void
    let onRemove: () -> Void
    
    @State private var isHovering = false
    @State private var isTesting = false
    @State private var showDeleteConfirm = false
    
    var body: some View {
        Button(action: onSelect) {
            HStack(spacing: 12) {
                // Status indicator
                Circle()
                    .fill(statusColor)
                    .frame(width: 8, height: 8)
                
                VStack(alignment: .leading, spacing: 4) {
                    // Agent name
                    HStack(spacing: 8) {
                        Text(agent.displayName)
                            .font(.system(.body, design: .default))
                            .fontWeight(.medium)
                        
                        if let workspace = agent.workspaceName {
                            Text("@\(workspace)")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        }
                        
                        if !agent.enabled {
                            Text("disabled")
                                .font(.caption2)
                                .foregroundStyle(.secondary)
                                .padding(.horizontal, 4)
                                .padding(.vertical, 2)
                                .background(Color.secondary.opacity(0.15))
                                .cornerRadius(4)
                        }
                    }
                    
                    // Type and description
                    HStack(spacing: 4) {
                        if let type = agent.type {
                            Text(type)
                                .font(.caption)
                                .padding(.horizontal, 4)
                                .padding(.vertical, 1)
                                .background(Color.blue.opacity(0.15))
                                .cornerRadius(3)
                        }
                        
                        if let desc = agent.description, !desc.isEmpty {
                            Text(desc.prefix(40) + (desc.count > 40 ? "..." : ""))
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        }
                    }
                }
                
                Spacer()
                
                // Reconnect button
                Button {
                    isTesting = true
                    onTest()
                    DispatchQueue.main.asyncAfter(deadline: .now() + 2) {
                        isTesting = false
                    }
                } label: {
                    if isTesting {
                        ProgressView()
                            .scaleEffect(0.6)
                    } else {
                        Image(systemName: "arrow.triangle.2.circlepath")
                            .font(.caption)
                    }
                }
                .buttonStyle(.plain)
                .help("Reconnect / Test connection")
                .disabled(!agent.enabled)
                
                // Toggle
                Toggle("", isOn: Binding(
                    get: { agent.enabled },
                    set: { onToggle($0) }
                ))
                .toggleStyle(.switch)
                .labelsHidden()
                .scaleEffect(0.8)
                
                // Delete button
                Button {
                    showDeleteConfirm = true
                } label: {
                    Image(systemName: "trash")
                        .font(.caption)
                        .foregroundStyle(.red.opacity(0.7))
                }
                .buttonStyle(.plain)
                .help("Remove agent")
            }
            .padding(.horizontal, 16)
            .padding(.vertical, 10)
            .background(isSelected ? Color.accentColor.opacity(0.1) : (isHovering ? Color.primary.opacity(0.05) : Color.clear))
        }
        .buttonStyle(.plain)
        .onHover { hovering in
            isHovering = hovering
        }
        .alert("Remove Agent", isPresented: $showDeleteConfirm) {
            Button("Cancel", role: .cancel) { }
            Button("Remove", role: .destructive) {
                onRemove()
            }
        } message: {
            Text("Are you sure you want to remove '\(agent.name)'? This cannot be undone.")
        }
    }
    
    private var statusColor: Color {
        if !agent.enabled {
            return .secondary
        }
        if let result = testResult {
            switch result.status {
            case "connected": return .green
            case "unreachable": return .red
            case "error": return .orange
            default: return .blue
            }
        }
        return .blue
    }
}

// MARK: - Log Row

struct LogRow: View {
    let log: AgentLog
    @State private var isExpanded = false
    @State private var isHovering = false
    
    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            Button {
                withAnimation(.easeInOut(duration: 0.2)) {
                    isExpanded.toggle()
                }
            } label: {
                HStack(spacing: 12) {
                    // Direction icon
                    Image(systemName: log.isRequest ? "arrow.up.circle.fill" : "arrow.down.circle.fill")
                        .font(.caption)
                        .foregroundStyle(log.isRequest ? .blue : .green)
                    
                    // Timestamp
                    Text(formatTime(log.timestamp))
                        .font(.caption.monospaced())
                        .foregroundStyle(.secondary)
                    
                    // Message type
                    Text(log.messageType)
                        .font(.caption.weight(.medium))
                    
                    Spacer()
                    
                    // Error indicator
                    if log.isError {
                        Image(systemName: "exclamationmark.triangle.fill")
                            .font(.caption)
                            .foregroundStyle(.orange)
                    }
                    
                    // Duration
                    if let duration = log.formattedDuration {
                        Text(duration)
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                    
                    // Expand indicator
                    Image(systemName: isExpanded ? "chevron.down" : "chevron.right")
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                }
                .padding(.horizontal, 16)
                .padding(.vertical, 8)
                .background(isHovering ? Color.primary.opacity(0.05) : Color.clear)
            }
            .buttonStyle(.plain)
            .onHover { hovering in
                isHovering = hovering
            }
            
            // Expanded content
            if isExpanded {
                VStack(alignment: .leading, spacing: 8) {
                    if let error = log.error {
                        VStack(alignment: .leading, spacing: 4) {
                            Text("Error")
                                .font(.caption2.weight(.semibold))
                                .foregroundStyle(.red)
                            Text(error)
                                .font(.system(.caption, design: .monospaced))
                                .foregroundStyle(.red)
                                .textSelection(.enabled)
                        }
                    }
                    
                    if let content = log.content, !content.isEmpty {
                        VStack(alignment: .leading, spacing: 4) {
                            Text("Content")
                                .font(.caption2.weight(.semibold))
                                .foregroundStyle(.secondary)
                            ScrollView(.horizontal, showsIndicators: false) {
                                Text(content)
                                    .font(.system(.caption, design: .monospaced))
                                    .textSelection(.enabled)
                            }
                            .frame(maxHeight: 100)
                            .padding(8)
                            .background(Color(nsColor: .textBackgroundColor))
                            .cornerRadius(6)
                        }
                    }
                }
                .padding(.horizontal, 16)
                .padding(.bottom, 12)
                .padding(.leading, 28)
            }
        }
    }
    
    private func formatTime(_ date: Date) -> String {
        let formatter = DateFormatter()
        formatter.dateFormat = "HH:mm:ss"
        return formatter.string(from: date)
    }
}

// MARK: - Preview

#Preview {
    AgentsView()
}
