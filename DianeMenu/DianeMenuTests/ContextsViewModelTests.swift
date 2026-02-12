import XCTest
@testable import DianeMenu

/// Comprehensive tests for `ContextsViewModel`.
///
/// Uses `MockDianeClient` injected via the ViewModel's initializer so no
/// network calls are made. All tests run on `@MainActor` because the
/// ViewModel itself is `@MainActor`.
@MainActor
final class ContextsViewModelTests: XCTestCase {

    private var mock: MockDianeClient!
    private var vm: ContextsViewModel!

    override func setUp() {
        super.setUp()
        mock = MockDianeClient()
        vm = ContextsViewModel(client: mock)
    }

    override func tearDown() {
        vm = nil
        mock = nil
        super.tearDown()
    }

    // MARK: - Helpers

    /// Populate mock with a standard context list and matching details.
    private func seedContexts() {
        let contexts = TestFixtures.makeContextList()
        mock.contextsList = contexts
        for ctx in contexts {
            mock.contextDetails[ctx.name] = TestFixtures.makeContextDetail(
                context: ctx,
                servers: TestFixtures.makeContextServerList(),
                summary: TestFixtures.makeContextSummary()
            )
        }
    }

    // MARK: - loadContexts

    func testLoadContexts_populatesListAndAutoSelectsFirst() async {
        seedContexts()

        await vm.loadContexts()

        XCTAssertEqual(vm.contexts.count, 3)
        XCTAssertEqual(vm.selectedContext?.name, "default")
        XCTAssertNotNil(vm.contextDetail)
        XCTAssertFalse(vm.isLoading)
        XCTAssertNil(vm.error)
        XCTAssertEqual(mock.callCount(for: "getContexts"), 1)
        XCTAssertEqual(mock.callCount(for: "getContextDetail"), 1)
    }

    func testLoadContexts_emptyList() async {
        mock.contextsList = []

        await vm.loadContexts()

        XCTAssertTrue(vm.contexts.isEmpty)
        XCTAssertNil(vm.selectedContext)
        XCTAssertNil(vm.contextDetail)
        XCTAssertFalse(vm.isLoading)
        XCTAssertNil(vm.error)
    }

    func testLoadContexts_preservesExistingSelection() async {
        seedContexts()

        // Pre-select the second context
        vm.selectedContext = mock.contextsList[1]

        await vm.loadContexts()

        // Should NOT auto-select first because selectedContext was already set
        XCTAssertEqual(vm.selectedContext?.name, "work")
        // getContextDetail should NOT be called because selectedContext was already set
        XCTAssertEqual(mock.callCount(for: "getContextDetail"), 0)
    }

    func testLoadContexts_error() async {
        mock.errorToThrow = MockError.networkFailure

        await vm.loadContexts()

        XCTAssertTrue(vm.contexts.isEmpty)
        XCTAssertNotNil(vm.error)
        XCTAssertTrue(vm.error!.contains("Network failure"))
        XCTAssertFalse(vm.isLoading)
    }

    func testLoadContexts_setsIsLoadingDuringOperation() async {
        // isLoading should be false after completion (we can only check final state in unit tests)
        seedContexts()
        XCTAssertTrue(vm.isLoading) // default initial value

        await vm.loadContexts()

        XCTAssertFalse(vm.isLoading)
    }

    // MARK: - loadContextDetail

    func testLoadContextDetail_success() async {
        let ctx = TestFixtures.makeContext()
        mock.contextDetails["default"] = TestFixtures.makeContextDetail(context: ctx)

        await vm.loadContextDetail("default")

        XCTAssertNotNil(vm.contextDetail)
        XCTAssertEqual(vm.contextDetail?.context.name, "default")
        XCTAssertEqual(mock.callCount(for: "getContextDetail"), 1)
    }

    func testLoadContextDetail_failure_clearsDetail() async {
        vm.contextDetail = TestFixtures.makeContextDetail()
        mock.errorToThrow = MockError.networkFailure

        await vm.loadContextDetail("nonexistent")

        XCTAssertNil(vm.contextDetail)
    }

    // MARK: - loadConnectionInfo

    func testLoadConnectionInfo_success() async {
        let info = TestFixtures.makeContextConnectInfo(context: "default")
        mock.contextConnectInfoMap["default"] = info

        await vm.loadConnectionInfo("default")

        XCTAssertNotNil(vm.connectionInfo)
        XCTAssertEqual(vm.connectionInfo?.context, "default")
        XCTAssertEqual(mock.callCount(for: "getContextConnectInfo"), 1)
    }

    func testLoadConnectionInfo_failure_leavesNil() async {
        mock.errorToThrow = MockError.networkFailure

        await vm.loadConnectionInfo("bad")

        XCTAssertNil(vm.connectionInfo)
    }

    func testLoadConnectionInfo_clearsOldInfoFirst() async {
        vm.connectionInfo = TestFixtures.makeContextConnectInfo(context: "old")
        mock.errorToThrow = MockError.networkFailure

        await vm.loadConnectionInfo("bad")

        // Should be nil because the fetch failed and old info was cleared
        XCTAssertNil(vm.connectionInfo)
    }

    // MARK: - syncTools

    func testSyncTools_success_reloadsDetail() async {
        mock.syncedToolsCount = 5
        let ctx = TestFixtures.makeContext()
        mock.contextDetails["default"] = TestFixtures.makeContextDetail(context: ctx)

        await vm.syncTools("default")

        XCTAssertEqual(mock.callCount(for: "syncContextTools"), 1)
        XCTAssertEqual(mock.callCount(for: "getContextDetail"), 1)
        XCTAssertNotNil(vm.contextDetail)
    }

    func testSyncTools_failure_silentlyFails() async {
        mock.errorToThrow = MockError.networkFailure

        await vm.syncTools("default")

        XCTAssertEqual(mock.callCount(for: "syncContextTools"), 1)
        XCTAssertEqual(mock.callCount(for: "getContextDetail"), 0) // should not reach reload
    }

    // MARK: - createContext

    func testCreateContext_success() async {
        seedContexts()
        vm.newContextName = "My New Context"
        vm.newContextDescription = "A new context"

        await vm.createContext()

        // Name should be normalized: lowercased, spaces -> dashes
        XCTAssertEqual(mock.lastCreateContextArgs?.name, "my-new-context")
        XCTAssertEqual(mock.lastCreateContextArgs?.description, "A new context")
        XCTAssertFalse(vm.isCreating)
        XCTAssertNil(vm.createError)
        XCTAssertFalse(vm.showCreateContext)
        // Form should be reset
        XCTAssertEqual(vm.newContextName, "")
        XCTAssertEqual(vm.newContextDescription, "")
        // Selection should be the new context
        XCTAssertEqual(vm.selectedContext?.name, "my-new-context")
        // getContexts called to refresh list + getContextDetail for the new context
        XCTAssertEqual(mock.callCount(for: "createContext"), 1)
        XCTAssertGreaterThanOrEqual(mock.callCount(for: "getContexts"), 1)
    }

    func testCreateContext_emptyDescription_passesNil() async {
        mock.contextsList = []
        vm.newContextName = "test"
        vm.newContextDescription = ""

        await vm.createContext()

        XCTAssertNil(mock.lastCreateContextArgs?.description)
    }

    func testCreateContext_error() async {
        mock.errorToThrow = MockError.serverError("Name taken")
        vm.newContextName = "dup"

        await vm.createContext()

        XCTAssertNotNil(vm.createError)
        XCTAssertTrue(vm.createError!.contains("Name taken"))
        XCTAssertFalse(vm.isCreating)
        // showCreateContext should remain unchanged (not dismissed on error)
    }

    func testCreateContext_nameNormalization() async {
        mock.contextsList = []
        vm.newContextName = "My Work Context"

        await vm.createContext()

        XCTAssertEqual(mock.lastCreateContextArgs?.name, "my-work-context")
    }

    // MARK: - setDefaultContext

    func testSetDefaultContext_success() async {
        seedContexts()
        let workCtx = mock.contextsList[1] // "work"
        vm.selectedContext = workCtx

        await vm.setDefaultContext(workCtx)

        XCTAssertEqual(mock.callCount(for: "setDefaultContext"), 1)
        XCTAssertEqual(mock.callCount(for: "getContexts"), 1)
        // The mock rebuilds the list with the new default
        let defaultCtx = vm.contexts.first { $0.isDefault }
        XCTAssertEqual(defaultCtx?.name, "work")
    }

    func testSetDefaultContext_updatesSelectedIfSameContext() async {
        seedContexts()
        let workCtx = mock.contextsList[1] // "work", id=2
        vm.selectedContext = workCtx

        await vm.setDefaultContext(workCtx)

        // selectedContext should be updated to the refreshed version
        XCTAssertEqual(vm.selectedContext?.id, workCtx.id)
    }

    // MARK: - deleteContext

    func testDeleteContext_removesFromList() async {
        seedContexts()
        let workCtx = mock.contextsList[1] // "work"

        await vm.deleteContext(workCtx)

        XCTAssertEqual(mock.callCount(for: "deleteContext"), 1)
        XCTAssertEqual(mock.callCount(for: "getContexts"), 1)
        XCTAssertNil(vm.contextToDelete)
    }

    func testDeleteContext_selectedContext_selectsFirst() async {
        seedContexts()
        let defaultCtx = mock.contextsList[0] // "default"
        vm.selectedContext = defaultCtx
        vm.contextDetail = TestFixtures.makeContextDetail(context: defaultCtx)

        await vm.deleteContext(defaultCtx)

        // After deleting selected context, first remaining should be selected
        // The mock removes "default", so remaining = ["work", "personal"]
        XCTAssertEqual(vm.selectedContext?.name, "work")
        // loadContextDetail should be called for the new selection
        XCTAssertGreaterThanOrEqual(mock.callCount(for: "getContextDetail"), 1)
    }

    func testDeleteContext_lastContext_clearsSelection() async {
        let onlyCtx = TestFixtures.makeContext(id: 1, name: "only")
        mock.contextsList = [onlyCtx]
        vm.selectedContext = onlyCtx
        vm.contextDetail = TestFixtures.makeContextDetail(context: onlyCtx)

        await vm.deleteContext(onlyCtx)

        // After deleting the only context, list is empty
        XCTAssertNil(vm.selectedContext)
        XCTAssertNil(vm.contextDetail)
    }

    func testDeleteContext_nonSelectedContext_preservesSelection() async {
        seedContexts()
        let defaultCtx = mock.contextsList[0]
        let workCtx = mock.contextsList[1]
        vm.selectedContext = defaultCtx

        await vm.deleteContext(workCtx)

        // Selected context should remain "default"
        XCTAssertEqual(vm.selectedContext?.name, "default")
    }

    func testDeleteContext_clearsContextToDelete() async {
        seedContexts()
        let ctx = mock.contextsList[0]
        vm.contextToDelete = ctx

        await vm.deleteContext(ctx)

        XCTAssertNil(vm.contextToDelete)
    }

    // MARK: - resetCreateForm

    func testResetCreateForm() {
        vm.newContextName = "test"
        vm.newContextDescription = "desc"
        vm.createError = "some error"

        vm.resetCreateForm()

        XCTAssertEqual(vm.newContextName, "")
        XCTAssertEqual(vm.newContextDescription, "")
        XCTAssertNil(vm.createError)
    }

    // MARK: - loadAvailableServers

    func testLoadAvailableServers_success_sortedCorrectly() async {
        seedContexts()
        vm.selectedContext = mock.contextsList[0]
        mock.availableServersList = TestFixtures.makeAvailableServerList()

        await vm.loadAvailableServers()

        XCTAssertFalse(vm.isLoadingServers)
        XCTAssertFalse(vm.availableServers.isEmpty)
        XCTAssertEqual(mock.callCount(for: "getAvailableServers"), 1)

        // Servers NOT in context should come first
        let firstNotInContext = vm.availableServers.first { !$0.inContext }
        let lastInContext = vm.availableServers.last { $0.inContext }
        if let notIn = firstNotInContext, let inCtx = lastInContext {
            let notInIndex = vm.availableServers.firstIndex(where: { $0.name == notIn.name })!
            let inCtxIndex = vm.availableServers.firstIndex(where: { $0.name == inCtx.name })!
            XCTAssertLessThan(notInIndex, inCtxIndex)
        }
    }

    func testLoadAvailableServers_noSelection_doesNothing() async {
        vm.selectedContext = nil

        await vm.loadAvailableServers()

        XCTAssertEqual(mock.callCount(for: "getAvailableServers"), 0)
        XCTAssertTrue(vm.availableServers.isEmpty)
    }

    func testLoadAvailableServers_error_emptiesList() async {
        vm.selectedContext = TestFixtures.makeContext()
        mock.errorToThrow = MockError.networkFailure

        await vm.loadAvailableServers()

        XCTAssertTrue(vm.availableServers.isEmpty)
        XCTAssertFalse(vm.isLoadingServers)
    }

    func testLoadAvailableServers_sorting_alphabeticalWithinGroups() async {
        vm.selectedContext = TestFixtures.makeContext()
        mock.availableServersList = [
            TestFixtures.makeAvailableServer(name: "zebra", inContext: false),
            TestFixtures.makeAvailableServer(name: "alpha", inContext: false),
            TestFixtures.makeAvailableServer(name: "zeta-in", inContext: true),
            TestFixtures.makeAvailableServer(name: "alpha-in", inContext: true),
        ]

        await vm.loadAvailableServers()

        let names = vm.availableServers.map(\.name)
        // Not-in-context first (alphabetical), then in-context (alphabetical)
        XCTAssertEqual(names, ["alpha", "zebra", "alpha-in", "zeta-in"])
    }

    // MARK: - addServer

    func testAddServer_success() async {
        seedContexts()
        vm.selectedContext = mock.contextsList[0] // "default"
        mock.availableServersList = TestFixtures.makeAvailableServerList()

        await vm.addServer("search-engine")

        XCTAssertEqual(mock.lastAddServerToContextArgs?.contextName, "default")
        XCTAssertEqual(mock.lastAddServerToContextArgs?.serverName, "search-engine")
        XCTAssertEqual(mock.lastAddServerToContextArgs?.enabled, true)
        XCTAssertEqual(mock.callCount(for: "addServerToContext"), 1)
        // Should reload available servers and context detail after adding
        XCTAssertEqual(mock.callCount(for: "getAvailableServers"), 1)
        XCTAssertEqual(mock.callCount(for: "getContextDetail"), 1)
    }

    func testAddServer_noSelection_doesNothing() async {
        vm.selectedContext = nil

        await vm.addServer("search-engine")

        XCTAssertEqual(mock.callCount(for: "addServerToContext"), 0)
    }

    // MARK: - onSelectContext

    func testOnSelectContext_updatesSelection() {
        let ctx = TestFixtures.makeContext(id: 5, name: "new-ctx")

        vm.onSelectContext(ctx)

        XCTAssertEqual(vm.selectedContext?.name, "new-ctx")
        XCTAssertEqual(vm.selectedContext?.id, 5)
    }

    // MARK: - prepareDeleteContext

    func testPrepareDeleteContext_setsConfirmState() {
        let ctx = TestFixtures.makeContext(id: 3, name: "to-delete")

        vm.prepareDeleteContext(ctx)

        XCTAssertEqual(vm.contextToDelete?.name, "to-delete")
        XCTAssertTrue(vm.showDeleteConfirm)
    }

    // MARK: - prepareAddServer

    func testPrepareAddServer_setsState() {
        vm.availableServers = TestFixtures.makeAvailableServerList()

        vm.prepareAddServer()

        XCTAssertTrue(vm.availableServers.isEmpty)
        XCTAssertTrue(vm.isLoadingServers)
        XCTAssertTrue(vm.showAddServer)
    }

    // MARK: - Initial State

    func testInitialState() {
        let freshVM = ContextsViewModel(client: mock)

        XCTAssertTrue(freshVM.contexts.isEmpty)
        XCTAssertNil(freshVM.selectedContext)
        XCTAssertNil(freshVM.contextDetail)
        XCTAssertTrue(freshVM.isLoading)
        XCTAssertNil(freshVM.error)
        XCTAssertFalse(freshVM.showCreateContext)
        XCTAssertEqual(freshVM.newContextName, "")
        XCTAssertEqual(freshVM.newContextDescription, "")
        XCTAssertFalse(freshVM.isCreating)
        XCTAssertNil(freshVM.createError)
        XCTAssertFalse(freshVM.showConnectionInfo)
        XCTAssertNil(freshVM.connectionInfo)
        XCTAssertFalse(freshVM.showDeleteConfirm)
        XCTAssertNil(freshVM.contextToDelete)
        XCTAssertFalse(freshVM.showAddServer)
        XCTAssertTrue(freshVM.availableServers.isEmpty)
        XCTAssertFalse(freshVM.isLoadingServers)
    }

    // MARK: - Client Injection

    func testClientInjection_usesProvidedClient() async {
        let customMock = MockDianeClient()
        let customVM = ContextsViewModel(client: customMock)
        customMock.contextsList = [TestFixtures.makeContext(id: 99, name: "injected")]

        await customVM.loadContexts()

        XCTAssertEqual(customVM.contexts.count, 1)
        XCTAssertEqual(customVM.contexts.first?.name, "injected")
        XCTAssertEqual(customMock.callCount(for: "getContexts"), 1)
    }

    // MARK: - Full Workflow Tests

    func testFullWorkflow_createThenDelete() async {
        // Start empty
        mock.contextsList = []

        // Create a context
        vm.newContextName = "Test Project"
        vm.newContextDescription = "My test project"
        await vm.createContext()

        XCTAssertEqual(vm.contexts.count, 1)
        XCTAssertEqual(vm.selectedContext?.name, "test-project")

        // Delete it
        let created = vm.selectedContext!
        await vm.deleteContext(created)

        XCTAssertTrue(vm.contexts.isEmpty)
        XCTAssertNil(vm.selectedContext)
    }

    func testFullWorkflow_loadAndSwitchContexts() async {
        seedContexts()

        await vm.loadContexts()

        XCTAssertEqual(vm.selectedContext?.name, "default")

        // Switch to "work" context
        vm.onSelectContext(vm.contexts[1])
        XCTAssertEqual(vm.selectedContext?.name, "work")

        // Load detail for work
        await vm.loadContextDetail("work")
        XCTAssertNotNil(vm.contextDetail)
        XCTAssertEqual(vm.contextDetail?.context.name, "work")
    }

    func testFullWorkflow_setDefaultAndVerify() async {
        seedContexts()
        await vm.loadContexts()

        // "default" is currently the default
        XCTAssertTrue(vm.contexts[0].isDefault)
        XCTAssertFalse(vm.contexts[1].isDefault)

        // Set "work" as default
        let workCtx = vm.contexts[1]
        await vm.setDefaultContext(workCtx)

        // Verify the list was refreshed with "work" as default
        let newDefault = vm.contexts.first { $0.isDefault }
        XCTAssertEqual(newDefault?.name, "work")
        let oldDefault = vm.contexts.first { $0.name == "default" }
        XCTAssertFalse(oldDefault?.isDefault ?? true)
    }
}
