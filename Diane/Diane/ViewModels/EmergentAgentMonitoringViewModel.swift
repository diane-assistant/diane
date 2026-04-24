import Foundation
import Combine
import os.log

// 3.1 Create EmergentAgentMonitoringViewModel to manage the real-time state of active agents
@MainActor
class EmergentAgentMonitoringViewModel: ObservableObject {
    @Published var selectedAgent: EmergentAgentConfig? {
        didSet {
            if let agent = selectedAgent {
                Task { await loadRuns(for: agent.id) }
            } else {
                runs = []
            }
        }
    }
    @Published var logs: [String] = []
    
    // Run history
    @Published var runs: [EmergentAgentRunDTO] = []
    @Published var isLoadingRuns = false
    @Published var error: String?
    
    private var agentService: EmergentAgentService
    private var cancellables = Set<AnyCancellable>()
    private var autoRefreshTimer: Timer?
    
    init(agentService: EmergentAgentService) {
        self.agentService = agentService
        
        // Listen for agent updates
        agentService.$agents
            .receive(on: RunLoop.main)
            .sink { [weak self] agents in
                if let selected = self?.selectedAgent {
                    self?.selectedAgent = agents.first(where: { $0.id == selected.id })
                }
            }
            .store(in: &cancellables)
    }
    
    func selectAgent(_ agent: EmergentAgentConfig) {
        self.selectedAgent = agent
        self.logs.removeAll()
        // Mock streaming logs
        self.logs.append("Agent \(agent.name) monitoring started...")
        agentService.startMonitoring(id: agent.id)
    }
    
    func loadRuns(for agentId: String) async {
        isLoadingRuns = true
        do {
            runs = try await EmergentAdminClient.shared.getRuns(agentId: agentId, limit: 10)
        } catch {
            Logger(subsystem: "com.diane.mac", category: "EmergentAgentMonitoringViewModel").error("Failed to load runs: \(error)")
        }
        isLoadingRuns = false
        
        let hasRunning = runs.contains(where: { $0.status == .running })
        if hasRunning {
            startAutoRefresh()
        } else {
            stopAutoRefresh()
        }
    }
    
    func triggerRun() async {
        guard let id = selectedAgent?.id else { return }
        do {
            _ = try await EmergentAdminClient.shared.triggerRun(agentId: id)
            await loadRuns(for: id)
            logs.append("Manually triggered run.")
        } catch {
            self.error = error.localizedDescription
            logs.append("Trigger failed: \(error.localizedDescription)")
        }
    }
    
    func cancelRun(runId: String) async {
        guard let id = selectedAgent?.id else { return }
        do {
            try await EmergentAdminClient.shared.cancelRun(agentId: id, runId: runId)
            await loadRuns(for: id)
            logs.append("Cancelled run \(runId).")
        } catch {
            self.error = error.localizedDescription
            logs.append("Cancel failed: \(error.localizedDescription)")
        }
    }
    
    private func startAutoRefresh() {
        guard autoRefreshTimer == nil, let id = selectedAgent?.id else { return }
        autoRefreshTimer = Timer.scheduledTimer(withTimeInterval: 5.0, repeats: true) { [weak self] _ in
            Task { @MainActor in
                await self?.loadRuns(for: id)
            }
        }
    }
    
    private func stopAutoRefresh() {
        autoRefreshTimer?.invalidate()
        autoRefreshTimer = nil
    }
}
