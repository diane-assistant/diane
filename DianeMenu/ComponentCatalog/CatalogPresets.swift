import SwiftUI

// MARK: - Preset Protocol

/// Describes a named state preset for a catalog item.
struct CatalogPreset: Identifiable, Hashable {
    let id: String
    let name: String
    let description: String
    let icon: String

    static func == (lhs: CatalogPreset, rhs: CatalogPreset) -> Bool {
        lhs.id == rhs.id
    }
    func hash(into hasher: inout Hasher) {
        hasher.combine(id)
    }
}

// MARK: - Standard Preset Definitions

/// Common preset descriptors reused across view-level presets.
enum StandardPresets {
    static let empty = CatalogPreset(
        id: "empty", name: "Empty", description: "No data loaded, not loading", icon: "tray"
    )
    static let loading = CatalogPreset(
        id: "loading", name: "Loading", description: "Data is loading", icon: "arrow.triangle.2.circlepath"
    )
    static let error = CatalogPreset(
        id: "error", name: "Error", description: "An error occurred", icon: "exclamationmark.triangle"
    )
    static let loaded = CatalogPreset(
        id: "loaded", name: "Loaded", description: "Populated with sample data", icon: "checkmark.circle"
    )

    static let all = [empty, loading, error, loaded]
}

// MARK: - MCPServers Presets

@MainActor
enum MCPServersPresets {

    static var presets: [CatalogPreset] { StandardPresets.all }

    static func viewModel(for preset: CatalogPreset) -> MCPServersViewModel {
        switch preset.id {
        case "empty":   return makeEmpty()
        case "loading": return makeLoading()
        case "error":   return makeError()
        case "loaded":  return makeLoaded()
        default:        return makeLoaded()
        }
    }

    static func makeEmpty() -> MCPServersViewModel {
        let client = MockDianeClient()
        let vm = MCPServersViewModel(client: client)
        vm.isLoading = false
        vm.servers = []
        return vm
    }

    static func makeLoading() -> MCPServersViewModel {
        let client = MockDianeClient()
        let vm = MCPServersViewModel(client: client)
        vm.isLoading = true
        return vm
    }

    static func makeError() -> MCPServersViewModel {
        let client = MockDianeClient()
        let vm = MCPServersViewModel(client: client)
        vm.isLoading = false
        vm.error = "Failed to load MCP servers: Connection refused"
        return vm
    }

    static func makeLoaded() -> MCPServersViewModel {
        let client = MockDianeClient()
        client.serverConfigs = TestFixtures.makeMixedServerList()
        let vm = MCPServersViewModel(client: client)
        vm.isLoading = false
        vm.servers = client.serverConfigs
        vm.selectedServer = client.serverConfigs.first
        return vm
    }
}

// MARK: - Agents Presets

@MainActor
enum AgentsPresets {

    static var presets: [CatalogPreset] { StandardPresets.all }

    static func viewModel(for preset: CatalogPreset) -> AgentsViewModel {
        switch preset.id {
        case "empty":   return makeEmpty()
        case "loading": return makeLoading()
        case "error":   return makeError()
        case "loaded":  return makeLoaded()
        default:        return makeLoaded()
        }
    }

    static func makeEmpty() -> AgentsViewModel {
        let client = MockDianeClient()
        let vm = AgentsViewModel(client: client)
        vm.isLoading = false
        vm.agents = []
        return vm
    }

    static func makeLoading() -> AgentsViewModel {
        let client = MockDianeClient()
        let vm = AgentsViewModel(client: client)
        vm.isLoading = true
        return vm
    }

    static func makeError() -> AgentsViewModel {
        let client = MockDianeClient()
        let vm = AgentsViewModel(client: client)
        vm.isLoading = false
        vm.error = "Failed to load agents: Connection refused"
        return vm
    }

    static func makeLoaded() -> AgentsViewModel {
        let client = MockDianeClient()
        let agents = TestFixtures.makeAgentList()
        client.agentsList = agents
        client.agentLogs = TestFixtures.makeAgentLogList(agentName: agents.first?.name ?? "test-agent")
        client.galleryEntriesList = TestFixtures.makeGalleryEntryList()

        let vm = AgentsViewModel(client: client)
        vm.isLoading = false
        vm.agents = agents
        vm.selectedAgent = agents.first
        vm.logs = client.agentLogs
        vm.galleryEntries = client.galleryEntriesList
        return vm
    }
}

// MARK: - Providers Presets

@MainActor
enum ProvidersPresets {

    static var presets: [CatalogPreset] { StandardPresets.all }

    static func viewModel(for preset: CatalogPreset) -> ProvidersViewModel {
        switch preset.id {
        case "empty":   return makeEmpty()
        case "loading": return makeLoading()
        case "error":   return makeError()
        case "loaded":  return makeLoaded()
        default:        return makeLoaded()
        }
    }

    static func makeEmpty() -> ProvidersViewModel {
        let client = MockDianeClient()
        let vm = ProvidersViewModel(client: client)
        vm.isLoading = false
        vm.providers = []
        return vm
    }

    static func makeLoading() -> ProvidersViewModel {
        let client = MockDianeClient()
        let vm = ProvidersViewModel(client: client)
        vm.isLoading = true
        return vm
    }

    static func makeError() -> ProvidersViewModel {
        let client = MockDianeClient()
        let vm = ProvidersViewModel(client: client)
        vm.isLoading = false
        vm.error = "Failed to load providers: Connection refused"
        return vm
    }

    static func makeLoaded() -> ProvidersViewModel {
        let client = MockDianeClient()
        let providers = TestFixtures.makeProviderList()
        client.providersList = providers
        client.providerTemplates = TestFixtures.makeProviderTemplateList()

        let vm = ProvidersViewModel(client: client)
        vm.isLoading = false
        vm.providers = providers
        vm.templates = client.providerTemplates
        vm.selectedProvider = providers.first
        return vm
    }
}

// MARK: - Contexts Presets

@MainActor
enum ContextsPresets {

    static var presets: [CatalogPreset] { StandardPresets.all }

    static func viewModel(for preset: CatalogPreset) -> ContextsViewModel {
        switch preset.id {
        case "empty":   return makeEmpty()
        case "loading": return makeLoading()
        case "error":   return makeError()
        case "loaded":  return makeLoaded()
        default:        return makeLoaded()
        }
    }

    static func makeEmpty() -> ContextsViewModel {
        let client = MockDianeClient()
        let vm = ContextsViewModel(client: client)
        vm.isLoading = false
        vm.contexts = []
        return vm
    }

    static func makeLoading() -> ContextsViewModel {
        let client = MockDianeClient()
        let vm = ContextsViewModel(client: client)
        vm.isLoading = true
        return vm
    }

    static func makeError() -> ContextsViewModel {
        let client = MockDianeClient()
        let vm = ContextsViewModel(client: client)
        vm.isLoading = false
        vm.error = "Failed to load contexts: Connection refused"
        return vm
    }

    static func makeLoaded() -> ContextsViewModel {
        let client = MockDianeClient()
        let contexts = TestFixtures.makeContextList()
        client.contextsList = contexts
        client.contextDetails["default"] = TestFixtures.makeContextDetail()

        let vm = ContextsViewModel(client: client)
        vm.isLoading = false
        vm.contexts = contexts
        vm.selectedContext = contexts.first
        vm.contextDetail = client.contextDetails["default"]
        return vm
    }
}

// MARK: - Scheduler Presets

@MainActor
enum SchedulerPresets {

    static var presets: [CatalogPreset] { StandardPresets.all }

    static func viewModel(for preset: CatalogPreset) -> SchedulerViewModel {
        switch preset.id {
        case "empty":   return makeEmpty()
        case "loading": return makeLoading()
        case "error":   return makeError()
        case "loaded":  return makeLoaded()
        default:        return makeLoaded()
        }
    }

    static func makeEmpty() -> SchedulerViewModel {
        let client = MockDianeClient()
        let vm = SchedulerViewModel(client: client)
        vm.isLoading = false
        vm.jobs = []
        vm.executions = []
        return vm
    }

    static func makeLoading() -> SchedulerViewModel {
        let client = MockDianeClient()
        let vm = SchedulerViewModel(client: client)
        vm.isLoading = true
        return vm
    }

    static func makeError() -> SchedulerViewModel {
        let client = MockDianeClient()
        let vm = SchedulerViewModel(client: client)
        vm.isLoading = false
        vm.error = "Failed to load jobs: Connection refused"
        return vm
    }

    static func makeLoaded() -> SchedulerViewModel {
        let client = MockDianeClient()
        let jobs = TestFixtures.makeJobList()
        let executions = TestFixtures.makeExecutionList()
        client.jobs = jobs
        client.jobExecutions = executions

        let vm = SchedulerViewModel(client: client)
        vm.isLoading = false
        vm.jobs = jobs
        vm.executions = executions
        vm.selectedJob = jobs.first
        return vm
    }
}

// MARK: - Usage Presets

@MainActor
enum UsagePresets {

    static var presets: [CatalogPreset] { StandardPresets.all }

    /// UsageViewModel has `private let client` — we configure the mock before init,
    /// then set observable state directly on the ViewModel.
    static func viewModel(for preset: CatalogPreset) -> UsageViewModel {
        switch preset.id {
        case "empty":   return makeEmpty()
        case "loading": return makeLoading()
        case "error":   return makeError()
        case "loaded":  return makeLoaded()
        default:        return makeLoaded()
        }
    }

    static func makeEmpty() -> UsageViewModel {
        let client = MockDianeClient()
        let vm = UsageViewModel(client: client)
        vm.isLoading = false
        vm.usageSummary = nil
        vm.recentUsage = nil
        return vm
    }

    static func makeLoading() -> UsageViewModel {
        let client = MockDianeClient()
        let vm = UsageViewModel(client: client)
        vm.isLoading = true
        return vm
    }

    static func makeError() -> UsageViewModel {
        let client = MockDianeClient()
        let vm = UsageViewModel(client: client)
        vm.isLoading = false
        vm.error = "Failed to load usage data: Connection refused"
        return vm
    }

    static func makeLoaded() -> UsageViewModel {
        let client = MockDianeClient()
        client.usageSummaryResponse = TestFixtures.makeUsageSummaryResponse()
        client.usageResponse = TestFixtures.makeUsageResponse()

        let vm = UsageViewModel(client: client)
        vm.isLoading = false
        vm.usageSummary = client.usageSummaryResponse
        vm.recentUsage = client.usageResponse
        return vm
    }
}

// MARK: - ToolsBrowser Presets

@MainActor
enum ToolsBrowserPresets {

    static var presets: [CatalogPreset] { StandardPresets.all }

    /// ToolsBrowserViewModel has `private let client` — we configure the mock before init,
    /// then set observable state directly on the ViewModel.
    static func viewModel(for preset: CatalogPreset) -> ToolsBrowserViewModel {
        switch preset.id {
        case "empty":   return makeEmpty()
        case "loading": return makeLoading()
        case "error":   return makeError()
        case "loaded":  return makeLoaded()
        default:        return makeLoaded()
        }
    }

    static func makeEmpty() -> ToolsBrowserViewModel {
        let client = MockDianeClient()
        let vm = ToolsBrowserViewModel(client: client)
        vm.isLoading = false
        vm.tools = []
        return vm
    }

    static func makeLoading() -> ToolsBrowserViewModel {
        let client = MockDianeClient()
        let vm = ToolsBrowserViewModel(client: client)
        vm.isLoading = true
        return vm
    }

    static func makeError() -> ToolsBrowserViewModel {
        let client = MockDianeClient()
        let vm = ToolsBrowserViewModel(client: client)
        vm.isLoading = false
        vm.error = "Failed to load tools: Connection refused"
        return vm
    }

    static func makeLoaded() -> ToolsBrowserViewModel {
        let client = MockDianeClient()
        let tools = TestFixtures.makeToolList()
        client.toolsList = tools

        let vm = ToolsBrowserViewModel(client: client)
        vm.isLoading = false
        vm.tools = tools
        return vm
    }
}

// MARK: - Reusable Component Presets

/// Provides sample views for each reusable component in the catalog.
@MainActor
enum ComponentPresets {

    // MARK: DetailSection

    @ViewBuilder
    static func detailSection() -> some View {
        VStack(spacing: 16) {
            DetailSection(title: "Server Configuration") {
                InfoRow(label: "Type", value: "stdio")
                InfoRow(label: "Command", value: "/usr/local/bin/mcp-server")
                InfoRow(label: "Status", value: "Running")
            }
            DetailSection(title: "Empty Section") {
                Text("No items to display")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
    }

    // MARK: InfoRow

    @ViewBuilder
    static func infoRow() -> some View {
        VStack(alignment: .leading, spacing: 8) {
            InfoRow(label: "Name", value: "node-mcp")
            InfoRow(label: "Type", value: "stdio")
            InfoRow(label: "Command", value: "/usr/local/bin/mcp-server --verbose")
            InfoRow(label: "Status", value: "Enabled")
            InfoRow(label: "Created", value: "2025-01-15 12:00:00")
        }
        .padding()
        .background(Color(nsColor: .controlBackgroundColor))
        .cornerRadius(8)
    }

    // MARK: SummaryCard

    @ViewBuilder
    static func summaryCard() -> some View {
        HStack(spacing: 12) {
            SummaryCard(title: "Total Cost", value: "$0.90", icon: "dollarsign.circle", color: .green)
            SummaryCard(title: "Requests", value: "40", icon: "arrow.up.arrow.down", color: .blue)
            SummaryCard(title: "Tokens", value: "27.5K", icon: "text.word.spacing", color: .purple)
        }
        .padding()
    }

    // MARK: StringArrayEditor

    @ViewBuilder
    static func stringArrayEditor() -> some View {
        StringArrayEditorPreview()
    }

    // MARK: KeyValueEditor

    @ViewBuilder
    static func keyValueEditor() -> some View {
        KeyValueEditorPreview()
    }

    // MARK: OAuthConfigEditor

    @ViewBuilder
    static func oauthConfigEditor() -> some View {
        OAuthConfigEditorPreview()
    }

    // MARK: MasterDetailView

    @ViewBuilder
    static func masterDetailView() -> some View {
        MasterDetailView {
            VStack(spacing: 0) {
                MasterListHeader(icon: "server.rack", title: "Servers", count: 3)
                List {
                    Text("node-mcp").tag("node-mcp")
                    Text("cloud-mcp").tag("cloud-mcp")
                    Text("api-mcp").tag("api-mcp")
                }
                .listStyle(.plain)
            }
        } detail: {
            VStack(alignment: .leading, spacing: 16) {
                Text("node-mcp")
                    .font(.title2.weight(.semibold))
                InfoRow(label: "Type", value: "stdio")
                InfoRow(label: "Command", value: "/usr/local/bin/mcp-server")
                Spacer()
            }
            .padding()
        }
    }

    // MARK: MasterListHeader

    @ViewBuilder
    static func masterListHeader() -> some View {
        VStack(spacing: 8) {
            MasterListHeader(icon: "server.rack", title: "Servers", count: 5)
            MasterListHeader(icon: "person.3", title: "Agents", count: 4)
            MasterListHeader(icon: "building.2", title: "Providers", count: 3)
            MasterListHeader(icon: "square.stack.3d.up", title: "Contexts", count: 2)
        }
    }

    // MARK: EmptyStateView

    @ViewBuilder
    static func emptyStateView() -> some View {
        VStack(spacing: 32) {
            // With action button
            EmptyStateView(
                icon: "server.rack",
                title: "No MCP servers configured",
                description: "Add an MCP server to extend Diane's capabilities",
                actionLabel: "Add Server",
                action: {}
            )
            .frame(height: 200)

            Divider()

            // Informational (no button)
            EmptyStateView(
                icon: "calendar.badge.clock",
                title: "No scheduled jobs",
                description: "Jobs can be created using the job_add tool"
            )
            .frame(height: 160)

            Divider()

            // With custom content (code example)
            EmptyStateView(
                icon: "person.3",
                title: "No agents configured",
                description: "Use the gallery to add agents:"
            ) {
                Text("./scripts/acp-gallery.sh install gemini")
                    .font(.system(.caption, design: .monospaced))
                    .padding(8)
                    .background(Color(nsColor: .textBackgroundColor))
                    .cornerRadius(6)
            }
            .frame(height: 200)
        }
    }

    // MARK: TimeRangePicker

    @ViewBuilder
    static func timeRangePicker() -> some View {
        TimeRangePickerPreview()
    }
}

// MARK: - Stateful Wrapper Views for Components with @Binding

/// Wrapper to provide @State for StringArrayEditor's @Binding parameter.
private struct StringArrayEditorPreview: View {
    @State private var items = ["--verbose", "--port=8080", "--log-level=debug"]

    var body: some View {
        StringArrayEditor(items: $items, title: "Arguments", placeholder: "Add argument")
            .padding()
            .background(Color(nsColor: .controlBackgroundColor))
            .cornerRadius(8)
    }
}

/// Wrapper to provide @State for KeyValueEditor's @Binding parameter.
private struct KeyValueEditorPreview: View {
    @State private var items = [
        "API_KEY": "sk-test-123",
        "NODE_ENV": "production",
        "LOG_LEVEL": "debug",
    ]

    var body: some View {
        KeyValueEditor(items: $items, title: "Environment Variables", keyPlaceholder: "Key", valuePlaceholder: "Value")
            .padding()
            .background(Color(nsColor: .controlBackgroundColor))
            .cornerRadius(8)
    }
}

/// Wrapper to provide @State for OAuthConfigEditor's @Binding parameter.
private struct OAuthConfigEditorPreview: View {
    @State private var config: OAuthConfig? = OAuthConfig(
        provider: "github",
        clientID: "abc123",
        clientSecret: nil,
        scopes: ["user:email", "repo"],
        deviceAuthURL: nil,
        tokenURL: nil
    )

    var body: some View {
        OAuthConfigEditor(config: $config)
            .padding()
            .background(Color(nsColor: .controlBackgroundColor))
            .cornerRadius(8)
    }
}

/// Demo enum and wrapper for TimeRangePicker preview.
private struct TimeRangePickerPreview: View {
    enum DemoRange: String, CaseIterable {
        case day = "24 Hours"
        case week = "7 Days"
        case month = "30 Days"
        case year = "1 Year"
    }

    @State private var selection: DemoRange = .month

    var body: some View {
        VStack(spacing: 16) {
            TimeRangePicker(selection: $selection)
                .frame(width: 300)

            Text("Selected: \(selection.rawValue)")
                .font(.caption)
                .foregroundStyle(.secondary)
        }
        .padding()
    }
}
