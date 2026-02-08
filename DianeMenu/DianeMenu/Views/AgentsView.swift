import SwiftUI
import AppKit

struct AgentsView: View {
    @State private var agents: [AgentConfig] = []
    @State private var logs: [AgentLog] = []
    @State private var isLoading = true
    @State private var error: String?
    @State private var selectedAgent: AgentConfig?
    @State private var testResults: [String: AgentTestResult] = [:]
    @State private var testPrompt = ""
    @State private var isRunningPrompt = false
    @State private var promptResult: AgentRunResult?
    
    private let client = DianeClient()
    
    var body: some View {
        VStack(spacing: 0) {
            headerView
            
            Divider()
            
            if isLoading {
                loadingView
            } else if let error = error {
                errorView(error)
            } else if agents.isEmpty {
                emptyView
            } else {
                HSplitView {
                    agentsListView
                        .frame(minWidth: 300, idealWidth: 400)
                    
                    detailView
                        .frame(minWidth: 350, idealWidth: 500)
                }
            }
        }
        .frame(minWidth: 750, idealWidth: 950, maxWidth: .infinity,
               minHeight: 450, idealHeight: 650, maxHeight: .infinity)
        .task {
            await loadData()
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
            
            // Refresh button
            Button {
                Task { await loadData() }
            } label: {
                Image(systemName: "arrow.clockwise")
            }
            .disabled(isLoading)
            
            // Stats
            let enabledCount = agents.filter { $0.enabled }.count
            Text("\(enabledCount)/\(agents.count) enabled")
                .font(.caption)
                .foregroundStyle(.secondary)
        }
        .padding()
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
                Task { await loadData() }
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
    
    // MARK: - Empty View
    
    private var emptyView: some View {
        VStack(spacing: 12) {
            Image(systemName: "person.3")
                .font(.largeTitle)
                .foregroundStyle(.secondary)
            Text("No agents configured")
                .font(.headline)
            Text("Use the gallery to add agents:")
                .font(.caption)
                .foregroundStyle(.secondary)
            Text("./scripts/acp-gallery.sh install gemini")
                .font(.system(.caption, design: .monospaced))
                .padding(8)
                .background(Color(nsColor: .textBackgroundColor))
                .cornerRadius(6)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
    
    // MARK: - Agents List
    
    private var agentsListView: some View {
        VStack(alignment: .leading, spacing: 0) {
            // Section header
            HStack {
                Image(systemName: "list.bullet")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                Text("Configured Agents")
                    .font(.subheadline.weight(.semibold))
                Spacer()
            }
            .padding(.horizontal)
            .padding(.vertical, 8)
            .background(Color(nsColor: .windowBackgroundColor))
            
            Divider()
            
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 0) {
                    ForEach(agents) { agent in
                        AgentRow(
                            agent: agent,
                            testResult: testResults[agent.name],
                            isSelected: selectedAgent?.name == agent.name,
                            onTest: {
                                Task { await testAgent(agent) }
                            },
                            onToggle: { enabled in
                                Task { await toggleAgent(agent, enabled: enabled) }
                            },
                            onSelect: {
                                selectedAgent = agent
                                promptResult = nil
                                Task { await loadLogs(forAgent: agent.name) }
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
            if let agent = selectedAgent {
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
                        if let result = testResults[agent.name] {
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
                }
                .padding()
                .background(Color(nsColor: .windowBackgroundColor))
                
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
                TextField("Enter a prompt...", text: $testPrompt)
                    .textFieldStyle(.roundedBorder)
                
                Button {
                    Task { await runPrompt(agent: agent) }
                } label: {
                    if isRunningPrompt {
                        ProgressView()
                            .scaleEffect(0.7)
                    } else {
                        Image(systemName: "paperplane.fill")
                    }
                }
                .disabled(testPrompt.isEmpty || isRunningPrompt || !agent.enabled)
                .buttonStyle(.borderedProminent)
            }
            
            // Result display
            if let result = promptResult {
                VStack(alignment: .leading, spacing: 4) {
                    if let error = result.error {
                        HStack(spacing: 4) {
                            Image(systemName: "xmark.circle.fill")
                                .foregroundStyle(.red)
                            Text("Error")
                                .font(.caption.weight(.semibold))
                        }
                        Text(error)
                            .font(.system(.caption, design: .monospaced))
                            .foregroundStyle(.red)
                    } else if let output = result.output {
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
                    Task { await loadLogs(forAgent: agent.name) }
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
            
            if logs.isEmpty {
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
                        ForEach(logs) { log in
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
    
    // MARK: - Actions
    
    private func loadData() async {
        isLoading = true
        error = nil
        
        do {
            agents = try await client.getAgents()
        } catch {
            self.error = error.localizedDescription
        }
        
        isLoading = false
    }
    
    private func loadLogs(forAgent agentName: String) async {
        do {
            logs = try await client.getAgentLogs(agentName: agentName, limit: 100)
        } catch {
            // Silently fail for log refresh
            logs = []
        }
    }
    
    private func testAgent(_ agent: AgentConfig) async {
        do {
            let result = try await client.testAgent(name: agent.name)
            testResults[agent.name] = result
        } catch {
            testResults[agent.name] = AgentTestResult(
                name: agent.name,
                url: agent.url,
                workdir: agent.workdir,
                enabled: agent.enabled,
                status: "error",
                error: error.localizedDescription,
                agentCount: nil,
                agents: nil
            )
        }
    }
    
    private func toggleAgent(_ agent: AgentConfig, enabled: Bool) async {
        do {
            try await client.toggleAgent(name: agent.name, enabled: enabled)
            agents = try await client.getAgents()
        } catch {
            // Show error somehow
        }
    }
    
    private func runPrompt(agent: AgentConfig) async {
        isRunningPrompt = true
        promptResult = nil
        
        do {
            promptResult = try await client.runAgentPrompt(agentName: agent.name, prompt: testPrompt)
        } catch {
            promptResult = AgentRunResult(runId: nil, status: "error", output: nil, error: error.localizedDescription)
        }
        
        isRunningPrompt = false
        
        // Refresh logs after running
        if let agentName = selectedAgent?.name {
            await loadLogs(forAgent: agentName)
        }
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
    
    @State private var isHovering = false
    @State private var isTesting = false
    
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
                
                // Test button
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
                        Image(systemName: "bolt.fill")
                            .font(.caption)
                    }
                }
                .buttonStyle(.plain)
                .help("Test connection")
                .disabled(!agent.enabled)
                
                // Toggle
                Toggle("", isOn: Binding(
                    get: { agent.enabled },
                    set: { onToggle($0) }
                ))
                .toggleStyle(.switch)
                .labelsHidden()
                .scaleEffect(0.8)
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
