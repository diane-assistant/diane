## ADDED Requirements

### Requirement: Tab-based navigation on iPhone
The app SHALL use a tab bar for primary navigation on iPhone (compact horizontal size class).

#### Scenario: App launches on iPhone
- **WHEN** the app launches on an iPhone
- **THEN** the bottom tab bar displays tabs for: Status, Servers, Agents, More
- **AND** the Status tab is selected by default

#### Scenario: User switches tabs
- **WHEN** the user taps a tab bar item
- **THEN** the corresponding section view is displayed
- **AND** the tab bar remains visible at the bottom

#### Scenario: More tab shows additional sections
- **WHEN** the user taps the "More" tab
- **THEN** a list is shown with: Providers, Contexts, Jobs, Usage, Settings

### Requirement: Sidebar navigation on iPad
The app SHALL use a sidebar for primary navigation on iPad (regular horizontal size class).

#### Scenario: App launches on iPad
- **WHEN** the app launches on an iPad in landscape orientation
- **THEN** a sidebar is displayed on the leading edge listing all sections: Status, MCP Servers, Agents, Contexts, Providers, Jobs, Usage, Settings
- **AND** the detail area shows the selected section content

#### Scenario: iPad portrait mode
- **WHEN** the iPad is in portrait orientation
- **THEN** the sidebar is collapsible via the standard SwiftUI toggle
- **AND** the detail area takes full width when the sidebar is hidden

#### Scenario: User selects section in sidebar
- **WHEN** the user taps a section in the sidebar
- **THEN** the detail area updates to show that section's content
- **AND** the selected item is highlighted in the sidebar

### Requirement: Navigation adapts to size class
The app SHALL automatically switch between tab and sidebar navigation based on the device's horizontal size class.

#### Scenario: iPad in split view (compact)
- **WHEN** the app is in an iPad Split View with compact horizontal size class
- **THEN** the app uses tab-based navigation (same as iPhone)

#### Scenario: iPad in full screen (regular)
- **WHEN** the app is in full screen on iPad with regular horizontal size class
- **THEN** the app uses sidebar navigation

### Requirement: Section navigation preserves state
The app SHALL preserve the user's navigation position within sections.

#### Scenario: User navigates within a section then switches
- **WHEN** the user drills into a detail view (e.g., a specific MCP server) and then switches to another tab/section
- **THEN** returning to the original section shows the same detail view

#### Scenario: State resets on server change
- **WHEN** the user changes the connected server address in Settings
- **THEN** all navigation state resets to the root of each section
