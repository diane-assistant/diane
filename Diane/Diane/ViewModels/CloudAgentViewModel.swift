import Foundation
import SwiftUI
import os.log

@MainActor
@Observable
final class CloudAgentViewModel {
    var agent: AgentConfig
    let client: DianeClientProtocol
    
    // Runs
    var runs: [EmergentAgentRunDTO] = []
    var isLoadingRuns = false
    var triggerResult: EmergentTriggerResponseDTO?
    
    // Pending Events
    var pendingEventsResponse: EmergentPendingEventsResponseDTO?
    var isLoadingEvents = false
    var batchTriggerResult: EmergentBatchTriggerResponseDTO?
    
    var error: String?
    
    init(agent: AgentConfig, client: DianeClientProtocol = DianeClient.shared) {
        self.agent = agent
        self.client = client
    }
    
    func refresh() async {
        guard let id = agent.cloudId else { return }
        await loadRuns(for: id)
        if agent.triggerType == "reaction" {
            await loadPendingEvents(for: id)
        }
    }
    
    func triggerRun() async {
        guard let id = agent.cloudId else { return }
        do {
            triggerResult = try await client.triggerRun(agentId: id)
            await loadRuns(for: id)
        } catch {
            self.error = error.localizedDescription
        }
    }
    
    func cancelRun(runId: String) async {
        guard let id = agent.cloudId else { return }
        do {
            try await client.cancelRun(agentId: id, runId: runId)
            await loadRuns(for: id)
        } catch {
            self.error = error.localizedDescription
        }
    }
    
    func loadRuns(for agentId: String) async {
        isLoadingRuns = true
        do {
            runs = try await client.getAgentRuns(agentId: agentId, limit: 10)
        } catch {
            Logger(subsystem: "com.diane.mac", category: "CloudAgentViewModel").error("Failed to load runs: \(error)")
        }
        isLoadingRuns = false
    }
    
    func loadPendingEvents(for agentId: String) async {
        isLoadingEvents = true
        do {
            pendingEventsResponse = try await client.getPendingEvents(agentId: agentId, limit: 100)
        } catch {
            Logger(subsystem: "com.diane.mac", category: "CloudAgentViewModel").error("Failed to load pending events: \(error)")
        }
        isLoadingEvents = false
    }
    
    func batchTrigger(objectIds: [String]) async {
        guard let id = agent.cloudId else { return }
        do {
            batchTriggerResult = try await client.batchTrigger(agentId: id, objectIds: objectIds)
            await loadPendingEvents(for: id)
            await loadRuns(for: id)
        } catch {
            self.error = error.localizedDescription
        }
    }
}
