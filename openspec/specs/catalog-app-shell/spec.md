# Capability: Catalog App Shell

## Purpose

Define the standalone ComponentCatalog macOS application target, its three-column layout, sidebar navigation, and preview canvas for rendering views and components at configurable sizes.

## Requirements

### Requirement: Catalog app launches as standalone macOS application
The ComponentCatalog SHALL be a separate macOS app target in the DianeMenu Xcode project that builds and runs independently from the main DianeMenu app.

#### Scenario: Build and launch
- **WHEN** the user selects the ComponentCatalog scheme and runs it
- **THEN** a standalone macOS window opens showing the catalog interface

#### Scenario: Independent from main app
- **WHEN** the ComponentCatalog is running
- **THEN** it does NOT require the diane daemon, network access, or any running DianeMenu instance

### Requirement: Sidebar navigation lists all cataloged items
The catalog SHALL display a sidebar listing all screen-level views and reusable components, organized by category.

#### Scenario: Screen-level views listed
- **WHEN** the catalog launches
- **THEN** the sidebar lists all 8 screen-level views: MCPServersView, AgentsView, ProvidersView, ContextsView, SchedulerView, UsageView, ToolsBrowserView, SettingsView

#### Scenario: Reusable components listed
- **WHEN** the catalog launches
- **THEN** the sidebar lists reusable components in a separate section: MasterDetailView, MasterListHeader, DetailSection, InfoRow, StringArrayEditor, KeyValueEditor, SummaryCard, OAuthConfigEditor

#### Scenario: Selecting an item shows its preview
- **WHEN** the user clicks a sidebar item
- **THEN** the main content area renders that view or component with its current state and parameters

### Requirement: Three-column layout
The catalog SHALL use a three-column layout: sidebar (component list), center (rendered preview), and trailing panel (state selector and controls).

#### Scenario: Layout structure
- **WHEN** the catalog is displaying a selected component
- **THEN** the sidebar shows the catalog items, the center shows the rendered component, and the trailing panel shows state and parameter controls

#### Scenario: Resizable columns
- **WHEN** the user drags a column divider
- **THEN** the columns resize proportionally while maintaining minimum widths

### Requirement: Preview canvas renders at configurable size
The center preview area SHALL render the selected view within a configurable frame size, allowing the user to see how it looks at different dimensions.

#### Scenario: Default preview size
- **WHEN** a screen-level view is selected
- **THEN** it renders at 800x600 by default

#### Scenario: Custom preview size
- **WHEN** the user adjusts width/height controls in the trailing panel
- **THEN** the preview re-renders at the specified dimensions immediately

#### Scenario: Reusable component preview
- **WHEN** a reusable component (e.g., InfoRow, SummaryCard) is selected
- **THEN** it renders at a size appropriate to its content (not forced into a full-window frame)
