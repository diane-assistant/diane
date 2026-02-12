# Spec: snapshot-testing

## ADDED Requirements

### Requirement: SnapshotTesting Framework Integration

Add the SnapshotTesting Swift package (~1.12.0) to the project and configure it for SwiftUI view snapshot testing. The framework should be integrated into the test target and ready for use.

#### Scenario: Package dependency added via SPM

**WHEN** the project is opened in Xcode  
**THEN** Package.swift or project settings must include SnapshotTesting ~1.12.0  
**AND** the package must be linked to DianeMenuTests target  
**AND** tests can import SnapshotTesting

#### Scenario: Snapshot directory configured

**WHEN** snapshot tests run  
**THEN** snapshots must be saved to `DianeMenuTests/__Snapshots__/`  
**AND** the directory structure must match test class names  
**AND** snapshots must be committed to version control

#### Scenario: CI and local testing produce consistent snapshots

**WHEN** tests run on different machines  
**THEN** snapshot comparisons must account for font rendering differences  
**AND** tests must use a consistent window size  
**AND** tests must use a consistent color scheme (light mode default)

### Requirement: MCP Servers View Snapshot Tests

Create snapshot tests for the MCPServersView in various states. Tests should capture the view's appearance with different data scenarios and verify visual consistency across code changes.

#### Scenario: Empty state snapshot

**WHEN** MCPServersView is rendered with no servers  
**THEN** a snapshot must be captured showing the empty state  
**AND** the empty state message must be visible  
**AND** the "Add Server" button must be visible

#### Scenario: Server list snapshot

**WHEN** MCPServersView is rendered with 3 sample servers  
**THEN** a snapshot must be captured showing all servers  
**AND** server names, commands, and status must be visible  
**AND** the list layout must be consistent

#### Scenario: Selected server snapshot

**WHEN** MCPServersView is rendered with a server selected  
**THEN** a snapshot must be captured showing the selection highlight  
**AND** the detail view or edit panel must be visible  
**AND** the visual hierarchy must be clear

#### Scenario: Error state snapshot

**WHEN** MCPServersView is rendered with an error message  
**THEN** a snapshot must be captured showing the error UI  
**AND** the error message must be readable  
**AND** any recovery actions must be visible

### Requirement: MCP Server Form Snapshot Tests

Create snapshot tests for the MCP server creation/edit form in various states. Tests should capture form validation, field states, and button enabled/disabled states.

#### Scenario: Empty form snapshot

**WHEN** the server form is rendered with no data  
**THEN** a snapshot must be captured showing the initial state  
**AND** all fields must be empty  
**AND** the save button must be disabled

#### Scenario: Valid form snapshot

**WHEN** the server form is rendered with valid data  
**THEN** a snapshot must be captured showing the filled form  
**AND** all fields must contain the test data  
**AND** the save button must be enabled

#### Scenario: Form validation error snapshot

**WHEN** the server form is rendered with validation errors  
**THEN** a snapshot must be captured showing error indicators  
**AND** error messages must be visible near the problematic fields  
**AND** the visual styling must clearly indicate errors

#### Scenario: Form in loading state snapshot

**WHEN** the server form is rendered during save operation  
**THEN** a snapshot must be captured showing loading indicators  
**AND** form fields must appear disabled  
**AND** the save button must show a loading spinner or be disabled

### Requirement: Dark Mode Snapshot Tests

Create parallel snapshot tests for both light and dark appearance modes. Tests should verify that all UI elements are visible and properly styled in both modes.

#### Scenario: Light mode baseline

**WHEN** snapshot tests run in light mode  
**THEN** snapshots must be captured with light appearance  
**AND** text must be legible on light backgrounds  
**AND** colors must follow the light mode palette

#### Scenario: Dark mode comparison

**WHEN** snapshot tests run in dark mode  
**THEN** snapshots must be captured with dark appearance  
**AND** text must be legible on dark backgrounds  
**AND** colors must follow the dark mode palette  
**AND** both light and dark snapshots must exist for each component

#### Scenario: Dynamic color usage verified

**WHEN** views are rendered in both light and dark mode  
**THEN** semantic colors (primary, secondary, background) must adapt correctly  
**AND** custom colors must have appropriate light/dark variants  
**AND** no hardcoded colors should cause visibility issues

### Requirement: Responsive Layout Snapshot Tests

Create snapshot tests that verify layout correctness at different window sizes. Tests should ensure the UI adapts properly to various display dimensions.

#### Scenario: Minimum window size snapshot

**WHEN** MCPServersView is rendered at minimum supported size (e.g., 800x600)  
**THEN** a snapshot must be captured showing the compact layout  
**AND** all essential UI elements must be visible  
**AND** text must not be truncated or overlap

#### Scenario: Standard window size snapshot

**WHEN** MCPServersView is rendered at standard size (e.g., 1024x768)  
**THEN** a snapshot must be captured showing the default layout  
**AND** spacing and proportions must be appropriate  
**AND** this should serve as the baseline for visual regression

#### Scenario: Large window size snapshot

**WHEN** MCPServersView is rendered at large size (e.g., 1920x1080)  
**THEN** a snapshot must be captured showing the expanded layout  
**AND** content must not appear stretched or poorly distributed  
**AND** increased whitespace must be handled gracefully

### Requirement: Snapshot Test Failure Workflow

Establish clear guidelines and tooling for handling snapshot test failures. Developers should be able to easily review differences and update snapshots when intentional changes are made.

#### Scenario: Failure generates diff artifacts

**WHEN** a snapshot test fails  
**THEN** a diff image must be generated showing the differences  
**AND** the diff must highlight changed pixels  
**AND** both old and new snapshots must be available for review

#### Scenario: Developers can record new snapshots

**WHEN** intentional UI changes are made  
**THEN** developers must be able to re-record snapshots with a command or flag  
**AND** the new snapshots must replace the old ones  
**AND** the change must be visible in version control diff

#### Scenario: Unintentional changes are caught

**WHEN** code changes cause unintended visual regressions  
**THEN** snapshot tests must fail in CI  
**AND** the failure must block the PR or build  
**AND** the diff must clearly show what changed visually

### Requirement: Snapshot Test Performance

Ensure snapshot tests run efficiently and don't significantly slow down the test suite. Tests should balance coverage with execution speed.

#### Scenario: Snapshot generation is reasonably fast

**WHEN** snapshot tests run locally  
**THEN** each snapshot should generate in under 2 seconds  
**AND** the full snapshot suite should complete in under 30 seconds  
**AND** snapshots should only be regenerated when necessary

#### Scenario: Snapshots are excluded from git LFS

**WHEN** snapshots are committed to the repository  
**THEN** they must be stored as regular files (not git LFS)  
**AND** file sizes must be reasonable (under 500KB each)  
**AND** the total snapshot directory size must be monitored

#### Scenario: Parallel test execution supported

**WHEN** tests run in parallel  
**THEN** snapshot tests must not conflict with each other  
**AND** each test must use unique snapshot file names  
**AND** parallel execution should not cause flaky failures
