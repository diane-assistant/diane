// Package patterns implements the SpecMCP pattern tools:
// spec_suggest_patterns, spec_apply_pattern.
package patterns

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/diane-assistant/diane/server/specmcp/internal/emergent"
	"github.com/diane-assistant/diane/server/specmcp/internal/mcp"
	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"
)

// --- spec_suggest_patterns ---

type suggestPatternsParams struct {
	EntityID   string `json:"entity_id"`
	EntityType string `json:"entity_type,omitempty"`
}

type SuggestPatterns struct {
	client *emergent.Client
}

func NewSuggestPatterns(client *emergent.Client) *SuggestPatterns {
	return &SuggestPatterns{client: client}
}

func (t *SuggestPatterns) Name() string { return "spec_suggest_patterns" }
func (t *SuggestPatterns) Description() string {
	return "Suggest applicable patterns for an entity based on its type, relationships, and the patterns used by similar entities in the graph."
}
func (t *SuggestPatterns) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "entity_id": {
      "type": "string",
      "description": "ID of the entity to suggest patterns for"
    },
    "entity_type": {
      "type": "string",
      "description": "Type of the entity (optional, auto-detected from ID)"
    }
  },
  "required": ["entity_id"]
}`)
}

func (t *SuggestPatterns) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p suggestPatternsParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}
	if p.EntityID == "" {
		return mcp.ErrorResult("entity_id is required"), nil
	}

	// Get the target entity
	obj, err := t.client.GetObject(ctx, p.EntityID)
	if err != nil {
		return mcp.ErrorResult(fmt.Sprintf("entity not found: %v", err)), nil
	}

	entityType := p.EntityType
	if entityType == "" {
		entityType = obj.Type
	}

	// Get patterns already used by this entity
	usedPatterns := make(map[string]bool)
	usedRels, err := t.client.ListRelationships(ctx, &graph.ListRelationshipsOptions{
		Type:  emergent.RelUsesPattern,
		SrcID: p.EntityID,
		Limit: 50,
	})
	if err == nil {
		for _, rel := range usedRels {
			usedPatterns[rel.DstID] = true
		}
	}

	// Find patterns used by other entities of the same type
	sameTypeEntities, err := t.client.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  entityType,
		Limit: 20,
	})
	if err != nil {
		return nil, fmt.Errorf("listing same-type entities: %w", err)
	}

	// Count pattern usage across similar entities using batch edge lookups
	patternUsage := make(map[string]int)
	for _, entity := range sameTypeEntities {
		if entity.ID == p.EntityID {
			continue
		}
		edges, err := t.client.GetObjectEdges(ctx, entity.ID, &graph.GetObjectEdgesOptions{
			Types:     []string{emergent.RelUsesPattern},
			Direction: "outgoing",
		})
		if err != nil {
			continue
		}
		for _, rel := range edges.Outgoing {
			patternUsage[rel.DstID]++
		}
	}

	// Also check if any constitution requires patterns for this entity type
	constitutions, err := t.client.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  emergent.TypeConstitution,
		Limit: 10,
	})
	if err == nil {
		for _, c := range constitutions {
			edges, err := t.client.GetObjectEdges(ctx, c.ID, &graph.GetObjectEdgesOptions{
				Types:     []string{emergent.RelRequiresPattern},
				Direction: "outgoing",
			})
			if err != nil {
				continue
			}
			for _, rel := range edges.Outgoing {
				patternUsage[rel.DstID] += 10 // boost required patterns
			}
		}
	}

	// Batch-fetch all discovered pattern objects in a single call.
	// Use ObjectIndex for dual-indexed lookup (ID/CanonicalID) to handle
	// the case where GetObjectEdges returns a different ID variant than GetObjects.
	patternIDs := make([]string, 0, len(patternUsage))
	for pid := range patternUsage {
		if !usedPatterns[pid] {
			patternIDs = append(patternIDs, pid)
		}
	}
	var patternIdx emergent.ObjectIndex
	if len(patternIDs) > 0 {
		objs, err := t.client.GetObjects(ctx, patternIDs)
		if err == nil {
			patternIdx = emergent.NewObjectIndex(objs)
		}
	}
	if patternIdx == nil {
		patternIdx = make(emergent.ObjectIndex)
	}

	// Build suggestions, excluding already-used patterns
	suggestions := make([]map[string]any, 0)
	for patternID, usage := range patternUsage {
		if usedPatterns[patternID] {
			continue
		}
		info := patternIdx[patternID]
		if info == nil {
			continue
		}
		entry := map[string]any{
			"id":    patternID,
			"usage": usage,
		}
		if info.Key != nil {
			entry["name"] = *info.Key
		}
		if info.Properties != nil {
			for _, k := range []string{"type", "scope", "description", "usage_guidance"} {
				if v, ok := info.Properties[k]; ok {
					entry[k] = v
				}
			}
		}
		if usage >= 10 {
			entry["reason"] = "required by constitution"
		} else {
			entry["reason"] = fmt.Sprintf("used by %d similar %s entities", usage, entityType)
		}
		suggestions = append(suggestions, entry)
	}

	return mcp.JSONResult(map[string]any{
		"entity_id":     p.EntityID,
		"entity_type":   entityType,
		"suggestions":   suggestions,
		"already_using": len(usedPatterns),
		"count":         len(suggestions),
	})
}

// --- spec_apply_pattern ---

type applyPatternParams struct {
	EntityID  string `json:"entity_id"`
	PatternID string `json:"pattern_id"`
}

type ApplyPattern struct {
	client *emergent.Client
}

func NewApplyPattern(client *emergent.Client) *ApplyPattern {
	return &ApplyPattern{client: client}
}

func (t *ApplyPattern) Name() string { return "spec_apply_pattern" }
func (t *ApplyPattern) Description() string {
	return "Apply a pattern to an entity by creating a uses_pattern relationship. Returns the pattern's example code and usage guidance."
}
func (t *ApplyPattern) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "entity_id": {"type": "string", "description": "ID of the entity to apply the pattern to"},
    "pattern_id": {"type": "string", "description": "ID of the pattern to apply"}
  },
  "required": ["entity_id", "pattern_id"]
}`)
}

func (t *ApplyPattern) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p applyPatternParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}
	if p.EntityID == "" || p.PatternID == "" {
		return mcp.ErrorResult("entity_id and pattern_id are required"), nil
	}

	// Verify both exist
	entity, err := t.client.GetObject(ctx, p.EntityID)
	if err != nil {
		return mcp.ErrorResult(fmt.Sprintf("entity not found: %v", err)), nil
	}
	pattern, err := t.client.GetObject(ctx, p.PatternID)
	if err != nil {
		return mcp.ErrorResult(fmt.Sprintf("pattern not found: %v", err)), nil
	}

	// Check if already applied
	already, err := t.client.HasRelationship(ctx, emergent.RelUsesPattern, p.EntityID, p.PatternID)
	if err != nil {
		return nil, fmt.Errorf("checking existing relationship: %w", err)
	}
	if already {
		return mcp.ErrorResult("pattern is already applied to this entity"), nil
	}

	// Create the relationship
	if _, err := t.client.CreateRelationship(ctx, emergent.RelUsesPattern, p.EntityID, p.PatternID, nil); err != nil {
		return nil, fmt.Errorf("creating uses_pattern relationship: %w", err)
	}

	result := map[string]any{
		"entity_id":   p.EntityID,
		"entity_type": entity.Type,
		"pattern_id":  p.PatternID,
		"message":     "Pattern applied",
	}
	if pattern.Key != nil {
		result["pattern_name"] = *pattern.Key
	}
	if pattern.Properties != nil {
		if code, ok := pattern.Properties["example_code"]; ok {
			result["example_code"] = code
		}
		if guidance, ok := pattern.Properties["usage_guidance"]; ok {
			result["usage_guidance"] = guidance
		}
	}

	return mcp.JSONResult(result)
}
