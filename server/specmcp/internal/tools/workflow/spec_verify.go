package workflow

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/diane-assistant/diane/server/specmcp/internal/emergent"
	"github.com/diane-assistant/diane/server/specmcp/internal/guards"
	"github.com/diane-assistant/diane/server/specmcp/internal/mcp"
	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"
)

// specVerifyParams defines the input for spec_verify.
type specVerifyParams struct {
	ChangeID string `json:"change_id"`
}

// SpecVerify verifies a change across 3 dimensions: completeness, correctness, coherence.
type SpecVerify struct {
	client *emergent.Client
}

// NewSpecVerify creates a SpecVerify tool.
func NewSpecVerify(client *emergent.Client) *SpecVerify {
	return &SpecVerify{client: client}
}

func (t *SpecVerify) Name() string { return "spec_verify" }

func (t *SpecVerify) Description() string {
	return "Verify a change across 3 dimensions: completeness (all required artifacts and tasks exist), correctness (requirements map to implementations), and coherence (design patterns are consistent). Returns a verification report with issues categorized by severity."
}

func (t *SpecVerify) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "change_id": {
      "type": "string",
      "description": "ID of the change to verify"
    }
  },
  "required": ["change_id"]
}`)
}

// verifyIssue represents a single verification issue.
type verifyIssue struct {
	Dimension string `json:"dimension"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
	Remedy    string `json:"remedy,omitempty"`
}

func (t *SpecVerify) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p specVerifyParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}

	if p.ChangeID == "" {
		return mcp.ErrorResult("change_id is required"), nil
	}

	// Verify change exists
	change, err := t.client.GetChange(ctx, p.ChangeID)
	if err != nil {
		return mcp.ErrorResult(fmt.Sprintf("change not found: %v", err)), nil
	}

	// Build guard context for state
	gctx := &guards.GuardContext{ChangeID: p.ChangeID}
	if err := guards.PopulateChangeState(ctx, t.client, gctx); err != nil {
		return nil, fmt.Errorf("populating change state: %w", err)
	}
	if err := guards.PopulateProjectState(ctx, t.client, gctx); err != nil {
		return nil, fmt.Errorf("populating project state: %w", err)
	}

	var issues []verifyIssue

	// --- Dimension 1: Completeness ---
	issues = append(issues, t.checkCompleteness(ctx, gctx)...)

	// --- Dimension 2: Correctness ---
	issues = append(issues, t.checkCorrectness(ctx, p.ChangeID, gctx)...)

	// --- Dimension 3: Coherence ---
	issues = append(issues, t.checkCoherence(ctx, p.ChangeID, gctx)...)

	// Build summary
	criticalCount := 0
	warningCount := 0
	suggestionCount := 0
	for _, issue := range issues {
		switch issue.Severity {
		case "CRITICAL":
			criticalCount++
		case "WARNING":
			warningCount++
		case "SUGGESTION":
			suggestionCount++
		}
	}

	status := "PASS"
	if criticalCount > 0 {
		status = "FAIL"
	} else if warningCount > 0 {
		status = "WARN"
	}

	result := map[string]any{
		"change_id":   p.ChangeID,
		"change_name": change.Name,
		"status":      status,
		"summary": map[string]any{
			"critical":    criticalCount,
			"warnings":    warningCount,
			"suggestions": suggestionCount,
			"total":       len(issues),
		},
		"issues": issues,
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.TextContent(string(b))},
	}, nil
}

// checkCompleteness verifies all required artifacts exist.
func (t *SpecVerify) checkCompleteness(_ context.Context, gctx *guards.GuardContext) []verifyIssue {
	var issues []verifyIssue

	// Proposal is required
	if !gctx.HasProposal {
		issues = append(issues, verifyIssue{
			Dimension: "completeness",
			Severity:  "CRITICAL",
			Message:   "Change has no Proposal. Every change needs a proposal documenting why it exists.",
			Remedy:    "Add a proposal using spec_artifact with artifact_type='proposal'.",
		})
	}

	// Specs should exist
	if !gctx.HasSpec {
		issues = append(issues, verifyIssue{
			Dimension: "completeness",
			Severity:  "CRITICAL",
			Message:   "Change has no Specs. Specs define what the change does.",
			Remedy:    "Add specs using spec_artifact with artifact_type='spec'.",
		})
	}

	// Design should exist
	if !gctx.HasDesign {
		issues = append(issues, verifyIssue{
			Dimension: "completeness",
			Severity:  "CRITICAL",
			Message:   "Change has no Design. A design defines the technical approach.",
			Remedy:    "Add a design using spec_artifact with artifact_type='design'.",
		})
	}

	// Tasks should exist
	if !gctx.HasTasks {
		issues = append(issues, verifyIssue{
			Dimension: "completeness",
			Severity:  "CRITICAL",
			Message:   "Change has no Tasks. Tasks break the work into implementable steps.",
			Remedy:    "Add tasks using spec_artifact with artifact_type='task' or spec_generate_tasks.",
		})
	}

	// Task completion audit
	if gctx.TaskCount > 0 {
		incomplete := gctx.TaskCount - gctx.CompletedTasks
		if incomplete > 0 {
			issues = append(issues, verifyIssue{
				Dimension: "completeness",
				Severity:  "CRITICAL",
				Message:   fmt.Sprintf("%d of %d tasks are incomplete.", incomplete, gctx.TaskCount),
				Remedy:    "Complete remaining tasks using spec_complete_task.",
			})
		}
	}

	// Constitution governance
	if !gctx.HasConstitution {
		issues = append(issues, verifyIssue{
			Dimension: "completeness",
			Severity:  "WARNING",
			Message:   "No Constitution governs this project. Changes without a constitution lack principled guardrails.",
			Remedy:    "Create a constitution using spec_artifact with artifact_type='constitution'.",
		})
	}

	return issues
}

// checkCorrectness verifies requirements map to implementations.
func (t *SpecVerify) checkCorrectness(ctx context.Context, changeID string, gctx *guards.GuardContext) []verifyIssue {
	var issues []verifyIssue

	if !gctx.HasSpec {
		return issues // Can't check correctness without specs
	}

	// Get specs and their requirements
	specRels, err := t.client.ListRelationships(ctx, &graph.ListRelationshipsOptions{
		Type:  emergent.RelHasSpec,
		SrcID: changeID,
		Limit: 100,
	})
	if err != nil {
		issues = append(issues, verifyIssue{
			Dimension: "correctness",
			Severity:  "WARNING",
			Message:   fmt.Sprintf("Could not retrieve specs for correctness check: %v", err),
		})
		return issues
	}

	totalRequirements := 0
	requirementsWithScenarios := 0

	for _, specRel := range specRels {
		// Get requirements for this spec
		reqRels, err := t.client.ListRelationships(ctx, &graph.ListRelationshipsOptions{
			Type:  emergent.RelHasRequirement,
			SrcID: specRel.DstID,
			Limit: 100,
		})
		if err != nil {
			continue
		}

		if len(reqRels) == 0 {
			// Get spec details for the message
			specObj, err := t.client.GetObject(ctx, specRel.DstID)
			if err != nil {
				continue
			}
			name := ""
			if n, ok := specObj.Properties["name"].(string); ok {
				name = n
			}
			issues = append(issues, verifyIssue{
				Dimension: "correctness",
				Severity:  "WARNING",
				Message:   fmt.Sprintf("Spec %q has no requirements. Specs without requirements may be underspecified.", name),
				Remedy:    "Add requirements to the spec to define expected behaviors.",
			})
			continue
		}

		for _, reqRel := range reqRels {
			totalRequirements++

			// Check if requirement has scenarios
			scenRels, err := t.client.ListRelationships(ctx, &graph.ListRelationshipsOptions{
				Type:  emergent.RelHasScenario,
				SrcID: reqRel.DstID,
				Limit: 1,
			})
			if err != nil {
				continue
			}
			if len(scenRels) > 0 {
				requirementsWithScenarios++
			}
		}
	}

	// Scenario coverage check
	if totalRequirements > 0 {
		uncovered := totalRequirements - requirementsWithScenarios
		if uncovered > 0 {
			issues = append(issues, verifyIssue{
				Dimension: "correctness",
				Severity:  "WARNING",
				Message:   fmt.Sprintf("%d of %d requirements have no scenarios. Scenarios provide concrete examples of expected behavior.", uncovered, totalRequirements),
				Remedy:    "Add scenarios to requirements using spec_artifact with artifact_type='scenario'.",
			})
		}
	}

	// Check that tasks implement something
	if gctx.HasTasks {
		taskRels, err := t.client.ListRelationships(ctx, &graph.ListRelationshipsOptions{
			Type:  emergent.RelHasTask,
			SrcID: changeID,
			Limit: 200,
		})
		if err == nil {
			orphanTasks := 0
			for _, taskRel := range taskRels {
				implRels, err := t.client.ListRelationships(ctx, &graph.ListRelationshipsOptions{
					Type:  emergent.RelImplements,
					SrcID: taskRel.DstID,
					Limit: 1,
				})
				if err != nil {
					continue
				}
				if len(implRels) == 0 {
					orphanTasks++
				}
			}
			if orphanTasks > 0 {
				issues = append(issues, verifyIssue{
					Dimension: "correctness",
					Severity:  "SUGGESTION",
					Message:   fmt.Sprintf("%d tasks have no 'implements' relationship. Linking tasks to requirements or specs improves traceability.", orphanTasks),
					Remedy:    "Add 'implements' field when creating tasks to link them to requirements or specs.",
				})
			}
		}
	}

	return issues
}

// checkCoherence verifies design adherence and pattern consistency.
func (t *SpecVerify) checkCoherence(ctx context.Context, changeID string, gctx *guards.GuardContext) []verifyIssue {
	var issues []verifyIssue

	// Check if change is governed by a constitution
	govRels, err := t.client.ListRelationships(ctx, &graph.ListRelationshipsOptions{
		Type:  emergent.RelGovernedBy,
		SrcID: changeID,
		Limit: 1,
	})
	if err == nil && len(govRels) == 0 && gctx.HasConstitution {
		issues = append(issues, verifyIssue{
			Dimension: "coherence",
			Severity:  "WARNING",
			Message:   "Change is not linked to a Constitution via 'governed_by'. A constitution exists in the project but this change doesn't reference it.",
			Remedy:    "Link the change to a constitution to ensure governance.",
		})
	}

	// Check pattern usage — if patterns exist, the change should use some
	if gctx.PatternCount > 0 && gctx.HasDesign {
		// Check if the design or any specs use patterns
		designRels, err := t.client.ListRelationships(ctx, &graph.ListRelationshipsOptions{
			Type:  emergent.RelHasDesign,
			SrcID: changeID,
			Limit: 1,
		})
		if err == nil && len(designRels) > 0 {
			designID := designRels[0].DstID
			patternRels, err := t.client.ListRelationships(ctx, &graph.ListRelationshipsOptions{
				Type:  emergent.RelUsesPattern,
				SrcID: designID,
				Limit: 1,
			})
			if err == nil && len(patternRels) == 0 {
				issues = append(issues, verifyIssue{
					Dimension: "coherence",
					Severity:  "SUGGESTION",
					Message:   fmt.Sprintf("Design does not reference any patterns. %d patterns are available in the project.", gctx.PatternCount),
					Remedy:    "Use spec_apply_pattern to link relevant patterns to the design for consistency.",
				})
			}
		}
	}

	return issues
}
