## ADDED Requirements

### Requirement: Test Connection button triggers connectivity check

The MCP server detail view SHALL display a "Test Connection" button for non-builtin servers. Clicking it SHALL trigger a connectivity check and display the result inline in the detail view.

#### Scenario: Test button visible for non-builtin servers
- **WHEN** a non-builtin server is selected
- **THEN** the detail view SHALL display a "Test Connection" button in the Runtime Status section

#### Scenario: Test button hidden for built-in servers
- **WHEN** a built-in server is selected
- **THEN** no "Test Connection" button SHALL be displayed

#### Scenario: Test in progress
- **WHEN** the user clicks "Test Connection"
- **THEN** the button SHALL show a loading indicator (spinner), and the button SHALL be disabled until the test completes

#### Scenario: Successful connection test
- **WHEN** the test completes and the server is connected
- **THEN** a success indicator SHALL be displayed inline (green status with "Connected" text and tool/prompt/resource counts), and the loading indicator SHALL be removed

#### Scenario: Failed connection test
- **WHEN** the test completes and the server is disconnected or an error occurs
- **THEN** a failure indicator SHALL be displayed inline (red status with "Disconnected" text and error message if available), and the loading indicator SHALL be removed

### Requirement: Test Connection uses status refresh

The test connection action SHALL refresh the server data and runtime status from the daemon, then read the connection state from the status monitor. This approach uses the existing `StatusMonitor` infrastructure rather than requiring a dedicated test endpoint.

#### Scenario: Data refreshed on test
- **WHEN** the user clicks "Test Connection"
- **THEN** the system SHALL call `loadData()` to refresh server and status information from the daemon before displaying the result

#### Scenario: Test result reflects current runtime status
- **WHEN** the data refresh completes
- **THEN** the displayed connection status SHALL match the `StatusMonitor` entry for the selected server (connected/disconnected, tool count, error message)

### Requirement: Test result display

The test result SHALL be displayed as a transient banner or updated status section in the detail view. The result SHALL persist until the user navigates away from the server or initiates another test.

#### Scenario: Result persists while server is selected
- **WHEN** a test result is displayed and the user stays on the same server
- **THEN** the result SHALL remain visible until another test is triggered or a different server is selected

#### Scenario: Result cleared on server switch
- **WHEN** the user selects a different server after a test result is displayed
- **THEN** the previous test result state SHALL be cleared
