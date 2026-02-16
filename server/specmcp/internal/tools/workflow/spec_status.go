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

// specStatusParams defines the input for spec_status.
type specStatusParams struct {
	ChangeID string `json:"change_id"`
}

// SpecStatus reports the current workflow position of a change, including
// artifact readiness summary and prioritized next steps.
type SpecStatus struct {
	client *emergent.Client
}

// NewSpecStatus creates a SpecStatus tool.
func NewSpecStatus(client *emergent.Client) *SpecStatus {
	return &SpecStatus{client: client}
}

func (t *SpecStatus) Name() string { return "spec_status" }

func (t *SpecStatus) Description() string {
	return "Get the workflow status of a change: current stage, readiness summary per artifact type, prioritized next steps to advance, and whether the change is ready to archive."
}

func (t *SpecStatus) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "change_id": {
      "type": "string",
      "description": "ID of the change to check status for"
    }
  },
  "required": ["change_id"]
}`)
}

// artifactSummary describes the readiness state of one artifact category.
type artifactSummary struct {
	Exists bool   `json:"exists"`
	Ready  bool   `json:"ready"`
	Count  int    `json:"count,omitempty"`
	Detail string `json:"detail,omitempty"`
}

func (t *SpecStatus) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p specStatusParams
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

	// Determine current stage and build per-artifact detail
	specDetail := t.buildSpecDetail(ctx, p.ChangeID, gctx)

	// Build artifact summaries
	artifacts := map[string]artifactSummary{
		"proposal": {
			Exists: gctx.HasProposal,
			Ready:  gctx.ProposalReady,
		},
		"specs": {
			Exists: gctx.HasSpec,
			Ready:  gctx.AllSpecsReady,
			Count:  gctx.SpecCount,
			Detail: specDetail,
		},
		"design": {
			Exists: gctx.HasDesign,
			Ready:  gctx.DesignReady,
		},
		"tasks": {
			Exists: gctx.HasTasks,
			Count:  gctx.TaskCount,
			Detail: fmt.Sprintf("%d/%d completed", gctx.CompletedTasks, gctx.TaskCount),
		},
	}

	// Determine stage
	stage := t.determineStage(gctx)

	// Build next steps
	nextSteps := t.buildNextSteps(gctx)

	// Ready to archive?
	readyToArchive := gctx.HasProposal && gctx.ProposalReady &&
		gctx.HasSpec && gctx.AllSpecsReady &&
		gctx.HasDesign && gctx.DesignReady &&
		gctx.HasTasks && gctx.TaskCount > 0 && gctx.CompletedTasks == gctx.TaskCount

	result := map[string]any{
		"change_id":        p.ChangeID,
		"change_name":      change.Name,
		"change_status":    change.Status,
		"stage":            stage,
		"artifacts":        artifacts,
		"next_steps":       nextSteps,
		"ready_to_archive": readyToArchive,
	}

	return mcp.JSONResult(result)
}

// determineStage returns the current workflow stage name.
func (t *SpecStatus) determineStage(gctx *guards.GuardContext) string {
	// Stages progress: propose → specify → design → implement → complete
	if !gctx.HasProposal {
		return "propose"
	}
	if !gctx.ProposalReady {
		return "propose" // Proposal exists but not ready
	}
	if !gctx.HasSpec {
		return "specify"
	}
	if !gctx.AllSpecsReady {
		return "specify" // Specs exist but not all ready
	}
	if !gctx.HasDesign {
		return "design"
	}
	if !gctx.DesignReady {
		return "design" // Design exists but not ready
	}
	if !gctx.HasTasks {
		return "implement"
	}
	if gctx.CompletedTasks < gctx.TaskCount {
		return "implement"
	}
	return "complete"
}

// buildNextSteps returns prioritized actions the user should take next.
func (t *SpecStatus) buildNextSteps(gctx *guards.GuardContext) []string {
	var steps []string

	if !gctx.HasProposal {
		steps = append(steps, "Add a proposal: spec_artifact with artifact_type='proposal'")
		return steps // Can't do anything else without a proposal
	}
	if !gctx.ProposalReady {
		steps = append(steps, "Mark proposal as ready: spec_mark_ready with the proposal's entity_id")
		return steps
	}

	if !gctx.HasSpec {
		steps = append(steps, "Add specs: spec_artifact with artifact_type='spec'")
		return steps
	}
	if !gctx.AllSpecsReady {
		steps = append(steps, "Mark all specs, requirements, and scenarios as ready (bottom-up) using spec_mark_ready")
		return steps
	}

	if !gctx.HasDesign {
		steps = append(steps, "Add a design: spec_artifact with artifact_type='design'")
		return steps
	}
	if !gctx.DesignReady {
		steps = append(steps, "Mark design as ready: spec_mark_ready with the design's entity_id")
		return steps
	}

	if !gctx.HasTasks {
		steps = append(steps, "Generate tasks: spec_generate_tasks")
		return steps
	}

	incomplete := gctx.TaskCount - gctx.CompletedTasks
	if incomplete > 0 {
		steps = append(steps, fmt.Sprintf("Complete %d remaining task(s): use spec_complete_task", incomplete))
		return steps
	}

	steps = append(steps, "All tasks complete. Archive the change: spec_archive")
	return steps
}

// buildSpecDetail returns a human-readable summary of spec readiness,
// including counts of unready requirements and scenarios.
func (t *SpecStatus) buildSpecDetail(ctx context.Context, changeID string, gctx *guards.GuardContext) string {
	if !gctx.HasSpec {
		return "no specs"
	}
	if gctx.AllSpecsReady {
		return "all ready"
	}

	// Use ExpandGraph to get detailed readiness breakdown
	resp, err := t.client.ExpandGraph(ctx, &graph.GraphExpandRequest{
		RootIDs:   []string{changeID},
		Direction: "outgoing",
		MaxDepth:  4,
		MaxNodes:  300,
		MaxEdges:  600,
		RelationshipTypes: []string{
			emergent.RelHasSpec,
			emergent.RelHasRequirement,
			emergent.RelHasScenario,
		},
	})
	if err != nil {
		return "readiness check failed"
	}

	nodeIdx := emergent.NewNodeIndex(resp.Nodes)
	emergent.CanonicalizeEdgeIDs(resp.Edges, nodeIdx)

	rootID := changeID
	if node, ok := nodeIdx[changeID]; ok {
		rootID = node.ID
	}

	edgesBySrcAndType := make(map[string]map[string][]string)
	for _, edge := range resp.Edges {
		if edgesBySrcAndType[edge.SrcID] == nil {
			edgesBySrcAndType[edge.SrcID] = make(map[string][]string)
		}
		edgesBySrcAndType[edge.SrcID][edge.Type] = append(
			edgesBySrcAndType[edge.SrcID][edge.Type], edge.DstID,
		)
	}

	nodeStatus := func(nodeID string) string {
		if node, ok := nodeIdx[nodeID]; ok && node.Properties != nil {
			if s, ok := node.Properties["status"].(string); ok && s != "" {
				return s
			}
		}
		return emergent.StatusDraft
	}

	specIDs := edgesBySrcAndType[rootID][emergent.RelHasSpec]
	draftSpecs := 0
	draftReqs := 0
	draftScens := 0
	totalReqs := 0
	totalScens := 0

	for _, specID := range specIDs {
		if nodeStatus(specID) != emergent.StatusReady {
			draftSpecs++
		}
		reqIDs := edgesBySrcAndType[specID][emergent.RelHasRequirement]
		totalReqs += len(reqIDs)
		for _, reqID := range reqIDs {
			if nodeStatus(reqID) != emergent.StatusReady {
				draftReqs++
			}
			scenIDs := edgesBySrcAndType[reqID][emergent.RelHasScenario]
			totalScens += len(scenIDs)
			for _, scenID := range scenIDs {
				if nodeStatus(scenID) != emergent.StatusReady {
					draftScens++
				}
			}
		}
	}

	parts := []string{}
	if draftScens > 0 {
		parts = append(parts, fmt.Sprintf("%d/%d scenarios draft", draftScens, totalScens))
	}
	if draftReqs > 0 {
		parts = append(parts, fmt.Sprintf("%d/%d requirements draft", draftReqs, totalReqs))
	}
	if draftSpecs > 0 {
		parts = append(parts, fmt.Sprintf("%d/%d specs draft", draftSpecs, len(specIDs)))
	}

	if len(parts) == 0 {
		return "all ready"
	}

	result := ""
	for i, p := range parts {
		if i > 0 {
			result += ", "
		}
		result += p
	}
	return result
}
