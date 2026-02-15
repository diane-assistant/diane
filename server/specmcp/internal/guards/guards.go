// Package guards implements the SpecMCP guardrail system.
//
// Guards are composable checks that enforce workflow constraints. Each guard returns
// a result with a severity level that determines how the system responds:
//
//   - HARD_BLOCK: Stops execution. User cannot proceed.
//   - SOFT_BLOCK: Stops execution by default but can be overridden with force=true.
//   - WARNING: Operation proceeds but includes advisory message in response.
//   - SUGGESTION: Operation proceeds with optional recommendation.
//
// Guards are grouped into GuardSets that run for specific operations (new change,
// add artifact, archive). The GuardRunner executes a set and aggregates results.
package guards

import (
	"context"
	"fmt"
	"strings"
)

// Severity indicates how a guard failure affects execution.
type Severity int

const (
	// Suggestion is advisory — operation proceeds, message included in response.
	Suggestion Severity = iota
	// Warning is advisory — operation proceeds, message included in response.
	Warning
	// SoftBlock stops execution unless force=true is provided.
	SoftBlock
	// HardBlock stops execution unconditionally.
	HardBlock
)

func (s Severity) String() string {
	switch s {
	case Suggestion:
		return "SUGGESTION"
	case Warning:
		return "WARNING"
	case SoftBlock:
		return "SOFT_BLOCK"
	case HardBlock:
		return "HARD_BLOCK"
	default:
		return "UNKNOWN"
	}
}

// Result is the outcome of a single guard check.
type Result struct {
	// GuardName identifies which guard produced this result.
	GuardName string `json:"guard_name"`
	// Passed is true if the guard check passed (no issue found).
	Passed bool `json:"passed"`
	// Severity of the failure (only meaningful when Passed is false).
	Severity Severity `json:"severity"`
	// Message describes the issue or recommendation.
	Message string `json:"message"`
	// Remedy suggests how to resolve the issue.
	Remedy string `json:"remedy,omitempty"`
}

// Outcome is the aggregated result of running a GuardSet.
type Outcome struct {
	// Blocked is true if any HARD_BLOCK or non-forced SOFT_BLOCK fired.
	Blocked bool `json:"blocked"`
	// Results contains all guard check results (both passed and failed).
	Results []Result `json:"results"`
}

// HardBlocks returns all hard block results.
func (o *Outcome) HardBlocks() []Result {
	return o.filterSeverity(HardBlock)
}

// SoftBlocks returns all soft block results.
func (o *Outcome) SoftBlocks() []Result {
	return o.filterSeverity(SoftBlock)
}

// Warnings returns all warning results.
func (o *Outcome) Warnings() []Result {
	return o.filterSeverity(Warning)
}

// Suggestions returns all suggestion results.
func (o *Outcome) Suggestions() []Result {
	return o.filterSeverity(Suggestion)
}

func (o *Outcome) filterSeverity(sev Severity) []Result {
	var out []Result
	for _, r := range o.Results {
		if !r.Passed && r.Severity == sev {
			out = append(out, r)
		}
	}
	return out
}

// FormatBlockMessage returns a human-readable message describing why the operation was blocked.
func (o *Outcome) FormatBlockMessage() string {
	if !o.Blocked {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Operation blocked by guards:\n")

	for _, r := range o.HardBlocks() {
		sb.WriteString(fmt.Sprintf("\n[HARD_BLOCK] %s: %s", r.GuardName, r.Message))
		if r.Remedy != "" {
			sb.WriteString(fmt.Sprintf("\n  Remedy: %s", r.Remedy))
		}
	}

	for _, r := range o.SoftBlocks() {
		sb.WriteString(fmt.Sprintf("\n[SOFT_BLOCK] %s: %s", r.GuardName, r.Message))
		if r.Remedy != "" {
			sb.WriteString(fmt.Sprintf("\n  Remedy: %s", r.Remedy))
		}
	}

	if len(o.SoftBlocks()) > 0 {
		sb.WriteString("\n\nUse force=true to override soft blocks.")
	}

	return sb.String()
}

// FormatAdvisoryMessage returns a human-readable message for warnings and suggestions.
func (o *Outcome) FormatAdvisoryMessage() string {
	warnings := o.Warnings()
	suggestions := o.Suggestions()
	if len(warnings) == 0 && len(suggestions) == 0 {
		return ""
	}

	var sb strings.Builder
	if len(warnings) > 0 {
		sb.WriteString("Warnings:\n")
		for _, r := range warnings {
			sb.WriteString(fmt.Sprintf("  - %s: %s", r.GuardName, r.Message))
			if r.Remedy != "" {
				sb.WriteString(fmt.Sprintf(" (%s)", r.Remedy))
			}
			sb.WriteString("\n")
		}
	}
	if len(suggestions) > 0 {
		sb.WriteString("Suggestions:\n")
		for _, r := range suggestions {
			sb.WriteString(fmt.Sprintf("  - %s: %s", r.GuardName, r.Message))
			if r.Remedy != "" {
				sb.WriteString(fmt.Sprintf(" (%s)", r.Remedy))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// Guard is a single check that can be composed into guard sets.
type Guard interface {
	// Name returns a short identifier for this guard.
	Name() string
	// Check evaluates the guard against the given context.
	// Returns a Result with Passed=true if the check passes.
	Check(ctx context.Context, gctx *GuardContext) Result
}

// GuardContext carries all the data guards need to make decisions.
// This avoids each guard needing to independently query the graph.
type GuardContext struct {
	// ChangeID is the change being operated on (empty for new changes).
	ChangeID string
	// ChangeName is the name of the change (for new changes).
	ChangeName string
	// ArtifactType is the type of artifact being added (for spec_artifact).
	ArtifactType string
	// Force allows overriding soft blocks.
	Force bool

	// Graph state — populated by the runner before executing guards.
	HasConstitution bool
	HasPatterns     bool // At least one Pattern entity exists
	PatternCount    int  // Number of Pattern entities
	HasProposal     bool // Change has a linked Proposal
	HasSpec         bool // Change has at least one linked Spec
	HasDesign       bool // Change has a linked Design
	HasTasks        bool // Change has at least one linked Task
	SpecCount       int  // Number of Specs for this change
	TaskCount       int  // Total tasks for this change
	CompletedTasks  int  // Completed tasks for this change
	PendingTasks    int  // Pending tasks for this change
	ContextCount    int  // Number of Context entities in the project
	ComponentCount  int  // Number of UIComponent entities in the project
}

// GuardFunc is a function-based guard for simple checks.
type GuardFunc struct {
	name  string
	check func(ctx context.Context, gctx *GuardContext) Result
}

// NewGuardFunc creates a guard from a function.
func NewGuardFunc(name string, fn func(ctx context.Context, gctx *GuardContext) Result) *GuardFunc {
	return &GuardFunc{name: name, check: fn}
}

func (g *GuardFunc) Name() string { return g.name }
func (g *GuardFunc) Check(ctx context.Context, gctx *GuardContext) Result {
	return g.check(ctx, gctx)
}

// Pass returns a passing result for the given guard name.
func Pass(guardName string) Result {
	return Result{GuardName: guardName, Passed: true}
}

// Fail returns a failing result with the given severity and message.
func Fail(guardName string, severity Severity, message, remedy string) Result {
	return Result{
		GuardName: guardName,
		Passed:    false,
		Severity:  severity,
		Message:   message,
		Remedy:    remedy,
	}
}

// Runner executes a set of guards and aggregates results.
type Runner struct{}

// NewRunner creates a guard runner.
func NewRunner() *Runner {
	return &Runner{}
}

// Run executes the given guards against the context and returns an aggregated outcome.
func (r *Runner) Run(ctx context.Context, gctx *GuardContext, guards []Guard) *Outcome {
	outcome := &Outcome{}

	for _, g := range guards {
		result := g.Check(ctx, gctx)
		outcome.Results = append(outcome.Results, result)

		if !result.Passed {
			switch result.Severity {
			case HardBlock:
				outcome.Blocked = true
			case SoftBlock:
				if !gctx.Force {
					outcome.Blocked = true
				}
			}
		}
	}

	return outcome
}
