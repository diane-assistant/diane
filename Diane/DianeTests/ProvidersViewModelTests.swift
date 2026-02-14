import XCTest
@testable import Diane

/// Tests for `ProvidersViewModel` — pure functions, async data operations,
/// CRUD workflows, filter logic, Google OAuth, and error handling.
@MainActor
final class ProvidersViewModelTests: XCTestCase {

    // MARK: - Helpers

    private func makeViewModel(
        providers: [Provider] = [],
        templates: [ProviderTemplate] = [],
        models: [AvailableModel] = []
    ) -> (ProvidersViewModel, MockDianeClient) {
        let mock = MockDianeClient()
        mock.providersList = providers
        mock.providerTemplates = templates
        mock.availableModelsList = models
        let vm = ProvidersViewModel(client: mock)
        return (vm, mock)
    }

    // =========================================================================
    // MARK: - Pure Function Tests: filteredProviders
    // =========================================================================

    func testFilteredProviders_noFilter_returnsAll() {
        let providers = TestFixtures.makeProviderList()
        let result = ProvidersViewModel.filteredProviders(providers, byType: nil)
        XCTAssertEqual(result.count, providers.count)
    }

    func testFilteredProviders_llmFilter() {
        let providers = TestFixtures.makeProviderList()
        let result = ProvidersViewModel.filteredProviders(providers, byType: .llm)
        XCTAssertTrue(result.allSatisfy { $0.type == .llm })
        // 3 LLM providers: openai-main, openai-backup, disabled-llm
        XCTAssertEqual(result.count, 3)
    }

    func testFilteredProviders_embeddingFilter() {
        let providers = TestFixtures.makeProviderList()
        let result = ProvidersViewModel.filteredProviders(providers, byType: .embedding)
        XCTAssertTrue(result.allSatisfy { $0.type == .embedding })
        XCTAssertEqual(result.count, 1)
    }

    func testFilteredProviders_storageFilter() {
        let providers = TestFixtures.makeProviderList()
        let result = ProvidersViewModel.filteredProviders(providers, byType: .storage)
        XCTAssertTrue(result.allSatisfy { $0.type == .storage })
        XCTAssertEqual(result.count, 1)
    }

    func testFilteredProviders_emptyList() {
        let result = ProvidersViewModel.filteredProviders([], byType: .llm)
        XCTAssertTrue(result.isEmpty)
    }

    // =========================================================================
    // MARK: - Pure Function Tests: isConfigValid
    // =========================================================================

    func testIsConfigValid_nilTemplate_returnsFalse() {
        let result = ProvidersViewModel.isConfigValid(
            template: nil, configValues: [:], authValues: [:]
        )
        XCTAssertFalse(result)
    }

    func testIsConfigValid_allRequiredFieldsFilled_apiKeyPresent() {
        let template = TestFixtures.makeProviderTemplate() // OpenAI: model required, apiKey auth
        let result = ProvidersViewModel.isConfigValid(
            template: template,
            configValues: ["model": "gpt-4o"],
            authValues: ["api_key": "sk-test123"]
        )
        XCTAssertTrue(result)
    }

    func testIsConfigValid_missingRequiredField_returnsFalse() {
        let template = TestFixtures.makeProviderTemplate()
        let result = ProvidersViewModel.isConfigValid(
            template: template,
            configValues: [:], // model is required but missing
            authValues: ["api_key": "sk-test"]
        )
        XCTAssertFalse(result)
    }

    func testIsConfigValid_emptyRequiredField_returnsFalse() {
        let template = TestFixtures.makeProviderTemplate()
        let result = ProvidersViewModel.isConfigValid(
            template: template,
            configValues: ["model": ""], // empty string
            authValues: ["api_key": "sk-test"]
        )
        XCTAssertFalse(result)
    }

    func testIsConfigValid_apiKeyAuth_missingKey_returnsFalse() {
        let template = TestFixtures.makeProviderTemplate() // apiKey auth
        let result = ProvidersViewModel.isConfigValid(
            template: template,
            configValues: ["model": "gpt-4o"],
            authValues: [:] // no api_key
        )
        XCTAssertFalse(result)
    }

    func testIsConfigValid_apiKeyAuth_emptyKey_returnsFalse() {
        let template = TestFixtures.makeProviderTemplate()
        let result = ProvidersViewModel.isConfigValid(
            template: template,
            configValues: ["model": "gpt-4o"],
            authValues: ["api_key": ""]
        )
        XCTAssertFalse(result)
    }

    func testIsConfigValid_oauthAuth_noApiKeyNeeded() {
        let template = TestFixtures.makeVertexAITemplate() // oauth auth
        let result = ProvidersViewModel.isConfigValid(
            template: template,
            configValues: ["project_id": "my-project", "region": "us-central1"],
            authValues: [:] // no api_key needed for oauth
        )
        XCTAssertTrue(result)
    }

    func testIsConfigValid_noAuth_noApiKeyNeeded() {
        let template = TestFixtures.makeProviderTemplate(
            service: "ollama", name: "Ollama", authType: .none,
            configSchema: [TestFixtures.makeConfigField(key: "model", label: "Model", required: true)],
            description: "Local Ollama"
        )
        let result = ProvidersViewModel.isConfigValid(
            template: template,
            configValues: ["model": "llama3"],
            authValues: [:]
        )
        XCTAssertTrue(result)
    }

    func testIsConfigValid_nonRequiredFieldsMissing_stillValid() {
        let template = TestFixtures.makeProviderTemplate(
            configSchema: [
                TestFixtures.makeConfigField(key: "model", label: "Model", required: true),
                TestFixtures.makeConfigField(key: "base_url", label: "Base URL", required: false),
            ]
        )
        let result = ProvidersViewModel.isConfigValid(
            template: template,
            configValues: ["model": "gpt-4o"], // base_url omitted
            authValues: ["api_key": "sk-test"]
        )
        XCTAssertTrue(result)
    }

    // =========================================================================
    // MARK: - Computed Property: filteredProviders (instance)
    // =========================================================================

    func testFilteredProviders_instanceProperty_noFilter() async {
        let providers = TestFixtures.makeProviderList()
        let (vm, _) = makeViewModel(providers: providers, templates: TestFixtures.makeProviderTemplateList())
        await vm.loadData()

        vm.typeFilter = nil
        XCTAssertEqual(vm.filteredProviders.count, providers.count)
    }

    func testFilteredProviders_instanceProperty_withFilter() async {
        let providers = TestFixtures.makeProviderList()
        let (vm, _) = makeViewModel(providers: providers, templates: TestFixtures.makeProviderTemplateList())
        await vm.loadData()

        vm.typeFilter = .embedding
        XCTAssertEqual(vm.filteredProviders.count, 1)
        XCTAssertEqual(vm.filteredProviders.first?.type, .embedding)
    }

    // =========================================================================
    // MARK: - loadData
    // =========================================================================

    func testLoadData_populatesProvidersAndTemplates() async {
        let providers = TestFixtures.makeProviderList()
        let templates = TestFixtures.makeProviderTemplateList()
        let (vm, _) = makeViewModel(providers: providers, templates: templates)

        await vm.loadData()

        XCTAssertEqual(vm.providers.count, providers.count)
        XCTAssertEqual(vm.templates.count, templates.count)
        XCTAssertFalse(vm.isLoading)
        XCTAssertNil(vm.error)
    }

    func testLoadData_setsErrorOnFailure() async {
        let (vm, mock) = makeViewModel()
        mock.errorToThrow = MockError.networkFailure

        await vm.loadData()

        XCTAssertNotNil(vm.error)
        XCTAssertTrue(vm.providers.isEmpty)
        XCTAssertTrue(vm.templates.isEmpty)
        XCTAssertFalse(vm.isLoading)
    }

    func testLoadData_callsClientMethods() async {
        let (vm, mock) = makeViewModel()

        await vm.loadData()

        XCTAssertEqual(mock.callCount(for: "getProviders"), 1)
        XCTAssertEqual(mock.callCount(for: "getProviderTemplates"), 1)
    }

    func testLoadData_clearsErrorOnRetry() async {
        let (vm, mock) = makeViewModel()
        mock.errorToThrow = MockError.networkFailure
        await vm.loadData()
        XCTAssertNotNil(vm.error)

        mock.errorToThrow = nil
        mock.providersList = TestFixtures.makeProviderList()
        await vm.loadData()
        XCTAssertNil(vm.error)
        XCTAssertFalse(vm.providers.isEmpty)
    }

    func testLoadData_updatesSelectedProvider() async {
        let providers = TestFixtures.makeProviderList()
        let (vm, _) = makeViewModel(providers: providers, templates: TestFixtures.makeProviderTemplateList())

        // Pre-select a provider
        vm.selectedProvider = providers[0]
        await vm.loadData()

        // selectedProvider should be refreshed from loaded data
        XCTAssertNotNil(vm.selectedProvider)
        XCTAssertEqual(vm.selectedProvider?.id, providers[0].id)
    }

    func testLoadData_clearsSelectedProviderIfRemoved() async {
        let original = TestFixtures.makeProviderList()
        let (vm, mock) = makeViewModel(providers: original, templates: TestFixtures.makeProviderTemplateList())

        // Pre-select a provider, then remove it from the mock
        vm.selectedProvider = original[0]
        mock.providersList = Array(original.dropFirst())
        await vm.loadData()

        XCTAssertNil(vm.selectedProvider)
    }

    func testLoadData_passesTypeFilter() async {
        let providers = TestFixtures.makeProviderList()
        let (vm, _) = makeViewModel(providers: providers, templates: TestFixtures.makeProviderTemplateList())
        vm.typeFilter = .embedding

        await vm.loadData()

        // getProviders is called with type filter, mock filters accordingly
        XCTAssertEqual(vm.providers.count, 1)
        XCTAssertEqual(vm.providers.first?.type, .embedding)
    }

    // =========================================================================
    // MARK: - createProvider
    // =========================================================================

    func testCreateProvider_success() async {
        let templates = TestFixtures.makeProviderTemplateList()
        let (vm, mock) = makeViewModel(templates: templates)
        await vm.loadData()

        vm.selectedTemplate = templates[0] // OpenAI template
        vm.newProviderName = "new-openai"
        vm.configValues = ["model": "gpt-4o"]
        vm.authValues = ["api_key": "sk-test123"]

        await vm.createProvider()

        XCTAssertFalse(vm.isCreating)
        XCTAssertNil(vm.createError)
        XCTAssertFalse(vm.showCreateProvider)
        XCTAssertEqual(vm.providers.count, 1)
        XCTAssertEqual(vm.selectedProvider?.name, "new-openai")
        XCTAssertEqual(mock.callCount(for: "createProvider"), 1)
        XCTAssertEqual(mock.lastCreateProviderArgs?.name, "new-openai")
        XCTAssertEqual(mock.lastCreateProviderArgs?.service, "openai")
    }

    func testCreateProvider_noTemplate_doesNothing() async {
        let (vm, mock) = makeViewModel()
        vm.selectedTemplate = nil

        await vm.createProvider()

        XCTAssertEqual(mock.callCount(for: "createProvider"), 0)
    }

    func testCreateProvider_setsErrorOnFailure() async {
        let templates = TestFixtures.makeProviderTemplateList()
        let (vm, mock) = makeViewModel(templates: templates)
        await vm.loadData()

        vm.selectedTemplate = templates[0]
        vm.newProviderName = "fail-provider"
        mock.errorToThrow = MockError.serverError("Service unavailable")

        await vm.createProvider()

        XCTAssertNotNil(vm.createError)
        XCTAssertFalse(vm.isCreating)
    }

    func testCreateProvider_resetsFormAfterSuccess() async {
        let templates = TestFixtures.makeProviderTemplateList()
        let (vm, _) = makeViewModel(templates: templates)
        await vm.loadData()

        vm.selectedTemplate = templates[0]
        vm.newProviderName = "test"
        vm.configValues = ["model": "gpt-4o"]
        vm.authValues = ["api_key": "sk-xxx"]

        await vm.createProvider()

        // Form should be reset
        XCTAssertNil(vm.selectedTemplate)
        XCTAssertEqual(vm.newProviderName, "")
        XCTAssertTrue(vm.configValues.isEmpty)
        XCTAssertTrue(vm.authValues.isEmpty)
        XCTAssertNil(vm.createError)
    }

    func testCreateProvider_configTypeParsing() async {
        // Template with int and bool fields
        let template = TestFixtures.makeProviderTemplate(
            configSchema: [
                TestFixtures.makeConfigField(key: "max_tokens", label: "Max Tokens", type: "int", required: true),
                TestFixtures.makeConfigField(key: "stream", label: "Stream", type: "bool", required: true),
                TestFixtures.makeConfigField(key: "model", label: "Model", type: "string", required: true),
            ]
        )
        let (vm, mock) = makeViewModel(templates: [template])
        await vm.loadData()

        vm.selectedTemplate = template
        vm.newProviderName = "typed-provider"
        vm.configValues = ["max_tokens": "4096", "stream": "true", "model": "gpt-4o"]
        vm.authValues = ["api_key": "sk-test"]

        await vm.createProvider()

        let config = mock.lastCreateProviderArgs?.config
        XCTAssertNotNil(config)
        // The mock receives the config dict; verify it was constructed
        XCTAssertEqual(mock.callCount(for: "createProvider"), 1)
    }

    func testCreateProvider_oauthTemplate_setsOAuthAccount() async {
        let template = TestFixtures.makeVertexAITemplate() // oauth auth
        let (vm, mock) = makeViewModel(templates: [template])
        await vm.loadData()

        vm.selectedTemplate = template
        vm.newProviderName = "vertex-provider"
        vm.configValues = ["project_id": "my-project", "region": "us-central1"]

        await vm.createProvider()

        XCTAssertEqual(mock.callCount(for: "createProvider"), 1)
        // Auth config should have oauth_account
        let authConfig = mock.lastCreateProviderArgs?.authConfig
        XCTAssertNotNil(authConfig)
        XCTAssertEqual(authConfig?["oauth_account"] as? String, "default")
    }

    // =========================================================================
    // MARK: - updateProvider
    // =========================================================================

    func testUpdateProvider_success() async {
        let providers = TestFixtures.makeProviderList()
        let templates = TestFixtures.makeProviderTemplateList()
        let (vm, mock) = makeViewModel(providers: providers, templates: templates)
        await vm.loadData()

        let provider = providers[0]
        vm.editingProvider = provider
        vm.editConfigValues = ["model": "gpt-4o-mini"]

        await vm.updateProvider()

        XCTAssertFalse(vm.isEditing)
        XCTAssertNil(vm.editError)
        XCTAssertNil(vm.editingProvider) // cleared after success
        XCTAssertEqual(mock.callCount(for: "updateProvider"), 1)
        XCTAssertEqual(mock.lastUpdateProviderArgs?.id, provider.id)
    }

    func testUpdateProvider_noEditingProvider_doesNothing() async {
        let (vm, mock) = makeViewModel()
        vm.editingProvider = nil

        await vm.updateProvider()

        XCTAssertEqual(mock.callCount(for: "updateProvider"), 0)
    }

    func testUpdateProvider_noMatchingTemplate_doesNothing() async {
        let provider = TestFixtures.makeProvider(service: "nonexistent_service")
        let (vm, mock) = makeViewModel(providers: [provider], templates: TestFixtures.makeProviderTemplateList())
        await vm.loadData()
        vm.editingProvider = provider

        await vm.updateProvider()

        // No matching template found, so guard fails
        XCTAssertEqual(mock.callCount(for: "updateProvider"), 0)
    }

    func testUpdateProvider_setsErrorOnFailure() async {
        let providers = TestFixtures.makeProviderList()
        let templates = TestFixtures.makeProviderTemplateList()
        let (vm, mock) = makeViewModel(providers: providers, templates: templates)
        await vm.loadData()

        vm.editingProvider = providers[0]
        mock.errorToThrow = MockError.networkFailure

        await vm.updateProvider()

        XCTAssertNotNil(vm.editError)
        XCTAssertFalse(vm.isEditing)
    }

    func testUpdateProvider_updatesLocalState() async {
        let providers = TestFixtures.makeProviderList()
        let templates = TestFixtures.makeProviderTemplateList()
        let (vm, _) = makeViewModel(providers: providers, templates: templates)
        await vm.loadData()

        let provider = providers[0]
        vm.editingProvider = provider
        vm.selectedProvider = provider
        vm.editConfigValues = ["model": "gpt-4o-mini"]

        await vm.updateProvider()

        // Provider should be updated in the list
        XCTAssertNotNil(vm.providers.first(where: { $0.id == provider.id }))
        // selectedProvider should be updated
        XCTAssertEqual(vm.selectedProvider?.id, provider.id)
    }

    // =========================================================================
    // MARK: - toggleProvider
    // =========================================================================

    func testToggleProvider_enable() async {
        let provider = TestFixtures.makeProvider(id: 1, enabled: false)
        let (vm, mock) = makeViewModel(providers: [provider])
        await vm.loadData()

        await vm.toggleProvider(provider, enabled: true)

        XCTAssertEqual(mock.callCount(for: "enableProvider"), 1)
        XCTAssertTrue(vm.providers[0].enabled)
    }

    func testToggleProvider_disable() async {
        let provider = TestFixtures.makeProvider(id: 1, enabled: true)
        let (vm, mock) = makeViewModel(providers: [provider])
        await vm.loadData()

        await vm.toggleProvider(provider, enabled: false)

        XCTAssertEqual(mock.callCount(for: "disableProvider"), 1)
        XCTAssertFalse(vm.providers[0].enabled)
    }

    func testToggleProvider_updatesSelectedProvider() async {
        let provider = TestFixtures.makeProvider(id: 1, enabled: true)
        let (vm, _) = makeViewModel(providers: [provider])
        await vm.loadData()
        vm.selectedProvider = provider

        await vm.toggleProvider(provider, enabled: false)

        XCTAssertEqual(vm.selectedProvider?.enabled, false)
    }

    func testToggleProvider_setsErrorOnFailure() async {
        let provider = TestFixtures.makeProvider(id: 1, enabled: true)
        let (vm, mock) = makeViewModel(providers: [provider])
        await vm.loadData()

        mock.errorToThrow = MockError.networkFailure
        await vm.toggleProvider(provider, enabled: false)

        XCTAssertNotNil(vm.error)
    }

    // =========================================================================
    // MARK: - setDefault
    // =========================================================================

    func testSetDefault_success() async {
        let providers = TestFixtures.makeProviderList()
        let templates = TestFixtures.makeProviderTemplateList()
        let (vm, mock) = makeViewModel(providers: providers, templates: templates)
        await vm.loadData()

        let provider = providers[1] // openai-backup, not default
        await vm.setDefault(provider)

        XCTAssertEqual(mock.callCount(for: "setDefaultProvider"), 1)
        // loadData is called again after setDefault
        XCTAssertGreaterThanOrEqual(mock.callCount(for: "getProviders"), 2)
    }

    func testSetDefault_setsErrorOnFailure() async {
        let providers = TestFixtures.makeProviderList()
        let (vm, mock) = makeViewModel(providers: providers)
        await vm.loadData()

        mock.errorToThrow = MockError.networkFailure
        await vm.setDefault(providers[1])

        XCTAssertNotNil(vm.error)
    }

    // =========================================================================
    // MARK: - deleteProvider
    // =========================================================================

    func testDeleteProvider_removesFromList() async {
        let providers = TestFixtures.makeProviderList()
        let (vm, mock) = makeViewModel(providers: providers)
        await vm.loadData()
        let initialCount = vm.providers.count

        await vm.deleteProvider(providers[0])

        XCTAssertEqual(vm.providers.count, initialCount - 1)
        XCTAssertNil(vm.providers.first(where: { $0.id == providers[0].id }))
        XCTAssertEqual(mock.callCount(for: "deleteProvider"), 1)
    }

    func testDeleteProvider_clearsSelectedIfDeleted() async {
        let providers = TestFixtures.makeProviderList()
        let (vm, _) = makeViewModel(providers: providers)
        await vm.loadData()
        vm.selectedProvider = providers[0]

        await vm.deleteProvider(providers[0])

        XCTAssertNil(vm.selectedProvider)
    }

    func testDeleteProvider_keepSelectedIfDifferent() async {
        let providers = TestFixtures.makeProviderList()
        let (vm, _) = makeViewModel(providers: providers)
        await vm.loadData()
        vm.selectedProvider = providers[1]

        await vm.deleteProvider(providers[0])

        XCTAssertNotNil(vm.selectedProvider)
        XCTAssertEqual(vm.selectedProvider?.id, providers[1].id)
    }

    func testDeleteProvider_setsErrorOnFailure() async {
        let providers = TestFixtures.makeProviderList()
        let (vm, mock) = makeViewModel(providers: providers)
        await vm.loadData()

        mock.errorToThrow = MockError.networkFailure
        await vm.deleteProvider(providers[0])

        XCTAssertNotNil(vm.error)
    }

    // =========================================================================
    // MARK: - prepareEdit
    // =========================================================================

    func testPrepareEdit_populatesConfigValues() {
        let provider = TestFixtures.makeProvider(
            config: ["model": AnyCodable("gpt-4o"), "temperature": AnyCodable(0.7)]
        )
        let (vm, _) = makeViewModel()

        vm.prepareEdit(provider)

        XCTAssertEqual(vm.editConfigValues["model"], "gpt-4o")
        XCTAssertTrue(vm.editAuthValues.isEmpty)
        XCTAssertNil(vm.editError)
    }

    func testPrepareEdit_handlesIntConfig() {
        let provider = TestFixtures.makeProvider(
            config: ["max_tokens": AnyCodable(4096)]
        )
        let (vm, _) = makeViewModel()

        vm.prepareEdit(provider)

        XCTAssertEqual(vm.editConfigValues["max_tokens"], "4096")
    }

    func testPrepareEdit_handlesBoolConfig() {
        let provider = TestFixtures.makeProvider(
            config: ["stream": AnyCodable(true)]
        )
        let (vm, _) = makeViewModel()

        vm.prepareEdit(provider)

        XCTAssertEqual(vm.editConfigValues["stream"], "true")
    }

    func testPrepareEdit_resetsExistingValues() {
        let (vm, _) = makeViewModel()
        vm.editConfigValues = ["old_key": "old_value"]
        vm.editAuthValues = ["old_auth": "old_token"]
        vm.editError = "previous error"

        let provider = TestFixtures.makeProvider(config: ["model": AnyCodable("gpt-4o")])
        vm.prepareEdit(provider)

        XCTAssertEqual(vm.editConfigValues.count, 1)
        XCTAssertTrue(vm.editAuthValues.isEmpty)
        XCTAssertNil(vm.editError)
    }

    // =========================================================================
    // MARK: - resetCreateForm
    // =========================================================================

    func testResetCreateForm_clearsAllFields() {
        let (vm, _) = makeViewModel()
        vm.selectedTemplate = TestFixtures.makeProviderTemplate()
        vm.newProviderName = "test"
        vm.configValues = ["key": "val"]
        vm.authValues = ["api_key": "sk-test"]
        vm.createError = "some error"
        vm.availableModels = TestFixtures.makeAvailableModelList()
        vm.modelsError = "model error"

        vm.resetCreateForm()

        XCTAssertNil(vm.selectedTemplate)
        XCTAssertEqual(vm.newProviderName, "")
        XCTAssertTrue(vm.configValues.isEmpty)
        XCTAssertTrue(vm.authValues.isEmpty)
        XCTAssertNil(vm.createError)
        XCTAssertTrue(vm.availableModels.isEmpty)
        XCTAssertNil(vm.modelsError)
    }

    // =========================================================================
    // MARK: - fetchModels
    // =========================================================================

    func testFetchModels_success() async {
        let models = TestFixtures.makeAvailableModelList()
        let (vm, mock) = makeViewModel(models: models)

        await vm.fetchModels(service: "openai")

        XCTAssertEqual(vm.availableModels.count, models.count)
        XCTAssertFalse(vm.isLoadingModels)
        XCTAssertNil(vm.modelsError)
        XCTAssertEqual(mock.callCount(for: "listModels"), 1)
    }

    func testFetchModels_setsDefaultModelIfEmpty() async {
        let models = TestFixtures.makeAvailableModelList()
        let (vm, _) = makeViewModel(models: models)
        // model config is empty
        vm.configValues = [:]

        await vm.fetchModels(service: "openai")

        XCTAssertEqual(vm.configValues["model"], models.first?.id)
    }

    func testFetchModels_doesNotOverrideExistingModel() async {
        let models = TestFixtures.makeAvailableModelList()
        let (vm, _) = makeViewModel(models: models)
        vm.configValues = ["model": "my-custom-model"]

        await vm.fetchModels(service: "openai")

        XCTAssertEqual(vm.configValues["model"], "my-custom-model")
    }

    func testFetchModels_setsErrorOnFailure() async {
        let (vm, mock) = makeViewModel()
        mock.errorToThrow = MockError.networkFailure

        await vm.fetchModels(service: "openai")

        XCTAssertNotNil(vm.modelsError)
        XCTAssertTrue(vm.availableModels.isEmpty)
        XCTAssertFalse(vm.isLoadingModels)
    }

    func testFetchModels_passesProjectID() async {
        let (vm, mock) = makeViewModel()
        vm.configValues = ["project_id": "my-project"]

        await vm.fetchModels(service: "vertex_ai")

        XCTAssertEqual(mock.callCount(for: "listModels"), 1)
    }

    // =========================================================================
    // MARK: - Google OAuth
    // =========================================================================

    func testCheckGoogleAuthStatus_success() async {
        let status = GoogleAuthStatus(
            authenticated: true, account: "default",
            hasToken: true, hasCredentials: true,
            hasADC: false, usingADC: false, tokenPath: "/path/to/token"
        )
        let (vm, mock) = makeViewModel()
        mock.googleAuthStatusResult = status

        await vm.checkGoogleAuthStatus()

        XCTAssertNotNil(vm.googleAuthStatus)
        XCTAssertTrue(vm.googleAuthStatus!.authenticated)
        XCTAssertFalse(vm.isCheckingGoogleAuth)
    }

    func testCheckGoogleAuthStatus_failure_setsNil() async {
        let (vm, mock) = makeViewModel()
        mock.errorToThrow = MockError.networkFailure

        await vm.checkGoogleAuthStatus()

        XCTAssertNil(vm.googleAuthStatus)
        XCTAssertFalse(vm.isCheckingGoogleAuth)
    }

    func testResetGoogleAuth_clearsState() {
        let (vm, _) = makeViewModel()
        vm.deviceCodeResponse = GoogleDeviceCodeResponse(
            userCode: "ABC-DEF", verificationURL: "https://example.com",
            expiresIn: 300, interval: 5, deviceCode: "device123"
        )
        vm.isPollingForToken = true
        vm.googleAuthError = "some error"

        vm.resetGoogleAuth()

        XCTAssertNil(vm.deviceCodeResponse)
        XCTAssertFalse(vm.isPollingForToken)
        XCTAssertNil(vm.googleAuthError)
    }

    // =========================================================================
    // MARK: - isConfigValid (instance method)
    // =========================================================================

    func testIsConfigValid_instance_delegatesToStatic() {
        let template = TestFixtures.makeProviderTemplate()
        let (vm, _) = makeViewModel()
        vm.selectedTemplate = template
        vm.configValues = ["model": "gpt-4o"]
        vm.authValues = ["api_key": "sk-test"]

        XCTAssertTrue(vm.isConfigValid())
    }

    func testIsConfigValid_instance_noTemplate_returnsFalse() {
        let (vm, _) = makeViewModel()
        vm.selectedTemplate = nil

        XCTAssertFalse(vm.isConfigValid())
    }

    // =========================================================================
    // MARK: - Initial State
    // =========================================================================

    func testInitialState() {
        let (vm, _) = makeViewModel()

        XCTAssertTrue(vm.providers.isEmpty)
        XCTAssertNil(vm.selectedProvider)
        XCTAssertTrue(vm.templates.isEmpty)
        XCTAssertTrue(vm.isLoading)
        XCTAssertNil(vm.error)
        XCTAssertFalse(vm.showCreateProvider)
        XCTAssertNil(vm.selectedTemplate)
        XCTAssertEqual(vm.newProviderName, "")
        XCTAssertTrue(vm.configValues.isEmpty)
        XCTAssertTrue(vm.authValues.isEmpty)
        XCTAssertFalse(vm.isCreating)
        XCTAssertNil(vm.createError)
        XCTAssertTrue(vm.availableModels.isEmpty)
        XCTAssertFalse(vm.isLoadingModels)
        XCTAssertNil(vm.modelsError)
        XCTAssertNil(vm.editingProvider)
        XCTAssertTrue(vm.editConfigValues.isEmpty)
        XCTAssertTrue(vm.editAuthValues.isEmpty)
        XCTAssertFalse(vm.isEditing)
        XCTAssertNil(vm.editError)
        XCTAssertFalse(vm.showDeleteConfirm)
        XCTAssertNil(vm.providerToDelete)
        XCTAssertNil(vm.typeFilter)
        XCTAssertFalse(vm.showGoogleAuth)
        XCTAssertNil(vm.googleAuthStatus)
        XCTAssertFalse(vm.isCheckingGoogleAuth)
        XCTAssertNil(vm.deviceCodeResponse)
        XCTAssertFalse(vm.isPollingForToken)
        XCTAssertNil(vm.googleAuthError)
    }
}
