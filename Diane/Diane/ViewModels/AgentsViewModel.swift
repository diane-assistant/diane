import Foundation
import Observation
import os.log

private let logger = Logger(subsystem: "com.diane.Diane", category: "Agents")

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

    var showGallerySheet = false
    var newAgentURL = ""
    var newAgentDescription = ""
    
    // Workspace Config State
    var newAgentBaseImage = ""
    var newAgentRepoURL = ""
    var newAgentRepoBranch = ""
    var newAgentProvider = ""
    var newAgentSetupCommands = [String]()
    
    var galleryEntries: [GalleryEntry] = []
    var isLoadingGallery = false
    var selectedGalleryEntry: GalleryEntry?
    var newAgentName = ""
    var newAgentWorkdir = ""
    var newAgentPort = ""
    var isInstalling = false
    var installError: String?


    // MARK: - Edit Agent State
    var showEditAgent = false
    var editAgentDescription = ""
    var editAgentWorkdir = ""
    var isEditing = false
    var editError: String?

    // MARK: - Init

    init(client: DianeClientProtocol = DianeClient.shared) {
        self.client = client
    }

    // MARK: - Data Operations

    func loadData() async {
        isLoading = true
        error = nil
        FileLogger.shared.info("Loading agents data...", category: "Agents")

        do {
            agents = try await client.getAgents()
            FileLogger.shared.info("Loaded \(agents.count) agents", category: "Agents")

            // Auto-test enabled agents in the background
            for agent in agents where agent.enabled {
                Task {
                    await testAgent(agent)
                }
            }
        } catch {
            self.error = error.localizedDescription
            FileLogger.shared.error("Failed to load agents: \(error.localizedDescription)", category: "Agents")
        }

        isLoading = false
    }

    func loadLogs(forAgent agentName: String) async {
        FileLogger.shared.info("Loading logs for agent '\(agentName)'", category: "Agents")
        do {
            logs = try await client.getAgentLogs(agentName: agentName, limit: 100)
        } catch {
            // Silently fail for log refresh
            FileLogger.shared.error("Failed to load logs for agent '\(agentName)': \(error.localizedDescription)", category: "Agents")
            logs = []
        }
    }

    func testAgent(_ agent: AgentConfig) async {
        FileLogger.shared.info("Testing agent '\(agent.name)'", category: "Agents")
        do {
            let result = try await client.testAgent(name: agent.name)
            testResults[agent.name] = result
            FileLogger.shared.info("Agent '\(agent.name)' test result: \(result.status)", category: "Agents")
        } catch {
            FileLogger.shared.error("Failed to test agent '\(agent.name)': \(error.localizedDescription)", category: "Agents")
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
        FileLogger.shared.info("Toggling agent '\(agent.name)' enabled=\(enabled)", category: "Agents")
        do {
            try await client.toggleAgent(name: agent.name, enabled: enabled)
            agents = try await client.getAgents()
            FileLogger.shared.info("Toggled agent '\(agent.name)' successfully", category: "Agents")
        } catch {
            // Show error somehow
            FileLogger.shared.error("Failed to toggle agent '\(agent.name)': \(error.localizedDescription)", category: "Agents")
        }
    }

    func loadRemoteAgents(for agent: AgentConfig) async {
        isLoadingRemoteAgents = true
        remoteAgentsError = nil
        FileLogger.shared.info("Loading remote agents for '\(agent.name)'", category: "Agents")

        do {
            remoteAgents = try await client.getRemoteAgents(agentName: agent.name)
            if remoteAgents.isEmpty {
                remoteAgentsError = "No configurable sub-agents found"
            }
            FileLogger.shared.info("Loaded \(remoteAgents.count) remote agents for '\(agent.name)'", category: "Agents")
        } catch {
            remoteAgentsError = error.localizedDescription
            remoteAgents = []
            FileLogger.shared.error("Failed to load remote agents for '\(agent.name)': \(error.localizedDescription)", category: "Agents")
        }

        isLoadingRemoteAgents = false
    }

    func saveSubAgent(for agent: AgentConfig) async {
        isSavingSubAgent = true
        FileLogger.shared.info("Saving sub-agent '\(selectedSubAgent)' for '\(agent.name)'", category: "Agents")

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
            FileLogger.shared.info("Saved sub-agent for '\(agent.name)' successfully", category: "Agents")
        } catch {
            remoteAgentsError = "Failed to save: \(error.localizedDescription)"
            FileLogger.shared.error("Failed to save sub-agent for '\(agent.name)': \(error.localizedDescription)", category: "Agents")
        }

        isSavingSubAgent = false
    }

    func runPrompt(agent: AgentConfig) async {
        isRunningPrompt = true
        promptResult = nil
        FileLogger.shared.info("Running prompt on agent '\(agent.name)'", category: "Agents")

        do {
            promptResult = try await client.runAgentPrompt(
                agentName: agent.name,
                prompt: testPrompt,
                remoteAgentName: nil
            )
            FileLogger.shared.info("Prompt on agent '\(agent.name)' completed successfully", category: "Agents")
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
            FileLogger.shared.error("Failed to run prompt on agent '\(agent.name)': \(error.localizedDescription)", category: "Agents")
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
        FileLogger.shared.info("Loading agent gallery...", category: "Agents")

        do {
            galleryEntries = try await client.getGallery(featured: false)
            FileLogger.shared.info("Loaded \(galleryEntries.count) gallery entries", category: "Agents")
        } catch {
            FileLogger.shared.error("Failed to load gallery: \(error.localizedDescription)", category: "Agents")
            galleryEntries = []
        }

        isLoadingGallery = false
    }

    func installAgent() async {
        guard let entry = selectedGalleryEntry else { return }

        isInstalling = true
        installError = nil
        FileLogger.shared.info("Installing agent from gallery entry '\(entry.id)'", category: "Agents")

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
            FileLogger.shared.info("Installed agent '\(agentName)' from gallery successfully", category: "Agents")
        } catch {
            installError = error.localizedDescription
            FileLogger.shared.error("Failed to install agent from gallery entry '\(entry.id)': \(error.localizedDescription)", category: "Agents")
        }

        isInstalling = false
    }

    func removeAgent(name: String) async {
        FileLogger.shared.info("Removing agent '\(name)'", category: "Agents")
        do {
            try await client.removeAgent(name: name)
            agents = try await client.getAgents()

            // Clear selection if removed agent was selected
            if selectedAgent?.name == name {
                selectedAgent = nil
            }
            FileLogger.shared.info("Removed agent '\(name)' successfully", category: "Agents")
        } catch {
            // TODO: Show error
            FileLogger.shared.error("Failed to remove agent '\(name)': \(error.localizedDescription)", category: "Agents")
        }
    }



    func addCustomAgent() async {
        isInstalling = true
        installError = nil
        
        do {
            var workspaceConfig: WorkspaceConfig? = nil
            if !newAgentBaseImage.isEmpty || !newAgentRepoURL.isEmpty || !newAgentSetupCommands.isEmpty {
                workspaceConfig = WorkspaceConfig(
                    baseImage: newAgentBaseImage.isEmpty ? nil : newAgentBaseImage,
                    repoUrl: newAgentRepoURL.isEmpty ? nil : newAgentRepoURL,
                    repoBranch: newAgentRepoBranch.isEmpty ? nil : newAgentRepoBranch,
                    provider: newAgentProvider.isEmpty ? nil : newAgentProvider,
                    setupCommands: newAgentSetupCommands.isEmpty ? nil : newAgentSetupCommands
                )
            }

            let agent = AgentConfig(
                name: newAgentName,
                url: newAgentURL.isEmpty ? nil : newAgentURL,
                type: "acp",
                command: nil,
                args: nil,
                env: nil,
                workdir: newAgentWorkdir.isEmpty ? nil : newAgentWorkdir,
                port: nil,
                subAgent: nil,
                enabled: true,
                description: newAgentDescription.isEmpty ? nil : newAgentDescription,
                tags: nil,
                workspaceConfig: workspaceConfig
            )
            
            try await client.addAgent(agent: agent)
            
            // Refresh
            agents = try await client.getAgents()
            showAddAgent = false
            
            // Reset
            newAgentName = ""
            newAgentURL = ""
            newAgentDescription = ""
            newAgentWorkdir = ""
            newAgentBaseImage = ""
            newAgentRepoURL = ""
            newAgentRepoBranch = ""
            newAgentProvider = ""
            newAgentSetupCommands = []
        } catch {
            installError = error.localizedDescription
        }
        
        isInstalling = false
    }

    func startEditing() {
        guard let agent = selectedAgent else { return }
        editAgentDescription = agent.description ?? ""
        editAgentWorkdir = agent.workdir ?? ""
        editError = nil
        showEditAgent = true
    }

    func saveEdit() async {
        guard let agent = selectedAgent else { return }
        
        isEditing = true
        editError = nil
        FileLogger.shared.info("Updating agent '\(agent.name)'", category: "Agents")
        
        do {
            let newDesc = editAgentDescription.isEmpty ? nil : editAgentDescription
            let newWorkdir = editAgentWorkdir.isEmpty ? nil : editAgentWorkdir
            
            try await client.updateAgent(
                name: agent.name,
                subAgent: nil,
                enabled: nil,
                description: newDesc,
                workdir: newWorkdir
            )
            
            // Refresh
            agents = try await client.getAgents()
            if let updated = agents.first(where: { $0.name == agent.name }) {
                selectedAgent = updated
            }
            showEditAgent = false
        } catch {
            editError = error.localizedDescription
            FileLogger.shared.error("Failed to update agent '\(agent.name)': \(error.localizedDescription)", category: "Agents")
        }
        
        isEditing = false
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
