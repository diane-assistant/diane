import Foundation
import Observation

/// ViewModel for MCPServersView that owns all MCP-server state and business logic.
///
/// Accepts `DianeClientProtocol` via its initializer so tests can inject a mock.
/// Uses the `@Observable` macro (requires macOS 14+/iOS 17+) so SwiftUI views
/// automatically track property changes without explicit `@Published` wrappers.
@MainActor
@Observable
final class MCPServersViewModel {

    // MARK: - Dependencies

    /// The client used for all daemon communication.
    /// Not observed — it's a stable reference, not reactive state.
    @ObservationIgnored
    let client: DianeClientProtocol

    // MARK: - Server List State

    var servers: [MCPServer] = []
    var selectedServer: MCPServer?
    var isLoading = true
    var error: String?

    // MARK: - Type Filter

    var typeFilter: MCPServerType?

    // MARK: - Create Server Form State

    var showCreateServer = false
    var newServerName = ""
    var newServerType: MCPServerType = .stdio
    var newServerEnabled = true
    var newServerCommand = ""
    var newServerArgs: [String] = []
    var newServerEnv: [String: String] = [:]
    var newServerURL = ""
    var newServerHeaders: [String: String] = [:]
    var newServerOAuth: OAuthConfig?
    var newServerNodeID = ""
    var newServerNodeMode: MCPNodeMode = .master
    var isCreating = false
    var createError: String?

    // MARK: - Edit Server Form State

    var editingServer: MCPServer?
    var editName = ""
    var editType: MCPServerType = .stdio
    var editEnabled = true
    var editCommand = ""
    var editArgs: [String] = []
    var editEnv: [String: String] = [:]
    var editURL = ""
    var editHeaders: [String: String] = [:]
    var editOAuth: OAuthConfig?
    var editNodeID = ""
    var editNodeMode: MCPNodeMode = .master
    var isEditing = false
    var editError: String?
    
    // MARK: - Slave/Node State
    
    var availableSlaves: [SlaveInfo] = []
    var isLoadingSlaves = false

    // MARK: - Delete Confirmation State

    var showDeleteConfirm = false
    var serverToDelete: MCPServer?

    // MARK: - Init

    init(client: DianeClientProtocol = DianeClient()) {
        self.client = client
    }

    // MARK: - Computed Properties

    var filteredServers: [MCPServer] {
        Self.filteredServers(servers, byType: typeFilter)
    }

    /// Whether the create-server form is valid and ready for submission.
    var canCreateServer: Bool {
        Self.isServerFormValid(
            name: newServerName,
            type: newServerType,
            command: newServerCommand,
            url: newServerURL,
            isBusy: isCreating
        )
    }

    /// Whether the edit-server form is valid and ready for submission.
    var canSaveEdit: Bool {
        Self.isServerFormValid(
            name: editName,
            type: editType,
            command: editCommand,
            url: editURL,
            isBusy: isEditing
        )
    }

    // MARK: - Data Operations

    func loadData() async {
        isLoading = true
        error = nil

        do {
            servers = try await client.getMCPServerConfigs()
            // Select first server if none selected
            if selectedServer == nil, let first = servers.first {
                selectedServer = first
            }
        } catch {
            self.error = error.localizedDescription
        }

        isLoading = false
    }

    func createServer() async {
        isCreating = true
        createError = nil

        do {
            let command = newServerType == .stdio ? (newServerCommand.isEmpty ? nil : newServerCommand) : nil
            let url = (newServerType == .sse || newServerType == .http) ? (newServerURL.isEmpty ? nil : newServerURL) : nil
            let nodeID = newServerNodeMode == .specific ? (newServerNodeID.isEmpty ? nil : newServerNodeID) : nil

            let server = try await client.createMCPServerConfig(
                name: newServerName,
                type: newServerType.rawValue,
                enabled: newServerEnabled,
                command: command,
                args: newServerArgs.isEmpty ? nil : newServerArgs,
                env: newServerEnv.isEmpty ? nil : newServerEnv,
                url: url,
                headers: newServerHeaders.isEmpty ? nil : newServerHeaders,
                oauth: newServerOAuth,
                nodeID: nodeID,
                nodeMode: newServerNodeMode.rawValue
            )

            servers.append(server)
            selectedServer = server
            showCreateServer = false

            // Reset form
            resetCreateForm()
        } catch {
            createError = error.localizedDescription
        }

        isCreating = false
    }

    func updateServer(_ server: MCPServer) async {
        isEditing = true
        editError = nil

        do {
            let command = editType == .stdio ? (editCommand.isEmpty ? nil : editCommand) : nil
            let url = (editType == .sse || editType == .http) ? (editURL.isEmpty ? nil : editURL) : nil
            let nodeID = editNodeMode == .specific ? (editNodeID.isEmpty ? nil : editNodeID) : nil

            let updatedServer = try await client.updateMCPServerConfig(
                id: server.id,
                name: editName,
                type: nil,
                enabled: editEnabled,
                command: command,
                args: editArgs.isEmpty ? nil : editArgs,
                env: editEnv.isEmpty ? nil : editEnv,
                url: url,
                headers: editHeaders.isEmpty ? nil : editHeaders,
                oauth: editOAuth,
                nodeID: nodeID,
                nodeMode: editNodeMode.rawValue
            )

            // Update in list
            if let index = servers.firstIndex(where: { $0.id == server.id }) {
                servers[index] = updatedServer
            }
            selectedServer = updatedServer
            editingServer = nil
        } catch {
            editError = error.localizedDescription
        }

        isEditing = false
    }

    func deleteServer(_ server: MCPServer) async {
        do {
            try await client.deleteMCPServerConfig(id: server.id)
            servers.removeAll { $0.id == server.id }
            if selectedServer?.id == server.id {
                selectedServer = servers.first
            }
        } catch {
            self.error = error.localizedDescription
        }
    }

    func duplicateServer(_ server: MCPServer) async {
        do {
            // Generate a new name with number suffix
            let newName = generateDuplicateName(from: server.name)

            // Create new server with all the same configuration
            let duplicatedServer = try await client.createMCPServerConfig(
                name: newName,
                type: server.type,
                enabled: server.enabled,
                command: server.command,
                args: server.args,
                env: server.env,
                url: server.url,
                headers: server.headers,
                oauth: server.oauth,
                nodeID: server.nodeID,
                nodeMode: server.nodeMode
            )

            // Add to list and select it
            servers.append(duplicatedServer)
            selectedServer = duplicatedServer
        } catch {
            self.error = error.localizedDescription
        }
    }
    
    func loadSlaves() async {
        isLoadingSlaves = true
        do {
            availableSlaves = try await client.getSlaves()
        } catch {
            // Silently fail - slaves are optional
        }
        isLoadingSlaves = false
    }

    // MARK: - Edit Helpers

    func populateEditFields(from server: MCPServer) {
        editName = server.name
        editType = MCPServerType(rawValue: server.type) ?? .stdio
        editEnabled = server.enabled
        editCommand = server.command ?? ""
        editArgs = server.args ?? []
        editEnv = server.env ?? [:]
        editURL = server.url ?? ""
        editHeaders = server.headers ?? [:]
        editNodeID = server.nodeID ?? ""
        editNodeMode = MCPNodeMode(rawValue: server.nodeMode ?? "master") ?? .master
        editOAuth = server.oauth
    }

    // MARK: - Pure Logic

    /// Generate a duplicate name from an existing server name.
    /// Delegates to the static pure function with current server names.
    func generateDuplicateName(from name: String) -> String {
        let existingNames = servers.map(\.name)
        return Self.generateDuplicateName(from: name, existingNames: existingNames)
    }

    // MARK: - Static Pure Functions (testable without ViewModel instance)

    /// Filter servers by type. Returns all servers when `type` is nil.
    static func filteredServers(_ servers: [MCPServer], byType type: MCPServerType?) -> [MCPServer] {
        guard let type = type else { return servers }
        return servers.filter { $0.type == type.rawValue }
    }

    /// Validate a server form: name must be non-empty, not busy, and
    /// type-specific required fields must be filled.
    static func isServerFormValid(
        name: String,
        type: MCPServerType,
        command: String,
        url: String,
        isBusy: Bool
    ) -> Bool {
        guard !name.isEmpty, !isBusy else { return false }
        switch type {
        case .stdio:
            return !command.isEmpty
        case .sse, .http:
            return !url.isEmpty
        case .builtin:
            return true
        }
    }

    /// Generate a duplicate name from an existing server name.
    ///
    /// Pure function — depends only on `name` and `existingNames`.
    ///
    /// Rules:
    /// - If the name already ends with `(N)`, increments to `(N+1)`.
    /// - Otherwise appends `(2)`, unless that already exists — in which case
    ///   it finds the highest existing suffix and increments.
    static func generateDuplicateName(from name: String, existingNames: [String]) -> String {
        // Check if name already ends with a number in parentheses like "server (2)"
        let pattern = #"^(.*?)\s*\((\d+)\)$"#
        if let regex = try? NSRegularExpression(pattern: pattern),
           let match = regex.firstMatch(in: name, range: NSRange(name.startIndex..., in: name)) {
            // Extract base name and number
            if let baseRange = Range(match.range(at: 1), in: name),
               let numberRange = Range(match.range(at: 2), in: name),
               let currentNumber = Int(name[numberRange]) {
                let baseName = String(name[baseRange])
                return "\(baseName) (\(currentNumber + 1))"
            }
        }

        // No number suffix found, check if "name (2)" already exists
        let candidateName = "\(name) (2)"
        let nameExists = existingNames.contains(candidateName)

        if nameExists {
            // Find the highest number
            var highestNumber = 2
            for existingName in existingNames {
                if existingName.hasPrefix("\(name) (") {
                    let serverPattern = #"^.*?\s*\((\d+)\)$"#
                    if let regex = try? NSRegularExpression(pattern: serverPattern),
                       let match = regex.firstMatch(in: existingName, range: NSRange(existingName.startIndex..., in: existingName)),
                       let numberRange = Range(match.range(at: 1), in: existingName),
                       let number = Int(existingName[numberRange]) {
                        highestNumber = max(highestNumber, number)
                    }
                }
            }
            return "\(name) (\(highestNumber + 1))"
        }

        return candidateName
    }

    // MARK: - Private Helpers

    private func resetCreateForm() {
        newServerName = ""
        newServerType = .stdio
        newServerEnabled = true
        newServerCommand = ""
        newServerArgs = []
        newServerEnv = [:]
        newServerURL = ""
        newServerHeaders = [:]
        newServerOAuth = nil
        newServerNodeID = ""
        newServerNodeMode = .master
    }
}
