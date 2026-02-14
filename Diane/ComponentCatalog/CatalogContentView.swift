import SwiftUI

// MARK: - CatalogItem

enum CatalogCategory: String, CaseIterable {
    case screenViews = "Screen Views"
    case reusableComponents = "Reusable Components"
}

enum CatalogItem: String, CaseIterable, Identifiable, Hashable {
    // Screen-level views
    case mcpServersView = "MCPServersView"
    case agentsView = "AgentsView"
    case providersView = "ProvidersView"
    case contextsView = "ContextsView"
    case schedulerView = "SchedulerView"
    case usageView = "UsageView"
    case toolsBrowserView = "ToolsBrowserView"
    case settingsView = "SettingsView"

    // Reusable components
    case masterDetailView = "MasterDetailView"
    case masterListHeader = "MasterListHeader"
    case detailSection = "DetailSection"
    case infoRow = "InfoRow"
    case stringArrayEditor = "StringArrayEditor"
    case keyValueEditor = "KeyValueEditor"
    case summaryCard = "SummaryCard"
    case emptyStateView = "EmptyStateView"
    case timeRangePicker = "TimeRangePicker"
    case oauthConfigEditor = "OAuthConfigEditor"

    var id: String { rawValue }

    var category: CatalogCategory {
        switch self {
        case .mcpServersView, .agentsView, .providersView, .contextsView,
             .schedulerView, .usageView, .toolsBrowserView, .settingsView:
            return .screenViews
        case .masterDetailView, .masterListHeader, .detailSection, .infoRow,
             .stringArrayEditor, .keyValueEditor, .summaryCard, .emptyStateView,
             .timeRangePicker, .oauthConfigEditor:
            return .reusableComponents
        }
    }

    var displayName: String { rawValue }

    var iconName: String {
        switch self {
        case .mcpServersView: return "server.rack"
        case .agentsView: return "person.3"
        case .providersView: return "building.2"
        case .contextsView: return "square.stack.3d.up"
        case .schedulerView: return "clock"
        case .usageView: return "chart.bar"
        case .toolsBrowserView: return "wrench.and.screwdriver"
        case .settingsView: return "gear"
        case .masterDetailView: return "rectangle.split.2x1"
        case .masterListHeader: return "text.justify.left"
        case .detailSection: return "rectangle.and.text.magnifyingglass"
        case .infoRow: return "info.circle"
        case .stringArrayEditor: return "list.bullet"
        case .keyValueEditor: return "list.dash.header.rectangle"
        case .summaryCard: return "rectangle.fill"
        case .emptyStateView: return "square.dashed"
        case .timeRangePicker: return "clock.badge"
        case .oauthConfigEditor: return "lock.shield"
        }
    }

    /// Available presets for this item (screen views have state presets, components don't).
    var presets: [CatalogPreset] {
        switch self {
        case .mcpServersView, .agentsView, .providersView, .contextsView,
             .schedulerView, .usageView, .toolsBrowserView:
            return StandardPresets.all
        case .settingsView:
            return [] // SettingsView has no ViewModel presets
        case .masterDetailView, .masterListHeader, .detailSection, .infoRow,
             .stringArrayEditor, .keyValueEditor, .summaryCard, .emptyStateView,
             .timeRangePicker, .oauthConfigEditor:
            return [] // Components use fixed sample data
        }
    }

    static func items(for category: CatalogCategory) -> [CatalogItem] {
        CatalogItem.allCases.filter { $0.category == category }
    }
}

// MARK: - CatalogContentView

struct CatalogContentView: View {
    @State private var selectedItem: CatalogItem? = .mcpServersView
    @State private var theme = CatalogTheme()
    @State private var selectedPresets: [CatalogItem: CatalogPreset] = [:]

    /// The mock status monitor injected into views that require `@EnvironmentObject`.
    @State private var mockStatusMonitor = MockStatusMonitor()

    // Feedback state
    @State private var feedbackText: [CatalogItem: String] = [:]
    @State private var isCreatingIssue: Bool = false
    @State private var issueResult: IssueCreationResult?
    @State private var columnVisibility: NavigationSplitViewVisibility = .all

    var body: some View {
        NavigationSplitView(columnVisibility: $columnVisibility) {
            sidebarContent
                .navigationSplitViewColumnWidth(min: 200, ideal: 220, max: 300)
        } content: {
            previewCanvas
                .navigationSplitViewColumnWidth(min: 500, ideal: 700)
        } detail: {
            controlsPanel
                .navigationSplitViewColumnWidth(min: 250, ideal: 280, max: 350)
        }
    }

    // MARK: - Current Preset

    private func currentPreset(for item: CatalogItem) -> CatalogPreset {
        selectedPresets[item] ?? item.presets.first ?? StandardPresets.loaded
    }

    // MARK: - Sidebar

    @ViewBuilder
    private var sidebarContent: some View {
        List(selection: $selectedItem) {
            ForEach(CatalogCategory.allCases, id: \.self) { category in
                Section(category.rawValue) {
                    ForEach(CatalogItem.items(for: category)) { item in
                        Label(item.displayName, systemImage: item.iconName)
                            .tag(item)
                    }
                }
            }
        }
        .listStyle(.sidebar)
        .navigationTitle("Component Catalog")
    }

    // MARK: - Preview Canvas

    @ViewBuilder
    private var previewCanvas: some View {
        if let item = selectedItem {
            VStack(spacing: 0) {
                // Title bar
                HStack {
                    Text(item.displayName)
                        .font(.headline)
                    Spacer()
                    if !item.presets.isEmpty {
                        Text(currentPreset(for: item).name)
                            .font(.caption)
                            .padding(.horizontal, 6)
                            .padding(.vertical, 2)
                            .background(.quaternary)
                            .clipShape(Capsule())
                    }
                    Text(item.category.rawValue)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                .padding(.horizontal)
                .padding(.vertical, 8)
                .background(.bar)

                Divider()

                // Preview area
                ScrollView([.horizontal, .vertical]) {
                    previewContent(for: item)
                        .frame(
                            width: theme.canvasWidth,
                            height: item.category == .reusableComponents ? nil : theme.canvasHeight
                        )
                        .padding(theme.padding)
                        .background(theme.backgroundColor)
                        .tint(theme.accentColor)
                        .clipShape(RoundedRectangle(cornerRadius: 4))
                        .overlay(
                            RoundedRectangle(cornerRadius: 4)
                                .strokeBorder(.separator, lineWidth: 1)
                        )
                        .padding()
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
                .background(Color(nsColor: .windowBackgroundColor))
            }
        } else {
            ContentUnavailableView(
                "Select a Component",
                systemImage: "sidebar.left",
                description: Text("Choose a view or component from the sidebar to preview it.")
            )
        }
    }

    // MARK: - Preview Content Factory

    @ViewBuilder
    private func previewContent(for item: CatalogItem) -> some View {
        switch item {
        // Screen-level views
        case .mcpServersView:
            MCPServersView(viewModel: MCPServersPresets.viewModel(for: currentPreset(for: item)))
        case .agentsView:
            AgentsView(viewModel: AgentsPresets.viewModel(for: currentPreset(for: item)))
        case .providersView:
            ProvidersView(viewModel: ProvidersPresets.viewModel(for: currentPreset(for: item)))
        case .contextsView:
            ContextsView(viewModel: ContextsPresets.viewModel(for: currentPreset(for: item)))
        case .schedulerView:
            SchedulerView(viewModel: SchedulerPresets.viewModel(for: currentPreset(for: item)))
        case .usageView:
            UsageView(viewModel: UsagePresets.viewModel(for: currentPreset(for: item)))
        case .toolsBrowserView:
            ToolsBrowserView(viewModel: ToolsBrowserPresets.viewModel(for: currentPreset(for: item)))
                .environmentObject(mockStatusMonitor as StatusMonitor)
        case .settingsView:
            SettingsView()
                .environmentObject(mockStatusMonitor as StatusMonitor)

        // Reusable components
        case .detailSection:
            ComponentPresets.detailSection()
        case .infoRow:
            ComponentPresets.infoRow()
        case .summaryCard:
            ComponentPresets.summaryCard()
        case .stringArrayEditor:
            ComponentPresets.stringArrayEditor()
        case .keyValueEditor:
            ComponentPresets.keyValueEditor()
        case .oauthConfigEditor:
            ComponentPresets.oauthConfigEditor()
        case .emptyStateView:
            ComponentPresets.emptyStateView()
        case .timeRangePicker:
            ComponentPresets.timeRangePicker()
        case .masterDetailView:
            ComponentPresets.masterDetailView()
        case .masterListHeader:
            ComponentPresets.masterListHeader()
        }
    }

    // MARK: - Controls Panel

    @ViewBuilder
    private var controlsPanel: some View {
        if let item = selectedItem {
            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
                    // State Presets section
                    if !item.presets.isEmpty {
                        presetSelector(for: item)
                    }

                    // Layout Controls section
                    layoutControls

                    // Canvas Size section
                    canvasSizeControls(for: item)

                    // Feedback section
                    feedbackSection(for: item)
                }
                .padding()
            }
        } else {
            ContentUnavailableView(
                "No Selection",
                systemImage: "slider.horizontal.3",
                description: Text("Select a component to see its controls.")
            )
        }
    }

    // MARK: - Preset Selector

    @ViewBuilder
    private func presetSelector(for item: CatalogItem) -> some View {
        GroupBox("State Presets") {
            VStack(alignment: .leading, spacing: 8) {
                ForEach(item.presets) { preset in
                    let isSelected = currentPreset(for: item).id == preset.id
                    Button {
                        selectedPresets[item] = preset
                    } label: {
                        HStack(spacing: 8) {
                            Image(systemName: preset.icon)
                                .frame(width: 16)
                                .foregroundStyle(isSelected ? .white : .secondary)
                            VStack(alignment: .leading, spacing: 1) {
                                Text(preset.name)
                                    .font(.caption.weight(.medium))
                                Text(preset.description)
                                    .font(.caption2)
                                    .foregroundStyle(isSelected ? .white.opacity(0.8) : .secondary)
                            }
                            Spacer()
                            if isSelected {
                                Image(systemName: "checkmark")
                                    .font(.caption.weight(.bold))
                                    .foregroundStyle(.white)
                            }
                        }
                        .padding(.horizontal, 8)
                        .padding(.vertical, 6)
                        .background(isSelected ? Color.accentColor : Color.clear)
                        .clipShape(RoundedRectangle(cornerRadius: 6))
                        .contentShape(Rectangle())
                    }
                    .buttonStyle(.plain)
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
        }
    }

    // MARK: - Layout Controls

    @ViewBuilder
    private var layoutControls: some View {
        GroupBox("Layout Controls") {
            VStack(alignment: .leading, spacing: 12) {
                controlRow("Spacing", value: theme.spacing, range: 0...32) {
                    theme.spacing = $0
                }
                controlRow("Padding", value: theme.padding, range: 0...48) {
                    theme.padding = $0
                }
                controlRow("Font Size", value: theme.fontSize, range: 8...32) {
                    theme.fontSize = $0
                }
                controlRow("Corner Radius", value: theme.cornerRadius, range: 0...24) {
                    theme.cornerRadius = $0
                }

                Divider()

                // Accent Color
                HStack {
                    Text("Accent")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .frame(width: 80, alignment: .leading)
                    ColorPicker("", selection: Binding(
                        get: { theme.accentColor },
                        set: { theme.accentColor = $0 }
                    ))
                    .labelsHidden()
                }

                // Background Color
                HStack {
                    Text("Background")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .frame(width: 80, alignment: .leading)
                    ColorPicker("", selection: Binding(
                        get: { theme.backgroundColor },
                        set: { theme.backgroundColor = $0 }
                    ))
                    .labelsHidden()
                    Spacer()
                    Button("Clear") {
                        theme.backgroundColor = .clear
                    }
                    .font(.caption)
                    .buttonStyle(.borderless)
                }

                Divider()

                // Reset button
                HStack {
                    Spacer()
                    Button("Reset All") {
                        theme.reset()
                    }
                    .font(.caption)
                    .buttonStyle(.bordered)
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
        }
    }

    @ViewBuilder
    private func controlRow(
        _ label: String,
        value: CGFloat,
        range: ClosedRange<CGFloat>,
        onChange: @escaping (CGFloat) -> Void
    ) -> some View {
        VStack(alignment: .leading, spacing: 2) {
            HStack {
                Text(label)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                Spacer()
                Text("\(Int(value))")
                    .font(.caption.monospacedDigit())
                    .foregroundStyle(.secondary)
            }
            Slider(
                value: Binding(
                    get: { value },
                    set: { onChange($0) }
                ),
                in: range,
                step: 1
            )
            .controlSize(.small)
        }
    }

    // MARK: - Canvas Size Controls

    @ViewBuilder
    private func canvasSizeControls(for item: CatalogItem) -> some View {
        let isScreenView = item.category == .screenViews
        GroupBox(isScreenView ? "Canvas Size" : "Component Width") {
            VStack(alignment: .leading, spacing: 12) {
                // Width/Height fields
                HStack(spacing: 12) {
                    VStack(alignment: .leading, spacing: 2) {
                        Text("Width")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                        TextField("Width", value: Binding<Double>(
                            get: { Double(theme.canvasWidth) },
                            set: { theme.canvasWidth = CGFloat($0) }
                        ), format: .number)
                        .textFieldStyle(.roundedBorder)
                        .frame(width: 80)
                    }
                    if isScreenView {
                        Text("\u{00D7}")
                            .foregroundStyle(.secondary)
                            .padding(.top, 14)
                        VStack(alignment: .leading, spacing: 2) {
                            Text("Height")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                            TextField("Height", value: Binding<Double>(
                                get: { Double(theme.canvasHeight) },
                                set: { theme.canvasHeight = CGFloat($0) }
                            ), format: .number)
                            .textFieldStyle(.roundedBorder)
                            .frame(width: 80)
                        }
                    }
                }

                // Size presets
                HStack(spacing: 6) {
                    ForEach(CatalogTheme.sizePresets) { preset in
                        Button(preset.name) {
                            theme.applyPreset(preset)
                        }
                        .font(.caption)
                        .buttonStyle(.bordered)
                        .controlSize(.small)
                    }
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
        }
    }

    // MARK: - Feedback Section

    @ViewBuilder
    private func feedbackSection(for item: CatalogItem) -> some View {
        let text = Binding<String>(
            get: { feedbackText[item] ?? "" },
            set: { feedbackText[item] = $0 }
        )
        let canSubmit = !(text.wrappedValue.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty) && !isCreatingIssue

        GroupBox("Feedback") {
            VStack(alignment: .leading, spacing: 10) {
                // Selector preview
                let preset = item.presets.isEmpty ? nil : currentPreset(for: item)
                let sel = GitHubIssueService.selector(for: item, preset: preset)
                HStack(spacing: 4) {
                    Image(systemName: "tag")
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                    Text(sel)
                        .font(.caption.monospaced())
                        .foregroundStyle(.secondary)
                }

                // Comment editor
                TextEditor(text: text)
                    .font(.body)
                    .frame(minHeight: 80, maxHeight: 150)
                    .scrollContentBackground(.hidden)
                    .padding(4)
                    .background(Color(nsColor: .textBackgroundColor))
                    .clipShape(RoundedRectangle(cornerRadius: 6))
                    .overlay(
                        RoundedRectangle(cornerRadius: 6)
                            .strokeBorder(.separator, lineWidth: 1)
                    )

                // Result feedback
                if let result = issueResult {
                    resultView(result)
                }

                // Submit button
                HStack {
                    Spacer()
                    Button {
                        submitFeedback(for: item)
                    } label: {
                        if isCreatingIssue {
                            HStack(spacing: 6) {
                                ProgressView()
                                    .controlSize(.small)
                                Text("Creating...")
                                    .font(.caption)
                            }
                        } else {
                            Label("Create Issue", systemImage: "paperplane")
                                .font(.caption)
                        }
                    }
                    .buttonStyle(.borderedProminent)
                    .controlSize(.small)
                    .disabled(!canSubmit)
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
        }
    }

    // MARK: - Result View

    @ViewBuilder
    private func resultView(_ result: IssueCreationResult) -> some View {
        switch result {
        case .success(let url):
            HStack(spacing: 4) {
                Image(systemName: "checkmark.circle.fill")
                    .foregroundStyle(.green)
                if let link = URL(string: url) {
                    Link("Issue created", destination: link)
                        .font(.caption)
                } else {
                    Text("Issue created: \(url)")
                        .font(.caption)
                }
            }
            .transition(.opacity)
        case .error(let message):
            HStack(alignment: .top, spacing: 4) {
                Image(systemName: "exclamationmark.triangle.fill")
                    .foregroundStyle(.red)
                Text(message)
                    .font(.caption)
                    .foregroundStyle(.red)
                    .lineLimit(3)
            }
            .transition(.opacity)
        }
    }

    // MARK: - Submit Feedback

    private func submitFeedback(for item: CatalogItem) {
        guard let comment = feedbackText[item],
              !comment.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { return }

        let preset = item.presets.isEmpty ? nil : currentPreset(for: item)

        isCreatingIssue = true
        issueResult = nil

        Task {
            let result = await GitHubIssueService.createIssue(
                item: item,
                preset: preset,
                comment: comment,
                repo: "diane-assistant/diane"
            )

            isCreatingIssue = false
            issueResult = result

            if case .success = result {
                feedbackText[item] = ""
                // Auto-dismiss success after 5 seconds
                try? await Task.sleep(for: .seconds(5))
                if case .success = issueResult {
                    withAnimation { issueResult = nil }
                }
            }
        }
    }
}
