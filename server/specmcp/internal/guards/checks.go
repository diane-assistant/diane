package guards

import (
	"context"
	"fmt"
	"regexp"
)

// kebabCaseRegex matches valid kebab-case identifiers: lowercase letters, digits, and hyphens.
var kebabCaseRegex = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// --- Pre-Change Guards ---
// These guards run before a new Change can be created.

// ConstitutionRequired ensures a Constitution exists in the project before any change is created.
// This is a HARD_BLOCK — every project must have a constitution before changes can be created.
// Use spec_create_constitution to bootstrap the project's constitution.
var ConstitutionRequired = NewGuardFunc("constitution_required", func(_ context.Context, gctx *GuardContext) Result {
	if gctx.HasConstitution {
		return Pass("constitution_required")
	}
	return Fail("constitution_required", HardBlock,
		"No Constitution found in the project. A constitution defines project principles, guardrails, testing requirements, and pattern mandates. Every project must have a constitution before changes can be created.",
		"Create a constitution first using spec_create_constitution.",
	)
})

// PatternsSeeded ensures at least some patterns exist in the project before changes are created.
// This is a SOFT_BLOCK — patterns should be seeded early but can be added later.
var PatternsSeeded = NewGuardFunc("patterns_seeded", func(_ context.Context, gctx *GuardContext) Result {
	if gctx.HasPatterns {
		return Pass("patterns_seeded")
	}
	return Fail("patterns_seeded", SoftBlock,
		"No patterns found in the project. Patterns capture recurring implementation decisions (naming conventions, error handling, etc.). Seeding patterns before starting changes helps maintain consistency.",
		"Use spec_seed_patterns to seed built-in patterns, or use force=true to skip.",
	)
})

// ContextDiscovery recommends that at least some Context entities (screens, pages, interaction surfaces)
// exist before starting work. This is a SUGGESTION — not all projects have UI contexts.
var ContextDiscovery = NewGuardFunc("context_discovery", func(_ context.Context, gctx *GuardContext) Result {
	if gctx.ContextCount > 0 {
		return Pass("context_discovery")
	}
	return Fail("context_discovery", Suggestion,
		"No Context entities found. Contexts represent screens, pages, or interaction surfaces. Mapping them before changes helps with impact analysis and scenario design.",
		"Use spec_artifact with artifact_type='context' to define project contexts, or ignore this suggestion if the project has no UI.",
	)
})

// ComponentDiscovery recommends that at least some UIComponent entities exist before starting work.
// This is a SUGGESTION — not all projects use components.
var ComponentDiscovery = NewGuardFunc("component_discovery", func(_ context.Context, gctx *GuardContext) Result {
	if gctx.ComponentCount > 0 {
		return Pass("component_discovery")
	}
	return Fail("component_discovery", Suggestion,
		"No UIComponent entities found. Mapping reusable components helps track dependencies and change impact.",
		"Use spec_artifact with artifact_type='ui_component' to register components, or ignore this if not applicable.",
	)
})

// --- Proposal Guards ---
// These guards validate a proposal before it's created.

// KebabCaseName ensures the change name follows kebab-case convention.
var KebabCaseName = NewGuardFunc("kebab_case_name", func(_ context.Context, gctx *GuardContext) Result {
	if gctx.ChangeName == "" {
		return Pass("kebab_case_name") // name validation happens elsewhere
	}
	if kebabCaseRegex.MatchString(gctx.ChangeName) {
		return Pass("kebab_case_name")
	}
	return Fail("kebab_case_name", HardBlock,
		"Change name must be kebab-case (lowercase letters, digits, and hyphens, starting with a letter). Got: "+gctx.ChangeName,
		"Use a name like 'add-user-permissions' or 'fix-login-bug'.",
	)
})

// --- Artifact Workflow Guards ---
// These guards enforce the Proposal → Spec → Design → Tasks dependency chain.

// ProposalBeforeSpec ensures a Proposal exists before Specs can be added.
var ProposalBeforeSpec = NewGuardFunc("proposal_before_spec", func(_ context.Context, gctx *GuardContext) Result {
	// Only applies to spec-related artifacts
	switch gctx.ArtifactType {
	case "spec", "requirement", "scenario", "scenario_step":
	default:
		return Pass("proposal_before_spec")
	}
	if gctx.HasProposal {
		return Pass("proposal_before_spec")
	}
	return Fail("proposal_before_spec", HardBlock,
		"Change must have a Proposal before adding Specs. The proposal captures the intent (why) before defining the specifications (what).",
		"Add a proposal first using spec_artifact with artifact_type='proposal'.",
	)
})

// SpecBeforeDesign ensures at least one Spec exists before a Design can be added.
var SpecBeforeDesign = NewGuardFunc("spec_before_design", func(_ context.Context, gctx *GuardContext) Result {
	if gctx.ArtifactType != "design" {
		return Pass("spec_before_design")
	}
	if !gctx.HasProposal {
		return Fail("spec_before_design", HardBlock,
			"Change must have a Proposal before adding a Design.",
			"Add a proposal first using spec_artifact with artifact_type='proposal'.",
		)
	}
	if !gctx.HasSpec {
		return Fail("spec_before_design", HardBlock,
			"Change must have at least one Spec before adding a Design. Specs define what changes are needed; the design defines how.",
			"Add specs first using spec_artifact with artifact_type='spec'.",
		)
	}
	return Pass("spec_before_design")
})

// DesignBeforeTasks ensures a Design exists before Tasks can be added.
var DesignBeforeTasks = NewGuardFunc("design_before_tasks", func(_ context.Context, gctx *GuardContext) Result {
	if gctx.ArtifactType != "task" {
		return Pass("design_before_tasks")
	}
	if !gctx.HasDesign {
		return Fail("design_before_tasks", HardBlock,
			"Change must have a Design before adding Tasks. The design defines the technical approach; tasks break it into implementable steps.",
			"Add a design first using spec_artifact with artifact_type='design'.",
		)
	}
	return Pass("design_before_tasks")
})

// --- Archive Guards ---
// These guards validate an archive operation.

// ArtifactCompleteness checks that core artifacts (proposal, specs, design, tasks) exist before archiving.
var ArtifactCompleteness = NewGuardFunc("artifact_completeness", func(_ context.Context, gctx *GuardContext) Result {
	var missing []string
	if !gctx.HasProposal {
		missing = append(missing, "proposal")
	}
	if !gctx.HasSpec {
		missing = append(missing, "specs")
	}
	if !gctx.HasDesign {
		missing = append(missing, "design")
	}
	if !gctx.HasTasks {
		missing = append(missing, "tasks")
	}

	if len(missing) == 0 {
		return Pass("artifact_completeness")
	}

	return Fail("artifact_completeness", SoftBlock,
		"Change is missing artifacts: "+joinComma(missing)+". Archiving an incomplete change may lose context about what was planned vs. implemented.",
		"Add the missing artifacts, or use force=true to archive anyway.",
	)
})

// TaskCompletionCheck ensures all tasks are completed before archiving.
var TaskCompletionCheck = NewGuardFunc("task_completion", func(_ context.Context, gctx *GuardContext) Result {
	if gctx.TaskCount == 0 {
		return Pass("task_completion") // No tasks to check
	}
	incomplete := gctx.TaskCount - gctx.CompletedTasks
	if incomplete == 0 {
		return Pass("task_completion")
	}
	return Fail("task_completion", SoftBlock,
		fmt.Sprintf("%d of %d tasks are incomplete. Archiving with incomplete tasks may indicate unfinished work.", incomplete, gctx.TaskCount),
		"Complete remaining tasks, or use force=true to archive anyway.",
	)
})

// --- Guard Sets ---
// Pre-built guard collections for each operation.

// NewChangeGuards returns the guards that run before creating a new change.
func NewChangeGuards() []Guard {
	return []Guard{
		KebabCaseName,
		ConstitutionRequired,
		PatternsSeeded,
		ContextDiscovery,
		ComponentDiscovery,
	}
}

// ArtifactGuards returns the guards that run before adding an artifact.
func ArtifactGuards() []Guard {
	return []Guard{
		ProposalBeforeSpec,
		SpecBeforeDesign,
		DesignBeforeTasks,
	}
}

// ArchiveGuards returns the guards that run before archiving a change.
func ArchiveGuards() []Guard {
	return []Guard{
		ArtifactCompleteness,
		TaskCompletionCheck,
	}
}

// joinComma joins strings with commas and "and" for the last element.
func joinComma(ss []string) string {
	switch len(ss) {
	case 0:
		return ""
	case 1:
		return ss[0]
	case 2:
		return ss[0] + " and " + ss[1]
	default:
		return joinStrings(ss[:len(ss)-1], ", ") + ", and " + ss[len(ss)-1]
	}
}

func joinStrings(ss []string, sep string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
