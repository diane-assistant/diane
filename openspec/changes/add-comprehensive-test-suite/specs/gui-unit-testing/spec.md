## ADDED Requirements

### Requirement: ViewModel extraction from SwiftUI views
The system SHALL extract business logic from SwiftUI views into separate ViewModel classes that conform to the Observable protocol. Each ViewModel SHALL manage state, handle user actions, and coordinate with services (e.g., DianeClient) while views remain focused on presentation.

#### Scenario: MCPServersViewModel manages server list state
- **WHEN** MCPServersView is initialized
- **THEN** it SHALL use MCPServersViewModel to manage servers array, loading state, and error state

#### Scenario: ViewModel exposes async methods for operations
- **WHEN** user triggers an action (create, duplicate, delete server)
- **THEN** ViewModel SHALL provide async methods that views can call from Task blocks

#### Scenario: ViewModel is injectable for testing
- **WHEN** creating a ViewModel instance for testing
- **THEN** it SHALL accept dependencies via initializer (e.g., DianeClientProtocol)

### Requirement: Pure function unit tests
The system SHALL provide unit tests for pure functions that perform data transformations or calculations without side effects. Tests SHALL verify correctness of logic independent of UI state.

#### Scenario: Test duplicate name generation logic
- **WHEN** generateDuplicateName is called with "server" and existing servers list
- **THEN** test SHALL verify it returns "server (2)"

#### Scenario: Test duplicate name increments existing numbers
- **WHEN** generateDuplicateName is called with "server" and list contains "server (2)"
- **THEN** test SHALL verify it returns "server (3)"

#### Scenario: Test filtering logic
- **WHEN** filterServers is called with type filter
- **THEN** test SHALL verify only servers of matching type are returned

### Requirement: Form validation unit tests
The system SHALL provide unit tests for form validation logic extracted into ViewModels. Tests SHALL verify validation rules are correctly enforced for server creation and editing.

#### Scenario: Test create form validation requires name
- **WHEN** canCreateServer is called with empty name
- **THEN** test SHALL verify it returns false

#### Scenario: Test stdio server requires command
- **WHEN** canCreateServer is called for stdio type without command
- **THEN** test SHALL verify it returns false

#### Scenario: Test SSE server requires URL
- **WHEN** canCreateServer is called for sse type without URL
- **THEN** test SHALL verify it returns false

#### Scenario: Test valid form passes validation
- **WHEN** canCreateServer is called with all required fields filled
- **THEN** test SHALL verify it returns true

### Requirement: State management unit tests
The system SHALL provide unit tests for ViewModel state changes in response to operations. Tests SHALL verify state transitions are correct and error handling works properly.

#### Scenario: Test loading state during async operations
- **WHEN** loadServers is called on ViewModel
- **THEN** test SHALL verify isLoading becomes true during operation and false after

#### Scenario: Test error state on API failure
- **WHEN** API call fails with error
- **THEN** test SHALL verify ViewModel sets error property with error message

#### Scenario: Test successful operation updates state
- **WHEN** createServer succeeds
- **THEN** test SHALL verify new server is appended to servers array

### Requirement: XCTest framework integration
The system SHALL use XCTest framework for writing and running unit tests. Tests SHALL be organized in test classes that inherit from XCTestCase with setUp/tearDown lifecycle methods.

#### Scenario: Test files use XCTest imports
- **WHEN** creating a unit test file
- **THEN** it SHALL import XCTest framework

#### Scenario: Test classes inherit from XCTestCase
- **WHEN** defining a test class
- **THEN** it SHALL inherit from XCTestCase

#### Scenario: Test methods are discoverable
- **WHEN** test methods are named with "test" prefix
- **THEN** XCTest SHALL automatically discover and run them

#### Scenario: Assertions use XCTest API
- **WHEN** verifying test conditions
- **THEN** tests SHALL use XCTAssert family functions (XCTAssertEqual, XCTAssertTrue, etc.)
