## ADDED Requirements

### Requirement: Create new change
The system SHALL provide a `spec_new` tool that creates a new Change entity with its Proposal. The tool accepts a change name, intent (why), scope (what changes), and impact, and creates both the Change and linked Proposal entities with a `has_proposal` relationship.

#### Scenario: Create change with proposal
- **WHEN** `spec_new` is called with name "add-user-permissions", intent "Users need role-based access control", scope "Add RBAC to user management", and impact "Auth module, API middleware"
- **THEN** a Change entity with status "active" is created, a Proposal entity is created with the provided fields, and they are linked via `has_proposal` relationship

#### Scenario: Duplicate change name
- **WHEN** `spec_new` is called with a name that already exists as an active Change
- **THEN** the tool returns an error indicating the change name is already in use

### Requirement: Add artifacts to change
The system SHALL provide a `spec_artifact` tool that adds any artifact type to an existing change: Specs (with Requirements and Scenarios), Designs, Tasks, Actors, Patterns, TestCases, APIContracts, Contexts, UIComponents, and Actions. The tool SHALL automatically create the appropriate relationship between the artifact and its parent.

#### Scenario: Add spec with requirements and scenarios
- **WHEN** `spec_artifact` is called with change_id, artifact_type "spec", and content containing a spec name, requirements, and scenarios
- **THEN** a Spec entity is created, Requirement entities are created for each requirement, Scenario entities are created for each scenario, and all are linked via `has_spec`, `has_requirement`, and `has_scenario` relationships

#### Scenario: Add design to change
- **WHEN** `spec_artifact` is called with artifact_type "design" and content containing approach, decisions, and file_changes
- **THEN** a Design entity is created and linked to the Change via `has_design`

#### Scenario: Prevent duplicate one-to-one artifacts
- **WHEN** `spec_artifact` is called to add a Proposal to a Change that already has a Proposal
- **THEN** the tool returns an error indicating the Change already has a Proposal (1:1 relationship)

### Requirement: Archive completed change
The system SHALL provide a `spec_archive` tool that transitions a Change's status from "active" to "archived". Archiving SHALL verify that all Tasks in the Change are completed before allowing the transition.

#### Scenario: Archive change with all tasks completed
- **WHEN** `spec_archive` is called on a Change whose Tasks are all in "completed" status
- **THEN** the Change status is updated to "archived"

#### Scenario: Reject archive with incomplete tasks
- **WHEN** `spec_archive` is called on a Change that has Tasks in "pending" or "in_progress" status
- **THEN** the tool returns an error listing the incomplete tasks

### Requirement: Automatic relationship creation
When artifacts are added, the system SHALL automatically create all implied relationships. For example, adding a Scenario to a Requirement creates `has_scenario`; adding a ScenarioStep that references a Context creates `occurs_in`.

#### Scenario: Scenario step creates context relationship
- **WHEN** a ScenarioStep is added with a reference to Context "user-management-screen"
- **THEN** an `occurs_in` relationship is automatically created between the ScenarioStep and the referenced Context

### Requirement: Workflow state validation
The system SHALL validate state transitions: a Change MUST have a Proposal before Specs can be added, Specs MUST exist before Design, and Design MUST exist before Tasks can be generated.

#### Scenario: Add spec before proposal exists
- **WHEN** `spec_artifact` is called to add a Spec to a Change that has no Proposal
- **THEN** the tool returns a validation error: "Change must have a Proposal before adding Specs"

#### Scenario: Valid workflow progression
- **WHEN** artifacts are added in order: Proposal → Spec → Design → Tasks
- **THEN** all artifacts are created successfully with no validation errors
