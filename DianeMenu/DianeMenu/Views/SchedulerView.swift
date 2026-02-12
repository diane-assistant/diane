import SwiftUI
import AppKit

struct SchedulerView: View {
    @State private var viewModel: SchedulerViewModel
    
    init(viewModel: SchedulerViewModel = SchedulerViewModel()) {
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
            } else if viewModel.jobs.isEmpty {
                emptyView
            } else {
                HSplitView {
                    jobsListView
                        .frame(minWidth: 300, idealWidth: 400)
                    
                    logsView
                        .frame(minWidth: 300, idealWidth: 400)
                }
            }
        }
        .frame(minWidth: 700, idealWidth: 900, maxWidth: .infinity,
               minHeight: 400, idealHeight: 600, maxHeight: .infinity)
        .task {
            await viewModel.loadData()
        }
    }
    
    // MARK: - Header
    
    private var headerView: some View {
        HStack(spacing: 12) {
            Image(systemName: "clock")
                .foregroundStyle(.secondary)
            
            Text("Scheduler")
                .font(.headline)
            
            Spacer()
            
            // Search field
            HStack(spacing: 6) {
                Image(systemName: "magnifyingglass")
                    .foregroundStyle(.secondary)
                TextField("Search jobs...", text: $viewModel.searchText)
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
            
            // Refresh button
            Button {
                Task { await viewModel.loadData() }
            } label: {
                Image(systemName: "arrow.clockwise")
            }
            .disabled(viewModel.isLoading)
            
            // Stats
            Text("\(viewModel.jobs.count) jobs")
                .font(.caption)
                .foregroundStyle(.secondary)
        }
        .padding()
    }
    
    // MARK: - Loading View
    
    private var loadingView: some View {
        VStack(spacing: 12) {
            ProgressView()
            Text("Loading scheduler...")
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
            Text("Failed to load scheduler")
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
            icon: "calendar.badge.clock",
            title: "No scheduled jobs",
            description: "Jobs can be created using the job_add tool"
        )
    }
    
    // MARK: - Jobs List
    
    private var jobsListView: some View {
        VStack(alignment: .leading, spacing: 0) {
            // Section header
            HStack {
                Image(systemName: "calendar.badge.clock")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                Text("Scheduled Jobs")
                    .font(.subheadline.weight(.semibold))
                Spacer()
            }
            .padding(.horizontal)
            .padding(.vertical, 8)
            .background(Color(nsColor: .windowBackgroundColor))
            
            Divider()
            
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 0) {
                    ForEach(viewModel.filteredJobs) { job in
                        JobRow(
                            job: job,
                            lastExecution: viewModel.executions.first { $0.jobID == job.id },
                            isSelected: viewModel.selectedJob?.id == job.id,
                            onToggle: { enabled in
                                Task { await viewModel.toggleJob(job, enabled: enabled) }
                            },
                            onSelect: {
                                viewModel.selectJob(job)
                                Task { await viewModel.loadLogs(forJob: job.name) }
                            }
                        )
                        Divider()
                            .padding(.leading, 16)
                    }
                }
            }
        }
    }
    
    // MARK: - Logs View
    
    private var logsView: some View {
        VStack(alignment: .leading, spacing: 0) {
            // Section header
            HStack {
                Image(systemName: "doc.text")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                if let jobName = viewModel.showLogsForJob {
                    Text("Logs: \(jobName)")
                        .font(.subheadline.weight(.semibold))
                } else {
                    Text("Execution Logs")
                        .font(.subheadline.weight(.semibold))
                }
                Spacer()
                
                if viewModel.showLogsForJob != nil {
                    Button {
                        viewModel.showAllLogs()
                        Task { await viewModel.loadLogs(forJob: nil) }
                    } label: {
                        Text("Show All")
                            .font(.caption)
                    }
                    .buttonStyle(.plain)
                }
            }
            .padding(.horizontal)
            .padding(.vertical, 8)
            .background(Color(nsColor: .windowBackgroundColor))
            
            Divider()
            
            if viewModel.filteredExecutions.isEmpty {
                VStack(spacing: 8) {
                    Image(systemName: "doc.text.magnifyingglass")
                        .font(.title)
                        .foregroundStyle(.secondary)
                    Text("No execution logs")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else {
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 0) {
                        ForEach(viewModel.filteredExecutions) { execution in
                            ExecutionRow(execution: execution)
                            Divider()
                                .padding(.leading, 16)
                        }
                    }
                }
            }
        }
    }
}

// MARK: - Job Row

struct JobRow: View {
    let job: Job
    let lastExecution: JobExecution?
    let isSelected: Bool
    let onToggle: (Bool) -> Void
    let onSelect: () -> Void
    
    @State private var isHovering = false
    
    var body: some View {
        Button(action: onSelect) {
            HStack(spacing: 12) {
                // Status indicator
                Circle()
                    .fill(statusColor)
                    .frame(width: 8, height: 8)
                
                VStack(alignment: .leading, spacing: 4) {
                    // Job name
                    HStack(spacing: 8) {
                        Text(job.name)
                            .font(.system(.body, design: .monospaced))
                            .fontWeight(.medium)
                        
                        // Action type badge
                        if job.isAgentAction {
                            HStack(spacing: 2) {
                                Image(systemName: "person.fill")
                                    .font(.caption2)
                                Text(job.agentName ?? "agent")
                                    .font(.caption2)
                            }
                            .foregroundStyle(.blue)
                            .padding(.horizontal, 4)
                            .padding(.vertical, 2)
                            .background(Color.blue.opacity(0.15))
                            .cornerRadius(4)
                        }
                        
                        if !job.enabled {
                            Text("disabled")
                                .font(.caption2)
                                .foregroundStyle(.secondary)
                                .padding(.horizontal, 4)
                                .padding(.vertical, 2)
                                .background(Color.secondary.opacity(0.15))
                                .cornerRadius(4)
                        }
                    }
                    
                    // Schedule
                    HStack(spacing: 4) {
                        Image(systemName: "clock")
                            .font(.caption2)
                        Text(job.scheduleDescription)
                            .font(.caption)
                    }
                    .foregroundStyle(.secondary)
                    
                    // Last execution info
                    if let lastExec = lastExecution {
                        HStack(spacing: 4) {
                            Image(systemName: lastExec.isSuccess ? "checkmark.circle.fill" : "xmark.circle.fill")
                                .font(.caption2)
                                .foregroundStyle(lastExec.isSuccess ? .green : .red)
                            Text("Last: \(formatRelativeTime(lastExec.startedAt))")
                                .font(.caption)
                            if let exitCode = lastExec.exitCode {
                                Text("(\(exitCode))")
                                    .font(.caption.monospaced())
                            }
                            Text("• \(lastExec.durationString)")
                                .font(.caption)
                        }
                        .foregroundStyle(.secondary)
                    }
                }
                
                Spacer()
                
                // Toggle
                Toggle("", isOn: Binding(
                    get: { job.enabled },
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
        if !job.enabled {
            return .secondary
        }
        if let lastExec = lastExecution {
            return lastExec.isSuccess ? .green : .red
        }
        return .blue
    }
    
    private func formatRelativeTime(_ date: Date) -> String {
        let formatter = RelativeDateTimeFormatter()
        formatter.unitsStyle = .abbreviated
        return formatter.localizedString(for: date, relativeTo: Date())
    }
}

// MARK: - Execution Row

struct ExecutionRow: View {
    let execution: JobExecution
    @State private var isExpanded = false
    @State private var isHovering = false
    
    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            // Main row
            Button {
                withAnimation(.easeInOut(duration: 0.2)) {
                    isExpanded.toggle()
                }
            } label: {
                HStack(spacing: 12) {
                    // Status icon
                    Image(systemName: execution.isSuccess ? "checkmark.circle.fill" : "xmark.circle.fill")
                        .font(.caption)
                        .foregroundStyle(execution.isSuccess ? .green : .red)
                    
                    // Timestamp
                    Text(formatTime(execution.startedAt))
                        .font(.caption.monospaced())
                        .foregroundStyle(.secondary)
                    
                    // Job name
                    if let jobName = execution.jobName {
                        Text(jobName)
                            .font(.caption.weight(.medium))
                    }
                    
                    Spacer()
                    
                    // Exit code
                    if let exitCode = execution.exitCode {
                        Text("exit \(exitCode)")
                            .font(.caption.monospaced())
                            .foregroundStyle(exitCode == 0 ? Color.secondary : Color.red)
                    }
                    
                    // Duration
                    Text(execution.durationString)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    
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
                    if let error = execution.error {
                        logSection(title: "Error", content: error, color: .red)
                    }
                    
                    if let stdout = execution.stdout, !stdout.isEmpty {
                        logSection(title: "stdout", content: stdout, color: .primary)
                    }
                    
                    if let stderr = execution.stderr, !stderr.isEmpty {
                        logSection(title: "stderr", content: stderr, color: .orange)
                    }
                    
                    if execution.error == nil && (execution.stdout?.isEmpty ?? true) && (execution.stderr?.isEmpty ?? true) {
                        Text("No output")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                            .italic()
                    }
                }
                .padding(.horizontal, 16)
                .padding(.bottom, 12)
                .padding(.leading, 28)
            }
        }
    }
    
    private func logSection(title: String, content: String, color: Color) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(title)
                .font(.caption2.weight(.semibold))
                .foregroundStyle(color.opacity(0.8))
            
            ScrollView(.horizontal, showsIndicators: false) {
                Text(content)
                    .font(.system(.caption, design: .monospaced))
                    .foregroundStyle(color)
                    .textSelection(.enabled)
            }
            .frame(maxHeight: 100)
            .padding(8)
            .background(Color(nsColor: .textBackgroundColor))
            .cornerRadius(6)
        }
    }
    
    private func formatTime(_ date: Date) -> String {
        let formatter = DateFormatter()
        formatter.dateFormat = "yyyy-MM-dd HH:mm:ss"
        return formatter.string(from: date)
    }
}

// MARK: - Preview

#Preview {
    SchedulerView()
}
