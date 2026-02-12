import Foundation
import Combine

/// A subclass of StatusMonitor that provides static mock data for the ComponentCatalog.
///
/// Avoids real daemon communication — never calls `start()` or `startPolling()`.
/// Sets `@Published` properties to static values so catalog views render immediately.
@MainActor
final class MockStatusMonitor: StatusMonitor {

    /// Create a mock status monitor with the specified connection state.
    ///
    /// - Parameters:
    ///   - connected: Whether to simulate a connected state (default `true`).
    ///   - serverCount: Number of mock MCP servers in the status (default `3`).
    ///   - toolCount: Total tool count in the status (default `7`).
    init(connected: Bool = true, serverCount: Int = 3, toolCount: Int = 7) {
        super.init()

        if connected {
            connectionState = .connected
            status = DianeStatus(
                running: true,
                pid: 12345,
                version: "1.2.3",
                uptime: "2h 15m",
                uptimeSeconds: 8100,
                startedAt: Date().addingTimeInterval(-8100),
                totalTools: toolCount,
                mcpServers: (0..<serverCount).map { i in
                    MCPServerStatus(
                        name: "server-\(i + 1)",
                        enabled: true,
                        connected: i < serverCount - 1, // last one disconnected
                        toolCount: toolCount / max(serverCount, 1),
                        error: i == serverCount - 1 ? "Connection timeout" : nil,
                        builtin: i == 0
                    )
                }
            )
        } else {
            connectionState = .disconnected
            status = .empty
        }

        isLoading = false
        lastError = nil
    }

    // Override start/polling to be no-ops
    override func start() async {}
    override func startPolling() async {}
    override func stopPolling() {}
    override func refresh() async {}
}
