import Foundation
import Observation

/// ViewModel for ProvidersView that owns all provider state and business logic.
///
/// Accepts `DianeClientProtocol` via its initializer so tests can inject a mock.
/// Uses the `@Observable` macro (requires macOS 14+/iOS 17+) so SwiftUI views
/// automatically track property changes without explicit `@Published` wrappers.
@MainActor
@Observable
final class ProvidersViewModel {

    // MARK: - Dependencies

    @ObservationIgnored
    let client: DianeClientProtocol

    // MARK: - Provider List State

    var providers: [Provider] = []
    var selectedProvider: Provider?
    var templates: [ProviderTemplate] = []
    var isLoading = true
    var error: String?

    // MARK: - Create Provider State

    var showCreateProvider = false
    var selectedTemplate: ProviderTemplate?
    var newProviderName = ""
    var configValues: [String: String] = [:]
    var authValues: [String: String] = [:]
    var isCreating = false
    var createError: String?

    // MARK: - Dynamic Model Discovery

    var availableModels: [AvailableModel] = []
    var isLoadingModels = false
    var modelsError: String?

    // MARK: - Edit Provider State

    var editingProvider: Provider?
    var editConfigValues: [String: String] = [:]
    var editAuthValues: [String: String] = [:]
    var isEditing = false
    var editError: String?

    // MARK: - Delete Confirmation

    var showDeleteConfirm = false
    var providerToDelete: Provider?

    // MARK: - Filter

    var typeFilter: ProviderType?

    // MARK: - Google OAuth State

    var showGoogleAuth = false
    var googleAuthStatus: GoogleAuthStatus?
    var isCheckingGoogleAuth = false
    var deviceCodeResponse: GoogleDeviceCodeResponse?
    var isPollingForToken = false
    var googleAuthError: String?

    // MARK: - Init

    init(client: DianeClientProtocol = DianeClient()) {
        self.client = client
    }

    // MARK: - Computed Properties

    var filteredProviders: [Provider] {
        Self.filteredProviders(providers, byType: typeFilter)
    }

    // MARK: - Data Operations

    func loadData() async {
        isLoading = true
        error = nil

        do {
            let loadedProviders = try await client.getProviders(type: typeFilter)
            let loadedTemplates = try await client.getProviderTemplates()

            providers = loadedProviders
            templates = loadedTemplates

            // Update selected provider if it still exists
            if let selected = selectedProvider {
                selectedProvider = providers.first { $0.id == selected.id }
            }
        } catch {
            self.error = error.localizedDescription
        }

        isLoading = false
    }

    func createProvider() async {
        guard let template = selectedTemplate else { return }

        isCreating = true
        createError = nil

        do {
            // Build config
            var config: [String: Any] = [:]
            for field in template.configSchema {
                if let value = configValues[field.key], !value.isEmpty {
                    if field.type == "int", let intValue = Int(value) {
                        config[field.key] = intValue
                    } else if field.type == "bool" {
                        config[field.key] = value == "true"
                    } else {
                        config[field.key] = value
                    }
                }
            }

            // Build auth config
            var authConfig: [String: Any]?
            if template.authType == .apiKey, let apiKey = authValues["api_key"], !apiKey.isEmpty {
                authConfig = ["api_key": apiKey]
            } else if template.authType == .oauth {
                // Use default OAuth account
                authConfig = ["oauth_account": "default"]
            }

            let newProvider = try await client.createProvider(
                name: newProviderName,
                service: template.service,
                config: config,
                authConfig: authConfig
            )

            providers.append(newProvider)
            selectedProvider = newProvider
            showCreateProvider = false
            resetCreateForm()

        } catch {
            createError = error.localizedDescription
        }

        isCreating = false
    }

    func updateProvider() async {
        guard let provider = editingProvider,
              let template = templates.first(where: { $0.service == provider.service }) else { return }

        isEditing = true
        editError = nil

        do {
            // Build config
            var config: [String: Any] = [:]
            for field in template.configSchema {
                if let value = editConfigValues[field.key], !value.isEmpty {
                    if field.type == "int", let intValue = Int(value) {
                        config[field.key] = intValue
                    } else if field.type == "bool" {
                        config[field.key] = value == "true"
                    } else {
                        config[field.key] = value
                    }
                }
            }

            // Build auth config (only if new key provided)
            var authConfig: [String: Any]?
            if template.authType == .apiKey, let apiKey = editAuthValues["api_key"], !apiKey.isEmpty {
                authConfig = ["api_key": apiKey]
            }

            let updated = try await client.updateProvider(
                id: provider.id,
                name: nil,
                config: config.isEmpty ? nil : config,
                authConfig: authConfig
            )

            // Update local state
            if let index = providers.firstIndex(where: { $0.id == provider.id }) {
                providers[index] = updated
            }
            selectedProvider = updated
            editingProvider = nil

        } catch {
            editError = error.localizedDescription
        }

        isEditing = false
    }

    func toggleProvider(_ provider: Provider, enabled: Bool) async {
        do {
            let updated: Provider
            if enabled {
                updated = try await client.enableProvider(id: provider.id)
            } else {
                updated = try await client.disableProvider(id: provider.id)
            }

            if let index = providers.firstIndex(where: { $0.id == provider.id }) {
                providers[index] = updated
            }
            if selectedProvider?.id == provider.id {
                selectedProvider = updated
            }
        } catch {
            self.error = error.localizedDescription
        }
    }

    func setDefault(_ provider: Provider) async {
        do {
            let updated = try await client.setDefaultProvider(id: provider.id)

            // Refresh all providers to update isDefault flags
            await loadData()

            selectedProvider = updated
        } catch {
            self.error = error.localizedDescription
        }
    }

    func deleteProvider(_ provider: Provider) async {
        do {
            try await client.deleteProvider(id: provider.id)
            providers.removeAll { $0.id == provider.id }
            if selectedProvider?.id == provider.id {
                selectedProvider = nil
            }
        } catch {
            self.error = error.localizedDescription
        }
    }

    func prepareEdit(_ provider: Provider) {
        editConfigValues = [:]
        editAuthValues = [:]
        editError = nil

        // Pre-populate config values
        for (key, value) in provider.config {
            if let str = value.value as? String {
                editConfigValues[key] = str
            } else if let num = value.value as? Int {
                editConfigValues[key] = String(num)
            } else if let bool = value.value as? Bool {
                editConfigValues[key] = bool ? "true" : "false"
            }
        }
    }

    func resetCreateForm() {
        selectedTemplate = nil
        newProviderName = ""
        configValues = [:]
        authValues = [:]
        createError = nil
        availableModels = []
        modelsError = nil
    }

    func fetchModels(service: String) async {
        isLoadingModels = true
        modelsError = nil

        do {
            // Get project_id from config values if available
            let projectID = configValues["project_id"]
            availableModels = try await client.listModels(service: service, projectID: projectID)

            // If we got models and there's a model field, set default to first available
            if let firstModel = availableModels.first, configValues["model"]?.isEmpty ?? true {
                configValues["model"] = firstModel.id
            }
        } catch {
            modelsError = "Failed to fetch models: \(error.localizedDescription)"
            // Keep static options as fallback
        }

        isLoadingModels = false
    }

    // MARK: - Google OAuth

    func checkGoogleAuthStatus() async {
        isCheckingGoogleAuth = true

        do {
            googleAuthStatus = try await client.getGoogleAuthStatus(account: "default")
        } catch {
            // Status check failed, leave as nil
            googleAuthStatus = nil
        }

        isCheckingGoogleAuth = false
    }

    func startGoogleAuth() async {
        deviceCodeResponse = nil
        googleAuthError = nil

        do {
            // Start device flow
            deviceCodeResponse = try await client.startGoogleAuth(account: nil, scopes: nil)

            // Start polling for token
            if let dcr = deviceCodeResponse {
                isPollingForToken = true
                await pollForToken(deviceCode: dcr.deviceCode, interval: dcr.interval)
            }
        } catch {
            googleAuthError = error.localizedDescription
        }
    }

    func pollForToken(deviceCode: String, interval: Int) async {
        let pollInterval = max(interval, 5) // At least 5 seconds

        while isPollingForToken && showGoogleAuth {
            // Wait before polling
            try? await Task.sleep(nanoseconds: UInt64(pollInterval) * 1_000_000_000)

            guard showGoogleAuth else { break }

            do {
                let response = try await client.pollGoogleAuth(account: "default", deviceCode: deviceCode, interval: pollInterval)

                if response.isSuccess {
                    // Success! Close the sheet and refresh status
                    isPollingForToken = false
                    showGoogleAuth = false
                    await checkGoogleAuthStatus()
                    resetGoogleAuth()
                    return
                } else if response.isPending {
                    // Keep polling
                    continue
                } else if response.shouldSlowDown {
                    // Slow down - wait extra time
                    try? await Task.sleep(nanoseconds: 5_000_000_000)
                    continue
                } else if response.isExpired {
                    googleAuthError = "Authorization code expired. Please try again."
                    isPollingForToken = false
                    return
                } else if response.isDenied {
                    googleAuthError = "Authorization was denied."
                    isPollingForToken = false
                    return
                } else {
                    // Unknown status
                    googleAuthError = response.message
                    isPollingForToken = false
                    return
                }
            } catch {
                // On error, check if it's a pending response (202) which throws
                // Continue polling on network errors
                continue
            }
        }
    }

    func resetGoogleAuth() {
        deviceCodeResponse = nil
        isPollingForToken = false
        googleAuthError = nil
    }

    // MARK: - Validation

    func isConfigValid() -> Bool {
        Self.isConfigValid(
            template: selectedTemplate,
            configValues: configValues,
            authValues: authValues
        )
    }

    // MARK: - Static Pure Functions

    /// Filter providers by type. Returns all providers when `type` is nil.
    static func filteredProviders(_ providers: [Provider], byType type: ProviderType?) -> [Provider] {
        guard let type = type else { return providers }
        return providers.filter { $0.type == type }
    }

    /// Validate config form: all required fields must be non-empty,
    /// and API key must be present for api_key auth type.
    static func isConfigValid(
        template: ProviderTemplate?,
        configValues: [String: String],
        authValues: [String: String]
    ) -> Bool {
        guard let template = template else { return false }

        for field in template.configSchema where field.required {
            if let value = configValues[field.key], !value.isEmpty {
                continue
            }
            return false
        }

        // Check API key if required
        if template.authType == .apiKey {
            if let apiKey = authValues["api_key"], !apiKey.isEmpty {
                return true
            }
            return false
        }

        return true
    }
}
