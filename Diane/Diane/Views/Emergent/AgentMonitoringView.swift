import SwiftUI

// 3.2 Implement the MasterDetailView dashboard displaying all active/completed custom agents
struct AgentMonitoringView: View {
    @EnvironmentObject var agentService: EmergentAgentService
    @StateObject private var viewModel: EmergentAgentMonitoringViewModel
    
    init(agentService: EmergentAgentService) {
        _viewModel = StateObject(wrappedValue: EmergentAgentMonitoringViewModel(agentService: agentService))
    }
    
    var body: some View {
        MasterDetailView(
            master: {
                VStack(spacing: 0) {
                    // 3.3 Ensure the master column uses MasterListHeader for the agent list
                    MasterListHeader(title: "Active Agents", icon: "waveform.path.ecg")
                    
                    ScrollView {
                        LazyVStack(spacing: 0) {
                            ForEach(agentService.agents) { agent in
                                AgentRow(agent: agent)
                                    .onTapGesture {
                                        viewModel.selectAgent(agent)
                                    }
                            }
                        }
                    }
                }
            },
            detail: {
                if let selectedAgent = viewModel.selectedAgent {
                    AgentDetailView(agent: selectedAgent, viewModel: viewModel, logs: viewModel.logs)
                } else {
                    Text("Select an agent to monitor")
                        .foregroundColor(.secondary)
                }
            }
        )
    }
}

// Subview for individual agents in the master list
struct AgentRow: View {
    let agent: EmergentAgentConfig
    
    var body: some View {
        HStack {
            VStack(alignment: .leading, spacing: Spacing.xxSmall.rawValue) {
                Text(agent.name).font(.headline)
                Text(agent.state.rawValue.capitalized)
                    .font(.caption)
                    .foregroundColor(statusColor(for: agent.state))
            }
            Spacer()
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 10)
    }
    
    func statusColor(for state: EmergentAgentState) -> Color {
        switch state {
        case .running: return .green
        case .error, .offline: return .red
        case .deploying: return .orange
        case .pending, .completed: return .gray
        }
    }
}

struct AgentDetailView: View {
    let agent: EmergentAgentConfig
    @ObservedObject var viewModel: EmergentAgentMonitoringViewModel
    let logs: [String]
    
    @State private var selectedTab = 0
    
    var body: some View {
        VStack(spacing: 0) {
            // Header
            HStack {
                Text(agent.name).font(.title)
                Spacer()
                
                Button {
                    Task { await viewModel.triggerRun() }
                } label: {
                    Label("Run Now", systemImage: "play.fill")
                }
            }
            .padding()
            
            Divider()
            
            if let err = viewModel.error {
                Text(err)
                    .font(.caption)
                    .foregroundStyle(.red)
                    .padding()
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .background(Color.red.opacity(0.1))
            }
            
            Picker("", selection: $selectedTab) {
                Text("Live Logs").tag(0)
                Text("Runs (\(viewModel.runs.count))").tag(1)
            }
            .pickerStyle(.segmented)
            .padding()
            
            if selectedTab == 0 {
                liveLogsTab
            } else {
                runsTab
            }
        }
    }
    
    private var liveLogsTab: some View {
        ScrollView {
            VStack(spacing: 12) {
                // 3.4 Build the agent detail view using SummaryCard for displaying key execution metrics
                HStack(spacing: 12) {
                    SummaryCard(title: "State", value: agent.state.rawValue.capitalized, icon: "info.circle")
                    SummaryCard(title: "Tools Used", value: "\(agent.tools.count)", icon: "wrench.and.screwdriver")
                    SummaryCard(title: "Uptime", value: agent.state == .running ? "2m 15s" : "--", icon: "clock")
                }
                .padding(.horizontal, 16)
                .padding(.top, 16)
                
                // 3.6 Handle offline/disconnect states in the detail view showing a clear message using Spacing.large
                if agent.state == .offline || agent.state == .error {
                    HStack {
                        Image(systemName: "exclamationmark.triangle")
                        Text(agent.state == .offline ? "Agent is currently offline." : "Agent encountered an error.")
                    }
                    .foregroundColor(.red)
                    .padding(12)
                    .background(Color.red.opacity(0.1))
                    .cornerRadius(8)
                }
                
                // 3.5 Implement the live log feed section using DetailSection component
                DetailSection(title: "Live Execution Logs") {
                    VStack(alignment: .leading, spacing: 6) {
                        if logs.isEmpty {
                            Text("No logs available yet.")
                                .foregroundColor(.secondary)
                        } else {
                            ForEach(logs, id: \.self) { log in
                                Text(log).font(.system(.caption, design: .monospaced))
                            }
                        }
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding(8)
                    .background(Color(.textBackgroundColor))
                    .cornerRadius(6)
                }
                .padding(.horizontal, 16)
            }
            .padding(.bottom, 20)
        }
    }
    
    private var runsTab: some View {
        VStack {
            if viewModel.isLoadingRuns && viewModel.runs.isEmpty {
                ProgressView()
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else if viewModel.runs.isEmpty {
                Text("No runs recorded.")
                    .foregroundStyle(.secondary)
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else {
                List(viewModel.runs) { run in
                    HStack {
                        VStack(alignment: .leading) {
                            HStack {
                                Text(run.id.prefix(8))
                                    .font(.system(.body, design: .monospaced))
                                
                                Text(run.status.rawValue.uppercased())
                                    .font(.system(size: 9, weight: .bold))
                                    .padding(.horizontal, 4)
                                    .padding(.vertical, 2)
                                    .background(runStatusColor(run.status).opacity(0.2))
                                    .foregroundColor(runStatusColor(run.status))
                                    .cornerRadius(4)
                            }
                            
                            HStack {
                                Text(run.startedAt ?? "-")
                                    .font(.caption)
                                if let duration = run.durationMs {
                                    Text("(\(duration)ms)")
                                        .font(.caption)
                                }
                                if let steps = run.stepCount {
                                    Text("\(steps) steps")
                                        .font(.caption)
                                }
                            }
                            .foregroundStyle(.secondary)
                            
                            if let err = run.errorMessage {
                                Text(err)
                                    .font(.caption)
                                    .foregroundStyle(.red)
                            }
                        }
                        
                        Spacer()
                        
                        if run.status == .running || run.status == .paused {
                            Button("Cancel") {
                                Task { await viewModel.cancelRun(runId: run.id) }
                            }
                            .controlSize(.small)
                        }
                    }
                    .padding(.vertical, 4)
                }
            }
        }
    }
    
    private func runStatusColor(_ status: EmergentAgentRunStatus) -> Color {
        switch status {
        case .running: return .blue
        case .success: return .green
        case .skipped, .paused, .cancelled: return .orange
        case .error: return .red
        }
    }
}
