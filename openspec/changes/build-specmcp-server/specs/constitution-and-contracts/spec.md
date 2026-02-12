## ADDED Requirements

### Requirement: Constitution management
The system SHALL support creating and managing Constitution entities that define project-wide non-negotiable rules. A Constitution includes principles, guardrails, testing requirements, security requirements, and pattern mandates/prohibitions.

#### Scenario: Create constitution
- **WHEN** a Constitution entity is created with guardrails ["all-endpoints-authenticated", "no-orm", "repository-pattern-required"]
- **THEN** the Constitution entity is stored in the graph with all properties and can be retrieved

#### Scenario: Link constitution to change
- **WHEN** a Change is created or updated with a `governed_by` relationship to a Constitution
- **THEN** the Change is associated with the Constitution's rules, enabling validation checks

### Requirement: Constitution enforcement
The system SHALL validate that Changes comply with their governing Constitution. When a Change has a `governed_by` relationship to a Constitution, the system SHALL check that required patterns are used and forbidden patterns are absent.

#### Scenario: Validate required patterns
- **WHEN** a Change is governed by a Constitution that requires_pattern "repository-pattern" and the Change's tasks implement database access
- **THEN** the system can check whether "repository-pattern" is applied to the relevant entities

#### Scenario: Detect forbidden patterns
- **WHEN** a Change is governed by a Constitution that forbids_pattern "direct-dom-manipulation" and a component in the Change uses that pattern
- **THEN** the system reports a constitution violation

### Requirement: API contract management
The system SHALL support creating and managing APIContract entities linked to Specs via `has_contract` relationships. APIContracts represent machine-readable API definitions in formats: openapi, asyncapi, typespec, smithy, graphql, grpc.

#### Scenario: Create API contract for a spec
- **WHEN** `spec_artifact` is called with artifact_type "api_contract" and content including format "openapi", version "3.1.0", and file_path "api/openapi.yaml"
- **THEN** an APIContract entity is created and linked to the Spec via `has_contract`

#### Scenario: Track contract validation status
- **WHEN** an APIContract's `validation_status` is updated to "invalid"
- **THEN** the system can report which Spec has an invalid contract, flagging it for attention

### Requirement: Contract implementation tracking
The system SHALL track which Contexts and Actions implement an APIContract via `implements_contract` relationships, enabling verification that all contract endpoints are implemented.

#### Scenario: Link context to contract
- **WHEN** a Context implements endpoints defined in an APIContract
- **THEN** `implements_contract` relationships are created between the Context/Actions and the APIContract

#### Scenario: Check contract coverage
- **WHEN** an APIContract defines 10 endpoints and 7 have `implements_contract` relationships
- **THEN** the system can report 70% contract coverage with the 3 unimplemented endpoints listed

### Requirement: TestCase management
The system SHALL support creating TestCase entities linked to Scenarios via `tested_by` relationships. TestCases reference executable test files with framework, status, and coverage information.

#### Scenario: Create test case for scenario
- **WHEN** a TestCase is created with test_file "tests/auth/login.test.ts", test_framework "vitest", and linked to Scenario "valid-credentials"
- **THEN** a TestCase entity is created and a `tested_by` relationship links the Scenario to the TestCase

#### Scenario: Track test status
- **WHEN** a TestCase's status is updated from "untested" to "passing" with last_run_at timestamp
- **THEN** the system can report which Scenarios have passing tests and which remain untested

### Requirement: Ownership tracking
The system SHALL support `owned_by` relationships from Specs, Contexts, UIComponents, and Actions to CodingAgent entities, establishing who is responsible for maintaining each entity.

#### Scenario: Assign ownership
- **WHEN** an `owned_by` relationship is created from Spec "auth" to CodingAgent "alice"
- **THEN** alice is the designated owner of the auth spec and can be queried as such

#### Scenario: Query ownership
- **WHEN** a query asks "what does alice own?"
- **THEN** the system returns all entities with `owned_by` relationships pointing to alice's CodingAgent entity
