import SwiftUI

/// Root content view for the iOS app.
/// Uses adaptive navigation: TabView on iPhone (compact), NavigationSplitView on iPad (regular).
struct IOSContentView: View {
    @Environment(\.horizontalSizeClass) private var sizeClass
    @Environment(IOSStatusMonitor.self) private var monitor

    @State private var selectedSection: AppSection? = .status
    @SceneStorage("selectedTab") private var selectedTab: String = AppSection.status.rawValue

    var body: some View {
        if sizeClass == .compact {
            compactLayout
        } else {
            regularLayout
        }
    }

    // MARK: - iPhone: Tab View

    private var compactLayout: some View {
        TabView(selection: Binding(
            get: { AppSection(rawValue: selectedTab) ?? .status },
            set: { selectedTab = $0.rawValue }
        )) {
            NavigationStack {
                StatusDashboardView()
            }
            .tabItem {
                Label("Status", systemImage: AppSection.status.icon)
            }
            .tag(AppSection.status)

            NavigationStack {
                MCPServersListView()
            }
            .tabItem {
                Label("Servers", systemImage: AppSection.mcpServers.icon)
            }
            .tag(AppSection.mcpServers)

            NavigationStack {
                AgentsListView()
            }
            .tabItem {
                Label("Agents", systemImage: AppSection.agents.icon)
            }
            .tag(AppSection.agents)

            NavigationStack {
                MoreView()
            }
            .tabItem {
                Label("More", systemImage: "ellipsis")
            }
            .tag(AppSection.more)
        }
    }

    // MARK: - iPad: Split View

    private var regularLayout: some View {
        NavigationSplitView {
            List(AppSection.allSections, selection: $selectedSection) { section in
                Label(section.title, systemImage: section.icon)
                    .tag(section)
            }
            .navigationTitle("Diane")
        } detail: {
            NavigationStack {
                if let selectedSection {
                    detailView(for: selectedSection)
                } else {
                    StatusDashboardView()
                }
            }
        }
    }

    @ViewBuilder
    private func detailView(for section: AppSection) -> some View {
        switch section {
        case .status:
            StatusDashboardView()
        case .mcpServers:
            MCPServersListView()
        case .agents:
            AgentsListView()
        case .contexts:
            ContextsListView()
        case .providers:
            ProvidersListView()
        case .jobs:
            JobsListView()
        case .usage:
            UsageView()
        case .settings:
            IOSSettingsView()
        case .more:
            MoreView()
        }
    }
}

// MARK: - App Section

enum AppSection: String, Identifiable, CaseIterable {
    case status
    case mcpServers
    case agents
    case contexts
    case providers
    case jobs
    case usage
    case settings
    case more

    var id: String { rawValue }

    var title: String {
        switch self {
        case .status: "Status"
        case .mcpServers: "MCP Servers"
        case .agents: "Agents"
        case .contexts: "Contexts"
        case .providers: "Providers"
        case .jobs: "Jobs"
        case .usage: "Usage"
        case .settings: "Settings"
        case .more: "More"
        }
    }

    var icon: String {
        switch self {
        case .status: "house"
        case .mcpServers: "server.rack"
        case .agents: "cpu"
        case .contexts: "rectangle.connected.to.line.below"
        case .providers: "cloud"
        case .jobs: "clock"
        case .usage: "chart.bar"
        case .settings: "gear"
        case .more: "ellipsis"
        }
    }

    /// Sections shown in iPad sidebar (excludes .more)
    static let allSections: [AppSection] = [
        .status, .mcpServers, .agents, .contexts, .providers, .jobs, .usage, .settings
    ]
}

// MARK: - More View (iPhone overflow tab)

struct MoreView: View {
    var body: some View {
        List {
            NavigationLink {
                ProvidersListView()
            } label: {
                Label("Providers", systemImage: AppSection.providers.icon)
            }

            NavigationLink {
                ContextsListView()
            } label: {
                Label("Contexts", systemImage: AppSection.contexts.icon)
            }

            NavigationLink {
                JobsListView()
            } label: {
                Label("Jobs", systemImage: AppSection.jobs.icon)
            }

            NavigationLink {
                UsageView()
            } label: {
                Label("Usage", systemImage: AppSection.usage.icon)
            }

            NavigationLink {
                IOSSettingsView()
            } label: {
                Label("Settings", systemImage: AppSection.settings.icon)
            }
        }
        .navigationTitle("More")
    }
}
