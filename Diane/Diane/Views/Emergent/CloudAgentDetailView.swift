import SwiftUI

struct CloudAgentDetailView: View {
    @State private var viewModel: CloudAgentViewModel
    let agent: AgentConfig
    
    init(agent: AgentConfig) {
        self.agent = agent
        _viewModel = State(initialValue: CloudAgentViewModel(agent: agent))
    }
    
    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                if let err = viewModel.error {
                    Text(err)
                        .font(.caption)
                        .foregroundStyle(.red)
                        .padding()
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .background(Color.red.opacity(0.1))
                }
                
                if let result = viewModel.triggerResult {
                    HStack {
                        Text(result.success ? "Triggered Run: \(result.runId ?? "")" : "Trigger Failed: \(result.error ?? "")")
                            .font(.caption)
                        Spacer()
                    }
                    .padding()
                    .background(result.success ? Color.green.opacity(0.1) : Color.red.opacity(0.1))
                }
                
                // Trigger Actions
                HStack(spacing: 12) {
                    Button {
                        Task { await viewModel.triggerRun() }
                    } label: {
                        Label("Run Now", systemImage: "play.fill")
                    }
                    .buttonStyle(.borderedProminent)
                    
                    if viewModel.agent.triggerType == "reaction" {
                        Button {
                            Task { await viewModel.refresh() }
                        } label: {
                            Label("Refresh", systemImage: "arrow.clockwise")
                        }
                    }
                }
                .padding(.horizontal)
                
                // Content based on trigger type
                if viewModel.agent.triggerType == "reaction" {
                    pendingEventsSection
                }
                
                runsSection
            }
            .padding(.vertical)
        }
        .task {
            await viewModel.refresh()
        }
    }
    
    private var pendingEventsSection: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text("Pending Events")
                    .font(.headline)
                if viewModel.isLoadingEvents {
                    ProgressView().controlSize(.small)
                }
                Spacer()
                if let count = viewModel.pendingEventsResponse?.objects?.count, count > 0 {
                    Button("Process All (\(count))") {
                        Task {
                            let ids = viewModel.pendingEventsResponse!.objects!.compactMap { $0.id }
                            await viewModel.batchTrigger(objectIds: ids)
                        }
                    }
                }
            }
            .padding(.horizontal)
            
            if let events = viewModel.pendingEventsResponse?.objects, !events.isEmpty {
                ForEach(events, id: \.id) { event in
                    HStack {
                        VStack(alignment: .leading) {
                            Text(event.type)
                                .font(.subheadline).bold()
                            Text("Object ID: \(event.id)")
                                .font(.caption).foregroundColor(.secondary)
                        }
                        Spacer()
                    }
                    .padding()
                    .background(Color(NSColor.controlBackgroundColor))
                    .cornerRadius(8)
                    .padding(.horizontal)
                }
            } else {
                Text("No pending events")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .padding(.horizontal)
            }
        }
    }
    
    private var runsSection: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text("Recent Runs")
                    .font(.headline)
                if viewModel.isLoadingRuns {
                    ProgressView().controlSize(.small)
                }
            }
            .padding(.horizontal)
            
            if viewModel.runs.isEmpty {
                Text("No recent runs")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .padding(.horizontal)
            } else {
                ForEach(viewModel.runs, id: \.id) { run in
                    HStack {
                        VStack(alignment: .leading, spacing: 4) {
                            Text(run.id)
                                .font(.system(.caption, design: .monospaced))
                            HStack {
                                Text(run.status.rawValue.capitalized)
                                    .font(.caption2).bold()
                                    .foregroundColor(statusColor(run.status))
                                if let startedAt = run.startedAt {
                                    Text(startedAt)
                                        .font(.caption2)
                                        .foregroundColor(.secondary)
                                }
                            }
                        }
                        Spacer()
                        if run.status == .running {
                            Button("Cancel") {
                                Task { await viewModel.cancelRun(runId: run.id) }
                            }
                            .controlSize(.small)
                        }
                    }
                    .padding()
                    .background(Color(NSColor.controlBackgroundColor))
                    .cornerRadius(8)
                    .padding(.horizontal)
                }
            }
        }
    }
    
    private func statusColor(_ status: EmergentAgentRunStatus) -> Color {
        switch status {
        case .success: return .green
        case .error: return .red
        case .running: return .blue
        case .paused: return .orange
        case .cancelled: return .gray
        default: return .secondary
        }
    }
}
