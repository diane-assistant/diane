// Package sync implements the SpecMCP sync tools:
// spec_sync_status, spec_sync.
package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/diane-assistant/diane/server/specmcp/internal/emergent"
	"github.com/diane-assistant/diane/server/specmcp/internal/mcp"
	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"
)

// --- spec_sync_status ---

type syncStatusParams struct {
	ChangeID string `json:"change_id,omitempty"`
}

type SyncStatus struct {
	client *emergent.Client
}

func NewSyncStatus(client *emergent.Client) *SyncStatus {
	return &SyncStatus{client: client}
}

func (t *SyncStatus) Name() string { return "spec_sync_status" }
func (t *SyncStatus) Description() string {
	return "Get the synchronization status of the graph, showing the last synced commit and timestamp. Optionally scoped to a change."
}
func (t *SyncStatus) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "change_id": {
      "type": "string",
      "description": "Optionally scope to a specific change"
    }
  }
}`)
}

func (t *SyncStatus) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p syncStatusParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}

	// Find the most recent GraphSync entity
	opts := &graph.ListObjectsOptions{
		Type:  emergent.TypeGraphSync,
		Limit: 1,
		Order: "desc",
	}

	objs, err := t.client.ListObjects(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("listing sync records: %w", err)
	}

	if len(objs) == 0 {
		return jsonResult(map[string]any{
			"status":  "never_synced",
			"message": "No sync records found. Run spec_sync to synchronize.",
		})
	}

	syncObj := objs[0]
	result := map[string]any{
		"id":         syncObj.ID,
		"properties": syncObj.Properties,
	}

	if syncObj.Properties != nil {
		result["last_synced_commit"] = syncObj.Properties["last_synced_commit"]
		result["last_synced_at"] = syncObj.Properties["last_synced_at"]
		result["status"] = syncObj.Properties["status"]
	}

	return jsonResult(result)
}

// --- spec_graph_summary ---

type graphSummaryParams struct {
	ChangeID string `json:"change_id,omitempty"`
}

// GraphSummary returns a summary of all entities in the graph, grouped by type.
type GraphSummary struct {
	client *emergent.Client
}

func NewGraphSummary(client *emergent.Client) *GraphSummary {
	return &GraphSummary{client: client}
}

func (t *GraphSummary) Name() string { return "spec_graph_summary" }
func (t *GraphSummary) Description() string {
	return "Get a summary of the graph contents: entity counts by type, relationship counts, and active changes. Useful for understanding the current state of the knowledge graph."
}
func (t *GraphSummary) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "change_id": {
      "type": "string",
      "description": "Optionally scope the summary to entities reachable from a specific change"
    }
  }
}`)
}

func (t *GraphSummary) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p graphSummaryParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}

	entityTypes := []string{
		emergent.TypeChange, emergent.TypeProposal, emergent.TypeSpec,
		emergent.TypeRequirement, emergent.TypeScenario, emergent.TypeScenarioStep,
		emergent.TypeDesign, emergent.TypeTask, emergent.TypeActor,
		emergent.TypeCodingAgent, emergent.TypePattern, emergent.TypeConstitution,
		emergent.TypeTestCase, emergent.TypeAPIContract, emergent.TypeContext,
		emergent.TypeUIComponent, emergent.TypeAction, emergent.TypeDataModel,
		emergent.TypeService, emergent.TypeGraphSync,
	}

	counts := make(map[string]int)
	total := 0

	for _, typeName := range entityTypes {
		count, err := t.client.CountObjects(ctx, typeName)
		if err != nil {
			counts[typeName] = -1 // indicate error
			continue
		}
		counts[typeName] = count
		total += count
	}

	// Get active changes for a quick status overview
	activeChanges := make([]map[string]any, 0)
	changes, err := t.client.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  emergent.TypeChange,
		Limit: 20,
	})
	if err == nil {
		for _, ch := range changes {
			entry := map[string]any{
				"id": ch.ID,
			}
			if ch.Key != nil {
				entry["name"] = *ch.Key
			}
			if ch.Properties != nil {
				if status, ok := ch.Properties["status"]; ok {
					entry["status"] = status
				}
			}
			activeChanges = append(activeChanges, entry)
		}
	}

	return jsonResult(map[string]any{
		"entity_counts":  counts,
		"total_entities": total,
		"changes":        activeChanges,
	})
}

// --- spec_sync ---

type syncParams struct {
	ChangeID string `json:"change_id,omitempty"`
	Commit   string `json:"commit,omitempty"`
	DryRun   bool   `json:"dry_run,omitempty"`
}

type Sync struct {
	client *emergent.Client
}

func NewSync(client *emergent.Client) *Sync {
	return &Sync{client: client}
}

func (t *Sync) Name() string { return "spec_sync" }
func (t *Sync) Description() string {
	return "Record a sync point between the codebase and the graph. Creates or updates a GraphSync entity with the current commit hash and timestamp."
}
func (t *Sync) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "change_id": {
      "type": "string",
      "description": "Optionally scope the sync to a specific change"
    },
    "commit": {
      "type": "string",
      "description": "Git commit hash to record as the sync point"
    },
    "dry_run": {
      "type": "boolean",
      "description": "If true, only report what would be synced without making changes"
    }
  }
}`)
}

func (t *Sync) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p syncParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}

	now := time.Now()

	if p.DryRun {
		return jsonResult(map[string]any{
			"dry_run":       true,
			"commit":        p.Commit,
			"would_sync_at": now.Format(time.RFC3339),
			"message":       "Dry run: no changes made",
		})
	}

	// Create a new GraphSync record
	props := map[string]any{
		"status":         "synced",
		"last_synced_at": now.Format(time.RFC3339),
	}
	if p.Commit != "" {
		props["last_synced_commit"] = p.Commit
	}

	key := fmt.Sprintf("sync-%s", now.Format("20060102-150405"))
	obj, err := t.client.CreateObject(ctx, emergent.TypeGraphSync, &key, props, nil)
	if err != nil {
		return nil, fmt.Errorf("creating sync record: %w", err)
	}

	return jsonResult(map[string]any{
		"id":        obj.ID,
		"commit":    p.Commit,
		"synced_at": now.Format(time.RFC3339),
		"status":    "synced",
		"message":   "Sync point recorded",
	})
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
