## ADDED Requirements

### Requirement: Feedback text entry
The controls panel SHALL include a "Feedback" section with a multi-line text editor where the user can write a comment about the currently selected catalog item.

#### Scenario: Feedback section visible when item selected
- **WHEN** a CatalogItem is selected in the sidebar
- **THEN** a "Feedback" GroupBox appears in the controls panel below the existing sections, containing a multi-line text editor and a "Create Issue" button

#### Scenario: Feedback text is independent per catalog item
- **WHEN** the user writes feedback for AgentsView, then selects MCPServersView, then returns to AgentsView
- **THEN** the feedback text for AgentsView is preserved

#### Scenario: Feedback text clears after successful issue creation
- **WHEN** an issue is successfully created
- **THEN** the feedback text for that catalog item is cleared

### Requirement: Selector generation
The system SHALL generate a human-readable selector string from the current catalog state to identify which component and state the feedback refers to.

#### Scenario: Screen view with preset
- **WHEN** the user is viewing AgentsView with the "Loaded" preset selected
- **THEN** the selector SHALL be `AgentsView [Loaded]`

#### Scenario: Screen view with no preset
- **WHEN** the user is viewing SettingsView which has no presets
- **THEN** the selector SHALL be `SettingsView`

#### Scenario: Reusable component
- **WHEN** the user is viewing the InfoRow component
- **THEN** the selector SHALL be `InfoRow`

### Requirement: GitHub issue creation via gh CLI
The system SHALL create a GitHub issue by shelling out to the `gh` CLI tool with a structured issue body.

#### Scenario: Successful issue creation
- **WHEN** the user writes feedback and clicks "Create Issue"
- **THEN** the system executes `gh issue create` with a title derived from the selector and a body containing the selector, preset state, component category, source file hint, and user comment
- **AND** the issue is labeled with `catalog-feedback`

#### Scenario: Issue body structure
- **WHEN** an issue is created for MCPServersView with preset "Error" and comment "The error message is truncated"
- **THEN** the issue body SHALL contain a "Component" field with `MCPServersView`, a "State" field with `Error`, a "Category" field with `Screen Views`, and a "Comment" section with the user's text

#### Scenario: Empty feedback rejected
- **WHEN** the user clicks "Create Issue" with an empty or whitespace-only comment
- **THEN** the button SHALL be disabled and no issue is created

#### Scenario: gh CLI not available
- **WHEN** the `gh` CLI is not found or not authenticated
- **THEN** the system SHALL display an error message inline in the feedback section

#### Scenario: Issue creation failure
- **WHEN** `gh issue create` exits with a non-zero status
- **THEN** the system SHALL display the error output inline in the feedback section

### Requirement: Issue creation feedback
The system SHALL provide visual feedback on the result of issue creation.

#### Scenario: Success feedback
- **WHEN** an issue is successfully created
- **THEN** the system SHALL display a brief success message with the issue URL, and the message SHALL dismiss after a few seconds or on next interaction

#### Scenario: In-progress feedback
- **WHEN** the "Create Issue" button is clicked and `gh` is running
- **THEN** the button SHALL show a loading/disabled state to prevent duplicate submissions

### Requirement: Repository configuration
The system SHALL target a configurable GitHub repository for issue creation, defaulting to the repository detected by `gh` in the project directory.

#### Scenario: Default repository
- **WHEN** no explicit repository is configured
- **THEN** the system SHALL use `gh`'s default repository resolution (current directory's git remote)

#### Scenario: Custom repository override
- **WHEN** the user sets a custom repository in the feedback section
- **THEN** all subsequent issues SHALL be created in that repository using `gh issue create -R <repo>`
