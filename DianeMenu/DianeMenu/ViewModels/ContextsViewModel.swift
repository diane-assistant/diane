import Foundation
import Observation

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

        do {
            contexts = try await client.getContexts()
            // Auto-select first context if none selected
            if selectedContext == nil, let first = contexts.first {
                selectedContext = first
                await loadContextDetail(first.name)
            }
        } catch {
            self.error = error.localizedDescription
        }

        isLoading = false
    }

    func loadContextDetail(_ name: String) async {
        do {
            contextDetail = try await client.getContextDetail(name: name)
        } catch {
            // Silently fail for detail loading
            contextDetail = nil
        }
    }

    func loadConnectionInfo(_ name: String) async {
        connectionInfo = nil
        do {
            connectionInfo = try await client.getContextConnectInfo(name: name)
        } catch {
            // Silently fail
        }
    }

    func syncTools(_ name: String) async {
        do {
            _ = try await client.syncContextTools(contextName: name)
            // Reload context detail to show updated tools
            await loadContextDetail(name)
        } catch {
            // Silently fail
        }
    }

    func createContext() async {
        isCreating = true
        createError = nil

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
        } catch {
            createError = error.localizedDescription
        }

        isCreating = false
    }

    func setDefaultContext(_ context: Context) async {
        do {
            try await client.setDefaultContext(name: context.name)
            contexts = try await client.getContexts()
            // Update selected context if it was the one we modified
            if selectedContext?.id == context.id {
                selectedContext = contexts.first { $0.id == context.id }
            }
        } catch {
            // Show error somehow
        }
    }

    func deleteContext(_ context: Context) async {
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
        } catch {
            // Show error somehow
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
        do {
            availableServers = try await client.getAvailableServers(contextName: contextName)
            // Sort: servers not in context first, then alphabetically
            availableServers.sort { a, b in
                if a.inContext != b.inContext {
                    return !a.inContext
                }
                return a.name < b.name
            }
        } catch {
            availableServers = []
        }
        isLoadingServers = false
    }

    func addServer(_ serverName: String) async {
        guard let contextName = selectedContext?.name else { return }
        do {
            try await client.addServerToContext(contextName: contextName, serverName: serverName, enabled: true)
            await loadAvailableServers()
            await loadContextDetail(contextName)
        } catch {
            // Show error somehow
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
