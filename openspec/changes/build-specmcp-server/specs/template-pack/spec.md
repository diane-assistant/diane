## ADDED Requirements

### Requirement: Entity type definitions
The system SHALL define a template pack containing all 17 entity types with their properties, types, and descriptions. Entity types include: Actor, CodingAgent, Pattern, Constitution, Change, Proposal, Spec, Requirement, Scenario, ScenarioStep, Design, Task, TestCase, APIContract, Context, UIComponent, Action, and GraphSync.

#### Scenario: Template pack contains all entity types
- **WHEN** the template pack JSON is loaded into Emergent
- **THEN** all 17 entity types are available in the project's type registry with correct property schemas

#### Scenario: Entity properties use correct types
- **WHEN** an entity type definition is inspected
- **THEN** each property has a defined type (string, text, number, boolean, enum, string[], timestamp) matching the design specification

### Requirement: Relationship type definitions
The system SHALL define all relationship types (~30) with source types, target types, and optional properties. Relationship types include: inherits_from, uses_pattern, extends_pattern, has_proposal, has_spec, has_design, has_task, has_requirement, has_scenario, executed_by, has_step, variant_of, occurs_in, performs, composed_of, uses_component, nested_in, available_in, navigates_to, has_subtask, blocks, implements, assigned_to, governed_by, requires_pattern, forbids_pattern, tested_by, tests, has_contract, implements_contract, and owned_by.

#### Scenario: Bidirectional blocking relationships
- **WHEN** a "blocks" relationship is created from Task A to Task B
- **THEN** Emergent automatically creates a "blocked_by" reverse relationship from Task B to Task A

#### Scenario: Relationship source and target type constraints
- **WHEN** a relationship type is defined with source_types and target_types
- **THEN** only entities matching those types can participate in that relationship

### Requirement: Template pack registration
The system SHALL provide a mechanism to register the template pack with an Emergent project via the SDK's `TemplatePacks.CreatePack()` and `TemplatePacks.AssignPack()` methods.

#### Scenario: Assign template pack to project
- **WHEN** the template pack is registered with an Emergent project
- **THEN** calling GetCompiledTypes returns all entity and relationship types defined in the pack

#### Scenario: Template pack idempotency
- **WHEN** the template pack registration is run multiple times against the same project
- **THEN** the result is identical — no duplicate types are created

### Requirement: Tag property convention
All entity types SHALL include a `tags` property of type `string[]` supporting the namespaced tagging convention: `domain:<value>`, `platform:<value>`, `lifecycle:<value>`.

#### Scenario: Entity created with tags
- **WHEN** an entity is created with tags `["domain:auth", "platform:web", "lifecycle:stable"]`
- **THEN** the entity's tags property contains all three tags and they can be used for filtering
