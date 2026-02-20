import Foundation
import Observation
import os.log

private let logger = Logger(subsystem: "com.diane.Diane", category: "MCPRegistry")

/// ViewModel for MCPRegistryView that manages MCP server definitions (not placements).
///
/// This is the "registry" view of MCPs - focused on defining what servers exist,
/// their configuration, and type-specific settings. Does NOT handle per-node
/// deployment (that's MCPServersViewModel's job).
///
/// Uses `DianeClientProtocol` for dependency injection and `@Observable` macro
/// for automatic SwiftUI reactivity.
@MainActor
@Observable
final class MCPRegistryViewModel {

    // MARK: - Dependencies

    /// The client used for all daemon communication.
    @ObservationIgnored
    let client: DianeClientProtocol

    // MARK: - Server List State

    var servers: [MCPServer] = []
    var placements: [MCPServerPlacement] = []
    var contexts: [Context] = []
    var contextServers: [String: [ContextServer]] = [:] // contextName -> list of servers
    var selectedServer: MCPServer?
    var isLoading = true
    var error: String?

    // MARK: - Type Filter

    var typeFilter: MCPServerType?

    // MARK: - Create Server Form State

    var showCreateServer = false
    var newServerName = ""
    var newServerType: MCPServerType = .stdio
    var newServerCommand = ""
    var newServerArgs: [String] = []
    var newServerEnv: [String: String] = [:]
    var newServerURL = ""
    var newServerHeaders: [String: String] = [:]
    var newServerOAuth: OAuthConfig?
    var isCreating = false
    var createError: String?

    // MARK: - Edit Server Form State

    var editingServer: MCPServer?
    var editName = ""
    var editType: MCPServerType = .stdio
    var editCommand = ""
    var editArgs: [String] = []
    var editEnv: [String: String] = [:]
    var editURL = ""
    var editHeaders: [String: String] = [:]
    var editOAuth: OAuthConfig?
    var isEditing = false
    var editError: String?

    // MARK: - Delete Confirmation State

    var showDeleteConfirm = false
    var serverToDelete: MCPServer?

    // MARK: - Init

    init(client: DianeClientProtocol = DianeClient.shared) {
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
        
        FileLogger.shared.info("Loading MCP registry data...", category: "MCPRegistry")

        do {
            async let fetchedServers = client.getMCPServerConfigs()
            async let fetchedPlacements = client.getPlacements(hostID: "master")
            async let fetchedContexts = client.getContexts()
            
            let (s, p, c) = try await (fetchedServers, fetchedPlacements, fetchedContexts)
            
            servers = s
            placements = p
            contexts = c
            
            FileLogger.shared.info("Loaded \(s.count) servers, \(p.count) placements, \(c.count) contexts", category: "MCPRegistry")
            
            // Load context servers for each context
            var contextServerMap: [String: [ContextServer]] = [:]
            for context in c {
                if let servers = try? await client.getContextServers(contextName: context.name) {
                    contextServerMap[context.name] = servers
                }
            }
            contextServers = contextServerMap
            
            // Select first server if none selected
            if selectedServer == nil, let first = servers.first {
                selectedServer = first
            }
        } catch {
            self.error = error.localizedDescription
            FileLogger.shared.error("Failed to load MCP registry: \(error.localizedDescription)", category: "MCPRegistry")
            logger.error("Failed to load MCP registry: \(error.localizedDescription)")
        }

        isLoading = false
    }

    /// Toggle a server's enabled state on the master node.
    func toggleServer(_ server: MCPServer, enabled: Bool) async {
        FileLogger.shared.info("Toggling server '\(server.name)' (id:\(server.id)) enabled=\(enabled)", category: "MCPRegistry")
        // Optimistic update
        let oldEnabled = isServerEnabled(server)
        updateLocalPlacement(serverID: server.id, enabled: enabled)
        
        do {
            let updatedPlacement = try await client.updatePlacement(
                serverID: server.id,
                hostID: "master",
                enabled: enabled
            )
            
            // Confirm update
            if let index = placements.firstIndex(where: { $0.serverID == server.id }) {
                placements[index] = updatedPlacement
            } else {
                placements.append(updatedPlacement)
            }
            
            // If enabling, also ensure it's in the default context
            if enabled {
                try? await client.addServerToContext(contextName: "personal", serverName: server.name, enabled: true)
            }
            FileLogger.shared.info("Server '\(server.name)' toggled successfully", category: "MCPRegistry")
        } catch {
            // Revert on error
            updateLocalPlacement(serverID: server.id, enabled: oldEnabled)
            self.error = error.localizedDescription
            FileLogger.shared.error("Failed to toggle server '\(server.name)': \(error.localizedDescription)", category: "MCPRegistry")
            logger.error("Failed to toggle server '\(server.name)': \(error.localizedDescription)")
        }
    }
    
    private func updateLocalPlacement(serverID: Int64, enabled: Bool) {
        if let index = placements.firstIndex(where: { $0.serverID == serverID }) {
            var p = placements[index]
            p.enabled = enabled
            placements[index] = p
        }
    }
    
    func isServerEnabled(_ server: MCPServer) -> Bool {
        placements.first(where: { $0.serverID == server.id })?.enabled ?? false
    }
    
    /// Get all contexts that contain this server
    func contextsForServer(_ server: MCPServer) -> [String] {
        var result: [String] = []
        for (contextName, servers) in contextServers {
            if servers.contains(where: { $0.name == server.name }) {
                result.append(contextName)
            }
        }
        return result.sorted()
    }
    
    /// Get all nodes (placements) where this server is deployed
    /// Currently only shows "master" since we only load master placements
    func nodesForServer(_ server: MCPServer) -> [String] {
        // For now we only show master node
        // In future when multi-node is implemented, we'd fetch all placements
        if isServerEnabled(server) {
            return ["master"]
        }
        return []
    }

    func createServer() async {
        isCreating = true
        createError = nil
        
        FileLogger.shared.info("Creating MCP server '\(newServerName)' type=\(newServerType.rawValue)", category: "MCPRegistry")

        do {
            let command = newServerType == .stdio ? (newServerCommand.isEmpty ? nil : newServerCommand) : nil
            let url = (newServerType == .sse || newServerType == .http) ? (newServerURL.isEmpty ? nil : newServerURL) : nil

            // Create the server definition
            let server = try await client.createMCPServerConfig(
                name: newServerName,
                type: newServerType.rawValue,
                enabled: false, // Global flag ignored in favor of placements
                command: command,
                args: newServerArgs.isEmpty ? nil : newServerArgs,
                env: newServerEnv.isEmpty ? nil : newServerEnv,
                url: url,
                headers: newServerHeaders.isEmpty ? nil : newServerHeaders,
                oauth: newServerOAuth,
                nodeID: nil,
                nodeMode: nil
            )

            servers.append(server)
            selectedServer = server
            
            // Automatically enable on master
            let placement = try await client.updatePlacement(
                serverID: server.id,
                hostID: "master",
                enabled: true
            )
            placements.append(placement)
            
            // Automatically add to default context
            try await client.addServerToContext(contextName: "personal", serverName: server.name, enabled: true)

            showCreateServer = false
            resetCreateForm()
            FileLogger.shared.info("Created MCP server '\(server.name)' id=\(server.id)", category: "MCPRegistry")
        } catch {
            createError = error.localizedDescription
            FileLogger.shared.error("Failed to create server '\(newServerName)': \(error.localizedDescription)", category: "MCPRegistry")
            logger.error("Failed to create server '\(self.newServerName)': \(error.localizedDescription)")
        }

        isCreating = false
    }

    func updateServer(_ server: MCPServer) async {
        isEditing = true
        editError = nil
        
        FileLogger.shared.info("Updating MCP server '\(server.name)' (id:\(server.id))", category: "MCPRegistry")

        do {
            let command = editType == .stdio ? (editCommand.isEmpty ? nil : editCommand) : nil
            let url = (editType == .sse || editType == .http) ? (editURL.isEmpty ? nil : editURL) : nil

            let updatedServer = try await client.updateMCPServerConfig(
                id: server.id,
                name: editName,
                type: nil, // Type cannot be changed after creation
                enabled: nil, // Don't change global enabled flag from registry
                command: command,
                args: editArgs.isEmpty ? nil : editArgs,
                env: editEnv.isEmpty ? nil : editEnv,
                url: url,
                headers: editHeaders.isEmpty ? nil : editHeaders,
                oauth: editOAuth,
                nodeID: nil, // Deprecated
                nodeMode: nil // Deprecated
            )

            // Update in list
            if let index = servers.firstIndex(where: { $0.id == server.id }) {
                servers[index] = updatedServer
            }
            selectedServer = updatedServer
            editingServer = nil
            FileLogger.shared.info("Updated MCP server '\(updatedServer.name)' successfully", category: "MCPRegistry")
        } catch {
            editError = error.localizedDescription
            FileLogger.shared.error("Failed to update server '\(server.name)': \(error.localizedDescription)", category: "MCPRegistry")
            logger.error("Failed to update server '\(server.name)': \(error.localizedDescription)")
        }

        isEditing = false
    }

    func deleteServer(_ server: MCPServer) async {
        FileLogger.shared.info("Deleting MCP server '\(server.name)' (id:\(server.id))", category: "MCPRegistry")
        do {
            try await client.deleteMCPServerConfig(id: server.id)
            servers.removeAll { $0.id == server.id }
            if selectedServer?.id == server.id {
                selectedServer = servers.first
            }
            FileLogger.shared.info("Deleted MCP server '\(server.name)' successfully", category: "MCPRegistry")
        } catch {
            self.error = error.localizedDescription
            FileLogger.shared.error("Failed to delete server '\(server.name)': \(error.localizedDescription)", category: "MCPRegistry")
            logger.error("Failed to delete server '\(server.name)': \(error.localizedDescription)")
        }
    }

    func duplicateServer(_ server: MCPServer) async {
        FileLogger.shared.info("Duplicating MCP server '\(server.name)'", category: "MCPRegistry")
        do {
            // Generate a new name with number suffix
            let newName = generateDuplicateName(from: server.name)

            // Create new server with all the same configuration
            // Note: Duplicates are also created disabled by default
            let duplicatedServer = try await client.createMCPServerConfig(
                name: newName,
                type: server.type,
                enabled: false, // Secure by default
                command: server.command,
                args: server.args,
                env: server.env,
                url: server.url,
                headers: server.headers,
                oauth: server.oauth,
                nodeID: nil, // Deprecated
                nodeMode: nil // Deprecated
            )

            // Add to list and select it
            servers.append(duplicatedServer)
            selectedServer = duplicatedServer
            FileLogger.shared.info("Duplicated server '\(server.name)' -> '\(duplicatedServer.name)'", category: "MCPRegistry")
        } catch {
            self.error = error.localizedDescription
            FileLogger.shared.error("Failed to duplicate server '\(server.name)': \(error.localizedDescription)", category: "MCPRegistry")
        }
    }

    // MARK: - Edit Helpers

    func populateEditFields(from server: MCPServer) {
        editName = server.name
        editType = MCPServerType(rawValue: server.type) ?? .stdio
        editCommand = server.command ?? ""
        editArgs = server.args ?? []
        editEnv = server.env ?? [:]
        editURL = server.url ?? ""
        editHeaders = server.headers ?? [:]
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
        newServerCommand = ""
        newServerArgs = []
        newServerEnv = [:]
        newServerURL = ""
        newServerHeaders = [:]
        newServerOAuth = nil
    }
}
