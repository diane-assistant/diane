import XCTest
@testable import Diane

/// Tests for `MCPServersViewModel` — pure functions, async data operations,
/// and form validation logic.
@MainActor
final class MCPServersViewModelTests: XCTestCase {

    // MARK: - Helpers

    private func makeViewModel(servers: [MCPServer] = []) -> (MCPServersViewModel, MockDianeClient) {
        let mock = MockDianeClient()
        mock.serverConfigs = servers
        let vm = MCPServersViewModel(client: mock)
        return (vm, mock)
    }

    // =========================================================================
    // MARK: - 6.4  Pure Function Tests
    // =========================================================================

    // MARK: generateDuplicateName

    func testGenerateDuplicateName_appendsTwo() {
        let result = MCPServersViewModel.generateDuplicateName(
            from: "my-server", existingNames: ["my-server"])
        XCTAssertEqual(result, "my-server (2)")
    }

    func testGenerateDuplicateName_incrementsExistingNumber() {
        let result = MCPServersViewModel.generateDuplicateName(
            from: "my-server (2)", existingNames: ["my-server", "my-server (2)"])
        XCTAssertEqual(result, "my-server (3)")
    }

    func testGenerateDuplicateName_findsHighestSuffix() {
        let existing = ["my-server", "my-server (2)", "my-server (3)", "my-server (5)"]
        let result = MCPServersViewModel.generateDuplicateName(
            from: "my-server", existingNames: existing)
        XCTAssertEqual(result, "my-server (6)")
    }

    func testGenerateDuplicateName_noConflict() {
        let result = MCPServersViewModel.generateDuplicateName(
            from: "unique", existingNames: ["other-server"])
        XCTAssertEqual(result, "unique (2)")
    }

    func testGenerateDuplicateName_emptyExisting() {
        let result = MCPServersViewModel.generateDuplicateName(
            from: "server", existingNames: [])
        XCTAssertEqual(result, "server (2)")
    }

    // MARK: filteredServers

    func testFilteredServers_nilTypeReturnsAll() {
        let servers = TestFixtures.makeMixedServerList()
        let result = MCPServersViewModel.filteredServers(servers, byType: nil)
        XCTAssertEqual(result.count, servers.count)
    }

    func testFilteredServers_filterByStdio() {
        let servers = TestFixtures.makeMixedServerList()
        let result = MCPServersViewModel.filteredServers(servers, byType: .stdio)
        XCTAssertTrue(result.allSatisfy { $0.type == "stdio" })
        XCTAssertEqual(result.count, 2) // node-mcp + disabled-server
    }

    func testFilteredServers_filterBySSE() {
        let servers = TestFixtures.makeMixedServerList()
        let result = MCPServersViewModel.filteredServers(servers, byType: .sse)
        XCTAssertTrue(result.allSatisfy { $0.type == "sse" })
        XCTAssertEqual(result.count, 1)
    }

    func testFilteredServers_filterByHTTP() {
        let servers = TestFixtures.makeMixedServerList()
        let result = MCPServersViewModel.filteredServers(servers, byType: .http)
        XCTAssertTrue(result.allSatisfy { $0.type == "http" })
        XCTAssertEqual(result.count, 1)
    }

    func testFilteredServers_filterByBuiltin() {
        let servers = TestFixtures.makeMixedServerList()
        let result = MCPServersViewModel.filteredServers(servers, byType: .builtin)
        XCTAssertTrue(result.allSatisfy { $0.type == "builtin" })
        XCTAssertEqual(result.count, 1)
    }

    func testFilteredServers_emptyList() {
        let result = MCPServersViewModel.filteredServers([], byType: .stdio)
        XCTAssertTrue(result.isEmpty)
    }

    // MARK: isServerFormValid

    func testFormValid_stdioWithNameAndCommand() {
        XCTAssertTrue(MCPServersViewModel.isServerFormValid(
            name: "test", type: .stdio, command: "/bin/mcp", url: "", isBusy: false))
    }

    func testFormInvalid_emptyName() {
        XCTAssertFalse(MCPServersViewModel.isServerFormValid(
            name: "", type: .stdio, command: "/bin/mcp", url: "", isBusy: false))
    }

    func testFormInvalid_stdioEmptyCommand() {
        XCTAssertFalse(MCPServersViewModel.isServerFormValid(
            name: "test", type: .stdio, command: "", url: "", isBusy: false))
    }

    func testFormValid_sseWithURL() {
        XCTAssertTrue(MCPServersViewModel.isServerFormValid(
            name: "test", type: .sse, command: "", url: "https://example.com", isBusy: false))
    }

    func testFormInvalid_sseEmptyURL() {
        XCTAssertFalse(MCPServersViewModel.isServerFormValid(
            name: "test", type: .sse, command: "", url: "", isBusy: false))
    }

    func testFormValid_httpWithURL() {
        XCTAssertTrue(MCPServersViewModel.isServerFormValid(
            name: "test", type: .http, command: "", url: "https://example.com", isBusy: false))
    }

    func testFormInvalid_httpEmptyURL() {
        XCTAssertFalse(MCPServersViewModel.isServerFormValid(
            name: "test", type: .http, command: "", url: "", isBusy: false))
    }

    func testFormValid_builtinOnlyNeedsName() {
        XCTAssertTrue(MCPServersViewModel.isServerFormValid(
            name: "test", type: .builtin, command: "", url: "", isBusy: false))
    }

    func testFormInvalid_isBusy() {
        XCTAssertFalse(MCPServersViewModel.isServerFormValid(
            name: "test", type: .builtin, command: "", url: "", isBusy: true))
    }

    // =========================================================================
    // MARK: - 7  ViewModel Async Operation Tests
    // =========================================================================

    // MARK: loadData

    func testLoadData_populatesServers() async {
        let servers = TestFixtures.makeMixedServerList()
        let (vm, _) = makeViewModel(servers: servers)

        await vm.loadData()

        XCTAssertEqual(vm.servers.count, servers.count)
        XCTAssertFalse(vm.isLoading)
        XCTAssertNil(vm.error)
    }

    func testLoadData_selectsFirstServerWhenNoneSelected() async {
        let servers = [TestFixtures.makeStdioServer()]
        let (vm, _) = makeViewModel(servers: servers)
        XCTAssertNil(vm.selectedServer)

        await vm.loadData()

        XCTAssertEqual(vm.selectedServer?.id, servers.first?.id)
    }

    func testLoadData_preservesExistingSelection() async {
        let servers = TestFixtures.makeMixedServerList()
        let (vm, _) = makeViewModel(servers: servers)
        // Pre-select the second server
        let preSelected = TestFixtures.makeSSEServer(id: 2)
        vm.selectedServer = preSelected

        await vm.loadData()

        // Should keep the pre-existing selection
        XCTAssertEqual(vm.selectedServer?.id, preSelected.id)
    }

    func testLoadData_setsErrorOnFailure() async {
        let (vm, mock) = makeViewModel()
        mock.errorToThrow = MockError.networkFailure

        await vm.loadData()

        XCTAssertNotNil(vm.error)
        XCTAssertTrue(vm.servers.isEmpty)
        XCTAssertFalse(vm.isLoading)
    }

    func testLoadData_setsIsLoadingDuringOperation() async {
        let (vm, _) = makeViewModel()

        // Before load, isLoading starts true (initial value)
        XCTAssertTrue(vm.isLoading)

        await vm.loadData()

        // After load completes, isLoading should be false
        XCTAssertFalse(vm.isLoading)
    }

    // MARK: createServer

    func testCreateServer_appendsAndSelects() async {
        let (vm, _) = makeViewModel()
        await vm.loadData()

        vm.newServerName = "new-server"
        vm.newServerType = .stdio
        vm.newServerCommand = "/bin/test"
        vm.showCreateServer = true

        await vm.createServer()

        XCTAssertEqual(vm.servers.count, 1)
        XCTAssertEqual(vm.servers.first?.name, "new-server")
        XCTAssertEqual(vm.selectedServer?.name, "new-server")
        XCTAssertFalse(vm.showCreateServer)
        XCTAssertFalse(vm.isCreating)
    }

    func testCreateServer_callsClientWithCorrectParams() async {
        let (vm, mock) = makeViewModel()

        vm.newServerName = "my-server"
        vm.newServerType = .sse
        vm.newServerURL = "https://example.com/sse"
        vm.newServerHeaders = ["X-Key": "abc"]

        await vm.createServer()

        XCTAssertEqual(mock.callCount(for: "createMCPServerConfig"), 1)
        XCTAssertEqual(mock.lastCreateArgs?.name, "my-server")
        XCTAssertEqual(mock.lastCreateArgs?.type, "sse")
        XCTAssertEqual(mock.lastCreateArgs?.url, "https://example.com/sse")
    }

    func testCreateServer_resetsFormAfterSuccess() async {
        let (vm, _) = makeViewModel()

        vm.newServerName = "server"
        vm.newServerType = .http
        vm.newServerURL = "https://example.com"

        await vm.createServer()

        XCTAssertEqual(vm.newServerName, "")
        XCTAssertEqual(vm.newServerType, .stdio) // reset to default
        XCTAssertEqual(vm.newServerURL, "")
    }

    func testCreateServer_setsErrorOnFailure() async {
        let (vm, mock) = makeViewModel()
        mock.errorToThrow = MockError.serverError("Duplicate name")

        vm.newServerName = "test"
        vm.newServerType = .builtin

        await vm.createServer()

        XCTAssertNotNil(vm.createError)
        XCTAssertTrue(vm.servers.isEmpty)
        XCTAssertFalse(vm.isCreating)
    }

    // MARK: updateServer

    func testUpdateServer_updatesInList() async {
        let server = TestFixtures.makeStdioServer(id: 1, name: "original")
        let (vm, _) = makeViewModel(servers: [server])
        await vm.loadData()

        vm.editName = "renamed"
        vm.editType = .stdio
        vm.editCommand = "/bin/test"
        vm.editEnabled = true

        await vm.updateServer(server)

        XCTAssertEqual(vm.servers.first?.name, "renamed")
        XCTAssertEqual(vm.selectedServer?.name, "renamed")
        XCTAssertNil(vm.editingServer)
        XCTAssertFalse(vm.isEditing)
    }

    func testUpdateServer_setsErrorOnFailure() async {
        let server = TestFixtures.makeStdioServer()
        let (vm, mock) = makeViewModel(servers: [server])
        await vm.loadData()

        mock.errorToThrow = MockError.networkFailure
        vm.editName = "renamed"

        await vm.updateServer(server)

        XCTAssertNotNil(vm.editError)
        // Original name should be preserved
        XCTAssertEqual(vm.servers.first?.name, "test-server")
    }

    // MARK: deleteServer

    func testDeleteServer_removesFromList() async {
        let server = TestFixtures.makeStdioServer(id: 1)
        let (vm, _) = makeViewModel(servers: [server])
        await vm.loadData()

        await vm.deleteServer(server)

        XCTAssertTrue(vm.servers.isEmpty)
        XCTAssertNil(vm.selectedServer)
    }

    func testDeleteServer_selectsFirstAfterDeletingSelected() async {
        let server1 = TestFixtures.makeStdioServer(id: 1, name: "s1")
        let server2 = TestFixtures.makeSSEServer(id: 2, name: "s2")
        let (vm, _) = makeViewModel(servers: [server1, server2])
        await vm.loadData()
        vm.selectedServer = server1

        await vm.deleteServer(server1)

        XCTAssertEqual(vm.servers.count, 1)
        XCTAssertEqual(vm.selectedServer?.id, 2)
    }

    func testDeleteServer_setsErrorOnFailure() async {
        let server = TestFixtures.makeStdioServer(id: 1)
        let (vm, mock) = makeViewModel(servers: [server])
        await vm.loadData()

        mock.errorToThrow = MockError.networkFailure

        await vm.deleteServer(server)

        XCTAssertNotNil(vm.error)
        // Server still in list since delete failed on the mock's throw
        // (but mock also throws before removeAll, so server remains)
        XCTAssertEqual(vm.servers.count, 1)
    }

    // MARK: duplicateServer

    func testDuplicateServer_createsWithSuffix() async {
        let server = TestFixtures.makeStdioServer(id: 1, name: "my-server")
        let (vm, mock) = makeViewModel(servers: [server])
        await vm.loadData()

        await vm.duplicateServer(server)

        XCTAssertEqual(vm.servers.count, 2)
        XCTAssertEqual(mock.lastCreateArgs?.name, "my-server (2)")
        XCTAssertEqual(vm.selectedServer?.name, "my-server (2)")
    }

    func testDuplicateServer_incrementsWhenTwoExists() async {
        let server1 = TestFixtures.makeStdioServer(id: 1, name: "my-server")
        let server2 = TestFixtures.makeStdioServer(id: 2, name: "my-server (2)")
        let (vm, mock) = makeViewModel(servers: [server1, server2])
        await vm.loadData()

        await vm.duplicateServer(server1)

        XCTAssertEqual(mock.lastCreateArgs?.name, "my-server (3)")
    }

    func testDuplicateServer_copiesConfiguration() async {
        let server = TestFixtures.makeStdioServer(
            id: 1, name: "configured",
            command: "/usr/bin/custom",
            args: ["--flag", "value"],
            env: ["KEY": "VAL"])
        let (vm, mock) = makeViewModel(servers: [server])
        await vm.loadData()

        await vm.duplicateServer(server)

        XCTAssertEqual(mock.lastCreateArgs?.command, "/usr/bin/custom")
        XCTAssertEqual(mock.lastCreateArgs?.args, ["--flag", "value"])
        XCTAssertEqual(mock.lastCreateArgs?.env, ["KEY": "VAL"])
    }

    func testDuplicateServer_setsErrorOnFailure() async {
        let server = TestFixtures.makeStdioServer()
        let (vm, mock) = makeViewModel(servers: [server])
        await vm.loadData()

        mock.errorToThrow = MockError.networkFailure

        await vm.duplicateServer(server)

        XCTAssertNotNil(vm.error)
    }

    // =========================================================================
    // MARK: - 8  Form Validation (computed properties)
    // =========================================================================

    func testCanCreateServer_validStdio() {
        let (vm, _) = makeViewModel()
        vm.newServerName = "test"
        vm.newServerType = .stdio
        vm.newServerCommand = "/bin/test"

        XCTAssertTrue(vm.canCreateServer)
    }

    func testCanCreateServer_invalidEmptyName() {
        let (vm, _) = makeViewModel()
        vm.newServerName = ""
        vm.newServerType = .stdio
        vm.newServerCommand = "/bin/test"

        XCTAssertFalse(vm.canCreateServer)
    }

    func testCanCreateServer_invalidWhileCreating() {
        let (vm, _) = makeViewModel()
        vm.newServerName = "test"
        vm.newServerType = .builtin
        vm.isCreating = true

        XCTAssertFalse(vm.canCreateServer)
    }

    func testCanSaveEdit_validSSE() {
        let (vm, _) = makeViewModel()
        vm.editName = "test"
        vm.editType = .sse
        vm.editURL = "https://example.com"

        XCTAssertTrue(vm.canSaveEdit)
    }

    func testCanSaveEdit_invalidWhileEditing() {
        let (vm, _) = makeViewModel()
        vm.editName = "test"
        vm.editType = .builtin
        vm.isEditing = true

        XCTAssertFalse(vm.canSaveEdit)
    }

    // MARK: populateEditFields

    func testPopulateEditFields() {
        let server = TestFixtures.makeStdioServer(
            name: "my-server",
            command: "/usr/bin/mcp",
            args: ["--v"],
            env: ["A": "B"])
        let (vm, _) = makeViewModel()

        vm.populateEditFields(from: server)

        XCTAssertEqual(vm.editName, "my-server")
        XCTAssertEqual(vm.editType, .stdio)
        XCTAssertEqual(vm.editCommand, "/usr/bin/mcp")
        XCTAssertEqual(vm.editArgs, ["--v"])
        XCTAssertEqual(vm.editEnv, ["A": "B"])
        XCTAssertTrue(vm.editEnabled)
    }

    // MARK: filteredServers (instance computed property)

    func testFilteredServersComputedProperty() async {
        let servers = TestFixtures.makeMixedServerList()
        let (vm, _) = makeViewModel(servers: servers)
        await vm.loadData()

        vm.typeFilter = .http
        XCTAssertEqual(vm.filteredServers.count, 1)
        XCTAssertEqual(vm.filteredServers.first?.type, "http")

        vm.typeFilter = nil
        XCTAssertEqual(vm.filteredServers.count, servers.count)
    }
}
