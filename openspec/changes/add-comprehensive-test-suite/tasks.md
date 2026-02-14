# Tasks: Comprehensive Test Suite Implementation

## 1. Test Infrastructure Setup

- [x] 1.1 Create DianeTests target in Xcode project (Unit Test Bundle)
- [ ] 1.2 Create DianeUITests target in Xcode project (UI Test Bundle)
- [ ] 1.3 Add ViewInspector (~0.9.0) dependency via Swift Package Manager
- [ ] 1.4 Add SnapshotTesting (~1.12.0) dependency via Swift Package Manager
- [ ] 1.5 Link ViewInspector to DianeTests target
- [ ] 1.6 Link SnapshotTesting to DianeTests target
- [ ] 1.7 Configure Diane scheme to run DianeTests with code coverage enabled
- [ ] 1.8 Create separate scheme for DianeUITests (or disable UI tests in main scheme)
- [x] 1.9 Verify both test targets build successfully

## 2. Test Helpers and Fixtures

- [x] 2.1 Create DianeTests/TestHelpers/ directory
- [x] 2.2 Create TestFixtures.swift with sample MCPServerConfig factory functions
- [ ] 2.3 Create AsyncTestHelpers.swift with async/await test utilities
- [ ] 2.4 Create ViewTestHelpers.swift with SwiftUI view rendering utilities
- [ ] 2.5 Create DianeUITests/TestHelpers/ directory
- [ ] 2.6 Create XCUIElementHelpers.swift with UI test utility extensions
- [ ] 2.7 Write DianeTests/README.md with testing guide and examples
- [ ] 2.8 Write DianeUITests/README.md with UI testing guide

## 3. DianeClient Protocol Abstraction

- [x] 3.1 Define DianeClientProtocol with all existing DianeClient public methods
- [x] 3.2 Add getMCPServerConfigs() async throws -> [MCPServerConfig] to protocol
- [x] 3.3 Add createMCPServerConfig(_:) async throws -> MCPServerConfig to protocol
- [x] 3.4 Add updateMCPServerConfig(_:) async throws -> MCPServerConfig to protocol
- [x] 3.5 Add deleteMCPServerConfig(_:) async throws to protocol
- [x] 3.6 Add any other daemon communication methods to protocol
- [x] 3.7 Make DianeClient conform to DianeClientProtocol (no implementation changes)
- [x] 3.8 Verify app builds and runs without changes to existing behavior

## 4. MockDianeClient Implementation

- [x] 4.1 Create MockDianeClient.swift in DianeTests/TestHelpers/
- [x] 4.2 Implement MockDianeClient class conforming to DianeClientProtocol
- [x] 4.3 Add configurable serverConfigs property for mock data
- [x] 4.4 Add shouldThrowError property for error simulation
- [x] 4.5 Add methodCallCounts dictionary for invocation tracking
- [x] 4.6 Implement getMCPServerConfigs() with configurable responses
- [x] 4.7 Implement createMCPServerConfig(_:) with validation and state updates
- [x] 4.8 Implement updateMCPServerConfig(_:) with validation and state updates
- [x] 4.9 Implement deleteMCPServerConfig(_:) with state updates
- [x] 4.10 Add helper methods for verifying method calls in tests

## 5. MCPServersViewModel Extraction

- [x] 5.1 Create Diane/ViewModels/ directory
- [x] 5.2 Create MCPServersViewModel.swift with @Observable macro
- [x] 5.3 Move servers array from MCPServersView to ViewModel
- [x] 5.4 Move isLoading state from MCPServersView to ViewModel
- [x] 5.5 Move errorMessage state from MCPServersView to ViewModel
- [x] 5.6 Add client: DianeClientProtocol property with default = DianeClient()
- [x] 5.7 Move loadServers() method to ViewModel
- [x] 5.8 Move createServer(_:) method to ViewModel
- [x] 5.9 Move updateServer(_:) method to ViewModel
- [x] 5.10 Move deleteServer(_:) method to ViewModel
- [x] 5.11 Move duplicateServer(_:) method to ViewModel
- [x] 5.12 Update MCPServersView to use @State var viewModel: MCPServersViewModel
- [x] 5.13 Update MCPServersView to delegate all actions to viewModel
- [ ] 5.14 Test app manually to verify MCP Servers view works identically

## 6. Pure Function Extraction (if needed)

- [x] 6.1 Extract duplicate name generation logic into standalone function
- [x] 6.2 Extract server filtering logic into standalone function (if exists)
- [x] 6.3 Extract form validation logic into standalone functions
- [x] 6.4 Add unit tests for extracted pure functions

## 7. Unit Tests for MCPServersViewModel

- [x] 7.1 Create DianeTests/UnitTests/ViewModels/ directory
- [x] 7.2 Create MCPServersViewModelTests.swift
- [x] 7.3 Test loadServers() sets isLoading true during operation and false after
- [x] 7.4 Test loadServers() populates servers array on success
- [x] 7.5 Test loadServers() sets errorMessage on failure
- [x] 7.6 Test createServer(_:) appends new server to servers array
- [x] 7.7 Test createServer(_:) calls client.createMCPServerConfig with correct parameters
- [x] 7.8 Test updateServer(_:) updates existing server in array
- [x] 7.9 Test deleteServer(_:) removes server from array
- [x] 7.10 Test duplicateServer(_:) creates new server with "(2)" suffix
- [x] 7.11 Test duplicateServer(_:) increments number if "(2)" already exists
- [x] 7.12 Test error handling for all async operations

## 8. Unit Tests for Form Validation

- [x] 8.1 Create ServerFormViewModelTests.swift (or add to MCPServersViewModelTests)
- [x] 8.2 Test canCreateServer returns false when name is empty
- [x] 8.3 Test canCreateServer returns false for stdio type without command
- [x] 8.4 Test canCreateServer returns false for SSE type without URL
- [x] 8.5 Test canCreateServer returns true when all required fields are filled
- [x] 8.6 Test validation updates as form fields change

## 9. Integration Tests for DianeClient

- [x] 9.1 Create DianeTests/IntegrationTests/ directory
- [x] 9.2 Create DianeClientProtocolTests.swift
- [x] 9.3 Test getMCPServerConfigs() returns all servers from mock
- [x] 9.4 Test getMCPServerConfigs() returns empty array when no servers
- [x] 9.5 Test getMCPServerConfigs() throws error on malformed data
- [x] 9.6 Test createMCPServerConfig(_:) returns new config with generated ID
- [x] 9.7 Test createMCPServerConfig(_:) throws error for duplicate name
- [x] 9.8 Test createMCPServerConfig(_:) throws error for invalid command
- [x] 9.9 Test updateMCPServerConfig(_:) returns updated config
- [x] 9.10 Test updateMCPServerConfig(_:) throws error for non-existent ID
- [x] 9.11 Test deleteMCPServerConfig(_:) removes server successfully
- [x] 9.12 Test deleteMCPServerConfig(_:) throws error for non-existent ID
- [x] 9.13 Test async/await behavior with proper error handling
- [x] 9.14 Test task cancellation propagates correctly

## 10. Snapshot Tests for MCPServersView

- [x] 10.1 Create DianeTests/SnapshotTests/ directory
- [x] 10.2 Create MCPServersViewSnapshotTests.swift
- [x] 10.3 Test snapshot of MCPServersView with empty state (no servers)
- [x] 10.4 Test snapshot of MCPServersView with 3 sample servers
- [x] 10.5 Test snapshot of MCPServersView with selected server
- [x] 10.6 Test snapshot of MCPServersView with error message
- [x] 10.7 Test snapshot of MCPServersView in light mode (baseline)
- [ ] 10.8 Test snapshot of MCPServersView in dark mode
- [x] 10.9 Test snapshot at minimum window size (800x600)
- [ ] 10.10 Test snapshot at standard window size (1024x768)
- [ ] 10.11 Test snapshot at large window size (1920x1080)
- [ ] 10.12 Commit generated snapshots to version control

## 11. Snapshot Tests for Server Form

- [ ] 11.1 Create ServerFormSnapshotTests.swift
- [ ] 11.2 Test snapshot of empty server creation form
- [ ] 11.3 Test snapshot of form with valid data filled
- [ ] 11.4 Test snapshot of form with validation errors
- [ ] 11.5 Test snapshot of form in loading state (during save)
- [ ] 11.6 Test snapshot of form in light mode
- [ ] 11.7 Test snapshot of form in dark mode
- [ ] 11.8 Commit generated snapshots to version control

## 12. UI Tests for MCP Server Creation

- [ ] 12.1 Create DianeUITests/MCPServersUITests.swift
- [ ] 12.2 Implement app launch with "UI-Testing" argument
- [ ] 12.3 Test navigation to MCP Servers section
- [ ] 12.4 Test opening create server sheet via "Add Server" button
- [ ] 12.5 Test "Create" button is disabled with empty fields
- [ ] 12.6 Test entering server name and command enables "Create" button
- [ ] 12.7 Test filling stdio server form (name, command, args)
- [ ] 12.8 Test tapping "Create" adds new server to list
- [ ] 12.9 Test new server appears in list with correct name

## 13. UI Tests for MCP Server Editing

- [ ] 13.1 Test selecting existing server from list
- [ ] 13.2 Test opening edit server sheet via "Edit" button
- [ ] 13.3 Test edit sheet is pre-filled with server data
- [ ] 13.4 Test server type picker is disabled in edit mode
- [ ] 13.5 Test changing server name
- [ ] 13.6 Test tapping "Save" updates server in list
- [ ] 13.7 Test updated name appears in server list

## 14. UI Tests for MCP Server Duplication

- [ ] 14.1 Test "Duplicate" button is visible for selected server
- [ ] 14.2 Test tapping "Duplicate" creates new server
- [ ] 14.3 Test duplicated server has "(2)" suffix
- [ ] 14.4 Test duplicated server copies all configuration (command, args, env)
- [ ] 14.5 Test duplicating again creates "(3)" suffix

## 15. UI Tests for MCP Server Deletion

- [ ] 15.1 Test tapping "Delete" button shows confirmation alert
- [ ] 15.2 Test "Cancel" in confirmation keeps server in list
- [ ] 15.3 Test "Delete" in confirmation removes server from list
- [ ] 15.4 Test server is no longer visible after deletion

## 16. Test Environment Configuration

- [ ] 16.1 Update DianeApp.swift to detect "UI-Testing" launch argument
- [ ] 16.2 Use MockDianeClient when app is launched in test mode
- [ ] 16.3 Configure MockDianeClient with sample data for UI tests
- [ ] 16.4 Ensure test environment uses isolated app container
- [ ] 16.5 Add accessibility identifiers to key UI elements (buttons, text fields)
- [ ] 16.6 Verify UI tests can find and interact with all necessary elements

## 17. Documentation and Examples

- [ ] 17.1 Document ViewModel testing pattern with examples in README
- [ ] 17.2 Document integration testing pattern with MockDianeClient examples
- [ ] 17.3 Document snapshot testing workflow (recording, updating)
- [ ] 17.4 Document UI testing patterns and best practices
- [ ] 17.5 Document test naming conventions
- [ ] 17.6 Document how to run tests via Xcode (Cmd+U)
- [ ] 17.7 Document how to run tests via command line (xcodebuild)
- [ ] 17.8 Document how to run specific test subsets
- [ ] 17.9 Add example test files demonstrating each test type

## 18. Code Coverage Configuration

- [x] 18.1 Enable code coverage in Diane test scheme
- [x] 18.2 Configure coverage to collect data for Diane target
- [ ] 18.3 Verify coverage reports appear in Xcode's coverage navigator
- [ ] 18.4 Document target coverage thresholds (80% for ViewModels, 60% overall)
- [ ] 18.5 Run full test suite and review coverage report

## 19. Verification and Quality Assurance

- [x] 19.1 Run all unit tests and verify 100% pass
- [x] 19.2 Run all integration tests and verify 100% pass
- [ ] 19.3 Run all snapshot tests and verify 100% pass
- [ ] 19.4 Run all UI tests and verify 100% pass
- [ ] 19.5 Achieve >80% code coverage for ViewModels
- [ ] 19.6 Achieve >60% overall code coverage
- [ ] 19.7 Manually test app to ensure no regressions
- [ ] 19.8 Test MCP Servers create, edit, duplicate, delete flows manually
- [ ] 19.9 Verify app behavior identical before and after refactoring
- [x] 19.10 Review test execution time (< 2 minutes for all tests)

## 20. Optional: CI Integration

- [ ] 20.1 Add test execution step to CI pipeline (if exists)
- [ ] 20.2 Configure CI to run unit tests first, then UI tests
- [ ] 20.3 Configure CI to upload snapshot diffs on failure
- [ ] 20.4 Configure CI to export code coverage reports
- [ ] 20.5 Add CI badge to README showing test status
- [ ] 20.6 Verify CI runs tests on every pull request
