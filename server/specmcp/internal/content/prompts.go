// Package content provides MCP prompts and resources for the SpecMCP server.
package content

import "github.com/diane-assistant/diane/server/specmcp/internal/mcp"

// --- specmcp-guide prompt ---

// GuidePrompt is the primary LLM-facing prompt that explains the SpecMCP
// concept, workflow, guardrails, and how to use the tools.
type GuidePrompt struct{}

func (p *GuidePrompt) Definition() mcp.PromptDefinition {
	return mcp.PromptDefinition{
		Name:        "specmcp-guide",
		Description: "Comprehensive guide to SpecMCP: concept, workflow, guardrails, and tool usage",
		Arguments: []mcp.PromptArgument{
			{
				Name:        "focus",
				Description: "Optional focus area: 'overview', 'workflow', 'guardrails', 'tools', or 'patterns'. Defaults to full guide.",
				Required:    false,
			},
		},
	}
}

func (p *GuidePrompt) Get(arguments map[string]string) (*mcp.PromptsGetResult, error) {
	focus := arguments["focus"]

	var text string
	switch focus {
	case "workflow":
		text = guideWorkflowSection
	case "guardrails":
		text = guideGuardrailsSection
	case "tools":
		text = guideToolsSection
	case "patterns":
		text = guidePatternsSection
	default:
		text = guideFull
	}

	return &mcp.PromptsGetResult{
		Description: "SpecMCP guide" + focusSuffix(focus),
		Messages: []mcp.PromptMessage{
			{
				Role:    "user",
				Content: mcp.TextContent(text),
			},
		},
	}, nil
}

func focusSuffix(focus string) string {
	if focus == "" {
		return ""
	}
	return " (" + focus + ")"
}

// --- specmcp-workflow prompt ---

// WorkflowPrompt is a focused prompt that guides an LLM through the
// artifact workflow sequence for a specific change.
type WorkflowPrompt struct{}

func (p *WorkflowPrompt) Definition() mcp.PromptDefinition {
	return mcp.PromptDefinition{
		Name:        "specmcp-workflow",
		Description: "Step-by-step guide for the SpecMCP artifact workflow (Proposal → Specs → Design → Tasks)",
		Arguments: []mcp.PromptArgument{
			{
				Name:        "change_name",
				Description: "Name of the change to work on (kebab-case)",
				Required:    false,
			},
			{
				Name:        "stage",
				Description: "Current stage: 'proposal', 'specs', 'design', 'tasks', 'implement', or 'archive'. Defaults to 'proposal'.",
				Required:    false,
			},
		},
	}
}

func (p *WorkflowPrompt) Get(arguments map[string]string) (*mcp.PromptsGetResult, error) {
	changeName := arguments["change_name"]
	stage := arguments["stage"]
	if stage == "" {
		stage = "proposal"
	}

	text := buildWorkflowGuide(changeName, stage)

	return &mcp.PromptsGetResult{
		Description: "SpecMCP workflow guide — stage: " + stage,
		Messages: []mcp.PromptMessage{
			{
				Role:    "user",
				Content: mcp.TextContent(text),
			},
		},
	}, nil
}

func buildWorkflowGuide(changeName, stage string) string {
	header := "# SpecMCP Workflow Guide\n\n"
	if changeName != "" {
		header += "**Change**: `" + changeName + "`\n"
	}
	header += "**Current stage**: " + stage + "\n\n"

	switch stage {
	case "proposal":
		return header + workflowProposal(changeName)
	case "specs":
		return header + workflowSpecs(changeName)
	case "design":
		return header + workflowDesign(changeName)
	case "tasks":
		return header + workflowTasks(changeName)
	case "implement":
		return header + workflowImplement(changeName)
	case "archive":
		return header + workflowArchive(changeName)
	default:
		return header + workflowProposal(changeName)
	}
}

func changeArg(name string) string {
	if name == "" {
		return "<change-name>"
	}
	return name
}

func workflowProposal(name string) string {
	cn := changeArg(name)
	return `## Stage 1: Proposal (Why)

The proposal captures the **intent** of the change — why it needs to happen.

### Steps

1. **Create the change** (if not done yet):
   Call ` + "`spec_new`" + ` with name="` + cn + `" and provide a clear intent.

2. **Add the proposal artifact**:
   Call ` + "`spec_artifact`" + ` with:
   - change_name="` + cn + `"
   - artifact_type="proposal"
   - Provide: intent (required), scope, and impact

3. **Review**: The proposal should answer:
   - What problem does this solve?
   - Who is affected?
   - What is the expected impact?

### Guardrails
- Change name must be **kebab-case** (hard block)
- A **Constitution** must exist (hard block — use ` + "`spec_create_constitution`" + ` to bootstrap)
- **Patterns** should be seeded (soft block, override with force=true)

### Next stage
Once the proposal is created, move to **specs** stage.
`
}

func workflowSpecs(name string) string {
	cn := changeArg(name)
	return `## Stage 2: Specs (What)

Specs define **what** the change needs to do. Each spec is a domain-specific container holding requirements and scenarios.

### Steps

1. **Add spec(s)**:
   Call ` + "`spec_artifact`" + ` with:
   - change_name="` + cn + `"
   - artifact_type="spec"
   - Provide: name, domain, purpose, delta_type

2. **Add requirements** to each spec:
   Call ` + "`spec_artifact`" + ` with artifact_type="requirement"
   - Provide: name, description, strength (MUST/SHOULD/MAY)

3. **Add scenarios** to requirements:
   Call ` + "`spec_artifact`" + ` with artifact_type="scenario"
   - Provide: name, given/when/then (BDD format)

### Guardrails
- Proposal **must exist** before adding specs (hard block)
- Each requirement should have at least one scenario

### Next stage
Once specs capture all behavioral requirements, move to **design** stage.
`
}

func workflowDesign(name string) string {
	cn := changeArg(name)
	return `## Stage 3: Design (How)

The design describes **how** the specs will be implemented.

### Steps

1. **Add the design**:
   Call ` + "`spec_artifact`" + ` with:
   - change_name="` + cn + `"
   - artifact_type="design"
   - Provide: approach, decisions, data_flow, file_changes

2. **Check patterns**: Call ` + "`spec_suggest_patterns`" + ` to see which patterns apply
3. **Apply patterns**: Call ` + "`spec_apply_pattern`" + ` for each relevant pattern

### Guardrails
- Proposal and at least one spec **must exist** before adding a design (hard block)
- Design should reference existing patterns where applicable

### Next stage
Once the design is complete, move to **tasks** stage.
`
}

func workflowTasks(name string) string {
	cn := changeArg(name)
	return `## Stage 4: Tasks (Steps)

Tasks break the design into concrete, implementable steps.

### Steps

1. **Generate tasks** (recommended):
   Call ` + "`spec_generate_tasks`" + ` with change_name="` + cn + `"
   This auto-generates tasks from the design.

2. **Or add manually**:
   Call ` + "`spec_artifact`" + ` with artifact_type="task"
   - Provide: number, description, task_type, complexity_points (1-10)

3. **Set dependencies**: Tasks can block other tasks using the blocks/blocked_by relationship

4. **View critical path**: Call ` + "`spec_get_critical_path`" + ` to see the dependency chain

### Guardrails
- A **design must exist** before adding tasks (hard block)
- Complexity uses **1-10 points**, NOT hours

### Next stage
Once tasks are defined, move to **implement** stage.
`
}

func workflowImplement(name string) string {
	cn := changeArg(name)
	return `## Stage 5: Implement

Work through tasks in dependency order.

### Steps

1. **Get available tasks**: Call ` + "`spec_get_available_tasks`" + ` with change_name="` + cn + `"
   Returns tasks whose dependencies are satisfied.

2. **Assign a task**: Call ` + "`spec_assign_task`" + ` to claim a task

3. **Complete a task**: Call ` + "`spec_complete_task`" + ` when done
   - Provide: verification_notes and artifacts (files changed)

4. **Repeat** until all tasks are complete

### Tips
- Use ` + "`spec_impact_analysis`" + ` to understand ripple effects
- Use ` + "`spec_get_patterns`" + ` to check pattern compliance
- Use ` + "`spec_validate_constitution`" + ` to verify against project principles

### Next stage
Once all tasks are complete, move to **archive** stage.
`
}

func workflowArchive(name string) string {
	cn := changeArg(name)
	return `## Stage 6: Archive

Finalize and archive the completed change.

### Steps

1. **Verify the change**: Call ` + "`spec_verify`" + ` with change_name="` + cn + `"
   Checks completeness, correctness, and coherence.

2. **Address any issues**: Fix warnings and suggestions from verification

3. **Archive**: Call ` + "`spec_archive`" + ` with change_name="` + cn + `"

### Guardrails
- All core artifacts should exist (soft block)
- All tasks should be completed (soft block)
- Override with force=true if needed

### Done!
The change is now archived with full traceability in the knowledge graph.
`
}

// --- Full guide content ---

const guideFull = `# SpecMCP — Spec-Driven Development via MCP

## What is SpecMCP?

SpecMCP is a Model Context Protocol (MCP) server that provides **spec-driven development workflow** backed by a knowledge graph. Instead of managing specifications as scattered files, SpecMCP stores all artifacts — proposals, specs, requirements, scenarios, designs, tasks, patterns, and constitutions — as graph entities with typed relationships.

This enables:
- **Traceability**: Every requirement traces to scenarios, design decisions, and tasks
- **Impact analysis**: Understand what's affected when something changes
- **Dependency management**: Tasks with explicit blocking relationships and critical path analysis
- **Pattern enforcement**: Reusable patterns with automatic detection and compliance checking
- **Guardrails**: Automated checks at every stage to prevent workflow mistakes

## Core Concept: Artifact Workflow

Every change follows a structured progression:

` + "```" + `
Proposal (Why) → Specs (What) → Design (How) → Tasks (Steps)
` + "```" + `

Each stage builds on the previous one, enforced by guardrails:
- You cannot add specs without a proposal
- You cannot add a design without at least one spec
- You cannot add tasks without a design

## Entity Model

SpecMCP uses 18 entity types and 30+ relationship types. Key entities:

| Entity | Purpose |
|--------|---------|
| **Change** | Top-level container for a feature, fix, or refactoring |
| **Proposal** | Intent — why the change exists |
| **Spec** | Domain-specific specification container |
| **Requirement** | Specific behavior the system must have |
| **Scenario** | BDD-style example (Given/When/Then) |
| **Design** | Technical approach — how specs will be implemented |
| **Task** | Implementable step with complexity, status, blocking |
| **Pattern** | Reusable implementation convention |
| **Constitution** | Project-wide principles and guardrails |
| **Actor** | User role or persona |
| **Context** | Screen, page, or interaction surface |
| **UIComponent** | Reusable UI component |
| **Action** | User action or system operation |
| **TestCase** | Executable test linked to a scenario |
| **APIContract** | Machine-readable API definition |
| **CodingAgent** | Developer or AI agent |

## Guardrails

SpecMCP enforces guardrails at three points:

### Pre-Change Guards (spec_new)
| Guard | Severity | What it checks |
|-------|----------|---------------|
| kebab_case_name | HARD_BLOCK | Change name must be kebab-case |
| constitution_required | HARD_BLOCK | Project should have a constitution |
| patterns_seeded | SOFT_BLOCK | Project should have patterns |
| context_discovery | SUGGESTION | Contexts should be mapped |
| component_discovery | SUGGESTION | Components should be mapped |

### Artifact Guards (spec_artifact)
| Guard | Severity | What it checks |
|-------|----------|---------------|
| proposal_before_spec | HARD_BLOCK | Proposal required before specs |
| spec_before_design | HARD_BLOCK | Specs required before design |
| design_before_tasks | HARD_BLOCK | Design required before tasks |

### Archive Guards (spec_archive)
| Guard | Severity | What it checks |
|-------|----------|---------------|
| artifact_completeness | SOFT_BLOCK | All core artifacts should exist |
| task_completion | SOFT_BLOCK | All tasks should be completed |

**Severity levels**:
- **HARD_BLOCK**: Cannot proceed. Fix the issue first.
- **SOFT_BLOCK**: Should not proceed. Use force=true to override.
- **WARNING**: Advisory — action recommended but not required.
- **SUGGESTION**: Informational tip.

## Tools Reference

### Workflow (4 tools)
- **spec_new** — Create a new change container
- **spec_artifact** — Add any artifact type to a change (16 types supported)
- **spec_archive** — Archive a completed change
- **spec_verify** — Verify completeness, correctness, and coherence

### Query (8 tools)
- **spec_list_changes** — List all changes (filter by status)
- **spec_get_change** — Get full change details with artifacts
- **spec_get_context** — Get a context entity with relationships
- **spec_get_component** — Get a UI component with relationships
- **spec_get_action** — Get an action entity with relationships
- **spec_get_scenario** — Get a scenario with steps
- **spec_get_patterns** — Get all patterns (filter by type)
- **spec_impact_analysis** — Analyze what's affected by a change

### Task Management (5 tools)
- **spec_generate_tasks** — Auto-generate tasks from a design
- **spec_get_available_tasks** — Get tasks ready for work (dependencies met)
- **spec_assign_task** — Assign a task to an agent
- **spec_complete_task** — Mark a task as done with verification
- **spec_get_critical_path** — Find the longest dependency chain

### Patterns (3 tools)
- **spec_suggest_patterns** — Suggest applicable patterns for a change
- **spec_apply_pattern** — Link a pattern to a change
- **spec_seed_patterns** — Seed 15 built-in patterns

### Constitution (2 tools)
- **spec_create_constitution** — Create or update the project constitution (no change_id required)
- **spec_validate_constitution** — Check change compliance against constitution

### Sync (3 tools)
- **spec_sync_status** — Check graph sync state
- **spec_sync** — Synchronize graph with codebase
- **spec_graph_summary** — Get entity/relationship counts

## Recommended Workflow

1. **Bootstrap** (once per project):
   - Seed patterns: ` + "`spec_seed_patterns`" + `
   - Create constitution: ` + "`spec_create_constitution`" + ` (no change_id required)

2. **Start a change**:
   - ` + "`spec_new`" + ` with a kebab-case name

3. **Define the change** (follow the artifact workflow):
   - Proposal → Specs → Requirements → Scenarios → Design → Tasks

4. **Implement**:
   - Work through tasks in dependency order using task management tools

5. **Verify and archive**:
   - ` + "`spec_verify`" + ` to check completeness
   - ` + "`spec_archive`" + ` to finalize
`

const guideWorkflowSection = `# SpecMCP Artifact Workflow

Every change follows a structured artifact progression, each stage building on the previous:

## Proposal (Why)
Captures the **intent** — why this change needs to happen.
- Fields: intent (required), scope, impact
- Created via: spec_artifact with artifact_type="proposal"

## Specs (What)
Define **what** the change does. Each spec is a domain-specific container.
- Fields: name, domain, purpose, delta_type
- Contains: Requirements (MUST/SHOULD/MAY)
- Requirements contain: Scenarios (Given/When/Then BDD format)

## Design (How)
Describes **how** specs will be implemented.
- Fields: approach, decisions, data_flow, file_changes
- Should reference applicable patterns

## Tasks (Steps)
Break the design into concrete, implementable steps.
- Fields: number, description, task_type, complexity_points (1-10)
- Relationships: blocks/blocked_by (auto-bidirectional), has_subtask, assigned_to
- Auto-generation available via spec_generate_tasks

## Enforcement
The workflow order is enforced by guardrails:
- Specs require a Proposal (hard block)
- Design requires Proposal + Specs (hard block)
- Tasks require a Design (hard block)
`

const guideGuardrailsSection = `# SpecMCP Guardrails

Guardrails are automated checks that run at key points in the workflow to prevent mistakes.

## Severity Levels

| Level | Meaning | Override |
|-------|---------|---------|
| HARD_BLOCK | Cannot proceed | Must fix the issue |
| SOFT_BLOCK | Should not proceed | Use force=true to override |
| WARNING | Advisory | Action recommended |
| SUGGESTION | Informational | No action required |

## Pre-Change Guards (run on spec_new)
1. **kebab_case_name** [HARD_BLOCK] — Name must be lowercase letters, digits, hyphens
2. **constitution_required** [HARD_BLOCK] — Project must have a constitution (use spec_create_constitution)
3. **patterns_seeded** [SOFT_BLOCK] — Project should have patterns seeded
4. **context_discovery** [SUGGESTION] — Map interaction surfaces first
5. **component_discovery** [SUGGESTION] — Map reusable components first

## Artifact Guards (run on spec_artifact)
1. **proposal_before_spec** [HARD_BLOCK] — Proposal must exist before specs
2. **spec_before_design** [HARD_BLOCK] — At least one spec must exist before design
3. **design_before_tasks** [HARD_BLOCK] — Design must exist before tasks

## Archive Guards (run on spec_archive)
1. **artifact_completeness** [SOFT_BLOCK] — All core artifacts should exist
2. **task_completion** [SOFT_BLOCK] — All tasks should be completed

## Verification (spec_verify)
Three-dimensional check:
1. **Completeness** — Do all required artifacts exist? Are tasks done?
2. **Correctness** — Do requirements map to scenarios? Is coverage sufficient?
3. **Coherence** — Does implementation follow the design and patterns?
`

const guideToolsSection = `# SpecMCP Tools Reference

## Workflow Tools
| Tool | Purpose |
|------|---------|
| spec_new | Create a new change (name must be kebab-case) |
| spec_artifact | Add any artifact type to a change |
| spec_archive | Archive a completed change |
| spec_verify | Verify change completeness/correctness/coherence |

## Query Tools
| Tool | Purpose |
|------|---------|
| spec_list_changes | List changes, filter by status |
| spec_get_change | Get change details + all artifacts |
| spec_get_context | Get a context with relationships |
| spec_get_component | Get a UI component with relationships |
| spec_get_action | Get an action with relationships |
| spec_get_scenario | Get scenario with steps |
| spec_get_patterns | Get patterns, filter by type |
| spec_impact_analysis | Analyze change impact across graph |

## Task Management Tools
| Tool | Purpose |
|------|---------|
| spec_generate_tasks | Auto-generate tasks from design |
| spec_get_available_tasks | Get unblocked tasks ready for work |
| spec_assign_task | Assign task to an agent |
| spec_complete_task | Complete task with verification notes |
| spec_get_critical_path | Find longest dependency chain |

## Pattern Tools
| Tool | Purpose |
|------|---------|
| spec_suggest_patterns | Suggest patterns for a change |
| spec_apply_pattern | Link pattern to change |
| spec_seed_patterns | Seed 15 built-in patterns |

## Constitution & Sync Tools
| Tool | Purpose |
|------|---------|
| spec_create_constitution | Create or update project constitution |
| spec_validate_constitution | Check constitution compliance |
| spec_sync_status | Check graph sync state |
| spec_sync | Sync graph with codebase |
| spec_graph_summary | Entity/relationship counts |
`

const guidePatternsSection = `# SpecMCP Patterns

Patterns are reusable implementation conventions stored in the knowledge graph. They capture recurring decisions so teams stay consistent.

## Pattern Types
- **naming** — Naming conventions (files, variables, types)
- **structural** — Code organization and architecture
- **behavioral** — Runtime behavior patterns (error handling, logging)
- **testing** — Test organization and coverage patterns
- **security** — Security-related conventions
- **api** — API design conventions

## Built-in Patterns (15)
SpecMCP ships with 15 built-in patterns covering common conventions:
- Kebab-case file naming, PascalCase types, camelCase functions
- Error handling with wrapped errors
- Repository pattern for data access
- Input validation at boundaries
- Structured logging
- Test naming conventions
- And more

Seed them with: spec_seed_patterns

## Pattern Workflow
1. **Seed** built-in patterns (spec_seed_patterns)
2. **Discover** patterns from existing code during sync
3. **Suggest** applicable patterns for a change (spec_suggest_patterns)
4. **Apply** patterns to link them (spec_apply_pattern)
5. **Enforce** via constitution (requires_pattern / forbids_pattern relationships)

## Constitution Integration
A Constitution can mandate or forbid patterns:
- **requires_pattern**: Change must use this pattern
- **forbids_pattern**: Change must not use this pattern
Use spec_validate_constitution to check compliance.
`
