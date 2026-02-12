## 1. Template Pack Definition

- [ ] 1.1 Research Emergent template pack JSON format by examining existing packs in the Emergent codebase
- [ ] 1.2 Create `templates/specmcp-pack.json` with Actor entity type (name, display_name, description, tags)
- [ ] 1.3 Add CodingAgent entity type (name, display_name, type, active, skills, specialization, instructions, velocity_points_per_hour, tags)
- [ ] 1.4 Add Pattern entity type (name, display_name, type, scope, description, example_code, usage_guidance, tags)
- [ ] 1.5 Add Constitution entity type (name, version, principles, guardrails, testing_requirements, security_requirements, patterns_required, patterns_forbidden, tags)
- [ ] 1.6 Add Change entity type (name, status, base_commit, tags)
- [ ] 1.7 Add Proposal entity type (intent, scope, impact, tags)
- [ ] 1.8 Add Spec entity type (name, domain, purpose, delta_type, tags)
- [ ] 1.9 Add Requirement entity type (name, description, strength, delta_type, tags)
- [ ] 1.10 Add Scenario entity type (name, title, given, when, then, and_also, tags)
- [ ] 1.11 Add ScenarioStep entity type (sequence, description, tags)
- [ ] 1.12 Add Design entity type (approach, decisions, data_flow, file_changes, tags)
- [ ] 1.13 Add Task entity type (number, description, task_type, status, complexity_points, started_at, completed_at, actual_hours, artifacts, verification_method, verification_notes, tags)
- [ ] 1.14 Add TestCase entity type (name, test_file, test_function, test_framework, status, last_run_at, coverage_percent, tags)
- [ ] 1.15 Add APIContract entity type (name, format, version, file_path, base_url, description, auto_generated, last_validated_at, validation_status, tags)
- [ ] 1.16 Add Context entity type (name, display_name, type, scope, platform, description, file_path, tags)
- [ ] 1.17 Add UIComponent entity type (name, display_name, type, file_path, description, tags)
- [ ] 1.18 Add Action entity type (name, display_label, type, description, handler_path, tags)
- [ ] 1.19 Add GraphSync entity type (last_synced_commit, last_synced_at, status, tags)

## 2. Template Pack Relationships

- [ ] 2.1 Define Actor relationships (inherits_from: Actor → Actor)
- [ ] 2.2 Define Pattern relationships (uses_pattern: Context/UIComponent/Action/ScenarioStep → Pattern, extends_pattern: Pattern → Pattern)
- [ ] 2.3 Define Change structure relationships (has_proposal, has_spec, has_design, has_task)
- [ ] 2.4 Define Spec structure relationships (has_requirement: Spec → Requirement, has_scenario: Requirement → Scenario)
- [ ] 2.5 Define Scenario relationships (executed_by: Scenario → Actor, has_step: Scenario → ScenarioStep, variant_of: Scenario → Scenario)
- [ ] 2.6 Define Step relationships (occurs_in: ScenarioStep → Context, performs: ScenarioStep → Action)
- [ ] 2.7 Define Context/Component relationships (composed_of, uses_component, nested_in)
- [ ] 2.8 Define Action relationships (available_in: Action → Context, navigates_to: Action → Context)
- [ ] 2.9 Define Task relationships (has_subtask, blocks, implements, assigned_to)
- [ ] 2.10 Define Constitution relationships (governed_by, requires_pattern, forbids_pattern)
- [ ] 2.11 Define Test relationships (tested_by: Scenario → TestCase, tests: TestCase → Scenario)
- [ ] 2.12 Define Contract relationships (has_contract, implements_contract)
- [ ] 2.13 Define Ownership relationships (owned_by: Spec/Context/UIComponent/Action → CodingAgent)

## 3. Template Pack Registration

- [ ] 3.1 Write seed script (`scripts/seed-pack.go` or Makefile target) that creates the template pack via `TemplatePacks.CreatePack()` and assigns it to the project via `TemplatePacks.AssignPack()`
- [ ] 3.2 Add `make seed` command to create/update the template pack in Emergent
- [ ] 3.3 Test template pack registration — verify all types appear in GetCompiledTypes
- [ ] 3.4 Test creating a sample entity of each type to verify property schemas work

## 4. MCP Server Scaffold

- [ ] 4.1 Initialize Go module at `server/specmcp/` with `go mod init`
- [ ] 4.2 Add Emergent SDK dependency (`go get github.com/emergent-company/emergent/apps/server-go/pkg/sdk@v0.7.0`)
- [ ] 4.3 Create `cmd/specmcp/main.go` — entrypoint that loads config, initializes SDK client, and starts MCP server
- [ ] 4.4 Implement `internal/config/config.go` — load from env vars and optional YAML file with precedence rules
- [ ] 4.5 Implement `internal/mcp/transport.go` — stdio JSON-RPC 2.0 reader/writer (stdin scanner, stdout encoder)
- [ ] 4.6 Implement `internal/mcp/handler.go` — dispatch initialize, tools/list, tools/call requests
- [ ] 4.7 Implement `internal/mcp/registry.go` — Tool interface and self-registration pattern
- [ ] 4.8 Implement `internal/mcp/types.go` — JSON-RPC request/response types, MCP message types
- [ ] 4.9 Add structured logging to stderr (JSON format, configurable level)
- [ ] 4.10 Create Makefile with `build`, `run`, `test`, `seed` targets
- [ ] 4.11 Verify the server starts, responds to `initialize`, and returns empty `tools/list`

## 5. Emergent Client Wrapper

- [ ] 5.1 Implement `internal/emergent/client.go` — initialize SDK client from config, expose typed domain methods
- [ ] 5.2 Implement entity struct types (`internal/emergent/types.go`) — Go structs for all 17 entity types with JSON/property mapping
- [ ] 5.3 Implement `internal/emergent/entities.go` — CreateEntity, GetEntity, ListEntities with type-safe property mapping
- [ ] 5.4 Implement `internal/emergent/relationships.go` — CreateRelationship, GetRelationships, TraverseFrom with typed wrappers
- [ ] 5.5 Implement `internal/emergent/queries.go` — ExpandGraph helpers, entity-with-relationships queries
- [ ] 5.6 Add error wrapping and Emergent-specific error handling

## 6. Query Tools

- [ ] 6.1 Implement `spec_get_context` tool — retrieve Context with uses_component, available_in, nested_in, uses_pattern relationships
- [ ] 6.2 Implement `spec_get_component` tool — retrieve UIComponent with composed_of, uses_component (reverse), owned_by, uses_pattern
- [ ] 6.3 Implement `spec_get_action` tool — retrieve Action with available_in, navigates_to, uses_pattern
- [ ] 6.4 Implement `spec_get_scenario` tool — retrieve Scenario with has_step, executed_by, parent Requirement, tested_by, variant_of
- [ ] 6.5 Implement `spec_get_patterns` tool — list patterns with optional type/scope filters
- [ ] 6.6 Implement `spec_impact_analysis` tool — multi-hop graph traversal to find all affected entities
- [ ] 6.7 Implement consistent response formatting (entity + relationships + metadata structure)
- [ ] 6.8 Write tests for all query tools using mock Emergent client

## 7. Workflow Tools

- [ ] 7.1 Implement `spec_new` tool — create Change + Proposal with has_proposal relationship
- [ ] 7.2 Implement `spec_artifact` tool — add Spec (with Requirements/Scenarios), Design, Tasks, and other entity types to a Change
- [ ] 7.3 Implement automatic relationship creation when adding artifacts (has_spec, has_requirement, has_scenario, etc.)
- [ ] 7.4 Implement workflow state validation (Proposal before Specs, Specs before Design, Design before Tasks)
- [ ] 7.5 Implement `spec_archive` tool — verify all tasks completed, transition Change status to archived
- [ ] 7.6 Implement duplicate prevention (1:1 relationships for Proposal and Design)
- [ ] 7.7 Write tests for workflow tools including validation edge cases

## 8. Task Management Tools

- [ ] 8.1 Implement `spec_generate_tasks` tool — two-pass task generation (entity pass, dependency pass) from specs
- [ ] 8.2 Implement dependency detection algorithm — walk uses_component, available_in, etc. to create blocks relationships
- [ ] 8.3 Implement `spec_get_available_tasks` tool — filter for pending + unblocked + unassigned tasks
- [ ] 8.4 Implement `spec_get_parallel_capacity` tool — count and list available tasks
- [ ] 8.5 Implement `spec_assign_task` tool — create assigned_to relationship, validate skill match, set in_progress status and started_at
- [ ] 8.5a Implement agent skill matching — for each available task, list suitable agents ranked by workload
- [ ] 8.6 Implement `spec_complete_task` tool — mark completed, record timestamps, calculate actual_hours, find newly unblocked tasks
- [ ] 8.7 Implement `spec_get_scenario_progress` tool — sum completed vs total complexity points
- [ ] 8.8 Implement `spec_get_critical_path` tool — longest dependency chain by complexity points (topological sort + DP)
- [ ] 8.9 Implement velocity tracking — update CodingAgent velocity_points_per_hour after task completion
- [ ] 8.10 Write tests for task management including dependency graph scenarios

## 9. Sync Tools

- [ ] 9.1 Implement `spec_sync_status` tool — compare GraphSync last_synced_commit against git HEAD
- [ ] 9.2 Implement git commit tracking — shell out to `git rev-parse HEAD` and `git diff --name-only`
- [ ] 9.3 Implement Analyzer interface and analyzer registry
- [ ] 9.4 Implement React/TypeScript analyzer — detect contexts (pages), components (exports), and actions (handlers)
- [ ] 9.5 Implement `spec_sync` tool — run analyzers, create/update entities, create relationships, update GraphSync
- [ ] 9.6 Implement pattern auto-detection during sync — analyze extracted entities for common patterns, create unconfirmed suggestions
- [ ] 9.7 Implement auto-tagging during sync — apply domain/platform/lifecycle tags based on file paths and component characteristics
- [ ] 9.8 Implement incremental sync — use `git diff` to limit analysis to changed files only
- [ ] 9.9 Handle deleted entities during incremental sync
- [ ] 9.8 Write tests for sync tools with sample React/TypeScript files

## 10. Pattern Tools

- [ ] 10.1 Create core pattern library seed data (JSON file with patterns across all 8 types)
- [ ] 10.2 Add core pattern seeding to `make seed` command
- [ ] 10.3 Implement `spec_suggest_patterns` tool — analyze entity properties and relationships to suggest applicable patterns
- [ ] 10.4 Implement `spec_apply_pattern` tool — create uses_pattern relationship with validation
- [ ] 10.5 Implement pattern confirmation workflow — suggested patterns require explicit confirmation
- [ ] 10.6 Support project-specific patterns with extends_pattern relationships
- [ ] 10.7 Write tests for pattern tools

## 11. Constitution and Contract Tools

- [ ] 11.1 Implement constitution CRUD via `spec_artifact` tool (create Constitution entities, governed_by relationships)
- [ ] 11.2 Implement constitution enforcement checks — validate required/forbidden patterns on a Change
- [ ] 11.3 Implement API contract CRUD via `spec_artifact` tool (create APIContract entities, has_contract relationships)
- [ ] 11.4 Implement contract coverage tracking — count implements_contract relationships vs contract endpoints
- [ ] 11.5 Implement TestCase CRUD via `spec_artifact` tool (create TestCase entities, tested_by relationships)
- [ ] 11.6 Implement ownership tracking — owned_by relationships and querying what an agent owns
- [ ] 11.7 Write tests for constitution, contract, and test case management

## 12. Integration and Polish

- [ ] 12.1 End-to-end test: create Change → Proposal → Spec → Design → generate Tasks → assign → complete → archive
- [ ] 12.2 End-to-end test: sync a sample codebase and verify entities/relationships
- [ ] 12.3 Verify all 20+ tools appear in tools/list with correct schemas
- [ ] 12.4 Add error handling for all tool edge cases (missing entities, invalid parameters, Emergent down)
- [ ] 12.5 Create MCP configuration example for Claude Desktop (`claude_desktop_config.json`)
- [ ] 12.6 Create MCP configuration example for OpenCode (`.opencode.json`)
- [ ] 12.7 Performance test with 100+ entities and relationship traversals
- [ ] 12.8 Write user documentation (README, tool reference, configuration guide)
- [ ] 12.9 Create demo/tutorial scenario — walkthrough of creating a change, adding specs, generating tasks, and completing workflow
- [ ] 12.10 Final build and verify binary size, startup time, release packaging
