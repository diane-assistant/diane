import Foundation
import Combine

// 3.1 Create EmergentAgentMonitoringViewModel to manage the real-time state of active agents
class EmergentAgentMonitoringViewModel: ObservableObject {
    @Published var selectedAgent: EmergentAgentConfig?
    @Published var logs: [String] = []
    
    private var agentService: EmergentAgentService
    private var cancellables = Set<AnyCancellable>()
    
    init(agentService: EmergentAgentService) {
        self.agentService = agentService
        
        // Listen for agent updates
        agentService.$agents
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
}
