## ADDED Requirements

### Requirement: Core pattern library
The system SHALL ship with a seed set of core patterns covering common implementation patterns across all 8 pattern types (ui, api, data, state, error, auth, navigation, layout). Core patterns have scope "core" and serve as a baseline for pattern suggestions.

#### Scenario: Seed core patterns
- **WHEN** the template pack is initialized for a project
- **THEN** a set of core Pattern entities (e.g., "optimistic-update", "retry-with-backoff", "skeleton-loading", "rbac") are created with scope "core", each with description, example_code, and usage_guidance

### Requirement: Pattern suggestion
The system SHALL provide a `spec_suggest_patterns` tool that, given an entity type and name, analyzes the entity's properties, relationships, and context to suggest applicable patterns with a confidence score.

#### Scenario: Suggest patterns for a context
- **WHEN** `spec_suggest_patterns` is called for Context "user-management-screen" which has a data table and CRUD actions
- **THEN** the response suggests patterns like "master-detail" (layout), "optimistic-update" (data), and "rbac" (auth) with confidence scores

#### Scenario: No patterns suggested
- **WHEN** `spec_suggest_patterns` is called for a simple entity with no detectable pattern affinities
- **THEN** the response returns an empty suggestions list with an explanation

### Requirement: Apply pattern to entity
The system SHALL provide a `spec_apply_pattern` tool that creates a `uses_pattern` relationship between a Pattern and an entity (Context, UIComponent, Action, or ScenarioStep).

#### Scenario: Apply pattern creates relationship
- **WHEN** `spec_apply_pattern` is called with pattern "optimistic-update" and entity UIComponent "user-list"
- **THEN** a `uses_pattern` relationship is created from the UIComponent to the Pattern

#### Scenario: Reject invalid entity type for pattern
- **WHEN** `spec_apply_pattern` is called with an entity type that does not support `uses_pattern` (e.g., Proposal)
- **THEN** the tool returns an error indicating that entity type cannot have patterns applied

### Requirement: Pattern confirmation workflow
Auto-detected patterns (from sync or suggestion) SHALL be marked as "suggested" and require explicit confirmation before being applied. The system SHALL provide a way to confirm or reject suggested patterns.

#### Scenario: Confirm suggested pattern
- **WHEN** a pattern is suggested during sync and the user confirms it
- **THEN** the `uses_pattern` relationship is created with a property indicating it was confirmed

#### Scenario: Reject suggested pattern
- **WHEN** a pattern is suggested during sync and the user rejects it
- **THEN** no `uses_pattern` relationship is created and the suggestion is recorded as rejected to avoid re-suggesting

### Requirement: Project-specific patterns
The system SHALL support creating project-specific patterns (scope "project") that can extend core patterns via the `extends_pattern` relationship.

#### Scenario: Create project pattern extending core
- **WHEN** a project-specific pattern "app-optimistic-update" is created with `extends_pattern` referencing core pattern "optimistic-update"
- **THEN** the project pattern is created with scope "project" and linked to the core pattern via `extends_pattern`
