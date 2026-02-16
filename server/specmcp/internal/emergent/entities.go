package emergent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"
)

// toProps converts a typed struct to a map[string]any via JSON round-trip.
func toProps(v any) (map[string]any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal entity: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("unmarshal to map: %w", err)
	}
	// Remove id field — it's not a property
	delete(m, "id")
	return m, nil
}

// fromProps converts a GraphObject's properties into a typed struct.
func fromProps[T any](obj *graph.GraphObject) (*T, error) {
	// Start with properties, then overlay with the object ID
	props := obj.Properties
	if props == nil {
		props = make(map[string]any)
	}
	b, err := json.Marshal(props)
	if err != nil {
		return nil, fmt.Errorf("marshal properties: %w", err)
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, fmt.Errorf("unmarshal to %T: %w", v, err)
	}
	return &v, nil
}

// strPtr returns a pointer to a string.
func strPtr(s string) *string {
	return &s
}

// --- Change ---

// CreateChange creates a new Change entity.
func (c *Client) CreateChange(ctx context.Context, ch *Change) (*Change, error) {
	props, err := toProps(ch)
	if err != nil {
		return nil, err
	}
	obj, err := c.CreateObject(ctx, TypeChange, strPtr(ch.Name), props, ch.Tags)
	if err != nil {
		return nil, err
	}
	result, err := fromProps[Change](obj)
	if err != nil {
		return nil, err
	}
	result.ID = obj.ID
	return result, nil
}

// GetChange retrieves a Change by ID.
func (c *Client) GetChange(ctx context.Context, id string) (*Change, error) {
	obj, err := c.GetObject(ctx, id)
	if err != nil {
		return nil, err
	}
	result, err := fromProps[Change](obj)
	if err != nil {
		return nil, err
	}
	result.ID = obj.ID
	return result, nil
}

// FindChange finds a Change by name.
func (c *Client) FindChange(ctx context.Context, name string) (*Change, error) {
	obj, err := c.FindByTypeAndKey(ctx, TypeChange, name)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, nil
	}
	result, err := fromProps[Change](obj)
	if err != nil {
		return nil, err
	}
	result.ID = obj.ID
	return result, nil
}

// ListChanges lists all Change entities, optionally filtered by status.
func (c *Client) ListChanges(ctx context.Context, status string) ([]*Change, error) {
	opts := &graph.ListObjectsOptions{
		Type:  TypeChange,
		Limit: 100,
	}
	if status != "" {
		opts.PropertyFilters = []graph.PropertyFilter{
			{Path: "status", Op: "eq", Value: status},
		}
	}
	objs, err := c.ListObjects(ctx, opts)
	if err != nil {
		return nil, err
	}
	changes := make([]*Change, 0, len(objs))
	for _, obj := range objs {
		ch, err := fromProps[Change](obj)
		if err != nil {
			return nil, err
		}
		ch.ID = obj.ID
		changes = append(changes, ch)
	}
	return changes, nil
}

// --- Proposal ---

// CreateProposal creates a Proposal and links it to a Change.
func (c *Client) CreateProposal(ctx context.Context, changeID string, p *Proposal) (*Proposal, error) {
	props, err := toProps(p)
	if err != nil {
		return nil, err
	}
	obj, err := c.CreateObject(ctx, TypeProposal, nil, props, p.Tags)
	if err != nil {
		return nil, err
	}
	// Link to change
	if _, err := c.CreateRelationship(ctx, RelHasProposal, changeID, obj.ID, nil); err != nil {
		return nil, fmt.Errorf("linking proposal to change: %w", err)
	}
	result, err := fromProps[Proposal](obj)
	if err != nil {
		return nil, err
	}
	result.ID = obj.ID
	return result, nil
}

// --- Spec ---

// CreateSpec creates a Spec and links it to a Change.
func (c *Client) CreateSpec(ctx context.Context, changeID string, s *Spec) (*Spec, error) {
	props, err := toProps(s)
	if err != nil {
		return nil, err
	}
	obj, err := c.CreateObject(ctx, TypeSpec, strPtr(s.Name), props, s.Tags)
	if err != nil {
		return nil, err
	}
	if _, err := c.CreateRelationship(ctx, RelHasSpec, changeID, obj.ID, nil); err != nil {
		return nil, fmt.Errorf("linking spec to change: %w", err)
	}
	result, err := fromProps[Spec](obj)
	if err != nil {
		return nil, err
	}
	result.ID = obj.ID
	return result, nil
}

// --- Requirement ---

// CreateRequirement creates a Requirement and links it to a Spec.
func (c *Client) CreateRequirement(ctx context.Context, specID string, r *Requirement) (*Requirement, error) {
	props, err := toProps(r)
	if err != nil {
		return nil, err
	}
	obj, err := c.CreateObject(ctx, TypeRequirement, strPtr(r.Name), props, r.Tags)
	if err != nil {
		return nil, err
	}
	if _, err := c.CreateRelationship(ctx, RelHasRequirement, specID, obj.ID, nil); err != nil {
		return nil, fmt.Errorf("linking requirement to spec: %w", err)
	}
	result, err := fromProps[Requirement](obj)
	if err != nil {
		return nil, err
	}
	result.ID = obj.ID
	return result, nil
}

// --- Scenario ---

// CreateScenario creates a Scenario and links it to a Requirement.
func (c *Client) CreateScenario(ctx context.Context, requirementID string, s *Scenario) (*Scenario, error) {
	props, err := toProps(s)
	if err != nil {
		return nil, err
	}
	obj, err := c.CreateObject(ctx, TypeScenario, strPtr(s.Name), props, s.Tags)
	if err != nil {
		return nil, err
	}
	if _, err := c.CreateRelationship(ctx, RelHasScenario, requirementID, obj.ID, nil); err != nil {
		return nil, fmt.Errorf("linking scenario to requirement: %w", err)
	}
	result, err := fromProps[Scenario](obj)
	if err != nil {
		return nil, err
	}
	result.ID = obj.ID
	return result, nil
}

// --- Design ---

// CreateDesign creates a Design and links it to a Change.
func (c *Client) CreateDesign(ctx context.Context, changeID string, d *Design) (*Design, error) {
	props, err := toProps(d)
	if err != nil {
		return nil, err
	}
	obj, err := c.CreateObject(ctx, TypeDesign, nil, props, d.Tags)
	if err != nil {
		return nil, err
	}
	if _, err := c.CreateRelationship(ctx, RelHasDesign, changeID, obj.ID, nil); err != nil {
		return nil, fmt.Errorf("linking design to change: %w", err)
	}
	result, err := fromProps[Design](obj)
	if err != nil {
		return nil, err
	}
	result.ID = obj.ID
	return result, nil
}

// --- Task ---

// CreateTask creates a Task and links it to a Change.
func (c *Client) CreateTask(ctx context.Context, changeID string, t *Task) (*Task, error) {
	props, err := toProps(t)
	if err != nil {
		return nil, err
	}
	obj, err := c.CreateObject(ctx, TypeTask, strPtr(t.Number), props, t.Tags)
	if err != nil {
		return nil, err
	}
	if _, err := c.CreateRelationship(ctx, RelHasTask, changeID, obj.ID, nil); err != nil {
		return nil, fmt.Errorf("linking task to change: %w", err)
	}
	result, err := fromProps[Task](obj)
	if err != nil {
		return nil, err
	}
	result.ID = obj.ID
	result.CanonicalID = obj.CanonicalID
	return result, nil
}

// GetTask retrieves a Task by ID.
func (c *Client) GetTask(ctx context.Context, id string) (*Task, error) {
	obj, err := c.GetObject(ctx, id)
	if err != nil {
		return nil, err
	}
	result, err := fromProps[Task](obj)
	if err != nil {
		return nil, err
	}
	result.ID = obj.ID
	result.CanonicalID = obj.CanonicalID
	return result, nil
}

// UpdateTaskStatus updates a task's status and timestamps.
func (c *Client) UpdateTaskStatus(ctx context.Context, taskID, status string, props map[string]any) (*Task, error) {
	if props == nil {
		props = make(map[string]any)
	}
	props["status"] = status
	obj, err := c.UpdateObject(ctx, taskID, props, nil)
	if err != nil {
		return nil, err
	}
	result, err := fromProps[Task](obj)
	if err != nil {
		return nil, err
	}
	result.ID = obj.ID
	result.CanonicalID = obj.CanonicalID
	return result, nil
}

// ListTasks lists tasks for a change by expanding the has_task relationship.
func (c *Client) ListTasks(ctx context.Context, changeID string) ([]*Task, error) {
	rels, err := c.ListRelationships(ctx, &graph.ListRelationshipsOptions{
		Type:  RelHasTask,
		SrcID: changeID,
		Limit: 200,
	})
	if err != nil {
		return nil, err
	}
	if len(rels) == 0 {
		return nil, nil
	}
	// Batch-fetch all task objects
	ids := make([]string, len(rels))
	for i, rel := range rels {
		ids[i] = rel.DstID
	}
	objs, err := c.GetObjects(ctx, ids)
	if err != nil {
		return nil, err
	}
	tasks := make([]*Task, 0, len(objs))
	for _, obj := range objs {
		t, err := fromProps[Task](obj)
		if err != nil {
			return nil, err
		}
		t.ID = obj.ID
		t.CanonicalID = obj.CanonicalID
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// --- Generic entity retrieval by expand ---

// GetEntityWithRelationships retrieves an entity and its immediate relationships.
func (c *Client) GetEntityWithRelationships(ctx context.Context, entityID string, relTypes []string, depth int) (*graph.GraphExpandResponse, error) {
	if depth <= 0 {
		depth = 1
	}
	return c.ExpandGraph(ctx, &graph.GraphExpandRequest{
		RootIDs:                       []string{entityID},
		Direction:                     "both",
		MaxDepth:                      depth,
		MaxNodes:                      100,
		MaxEdges:                      200,
		RelationshipTypes:             relTypes,
		IncludeRelationshipProperties: true,
	})
}

// GetRelatedObjects gets objects related to a source via a specific relationship type.
func (c *Client) GetRelatedObjects(ctx context.Context, srcID, relType string) ([]*graph.GraphObject, error) {
	rels, err := c.ListRelationships(ctx, &graph.ListRelationshipsOptions{
		Type:  relType,
		SrcID: srcID,
		Limit: 100,
	})
	if err != nil {
		return nil, err
	}
	if len(rels) == 0 {
		return nil, nil
	}
	// Batch-fetch all destination objects
	ids := make([]string, len(rels))
	for i, rel := range rels {
		ids[i] = rel.DstID
	}
	return c.GetObjects(ctx, ids)
}

// GetReverseRelatedObjects gets objects that point to the target via a specific relationship type.
func (c *Client) GetReverseRelatedObjects(ctx context.Context, dstID, relType string) ([]*graph.GraphObject, error) {
	rels, err := c.ListRelationships(ctx, &graph.ListRelationshipsOptions{
		Type:  relType,
		DstID: dstID,
		Limit: 100,
	})
	if err != nil {
		return nil, err
	}
	if len(rels) == 0 {
		return nil, nil
	}
	// Batch-fetch all source objects
	ids := make([]string, len(rels))
	for i, rel := range rels {
		ids[i] = rel.SrcID
	}
	return c.GetObjects(ctx, ids)
}

// HasRelationship checks if a relationship of the given type exists between src and dst.
func (c *Client) HasRelationship(ctx context.Context, relType, srcID, dstID string) (bool, error) {
	rels, err := c.ListRelationships(ctx, &graph.ListRelationshipsOptions{
		Type:  relType,
		SrcID: srcID,
		DstID: dstID,
		Limit: 1,
	})
	if err != nil {
		return false, err
	}
	return len(rels) > 0, nil
}
