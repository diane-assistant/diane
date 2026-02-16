package workflow

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/diane-assistant/diane/server/specmcp/internal/emergent"
	"github.com/diane-assistant/diane/server/specmcp/internal/mcp"
	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"
)

// specMarkReadyParams defines the input for spec_mark_ready.
type specMarkReadyParams struct {
	EntityID string `json:"entity_id"`
}

// SpecMarkReady marks a workflow artifact as ready after validating
// that all its children (if any) are already ready.
type SpecMarkReady struct {
	client *emergent.Client
}

// NewSpecMarkReady creates a SpecMarkReady tool.
func NewSpecMarkReady(client *emergent.Client) *SpecMarkReady {
	return &SpecMarkReady{client: client}
}

func (t *SpecMarkReady) Name() string { return "spec_mark_ready" }

func (t *SpecMarkReady) Description() string {
	return "Mark a workflow artifact (Proposal, Spec, Requirement, Scenario, Design) as ready. Validates that all children are ready before allowing the transition: a Spec requires all its Requirements to be ready, and a Requirement requires all its Scenarios to be ready. Artifacts must be marked ready bottom-up before the next workflow stage unlocks."
}

func (t *SpecMarkReady) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "entity_id": {
      "type": "string",
      "description": "ID of the workflow artifact to mark as ready"
    }
  },
  "required": ["entity_id"]
}`)
}

// blocker describes a child entity that is not yet ready.
type blocker struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Name   string `json:"name,omitempty"`
	Status string `json:"status"`
}

func (t *SpecMarkReady) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p specMarkReadyParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}

	if p.EntityID == "" {
		return mcp.ErrorResult("entity_id is required"), nil
	}

	// Fetch the entity to determine its type
	obj, err := t.client.GetObject(ctx, p.EntityID)
	if err != nil {
		return mcp.ErrorResult(fmt.Sprintf("entity not found: %v", err)), nil
	}

	// Validate it's a workflow artifact type
	if !emergent.IsWorkflowArtifactType(obj.Type) {
		return mcp.ErrorResult(fmt.Sprintf(
			"entity type %q is not a workflow artifact. Only Proposal, Spec, Requirement, Scenario, and Design support readiness tracking.",
			obj.Type,
		)), nil
	}

	// Check if already ready
	if status, _ := obj.Properties["status"].(string); status == emergent.StatusReady {
		return mcp.JSONResult(map[string]any{
			"entity_id": obj.ID,
			"type":      obj.Type,
			"status":    emergent.StatusReady,
			"message":   "Already ready",
		})
	}

	// Determine which child relationship types to check based on entity type
	var childRelTypes []string
	switch obj.Type {
	case emergent.TypeSpec:
		childRelTypes = []string{emergent.RelHasRequirement}
	case emergent.TypeRequirement:
		childRelTypes = []string{emergent.RelHasScenario}
	}
	// Proposal, Scenario, Design have no children — can be marked ready immediately

	// Validate children are all ready (if this type has children)
	var blockers []blocker
	if len(childRelTypes) > 0 {
		blockers, err = t.findUnreadyChildren(ctx, obj.ID, childRelTypes)
		if err != nil {
			return nil, fmt.Errorf("checking children readiness: %w", err)
		}
	}

	if len(blockers) > 0 {
		return mcp.JSONResult(map[string]any{
			"entity_id": obj.ID,
			"type":      obj.Type,
			"status":    emergent.StatusDraft,
			"message":   fmt.Sprintf("Cannot mark as ready: %d child artifact(s) are not ready", len(blockers)),
			"blockers":  blockers,
			"remedy":    "Mark all child artifacts as ready first using spec_mark_ready, then retry.",
		})
	}

	// All children ready (or no children) — mark as ready
	_, err = t.client.UpdateObject(ctx, obj.ID, map[string]any{"status": emergent.StatusReady}, nil)
	if err != nil {
		return nil, fmt.Errorf("updating status to ready: %w", err)
	}

	name := ""
	if n, ok := obj.Properties["name"].(string); ok {
		name = n
	}

	result := map[string]any{
		"entity_id": obj.ID,
		"type":      obj.Type,
		"status":    emergent.StatusReady,
		"message":   fmt.Sprintf("Marked %s as ready", obj.Type),
	}
	if name != "" {
		result["name"] = name
	}

	return mcp.JSONResult(result)
}

// findUnreadyChildren uses ExpandGraph to find direct children that are not status=ready.
// For Specs, this checks Requirements (depth 1).
// For Requirements, this checks Scenarios (depth 1).
// A Spec also needs to recursively check that its Requirements' Scenarios are ready,
// so we expand to depth 2 when checking a Spec.
func (t *SpecMarkReady) findUnreadyChildren(ctx context.Context, entityID string, childRelTypes []string) ([]blocker, error) {
	// Determine expand depth: Spec needs depth 2 (req→scenario), others need depth 1
	maxDepth := 1
	allRelTypes := childRelTypes
	if len(childRelTypes) == 1 && childRelTypes[0] == emergent.RelHasRequirement {
		// Spec → we also need to check requirement→scenario
		maxDepth = 2
		allRelTypes = []string{emergent.RelHasRequirement, emergent.RelHasScenario}
	}

	resp, err := t.client.ExpandGraph(ctx, &graph.GraphExpandRequest{
		RootIDs:           []string{entityID},
		Direction:         "outgoing",
		MaxDepth:          maxDepth,
		MaxNodes:          200,
		MaxEdges:          500,
		RelationshipTypes: allRelTypes,
	})
	if err != nil {
		return nil, err
	}

	// Build node index and canonicalize edges
	nodeIdx := emergent.NewNodeIndex(resp.Nodes)
	emergent.CanonicalizeEdgeIDs(resp.Edges, nodeIdx)

	// Resolve the entity ID to the node's primary ID
	rootID := entityID
	if node, ok := nodeIdx[entityID]; ok {
		rootID = node.ID
	}

	// Index edges by src → type → []dstIDs
	edgesBySrcAndType := make(map[string]map[string][]string)
	for _, edge := range resp.Edges {
		if edgesBySrcAndType[edge.SrcID] == nil {
			edgesBySrcAndType[edge.SrcID] = make(map[string][]string)
		}
		edgesBySrcAndType[edge.SrcID][edge.Type] = append(
			edgesBySrcAndType[edge.SrcID][edge.Type], edge.DstID,
		)
	}

	// Helper to get node status, treating missing/empty as draft
	nodeStatus := func(nodeID string) string {
		if node, ok := nodeIdx[nodeID]; ok && node.Properties != nil {
			if s, ok := node.Properties["status"].(string); ok && s != "" {
				return s
			}
		}
		return emergent.StatusDraft
	}

	// Helper to get node name
	nodeName := func(nodeID string) string {
		if node, ok := nodeIdx[nodeID]; ok && node.Properties != nil {
			if n, ok := node.Properties["name"].(string); ok {
				return n
			}
		}
		return ""
	}

	// Helper to get node type
	nodeType := func(nodeID string) string {
		if node, ok := nodeIdx[nodeID]; ok {
			return node.Type
		}
		return ""
	}

	var result []blocker

	// Check direct children
	for _, relType := range childRelTypes {
		childIDs := edgesBySrcAndType[rootID][relType]
		for _, childID := range childIDs {
			if nodeStatus(childID) != emergent.StatusReady {
				result = append(result, blocker{
					ID:     childID,
					Type:   nodeType(childID),
					Name:   nodeName(childID),
					Status: nodeStatus(childID),
				})
			}

			// For Spec→Requirement, also check the requirement's scenarios
			if relType == emergent.RelHasRequirement {
				scenIDs := edgesBySrcAndType[childID][emergent.RelHasScenario]
				for _, scenID := range scenIDs {
					if nodeStatus(scenID) != emergent.StatusReady {
						result = append(result, blocker{
							ID:     scenID,
							Type:   nodeType(scenID),
							Name:   nodeName(scenID),
							Status: nodeStatus(scenID),
						})
					}
				}
			}
		}
	}

	return result, nil
}
