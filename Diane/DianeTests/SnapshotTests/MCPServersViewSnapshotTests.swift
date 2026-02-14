import XCTest
import SwiftUI
import SnapshotTesting
@testable import Diane

/// Snapshot tests for MCPServersView in various states.
///
/// On the **first run** these tests will always fail because no reference
/// images exist yet.  The library writes the reference images to disk
/// automatically.  Re-run the tests and they should pass.
///
/// To regenerate baselines, set `isRecording = true` in `setUp()` or on
/// individual tests via `withSnapshotTesting(record: .all) { ... }`.
@MainActor
final class MCPServersViewSnapshotTests: XCTestCase {

    // Common snapshot size – matches the view's idealWidth/minHeight.
    private let defaultSize = CGSize(width: 800, height: 600)
    private let compactSize = CGSize(width: 700, height: 400)

    // MARK: - Helpers

    /// Wraps a SwiftUI view in an `NSHostingController` ready for snapshotting.
    private func hostingController<V: View>(for view: V, size: CGSize? = nil) -> NSHostingController<V> {
        let hc = NSHostingController(rootView: view)
        let s = size ?? defaultSize
        hc.view.frame = CGRect(origin: .zero, size: s)
        return hc
    }

    /// Creates an MCPServersViewModel pre-loaded with given servers (skips network).
    private func makeViewModel(
        servers: [MCPServer] = [],
        isLoading: Bool = false,
        error: String? = nil,
        selectedServer: MCPServer? = nil
    ) -> MCPServersViewModel {
        let mock = MockDianeClient()
        mock.serverConfigs = servers
        let vm = MCPServersViewModel(client: mock)
        vm.servers = servers
        vm.isLoading = isLoading
        vm.error = error
        vm.selectedServer = selectedServer
        return vm
    }

    // MARK: - Loading State

    func testLoadingState() {
        let vm = makeViewModel(isLoading: true)
        let view = MCPServersView(viewModel: vm)
        let hc = hostingController(for: view)

        assertSnapshot(of: hc, as: .image(size: defaultSize))
    }

    // MARK: - Empty State

    func testEmptyState() {
        let vm = makeViewModel(servers: [])
        let view = MCPServersView(viewModel: vm)
        let hc = hostingController(for: view)

        assertSnapshot(of: hc, as: .image(size: defaultSize))
    }

    // MARK: - Error State

    func testErrorState() {
        let vm = makeViewModel(error: "Failed to connect to daemon")
        let view = MCPServersView(viewModel: vm)
        let hc = hostingController(for: view)

        assertSnapshot(of: hc, as: .image(size: defaultSize))
    }

    // MARK: - Loaded With Servers

    func testLoadedWithMixedServers() {
        let servers = [
            TestFixtures.makeStdioServer(id: 1, name: "filesystem", enabled: true),
            TestFixtures.makeSSEServer(id: 2, name: "web-search", enabled: true),
            TestFixtures.makeHTTPServer(id: 3, name: "database", enabled: false),
            TestFixtures.makeBuiltinServer(id: 4, name: "built-in-tools", enabled: true),
        ]
        let vm = makeViewModel(servers: servers)
        let view = MCPServersView(viewModel: vm)
        let hc = hostingController(for: view)

        assertSnapshot(of: hc, as: .image(size: defaultSize))
    }

    // MARK: - Selected Server Detail

    func testSelectedStdioServer() {
        let server = TestFixtures.makeStdioServer(
            id: 1,
            name: "filesystem",
            enabled: true,
            command: "/usr/local/bin/mcp-filesystem",
            args: ["--root", "/home/user"]
        )
        let vm = makeViewModel(servers: [server], selectedServer: server)
        let view = MCPServersView(viewModel: vm)
        let hc = hostingController(for: view)

        assertSnapshot(of: hc, as: .image(size: defaultSize))
    }

    func testSelectedSSEServer() {
        let server = TestFixtures.makeSSEServer(
            id: 2,
            name: "remote-api",
            enabled: true,
            url: "https://api.example.com/mcp/sse"
        )
        let vm = makeViewModel(servers: [server], selectedServer: server)
        let view = MCPServersView(viewModel: vm)
        let hc = hostingController(for: view)

        assertSnapshot(of: hc, as: .image(size: defaultSize))
    }

    func testSelectedDisabledServer() {
        let server = TestFixtures.makeStdioServer(id: 1, name: "disabled-server", enabled: false)
        let vm = makeViewModel(servers: [server], selectedServer: server)
        let view = MCPServersView(viewModel: vm)
        let hc = hostingController(for: view)

        assertSnapshot(of: hc, as: .image(size: defaultSize))
    }

    // MARK: - Compact Size

    func testLoadedCompactSize() {
        let servers = [
            TestFixtures.makeStdioServer(id: 1, name: "server-a"),
            TestFixtures.makeSSEServer(id: 2, name: "server-b"),
        ]
        let vm = makeViewModel(servers: servers)
        let view = MCPServersView(viewModel: vm)
        let hc = hostingController(for: view, size: compactSize)

        assertSnapshot(of: hc, as: .image(size: compactSize))
    }

    func testEmptyCompactSize() {
        let vm = makeViewModel(servers: [])
        let view = MCPServersView(viewModel: vm)
        let hc = hostingController(for: view, size: compactSize)

        assertSnapshot(of: hc, as: .image(size: compactSize))
    }

    // MARK: - Many Servers (scrollable list)

    func testManyServers() {
        let servers = (1...12).map { i in
            TestFixtures.makeStdioServer(
                id: Int64(i),
                name: "server-\(String(format: "%02d", i))",
                enabled: i % 3 != 0  // every 3rd disabled
            )
        }
        let vm = makeViewModel(servers: servers)
        let view = MCPServersView(viewModel: vm)
        let hc = hostingController(for: view)

        assertSnapshot(of: hc, as: .image(size: defaultSize))
    }
}
