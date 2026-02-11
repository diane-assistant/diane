import SwiftUI
import AppKit

struct ProvidersView: View {
    @State private var providers: [Provider] = []
    @State private var selectedProvider: Provider?
    @State private var templates: [ProviderTemplate] = []
    @State private var isLoading = true
    @State private var error: String?
    
    // Create provider state
    @State private var showCreateProvider = false
    @State private var selectedTemplate: ProviderTemplate?
    @State private var newProviderName = ""
    @State private var configValues: [String: String] = [:]
    @State private var authValues: [String: String] = [:]
    @State private var isCreating = false
    @State private var createError: String?
    
    // Dynamic model discovery
    @State private var availableModels: [AvailableModel] = []
    @State private var isLoadingModels = false
    @State private var modelsError: String?
    
    // Edit provider state
    @State private var editingProvider: Provider?  // Used for sheet(item:) pattern
    @State private var editConfigValues: [String: String] = [:]
    @State private var editAuthValues: [String: String] = [:]
    @State private var isEditing = false
    @State private var editError: String?
    
    // Delete confirmation
    @State private var showDeleteConfirm = false
    @State private var providerToDelete: Provider?
    
    // Filter
    @State private var typeFilter: ProviderType?
    
    // Google OAuth state
    @State private var showGoogleAuth = false
    @State private var googleAuthStatus: GoogleAuthStatus?
    @State private var isCheckingGoogleAuth = false
    @State private var deviceCodeResponse: GoogleDeviceCodeResponse?
    @State private var isPollingForToken = false
    @State private var googleAuthError: String?
    
    private let client = DianeClient()
    
    var body: some View {
        VStack(spacing: 0) {
            headerView
            
            Divider()
            
            if isLoading {
                loadingView
            } else if let error = error {
                errorView(error)
            } else if providers.isEmpty {
                emptyView
            } else {
                HSplitView {
                    providersListView
                        .frame(minWidth: 200, idealWidth: 250, maxWidth: 350)
                    
                    detailView
                        .frame(minWidth: 400, idealWidth: 550)
                }
            }
        }
        .frame(minWidth: 700, idealWidth: 800, maxWidth: .infinity,
               minHeight: 400, idealHeight: 500, maxHeight: .infinity)
        .task {
            await loadData()
        }
        .sheet(isPresented: $showCreateProvider) {
            createProviderSheet
        }
        .sheet(item: $editingProvider) { provider in
            editProviderSheet(for: provider)
        }
        .alert("Delete Provider", isPresented: $showDeleteConfirm) {
            Button("Cancel", role: .cancel) { }
            Button("Delete", role: .destructive) {
                if let provider = providerToDelete {
                    Task { await deleteProvider(provider) }
                }
            }
        } message: {
            if let provider = providerToDelete {
                Text("Are you sure you want to delete '\(provider.name)'? This cannot be undone.")
            }
        }
        .sheet(isPresented: $showGoogleAuth) {
            googleAuthSheet
        }
    }
    
    // MARK: - Header
    
    private var headerView: some View {
        HStack(spacing: 12) {
            Image(systemName: "cpu")
                .foregroundStyle(.secondary)
            
            Text("Providers")
                .font(.headline)
            
            Spacer()
            
            // Type filter
            Picker("", selection: $typeFilter) {
                Text("All").tag(nil as ProviderType?)
                ForEach(ProviderType.allCases, id: \.self) { type in
                    Label(type.displayName, systemImage: type.icon).tag(type as ProviderType?)
                }
            }
            .pickerStyle(.menu)
            .frame(width: 130)
            
            // Create provider button
            Button {
                showCreateProvider = true
            } label: {
                Label("Add Provider", systemImage: "plus")
            }
            
            // Refresh button
            Button {
                Task { await loadData() }
            } label: {
                Image(systemName: "arrow.clockwise")
            }
            .disabled(isLoading)
        }
        .padding()
    }
    
    // MARK: - Loading View
    
    private var loadingView: some View {
        VStack(spacing: 12) {
            ProgressView()
            Text("Loading providers...")
                .font(.caption)
                .foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
    
    // MARK: - Error View
    
    private func errorView(_ message: String) -> some View {
        VStack(spacing: 12) {
            Image(systemName: "exclamationmark.triangle.fill")
                .font(.largeTitle)
                .foregroundStyle(.orange)
            Text("Failed to load providers")
                .font(.headline)
            Text(message)
                .font(.caption)
                .foregroundStyle(.secondary)
            Button("Retry") {
                Task { await loadData() }
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
    
    // MARK: - Empty View
    
    private var emptyView: some View {
        VStack(spacing: 16) {
            Image(systemName: "cpu")
                .font(.system(size: 48))
                .foregroundStyle(.secondary)
            Text("No providers configured")
                .font(.headline)
            Text("Add a provider to enable features like embeddings, LLM, or storage")
                .font(.caption)
                .foregroundStyle(.secondary)
                .multilineTextAlignment(.center)
                .frame(maxWidth: 300)
            Button {
                showCreateProvider = true
            } label: {
                Label("Add Provider", systemImage: "plus")
            }
            .buttonStyle(.borderedProminent)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
    
    // MARK: - Providers List
    
    private var filteredProviders: [Provider] {
        if let typeFilter = typeFilter {
            return providers.filter { $0.type == typeFilter }
        }
        return providers
    }
    
    private var providersListView: some View {
        VStack(alignment: .leading, spacing: 0) {
            // Section header
            HStack {
                Image(systemName: "list.bullet")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                Text("Configured Providers")
                    .font(.subheadline.weight(.semibold))
                Spacer()
                Text("\(filteredProviders.count)")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            .padding(.horizontal)
            .padding(.vertical, 8)
            .background(Color(nsColor: .windowBackgroundColor))
            
            Divider()
            
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 0) {
                    ForEach(filteredProviders) { provider in
                        ProviderRow(
                            provider: provider,
                            isSelected: selectedProvider?.id == provider.id,
                            onSelect: {
                                selectedProvider = provider
                            },
                            onToggle: { enabled in
                                Task { await toggleProvider(provider, enabled: enabled) }
                            },
                            onSetDefault: {
                                Task { await setDefault(provider) }
                            },
                            onDelete: {
                                providerToDelete = provider
                                showDeleteConfirm = true
                            }
                        )
                        Divider()
                    }
                }
            }
        }
        .background(Color(nsColor: .controlBackgroundColor))
    }
    
    // MARK: - Detail View
    
    private var detailView: some View {
        Group {
            if let provider = selectedProvider {
                ProviderDetailView(
                    provider: provider,
                    template: templates.first { $0.service == provider.service },
                    onEdit: {
                        editingProvider = provider
                    },
                    onToggle: { enabled in
                        Task { await toggleProvider(provider, enabled: enabled) }
                    },
                    onSetDefault: {
                        Task { await setDefault(provider) }
                    },
                    onDelete: {
                        providerToDelete = provider
                        showDeleteConfirm = true
                    },
                    onAuthenticate: {
                        showGoogleAuth = true
                        Task { await startGoogleAuth() }
                    }
                )
            } else {
                VStack(spacing: 12) {
                    Image(systemName: "sidebar.left")
                        .font(.largeTitle)
                        .foregroundStyle(.secondary)
                    Text("Select a provider")
                        .font(.headline)
                        .foregroundStyle(.secondary)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            }
        }
        .background(Color(nsColor: .textBackgroundColor))
    }
    
    // MARK: - Create Provider Sheet
    
    private var createProviderSheet: some View {
        VStack(spacing: 0) {
            // Header
            HStack {
                Text("Add Provider")
                    .font(.headline)
                Spacer()
                Button {
                    showCreateProvider = false
                    resetCreateForm()
                } label: {
                    Image(systemName: "xmark.circle.fill")
                        .foregroundStyle(.secondary)
                }
                .buttonStyle(.plain)
            }
            .padding()
            
            Divider()
            
            // Content
            ScrollView {
                VStack(alignment: .leading, spacing: 20) {
                    // Template selection
                    if selectedTemplate == nil {
                        templateSelectionView
                    } else {
                        // Back button
                        Button {
                            selectedTemplate = nil
                            resetCreateForm()
                        } label: {
                            Label("Back to templates", systemImage: "chevron.left")
                        }
                        .buttonStyle(.plain)
                        .foregroundStyle(.secondary)
                        
                        // Provider configuration
                        if let template = selectedTemplate {
                            providerConfigForm(template: template)
                        }
                    }
                    
                    // Error message
                    if let error = createError {
                        HStack {
                            Image(systemName: "exclamationmark.triangle.fill")
                                .foregroundStyle(.orange)
                            Text(error)
                                .font(.caption)
                                .foregroundStyle(.red)
                        }
                    }
                }
                .padding()
            }
            
            Divider()
            
            // Footer
            HStack {
                Button("Cancel") {
                    showCreateProvider = false
                    resetCreateForm()
                }
                
                Spacer()
                
                if selectedTemplate != nil {
                    Button {
                        Task { await createProvider() }
                    } label: {
                        if isCreating {
                            ProgressView()
                                .scaleEffect(0.7)
                        } else {
                            Text("Create Provider")
                        }
                    }
                    .buttonStyle(.borderedProminent)
                    .disabled(isCreating || newProviderName.isEmpty || !isConfigValid())
                }
            }
            .padding()
        }
        .frame(width: 500, height: 500)
    }
    
    private var templateSelectionView: some View {
        VStack(alignment: .leading, spacing: 16) {
            Text("Choose a provider type")
                .font(.subheadline.weight(.medium))
            
            ForEach(templates) { template in
                Button {
                    selectedTemplate = template
                    // Set default values
                    for field in template.configSchema {
                        configValues[field.key] = field.defaultString
                    }
                    // Fetch available models for LLM providers
                    if template.service == "vertex_ai_llm" {
                        Task { await fetchModels(service: template.service) }
                    }
                } label: {
                    HStack(spacing: 12) {
                        Image(systemName: template.icon)
                            .font(.title2)
                            .foregroundStyle(.blue)
                            .frame(width: 40)
                        
                        VStack(alignment: .leading, spacing: 2) {
                            Text(template.name)
                                .font(.headline)
                            Text(template.description)
                                .font(.caption)
                                .foregroundStyle(.secondary)
                            
                            HStack(spacing: 8) {
                                Label(template.type.displayName, systemImage: template.type.icon)
                                Label(template.authType.displayName, systemImage: template.authType == .oauth ? "person.badge.key" : (template.authType == .apiKey ? "key" : "lock.open"))
                            }
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                        }
                        
                        Spacer()
                        
                        Image(systemName: "chevron.right")
                            .foregroundStyle(.secondary)
                    }
                    .padding()
                    .background(Color(nsColor: .controlBackgroundColor))
                    .cornerRadius(8)
                }
                .buttonStyle(.plain)
            }
        }
    }
    
    private func providerConfigForm(template: ProviderTemplate) -> some View {
        VStack(alignment: .leading, spacing: 16) {
            // Template info
            HStack(spacing: 12) {
                Image(systemName: template.icon)
                    .font(.title)
                    .foregroundStyle(.blue)
                VStack(alignment: .leading) {
                    Text(template.name)
                        .font(.headline)
                    Text(template.description)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            }
            .padding(.bottom, 8)
            
            Divider()
            
            // Provider name
            VStack(alignment: .leading, spacing: 4) {
                Text("Provider Name")
                    .font(.subheadline.weight(.medium))
                TextField("e.g., my-embeddings", text: $newProviderName)
                    .textFieldStyle(.roundedBorder)
                Text("A unique name to identify this provider")
                    .font(.caption2)
                    .foregroundStyle(.secondary)
            }
            
            // Auth config (for API key providers)
            if template.authType == .apiKey {
                VStack(alignment: .leading, spacing: 4) {
                    Text("API Key")
                        .font(.subheadline.weight(.medium))
                    SecureField("Enter your API key", text: Binding(
                        get: { authValues["api_key"] ?? "" },
                        set: { authValues["api_key"] = $0 }
                    ))
                    .textFieldStyle(.roundedBorder)
                }
            }
            
            // OAuth info
            if template.authType == .oauth {
                VStack(alignment: .leading, spacing: 8) {
                    HStack(spacing: 8) {
                        if isCheckingGoogleAuth {
                            ProgressView()
                                .scaleEffect(0.7)
                            Text("Checking authentication...")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        } else if let status = googleAuthStatus {
                            if status.authenticated {
                                Image(systemName: "checkmark.circle.fill")
                                    .foregroundStyle(.green)
                                if status.usingADC {
                                    VStack(alignment: .leading, spacing: 2) {
                                        Text("Authenticated via gcloud CLI")
                                            .font(.caption)
                                            .foregroundStyle(.secondary)
                                        Text("Using Application Default Credentials")
                                            .font(.caption2)
                                            .foregroundStyle(.secondary)
                                    }
                                } else {
                                    Text("Google account authenticated")
                                        .font(.caption)
                                        .foregroundStyle(.secondary)
                                }
                            } else if !status.hasCredentials && !status.hasADC {
                                Image(systemName: "exclamationmark.triangle.fill")
                                    .foregroundStyle(.orange)
                                VStack(alignment: .leading, spacing: 2) {
                                    Text("No Google credentials found")
                                        .font(.caption)
                                        .foregroundStyle(.orange)
                                    Text("Run 'gcloud auth application-default login' or place credentials.json in ~/.diane/secrets/google/")
                                        .font(.caption2)
                                        .foregroundStyle(.secondary)
                                }
                            } else {
                                Image(systemName: "person.badge.key")
                                    .foregroundStyle(.blue)
                                Text("Requires Google authentication")
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                                
                                Button("Authenticate") {
                                    showGoogleAuth = true
                                    Task { await startGoogleAuth() }
                                }
                                .buttonStyle(.borderedProminent)
                                .controlSize(.small)
                            }
                        } else {
                            Image(systemName: "person.badge.key")
                                .foregroundStyle(.secondary)
                            Text("Uses Google OAuth authentication")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                            
                            Button("Check Status") {
                                Task { await checkGoogleAuthStatus() }
                            }
                            .controlSize(.small)
                        }
                    }
                }
                .padding(.vertical, 4)
                .task {
                    await checkGoogleAuthStatus()
                }
            }
            
            // Config fields
            ForEach(template.configSchema) { field in
                configFieldView(field: field, values: $configValues)
            }
        }
    }
    
    private func configFieldView(field: ConfigField, values: Binding<[String: String]>) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                Text(field.label)
                    .font(.subheadline.weight(.medium))
                if field.required {
                    Text("*")
                        .foregroundStyle(.red)
                }
                
                // Show loading indicator for model field when fetching
                if field.key == "model" && isLoadingModels {
                    ProgressView()
                        .scaleEffect(0.6)
                }
            }
            
            // Use dynamic models for the model field if available
            if field.key == "model" && !availableModels.isEmpty {
                VStack(alignment: .leading, spacing: 4) {
                    Picker("", selection: Binding(
                        get: { values.wrappedValue[field.key] ?? field.defaultString },
                        set: { values.wrappedValue[field.key] = $0 }
                    )) {
                        ForEach(availableModels) { model in
                            HStack {
                                Text(model.id)
                                if let stage = model.launchStage, stage != "GA" {
                                    Text("(\(stage))")
                                        .foregroundStyle(.secondary)
                                }
                            }
                            .tag(model.id)
                        }
                    }
                    .labelsHidden()
                    .frame(maxWidth: .infinity, alignment: .leading)
                    
                    // Show pricing for selected model
                    if let selectedModelID = values.wrappedValue[field.key],
                       let selectedModel = availableModels.first(where: { $0.id == selectedModelID }) {
                        HStack(spacing: 12) {
                            if let pricing = selectedModel.pricingInfo {
                                HStack(spacing: 4) {
                                    Image(systemName: "dollarsign.circle")
                                        .font(.caption2)
                                    Text(pricing)
                                }
                                .foregroundStyle(.secondary)
                            }
                            if let limits = selectedModel.limits {
                                HStack(spacing: 4) {
                                    Image(systemName: "text.word.spacing")
                                        .font(.caption2)
                                    Text(limits.limitsFormatted)
                                }
                                .foregroundStyle(.secondary)
                                .help("Context: \(limits.context.formatted()) tokens, Output: \(limits.output.formatted()) tokens")
                            }
                            if selectedModel.toolCall ?? false {
                                HStack(spacing: 2) {
                                    Image(systemName: "wrench.and.screwdriver")
                                        .font(.caption2)
                                    Text("Tools")
                                }
                                .foregroundStyle(.blue)
                            }
                            if selectedModel.reasoning ?? false {
                                HStack(spacing: 2) {
                                    Image(systemName: "brain")
                                        .font(.caption2)
                                    Text("Reasoning")
                                }
                                .foregroundStyle(.purple)
                            }
                        }
                        .font(.caption2)
                    }
                }
            } else {
                switch field.type {
                case "select":
                    Picker("", selection: Binding(
                        get: { values.wrappedValue[field.key] ?? field.defaultString },
                        set: { values.wrappedValue[field.key] = $0 }
                    )) {
                        ForEach(field.options ?? [], id: \.self) { option in
                            Text(option).tag(option)
                        }
                    }
                    .labelsHidden()
                    .frame(maxWidth: .infinity, alignment: .leading)
                    
                case "int":
                    TextField("", text: Binding(
                        get: { values.wrappedValue[field.key] ?? field.defaultString },
                        set: { values.wrappedValue[field.key] = $0 }
                    ))
                    .textFieldStyle(.roundedBorder)
                    
                case "bool":
                    Toggle("", isOn: Binding(
                        get: { values.wrappedValue[field.key] == "true" },
                        set: { values.wrappedValue[field.key] = $0 ? "true" : "false" }
                    ))
                    .labelsHidden()
                    
                default: // string
                    TextField("", text: Binding(
                        get: { values.wrappedValue[field.key] ?? "" },
                        set: { values.wrappedValue[field.key] = $0 }
                    ))
                    .textFieldStyle(.roundedBorder)
                }
            }
            
            if let description = field.description {
                Text(description)
                    .font(.caption2)
                    .foregroundStyle(.secondary)
            }
            
            // Show models error if any
            if field.key == "model", let error = modelsError {
                Text(error)
                    .font(.caption2)
                    .foregroundStyle(.red)
            }
        }
    }
    
    // MARK: - Edit Provider Sheet
    
    @ViewBuilder
    private func editProviderSheet(for provider: Provider) -> some View {
        VStack(spacing: 0) {
            // Header
            HStack {
                Text("Edit Provider")
                    .font(.headline)
                Spacer()
                Button {
                    editingProvider = nil
                } label: {
                    Image(systemName: "xmark.circle.fill")
                        .foregroundStyle(.secondary)
                }
                .buttonStyle(.plain)
            }
            .padding()
            
            Divider()
            
            // Content
            if let template = templates.first(where: { $0.service == provider.service }) {
                ScrollView {
                    VStack(alignment: .leading, spacing: 16) {
                        // Provider info
                        HStack(spacing: 12) {
                            Image(systemName: template.icon)
                                .font(.title)
                                .foregroundStyle(.blue)
                            VStack(alignment: .leading) {
                                Text(provider.name)
                                    .font(.headline)
                                Text(template.name)
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                            }
                        }
                        .padding(.bottom, 8)
                        
                        Divider()
                        
                        // Auth config (for API key providers)
                        if template.authType == .apiKey {
                            VStack(alignment: .leading, spacing: 4) {
                                Text("API Key")
                                    .font(.subheadline.weight(.medium))
                                SecureField("Enter new API key (leave empty to keep current)", text: Binding(
                                    get: { editAuthValues["api_key"] ?? "" },
                                    set: { editAuthValues["api_key"] = $0 }
                                ))
                                .textFieldStyle(.roundedBorder)
                                Text("Current key is masked. Enter a new key to update.")
                                    .font(.caption2)
                                    .foregroundStyle(.secondary)
                            }
                        }
                        
                        // Config fields
                        ForEach(template.configSchema) { field in
                            configFieldView(field: field, values: $editConfigValues)
                        }
                        
                        // Error message
                        if let error = editError {
                            HStack {
                                Image(systemName: "exclamationmark.triangle.fill")
                                    .foregroundStyle(.orange)
                                Text(error)
                                    .font(.caption)
                                    .foregroundStyle(.red)
                            }
                        }
                    }
                    .padding()
                }
            }
            
            Divider()
            
            // Footer
            HStack {
                Button("Cancel") {
                    editingProvider = nil
                }
                
                Spacer()
                
                Button {
                    Task { await updateProvider() }
                } label: {
                    if isEditing {
                        ProgressView()
                            .scaleEffect(0.7)
                    } else {
                        Text("Save Changes")
                    }
                }
                .buttonStyle(.borderedProminent)
                .disabled(isEditing)
            }
            .padding()
        }
        .frame(width: 450, height: 450)
        .onAppear {
            // Initialize edit values when sheet appears
            prepareEdit(provider)
        }
    }
    
    // MARK: - Data Operations
    
    private func loadData() async {
        isLoading = true
        error = nil
        
        do {
            async let providersTask = client.getProviders(type: typeFilter)
            async let templatesTask = client.getProviderTemplates()
            
            let (loadedProviders, loadedTemplates) = try await (providersTask, templatesTask)
            
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
    
    private func createProvider() async {
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
    
    private func updateProvider() async {
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
    
    private func toggleProvider(_ provider: Provider, enabled: Bool) async {
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
    
    private func setDefault(_ provider: Provider) async {
        do {
            let updated = try await client.setDefaultProvider(id: provider.id)
            
            // Refresh all providers to update isDefault flags
            await loadData()
            
            selectedProvider = updated
        } catch {
            self.error = error.localizedDescription
        }
    }
    
    private func deleteProvider(_ provider: Provider) async {
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
    
    private func prepareEdit(_ provider: Provider) {
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
    
    private func resetCreateForm() {
        selectedTemplate = nil
        newProviderName = ""
        configValues = [:]
        authValues = [:]
        createError = nil
        availableModels = []
        modelsError = nil
    }
    
    private func fetchModels(service: String) async {
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
    
    private func isConfigValid() -> Bool {
        guard let template = selectedTemplate else { return false }
        
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
    
    // MARK: - Google OAuth Sheet
    
    private var googleAuthSheet: some View {
        VStack(spacing: 0) {
            // Header
            HStack {
                Image(systemName: "person.badge.key")
                    .foregroundStyle(.blue)
                Text("Google Authentication")
                    .font(.headline)
                Spacer()
                Button {
                    showGoogleAuth = false
                    resetGoogleAuth()
                } label: {
                    Image(systemName: "xmark.circle.fill")
                        .foregroundStyle(.secondary)
                }
                .buttonStyle(.plain)
            }
            .padding()
            
            Divider()
            
            // Content
            VStack(spacing: 20) {
                if let error = googleAuthError {
                    // Error state
                    VStack(spacing: 12) {
                        Image(systemName: "exclamationmark.triangle.fill")
                            .font(.system(size: 40))
                            .foregroundStyle(.orange)
                        Text("Authentication Failed")
                            .font(.headline)
                        Text(error)
                            .font(.caption)
                            .foregroundStyle(.secondary)
                            .multilineTextAlignment(.center)
                        
                        Button("Try Again") {
                            googleAuthError = nil
                            Task { await startGoogleAuth() }
                        }
                        .buttonStyle(.borderedProminent)
                    }
                } else if let dcr = deviceCodeResponse {
                    // Show device code
                    VStack(spacing: 16) {
                        Text("Sign in to your Google account")
                            .font(.headline)
                        
                        Text("Visit this URL in your browser:")
                            .font(.subheadline)
                            .foregroundStyle(.secondary)
                        
                        // Verification URL
                        HStack {
                            Text(dcr.verificationURL)
                                .font(.system(.body, design: .monospaced))
                                .foregroundStyle(.blue)
                            
                            Button {
                                NSPasteboard.general.clearContents()
                                NSPasteboard.general.setString(dcr.verificationURL, forType: .string)
                            } label: {
                                Image(systemName: "doc.on.doc")
                            }
                            .buttonStyle(.plain)
                            .help("Copy URL")
                            
                            Button {
                                if let url = URL(string: dcr.verificationURL) {
                                    NSWorkspace.shared.open(url)
                                }
                            } label: {
                                Image(systemName: "safari")
                            }
                            .buttonStyle(.plain)
                            .help("Open in browser")
                        }
                        .padding()
                        .background(Color(nsColor: .controlBackgroundColor))
                        .cornerRadius(8)
                        
                        Text("Then enter this code:")
                            .font(.subheadline)
                            .foregroundStyle(.secondary)
                        
                        // User code
                        HStack(spacing: 12) {
                            Text(dcr.userCode)
                                .font(.system(size: 28, weight: .bold, design: .monospaced))
                                .tracking(4)
                            
                            Button {
                                NSPasteboard.general.clearContents()
                                NSPasteboard.general.setString(dcr.userCode, forType: .string)
                            } label: {
                                Image(systemName: "doc.on.doc")
                            }
                            .buttonStyle(.plain)
                            .help("Copy code")
                        }
                        .padding()
                        .background(Color(nsColor: .controlBackgroundColor))
                        .cornerRadius(8)
                        
                        if isPollingForToken {
                            HStack(spacing: 8) {
                                ProgressView()
                                    .scaleEffect(0.8)
                                Text("Waiting for authorization...")
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                            }
                        }
                        
                        Text("Code expires in \(dcr.expiresIn / 60) minutes")
                            .font(.caption2)
                            .foregroundStyle(.tertiary)
                    }
                } else {
                    // Loading state
                    VStack(spacing: 12) {
                        ProgressView()
                        Text("Starting authentication...")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }
            }
            .padding()
            .frame(maxWidth: .infinity, maxHeight: .infinity)
            
            Divider()
            
            // Footer
            HStack {
                Button("Cancel") {
                    showGoogleAuth = false
                    resetGoogleAuth()
                }
                Spacer()
            }
            .padding()
        }
        .frame(width: 450, height: 350)
    }
    
    // MARK: - Google OAuth Functions
    
    private func checkGoogleAuthStatus() async {
        isCheckingGoogleAuth = true
        
        do {
            googleAuthStatus = try await client.getGoogleAuthStatus()
        } catch {
            // Status check failed, leave as nil
            googleAuthStatus = nil
        }
        
        isCheckingGoogleAuth = false
    }
    
    private func startGoogleAuth() async {
        deviceCodeResponse = nil
        googleAuthError = nil
        
        do {
            // Start device flow
            deviceCodeResponse = try await client.startGoogleAuth()
            
            // Start polling for token
            if let dcr = deviceCodeResponse {
                isPollingForToken = true
                await pollForToken(deviceCode: dcr.deviceCode, interval: dcr.interval)
            }
        } catch {
            googleAuthError = error.localizedDescription
        }
    }
    
    private func pollForToken(deviceCode: String, interval: Int) async {
        let pollInterval = max(interval, 5) // At least 5 seconds
        
        while isPollingForToken && showGoogleAuth {
            // Wait before polling
            try? await Task.sleep(nanoseconds: UInt64(pollInterval) * 1_000_000_000)
            
            guard showGoogleAuth else { break }
            
            do {
                let response = try await client.pollGoogleAuth(deviceCode: deviceCode, interval: pollInterval)
                
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
    
    private func resetGoogleAuth() {
        deviceCodeResponse = nil
        isPollingForToken = false
        googleAuthError = nil
    }
}

// MARK: - Provider Row

struct ProviderRow: View {
    let provider: Provider
    let isSelected: Bool
    let onSelect: () -> Void
    let onToggle: (Bool) -> Void
    let onSetDefault: () -> Void
    let onDelete: () -> Void
    
    @State private var isHovering = false
    
    var body: some View {
        HStack(spacing: 10) {
            // Service icon
            Image(systemName: provider.serviceIcon)
                .font(.title3)
                .foregroundStyle(provider.enabled ? .blue : .secondary)
                .frame(width: 24)
            
            VStack(alignment: .leading, spacing: 2) {
                HStack(spacing: 6) {
                    Text(provider.name)
                        .font(.subheadline.weight(.medium))
                        .lineLimit(1)
                    
                    if provider.isDefault {
                        Text("Default")
                            .font(.caption2)
                            .padding(.horizontal, 4)
                            .padding(.vertical, 1)
                            .background(Color.green.opacity(0.2))
                            .foregroundStyle(.green)
                            .cornerRadius(3)
                    }
                }
                
                HStack(spacing: 6) {
                    Text(provider.serviceName)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    
                    Text("•")
                        .font(.caption2)
                        .foregroundStyle(.quaternary)
                    
                    Text(provider.type.displayName)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            }
            
            Spacer()
            
            // Enable/disable toggle
            Toggle("", isOn: Binding(
                get: { provider.enabled },
                set: { onToggle($0) }
            ))
            .toggleStyle(.switch)
            .labelsHidden()
            .scaleEffect(0.7)
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 8)
        .background(isSelected ? Color.accentColor.opacity(0.1) : (isHovering ? Color.primary.opacity(0.05) : Color.clear))
        .contentShape(Rectangle())
        .onTapGesture {
            onSelect()
        }
        .onHover { hovering in
            isHovering = hovering
        }
        .contextMenu {
            if !provider.isDefault {
                Button {
                    onSetDefault()
                } label: {
                    Label("Set as Default", systemImage: "star")
                }
            }
            
            Divider()
            
            Button(role: .destructive) {
                onDelete()
            } label: {
                Label("Delete", systemImage: "trash")
            }
        }
    }
}

// MARK: - Provider Detail View

struct ProviderDetailView: View {
    let provider: Provider
    let template: ProviderTemplate?
    let onEdit: () -> Void
    let onToggle: (Bool) -> Void
    let onSetDefault: () -> Void
    let onDelete: () -> Void
    let onAuthenticate: () -> Void  // New callback for triggering auth
    
    // Test state
    @State private var isTesting = false
    @State private var testResult: ProviderTestResult?
    @State private var testError: String?
    
    // Google auth state
    @State private var googleAuthStatus: GoogleAuthStatus?
    @State private var isLoadingAuthStatus = false
    @State private var showAuthOptions = false
    
    // Model info state
    @State private var modelInfo: AvailableModel?
    @State private var isLoadingModelInfo = false
    
    private let client = DianeClient()
    
    /// Check if the test result indicates an auth problem
    private var isAuthError: Bool {
        guard let result = testResult, !result.success else { return false }
        let message = result.message.lowercased()
        return message.contains("oauth") || 
               message.contains("token") || 
               message.contains("credentials") ||
               message.contains("authentication") ||
               message.contains("unauthorized")
    }
    
    /// Check if this provider uses OAuth
    private var usesOAuth: Bool {
        provider.authType == .oauth
    }
    
    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                // Header
                HStack(spacing: 16) {
                    Image(systemName: provider.serviceIcon)
                        .font(.system(size: 40))
                        .foregroundStyle(.blue)
                    
                    VStack(alignment: .leading, spacing: 4) {
                        HStack(spacing: 8) {
                            Text(provider.name)
                                .font(.title2.weight(.semibold))
                            
                            if provider.isDefault {
                                Text("Default")
                                    .font(.caption)
                                    .padding(.horizontal, 6)
                                    .padding(.vertical, 2)
                                    .background(Color.green.opacity(0.2))
                                    .foregroundStyle(.green)
                                    .cornerRadius(4)
                            }
                            
                            Text(provider.enabled ? "Enabled" : "Disabled")
                                .font(.caption)
                                .padding(.horizontal, 6)
                                .padding(.vertical, 2)
                                .background(provider.enabled ? Color.blue.opacity(0.2) : Color.gray.opacity(0.2))
                                .foregroundStyle(provider.enabled ? .blue : .gray)
                                .cornerRadius(4)
                        }
                        
                        Text("\(provider.serviceName) • \(provider.type.displayName)")
                            .font(.subheadline)
                            .foregroundStyle(.secondary)
                    }
                    
                    Spacer()
                    
                    // Actions
                    HStack(spacing: 8) {
                        Button {
                            onEdit()
                        } label: {
                            Label("Edit", systemImage: "pencil")
                        }
                        
                        if !provider.isDefault {
                            Button {
                                onSetDefault()
                            } label: {
                                Label("Set Default", systemImage: "star")
                            }
                        }
                    }
                }
                .padding()
                .background(Color(nsColor: .controlBackgroundColor))
                .cornerRadius(8)
                
                // Configuration section
                VStack(alignment: .leading, spacing: 12) {
                    Text("Configuration")
                        .font(.headline)
                    
                    if let template = template {
                        ForEach(template.configSchema) { field in
                            HStack {
                                Text(field.label)
                                    .foregroundStyle(.secondary)
                                Spacer()
                                Text(provider.getConfigString(field.key).isEmpty ? "-" : provider.getConfigString(field.key))
                                    .font(.system(.body, design: .monospaced))
                            }
                            .padding(.vertical, 4)
                            
                            if field.key != template.configSchema.last?.key {
                                Divider()
                            }
                        }
                    } else {
                        // Show raw config if no template
                        ForEach(Array(provider.config.keys.sorted()), id: \.self) { key in
                            HStack {
                                Text(key)
                                    .foregroundStyle(.secondary)
                                Spacer()
                                Text(provider.getConfigString(key))
                                    .font(.system(.body, design: .monospaced))
                            }
                            .padding(.vertical, 4)
                        }
                    }
                }
                .padding()
                .background(Color(nsColor: .controlBackgroundColor))
                .cornerRadius(8)
                
                // Model Info section (for LLM providers with model config)
                if provider.type == .llm, let modelID = provider.config["model"]?.value as? String, !modelID.isEmpty {
                    VStack(alignment: .leading, spacing: 12) {
                        HStack {
                            Text("Model Info")
                                .font(.headline)
                            Spacer()
                            if isLoadingModelInfo {
                                ProgressView()
                                    .scaleEffect(0.7)
                            }
                        }
                        
                        if let info = modelInfo {
                            // Model name
                            HStack {
                                Text("Model")
                                    .foregroundStyle(.secondary)
                                Spacer()
                                Text(info.displayName)
                            }
                            
                            // Limits
                            if let limits = info.limits {
                                Divider()
                                HStack {
                                    Text("Context Limit")
                                        .foregroundStyle(.secondary)
                                    Spacer()
                                    Text("\(limits.contextFormatted) tokens")
                                        .help("\(limits.context.formatted()) tokens")
                                }
                                
                                Divider()
                                HStack {
                                    Text("Output Limit")
                                        .foregroundStyle(.secondary)
                                    Spacer()
                                    Text("\(limits.outputFormatted) tokens")
                                        .help("\(limits.output.formatted()) tokens")
                                }
                            }
                            
                            // Pricing
                            if let cost = info.cost {
                                Divider()
                                HStack {
                                    Text("Pricing (per 1M)")
                                        .foregroundStyle(.secondary)
                                    Spacer()
                                    Text("$\(String(format: "%.2f", cost.input)) in / $\(String(format: "%.2f", cost.output)) out")
                                }
                            }
                            
                            // Capabilities
                            Divider()
                            HStack {
                                Text("Capabilities")
                                    .foregroundStyle(.secondary)
                                Spacer()
                                HStack(spacing: 8) {
                                    if info.toolCall ?? false {
                                        HStack(spacing: 2) {
                                            Image(systemName: "wrench.and.screwdriver")
                                                .font(.caption2)
                                            Text("Tools")
                                        }
                                        .foregroundStyle(.blue)
                                    }
                                    if info.reasoning ?? false {
                                        HStack(spacing: 2) {
                                            Image(systemName: "brain")
                                                .font(.caption2)
                                            Text("Reasoning")
                                        }
                                        .foregroundStyle(.purple)
                                    }
                                    if !(info.toolCall ?? false) && !(info.reasoning ?? false) {
                                        Text("Standard")
                                            .foregroundStyle(.secondary)
                                    }
                                }
                                .font(.caption)
                            }
                        } else if !isLoadingModelInfo {
                            Text("Model info not available")
                                .foregroundStyle(.secondary)
                                .font(.caption)
                        }
                    }
                    .padding()
                    .background(Color(nsColor: .controlBackgroundColor))
                    .cornerRadius(8)
                    .task {
                        await loadModelInfo(modelID: modelID)
                    }
                }
                
                // Authentication section
                VStack(alignment: .leading, spacing: 12) {
                    HStack {
                        Text("Authentication")
                            .font(.headline)
                        Spacer()
                        
                        // Show options menu for OAuth providers
                        if provider.authType == .oauth {
                            Menu {
                                Button {
                                    onAuthenticate()
                                } label: {
                                    Label("Authenticate with Device Flow", systemImage: "person.badge.key")
                                }
                                
                                if let status = googleAuthStatus, status.hasToken {
                                    Button(role: .destructive) {
                                        Task { await clearOAuthToken() }
                                    } label: {
                                        Label("Clear OAuth Token (Use ADC)", systemImage: "trash")
                                    }
                                }
                                
                                Divider()
                                
                                Button {
                                    Task { await loadAuthStatus() }
                                } label: {
                                    Label("Refresh Status", systemImage: "arrow.clockwise")
                                }
                            } label: {
                                Label("Options", systemImage: "ellipsis.circle")
                            }
                            .controlSize(.small)
                        }
                    }
                    
                    HStack {
                        Text("Type")
                            .foregroundStyle(.secondary)
                        Spacer()
                        // Show actual credential source, not just the auth type
                        if provider.authType == .oauth {
                            if let status = googleAuthStatus {
                                if status.usingADC {
                                    Text("gcloud CLI (ADC)")
                                } else if status.hasToken {
                                    Text("OAuth Token")
                                } else {
                                    Text("OAuth")
                                }
                            } else {
                                Text(provider.authType.displayName)
                            }
                        } else {
                            Text(provider.authType.displayName)
                        }
                    }
                    
                    if provider.authType == .oauth {
                        Divider()
                        HStack {
                            Text("Account")
                                .foregroundStyle(.secondary)
                            Spacer()
                            Text(provider.getAuthString("oauth_account").isEmpty ? "default" : provider.getAuthString("oauth_account"))
                        }
                        
                        Divider()
                        HStack {
                            Text("Status")
                                .foregroundStyle(.secondary)
                            Spacer()
                            
                            if isLoadingAuthStatus {
                                ProgressView()
                                    .scaleEffect(0.7)
                            } else if let status = googleAuthStatus {
                                if status.authenticated {
                                    HStack(spacing: 6) {
                                        Image(systemName: "checkmark.circle.fill")
                                            .foregroundStyle(.green)
                                        Text("Authenticated")
                                            .foregroundStyle(.green)
                                    }
                                } else {
                                    HStack(spacing: 6) {
                                        Image(systemName: "xmark.circle.fill")
                                            .foregroundStyle(.red)
                                        Text("Not authenticated")
                                            .foregroundStyle(.red)
                                    }
                                }
                            } else {
                                Text("Unknown")
                                    .foregroundStyle(.secondary)
                            }
                        }
                        
                        // Show hint about ADC if not authenticated
                        if let status = googleAuthStatus, !status.authenticated && !status.hasADC {
                            Divider()
                            VStack(alignment: .leading, spacing: 4) {
                                Text("No credentials found")
                                    .font(.caption)
                                    .foregroundStyle(.orange)
                                Text("Use the Options menu to authenticate, or run:\ngcloud auth application-default login")
                                    .font(.caption2)
                                    .foregroundStyle(.secondary)
                            }
                        }
                    }
                    
                    if provider.authType == .apiKey {
                        Divider()
                        HStack {
                            Text("API Key")
                                .foregroundStyle(.secondary)
                            Spacer()
                            Text(provider.getAuthString("api_key"))
                                .font(.system(.body, design: .monospaced))
                        }
                    }
                }
                .padding()
                .background(Color(nsColor: .controlBackgroundColor))
                .cornerRadius(8)
                .task {
                    if provider.authType == .oauth {
                        await loadAuthStatus()
                    }
                }
                
                // Test Connection section
                VStack(alignment: .leading, spacing: 12) {
                    HStack {
                        Text("Connection Test")
                            .font(.headline)
                        Spacer()
                        Button {
                            Task { await testProvider() }
                        } label: {
                            if isTesting {
                                HStack(spacing: 6) {
                                    ProgressView()
                                        .scaleEffect(0.7)
                                    Text("Testing...")
                                }
                            } else {
                                Label("Test Connection", systemImage: "bolt.circle")
                            }
                        }
                        .disabled(isTesting || !provider.enabled)
                    }
                    
                    // Test result display
                    if let result = testResult {
                        Divider()
                        
                        HStack {
                            Image(systemName: result.success ? "checkmark.circle.fill" : "xmark.circle.fill")
                                .foregroundStyle(result.success ? .green : .red)
                            Text(result.message)
                                .font(.subheadline)
                            Spacer()
                            
                            // Show authenticate button for OAuth errors
                            if isAuthError && usesOAuth {
                                Button {
                                    onAuthenticate()
                                } label: {
                                    Label("Authenticate", systemImage: "person.badge.key")
                                }
                                .buttonStyle(.borderedProminent)
                                .controlSize(.small)
                            }
                        }
                        
                        if result.responseTimeMs > 0 {
                            Divider()
                            HStack {
                                Text("Response Time")
                                    .foregroundStyle(.secondary)
                                Spacer()
                                Text(String(format: "%.0f ms", result.responseTimeMs))
                                    .font(.system(.body, design: .monospaced))
                            }
                        }
                        
                        // Show details if available
                        if let details = result.details, !details.isEmpty {
                            let embeddingLength = result.getDetail("embedding_length")
                            let model = result.getDetail("model")
                            let location = result.getDetail("location")
                            let response = result.getDetail("response")
                            let inputTokens = result.getDetail("input_tokens")
                            let outputTokens = result.getDetail("output_tokens")
                            
                            if !model.isEmpty {
                                Divider()
                                HStack {
                                    Text("Model")
                                        .foregroundStyle(.secondary)
                                    Spacer()
                                    Text(model)
                                        .font(.system(.body, design: .monospaced))
                                }
                            }
                            
                            if !location.isEmpty {
                                Divider()
                                HStack {
                                    Text("Location")
                                        .foregroundStyle(.secondary)
                                    Spacer()
                                    Text(location)
                                        .font(.system(.body, design: .monospaced))
                                }
                            }
                            
                            // Embedding-specific
                            if !embeddingLength.isEmpty {
                                Divider()
                                HStack {
                                    Text("Embedding Dimensions")
                                        .foregroundStyle(.secondary)
                                    Spacer()
                                    Text(embeddingLength)
                                        .font(.system(.body, design: .monospaced))
                                }
                            }
                            
                            // LLM-specific
                            if !response.isEmpty {
                                Divider()
                                HStack(alignment: .top) {
                                    Text("Response")
                                        .foregroundStyle(.secondary)
                                    Spacer()
                                    Text(response)
                                        .font(.system(.body, design: .monospaced))
                                        .multilineTextAlignment(.trailing)
                                }
                            }
                            
                            if !inputTokens.isEmpty || !outputTokens.isEmpty {
                                Divider()
                                HStack {
                                    Text("Tokens")
                                        .foregroundStyle(.secondary)
                                    Spacer()
                                    Text("\(inputTokens) in / \(outputTokens) out")
                                        .font(.system(.body, design: .monospaced))
                                }
                            }
                        }
                    } else if let error = testError {
                        Divider()
                        HStack {
                            Image(systemName: "exclamationmark.triangle.fill")
                                .foregroundStyle(.orange)
                            Text(error)
                                .font(.subheadline)
                                .foregroundStyle(.red)
                            Spacer()
                        }
                    } else if !provider.enabled {
                        Divider()
                        HStack {
                            Image(systemName: "info.circle")
                                .foregroundStyle(.secondary)
                            Text("Enable provider to test connection")
                                .font(.subheadline)
                                .foregroundStyle(.secondary)
                            Spacer()
                        }
                    }
                }
                .padding()
                .background(Color(nsColor: .controlBackgroundColor))
                .cornerRadius(8)
                
                // Metadata section
                VStack(alignment: .leading, spacing: 12) {
                    Text("Metadata")
                        .font(.headline)
                    
                    HStack {
                        Text("Service")
                            .foregroundStyle(.secondary)
                        Spacer()
                        Text(provider.service)
                            .font(.system(.body, design: .monospaced))
                    }
                    
                    Divider()
                    
                    HStack {
                        Text("Type")
                            .foregroundStyle(.secondary)
                        Spacer()
                        Text(provider.type.rawValue)
                            .font(.system(.body, design: .monospaced))
                    }
                    
                    Divider()
                    
                    HStack {
                        Text("ID")
                            .foregroundStyle(.secondary)
                        Spacer()
                        Text("\(provider.id)")
                            .font(.system(.body, design: .monospaced))
                    }
                }
                .padding()
                .background(Color(nsColor: .controlBackgroundColor))
                .cornerRadius(8)
                
                // Delete button
                HStack {
                    Spacer()
                    Button(role: .destructive) {
                        onDelete()
                    } label: {
                        Label("Delete Provider", systemImage: "trash")
                    }
                }
                .padding(.top, 8)
            }
            .padding()
        }
        .onChange(of: provider.id) { _, _ in
            // Clear test results when provider changes
            testResult = nil
            testError = nil
            googleAuthStatus = nil
        }
    }
    
    private func testProvider() async {
        isTesting = true
        testResult = nil
        testError = nil
        
        do {
            testResult = try await client.testProvider(id: provider.id)
        } catch {
            testError = error.localizedDescription
        }
        
        isTesting = false
    }
    
    private func loadAuthStatus() async {
        guard provider.authType == .oauth else { return }
        
        isLoadingAuthStatus = true
        let account = provider.getAuthString("oauth_account").isEmpty ? "default" : provider.getAuthString("oauth_account")
        
        do {
            googleAuthStatus = try await client.getGoogleAuthStatus(account: account)
        } catch {
            print("Failed to load auth status: \(error)")
        }
        
        isLoadingAuthStatus = false
    }
    
    private func clearOAuthToken() async {
        let account = provider.getAuthString("oauth_account").isEmpty ? "default" : provider.getAuthString("oauth_account")
        
        do {
            try await client.deleteGoogleAuth(account: account)
            await loadAuthStatus()
        } catch {
            print("Failed to clear OAuth token: \(error)")
        }
    }
    
    private func loadModelInfo(modelID: String) async {
        guard !modelID.isEmpty else { return }
        
        isLoadingModelInfo = true
        
        // Map service to models.dev provider ID
        let providerID: String
        switch provider.service {
        case "vertex_ai_llm":
            providerID = "google-vertex"
        case "openai":
            providerID = "openai"
        case "anthropic":
            providerID = "anthropic"
        default:
            providerID = provider.service
        }
        
        do {
            modelInfo = try await client.getModelInfo(provider: providerID, modelID: modelID)
        } catch {
            print("Failed to load model info: \(error)")
        }
        
        isLoadingModelInfo = false
    }
}

#Preview {
    ProvidersView()
}
