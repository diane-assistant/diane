package query

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/diane-assistant/diane/server/specmcp/internal/emergent"
	"github.com/diane-assistant/diane/server/specmcp/internal/mcp"
	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"
)

// --- spec_search ---

// Default types to search when none specified.
var defaultSearchTypes = []string{
	emergent.TypePattern,
	emergent.TypeContext,
	emergent.TypeUIComponent,
	emergent.TypeAction,
	emergent.TypeChange,
	emergent.TypeSpec,
	emergent.TypeTask,
	emergent.TypeActor,
	emergent.TypeCodingAgent,
	emergent.TypeRequirement,
	emergent.TypeScenario,
	emergent.TypeDesign,
	emergent.TypeTestCase,
	emergent.TypeAPIContract,
}

type searchParams struct {
	Query  string   `json:"query"`
	Types  []string `json:"types,omitempty"`
	Labels []string `json:"labels,omitempty"`
	Limit  int      `json:"limit,omitempty"`
}

// Search performs full-text search across all graph entities.
// Uses Emergent FTS when available; falls back to client-side property matching.
type Search struct {
	client *emergent.Client
}

func NewSearch(client *emergent.Client) *Search {
	return &Search{client: client}
}

func (t *Search) Name() string { return "spec_search" }
func (t *Search) Description() string {
	return "Search the knowledge graph using full-text search. Find entities by keyword across names, descriptions, and all properties. Filter by entity type and labels."
}
func (t *Search) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "query": {
      "type": "string",
      "description": "Search query text (e.g. 'SSE streaming', 'error handling', 'sidebar navigation')"
    },
    "types": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Filter to specific entity types (e.g. ['Pattern', 'Context', 'UIComponent']). Valid types: Pattern, Context, UIComponent, Action, Change, Spec, Task, Actor, CodingAgent, Requirement, Scenario, Design, TestCase, APIContract"
    },
    "labels": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Filter to entities with specific labels/tags"
    },
    "limit": {
      "type": "integer",
      "description": "Max results to return (default: 20, max: 100)",
      "default": 20
    }
  },
  "required": ["query"]
}`)
}

func (t *Search) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p searchParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}

	if p.Query == "" {
		return mcp.ErrorResult("query is required"), nil
	}

	limit := p.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// Try FTS first
	resp, err := t.client.FTSSearch(ctx, &graph.FTSSearchOptions{
		Query:  p.Query,
		Types:  p.Types,
		Labels: p.Labels,
		Limit:  limit,
	})

	if err == nil && resp.Total > 0 {
		// FTS worked — return results
		results := make([]map[string]any, 0, len(resp.Data))
		for _, item := range resp.Data {
			results = append(results, buildSearchResult(item.Object, item.Score))
		}
		return jsonResult(map[string]any{
			"results": results,
			"total":   resp.Total,
			"count":   len(results),
			"query":   p.Query,
			"method":  "fts",
		})
	}

	// FTS returned empty or failed — fall back to client-side search
	return t.fallbackSearch(ctx, p.Query, p.Types, p.Labels, limit)
}

// fallbackSearch lists objects by type and filters by checking if the query
// appears in the key or any string property value (case-insensitive).
func (t *Search) fallbackSearch(ctx context.Context, queryStr string, types, labels []string, limit int) (*mcp.ToolsCallResult, error) {
	if len(types) == 0 {
		types = defaultSearchTypes
	}

	queryLower := strings.ToLower(queryStr)
	queryTerms := strings.Fields(queryLower)

	var results []map[string]any

	for _, typeName := range types {
		if len(results) >= limit {
			break
		}

		opts := &graph.ListObjectsOptions{
			Type:   typeName,
			Labels: labels,
			Limit:  200, // fetch a batch per type
		}

		// If query looks like a single word, try using the "contains" property filter
		// on name field first for efficiency
		if len(queryTerms) == 1 {
			opts.PropertyFilters = []graph.PropertyFilter{
				{Path: "name", Op: "contains", Value: queryLower},
			}
		}

		objs, err := t.client.ListObjects(ctx, opts)
		if err != nil {
			// If property filter fails (some backends don't support 'contains'),
			// retry without it
			opts.PropertyFilters = nil
			objs, err = t.client.ListObjects(ctx, opts)
			if err != nil {
				continue // skip this type on error
			}
		}

		for _, obj := range objs {
			if len(results) >= limit {
				break
			}
			if matchesQuery(obj, queryTerms) {
				results = append(results, buildSearchResult(obj, 0))
			}
		}

		// If we used the name filter and got results, also try without it
		// to catch matches in other fields
		if len(queryTerms) == 1 && len(results) < limit {
			opts.PropertyFilters = nil
			objs2, err := t.client.ListObjects(ctx, opts)
			if err == nil {
				seen := make(map[string]bool, len(results))
				for _, r := range results {
					if id, ok := r["id"].(string); ok {
						seen[id] = true
					}
				}
				for _, obj := range objs2 {
					if len(results) >= limit {
						break
					}
					if seen[obj.ID] {
						continue
					}
					if matchesQuery(obj, queryTerms) {
						results = append(results, buildSearchResult(obj, 0))
					}
				}
			}
		}
	}

	return jsonResult(map[string]any{
		"results": results,
		"total":   len(results),
		"count":   len(results),
		"query":   queryStr,
		"method":  "fallback",
	})
}

// matchesQuery checks if any query term appears in the object's key or string properties.
func matchesQuery(obj *graph.GraphObject, queryTerms []string) bool {
	// Check key
	if obj.Key != nil {
		keyLower := strings.ToLower(*obj.Key)
		if allTermsMatch(keyLower, queryTerms) {
			return true
		}
	}

	// Check string properties
	for _, v := range obj.Properties {
		if s, ok := v.(string); ok {
			sLower := strings.ToLower(s)
			if allTermsMatch(sLower, queryTerms) {
				return true
			}
		}
	}

	// Check labels
	for _, label := range obj.Labels {
		labelLower := strings.ToLower(label)
		if allTermsMatch(labelLower, queryTerms) {
			return true
		}
	}

	return false
}

// allTermsMatch returns true if all query terms appear in the text.
func allTermsMatch(text string, terms []string) bool {
	for _, term := range terms {
		if !strings.Contains(text, term) {
			return false
		}
	}
	return true
}

// buildSearchResult creates a standardized result entry from a graph object.
func buildSearchResult(obj *graph.GraphObject, score float32) map[string]any {
	entry := map[string]any{
		"id":         obj.ID,
		"type":       obj.Type,
		"properties": obj.Properties,
		"labels":     obj.Labels,
	}
	if obj.Key != nil {
		entry["name"] = *obj.Key
	}
	if score > 0 {
		entry["score"] = score
	}
	return entry
}
