# Spec: integration-testing

## ADDED Requirements

### Requirement: DianeClient Protocol Abstraction

Extract a `DianeClientProtocol` from the concrete `DianeClient` class to enable dependency injection and mocking in tests. The protocol should define all public methods that interact with the Diane daemon via Unix socket.

#### Scenario: Protocol defines all MCP server operations

**WHEN** the DianeClientProtocol is defined  
**THEN** it must include methods for:
- `getMCPServerConfigs() async throws -> [MCPServerConfig]`
- `createMCPServerConfig(_:) async throws -> MCPServerConfig`
- `updateMCPServerConfig(_:) async throws -> MCPServerConfig`
- `deleteMCPServerConfig(_:) async throws`
- Any other daemon communication methods

#### Scenario: DianeClient conforms to protocol

**WHEN** the protocol is extracted  
**THEN** the existing DianeClient class must conform to DianeClientProtocol  
**AND** no existing functionality should be broken  
**AND** the app should compile and run without changes to call sites

### Requirement: Mock DianeClient Implementation

Create a `MockDianeClient` class that conforms to `DianeClientProtocol` for use in tests. The mock should allow tests to control responses, simulate errors, and verify method calls.

#### Scenario: Mock returns configurable responses

**WHEN** a test configures the mock with server configs  
**THEN** calls to `getMCPServerConfigs()` must return the configured data  
**AND** the test can simulate various server state scenarios

#### Scenario: Mock simulates network errors

**WHEN** a test configures the mock to throw an error  
**THEN** calls to any method must throw the configured error  
**AND** the test can verify error handling behavior

#### Scenario: Mock verifies method invocations

**WHEN** a test uses the mock  
**THEN** the mock must track which methods were called  
**AND** the mock must track method call counts  
**AND** the mock must track method parameters  
**AND** tests can assert on these invocations

### Requirement: Create MCP Server Integration Tests

Write integration tests for the create MCP server flow using the mock client. Tests should verify that the business logic correctly handles successful creation and error cases.

#### Scenario: Successful server creation

**WHEN** createMCPServerConfig is called with valid data  
**THEN** the mock should return a new MCPServerConfig  
**AND** the returned config should have a generated ID  
**AND** the returned config should contain the provided name, command, and arguments

#### Scenario: Creation fails with duplicate name

**WHEN** createMCPServerConfig is called with a name that already exists  
**THEN** the mock should throw an appropriate error  
**AND** the test should verify error handling behavior

#### Scenario: Creation fails with invalid command

**WHEN** createMCPServerConfig is called with an empty or invalid command  
**THEN** the mock should throw a validation error  
**AND** the test should verify the error message is appropriate

### Requirement: Update MCP Server Integration Tests

Write integration tests for the update MCP server flow using the mock client. Tests should verify that updates are applied correctly and that invalid updates are rejected.

#### Scenario: Successful server update

**WHEN** updateMCPServerConfig is called with valid changes  
**THEN** the mock should return the updated MCPServerConfig  
**AND** the updated config should reflect the new values  
**AND** unchanged fields should remain the same

#### Scenario: Update fails for non-existent server

**WHEN** updateMCPServerConfig is called with an ID that doesn't exist  
**THEN** the mock should throw a not-found error  
**AND** the test should verify error handling behavior

#### Scenario: Update fails with invalid data

**WHEN** updateMCPServerConfig is called with invalid data (empty name, invalid command)  
**THEN** the mock should throw a validation error  
**AND** the test should verify the error is handled appropriately

### Requirement: Delete MCP Server Integration Tests

Write integration tests for the delete MCP server flow using the mock client. Tests should verify that deletions work correctly and handle edge cases.

#### Scenario: Successful server deletion

**WHEN** deleteMCPServerConfig is called with a valid server ID  
**THEN** the mock should complete successfully without throwing  
**AND** subsequent calls to getMCPServerConfigs should not include the deleted server

#### Scenario: Delete fails for non-existent server

**WHEN** deleteMCPServerConfig is called with an ID that doesn't exist  
**THEN** the mock should throw a not-found error  
**AND** the test should verify error handling behavior

#### Scenario: Delete with active connections

**WHEN** deleteMCPServerConfig is called for a server with active connections  
**THEN** the mock should simulate the daemon's behavior (either allow or reject)  
**AND** the test should verify the appropriate user feedback

### Requirement: List and Filter MCP Servers Integration Tests

Write integration tests for retrieving and filtering MCP server configurations. Tests should verify that the mock correctly simulates various server states and filtering scenarios.

#### Scenario: Retrieve all servers

**WHEN** getMCPServerConfigs is called with no filters  
**THEN** the mock should return all configured servers  
**AND** the list should be properly deserialized into MCPServerConfig objects

#### Scenario: Handle empty server list

**WHEN** getMCPServerConfigs is called and no servers exist  
**THEN** the mock should return an empty array  
**AND** the UI should handle this gracefully

#### Scenario: Handle malformed response data

**WHEN** the mock is configured to return malformed JSON  
**THEN** calls to getMCPServerConfigs should throw a parsing error  
**AND** the test should verify error handling behavior

### Requirement: Async/Await Test Infrastructure

Ensure all integration tests properly handle Swift's async/await concurrency model. Tests should use XCTest's async test support and handle task cancellation appropriately.

#### Scenario: Tests use async test methods

**WHEN** integration tests call async DianeClient methods  
**THEN** the test methods must be marked with `async throws`  
**AND** the tests must use `await` for all async calls  
**AND** XCTest should properly manage the async context

#### Scenario: Tests handle timeout scenarios

**WHEN** a mock is configured to simulate a timeout  
**THEN** the test should verify that the timeout is handled correctly  
**AND** the test should use appropriate timeout values for test execution

#### Scenario: Tests handle cancellation

**WHEN** a test simulates task cancellation  
**THEN** the mock should properly handle cancellation  
**AND** the test should verify that cancellation propagates correctly
