import Foundation
import Observation
import os.log

private let logger = Logger(subsystem: "com.diane.Diane", category: "MCPServers")

/// ViewModel for MCPServersView that manages node-centric MCP deployment.
///
/// This is the "deployment" view of MCPs - focused on selecting a node (master or slave)
/// and toggling which MCP servers are enabled on that node. Does NOT handle server
/// definition/configuration (that's MCPRegistryViewModel's job).
///
/// Key concepts:
/// - Each node (master + slaves) can have different MCPs enabled
/// - All MCPs default to OFF (secure by default)
/// - User explicitly toggles them ON per-node
/// - Builtins are first-class citizens that must also be enabled
///
/// Uses `DianeClientProtocol` for dependency injection and `@Observable` macro
/// for automatic SwiftUI reactivity.
@MainActor
@Observable
final class MCPServersViewModel {

    // MARK: - Dependencies

    /// The client used for all daemon communication.
    @ObservationIgnored
    let client: DianeClientProtocol

    // MARK: - Node/Host State

    var hosts: [HostInfo] = []
    var selectedHost: HostInfo?
    var isLoadingHosts = true
    var hostsError: String?

    // MARK: - Placement State

    var placements: [MCPServerPlacement] = []
    var isLoadingPlacements = false
    var placementsError: String?
    
    // MARK: - Server Definitions State
    
    var allServers: [MCPServer] = []
    var isLoadingServers = false
    var serversError: String?

    // MARK: - Init

    init(client: DianeClientProtocol = DianeClient()) {
        self.client = client
    }

    // MARK: - Computed Properties

    /// Returns true if no slaves are available (master-only setup).
    /// Used to degrade UI - hide node picker when only master exists.
    var isMasterOnly: Bool {
        hosts.count <= 1
    }
    
    /// Current host ID for API calls (selected host or default to "master")
    var currentHostID: String {
        selectedHost?.id ?? "master"
    }
    
    /// All available MCP servers with their placement status for the selected host.
    /// Each server includes whether it's enabled on the current node.
    var serversWithPlacementStatus: [(server: MCPServer, isEnabledOnNode: Bool)] {
        allServers.map { server in
            let placement = placements.first(where: { $0.serverID == server.id })
            let isEnabled = placement?.enabled ?? false
            return (server: server, isEnabledOnNode: isEnabled)
        }
    }
    
    // MARK: - Data Operations

    /// Load all data needed for the view: hosts, servers, and placements for selected host.
    func loadData() async {
        FileLogger.shared.info("Loading MCP servers deployment data...", category: "MCPServers")
        await loadHosts()
        await loadServers()
        await loadPlacements()
    }
    
    /// Load all hosts (master + slaves).
    func loadHosts() async {
        isLoadingHosts = true
        hostsError = nil

        do {
            hosts = try await client.getHosts()
            FileLogger.shared.info("Loaded \(hosts.count) hosts", category: "MCPServers")
            
            // Auto-select master on first load if nothing selected
            if selectedHost == nil {
                selectedHost = hosts.first { $0.id == "master" }
            }
        } catch {
            hostsError = error.localizedDescription
            FileLogger.shared.error("Failed to load hosts: \(error.localizedDescription)", category: "MCPServers")
            logger.error("Failed to load hosts: \(error.localizedDescription)")
        }

        isLoadingHosts = false
    }
    
    /// Load all MCP server definitions.
    func loadServers() async {
        isLoadingServers = true
        serversError = nil
        
        do {
            allServers = try await client.getMCPServerConfigs()
            FileLogger.shared.info("Loaded \(allServers.count) server definitions", category: "MCPServers")
        } catch {
            serversError = error.localizedDescription
            FileLogger.shared.error("Failed to load server definitions: \(error.localizedDescription)", category: "MCPServers")
            logger.error("Failed to load server definitions: \(error.localizedDescription)")
        }
        
        isLoadingServers = false
    }

    /// Load placements for the currently selected host.
    func loadPlacements() async {
        guard selectedHost != nil else { return }
        
        isLoadingPlacements = true
        placementsError = nil

        do {
            placements = try await client.getPlacements(hostID: currentHostID)
            FileLogger.shared.info("Loaded \(placements.count) placements for host '\(currentHostID)'", category: "MCPServers")
        } catch {
            placementsError = error.localizedDescription
            FileLogger.shared.error("Failed to load placements for '\(currentHostID)': \(error.localizedDescription)", category: "MCPServers")
            logger.error("Failed to load placements for '\(self.currentHostID)': \(error.localizedDescription)")
        }

        isLoadingPlacements = false
    }
    
    /// Change the selected host and reload placements.
    func selectHost(_ host: HostInfo) async {
        selectedHost = host
        await loadPlacements()
    }

    /// Toggle a server's enabled state on the current host.
    func toggleServerOnCurrentHost(server: MCPServer, enabled: Bool) async {
        FileLogger.shared.info("Toggling server '\(server.name)' on '\(currentHostID)' enabled=\(enabled)", category: "MCPServers")
        do {
            let updatedPlacement = try await client.updatePlacement(
                serverID: server.id,
                hostID: currentHostID,
                enabled: enabled
            )
            
            // Update local state
            if let index = placements.firstIndex(where: { $0.serverID == server.id }) {
                placements[index] = updatedPlacement
            } else {
                // Placement didn't exist before, add it now
                placements.append(updatedPlacement)
            }
            FileLogger.shared.info("Toggled server '\(server.name)' on '\(currentHostID)' successfully", category: "MCPServers")
        } catch {
            placementsError = error.localizedDescription
            FileLogger.shared.error("Failed to toggle server '\(server.name)' on '\(currentHostID)': \(error.localizedDescription)", category: "MCPServers")
            logger.error("Failed to toggle server '\(server.name)' on '\(self.currentHostID)': \(error.localizedDescription)")
        }
    }
    
    /// Delete a placement (reset to default OFF state).
    func deletePlacement(server: MCPServer) async {
        FileLogger.shared.info("Deleting placement for '\(server.name)' on '\(currentHostID)'", category: "MCPServers")
        do {
            try await client.deletePlacement(serverID: server.id, hostID: currentHostID)
            
            // Remove from local state
            placements.removeAll(where: { $0.serverID == server.id })
            FileLogger.shared.info("Deleted placement for '\(server.name)' on '\(currentHostID)' successfully", category: "MCPServers")
        } catch {
            placementsError = error.localizedDescription
            FileLogger.shared.error("Failed to delete placement for '\(server.name)' on '\(currentHostID)': \(error.localizedDescription)", category: "MCPServers")
            logger.error("Failed to delete placement for '\(server.name)' on '\(self.currentHostID)': \(error.localizedDescription)")
        }
    }
}
