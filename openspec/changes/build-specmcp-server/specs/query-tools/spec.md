## ADDED Requirements

### Requirement: Get context details
The system SHALL provide a `spec_get_context` tool that retrieves a Context entity and its relationships — including components used, actions available, nested contexts, and patterns applied.

#### Scenario: Retrieve context with all relationships
- **WHEN** `spec_get_context` is called with name "user-management-screen"
- **THEN** the response includes the Context properties plus lists of related UIComponents (via uses_component), Actions (via available_in), nested Contexts (via nested_in), and Patterns (via uses_pattern)

#### Scenario: Context not found
- **WHEN** `spec_get_context` is called with a name that does not exist in the graph
- **THEN** the response includes an error message indicating the context was not found

### Requirement: Get component details
The system SHALL provide a `spec_get_component` tool that retrieves a UIComponent entity and its relationships — including contexts that use it, child components (composed_of), parent components, patterns applied, and ownership.

#### Scenario: Retrieve component with composition hierarchy
- **WHEN** `spec_get_component` is called with name "user-list"
- **THEN** the response includes the component properties plus child components (composed_of), parent components, contexts that use it (uses_component), and owning CodingAgent (owned_by)

### Requirement: Get action details
The system SHALL provide a `spec_get_action` tool that retrieves an Action entity with its contexts (available_in), navigation targets (navigates_to), and related patterns.

#### Scenario: Retrieve action with availability
- **WHEN** `spec_get_action` is called with name "update-user-role"
- **THEN** the response includes the action properties plus all contexts where it is available and any navigation targets

### Requirement: Get scenario details
The system SHALL provide a `spec_get_scenario` tool that retrieves a Scenario entity with its steps, executing actor, parent requirement, variants, and linked test cases.

#### Scenario: Retrieve scenario with full context
- **WHEN** `spec_get_scenario` is called with name "valid-credentials"
- **THEN** the response includes scenario properties (given/when/then), ScenarioSteps (has_step), executing Actor (executed_by), parent Requirement, and TestCases (tested_by)

### Requirement: Get patterns
The system SHALL provide a `spec_get_patterns` tool that lists all Pattern entities, with optional filtering by type (ui, api, data, state, error, auth, navigation, layout) and scope (core, project).

#### Scenario: List patterns by type
- **WHEN** `spec_get_patterns` is called with type "ui"
- **THEN** the response includes only Pattern entities where type equals "ui"

#### Scenario: List all patterns
- **WHEN** `spec_get_patterns` is called with no filters
- **THEN** the response includes all Pattern entities across all types and scopes

### Requirement: Impact analysis
The system SHALL provide a `spec_impact_analysis` tool that, given an entity type and name, traverses the graph to find all affected entities — scenarios that reference it, tasks that implement it, components that depend on it, and changes that include it.

#### Scenario: Component impact analysis
- **WHEN** `spec_impact_analysis` is called for UIComponent "user-list"
- **THEN** the response includes all Contexts using the component, Scenarios whose steps reference those Contexts, Tasks that implement the component, and any Changes containing those Tasks

#### Scenario: Requirement impact analysis
- **WHEN** `spec_impact_analysis` is called for Requirement "session-expiration"
- **THEN** the response includes the parent Spec, all Scenarios under the requirement, Tasks implementing it, and related TestCases

### Requirement: Query result formatting
All query tools SHALL return results in a consistent structured format with the entity's properties, a list of related entities grouped by relationship type, and metadata (entity ID, type, version).

#### Scenario: Consistent response structure
- **WHEN** any query tool returns a result
- **THEN** the response contains `entity` (properties), `relationships` (grouped by type), and `metadata` (id, type, version) fields
