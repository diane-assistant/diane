import XCTest
@testable import DianeMenu

/// Tests for `AgentsViewModel` — async data operations, state management,
/// gallery install, sub-agent configuration, prompt execution, and error handling.
@MainActor
final class AgentsViewModelTests: XCTestCase {

    // MARK: - Helpers

    private func makeViewModel(
        agents: [AgentConfig] = [],
        testResults: [String: AgentTestResult] = [:],
        logs: [AgentLog] = [],
        remoteAgents: [RemoteAgentInfo] = [],
        galleryEntries: [GalleryEntry] = [],
        installResponse: GalleryInstallResponse? = nil,
        runResult: AgentRunResult? = nil
    ) -> (AgentsViewModel, MockDianeClient) {
        let mock = MockDianeClient()
        mock.agentsList = agents
        mock.agentTestResults = testResults
        mock.agentLogs = logs
        mock.remoteAgentsList = remoteAgents
        mock.galleryEntriesList = galleryEntries
        mock.installResponse = installResponse
        mock.agentRunResultToReturn = runResult
        let vm = AgentsViewModel(client: mock)
        return (vm, mock)
    }

    // =========================================================================
    // MARK: - Initial State
    // =========================================================================

    func testInitialState() {
        let (vm, _) = makeViewModel()
        XCTAssertTrue(vm.agents.isEmpty)
        XCTAssertTrue(vm.logs.isEmpty)
        XCTAssertTrue(vm.isLoading)
        XCTAssertNil(vm.error)
        XCTAssertNil(vm.selectedAgent)
        XCTAssertTrue(vm.testResults.isEmpty)
        XCTAssertEqual(vm.testPrompt, "")
        XCTAssertFalse(vm.isRunningPrompt)
        XCTAssertNil(vm.promptResult)
        XCTAssertTrue(vm.remoteAgents.isEmpty)
        XCTAssertFalse(vm.isLoadingRemoteAgents)
        XCTAssertNil(vm.remoteAgentsError)
        XCTAssertEqual(vm.selectedSubAgent, "")
        XCTAssertFalse(vm.isSavingSubAgent)
        XCTAssertFalse(vm.showAddAgent)
        XCTAssertTrue(vm.galleryEntries.isEmpty)
        XCTAssertFalse(vm.isLoadingGallery)
        XCTAssertNil(vm.selectedGalleryEntry)
        XCTAssertEqual(vm.newAgentName, "")
        XCTAssertEqual(vm.newAgentWorkdir, "")
        XCTAssertEqual(vm.newAgentPort, "")
        XCTAssertFalse(vm.isInstalling)
        XCTAssertNil(vm.installError)
    }

    // =========================================================================
    // MARK: - loadData
    // =========================================================================

    func testLoadData_populatesAgents() async {
        let agents = TestFixtures.makeAgentList()
        let (vm, mock) = makeViewModel(agents: agents)
        // Pre-populate test results so the auto-test background tasks can succeed
        for agent in agents where agent.enabled {
            mock.agentTestResults[agent.name] = TestFixtures.makeAgentTestResult(name: agent.name)
        }

        await vm.loadData()

        XCTAssertFalse(vm.isLoading)
        XCTAssertNil(vm.error)
        XCTAssertEqual(vm.agents.count, agents.count)
        XCTAssertEqual(mock.callCount(for: "getAgents"), 1)
    }

    func testLoadData_setsErrorOnFailure() async {
        let (vm, mock) = makeViewModel()
        mock.errorToThrow = MockError.networkFailure

        await vm.loadData()

        XCTAssertFalse(vm.isLoading)
        XCTAssertNotNil(vm.error)
        XCTAssertTrue(vm.agents.isEmpty)
    }

    func testLoadData_clearsErrorOnRetry() async {
        let agents = TestFixtures.makeAgentList()
        let (vm, mock) = makeViewModel(agents: agents)
        // Pre-populate test results for enabled agents
        for agent in agents where agent.enabled {
            mock.agentTestResults[agent.name] = TestFixtures.makeAgentTestResult(name: agent.name)
        }

        // First call: error
        mock.errorToThrow = MockError.networkFailure
        await vm.loadData()
        XCTAssertNotNil(vm.error)

        // Second call: success
        mock.errorToThrow = nil
        await vm.loadData()
        XCTAssertNil(vm.error)
        XCTAssertEqual(vm.agents.count, agents.count)
    }

    // =========================================================================
    // MARK: - loadLogs
    // =========================================================================

    func testLoadLogs_populatesLogs() async {
        let logs = TestFixtures.makeAgentLogList(agentName: "gemini")
        let (vm, mock) = makeViewModel(logs: logs)

        await vm.loadLogs(forAgent: "gemini")

        XCTAssertEqual(vm.logs.count, logs.count)
        XCTAssertEqual(mock.callCount(for: "getAgentLogs"), 1)
    }

    func testLoadLogs_filtersbyAgentName() async {
        let logs = [
            TestFixtures.makeAgentLog(id: 1, agentName: "gemini"),
            TestFixtures.makeAgentLog(id: 2, agentName: "claude"),
            TestFixtures.makeAgentLog(id: 3, agentName: "gemini"),
        ]
        let (vm, _) = makeViewModel(logs: logs)

        await vm.loadLogs(forAgent: "gemini")

        XCTAssertEqual(vm.logs.count, 2)
        XCTAssertTrue(vm.logs.allSatisfy { $0.agentName == "gemini" })
    }

    func testLoadLogs_clearsLogsOnError() async {
        let (vm, mock) = makeViewModel(logs: TestFixtures.makeAgentLogList())
        // First populate logs
        await vm.loadLogs(forAgent: "test-agent")
        XCTAssertFalse(vm.logs.isEmpty)

        // Now fail
        mock.errorToThrow = MockError.networkFailure
        await vm.loadLogs(forAgent: "test-agent")
        XCTAssertTrue(vm.logs.isEmpty)
    }

    // =========================================================================
    // MARK: - testAgent
    // =========================================================================

    func testTestAgent_storesResult() async {
        let agent = TestFixtures.makeAgentConfig(name: "gemini")
        let result = TestFixtures.makeAgentTestResult(name: "gemini", status: "connected")
        let (vm, _) = makeViewModel(testResults: ["gemini": result])

        await vm.testAgent(agent)

        XCTAssertNotNil(vm.testResults["gemini"])
        XCTAssertEqual(vm.testResults["gemini"]?.status, "connected")
    }

    func testTestAgent_storesErrorResult() async {
        let agent = TestFixtures.makeAgentConfig(name: "broken")
        let (vm, mock) = makeViewModel()
        mock.errorToThrow = MockError.networkFailure

        await vm.testAgent(agent)

        XCTAssertNotNil(vm.testResults["broken"])
        XCTAssertEqual(vm.testResults["broken"]?.status, "error")
        XCTAssertNotNil(vm.testResults["broken"]?.error)
    }

    // =========================================================================
    // MARK: - toggleAgent
    // =========================================================================

    func testToggleAgent_callsClientAndRefreshes() async {
        let agents = [TestFixtures.makeAgentConfig(name: "gemini", enabled: true)]
        let (vm, mock) = makeViewModel(agents: agents)

        await vm.toggleAgent(agents[0], enabled: false)

        XCTAssertEqual(mock.callCount(for: "toggleAgent"), 1)
        // getAgents called to refresh
        XCTAssertEqual(mock.callCount(for: "getAgents"), 1)
        // Mock actually mutates the list
        XCTAssertEqual(vm.agents.first?.enabled, false)
    }

    // =========================================================================
    // MARK: - loadRemoteAgents
    // =========================================================================

    func testLoadRemoteAgents_populatesList() async {
        let remotes = TestFixtures.makeRemoteAgentList()
        let agent = TestFixtures.makeAgentConfig(name: "gemini")
        let (vm, mock) = makeViewModel(remoteAgents: remotes)

        await vm.loadRemoteAgents(for: agent)

        XCTAssertFalse(vm.isLoadingRemoteAgents)
        XCTAssertNil(vm.remoteAgentsError)
        XCTAssertEqual(vm.remoteAgents.count, remotes.count)
        XCTAssertEqual(mock.callCount(for: "getRemoteAgents"), 1)
    }

    func testLoadRemoteAgents_setsErrorWhenEmpty() async {
        let agent = TestFixtures.makeAgentConfig(name: "gemini")
        let (vm, _) = makeViewModel(remoteAgents: [])

        await vm.loadRemoteAgents(for: agent)

        XCTAssertFalse(vm.isLoadingRemoteAgents)
        XCTAssertEqual(vm.remoteAgentsError, "No configurable sub-agents found")
        XCTAssertTrue(vm.remoteAgents.isEmpty)
    }

    func testLoadRemoteAgents_setsErrorOnFailure() async {
        let agent = TestFixtures.makeAgentConfig(name: "gemini")
        let (vm, mock) = makeViewModel()
        mock.errorToThrow = MockError.networkFailure

        await vm.loadRemoteAgents(for: agent)

        XCTAssertFalse(vm.isLoadingRemoteAgents)
        XCTAssertNotNil(vm.remoteAgentsError)
        XCTAssertTrue(vm.remoteAgents.isEmpty)
    }

    // =========================================================================
    // MARK: - saveSubAgent
    // =========================================================================

    func testSaveSubAgent_updatesAndRefreshes() async {
        let agents = [TestFixtures.makeAgentConfig(name: "gemini", subAgent: nil)]
        let (vm, mock) = makeViewModel(agents: agents)
        vm.selectedSubAgent = "gpt-4o"

        await vm.saveSubAgent(for: agents[0])

        XCTAssertFalse(vm.isSavingSubAgent)
        XCTAssertEqual(mock.callCount(for: "updateAgent"), 1)
        XCTAssertEqual(mock.lastUpdateAgentArgs?.subAgent, "gpt-4o")
        // getAgents called to refresh
        XCTAssertEqual(mock.callCount(for: "getAgents"), 1)
    }

    func testSaveSubAgent_updatesSelectedAgent() async {
        let agents = [TestFixtures.makeAgentConfig(name: "gemini", subAgent: nil)]
        let (vm, _) = makeViewModel(agents: agents)
        vm.selectedAgent = agents[0]
        vm.selectedSubAgent = "claude-sonnet"

        await vm.saveSubAgent(for: agents[0])

        // After refresh, selectedAgent should be updated
        XCTAssertEqual(vm.selectedAgent?.subAgent, "claude-sonnet")
    }

    func testSaveSubAgent_setsErrorOnFailure() async {
        let agents = [TestFixtures.makeAgentConfig(name: "gemini")]
        let (vm, mock) = makeViewModel(agents: agents)
        mock.errorToThrow = MockError.networkFailure

        await vm.saveSubAgent(for: agents[0])

        XCTAssertFalse(vm.isSavingSubAgent)
        XCTAssertNotNil(vm.remoteAgentsError)
        XCTAssertTrue(vm.remoteAgentsError?.contains("Failed to save") == true)
    }

    // =========================================================================
    // MARK: - runPrompt
    // =========================================================================

    func testRunPrompt_setsResultOnSuccess() async {
        let agent = TestFixtures.makeAgentConfig(name: "gemini")
        let result = TestFixtures.makeAgentRunResultWithOutput(agentName: "gemini", text: "Hello!")
        let (vm, mock) = makeViewModel(runResult: result)
        vm.testPrompt = "Say hello"
        vm.selectedAgent = agent

        await vm.runPrompt(agent: agent)

        XCTAssertFalse(vm.isRunningPrompt)
        XCTAssertNotNil(vm.promptResult)
        XCTAssertEqual(vm.promptResult?.textOutput, "Hello!")
        XCTAssertEqual(mock.callCount(for: "runAgentPrompt"), 1)
    }

    func testRunPrompt_setsErrorResultOnFailure() async {
        let agent = TestFixtures.makeAgentConfig(name: "gemini")
        let (vm, mock) = makeViewModel()
        mock.errorToThrow = MockError.networkFailure
        vm.testPrompt = "Say hello"
        vm.selectedAgent = agent

        await vm.runPrompt(agent: agent)

        XCTAssertFalse(vm.isRunningPrompt)
        XCTAssertNotNil(vm.promptResult)
        XCTAssertEqual(vm.promptResult?.status, "failed")
        XCTAssertNotNil(vm.promptResult?.error)
        XCTAssertEqual(vm.promptResult?.error?.code, "client_error")
    }

    func testRunPrompt_refreshesLogs() async {
        let agent = TestFixtures.makeAgentConfig(name: "gemini")
        let logs = TestFixtures.makeAgentLogList(agentName: "gemini")
        let result = TestFixtures.makeAgentRunResultWithOutput(agentName: "gemini")
        let (vm, mock) = makeViewModel(logs: logs, runResult: result)
        vm.testPrompt = "test"
        vm.selectedAgent = agent

        await vm.runPrompt(agent: agent)

        // getAgentLogs should have been called to refresh logs after prompt
        XCTAssertEqual(mock.callCount(for: "getAgentLogs"), 1)
        XCTAssertFalse(vm.logs.isEmpty)
    }

    func testRunPrompt_clearsPromptResultFirst() async {
        let agent = TestFixtures.makeAgentConfig(name: "gemini")
        let result = TestFixtures.makeAgentRunResultWithOutput(agentName: "gemini")
        let (vm, _) = makeViewModel(runResult: result)
        vm.promptResult = TestFixtures.makeAgentRunResultFailed(agentName: "gemini")
        vm.testPrompt = "hello"
        vm.selectedAgent = agent

        // Verify old result is replaced
        await vm.runPrompt(agent: agent)
        XCTAssertEqual(vm.promptResult?.status, "completed")
    }

    // =========================================================================
    // MARK: - loadGallery
    // =========================================================================

    func testLoadGallery_populatesEntries() async {
        let entries = TestFixtures.makeGalleryEntryList()
        let (vm, mock) = makeViewModel(galleryEntries: entries)

        await vm.loadGallery()

        XCTAssertFalse(vm.isLoadingGallery)
        XCTAssertEqual(vm.galleryEntries.count, entries.count)
        XCTAssertEqual(mock.callCount(for: "getGallery"), 1)
    }

    func testLoadGallery_clearsOnError() async {
        let (vm, mock) = makeViewModel()
        mock.errorToThrow = MockError.networkFailure

        await vm.loadGallery()

        XCTAssertFalse(vm.isLoadingGallery)
        XCTAssertTrue(vm.galleryEntries.isEmpty)
    }

    // =========================================================================
    // MARK: - installAgent
    // =========================================================================

    func testInstallAgent_successClosesSheet() async {
        let entry = TestFixtures.makeGalleryEntry(entryId: "gemini", name: "Gemini")
        let response = TestFixtures.makeGalleryInstallResponse(agent: "gemini")
        let agents = [TestFixtures.makeAgentConfig(name: "gemini")]
        let testResult = TestFixtures.makeAgentTestResult(name: "gemini")
        let (vm, mock) = makeViewModel(
            agents: agents,
            testResults: ["gemini": testResult],
            installResponse: response
        )
        vm.showAddAgent = true
        vm.selectedGalleryEntry = entry
        vm.newAgentName = "custom-name"
        vm.newAgentWorkdir = "/tmp"
        vm.newAgentPort = "4322"

        await vm.installAgent()

        XCTAssertFalse(vm.isInstalling)
        XCTAssertFalse(vm.showAddAgent)
        XCTAssertNil(vm.selectedGalleryEntry)
        XCTAssertEqual(vm.newAgentName, "")
        XCTAssertEqual(vm.newAgentWorkdir, "")
        XCTAssertEqual(vm.newAgentPort, "")
        XCTAssertNil(vm.installError)
        XCTAssertEqual(mock.callCount(for: "installGalleryAgent"), 1)
        XCTAssertEqual(mock.lastInstallAgentArgs?.id, "gemini")
        XCTAssertEqual(mock.lastInstallAgentArgs?.name, "custom-name")
        XCTAssertEqual(mock.lastInstallAgentArgs?.workdir, "/tmp")
        XCTAssertEqual(mock.lastInstallAgentArgs?.port, 4322)
    }

    func testInstallAgent_noSelectionDoesNothing() async {
        let (vm, mock) = makeViewModel()
        vm.selectedGalleryEntry = nil

        await vm.installAgent()

        XCTAssertEqual(mock.callCount(for: "installGalleryAgent"), 0)
    }

    func testInstallAgent_setsErrorOnFailure() async {
        let entry = TestFixtures.makeGalleryEntry()
        let (vm, mock) = makeViewModel()
        mock.errorToThrow = MockError.networkFailure
        vm.selectedGalleryEntry = entry

        await vm.installAgent()

        XCTAssertFalse(vm.isInstalling)
        XCTAssertNotNil(vm.installError)
    }

    func testInstallAgent_emptyFieldsPassNil() async {
        let entry = TestFixtures.makeGalleryEntry(entryId: "gemini")
        let response = TestFixtures.makeGalleryInstallResponse(agent: "gemini")
        let agents = [TestFixtures.makeAgentConfig(name: "gemini")]
        let testResult = TestFixtures.makeAgentTestResult(name: "gemini")
        let (vm, mock) = makeViewModel(
            agents: agents,
            testResults: ["gemini": testResult],
            installResponse: response
        )
        vm.selectedGalleryEntry = entry
        vm.newAgentName = ""
        vm.newAgentWorkdir = ""
        vm.newAgentPort = ""

        await vm.installAgent()

        XCTAssertNil(mock.lastInstallAgentArgs?.name)
        XCTAssertNil(mock.lastInstallAgentArgs?.workdir)
        XCTAssertNil(mock.lastInstallAgentArgs?.port)
    }

    // =========================================================================
    // MARK: - removeAgent
    // =========================================================================

    func testRemoveAgent_removesFromList() async {
        let agents = TestFixtures.makeAgentList()
        let (vm, mock) = makeViewModel(agents: agents)

        await vm.removeAgent(name: "gemini")

        XCTAssertEqual(mock.callCount(for: "removeAgent"), 1)
        XCTAssertEqual(mock.callCount(for: "getAgents"), 1)
        XCTAssertFalse(vm.agents.contains(where: { $0.name == "gemini" }))
    }

    func testRemoveAgent_clearsSelectionIfRemovedAgentSelected() async {
        let agents = TestFixtures.makeAgentList()
        let (vm, _) = makeViewModel(agents: agents)
        vm.selectedAgent = agents[0] // gemini

        await vm.removeAgent(name: "gemini")

        XCTAssertNil(vm.selectedAgent)
    }

    func testRemoveAgent_keepsSelectionIfDifferentAgentRemoved() async {
        let agents = TestFixtures.makeAgentList()
        let (vm, _) = makeViewModel(agents: agents)
        vm.selectedAgent = agents[0] // gemini

        await vm.removeAgent(name: "opencode")

        XCTAssertEqual(vm.selectedAgent?.name, "gemini")
    }

    // =========================================================================
    // MARK: - resetInstallForm
    // =========================================================================

    func testResetInstallForm_clearsAllFields() {
        let (vm, _) = makeViewModel()
        vm.selectedGalleryEntry = TestFixtures.makeGalleryEntry()
        vm.newAgentName = "test"
        vm.newAgentWorkdir = "/tmp"
        vm.newAgentPort = "4322"
        vm.installError = "some error"

        vm.resetInstallForm()

        XCTAssertNil(vm.selectedGalleryEntry)
        XCTAssertEqual(vm.newAgentName, "")
        XCTAssertEqual(vm.newAgentWorkdir, "")
        XCTAssertEqual(vm.newAgentPort, "")
        XCTAssertNil(vm.installError)
    }

    // =========================================================================
    // MARK: - onSelectAgent
    // =========================================================================

    func testOnSelectAgent_setsSelectedAndClearsPromptResult() {
        let agent = TestFixtures.makeAgentConfig(name: "gemini")
        let (vm, _) = makeViewModel()
        vm.promptResult = TestFixtures.makeAgentRunResultWithOutput()

        vm.onSelectAgent(agent)

        XCTAssertEqual(vm.selectedAgent?.name, "gemini")
        XCTAssertNil(vm.promptResult)
    }

    // =========================================================================
    // MARK: - onSelectedAgentChanged
    // =========================================================================

    func testOnSelectedAgentChanged_resetsRemoteAgentState() {
        let (vm, _) = makeViewModel()
        vm.remoteAgents = TestFixtures.makeRemoteAgentList()
        vm.remoteAgentsError = "old error"
        vm.selectedAgent = TestFixtures.makeAgentConfig(name: "gemini", subAgent: "gpt-4o")

        vm.onSelectedAgentChanged()

        XCTAssertTrue(vm.remoteAgents.isEmpty)
        XCTAssertNil(vm.remoteAgentsError)
        XCTAssertEqual(vm.selectedSubAgent, "gpt-4o")
    }

    func testOnSelectedAgentChanged_defaultsSubAgentToEmpty() {
        let (vm, _) = makeViewModel()
        vm.selectedAgent = TestFixtures.makeAgentConfig(name: "gemini", subAgent: nil)

        vm.onSelectedAgentChanged()

        XCTAssertEqual(vm.selectedSubAgent, "")
    }

    func testOnSelectedAgentChanged_noSelectedAgent() {
        let (vm, _) = makeViewModel()
        vm.remoteAgents = TestFixtures.makeRemoteAgentList()
        vm.selectedAgent = nil

        vm.onSelectedAgentChanged()

        XCTAssertTrue(vm.remoteAgents.isEmpty)
    }

    // =========================================================================
    // MARK: - Error Recovery
    // =========================================================================

    func testLoadData_errorThenSuccess() async {
        let agents = TestFixtures.makeAgentList()
        let (vm, mock) = makeViewModel(agents: agents)
        for agent in agents where agent.enabled {
            mock.agentTestResults[agent.name] = TestFixtures.makeAgentTestResult(name: agent.name)
        }

        // Error
        mock.errorToThrow = MockError.serverError("Server down")
        await vm.loadData()
        XCTAssertNotNil(vm.error)
        XCTAssertTrue(vm.error?.contains("Server down") == true)
        XCTAssertTrue(vm.agents.isEmpty)

        // Recover
        mock.errorToThrow = nil
        await vm.loadData()
        XCTAssertNil(vm.error)
        XCTAssertEqual(vm.agents.count, agents.count)
    }

    // =========================================================================
    // MARK: - Integration-style Tests
    // =========================================================================

    func testFullWorkflow_loadSelectTestRun() async {
        let agents = [
            TestFixtures.makeAgentConfig(name: "gemini", enabled: true),
        ]
        let testResult = TestFixtures.makeAgentTestResult(name: "gemini", status: "connected")
        let runResult = TestFixtures.makeAgentRunResultWithOutput(agentName: "gemini", text: "I'm alive!")
        let logs = TestFixtures.makeAgentLogList(agentName: "gemini")
        let (vm, mock) = makeViewModel(
            agents: agents,
            testResults: ["gemini": testResult],
            logs: logs,
            runResult: runResult
        )

        // 1. Load data
        await vm.loadData()
        XCTAssertEqual(vm.agents.count, 1)

        // 2. Select agent
        vm.onSelectAgent(agents[0])
        XCTAssertEqual(vm.selectedAgent?.name, "gemini")

        // 3. Load logs
        await vm.loadLogs(forAgent: "gemini")
        XCTAssertFalse(vm.logs.isEmpty)

        // 4. Run prompt
        vm.testPrompt = "Are you alive?"
        await vm.runPrompt(agent: agents[0])
        XCTAssertEqual(vm.promptResult?.textOutput, "I'm alive!")

        // Verify call counts
        XCTAssertGreaterThanOrEqual(mock.callCount(for: "getAgents"), 1)
        XCTAssertGreaterThanOrEqual(mock.callCount(for: "getAgentLogs"), 1)
        XCTAssertEqual(mock.callCount(for: "runAgentPrompt"), 1)
    }

    func testFullWorkflow_installFromGallery() async {
        let entry = TestFixtures.makeGalleryEntry(entryId: "gemini")
        let entries = TestFixtures.makeGalleryEntryList()
        let response = TestFixtures.makeGalleryInstallResponse(agent: "gemini")
        let installedAgent = TestFixtures.makeAgentConfig(name: "gemini")
        let testResult = TestFixtures.makeAgentTestResult(name: "gemini")
        let (vm, mock) = makeViewModel(
            agents: [installedAgent],
            testResults: ["gemini": testResult],
            galleryEntries: entries,
            installResponse: response
        )

        // 1. Load gallery
        await vm.loadGallery()
        XCTAssertEqual(vm.galleryEntries.count, entries.count)

        // 2. Select entry and configure
        vm.selectedGalleryEntry = entry
        vm.newAgentName = "my-gemini"

        // 3. Install
        await vm.installAgent()
        XCTAssertFalse(vm.showAddAgent)
        XCTAssertEqual(mock.callCount(for: "installGalleryAgent"), 1)
        XCTAssertEqual(mock.callCount(for: "getAgents"), 1)
    }

    func testFullWorkflow_discoverAndSaveSubAgent() async {
        let agents = [TestFixtures.makeAgentConfig(name: "gemini", subAgent: nil)]
        let remotes = TestFixtures.makeRemoteAgentList()
        let (vm, mock) = makeViewModel(agents: agents, remoteAgents: remotes)

        // 1. Select agent
        vm.onSelectAgent(agents[0])

        // 2. Discover remote agents
        await vm.loadRemoteAgents(for: agents[0])
        XCTAssertEqual(vm.remoteAgents.count, remotes.count)

        // 3. Select and save sub-agent
        vm.selectedSubAgent = "gpt-4o"
        await vm.saveSubAgent(for: agents[0])

        XCTAssertEqual(mock.callCount(for: "updateAgent"), 1)
        XCTAssertEqual(mock.lastUpdateAgentArgs?.subAgent, "gpt-4o")
        XCTAssertEqual(vm.selectedAgent?.subAgent, "gpt-4o")
    }
}
