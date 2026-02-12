import XCTest
@testable import DianeMenu

/// Tests for `ToolsBrowserViewModel` — pure functions, async data operations,
/// computed properties, and state management.
@MainActor
final class ToolsBrowserViewModelTests: XCTestCase {

    // MARK: - Helpers

    private func makeViewModel(
        tools: [ToolInfo] = []
    ) -> (ToolsBrowserViewModel, MockDianeClient) {
        let mock = MockDianeClient()
        mock.toolsList = tools
        let vm = ToolsBrowserViewModel(client: mock)
        return (vm, mock)
    }

    // =========================================================================
    // MARK: - Pure Function Tests
    // =========================================================================

    // MARK: servers(from:)

    func testServers_extractsUniqueServersSorted() {
        let tools = TestFixtures.makeToolList()
        let servers = ToolsBrowserViewModel.servers(from: tools)
        XCTAssertEqual(servers, ["builtin-tools", "filesystem", "search-engine"])
    }

    func testServers_emptyList() {
        XCTAssertEqual(ToolsBrowserViewModel.servers(from: []), [])
    }

    func testServers_singleServer() {
        let tools = [
            TestFixtures.makeTool(name: "a", server: "srv"),
            TestFixtures.makeTool(name: "b", server: "srv"),
        ]
        XCTAssertEqual(ToolsBrowserViewModel.servers(from: tools), ["srv"])
    }

    // MARK: filteredTools

    func testFilteredTools_noFilters() {
        let tools = TestFixtures.makeToolList()
        let result = ToolsBrowserViewModel.filteredTools(tools, searchText: "", selectedServer: nil)
        XCTAssertEqual(result.count, tools.count)
    }

    func testFilteredTools_filterByServer() {
        let tools = TestFixtures.makeToolList()
        let result = ToolsBrowserViewModel.filteredTools(tools, searchText: "", selectedServer: "filesystem")
        XCTAssertEqual(result.count, 3)
        XCTAssertTrue(result.allSatisfy { $0.server == "filesystem" })
    }

    func testFilteredTools_filterBySearchText() {
        let tools = TestFixtures.makeToolList()
        let result = ToolsBrowserViewModel.filteredTools(tools, searchText: "file", selectedServer: nil)
        // Matches: read_file (name), write_file (name), list_directory (no)
        // Also read_file description "Read the contents of a file", write_file description "Write content to a file"
        XCTAssertEqual(result.count, 2)
        XCTAssertTrue(result.allSatisfy { $0.name.contains("file") })
    }

    func testFilteredTools_filterByDescription() {
        let tools = TestFixtures.makeToolList()
        let result = ToolsBrowserViewModel.filteredTools(tools, searchText: "math", selectedServer: nil)
        XCTAssertEqual(result.count, 1)
        XCTAssertEqual(result.first?.name, "calculator")
    }

    func testFilteredTools_caseInsensitive() {
        let tools = TestFixtures.makeToolList()
        let result = ToolsBrowserViewModel.filteredTools(tools, searchText: "READ", selectedServer: nil)
        XCTAssertTrue(result.contains(where: { $0.name == "read_file" }))
    }

    func testFilteredTools_bothFilters() {
        let tools = TestFixtures.makeToolList()
        let result = ToolsBrowserViewModel.filteredTools(tools, searchText: "file", selectedServer: "filesystem")
        XCTAssertEqual(result.count, 2) // read_file and write_file
    }

    func testFilteredTools_noMatch() {
        let tools = TestFixtures.makeToolList()
        let result = ToolsBrowserViewModel.filteredTools(tools, searchText: "nonexistent", selectedServer: nil)
        XCTAssertTrue(result.isEmpty)
    }

    // MARK: groupedTools

    func testGroupedTools_groupsByServer() {
        let tools = TestFixtures.makeToolList()
        let grouped = ToolsBrowserViewModel.groupedTools(from: tools)
        XCTAssertEqual(grouped.count, 3)
        XCTAssertEqual(grouped.map(\.server), ["builtin-tools", "filesystem", "search-engine"])
    }

    func testGroupedTools_sortedWithinGroups() {
        let tools = TestFixtures.makeToolList()
        let grouped = ToolsBrowserViewModel.groupedTools(from: tools)

        // filesystem tools should be sorted: list_directory, read_file, write_file
        let fsGroup = grouped.first(where: { $0.server == "filesystem" })!
        XCTAssertEqual(fsGroup.tools.map(\.name), ["list_directory", "read_file", "write_file"])
    }

    func testGroupedTools_emptyList() {
        let grouped = ToolsBrowserViewModel.groupedTools(from: [])
        XCTAssertTrue(grouped.isEmpty)
    }

    // =========================================================================
    // MARK: - Async Operation Tests
    // =========================================================================

    func testLoadTools_populatesTools() async {
        let tools = TestFixtures.makeToolList()
        let (vm, _) = makeViewModel(tools: tools)

        await vm.loadTools()

        XCTAssertEqual(vm.tools.count, 7)
        XCTAssertFalse(vm.isLoading)
        XCTAssertNil(vm.error)
    }

    func testLoadTools_setsErrorOnFailure() async {
        let (vm, mock) = makeViewModel()
        mock.errorToThrow = MockError.networkFailure

        await vm.loadTools()

        XCTAssertNotNil(vm.error)
        XCTAssertTrue(vm.tools.isEmpty)
        XCTAssertFalse(vm.isLoading)
    }

    func testLoadTools_setsIsLoadingFalse() async {
        let (vm, _) = makeViewModel()
        XCTAssertTrue(vm.isLoading)

        await vm.loadTools()

        XCTAssertFalse(vm.isLoading)
    }

    func testLoadTools_callsClient() async {
        let (vm, mock) = makeViewModel()

        await vm.loadTools()

        XCTAssertEqual(mock.callCount(for: "getTools"), 1)
    }

    // =========================================================================
    // MARK: - Computed Properties (instance)
    // =========================================================================

    func testServersComputedProperty() async {
        let tools = TestFixtures.makeToolList()
        let (vm, _) = makeViewModel(tools: tools)
        await vm.loadTools()

        XCTAssertEqual(vm.servers, ["builtin-tools", "filesystem", "search-engine"])
    }

    func testFilteredToolsComputedProperty() async {
        let tools = TestFixtures.makeToolList()
        let (vm, _) = makeViewModel(tools: tools)
        await vm.loadTools()

        vm.searchText = "file"
        XCTAssertEqual(vm.filteredTools.count, 2)

        vm.searchText = ""
        XCTAssertEqual(vm.filteredTools.count, 7)

        vm.selectedServer = "search-engine"
        XCTAssertEqual(vm.filteredTools.count, 2)

        vm.selectedServer = nil
        XCTAssertEqual(vm.filteredTools.count, 7)
    }

    func testGroupedToolsComputedProperty() async {
        let tools = TestFixtures.makeToolList()
        let (vm, _) = makeViewModel(tools: tools)
        await vm.loadTools()

        XCTAssertEqual(vm.groupedTools.count, 3)

        vm.selectedServer = "filesystem"
        XCTAssertEqual(vm.groupedTools.count, 1)
        XCTAssertEqual(vm.groupedTools.first?.server, "filesystem")
    }
}
