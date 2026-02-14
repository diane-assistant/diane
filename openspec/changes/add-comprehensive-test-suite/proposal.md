## Why

Diane currently has no automated tests, making it difficult to confidently refactor code, catch regressions, or maintain quality as the application grows. A comprehensive test suite covering GUI components, business logic, and API interactions will increase reliability, enable faster iteration, and provide confidence when making changes to critical user flows like MCP server configuration.

## What Changes

- Add **DianeTests** target to Xcode project for unit and integration tests
- Add **DianeUITests** target to Xcode project for end-to-end UI automation
- Extract ViewModels from SwiftUI views to separate presentation logic from business logic (enables unit testing)
- Introduce protocol-based dependency injection for `DianeClient` and other services (enables mocking)
- Add testing dependencies: ViewInspector, SnapshotTesting via Swift Package Manager
- Create test fixtures, mock implementations, and helper utilities
- Establish testing patterns and examples for each test type (unit, UI, integration, snapshot)
- Document testing strategies and best practices in project documentation

## Capabilities

### New Capabilities

- `gui-unit-testing`: Unit test framework for SwiftUI view business logic using extracted ViewModels. Covers form validation, state management, and pure functions (e.g., duplicate name generation, filtering).

- `ui-automation-testing`: XCTest UI automation framework for testing end-to-end user flows. Covers critical paths like creating, editing, duplicating, and deleting MCP servers through the actual UI.

- `integration-testing`: Integration test framework for testing API client interactions with mock backend responses. Covers DianeClient methods, error handling, and data transformations without requiring a running server.

- `snapshot-testing`: Visual regression testing framework using image-based snapshots. Covers UI consistency across different states (empty, loaded, error) and screen sizes to catch unintended visual changes.

- `test-infrastructure`: Core testing infrastructure including Xcode test targets, mock implementations, test fixtures, helper utilities, and CI/CD integration setup.

### Modified Capabilities

None - this is net-new testing infrastructure with no changes to existing requirements.

## Impact

**Code Architecture:**
- SwiftUI views will be refactored to use ViewModels (e.g., `MCPServersViewModel`, `ServerFormViewModel`)
- `DianeClient` will implement `DianeClientProtocol` for dependency injection
- Business logic will be extracted from view code into testable units

**Xcode Project:**
- New test targets added: `DianeTests` and `DianeUITests`
- Swift Package Manager dependencies added for testing libraries

**Dependencies:**
- ViewInspector (~0.9.0) - for SwiftUI view inspection in tests
- SnapshotTesting (~1.12.0) - for visual regression testing

**Development Workflow:**
- Developers can run tests locally via Xcode (Cmd+U) or command line
- Tests should be run before committing changes to catch regressions
- CI pipeline can be configured to run tests automatically (future enhancement)

**No Breaking Changes:**
- Refactoring to ViewModels maintains existing UI/UX behavior
- No changes to public APIs or user-facing functionality
