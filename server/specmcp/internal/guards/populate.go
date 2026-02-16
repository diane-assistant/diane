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
// of multiple ListRelationships calls. Also computes readiness booleans
// by reading status properties from workflow artifacts.
func PopulateChangeState(ctx context.Context, client *emergent.Client, gctx *GuardContext) error {
	if gctx.ChangeID == "" {
		return nil
	}

	// Single ExpandGraph call to get all change relationships, task properties,
	// and spec→requirement→scenario hierarchy for readiness checking
	resp, err := client.ExpandGraph(ctx, &graph.GraphExpandRequest{
		RootIDs:   []string{gctx.ChangeID},
		Direction: "outgoing",
		MaxDepth:  4, // change→spec→requirement→scenario (depth 3) + change→task (depth 1)
		MaxNodes:  500,
		MaxEdges:  1000,
		RelationshipTypes: []string{
			emergent.RelHasProposal,
			emergent.RelHasSpec,
			emergent.RelHasDesign,
			emergent.RelHasTask,
			emergent.RelHasRequirement,
			emergent.RelHasScenario,
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

	// Index edges by src → type → []dstIDs for hierarchy traversal
	edgesBySrcAndType := make(map[string]map[string][]string)
	for _, edge := range resp.Edges {
		if edgesBySrcAndType[edge.SrcID] == nil {
			edgesBySrcAndType[edge.SrcID] = make(map[string][]string)
		}
		edgesBySrcAndType[edge.SrcID][edge.Type] = append(
			edgesBySrcAndType[edge.SrcID][edge.Type], edge.DstID,
		)
	}

	// Helper: get status property from a node, treating missing/empty as "draft"
	nodeStatus := func(nodeID string) string {
		if node, ok := nodeMap[nodeID]; ok && node.Properties != nil {
			if s, ok := node.Properties["status"].(string); ok && s != "" {
				return s
			}
		}
		return emergent.StatusDraft
	}

	// Process direct change edges to populate state
	var taskIDs []string
	var proposalIDs []string
	var specIDs []string
	var designIDs []string

	for _, edge := range resp.Edges {
		if !changeIDs[edge.SrcID] {
			continue
		}
		switch edge.Type {
		case emergent.RelHasProposal:
			gctx.HasProposal = true
			proposalIDs = append(proposalIDs, edge.DstID)
		case emergent.RelHasSpec:
			gctx.SpecCount++
			specIDs = append(specIDs, edge.DstID)
		case emergent.RelHasDesign:
			gctx.HasDesign = true
			designIDs = append(designIDs, edge.DstID)
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

	// --- Readiness computation ---

	// Proposal readiness: all proposals must be ready (typically there's just one)
	if gctx.HasProposal && len(proposalIDs) > 0 {
		allReady := true
		for _, pid := range proposalIDs {
			if nodeStatus(pid) != emergent.StatusReady {
				allReady = false
				break
			}
		}
		gctx.ProposalReady = allReady
	}

	// Design readiness: all designs must be ready (typically there's just one)
	if gctx.HasDesign && len(designIDs) > 0 {
		allReady := true
		for _, did := range designIDs {
			if nodeStatus(did) != emergent.StatusReady {
				allReady = false
				break
			}
		}
		gctx.DesignReady = allReady
	}

	// Spec readiness: cascading — all specs ready, AND each spec's
	// requirements must be ready, AND each requirement's scenarios must be ready
	if gctx.HasSpec && len(specIDs) > 0 {
		allSpecsReady := true
		for _, specID := range specIDs {
			if nodeStatus(specID) != emergent.StatusReady {
				allSpecsReady = false
				break
			}
			// Check requirements
			reqIDs := edgesBySrcAndType[specID][emergent.RelHasRequirement]
			for _, reqID := range reqIDs {
				if nodeStatus(reqID) != emergent.StatusReady {
					allSpecsReady = false
					break
				}
				// Check scenarios
				scenIDs := edgesBySrcAndType[reqID][emergent.RelHasScenario]
				for _, scenID := range scenIDs {
					if nodeStatus(scenID) != emergent.StatusReady {
						allSpecsReady = false
						break
					}
				}
				if !allSpecsReady {
					break
				}
			}
			if !allSpecsReady {
				break
			}
		}
		gctx.AllSpecsReady = allSpecsReady
	}

	return nil
}
