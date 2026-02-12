# Spec: test-infrastructure

## ADDED Requirements

### Requirement: DianeMenuTests Unit Test Target

Create a new test target named `DianeMenuTests` for unit and integration tests. The target should be properly configured with dependencies and test infrastructure.

#### Scenario: Test target exists in Xcode project

**WHEN** the project is opened in Xcode  
**THEN** a DianeMenuTests target must exist  
**AND** the target type must be "Bundle" with product type "Unit Test Bundle"  
**AND** the target must be associated with the DianeMenu app target

#### Scenario: Test target has access to app code

**WHEN** tests in DianeMenuTests need to import app code  
**THEN** the DianeMenu target must be available via @testable import  
**AND** all internal types and methods must be accessible in tests  
**AND** tests can instantiate ViewModels, models, and services

#### Scenario: Test target includes SPM dependencies

**WHEN** tests need to use testing frameworks  
**THEN** ViewInspector must be linked to DianeMenuTests  
**AND** SnapshotTesting must be linked to DianeMenuTests  
**AND** XCTest must be available by default  
**AND** all dependencies must resolve correctly

### Requirement: DianeMenuUITests UI Test Target

Create a new test target named `DianeMenuUITests` for end-to-end UI automation tests. The target should be configured for XCUITest and have access to launch the app.

#### Scenario: UI test target exists in Xcode project

**WHEN** the project is opened in Xcode  
**THEN** a DianeMenuUITests target must exist  
**AND** the target type must be "Bundle" with product type "UI Test Bundle"  
**AND** the target must be associated with the DianeMenu app target

#### Scenario: UI test target can launch the app

**WHEN** UI tests run  
**THEN** tests must be able to launch DianeMenu.app  
**AND** the app must launch in a clean test environment  
**AND** XCUIApplication must be available for automation

#### Scenario: UI tests use test environment configuration

**WHEN** the app is launched from UI tests  
**THEN** tests can pass launch arguments to configure test mode  
**AND** tests can set environment variables for feature flags  
**AND** the app can detect it's running in a UI test context

### Requirement: Test Fixtures and Helpers

Create reusable test fixtures and helper utilities for common test scenarios. Helpers should reduce boilerplate and make tests more readable.

#### Scenario: Mock MCP server data fixtures

**WHEN** tests need sample MCP server data  
**THEN** fixtures must provide MCPServerConfig instances with realistic data  
**AND** fixtures must cover various scenarios (valid, invalid, edge cases)  
**AND** fixtures must be easily customizable for specific test needs

#### Scenario: Test helper for async operations

**WHEN** tests need to wait for async operations  
**THEN** helpers must provide utilities for awaiting expectations  
**AND** helpers must support timeout configuration  
**AND** helpers must provide clear error messages on timeout

#### Scenario: Test helper for SwiftUI view rendering

**WHEN** tests need to render SwiftUI views  
**THEN** helpers must provide utilities for creating test host windows  
**AND** helpers must support injecting mock dependencies  
**AND** helpers must handle view lifecycle properly

### Requirement: Test Configuration and Schemes

Configure Xcode schemes for running tests with appropriate settings. Schemes should support running all tests, specific test classes, or individual tests.

#### Scenario: Test scheme runs unit tests

**WHEN** the "Test" action is executed on DianeMenu scheme  
**THEN** DianeMenuTests must be enabled in the scheme  
**AND** tests must run with code coverage enabled  
**AND** test results must be reported in Xcode's test navigator

#### Scenario: Test scheme runs UI tests separately

**WHEN** developers want to run only UI tests  
**THEN** a separate scheme or scheme option must exist for DianeMenuUITests  
**AND** UI tests must not run during quick unit test runs  
**AND** UI tests must be clearly distinguishable in test output

#### Scenario: Test schemes support parallel execution

**WHEN** tests run in Xcode or CI  
**THEN** unit tests must be able to run in parallel  
**AND** parallelization must be configurable in the scheme  
**AND** tests must be written to avoid shared state issues

### Requirement: Test Data Isolation

Ensure tests run in isolation without affecting each other or the developer's local environment. Tests should not persist data or share state.

#### Scenario: Mock DianeClient never hits real daemon

**WHEN** unit or integration tests run  
**THEN** tests must use MockDianeClient instead of the real DianeClient  
**AND** no actual Unix socket connections must be made  
**AND** no real data must be modified in ~/.diane/

#### Scenario: UI tests use isolated app container

**WHEN** UI tests launch the app  
**THEN** the app must use a separate container directory  
**AND** test data must not persist between test runs  
**AND** tests must not interfere with the developer's personal DianeMenu data

#### Scenario: Test cleanup happens automatically

**WHEN** tests complete (pass or fail)  
**THEN** any temporary files must be cleaned up  
**AND** any test state must be reset  
**AND** subsequent tests must start with a clean slate

### Requirement: Continuous Integration Test Execution

Configure test execution for CI environments (GitHub Actions, etc.). Tests should run reliably in headless environments and report results correctly.

#### Scenario: Unit tests run in CI

**WHEN** CI runs the test suite  
**THEN** unit tests must execute via xcodebuild or swift test  
**AND** test failures must cause the CI build to fail  
**AND** test results must be parseable for CI reporting

#### Scenario: Snapshot tests behave consistently in CI

**WHEN** snapshot tests run in CI  
**THEN** snapshots must match those recorded on developer machines  
**AND** rendering differences due to OS versions must be minimized  
**AND** snapshot failures must provide clear diff artifacts

#### Scenario: UI tests run in CI with virtual display

**WHEN** UI tests run in headless CI  
**THEN** tests must use a virtual framebuffer or headless mode  
**AND** the app must launch successfully without a display  
**AND** UI test results must be reliable and not flaky

### Requirement: Code Coverage Configuration

Enable and configure code coverage reporting for the test suite. Coverage reports should help identify untested code paths.

#### Scenario: Code coverage enabled in scheme

**WHEN** tests run in Xcode  
**THEN** code coverage gathering must be enabled  
**AND** coverage data must be collected for DianeMenu target  
**AND** coverage must be viewable in Xcode's coverage navigator

#### Scenario: Coverage reports exported for CI

**WHEN** tests run in CI  
**THEN** coverage data must be exported in a standard format (e.g., lcov)  
**AND** coverage reports must be uploadable to services like Codecov  
**AND** coverage trends must be trackable over time

#### Scenario: Coverage thresholds enforced

**WHEN** evaluating test coverage  
**THEN** minimum coverage thresholds should be documented  
**AND** CI should warn when coverage decreases  
**AND** critical paths (e.g., DianeClient methods) must have high coverage

### Requirement: Test Documentation and Guidelines

Provide clear documentation for writing, running, and maintaining tests. Guidelines should help developers write consistent, effective tests.

#### Scenario: Test README explains test structure

**WHEN** developers want to understand the test suite  
**THEN** a DianeMenuTests/README.md must exist  
**AND** the README must explain unit vs integration vs UI tests  
**AND** the README must provide examples of each test type

#### Scenario: Test naming conventions documented

**WHEN** developers write new tests  
**THEN** test naming conventions must be documented  
**AND** test class names must follow a consistent pattern (e.g., `*ViewModelTests`, `*IntegrationTests`)  
**AND** test method names must be descriptive and follow a consistent format

#### Scenario: Running tests locally documented

**WHEN** developers want to run tests  
**THEN** documentation must explain how to run tests via Xcode  
**AND** documentation must explain how to run tests via command line  
**AND** documentation must explain how to run specific test subsets
