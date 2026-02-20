import SwiftUI
import AppKit

struct ProvidersView: View {
    @EnvironmentObject var statusMonitor: StatusMonitor
    @State private var viewModel: ProvidersViewModel
    @State private var clientInitialized = false
    
    init(viewModel: ProvidersViewModel = ProvidersViewModel()) {
        _viewModel = State(initialValue: viewModel)
    }
    
    var body: some View {
        VStack(spacing: 0) {
            headerView
            
            Divider()
            
            if viewModel.isLoading {
                loadingView
            } else if let error = viewModel.error {
                errorView(error)
            } else if viewModel.providers.isEmpty {
                emptyView
            } else {
                MasterDetailView {
                    providersListView
                } detail: {
                    detailView
                }
            }
        }
        .frame(minWidth: 700, idealWidth: 800, maxWidth: .infinity,
               minHeight: 400, idealHeight: 500, maxHeight: .infinity)
        .task {
            // Initialize with the correct client from StatusMonitor if available
            if !clientInitialized, let configuredClient = statusMonitor.configuredClient {
                viewModel = ProvidersViewModel(client: configuredClient)
                clientInitialized = true
            }
            await viewModel.loadData()
        }
        .sheet(isPresented: $viewModel.showCreateProvider) {
            createProviderSheet
        }
        .sheet(item: $viewModel.editingProvider) { provider in
            editProviderSheet(for: provider)
        }
        .alert("Delete Provider", isPresented: $viewModel.showDeleteConfirm) {
            Button("Cancel", role: .cancel) { }
            Button("Delete", role: .destructive) {
                if let provider = viewModel.providerToDelete {
                    Task { await viewModel.deleteProvider(provider) }
                }
            }
        } message: {
            if let provider = viewModel.providerToDelete {
                Text("Are you sure you want to delete '\(provider.name)'? This cannot be undone.")
            }
        }
        .sheet(isPresented: $viewModel.showGoogleAuth) {
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
            Picker("", selection: $viewModel.typeFilter) {
                Text("All").tag(nil as ProviderType?)
                ForEach(ProviderType.allCases, id: \.self) { type in
                    Label(type.displayName, systemImage: type.icon).tag(type as ProviderType?)
                }
            }
            .pickerStyle(.menu)
            .frame(width: 130)
            
            // Create provider button
            Button {
                viewModel.showCreateProvider = true
            } label: {
                Label("Add Provider", systemImage: "plus")
            }
            
            // Refresh button
            Button {
                Task { await viewModel.loadData() }
            } label: {
                Image(systemName: "arrow.clockwise")
            }
            .disabled(viewModel.isLoading)
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
                Task { await viewModel.loadData() }
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
    
    // MARK: - Empty View
    
    private var emptyView: some View {
        EmptyStateView(
            icon: "cpu",
            title: "No providers configured",
            description: "Add a provider to enable features like embeddings, LLM, or storage",
            actionLabel: "Add Provider",
            action: { viewModel.showCreateProvider = true }
        )
    }
    
    // MARK: - Providers List
    
    private var providersListView: some View {
        VStack(alignment: .leading, spacing: 0) {
            // Section header
            MasterListHeader(icon: "list.bullet", title: "Configured Providers")
            
            Divider()
            
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 0) {
                    ForEach(viewModel.filteredProviders) { provider in
                        ProviderRow(
                            provider: provider,
                            isSelected: viewModel.selectedProvider?.id == provider.id,
                            onSelect: {
                                viewModel.selectedProvider = provider
                            },
                            onToggle: { enabled in
                                Task { await viewModel.toggleProvider(provider, enabled: enabled) }
                            },
                            onSetDefault: {
                                Task { await viewModel.setDefault(provider) }
                            },
                            onDelete: {
                                viewModel.providerToDelete = provider
                                viewModel.showDeleteConfirm = true
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
            if let provider = viewModel.selectedProvider {
                ProviderDetailView(
                    provider: provider,
                    template: viewModel.templates.first { $0.service == provider.service },
                    onEdit: {
                        viewModel.editingProvider = provider
                    },
                    onToggle: { enabled in
                        Task { await viewModel.toggleProvider(provider, enabled: enabled) }
                    },
                    onSetDefault: {
                        Task { await viewModel.setDefault(provider) }
                    },
                    onDelete: {
                        viewModel.providerToDelete = provider
                        viewModel.showDeleteConfirm = true
                    },
                    onAuthenticate: {
                        viewModel.showGoogleAuth = true
                        Task { await viewModel.startGoogleAuth() }
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
                    viewModel.showCreateProvider = false
                    viewModel.resetCreateForm()
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
                    if viewModel.selectedTemplate == nil {
                        templateSelectionView
                    } else {
                        // Back button
                        Button {
                            viewModel.selectedTemplate = nil
                            viewModel.resetCreateForm()
                        } label: {
                            Label("Back to templates", systemImage: "chevron.left")
                        }
                        .buttonStyle(.plain)
                        .foregroundStyle(.secondary)
                        
                        // Provider configuration
                        if let template = viewModel.selectedTemplate {
                            providerConfigForm(template: template)
                        }
                    }
                    
                    // Error message
                    if let error = viewModel.createError {
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
                    viewModel.showCreateProvider = false
                    viewModel.resetCreateForm()
                }
                
                Spacer()
                
                if viewModel.selectedTemplate != nil {
                    Button {
                        Task { await viewModel.createProvider() }
                    } label: {
                        if viewModel.isCreating {
                            ProgressView()
                                .scaleEffect(0.7)
                        } else {
                            Text("Create Provider")
                        }
                    }
                    .buttonStyle(.borderedProminent)
                    .disabled(viewModel.isCreating || viewModel.newProviderName.isEmpty || !viewModel.isConfigValid())
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
            
            ForEach(viewModel.templates) { template in
                Button {
                    viewModel.selectedTemplate = template
                    // Set default values
                    for field in template.configSchema {
                        viewModel.configValues[field.key] = field.defaultString
                    }
                    // Fetch available models for LLM providers
                    if template.service == "vertex_ai_llm" {
                        Task { await viewModel.fetchModels(service: template.service) }
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
                TextField("e.g., my-embeddings", text: $viewModel.newProviderName)
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
                        get: { viewModel.authValues["api_key"] ?? "" },
                        set: { viewModel.authValues["api_key"] = $0 }
                    ))
                    .textFieldStyle(.roundedBorder)
                }
            }
            
            // OAuth info
            if template.authType == .oauth {
                VStack(alignment: .leading, spacing: 8) {
                    HStack(spacing: 8) {
                        if viewModel.isCheckingGoogleAuth {
                            ProgressView()
                                .scaleEffect(0.7)
                            Text("Checking authentication...")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        } else if let status = viewModel.googleAuthStatus {
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
                                    viewModel.showGoogleAuth = true
                                    Task { await viewModel.startGoogleAuth() }
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
                                Task { await viewModel.checkGoogleAuthStatus() }
                            }
                            .controlSize(.small)
                        }
                    }
                }
                .padding(.vertical, 4)
                .task {
                    await viewModel.checkGoogleAuthStatus()
                }
            }
            
            // Config fields
            ForEach(template.configSchema) { field in
                configFieldView(field: field, values: $viewModel.configValues)
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
                if field.key == "model" && viewModel.isLoadingModels {
                    ProgressView()
                        .scaleEffect(0.6)
                }
            }
            
            // Use dynamic models for the model field if available
            if field.key == "model" && !viewModel.availableModels.isEmpty {
                VStack(alignment: .leading, spacing: 4) {
                    Picker("", selection: Binding(
                        get: { values.wrappedValue[field.key] ?? field.defaultString },
                        set: { values.wrappedValue[field.key] = $0 }
                    )) {
                        ForEach(viewModel.availableModels) { model in
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
                       let selectedModel = viewModel.availableModels.first(where: { $0.id == selectedModelID }) {
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
            if field.key == "model", let error = viewModel.modelsError {
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
                    viewModel.editingProvider = nil
                } label: {
                    Image(systemName: "xmark.circle.fill")
                        .foregroundStyle(.secondary)
                }
                .buttonStyle(.plain)
            }
            .padding()
            
            Divider()
            
            // Content
            if let template = viewModel.templates.first(where: { $0.service == provider.service }) {
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
                                    get: { viewModel.editAuthValues["api_key"] ?? "" },
                                    set: { viewModel.editAuthValues["api_key"] = $0 }
                                ))
                                .textFieldStyle(.roundedBorder)
                                Text("Current key is masked. Enter a new key to update.")
                                    .font(.caption2)
                                    .foregroundStyle(.secondary)
                            }
                        }
                        
                        // Config fields
                        ForEach(template.configSchema) { field in
                            configFieldView(field: field, values: $viewModel.editConfigValues)
                        }
                        
                        // Error message
                        if let error = viewModel.editError {
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
                    viewModel.editingProvider = nil
                }
                
                Spacer()
                
                Button {
                    Task { await viewModel.updateProvider() }
                } label: {
                    if viewModel.isEditing {
                        ProgressView()
                            .scaleEffect(0.7)
                    } else {
                        Text("Save Changes")
                    }
                }
                .buttonStyle(.borderedProminent)
                .disabled(viewModel.isEditing)
            }
            .padding()
        }
        .frame(width: 450, height: 450)
        .onAppear {
            // Initialize edit values when sheet appears
            viewModel.prepareEdit(provider)
        }
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
                    viewModel.showGoogleAuth = false
                    viewModel.resetGoogleAuth()
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
                if let error = viewModel.googleAuthError {
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
                            viewModel.googleAuthError = nil
                            Task { await viewModel.startGoogleAuth() }
                        }
                        .buttonStyle(.borderedProminent)
                    }
                } else if let dcr = viewModel.deviceCodeResponse {
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
                        
                        if viewModel.isPollingForToken {
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
                    viewModel.showGoogleAuth = false
                    viewModel.resetGoogleAuth()
                }
                Spacer()
            }
            .padding()
        }
        .frame(width: 450, height: 350)
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
    
    private let client = DianeClient.shared
    
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
