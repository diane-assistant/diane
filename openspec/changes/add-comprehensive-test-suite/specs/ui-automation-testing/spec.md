## ADDED Requirements

### Requirement: XCUITest framework for end-to-end testing
The system SHALL use XCUITest framework to test user flows through the actual application UI. Tests SHALL interact with UI elements using accessibility identifiers and verify expected outcomes.

#### Scenario: Test launches the application
- **WHEN** UI test suite runs
- **THEN** it SHALL launch Diane application in test mode

#### Scenario: Test navigates to MCP Servers section
- **WHEN** test clicks "MCP Servers" navigation item
- **THEN** application SHALL display the MCP Servers view

#### Scenario: Test interacts with buttons and text fields
- **WHEN** test taps buttons or enters text
- **THEN** XCUITest SHALL find elements by accessibility identifiers

### Requirement: Create MCP server flow testing
The system SHALL test the complete flow of creating a new MCP server through the UI. Tests SHALL verify form validation, data entry, and successful server creation.

#### Scenario: Test opens create server sheet
- **WHEN** test taps "Add Server" button
- **THEN** create server sheet SHALL be displayed

#### Scenario: Test create button is disabled without required fields
- **WHEN** create server sheet is shown with empty fields
- **THEN** "Create" button SHALL be disabled

#### Scenario: Test entering server details enables create button
- **WHEN** test fills in server name and required type-specific fields
- **THEN** "Create" button SHALL be enabled

#### Scenario: Test successful server creation
- **WHEN** test fills valid server details and taps "Create"
- **THEN** new server SHALL appear in the server list

### Requirement: Edit MCP server flow testing
The system SHALL test the complete flow of editing an existing MCP server. Tests SHALL verify form pre-population, editing, and successful updates.

#### Scenario: Test opens edit server sheet
- **WHEN** test selects a server and taps "Edit" button
- **THEN** edit server sheet SHALL be displayed with pre-filled values

#### Scenario: Test modifying server name
- **WHEN** test changes server name and taps "Save"
- **THEN** server list SHALL show updated name

#### Scenario: Test server type is disabled in edit mode
- **WHEN** edit server sheet is displayed
- **THEN** server type picker SHALL be disabled

### Requirement: Duplicate MCP server flow testing
The system SHALL test the duplicate server functionality. Tests SHALL verify that duplicating creates a new server with copied configuration and incremented name.

#### Scenario: Test duplicate button is visible
- **WHEN** test selects a server
- **THEN** detail view SHALL show "Duplicate" button

#### Scenario: Test duplicating a server
- **WHEN** test taps "Duplicate" button
- **THEN** new server SHALL appear with name suffix "(2)"

#### Scenario: Test duplicate copies all configuration
- **WHEN** test duplicates a stdio server with args and env
- **THEN** duplicated server SHALL have identical args and env values

### Requirement: Delete MCP server flow testing
The system SHALL test the delete server functionality including confirmation dialog. Tests SHALL verify that deletion removes the server from the list.

#### Scenario: Test delete shows confirmation dialog
- **WHEN** test taps "Delete" button
- **THEN** confirmation alert SHALL be displayed

#### Scenario: Test canceling delete keeps server
- **WHEN** test taps "Cancel" in delete confirmation
- **THEN** server SHALL remain in the list

#### Scenario: Test confirming delete removes server
- **WHEN** test taps "Delete" in confirmation alert
- **THEN** server SHALL be removed from the list

### Requirement: Test environment configuration
The system SHALL provide mechanisms to configure the test environment and application state for UI tests. Tests SHALL be able to launch with specific configurations or mock data.

#### Scenario: Test launches with UI-Testing flag
- **WHEN** UI test starts
- **THEN** app SHALL launch with "UI-Testing" launch argument

#### Scenario: Test uses mock backend
- **WHEN** app launches in test mode
- **THEN** it SHALL use mock DianeClient instead of real Unix socket connection

#### Scenario: Test resets state between runs
- **WHEN** each test starts
- **THEN** application state SHALL be reset to clean slate
