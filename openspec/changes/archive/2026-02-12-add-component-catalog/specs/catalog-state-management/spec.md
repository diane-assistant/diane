## ADDED Requirements

### Requirement: Each view has predefined state presets
Every screen-level view in the catalog SHALL have a set of named state presets that configure its ViewModel with representative data, without requiring network or daemon access.

#### Scenario: State presets for ViewModel-backed views
- **WHEN** a ViewModel-backed view (MCPServersView, AgentsView, ProvidersView, ContextsView, SchedulerView, UsageView, ToolsBrowserView) is selected in the catalog
- **THEN** the trailing panel lists at minimum these state presets: Empty, Loading, Error, Loaded

#### Scenario: Loaded state variants
- **WHEN** the "Loaded" preset is selected
- **THEN** the view renders with realistic mock data from TestFixtures (multiple items of varying types and states)

#### Scenario: Empty state
- **WHEN** the "Empty" preset is selected
- **THEN** the view renders with zero items and isLoading=false, showing the empty-state placeholder

#### Scenario: Loading state
- **WHEN** the "Loading" preset is selected
- **THEN** the view renders with isLoading=true, showing loading indicators

#### Scenario: Error state
- **WHEN** the "Error" preset is selected
- **THEN** the view renders with a representative error message, showing the error UI

### Requirement: State presets use MockDianeClient and TestFixtures
State presets SHALL be constructed using MockDianeClient for the client dependency and TestFixtures for model data, reusing the existing test infrastructure.

#### Scenario: Mock client provides data
- **WHEN** a state preset configures a ViewModel
- **THEN** the ViewModel's client property is a MockDianeClient instance with pre-configured return values

#### Scenario: No daemon dependency
- **WHEN** any state preset is active
- **THEN** no network requests or Unix socket connections are attempted

### Requirement: State selector UI
The trailing panel SHALL include a picker or segmented control to switch between state presets for the currently selected view.

#### Scenario: Switching states
- **WHEN** the user selects a different state preset from the selector
- **THEN** the preview re-renders with the new state within one frame (no visible delay)

#### Scenario: State persists during session
- **WHEN** the user navigates away from a view and back
- **THEN** the previously selected state preset is remembered for that view

### Requirement: SettingsView state management
SettingsView, which uses EnvironmentObject rather than a ViewModel, SHALL be rendered with mock environment objects.

#### Scenario: SettingsView renders in catalog
- **WHEN** SettingsView is selected in the catalog
- **THEN** it renders with a mock StatusMonitor injected via `.environmentObject()`, showing representative settings state

### Requirement: Reusable component state presets
Reusable components (InfoRow, SummaryCard, DetailSection, etc.) SHALL have presets that demonstrate different content configurations.

#### Scenario: InfoRow presets
- **WHEN** InfoRow is selected in the catalog
- **THEN** presets include: short label/value, long label/value, empty value

#### Scenario: SummaryCard presets
- **WHEN** SummaryCard is selected in the catalog
- **THEN** presets include: normal value, large number, zero value
