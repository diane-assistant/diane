## ADDED Requirements

### Requirement: Menu bar icon is visible
The application SHALL display an icon in the macOS menu bar for quick access.

#### Scenario: Menu bar icon appears on launch
- **WHEN** the application launches
- **THEN** an icon appears in the macOS menu bar

#### Scenario: Menu bar icon reflects connection state
- **WHEN** the Diane backend connection state changes
- **THEN** the menu bar icon updates to reflect the current state (connected, disconnected, error)

### Requirement: Menu bar icon opens main window
Clicking the menu bar icon SHALL open and activate the main desktop window.

#### Scenario: Clicking icon when window is closed opens window
- **WHEN** user clicks the menu bar icon while the main window is closed
- **THEN** the main window opens and comes to the front

#### Scenario: Clicking icon when window is hidden brings it forward
- **WHEN** user clicks the menu bar icon while the main window is hidden behind other windows
- **THEN** the main window comes to the front and gains focus

#### Scenario: Clicking icon when window is minimized restores it
- **WHEN** user clicks the menu bar icon while the main window is minimized to the dock
- **THEN** the main window restores from the dock and comes to the front

### Requirement: Menu bar dropdown shows quick status
The menu bar icon SHALL display a dropdown menu with essential status information and quick actions.

#### Scenario: Dropdown shows connection status
- **WHEN** user opens the menu bar dropdown
- **THEN** the dropdown displays the current Diane backend connection status

#### Scenario: Dropdown shows version and uptime
- **WHEN** user opens the menu bar dropdown and Diane is connected
- **THEN** the dropdown displays the Diane version and uptime information

### Requirement: Menu bar provides quick control actions
The menu bar dropdown SHALL provide quick access to start, stop, and restart controls.

#### Scenario: Start button available when disconnected
- **WHEN** user opens the menu bar dropdown and Diane is not running
- **THEN** the dropdown shows a "Start" button

#### Scenario: Stop button available when connected
- **WHEN** user opens the menu bar dropdown and Diane is running
- **THEN** the dropdown shows a "Stop" button

#### Scenario: Restart button available when connected
- **WHEN** user opens the menu bar dropdown and Diane is running
- **THEN** the dropdown shows a "Restart" button

#### Scenario: Clicking Start button starts Diane
- **WHEN** user clicks the "Start" button in the menu bar dropdown
- **THEN** the application attempts to start the Diane backend

#### Scenario: Clicking Stop button stops Diane
- **WHEN** user clicks the "Stop" button in the menu bar dropdown
- **THEN** the application stops the Diane backend

#### Scenario: Clicking Restart button restarts Diane
- **WHEN** user clicks the "Restart" button in the menu bar dropdown
- **THEN** the application restarts the Diane backend

### Requirement: Menu bar provides quick MCP server status
The menu bar dropdown SHALL display the status of MCP servers.

#### Scenario: Dropdown shows MCP server list
- **WHEN** user opens the menu bar dropdown and Diane is connected
- **THEN** the dropdown displays a collapsible list of MCP servers with their connection status

#### Scenario: MCP server indicators show connection state
- **WHEN** viewing the MCP server list in the dropdown
- **THEN** each server displays a color indicator (green for connected, red for disconnected, orange for error)

### Requirement: Menu bar provides open main window action
The menu bar dropdown SHALL include an explicit action to open the main window.

#### Scenario: Open button is present in dropdown
- **WHEN** user opens the menu bar dropdown
- **THEN** the dropdown includes an "Open Diane" button

#### Scenario: Clicking Open button opens main window
- **WHEN** user clicks the "Open Diane" button in the dropdown
- **THEN** the main desktop window opens and comes to the front

### Requirement: Menu bar provides quit action
The menu bar dropdown SHALL include a quit action to terminate the application.

#### Scenario: Quit button is present in dropdown
- **WHEN** user opens the menu bar dropdown
- **THEN** the dropdown includes a "Quit" button

#### Scenario: Clicking Quit button terminates application
- **WHEN** user clicks the "Quit" button in the dropdown
- **THEN** the application saves state and quits completely

### Requirement: Menu bar shows update notifications
The menu bar icon SHALL indicate when an update is available.

#### Scenario: Update indicator appears when update available
- **WHEN** a new version of Diane is available
- **THEN** the menu bar icon displays a visual indicator (badge or modified icon)

#### Scenario: Dropdown shows update information
- **WHEN** user opens the menu bar dropdown and an update is available
- **THEN** the dropdown displays update information with version numbers and an install button

#### Scenario: Clicking install button performs update
- **WHEN** user clicks the "Install" button for an available update
- **THEN** the application downloads and installs the update with progress feedback
