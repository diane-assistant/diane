// Package query implements the SpecMCP query tools:
// spec_get_context, spec_get_component, spec_get_action, spec_get_scenario,
// spec_get_patterns, spec_impact_analysis, spec_list_changes.
package query

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/diane-assistant/diane/server/specmcp/internal/emergent"
	"github.com/diane-assistant/diane/server/specmcp/internal/mcp"
	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"
)

// --- spec_get_context ---

type getContextParams struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// GetContext retrieves a Context entity and its relationships.
type GetContext struct {
	client *emergent.Client
}

func NewGetContext(client *emergent.Client) *GetContext {
	return &GetContext{client: client}
}

func (t *GetContext) Name() string { return "spec_get_context" }
func (t *GetContext) Description() string {
	return "Get a Context (screen/modal/interaction surface) with its components, actions, nested contexts, and patterns."
}
func (t *GetContext) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "id": {"type": "string", "description": "Context entity ID"},
    "name": {"type": "string", "description": "Context name (used if id not provided)"}
  }
}`)
}

func (t *GetContext) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p getContextParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}

	obj, err := resolveEntity(ctx, t.client, emergent.TypeContext, p.ID, p.Name)
	if err != nil {
		return mcp.ErrorResult(err.Error()), nil
	}

	// Expand graph to get related entities
	expanded, err := t.client.GetEntityWithRelationships(ctx, obj.ID, []string{
		emergent.RelUsesComponent,
		emergent.RelAvailableIn,
		emergent.RelNavigatesTo,
		emergent.RelNestedIn,
		emergent.RelUsesPattern,
		emergent.RelOwnedBy,
	}, 1)
	if err != nil {
		return nil, fmt.Errorf("expanding context: %w", err)
	}

	return jsonResult(buildEntityResponse(obj, expanded))
}

// --- spec_get_component ---

type getComponentParams struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type GetComponent struct {
	client *emergent.Client
}

func NewGetComponent(client *emergent.Client) *GetComponent {
	return &GetComponent{client: client}
}

func (t *GetComponent) Name() string { return "spec_get_component" }
func (t *GetComponent) Description() string {
	return "Get a UIComponent with its composition hierarchy, contexts, and patterns."
}
func (t *GetComponent) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "id": {"type": "string", "description": "UIComponent entity ID"},
    "name": {"type": "string", "description": "UIComponent name (used if id not provided)"}
  }
}`)
}

func (t *GetComponent) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p getComponentParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}

	obj, err := resolveEntity(ctx, t.client, emergent.TypeUIComponent, p.ID, p.Name)
	if err != nil {
		return mcp.ErrorResult(err.Error()), nil
	}

	expanded, err := t.client.GetEntityWithRelationships(ctx, obj.ID, []string{
		emergent.RelComposedOf,
		emergent.RelUsesComponent,
		emergent.RelUsesPattern,
		emergent.RelOwnedBy,
	}, 2) // depth 2 for composition hierarchy
	if err != nil {
		return nil, fmt.Errorf("expanding component: %w", err)
	}

	return jsonResult(buildEntityResponse(obj, expanded))
}

// --- spec_get_action ---

type getActionParams struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type GetAction struct {
	client *emergent.Client
}

func NewGetAction(client *emergent.Client) *GetAction {
	return &GetAction{client: client}
}

func (t *GetAction) Name() string { return "spec_get_action" }
func (t *GetAction) Description() string {
	return "Get an Action with its available contexts, navigation targets, and patterns."
}
func (t *GetAction) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "id": {"type": "string", "description": "Action entity ID"},
    "name": {"type": "string", "description": "Action name (used if id not provided)"}
  }
}`)
}

func (t *GetAction) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p getActionParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}

	obj, err := resolveEntity(ctx, t.client, emergent.TypeAction, p.ID, p.Name)
	if err != nil {
		return mcp.ErrorResult(err.Error()), nil
	}

	expanded, err := t.client.GetEntityWithRelationships(ctx, obj.ID, []string{
		emergent.RelAvailableIn,
		emergent.RelNavigatesTo,
		emergent.RelUsesPattern,
		emergent.RelImplementsContract,
		emergent.RelOwnedBy,
	}, 1)
	if err != nil {
		return nil, fmt.Errorf("expanding action: %w", err)
	}

	return jsonResult(buildEntityResponse(obj, expanded))
}

// --- spec_get_scenario ---

type getScenarioParams struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type GetScenario struct {
	client *emergent.Client
}

func NewGetScenario(client *emergent.Client) *GetScenario {
	return &GetScenario{client: client}
}

func (t *GetScenario) Name() string { return "spec_get_scenario" }
func (t *GetScenario) Description() string {
	return "Get a Scenario with its steps, actor, requirement, test cases, and variants."
}
func (t *GetScenario) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "id": {"type": "string", "description": "Scenario entity ID"},
    "name": {"type": "string", "description": "Scenario name (used if id not provided)"}
  }
}`)
}

func (t *GetScenario) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p getScenarioParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}

	obj, err := resolveEntity(ctx, t.client, emergent.TypeScenario, p.ID, p.Name)
	if err != nil {
		return mcp.ErrorResult(err.Error()), nil
	}

	expanded, err := t.client.GetEntityWithRelationships(ctx, obj.ID, []string{
		emergent.RelHasStep,
		emergent.RelExecutedBy,
		emergent.RelTestedBy,
		emergent.RelVariantOf,
		emergent.RelHasScenario, // reverse: find parent requirement
	}, 1)
	if err != nil {
		return nil, fmt.Errorf("expanding scenario: %w", err)
	}

	return jsonResult(buildEntityResponse(obj, expanded))
}

// --- spec_get_patterns ---

type getPatternsParams struct {
	Type  string `json:"type,omitempty"`
	Scope string `json:"scope,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

type GetPatterns struct {
	client *emergent.Client
}

func NewGetPatterns(client *emergent.Client) *GetPatterns {
	return &GetPatterns{client: client}
}

func (t *GetPatterns) Name() string { return "spec_get_patterns" }
func (t *GetPatterns) Description() string {
	return "List patterns with optional filtering by type (naming, structural, behavioral, error_handling) and scope (component, module, system)."
}
func (t *GetPatterns) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "type": {
      "type": "string",
      "description": "Filter by pattern type",
      "enum": ["naming", "structural", "behavioral", "error_handling"]
    },
    "scope": {
      "type": "string",
      "description": "Filter by pattern scope",
      "enum": ["component", "module", "system"]
    },
    "limit": {
      "type": "integer",
      "description": "Max results (default 50)",
      "default": 50
    }
  }
}`)
}

func (t *GetPatterns) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p getPatternsParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}

	limit := p.Limit
	if limit <= 0 {
		limit = 50
	}

	opts := &graph.ListObjectsOptions{
		Type:  emergent.TypePattern,
		Limit: limit,
	}

	// Add property filters
	var filters []graph.PropertyFilter
	if p.Type != "" {
		filters = append(filters, graph.PropertyFilter{
			Path: "type", Op: "eq", Value: p.Type,
		})
	}
	if p.Scope != "" {
		filters = append(filters, graph.PropertyFilter{
			Path: "scope", Op: "eq", Value: p.Scope,
		})
	}
	if len(filters) > 0 {
		opts.PropertyFilters = filters
	}

	objs, err := t.client.ListObjects(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("listing patterns: %w", err)
	}

	patterns := make([]map[string]any, 0, len(objs))
	for _, obj := range objs {
		p := map[string]any{
			"id":         obj.ID,
			"properties": obj.Properties,
		}
		if obj.Key != nil {
			p["name"] = *obj.Key
		}
		patterns = append(patterns, p)
	}

	return jsonResult(map[string]any{
		"patterns": patterns,
		"count":    len(patterns),
	})
}

// --- spec_impact_analysis ---

type impactAnalysisParams struct {
	EntityID  string   `json:"entity_id"`
	MaxDepth  int      `json:"max_depth,omitempty"`
	Direction string   `json:"direction,omitempty"`
	RelTypes  []string `json:"relationship_types,omitempty"`
}

type ImpactAnalysis struct {
	client *emergent.Client
}

func NewImpactAnalysis(client *emergent.Client) *ImpactAnalysis {
	return &ImpactAnalysis{client: client}
}

func (t *ImpactAnalysis) Name() string { return "spec_impact_analysis" }
func (t *ImpactAnalysis) Description() string {
	return "Analyze the impact of changing an entity by traversing the graph to find all affected entities. Uses multi-hop graph traversal."
}
func (t *ImpactAnalysis) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "entity_id": {
      "type": "string",
      "description": "ID of the entity to analyze impact for"
    },
    "max_depth": {
      "type": "integer",
      "description": "Maximum traversal depth (default: 3)",
      "default": 3
    },
    "direction": {
      "type": "string",
      "description": "Traversal direction: 'outgoing', 'incoming', or 'both' (default: 'both')",
      "enum": ["outgoing", "incoming", "both"],
      "default": "both"
    },
    "relationship_types": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Filter to specific relationship types (default: all)"
    }
  },
  "required": ["entity_id"]
}`)
}

func (t *ImpactAnalysis) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p impactAnalysisParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}

	if p.EntityID == "" {
		return mcp.ErrorResult("entity_id is required"), nil
	}

	maxDepth := p.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 3
	}
	direction := p.Direction
	if direction == "" {
		direction = "both"
	}

	// Get the source entity
	root, err := t.client.GetObject(ctx, p.EntityID)
	if err != nil {
		return mcp.ErrorResult(fmt.Sprintf("entity not found: %v", err)), nil
	}

	// Expand graph from this entity
	expanded, err := t.client.ExpandGraph(ctx, &graph.GraphExpandRequest{
		RootIDs:                       []string{p.EntityID},
		Direction:                     direction,
		MaxDepth:                      maxDepth,
		MaxNodes:                      200,
		MaxEdges:                      500,
		RelationshipTypes:             p.RelTypes,
		IncludeRelationshipProperties: true,
	})
	if err != nil {
		return nil, fmt.Errorf("expanding graph: %w", err)
	}

	// Build impact summary grouped by type
	typeGroups := make(map[string][]map[string]any)
	if expanded.Nodes != nil {
		for _, node := range expanded.Nodes {
			if node.ID == p.EntityID {
				continue // skip the root
			}
			entry := map[string]any{
				"id": node.ID,
			}
			if node.Key != nil {
				entry["name"] = *node.Key
			}
			if node.Properties != nil {
				if name, ok := node.Properties["name"]; ok {
					entry["name"] = name
				}
			}
			typeGroups[node.Type] = append(typeGroups[node.Type], entry)
		}
	}

	// Build edge summary
	edges := make([]map[string]any, 0)
	if expanded.Edges != nil {
		for _, edge := range expanded.Edges {
			edges = append(edges, map[string]any{
				"type": edge.Type,
				"from": edge.SrcID,
				"to":   edge.DstID,
			})
		}
	}

	totalAffected := 0
	for _, group := range typeGroups {
		totalAffected += len(group)
	}

	rootName := ""
	if root.Key != nil {
		rootName = *root.Key
	}

	return jsonResult(map[string]any{
		"entity": map[string]any{
			"id":   p.EntityID,
			"type": root.Type,
			"name": rootName,
		},
		"impact": map[string]any{
			"total_affected": totalAffected,
			"by_type":        typeGroups,
			"max_depth":      maxDepth,
			"direction":      direction,
		},
		"edges": edges,
	})
}

// --- spec_list_changes ---

type listChangesParams struct {
	Status string `json:"status,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type ListChanges struct {
	client *emergent.Client
}

func NewListChanges(client *emergent.Client) *ListChanges {
	return &ListChanges{client: client}
}

func (t *ListChanges) Name() string { return "spec_list_changes" }
func (t *ListChanges) Description() string {
	return "List all changes, optionally filtered by status (active, archived)."
}
func (t *ListChanges) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "status": {
      "type": "string",
      "description": "Filter by status",
      "enum": ["active", "archived"]
    },
    "limit": {
      "type": "integer",
      "description": "Max results (default 50)"
    }
  }
}`)
}

func (t *ListChanges) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p listChangesParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}

	changes, err := t.client.ListChanges(ctx, p.Status)
	if err != nil {
		return nil, fmt.Errorf("listing changes: %w", err)
	}

	results := make([]map[string]any, 0, len(changes))
	for _, ch := range changes {
		results = append(results, map[string]any{
			"id":     ch.ID,
			"name":   ch.Name,
			"status": ch.Status,
			"tags":   ch.Tags,
		})
	}

	return jsonResult(map[string]any{
		"changes": results,
		"count":   len(results),
	})
}

// --- spec_get_change ---

type getChangeParams struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type GetChange struct {
	client *emergent.Client
}

func NewGetChange(client *emergent.Client) *GetChange {
	return &GetChange{client: client}
}

func (t *GetChange) Name() string { return "spec_get_change" }
func (t *GetChange) Description() string {
	return "Get a change with all its artifacts: proposal, specs, design, tasks, and constitution."
}
func (t *GetChange) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "id": {"type": "string", "description": "Change entity ID"},
    "name": {"type": "string", "description": "Change name (used if id not provided)"}
  }
}`)
}

func (t *GetChange) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p getChangeParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}

	obj, err := resolveEntity(ctx, t.client, emergent.TypeChange, p.ID, p.Name)
	if err != nil {
		return mcp.ErrorResult(err.Error()), nil
	}

	// Expand to get all direct artifacts
	expanded, err := t.client.GetEntityWithRelationships(ctx, obj.ID, []string{
		emergent.RelHasProposal,
		emergent.RelHasSpec,
		emergent.RelHasDesign,
		emergent.RelHasTask,
		emergent.RelGovernedBy,
	}, 1)
	if err != nil {
		return nil, fmt.Errorf("expanding change: %w", err)
	}

	return jsonResult(buildEntityResponse(obj, expanded))
}

// --- Shared helpers ---

// resolveEntity finds an entity by ID or by type+name lookup.
func resolveEntity(ctx context.Context, client *emergent.Client, typeName, id, name string) (*graph.GraphObject, error) {
	if id != "" {
		obj, err := client.GetObject(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("%s not found: %v", typeName, err)
		}
		return obj, nil
	}
	if name != "" {
		obj, err := client.FindByTypeAndKey(ctx, typeName, name)
		if err != nil {
			return nil, fmt.Errorf("looking up %s: %v", typeName, err)
		}
		if obj == nil {
			return nil, fmt.Errorf("%s %q not found", typeName, name)
		}
		return obj, nil
	}
	return nil, fmt.Errorf("either id or name is required")
}

// buildEntityResponse constructs the standard entity+relationships response.
func buildEntityResponse(obj *graph.GraphObject, expanded *graph.GraphExpandResponse) map[string]any {
	entity := map[string]any{
		"id":         obj.ID,
		"type":       obj.Type,
		"properties": obj.Properties,
		"labels":     obj.Labels,
		"created_at": obj.CreatedAt,
	}
	if obj.Key != nil {
		entity["name"] = *obj.Key
	}

	// Group related entities by relationship type
	relationships := make(map[string][]map[string]any)
	if expanded != nil && expanded.Edges != nil {
		// Build node lookup from ExpandNode types
		nodeLookup := make(map[string]*graph.ExpandNode)
		if expanded.Nodes != nil {
			for _, node := range expanded.Nodes {
				nodeLookup[node.ID] = node
			}
		}

		for _, edge := range expanded.Edges {
			// Find the "other" end of the relationship
			otherID := edge.DstID
			if edge.DstID == obj.ID {
				otherID = edge.SrcID
			}

			other := nodeLookup[otherID]
			if other == nil {
				continue
			}

			entry := map[string]any{
				"id":   other.ID,
				"type": other.Type,
			}
			if other.Key != nil {
				entry["name"] = *other.Key
			}
			if other.Properties != nil {
				// Include a subset of key properties
				for _, key := range []string{"name", "description", "status", "intent", "approach", "number"} {
					if v, ok := other.Properties[key]; ok {
						entry[key] = v
					}
				}
			}
			relationships[edge.Type] = append(relationships[edge.Type], entry)
		}
	}

	return map[string]any{
		"entity":        entity,
		"relationships": relationships,
		"metadata": map[string]any{
			"node_count": countNodes(expanded),
			"edge_count": countEdges(expanded),
		},
	}
}

func countNodes(expanded *graph.GraphExpandResponse) int {
	if expanded == nil || expanded.Nodes == nil {
		return 0
	}
	return len(expanded.Nodes)
}

func countEdges(expanded *graph.GraphExpandResponse) int {
	if expanded == nil || expanded.Edges == nil {
		return 0
	}
	return len(expanded.Edges)
}

func jsonResult(v any) (*mcp.ToolsCallResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling result: %w", err)
	}
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.TextContent(string(b))},
	}, nil
}
