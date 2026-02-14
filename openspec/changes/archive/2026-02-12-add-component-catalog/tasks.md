## 1. View Injection Pattern (6 views)

- [x] 1.1 Add injectable `init(viewModel:)` to AgentsView (change `@State private var viewModel = AgentsViewModel()` to use `_viewModel = State(initialValue:)` pattern with default parameter)
- [x] 1.2 Add injectable `init(viewModel:)` to ProvidersView
- [x] 1.3 Add injectable `init(viewModel:)` to ContextsView
- [x] 1.4 Add injectable `init(viewModel:)` to SchedulerView
- [x] 1.5 Add injectable `init(viewModel:)` to UsageView
- [x] 1.6 Add injectable `init(viewModel:)` to ToolsBrowserView
- [x] 1.7 Build Diane target to verify all call sites (MainWindowView etc.) still compile with no-argument constructors
- [x] 1.8 Run existing test suite (253 tests) to confirm no regressions

## 2. Shared Components Extraction

- [x] 2.1 Create `Diane/Diane/Components/` directory
- [x] 2.2 Extract `DetailSection` from MCPServersView.swift into `Components/DetailSection.swift`
- [x] 2.3 Extract `InfoRow` from MCPServersView.swift into `Components/InfoRow.swift`
- [x] 2.4 Extract `StringArrayEditor` from MCPServersView.swift into `Components/StringArrayEditor.swift`
- [x] 2.5 Extract `KeyValueEditor` from MCPServersView.swift into `Components/KeyValueEditor.swift`
- [x] 2.6 Extract `OAuthConfigEditor` from MCPServersView.swift into `Components/OAuthConfigEditor.swift`
- [x] 2.7 Extract `SummaryCard` from UsageView.swift into `Components/SummaryCard.swift`
- [x] 2.8 Add all 6 component files to the Diane target in pbxproj (PBXFileReference + PBXBuildFile + PBXGroup entries)
- [x] 2.9 Remove extracted component definitions from MCPServersView.swift and UsageView.swift
- [x] 2.10 Build Diane target to verify extraction preserves compilation
- [x] 2.11 Run full test suite (including snapshot tests) to verify visual output unchanged

## 3. ComponentCatalog Xcode Target Setup

- [x] 3.1 Create `Diane/ComponentCatalog/` directory for catalog-only source files
- [x] 3.2 Create minimal `CatalogApp.swift` (SwiftUI App entry point with `@main`)
- [x] 3.3 Add ComponentCatalog native target to pbxproj (PBXNativeTarget, XCBuildConfiguration Debug/Release, XCConfigurationList, product reference)
- [x] 3.4 Add ComponentCatalog scheme or ensure the target is buildable
- [x] 3.5 Add all shared source files (Models, Services, Views, ViewModels, Components) to ComponentCatalog target's Compile Sources build phase via PBXBuildFile entries
- [x] 3.6 Add catalog-only files (CatalogApp.swift) to ComponentCatalog target's Compile Sources
- [x] 3.7 Build ComponentCatalog target to verify it compiles with shared sources

## 4. Catalog App Shell (Three-Column Layout)

- [x] 4.1 Create `CatalogContentView.swift` with NavigationSplitView three-column layout (sidebar, content, detail)
- [x] 4.2 Define `CatalogItem` enum listing all screen-level views and reusable components, organized by category
- [x] 4.3 Implement sidebar with two sections: "Screen Views" (8 items) and "Reusable Components" (8 items including MasterDetailView, MasterListHeader)
- [x] 4.4 Implement center column as preview canvas with configurable frame size (default 800x600 for views)
- [x] 4.5 Implement trailing panel placeholder with state selector and controls sections
- [x] 4.6 Add CatalogContentView.swift to ComponentCatalog target in pbxproj
- [x] 4.7 Wire CatalogApp.swift to open a window with CatalogContentView
- [x] 4.8 Build and run ComponentCatalog to verify three-column layout renders

## 5. CatalogTheme (Observable Layout Token Object)

- [x] 5.1 Create `CatalogTheme.swift` with `@Observable` class holding layout parameters: spacing, padding, fontSize, cornerRadius, accentColor, backgroundColor, canvasWidth, canvasHeight
- [x] 5.2 Set sensible defaults for all parameters (spacing: 8, padding: 16, fontSize: 13, cornerRadius: 8, accent: .accentColor, bg: clear, canvas: 800x600)
- [x] 5.3 Inject CatalogTheme into the preview container's environment in CatalogContentView
- [x] 5.4 Add CatalogTheme.swift to ComponentCatalog target in pbxproj
- [x] 5.5 Build to verify CatalogTheme compiles and integrates

## 6. State Presets

- [x] 6.1 Create `CatalogPresets.swift` with `enum CatalogPresets` and nested per-view enums
- [x] 6.2 Implement `CatalogPresets.MCPServers` presets: empty(), loading(), error(), loaded() — using MockDianeClient + TestFixtures
- [x] 6.3 Implement `CatalogPresets.Agents` presets: empty(), loading(), error(), loaded()
- [x] 6.4 Implement `CatalogPresets.Providers` presets: empty(), loading(), error(), loaded()
- [x] 6.5 Implement `CatalogPresets.Contexts` presets: empty(), loading(), error(), loaded()
- [x] 6.6 Implement `CatalogPresets.Scheduler` presets: empty(), loading(), error(), loaded()
- [x] 6.7 Implement `CatalogPresets.Usage` presets: empty(), loading(), error(), loaded()
- [x] 6.8 Implement `CatalogPresets.ToolsBrowser` presets: empty(), loading(), error(), loaded()
- [x] 6.9 Implement reusable component presets: InfoRow (short, long, empty), SummaryCard (normal, large, zero), DetailSection (with/without content), StringArrayEditor/KeyValueEditor (empty, populated), OAuthConfigEditor (configured, unconfigured)
- [x] 6.10 Add MockDianeClient.swift and TestFixtures.swift to ComponentCatalog target's build sources (multi-target membership)
- [x] 6.11 Add CatalogPresets.swift to ComponentCatalog target in pbxproj
- [x] 6.12 Build to verify all presets compile

## 7. Mock Environment Dependencies

- [x] 7.1 Create `MockStatusMonitor.swift` — a StatusMonitor subclass (or mock) providing static values for `isConnected`, etc.
- [x] 7.2 Wrap ToolsBrowserView and SettingsView catalog previews with `.environmentObject(MockStatusMonitor())`
- [x] 7.3 Add MockStatusMonitor.swift to ComponentCatalog target in pbxproj
- [x] 7.4 Build to verify ToolsBrowserView and SettingsView render in the catalog

## 8. State Selector UI

- [x] 8.1 Create state selector (Picker/segmented control) in the trailing panel that lists preset names for the selected catalog item
- [x] 8.2 Wire state selection to recreate the preview view with the chosen preset's ViewModel
- [x] 8.3 Persist selected preset per catalog item during the session (using a dictionary keyed by CatalogItem)
- [x] 8.4 Verify switching between presets updates the preview immediately

## 9. Interactive Controls Panel

- [x] 9.1 Add spacing slider (range 0–32, step 1) bound to CatalogTheme.spacing
- [x] 9.2 Add padding slider (range 0–48, step 1) bound to CatalogTheme.padding
- [x] 9.3 Add font size stepper (range 8–32, step 1) bound to CatalogTheme.fontSize
- [x] 9.4 Add corner radius slider (range 0–24, step 1) bound to CatalogTheme.cornerRadius
- [x] 9.5 Add accent color picker bound to CatalogTheme.accentColor, applied via `.tint()` on preview container
- [x] 9.6 Add background color picker bound to CatalogTheme.backgroundColor, applied via `.background()` on preview container
- [x] 9.7 Add canvas width/height numeric fields bound to CatalogTheme.canvasWidth/canvasHeight
- [x] 9.8 Add size preset buttons: "Compact" (400x300), "Default" (800x600), "Wide" (1200x800)
- [x] 9.9 Display current numeric values next to each slider/stepper
- [x] 9.10 Apply CatalogTheme values to the preview container (padding, background, tint, frame size)

## 10. Preview Rendering Wiring

- [x] 10.1 Create preview factory in CatalogContentView that maps CatalogItem + selected preset → rendered SwiftUI View
- [x] 10.2 Wire screen-level view rendering: instantiate each view with the preset's ViewModel via injectable init
- [x] 10.3 Wire reusable component rendering: instantiate each component with preset data
- [x] 10.4 Apply CatalogTheme environment values to the preview container wrapper
- [x] 10.5 Verify all 8 screen-level views render correctly in the catalog with each preset
- [x] 10.6 Verify all reusable components render correctly with their presets

## 11. Final Integration and Verification

- [x] 11.1 Build Diane target — verify no regressions from view injection and component extraction
- [x] 11.2 Run full Diane test suite — verify all 253+ tests pass
- [x] 11.3 Build ComponentCatalog target — verify clean compilation
- [ ] 11.4 Launch ComponentCatalog — verify sidebar lists all items, preview renders, controls respond
- [ ] 11.5 Verify state preset switching works for all 7 ViewModel-backed views
- [ ] 11.6 Verify interactive controls update the preview canvas in real time
