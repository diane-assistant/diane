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
                    AgentDetailView(agent: selectedAgent, logs: viewModel.logs)
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
    let logs: [String]
    
    var body: some View {
        VStack(spacing: 0) {
            // Header
            HStack {
                Text(agent.name).font(.title)
                Spacer()
                // Status badge
            }
            .padding(Padding.standard.rawValue)
            
            Divider()
            
            ScrollView {
                VStack(spacing: Spacing.large.rawValue) {
                    // 3.4 Build the agent detail view using SummaryCard for displaying key execution metrics
                    HStack(spacing: Spacing.large.rawValue) {
                        SummaryCard(title: "State", value: agent.state.rawValue.capitalized, icon: "info.circle")
                        SummaryCard(title: "Tools Used", value: "\(agent.tools.count)", icon: "wrench.and.screwdriver")
                        SummaryCard(title: "Uptime", value: agent.state == .running ? "2m 15s" : "--", icon: "clock")
                    }
                    .padding(.horizontal, Padding.standard.rawValue)
                    .padding(.top, Padding.standard.rawValue)
                    
                    // 3.6 Handle offline/disconnect states in the detail view showing a clear message using Spacing.large
                    if agent.state == .offline || agent.state == .error {
                        HStack {
                            Image(systemName: "exclamationmark.triangle")
                            Text(agent.state == .offline ? "Agent is currently offline." : "Agent encountered an error.")
                        }
                        .foregroundColor(.red)
                        .padding(Spacing.large.rawValue)
                        .background(Color.red.opacity(0.1))
                        .cornerRadius(CornerRadius.standard.rawValue)
                    }
                    
                    // 3.5 Implement the live log feed section using DetailSection component
                    DetailSection(title: "Live Execution Logs") {
                        VStack(alignment: .leading, spacing: Spacing.small.rawValue) {
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
                        .padding(Padding.small.rawValue)
                        .background(Color(.textBackgroundColor))
                        .cornerRadius(CornerRadius.medium.rawValue)
                    }
                    .padding(.horizontal, Padding.standard.rawValue)
                }
                .padding(.bottom, Padding.large.rawValue)
            }
        }
    }
}
