package workflow

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/diane-assistant/diane/server/specmcp/internal/emergent"
	"github.com/diane-assistant/diane/server/specmcp/internal/guards"
	"github.com/diane-assistant/diane/server/specmcp/internal/mcp"
)

// specArchiveParams defines the input for spec_archive.
type specArchiveParams struct {
	ChangeID string `json:"change_id"`
	Force    bool   `json:"force,omitempty"`
}

// SpecArchive archives a completed change.
type SpecArchive struct {
	client *emergent.Client
	runner *guards.Runner
}

// NewSpecArchive creates a SpecArchive tool.
func NewSpecArchive(client *emergent.Client) *SpecArchive {
	return &SpecArchive{
		client: client,
		runner: guards.NewRunner(),
	}
}

func (t *SpecArchive) Name() string { return "spec_archive" }

func (t *SpecArchive) Description() string {
	return "Archive a completed change. Runs guards to verify artifact completeness and task completion. Use force=true to override soft blocks."
}

func (t *SpecArchive) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "change_id": {
      "type": "string",
      "description": "ID of the change to archive"
    },
    "force": {
      "type": "boolean",
      "description": "Override soft blocks like incomplete tasks or missing artifacts (default: false)"
    }
  },
  "required": ["change_id"]
}`)
}

func (t *SpecArchive) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p specArchiveParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}

	if p.ChangeID == "" {
		return mcp.ErrorResult("change_id is required"), nil
	}

	// Get the change
	change, err := t.client.GetChange(ctx, p.ChangeID)
	if err != nil {
		return mcp.ErrorResult(fmt.Sprintf("change not found: %v", err)), nil
	}

	if change.Status == emergent.StatusArchived {
		return mcp.ErrorResult("change is already archived"), nil
	}

	// Build guard context and populate change state
	gctx := &guards.GuardContext{
		ChangeID: p.ChangeID,
		Force:    p.Force,
	}
	if err := guards.PopulateChangeState(ctx, t.client, gctx); err != nil {
		return nil, fmt.Errorf("populating change state for guards: %w", err)
	}

	// Run archive guards
	outcome := t.runner.Run(ctx, gctx, guards.ArchiveGuards())
	if outcome.Blocked {
		return mcp.ErrorResult(outcome.FormatBlockMessage()), nil
	}

	// Archive the change
	_, err = t.client.UpdateObject(ctx, p.ChangeID, map[string]any{
		"status": emergent.StatusArchived,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("archiving change: %w", err)
	}

	result := map[string]any{
		"change_id": p.ChangeID,
		"name":      change.Name,
		"status":    emergent.StatusArchived,
		"message":   fmt.Sprintf("Archived change %q", change.Name),
	}

	// Include advisory messages if any
	if advisory := outcome.FormatAdvisoryMessage(); advisory != "" {
		result["advisories"] = advisory
	}

	return jsonResult(result)
}
