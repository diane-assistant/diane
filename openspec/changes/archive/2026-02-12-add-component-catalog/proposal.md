## Why

Adjusting the layout of Diane's SwiftUI views currently requires an edit-build-run cycle for every small change — spacing, padding, font sizes, colors. There is no way to see all views side-by-side with different states, or to interactively tweak visual properties without recompiling. A Storybook-like component catalog app would let a developer (or AI assistant) see the full component system at a glance, manipulate layout parameters with live controls, and make informed design decisions before committing code changes.

## What Changes

- **New macOS app target** (`ComponentCatalog`) in the existing Xcode project that imports `Diane` module types via `@testable import`-style access or a shared framework
- **Catalog sidebar** listing every screen-level view (MCPServersView, AgentsView, ProvidersView, ContextsView, SchedulerView, UsageView, ToolsBrowserView, SettingsView) and every reusable component (MasterDetailView, MasterListHeader, DetailSection, InfoRow, StringArrayEditor, KeyValueEditor, SummaryCard, etc.)
- **State selector** for each view — pick from predefined states (empty, loading, error, loaded with N items, selected item) backed by MockDianeClient and TestFixtures
- **Interactive controls panel** exposing key layout parameters (spacing, padding, font sizes, corner radii, colors) as sliders, steppers, and color pickers that update the preview in real time without recompilation
- **Injectable init pattern** added to the 6 remaining ViewModel-backed views (AgentsView, ProvidersView, ContextsView, ToolsBrowserView, UsageView, SchedulerView) matching the pattern already in MCPServersView — `init(viewModel:)` with a default
- **Extraction of shared components** currently embedded in domain-specific view files (DetailSection, InfoRow, StringArrayEditor, KeyValueEditor from MCPServersView.swift; SummaryCard from UsageView.swift) into a shared `Components/` directory so both the main app and the catalog can reference them

## Capabilities

### New Capabilities

- `catalog-app-shell`: The standalone macOS app target — sidebar navigation, component selection, preview canvas area, and controls panel layout
- `catalog-state-management`: State presets for each view (empty, loading, error, loaded variants) using MockDianeClient and TestFixtures, with a selector UI to switch between them
- `catalog-interactive-controls`: Live parameter controls (sliders, steppers, pickers) that bind to layout tokens/values and update rendered previews without recompilation
- `view-injection-pattern`: Adding `init(viewModel:)` to the 6 views that currently hardcode their ViewModel construction, enabling external state injection from the catalog (and from tests)
- `shared-components-extraction`: Moving reusable components (DetailSection, InfoRow, StringArrayEditor, KeyValueEditor, SummaryCard, OAuthConfigEditor) out of domain-specific view files into a shared location accessible by both the main app and the catalog

### Modified Capabilities

_(No existing specs to modify — `openspec/specs/` is empty)_

## Impact

- **Xcode project**: New app target (`ComponentCatalog`) with its own build scheme, sharing source files with the main target or via a shared framework/module
- **Source files**: 6 view files gain `init(viewModel:)` (non-breaking — default parameter preserves existing call sites); 2-3 view files have components extracted into separate files (file moves, not behavior changes)
- **Dependencies**: Reuses existing MockDianeClient and TestFixtures from the test target; no new external dependencies
- **Build time**: Adds a second app target; no impact on main app build unless shared framework approach is chosen
- **Existing tests**: The view injection changes will benefit the existing snapshot test infrastructure (more views become snapshotable)
