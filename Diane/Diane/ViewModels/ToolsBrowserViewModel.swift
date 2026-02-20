import Foundation
import os.log

private let logger = Logger(subsystem: "com.diane.Diane", category: "ToolsBrowser")

/// Drives the Tools Browser tab — loads tool list from the daemon,
/// manages filtering by search text and server name.
@MainActor @Observable
final class ToolsBrowserViewModel {

    // MARK: - Published State

    var tools: [ToolInfo] = []
    var isLoading = true
    var error: String?
    var searchText = ""
    var selectedServer: String?

    // MARK: - Dependencies

    private let client: DianeClientProtocol

    init(client: DianeClientProtocol = DianeClient()) {
        self.client = client
    }

    // MARK: - Computed Properties

    /// Distinct server names sorted alphabetically.
    var servers: [String] {
        ToolsBrowserViewModel.servers(from: tools)
    }

    /// Tools filtered by selected server and search text.
    var filteredTools: [ToolInfo] {
        ToolsBrowserViewModel.filteredTools(tools, searchText: searchText, selectedServer: selectedServer)
    }

    /// Filtered tools grouped by server, sorted alphabetically.
    var groupedTools: [(server: String, tools: [ToolInfo])] {
        ToolsBrowserViewModel.groupedTools(from: filteredTools)
    }

    // MARK: - Async Operations

    func loadTools() async {
        isLoading = true
        error = nil
        FileLogger.shared.info("Loading tools...", category: "ToolsBrowser")

        do {
            tools = try await client.getTools()
            FileLogger.shared.info("Loaded \(tools.count) tools", category: "ToolsBrowser")
        } catch {
            self.error = "Failed to load tools: \(error)"
            FileLogger.shared.error("Failed to load tools: \(error.localizedDescription)", category: "ToolsBrowser")
        }

        isLoading = false
    }

    // MARK: - Pure / Static Helpers

    /// Extract distinct server names from a tool list, sorted alphabetically.
    static func servers(from tools: [ToolInfo]) -> [String] {
        Array(Set(tools.map { $0.server })).sorted()
    }

    /// Filter tools by optional server name and search text.
    static func filteredTools(
        _ tools: [ToolInfo],
        searchText: String,
        selectedServer: String?
    ) -> [ToolInfo] {
        var result = tools

        if let server = selectedServer {
            result = result.filter { $0.server == server }
        }

        if !searchText.isEmpty {
            result = result.filter {
                $0.name.localizedCaseInsensitiveContains(searchText) ||
                $0.description.localizedCaseInsensitiveContains(searchText)
            }
        }

        return result
    }

    /// Group tools by server, with both servers and tools within each group sorted.
    static func groupedTools(from tools: [ToolInfo]) -> [(server: String, tools: [ToolInfo])] {
        let grouped = Dictionary(grouping: tools) { $0.server }
        return grouped.keys.sorted().map { server in
            (server: server, tools: grouped[server]!.sorted { $0.name < $1.name })
        }
    }
}
