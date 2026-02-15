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
	constitutions, err := client.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  emergent.TypeConstitution,
		Limit: 1,
	})
	if err != nil {
		return err
	}
	gctx.HasConstitution = len(constitutions) > 0

	// Check for patterns
	patterns, err := client.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  emergent.TypePattern,
		Limit: 100,
	})
	if err != nil {
		return err
	}
	gctx.HasPatterns = len(patterns) > 0
	gctx.PatternCount = len(patterns)

	// Check for contexts
	contexts, err := client.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  emergent.TypeContext,
		Limit: 100,
	})
	if err != nil {
		return err
	}
	gctx.ContextCount = len(contexts)

	// Check for components
	components, err := client.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  emergent.TypeUIComponent,
		Limit: 100,
	})
	if err != nil {
		return err
	}
	gctx.ComponentCount = len(components)

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
		for _, rel := range taskRels {
			task, err := client.GetTask(ctx, rel.DstID)
			if err != nil {
				return err
			}
			switch task.Status {
			case emergent.StatusCompleted:
				completed++
			case emergent.StatusPending:
				pending++
			}
		}
		gctx.CompletedTasks = completed
		gctx.PendingTasks = pending
	}

	return nil
}
