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
// (proposal, specs, design, tasks). Used for artifact and archive guards.
func PopulateChangeState(ctx context.Context, client *emergent.Client, gctx *GuardContext) error {
	if gctx.ChangeID == "" {
		return nil
	}

	// Check for proposal
	proposalRels, err := client.ListRelationships(ctx, &graph.ListRelationshipsOptions{
		Type:  emergent.RelHasProposal,
		SrcID: gctx.ChangeID,
		Limit: 1,
	})
	if err != nil {
		return err
	}
	gctx.HasProposal = len(proposalRels) > 0

	// Check for specs
	specRels, err := client.ListRelationships(ctx, &graph.ListRelationshipsOptions{
		Type:  emergent.RelHasSpec,
		SrcID: gctx.ChangeID,
		Limit: 100,
	})
	if err != nil {
		return err
	}
	gctx.HasSpec = len(specRels) > 0
	gctx.SpecCount = len(specRels)

	// Check for design
	designRels, err := client.ListRelationships(ctx, &graph.ListRelationshipsOptions{
		Type:  emergent.RelHasDesign,
		SrcID: gctx.ChangeID,
		Limit: 1,
	})
	if err != nil {
		return err
	}
	gctx.HasDesign = len(designRels) > 0

	// Check for tasks
	taskRels, err := client.ListRelationships(ctx, &graph.ListRelationshipsOptions{
		Type:  emergent.RelHasTask,
		SrcID: gctx.ChangeID,
		Limit: 200,
	})
	if err != nil {
		return err
	}
	gctx.HasTasks = len(taskRels) > 0
	gctx.TaskCount = len(taskRels)

	// Count completed tasks
	if gctx.TaskCount > 0 {
		completed := 0
		pending := 0
		// Batch-fetch all task objects
		taskIDs := make([]string, len(taskRels))
		for i, rel := range taskRels {
			taskIDs[i] = rel.DstID
		}
		taskObjs, err := client.GetObjects(ctx, taskIDs)
		if err != nil {
			return err
		}
		for _, obj := range taskObjs {
			if obj.Properties != nil {
				switch obj.Properties["status"] {
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
