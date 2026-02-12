## Context

DianeMenu is a macOS menu-bar/window app built with SwiftUI. It has 8 screen-level views and ~8 reusable components. We recently refactored all views to an MVVM pattern with `DianeClientProtocol` for dependency injection, created `MockDianeClient` and `TestFixtures`, and have 253 passing tests including 10 snapshot tests.

The codebase currently has:
- 7 ViewModels, all accepting `DianeClientProtocol` via `init(client:)`
- Only MCPServersView has an injectable `init(viewModel:)` — the other 6 views hardcode `@State private var viewModel = XxxViewModel()`
- 6 reusable components embedded inside domain-specific view files (DetailSection, InfoRow, StringArrayEditor, KeyValueEditor, OAuthConfigEditor in MCPServersView.swift; SummaryCard in UsageView.swift)
- MasterDetailView and MasterListHeader already in their own file
- ToolsBrowserView and SettingsView depend on `@EnvironmentObject var statusMonitor: StatusMonitor`
- The Xcode project uses simple sequential decimal IDs in pbxproj (not standard hex UUIDs)

## Goals / Non-Goals

**Goals:**
- A standalone macOS app target that renders every view and component with mock data, no daemon required
- Interactive controls to tweak layout parameters (spacing, padding, font size, colors, corner radius) in real time
- State presets (empty, loading, error, loaded) switchable per view
- All 7 ViewModel-backed views become injectable, unlocking both catalog usage and expanded snapshot testing
- Shared components extracted so both targets compile them

**Non-Goals:**
- Hot-reload or live code editing — changes are ephemeral, not written back to source
- Replacing Xcode Previews — this is complementary, focused on interactive parameter exploration
- Dark mode toggle in controls — macOS system appearance handles this; the catalog inherits system setting
- Testing the catalog app itself — it's a development tool, not production code
- Full design-system token architecture — we use simple environment values, not a token pipeline

## Decisions

### 1. Source file sharing via multi-target membership (not a framework)

**Decision:** The ComponentCatalog target will compile the same source files as DianeMenu by adding them to both targets' "Compile Sources" build phases in pbxproj. No shared framework or module.

**Alternatives considered:**
- **Shared framework target**: Cleaner separation, but adds a third target, complicates the pbxproj significantly, requires restructuring imports across the entire codebase, and introduces framework linking overhead. Overkill for a dev tool.
- **`@testable import DianeMenu`**: Would require DianeMenu to be built as a framework (it's an app), and `@testable` doesn't work across app targets.

**Rationale:** Multi-target file membership is the simplest approach for Xcode. Each source file gets a second entry in the ComponentCatalog's Sources build phase. No import changes needed — types are compiled directly into the catalog binary. This is the standard Xcode pattern for companion apps that share code with a main app.

**Implication:** Every shared source file (Models, Services, Views, ViewModels, Components) needs a PBXBuildFile entry for the ComponentCatalog target. The catalog-only files (CatalogApp.swift, catalog views, state presets) belong exclusively to the catalog target.

### 2. Layout token propagation via `@Observable` CatalogTheme object

**Decision:** Create a `CatalogTheme` class (using `@Observable` macro) that holds all adjustable layout parameters. The catalog injects it into the preview's environment. Components read from it via `@Environment(CatalogTheme.self)`.

**Alternatives considered:**
- **Custom `EnvironmentKey` per parameter**: More idiomatic SwiftUI, but requires defining ~8 separate environment keys, their default values, and `.environment()` modifiers. Verbose and tedious.
- **`@Bindable` property on each component**: Would require modifying every component's API to accept theme bindings. Invasive and defeats the "no source modification" goal.
- **Wrapper view with `.font()` / `.padding()` modifiers**: Apply modifiers to the preview container rather than propagating into child views. Simple but limited — only affects the outermost frame, not internal spacing.

**Rationale:** A single `@Observable` object in the environment is clean, requires one `.environment()` call on the preview container, and components can opt-in to reading values via `@Environment(CatalogTheme.self)`. For V1, only a subset of components will read the theme — the controls still provide value by adjusting the wrapper frame's padding, background, and the preview canvas size. Over time, individual components can be updated to read theme values for their internal layout.

**Pragmatic note:** Not all layout parameters can be externally controlled without modifying components. Spacing inside a view's VStack, for instance, is hardcoded. The controls will realistically affect: (a) the preview canvas container (padding, background, size), (b) accent color (via `.tint()`), and (c) any components that are updated to read `CatalogTheme`. This is honest about the initial scope — full theme propagation is incremental.

### 3. State presets as static factory functions on a per-view enum

**Decision:** Create a `CatalogPresets` enum with nested enums per view (e.g., `CatalogPresets.MCPServers`), each containing static functions that return a configured ViewModel.

```
enum CatalogPresets {
    enum MCPServers {
        static func empty() -> MCPServersViewModel { ... }
        static func loading() -> MCPServersViewModel { ... }
        static func error() -> MCPServersViewModel { ... }
        static func loaded() -> MCPServersViewModel { ... }
    }
    // ... per view
}
```

**Alternatives considered:**
- **Protocol-based preset system**: Define a `PresetProvider` protocol. More extensible but over-engineered for a fixed set of views.
- **Dictionary/plist configuration**: External configuration. Unnecessary indirection — presets are code that constructs ViewModels.

**Rationale:** Static factory functions are simple, type-safe, and easy to read. Each function creates a `MockDianeClient`, configures it, creates the ViewModel, and sets its state properties directly. This reuses `TestFixtures` for model data. The pattern is identical to what the snapshot tests already do.

### 4. Three-column layout using NavigationSplitView

**Decision:** Use SwiftUI's `NavigationSplitView` with three columns: sidebar (catalog items), content (preview canvas), detail (state selector + controls).

**Alternatives considered:**
- **HSplitView with manual layout**: More control over proportions, but no automatic column collapse behavior and more boilerplate.
- **Custom split-view**: Maximum flexibility but unnecessary — NavigationSplitView handles the three-column pattern natively on macOS.

**Rationale:** NavigationSplitView is the idiomatic SwiftUI approach for three-column macOS apps. It provides automatic resizing, minimum widths, and column visibility toggles for free.

### 5. Catalog-only files live in `ComponentCatalog/` directory

**Decision:** Catalog-specific files (CatalogApp.swift, CatalogContentView.swift, CatalogPresets.swift, CatalogTheme.swift, component preview wrappers) live in a new `DianeMenu/ComponentCatalog/` directory alongside the existing `DianeMenu/DianeMenu/` directory.

**Rationale:** Keeps catalog code physically separate from the main app code. The pbxproj references these files only in the ComponentCatalog target's Sources build phase.

### 6. Component extraction into `DianeMenu/DianeMenu/Components/`

**Decision:** Extract shared components into `DianeMenu/DianeMenu/Components/`, one file per component. This directory is inside the main app's source tree because these components belong to the DianeMenu module — the catalog just also compiles them.

**Rationale:** These are DianeMenu components that happen to be reusable. Placing them alongside Views and ViewModels is the natural home. Both targets compile them via multi-target membership.

### 7. View injection pattern matches MCPServersView exactly

**Decision:** For each of the 6 remaining views, change:
```swift
@State private var viewModel = XxxViewModel()
```
to:
```swift
@State private var viewModel: XxxViewModel

init(viewModel: XxxViewModel = XxxViewModel()) {
    _viewModel = State(initialValue: viewModel)
}
```

**Rationale:** This is proven working in MCPServersView and the snapshot tests. The default parameter ensures zero changes at call sites. The `_viewModel = State(initialValue:)` pattern is the correct way to initialize `@State` from an external value.

### 8. SettingsView and ToolsBrowserView environment dependencies

**Decision:** For views that use `@EnvironmentObject var statusMonitor: StatusMonitor`, the catalog wraps them with `.environmentObject(MockStatusMonitor())` where `MockStatusMonitor` is a minimal StatusMonitor subclass or protocol-based mock that provides static data.

**Alternative considered:** Making StatusMonitor protocol-based. This is the correct long-term approach but is scope creep for this change — StatusMonitor has observation/timer behavior that's complex to abstract.

**Rationale:** A mock EnvironmentObject is the simplest path. SettingsView reads `statusMonitor.isConnected` and similar properties. A mock that returns fixed values is sufficient for catalog rendering.

## Risks / Trade-offs

**[Multi-target file membership is verbose in pbxproj]** → Every shared source file needs duplicate PBXBuildFile entries. We already manage pbxproj manually with sequential IDs, so this is tedious but mechanical. We'll need ~30+ new build file entries for the catalog target.

**[Layout controls have limited reach in V1]** → Most internal view spacing is hardcoded, not read from environment. The controls will primarily affect the preview container (padding, background, canvas size) and accent color. Full internal theme propagation is future work. → This is acceptable — the primary value is state previews and canvas sizing, not pixel-perfect theme control.

**[SettingsView mock may be incomplete]** → SettingsView uses `@AppStorage` for actual preferences. In the catalog, `@AppStorage` will use the catalog app's UserDefaults, which is isolated from DianeMenu's. This is actually fine — it means toggles work but don't affect the real app. → No mitigation needed; this is desirable isolation.

**[Snapshot test baselines may need re-recording]** → Extracting components into separate files shouldn't change rendering, but if the extraction accidentally changes access levels or initializer defaults, snapshot tests will catch it. → Run full test suite after extraction to verify.

**[Build time increases]** → The catalog target compiles all shared sources plus catalog-only files. This roughly doubles compile time for a clean build. → Incremental builds mitigate this; the catalog is only built when explicitly selected.

## Open Questions

1. **Should the catalog include MenuBarView?** — MenuBarView uses `@EnvironmentObject` for both StatusMonitor and UpdateChecker, and its popover presentation model is unusual. May be simpler to exclude it from V1.

2. **ToolsBrowserView's StatusMonitor usage** — ToolsBrowserView reads `statusMonitor` to get connection status. Should we make a catalog-specific wrapper that injects a mock, or defer ToolsBrowserView to V2?

3. **Preview canvas scroll behavior** — When the preview size exceeds the available space, should the canvas scroll or scale down? Scrolling is more accurate but scaling gives a better overview.
