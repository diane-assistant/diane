package guards

import (
	"context"

	"github.com/diane-assistant/diane/server/specmcp/internal/emergent"
	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"
)

// PopulateProjectState fills the GuardContext with project-level state
// (constitution, patterns, contexts, components). Used for pre-change guards.
func PopulateProjectState(ctx context.Context, client *emergent.Client, gctx *GuardContext) error {
	// Check for constitution
	constCount, err := client.CountObjects(ctx, emergent.TypeConstitution)
	if err != nil {
		return err
	}
	gctx.HasConstitution = constCount > 0

	// Count patterns
	patternCount, err := client.CountObjects(ctx, emergent.TypePattern)
	if err != nil {
		return err
	}
	gctx.HasPatterns = patternCount > 0
	gctx.PatternCount = patternCount

	// Count contexts
	contextCount, err := client.CountObjects(ctx, emergent.TypeContext)
	if err != nil {
		return err
	}
	gctx.ContextCount = contextCount

	// Count components
	componentCount, err := client.CountObjects(ctx, emergent.TypeUIComponent)
	if err != nil {
		return err
	}
	gctx.ComponentCount = componentCount

	return nil
}

// PopulateChangeState fills the GuardContext with change-level state
// (proposal, specs, design, tasks). Uses a single ExpandGraph call instead
// of multiple ListRelationships calls.
func PopulateChangeState(ctx context.Context, client *emergent.Client, gctx *GuardContext) error {
	if gctx.ChangeID == "" {
		return nil
	}

	// Single ExpandGraph call to get all change relationships and task properties
	resp, err := client.ExpandGraph(ctx, &graph.GraphExpandRequest{
		RootIDs:   []string{gctx.ChangeID},
		Direction: "outgoing",
		MaxDepth:  1,
		MaxNodes:  300,
		MaxEdges:  300,
		RelationshipTypes: []string{
			emergent.RelHasProposal,
			emergent.RelHasSpec,
			emergent.RelHasDesign,
			emergent.RelHasTask,
		},
	})
	if err != nil {
		return err
	}

	// Build node map for property access, dual-indexed by ID and CanonicalID
	// so edge endpoint lookups work regardless of which ID variant is stored.
	nodeMap := emergent.NewNodeIndex(resp.Nodes)

	// Normalize edge SrcID/DstID to match node primary IDs
	emergent.CanonicalizeEdgeIDs(resp.Edges, nodeMap)

	// Build change ID set for edge filtering (edges may reference either ID)
	changeIDs := emergent.IDSet{gctx.ChangeID: true}
	if node, ok := nodeMap[gctx.ChangeID]; ok {
		changeIDs = emergent.NewIDSet(node.ID, node.CanonicalID)
	}

	// Process edges to populate state
	var taskIDs []string
	for _, edge := range resp.Edges {
		if !changeIDs[edge.SrcID] {
			continue
		}
		switch edge.Type {
		case emergent.RelHasProposal:
			gctx.HasProposal = true
		case emergent.RelHasSpec:
			gctx.SpecCount++
		case emergent.RelHasDesign:
			gctx.HasDesign = true
		case emergent.RelHasTask:
			taskIDs = append(taskIDs, edge.DstID)
		}
	}

	gctx.HasSpec = gctx.SpecCount > 0
	gctx.HasTasks = len(taskIDs) > 0
	gctx.TaskCount = len(taskIDs)

	// Count completed/pending tasks from expand node properties
	if gctx.TaskCount > 0 {
		completed := 0
		pending := 0
		for _, taskID := range taskIDs {
			if node, ok := nodeMap[taskID]; ok && node.Properties != nil {
				switch node.Properties["status"] {
				case emergent.StatusCompleted:
					completed++
				case emergent.StatusPending:
					pending++
				}
			}
		}
		gctx.CompletedTasks = completed
		gctx.PendingTasks = pending
	}

	return nil
}
