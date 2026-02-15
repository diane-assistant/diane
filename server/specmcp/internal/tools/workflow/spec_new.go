// Package workflow implements the SpecMCP workflow tools:
// spec_new, spec_artifact, and spec_archive.
package workflow

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/diane-assistant/diane/server/specmcp/internal/emergent"
	"github.com/diane-assistant/diane/server/specmcp/internal/guards"
	"github.com/diane-assistant/diane/server/specmcp/internal/mcp"
)

// specNewParams defines the input for spec_new.
type specNewParams struct {
	Name   string   `json:"name"`
	Intent string   `json:"intent"`
	Scope  string   `json:"scope,omitempty"`
	Impact string   `json:"impact,omitempty"`
	Tags   []string `json:"tags,omitempty"`
	Force  bool     `json:"force,omitempty"`
}

// SpecNew creates a new Change with its Proposal.
type SpecNew struct {
	client *emergent.Client
	runner *guards.Runner
}

// NewSpecNew creates a SpecNew tool.
func NewSpecNew(client *emergent.Client) *SpecNew {
	return &SpecNew{
		client: client,
		runner: guards.NewRunner(),
	}
}

func (t *SpecNew) Name() string { return "spec_new" }

func (t *SpecNew) Description() string {
	return "Create a new change with its proposal. Runs pre-change guards to check for constitution, patterns, and project context. Use force=true to override soft blocks."
}

func (t *SpecNew) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "name": {
      "type": "string",
      "description": "Unique name for the change in kebab-case (e.g. 'add-user-permissions')"
    },
    "intent": {
      "type": "string",
      "description": "Why this change is needed"
    },
    "scope": {
      "type": "string",
      "description": "What areas of the system will change"
    },
    "impact": {
      "type": "string",
      "description": "Expected impact on existing systems"
    },
    "tags": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Optional tags for the change (e.g. 'domain:auth', 'priority:high')"
    },
    "force": {
      "type": "boolean",
      "description": "Override soft blocks (e.g. missing constitution or patterns). Default: false"
    }
  },
  "required": ["name", "intent"]
}`)
}

func (t *SpecNew) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p specNewParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}

	if p.Name == "" {
		return mcp.ErrorResult("name is required"), nil
	}
	if p.Intent == "" {
		return mcp.ErrorResult("intent is required"), nil
	}

	// Check for duplicate active change (this stays as a direct check — it's not a guard
	// because it needs change-specific lookup rather than GuardContext state).
	existing, err := t.client.FindChange(ctx, p.Name)
	if err != nil {
		return nil, fmt.Errorf("checking for existing change: %w", err)
	}
	if existing != nil && existing.Status != emergent.StatusArchived {
		return mcp.ErrorResult(fmt.Sprintf(
			"change %q already exists with status %q — consider continuing that change instead of creating a new one",
			p.Name, existing.Status,
		)), nil
	}

	// Build guard context and populate project state
	gctx := &guards.GuardContext{
		ChangeName: p.Name,
		Force:      p.Force,
	}
	if err := guards.PopulateProjectState(ctx, t.client, gctx); err != nil {
		return nil, fmt.Errorf("populating project state for guards: %w", err)
	}

	// Run pre-change guards
	outcome := t.runner.Run(ctx, gctx, guards.NewChangeGuards())
	if outcome.Blocked {
		return mcp.ErrorResult(outcome.FormatBlockMessage()), nil
	}

	// Create the Change
	change, err := t.client.CreateChange(ctx, &emergent.Change{
		Name:   p.Name,
		Status: emergent.StatusActive,
		Tags:   p.Tags,
	})
	if err != nil {
		return nil, fmt.Errorf("creating change: %w", err)
	}

	// Create the Proposal linked to the Change
	proposal, err := t.client.CreateProposal(ctx, change.ID, &emergent.Proposal{
		Intent: p.Intent,
		Scope:  p.Scope,
		Impact: p.Impact,
		Tags:   p.Tags,
	})
	if err != nil {
		return nil, fmt.Errorf("creating proposal: %w", err)
	}

	result := map[string]any{
		"change": map[string]any{
			"id":     change.ID,
			"name":   change.Name,
			"status": change.Status,
		},
		"proposal": map[string]any{
			"id":     proposal.ID,
			"intent": proposal.Intent,
			"scope":  proposal.Scope,
			"impact": proposal.Impact,
		},
		"message": fmt.Sprintf("Created change %q with proposal", p.Name),
	}

	// Include advisory messages (warnings/suggestions) if any
	if advisory := outcome.FormatAdvisoryMessage(); advisory != "" {
		result["advisories"] = advisory
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.TextContent(string(b))},
	}, nil
}
