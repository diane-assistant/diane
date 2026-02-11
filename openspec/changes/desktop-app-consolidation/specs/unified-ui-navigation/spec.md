## ADDED Requirements

### Requirement: Sidebar navigation is visible
The main window SHALL display a persistent sidebar containing navigation links to all major functional areas.

#### Scenario: Sidebar shows all sections
- **WHEN** the main window is open
- **THEN** the sidebar displays navigation items for Tools, Scheduler, Agents, Contexts, Providers, Usage, and Settings

#### Scenario: Each section has an icon
- **WHEN** viewing the sidebar
- **THEN** each navigation item displays an appropriate SF Symbol icon alongside its label

### Requirement: Navigation sections are selectable
Users SHALL be able to navigate between different functional areas by selecting items in the sidebar.

#### Scenario: Clicking Tools shows tools browser
- **WHEN** user clicks the "Tools" navigation item
- **THEN** the detail view displays the tools browser interface

#### Scenario: Clicking Scheduler shows scheduler
- **WHEN** user clicks the "Scheduler" navigation item
- **THEN** the detail view displays the scheduler interface

#### Scenario: Clicking Agents shows agents management
- **WHEN** user clicks the "Agents" navigation item
- **THEN** the detail view displays the ACP agents management interface

#### Scenario: Clicking Contexts shows contexts
- **WHEN** user clicks the "Contexts" navigation item
- **THEN** the detail view displays the MCP server contexts interface

#### Scenario: Clicking Providers shows providers
- **WHEN** user clicks the "Providers" navigation item
- **THEN** the detail view displays the embedding and LLM providers interface

#### Scenario: Clicking Usage shows usage data
- **WHEN** user clicks the "Usage" navigation item
- **THEN** the detail view displays usage statistics and costs

#### Scenario: Clicking Settings shows settings
- **WHEN** user clicks the "Settings" navigation item
- **THEN** the detail view displays application settings

### Requirement: Selected section is visually indicated
The sidebar SHALL highlight the currently selected section.

#### Scenario: Active section is highlighted
- **WHEN** user navigates to a section
- **THEN** that section's navigation item appears highlighted in the sidebar

#### Scenario: Only one section is highlighted at a time
- **WHEN** user switches from one section to another
- **THEN** the previous section's highlight is removed and the new section becomes highlighted

### Requirement: Last viewed section is remembered
The application SHALL remember which section was last viewed and restore it on next launch.

#### Scenario: Application remembers last section
- **WHEN** user views the "Providers" section and then quits the application
- **THEN** the application opens to the "Providers" section on next launch

#### Scenario: Default section on first launch
- **WHEN** user launches the application for the first time
- **THEN** the application opens to the "Tools" section by default

### Requirement: Sidebar can be collapsed
Users SHALL be able to collapse the sidebar to maximize content viewing area.

#### Scenario: User collapses sidebar
- **WHEN** user clicks the sidebar collapse control
- **THEN** the sidebar collapses showing only icons without labels

#### Scenario: User expands collapsed sidebar
- **WHEN** user clicks the expand control on a collapsed sidebar
- **THEN** the sidebar expands showing both icons and labels

#### Scenario: Sidebar state persists
- **WHEN** user collapses the sidebar and quits the application
- **THEN** the sidebar remains collapsed on next launch

### Requirement: Keyboard shortcuts enable quick navigation
Users SHALL be able to navigate between sections using keyboard shortcuts.

#### Scenario: Cmd+1 navigates to Tools
- **WHEN** user presses Command+1
- **THEN** the application navigates to the Tools section

#### Scenario: Cmd+2 navigates to Scheduler
- **WHEN** user presses Command+2
- **THEN** the application navigates to the Scheduler section

#### Scenario: Cmd+3 navigates to Agents
- **WHEN** user presses Command+3
- **THEN** the application navigates to the Agents section

#### Scenario: Cmd+4 navigates to Contexts
- **WHEN** user presses Command+4
- **THEN** the application navigates to the Contexts section

#### Scenario: Cmd+5 navigates to Providers
- **WHEN** user presses Command+5
- **THEN** the application navigates to the Providers section

#### Scenario: Cmd+6 navigates to Usage
- **WHEN** user presses Command+6
- **THEN** the application navigates to the Usage section

#### Scenario: Cmd+, opens Settings
- **WHEN** user presses Command+,
- **THEN** the application navigates to the Settings section

### Requirement: Detail view updates based on selection
The main content area SHALL display the appropriate interface for the selected section.

#### Scenario: Detail view switches without page reload
- **WHEN** user navigates between sections
- **THEN** the detail view updates smoothly without flickering or reloading the entire window

#### Scenario: Each view maintains its own state
- **WHEN** user switches away from a section and then returns to it
- **THEN** the view retains any state like scroll position or expanded sections (where applicable)
