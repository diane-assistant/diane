import Foundation
import Observation
import os.log

private let logger = Logger(subsystem: "com.diane.Diane", category: "Contexts")

/// ViewModel for ContextsView that owns all context state and business logic.
///
/// Accepts `DianeClientProtocol` via its initializer so tests can inject a mock.
/// Uses the `@Observable` macro (requires macOS 14+/iOS 17+) so SwiftUI views
/// automatically track property changes without explicit `@Published` wrappers.
@MainActor
@Observable
final class ContextsViewModel {

    // MARK: - Dependencies

    @ObservationIgnored
    let client: DianeClientProtocol

    // MARK: - Context List State

    var contexts: [Context] = []
    var selectedContext: Context?
    var contextDetail: ContextDetail?
    var isLoading = true
    var error: String?

    // MARK: - Create Context State

    var showCreateContext = false
    var newContextName = ""
    var newContextDescription = ""
    var isCreating = false
    var createError: String?

    // MARK: - Connection Info State

    var showConnectionInfo = false
    var connectionInfo: ContextConnectInfo?

    // MARK: - Delete Confirmation

    var showDeleteConfirm = false
    var contextToDelete: Context?

    // MARK: - Add Server State

    var showAddServer = false
    var availableServers: [AvailableServer] = []
    var isLoadingServers = false

    // MARK: - Init

    init(client: DianeClientProtocol = DianeClient()) {
        self.client = client
    }

    // MARK: - Data Operations

    func loadContexts() async {
        isLoading = true
        error = nil
        FileLogger.shared.info("Loading contexts...", category: "Contexts")

        do {
            contexts = try await client.getContexts()
            FileLogger.shared.info("Loaded \(contexts.count) contexts", category: "Contexts")
            // Auto-select first context if none selected
            if selectedContext == nil, let first = contexts.first {
                selectedContext = first
                await loadContextDetail(first.name)
            }
        } catch {
            self.error = error.localizedDescription
            FileLogger.shared.error("Failed to load contexts: \(error.localizedDescription)", category: "Contexts")
        }

        isLoading = false
    }

    func loadContextDetail(_ name: String) async {
        FileLogger.shared.info("Loading context detail for '\(name)'", category: "Contexts")
        do {
            contextDetail = try await client.getContextDetail(name: name)
            FileLogger.shared.info("Loaded context detail for '\(name)' successfully", category: "Contexts")
        } catch {
            // Silently fail for detail loading
            FileLogger.shared.error("Failed to load context detail for '\(name)': \(error.localizedDescription)", category: "Contexts")
            contextDetail = nil
        }
    }

    func loadConnectionInfo(_ name: String) async {
        connectionInfo = nil
        FileLogger.shared.info("Loading connection info for context '\(name)'", category: "Contexts")
        do {
            connectionInfo = try await client.getContextConnectInfo(name: name)
            FileLogger.shared.info("Loaded connection info for context '\(name)' successfully", category: "Contexts")
        } catch {
            // Silently fail
            FileLogger.shared.error("Failed to load connection info for '\(name)': \(error.localizedDescription)", category: "Contexts")
        }
    }

    func syncTools(_ name: String) async {
        FileLogger.shared.info("Syncing tools for context '\(name)'", category: "Contexts")
        do {
            _ = try await client.syncContextTools(contextName: name)
            FileLogger.shared.info("Synced tools for context '\(name)' successfully", category: "Contexts")
            // Reload context detail to show updated tools
            await loadContextDetail(name)
        } catch {
            // Silently fail
            FileLogger.shared.error("Failed to sync tools for context '\(name)': \(error.localizedDescription)", category: "Contexts")
        }
    }

    func createContext() async {
        isCreating = true
        createError = nil
        FileLogger.shared.info("Creating context '\(newContextName)'", category: "Contexts")

        do {
            let context = try await client.createContext(
                name: newContextName.lowercased().replacingOccurrences(of: " ", with: "-"),
                description: newContextDescription.isEmpty ? nil : newContextDescription
            )
            contexts = try await client.getContexts()
            selectedContext = context
            await loadContextDetail(context.name)
            showCreateContext = false
            resetCreateForm()
            FileLogger.shared.info("Created context '\(context.name)' successfully", category: "Contexts")
        } catch {
            createError = error.localizedDescription
            FileLogger.shared.error("Failed to create context '\(newContextName)': \(error.localizedDescription)", category: "Contexts")
        }

        isCreating = false
    }

    func setDefaultContext(_ context: Context) async {
        FileLogger.shared.info("Setting context '\(context.name)' as default", category: "Contexts")
        do {
            try await client.setDefaultContext(name: context.name)
            contexts = try await client.getContexts()
            // Update selected context if it was the one we modified
            if selectedContext?.id == context.id {
                selectedContext = contexts.first { $0.id == context.id }
            }
            FileLogger.shared.info("Set context '\(context.name)' as default successfully", category: "Contexts")
        } catch {
            // Show error somehow
            FileLogger.shared.error("Failed to set context '\(context.name)' as default: \(error.localizedDescription)", category: "Contexts")
        }
    }

    func deleteContext(_ context: Context) async {
        FileLogger.shared.info("Deleting context '\(context.name)'", category: "Contexts")
        do {
            try await client.deleteContext(name: context.name)
            contexts = try await client.getContexts()

            // Clear selection if deleted context was selected
            if selectedContext?.id == context.id {
                selectedContext = contexts.first
                if let first = selectedContext {
                    await loadContextDetail(first.name)
                } else {
                    contextDetail = nil
                }
            }
            FileLogger.shared.info("Deleted context '\(context.name)' successfully", category: "Contexts")
        } catch {
            // Show error somehow
            FileLogger.shared.error("Failed to delete context '\(context.name)': \(error.localizedDescription)", category: "Contexts")
        }
        contextToDelete = nil
    }

    func resetCreateForm() {
        newContextName = ""
        newContextDescription = ""
        createError = nil
    }

    func loadAvailableServers() async {
        guard let contextName = selectedContext?.name else { return }
        isLoadingServers = true
        FileLogger.shared.info("Loading available servers for context '\(contextName)'", category: "Contexts")
        do {
            availableServers = try await client.getAvailableServers(contextName: contextName)
            // Sort: servers not in context first, then alphabetically
            availableServers.sort { a, b in
                if a.inContext != b.inContext {
                    return !a.inContext
                }
                return a.name < b.name
            }
            FileLogger.shared.info("Loaded \(availableServers.count) available servers for context '\(contextName)'", category: "Contexts")
        } catch {
            FileLogger.shared.error("Failed to load available servers for '\(contextName)': \(error.localizedDescription)", category: "Contexts")
            availableServers = []
        }
        isLoadingServers = false
    }

    func addServer(_ serverName: String) async {
        guard let contextName = selectedContext?.name else { return }
        FileLogger.shared.info("Adding server '\(serverName)' to context '\(contextName)'", category: "Contexts")
        do {
            try await client.addServerToContext(contextName: contextName, serverName: serverName, enabled: true)
            FileLogger.shared.info("Added server '\(serverName)' to context '\(contextName)' successfully", category: "Contexts")
            await loadAvailableServers()
            await loadContextDetail(contextName)
        } catch {
            // Show error somehow
            FileLogger.shared.error("Failed to add server '\(serverName)' to context '\(contextName)': \(error.localizedDescription)", category: "Contexts")
        }
    }

    // MARK: - Selection Helpers

    func onSelectContext(_ context: Context) {
        selectedContext = context
    }

    func prepareDeleteContext(_ context: Context) {
        contextToDelete = context
        showDeleteConfirm = true
    }

    func prepareAddServer() {
        availableServers = []
        isLoadingServers = true
        showAddServer = true
    }
}
