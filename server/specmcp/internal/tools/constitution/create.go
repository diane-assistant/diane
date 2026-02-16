package constitution

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/diane-assistant/diane/server/specmcp/internal/emergent"
	"github.com/diane-assistant/diane/server/specmcp/internal/mcp"
)

// --- spec_create_constitution ---

type createParams struct {
	Name                 string   `json:"name"`
	Version              string   `json:"version"`
	Principles           string   `json:"principles,omitempty"`
	Guardrails           []string `json:"guardrails,omitempty"`
	TestingRequirements  string   `json:"testing_requirements,omitempty"`
	SecurityRequirements string   `json:"security_requirements,omitempty"`
	PatternsRequired     []string `json:"patterns_required,omitempty"`
	PatternsForbidden    []string `json:"patterns_forbidden,omitempty"`
	Tags                 []string `json:"tags,omitempty"`
}

// CreateConstitution creates or updates a project Constitution.
// This is a standalone tool that does not require a Change — it is meant
// to bootstrap the project before any changes can be created.
type CreateConstitution struct {
	client *emergent.Client
}

func NewCreateConstitution(client *emergent.Client) *CreateConstitution {
	return &CreateConstitution{client: client}
}

func (t *CreateConstitution) Name() string { return "spec_create_constitution" }
func (t *CreateConstitution) Description() string {
	return "Create or update the project's constitution. A constitution defines project principles, guardrails, testing requirements, and pattern mandates. Must exist before any changes can be created."
}
func (t *CreateConstitution) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "name": {
      "type": "string",
      "description": "Name of the constitution (e.g. 'diane-constitution')"
    },
    "version": {
      "type": "string",
      "description": "Version string (e.g. '1.0.0')"
    },
    "principles": {
      "type": "string",
      "description": "Core architectural and design principles the project follows"
    },
    "guardrails": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Non-negotiable rules that all changes must respect"
    },
    "testing_requirements": {
      "type": "string",
      "description": "Testing standards and coverage expectations"
    },
    "security_requirements": {
      "type": "string",
      "description": "Security standards and requirements"
    },
    "patterns_required": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Pattern names that all entities must use"
    },
    "patterns_forbidden": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Pattern names that no entity may use"
    },
    "tags": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Optional tags"
    }
  },
  "required": ["name", "version"]
}`)
}

func (t *CreateConstitution) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p createParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}
	if p.Name == "" {
		return mcp.ErrorResult("name is required"), nil
	}
	if p.Version == "" {
		return mcp.ErrorResult("version is required"), nil
	}

	// Build properties
	props := map[string]any{
		"name":    p.Name,
		"version": p.Version,
	}
	if p.Principles != "" {
		props["principles"] = p.Principles
	}
	if len(p.Guardrails) > 0 {
		props["guardrails"] = p.Guardrails
	}
	if p.TestingRequirements != "" {
		props["testing_requirements"] = p.TestingRequirements
	}
	if p.SecurityRequirements != "" {
		props["security_requirements"] = p.SecurityRequirements
	}
	if len(p.PatternsRequired) > 0 {
		props["patterns_required"] = p.PatternsRequired
	}
	if len(p.PatternsForbidden) > 0 {
		props["patterns_forbidden"] = p.PatternsForbidden
	}

	// Upsert so re-running updates the existing constitution
	key := p.Name
	obj, err := t.client.UpsertObject(ctx, emergent.TypeConstitution, &key, props, p.Tags)
	if err != nil {
		return nil, fmt.Errorf("creating constitution: %w", err)
	}

	// Link patterns if specified (requires_pattern and forbids_pattern)
	relCount := 0
	for _, patternName := range p.PatternsRequired {
		pattern, err := t.client.FindByTypeAndKey(ctx, emergent.TypePattern, patternName)
		if err != nil || pattern == nil {
			continue // skip patterns that don't exist yet
		}
		// Check if relationship already exists
		exists, err := t.client.HasRelationship(ctx, emergent.RelRequiresPattern, obj.ID, pattern.ID)
		if err != nil || exists {
			continue
		}
		if _, err := t.client.CreateRelationship(ctx, emergent.RelRequiresPattern, obj.ID, pattern.ID, nil); err != nil {
			return nil, fmt.Errorf("creating requires_pattern relationship: %w", err)
		}
		relCount++
	}
	for _, patternName := range p.PatternsForbidden {
		pattern, err := t.client.FindByTypeAndKey(ctx, emergent.TypePattern, patternName)
		if err != nil || pattern == nil {
			continue
		}
		exists, err := t.client.HasRelationship(ctx, emergent.RelForbidsPattern, obj.ID, pattern.ID)
		if err != nil || exists {
			continue
		}
		if _, err := t.client.CreateRelationship(ctx, emergent.RelForbidsPattern, obj.ID, pattern.ID, nil); err != nil {
			return nil, fmt.Errorf("creating forbids_pattern relationship: %w", err)
		}
		relCount++
	}

	return mcp.JSONResult(map[string]any{
		"id":                    obj.ID,
		"name":                  p.Name,
		"version":               p.Version,
		"pattern_relationships": relCount,
		"message":               fmt.Sprintf("Constitution %q (v%s) created. Changes can now be created.", p.Name, p.Version),
	})
}
