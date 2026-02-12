import Foundation
import Observation

/// ViewModel for AgentsView that owns all agent state and business logic.
///
/// Accepts `DianeClientProtocol` via its initializer so tests can inject a mock.
/// Uses the `@Observable` macro (requires macOS 14+/iOS 17+) so SwiftUI views
/// automatically track property changes without explicit `@Published` wrappers.
@MainActor
@Observable
final class AgentsViewModel {

    // MARK: - Dependencies

    @ObservationIgnored
    let client: DianeClientProtocol

    // MARK: - Agent List State

    var agents: [AgentConfig] = []
    var logs: [AgentLog] = []
    var isLoading = true
    var error: String?
    var selectedAgent: AgentConfig?
    var testResults: [String: AgentTestResult] = [:]

    // MARK: - Test Prompt State

    var testPrompt = ""
    var isRunningPrompt = false
    var promptResult: AgentRunResult?

    // MARK: - Remote Agents State

    var remoteAgents: [RemoteAgentInfo] = []
    var isLoadingRemoteAgents = false
    var remoteAgentsError: String?
    var selectedSubAgent: String = ""
    var isSavingSubAgent = false

    // MARK: - Gallery State

    var showAddAgent = false
    var galleryEntries: [GalleryEntry] = []
    var isLoadingGallery = false
    var selectedGalleryEntry: GalleryEntry?
    var newAgentName = ""
    var newAgentWorkdir = ""
    var newAgentPort = ""
    var isInstalling = false
    var installError: String?

    // MARK: - Init

    init(client: DianeClientProtocol = DianeClient()) {
        self.client = client
    }

    // MARK: - Data Operations

    func loadData() async {
        isLoading = true
        error = nil

        do {
            agents = try await client.getAgents()

            // Auto-test enabled agents in the background
            for agent in agents where agent.enabled {
                Task {
                    await testAgent(agent)
                }
            }
        } catch {
            self.error = error.localizedDescription
        }

        isLoading = false
    }

    func loadLogs(forAgent agentName: String) async {
        do {
            logs = try await client.getAgentLogs(agentName: agentName, limit: 100)
        } catch {
            // Silently fail for log refresh
            logs = []
        }
    }

    func testAgent(_ agent: AgentConfig) async {
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
                version: nil,
                agentCount: nil,
                agents: nil
            )
        }
    }

    func toggleAgent(_ agent: AgentConfig, enabled: Bool) async {
        do {
            try await client.toggleAgent(name: agent.name, enabled: enabled)
            agents = try await client.getAgents()
        } catch {
            // Show error somehow
        }
    }

    func loadRemoteAgents(for agent: AgentConfig) async {
        isLoadingRemoteAgents = true
        remoteAgentsError = nil

        do {
            remoteAgents = try await client.getRemoteAgents(agentName: agent.name)
            if remoteAgents.isEmpty {
                remoteAgentsError = "No configurable sub-agents found"
            }
        } catch {
            remoteAgentsError = error.localizedDescription
            remoteAgents = []
        }

        isLoadingRemoteAgents = false
    }

    func saveSubAgent(for agent: AgentConfig) async {
        isSavingSubAgent = true

        do {
            try await client.updateAgent(
                name: agent.name,
                subAgent: selectedSubAgent,
                enabled: nil,
                description: nil,
                workdir: nil
            )
            // Refresh agents list to get updated config
            agents = try await client.getAgents()
            // Update selected agent if it's the same one
            if let updated = agents.first(where: { $0.name == agent.name }) {
                selectedAgent = updated
            }
        } catch {
            remoteAgentsError = "Failed to save: \(error.localizedDescription)"
        }

        isSavingSubAgent = false
    }

    func runPrompt(agent: AgentConfig) async {
        isRunningPrompt = true
        promptResult = nil

        do {
            promptResult = try await client.runAgentPrompt(
                agentName: agent.name,
                prompt: testPrompt,
                remoteAgentName: nil
            )
        } catch {
            promptResult = AgentRunResult(
                agentName: agent.name,
                sessionId: nil,
                runId: UUID().uuidString,
                status: "failed",
                awaitRequest: nil,
                output: [],
                error: AgentError(code: "client_error", message: error.localizedDescription, data: nil),
                createdAt: Date(),
                finishedAt: Date()
            )
        }

        isRunningPrompt = false

        // Refresh logs after running
        if let agentName = selectedAgent?.name {
            await loadLogs(forAgent: agentName)
        }
    }

    // MARK: - Gallery Methods

    func loadGallery() async {
        isLoadingGallery = true

        do {
            galleryEntries = try await client.getGallery(featured: false)
        } catch {
            galleryEntries = []
        }

        isLoadingGallery = false
    }

    func installAgent() async {
        guard let entry = selectedGalleryEntry else { return }

        isInstalling = true
        installError = nil

        do {
            let name = newAgentName.isEmpty ? nil : newAgentName
            let workdir = newAgentWorkdir.isEmpty ? nil : newAgentWorkdir
            let port = Int(newAgentPort)

            let result = try await client.installGalleryAgent(
                id: entry.id,
                name: name,
                workdir: workdir,
                port: port
            )

            // Refresh agents list
            agents = try await client.getAgents()

            // Auto-test the newly installed agent
            let agentName = result.agent
            Task {
                if let agent = agents.first(where: { $0.name == agentName }) {
                    await testAgent(agent)
                }
            }

            // Reset and close sheet
            resetInstallForm()
            showAddAgent = false
        } catch {
            installError = error.localizedDescription
        }

        isInstalling = false
    }

    func removeAgent(name: String) async {
        do {
            try await client.removeAgent(name: name)
            agents = try await client.getAgents()

            // Clear selection if removed agent was selected
            if selectedAgent?.name == name {
                selectedAgent = nil
            }
        } catch {
            // TODO: Show error
        }
    }

    // MARK: - Helpers

    func resetInstallForm() {
        selectedGalleryEntry = nil
        newAgentName = ""
        newAgentWorkdir = ""
        newAgentPort = ""
        installError = nil
    }

    func onSelectAgent(_ agent: AgentConfig) {
        selectedAgent = agent
        promptResult = nil
    }

    func onSelectedAgentChanged() {
        remoteAgents = []
        remoteAgentsError = nil
        if let agent = selectedAgent {
            selectedSubAgent = agent.subAgent ?? ""
        }
    }
}
