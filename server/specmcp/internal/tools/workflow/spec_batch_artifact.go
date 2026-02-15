package workflow

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/diane-assistant/diane/server/specmcp/internal/mcp"
)

// specBatchArtifactParams defines the input for spec_batch_artifact.
type specBatchArtifactParams struct {
	ChangeID  string          `json:"change_id"`
	Artifacts []batchArtifact `json:"artifacts"`
}

type batchArtifact struct {
	ArtifactType string         `json:"artifact_type"`
	Content      map[string]any `json:"content"`
}

// SpecBatchArtifact adds multiple artifacts to a change in a single call.
type SpecBatchArtifact struct {
	single *SpecArtifact
}

// NewSpecBatchArtifact creates a SpecBatchArtifact tool.
func NewSpecBatchArtifact(single *SpecArtifact) *SpecBatchArtifact {
	return &SpecBatchArtifact{single: single}
}

func (t *SpecBatchArtifact) Name() string { return "spec_batch_artifact" }

func (t *SpecBatchArtifact) Description() string {
	return "Add multiple artifacts to a change in a single call. Accepts an array of artifacts, each with artifact_type and content. Returns results for each artifact. Stops on first error."
}

func (t *SpecBatchArtifact) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "change_id": {
      "type": "string",
      "description": "ID of the change to add artifacts to"
    },
    "artifacts": {
      "type": "array",
      "description": "Array of artifacts to add",
      "items": {
        "type": "object",
        "properties": {
          "artifact_type": {
            "type": "string",
            "description": "Type of artifact",
            "enum": ["proposal", "spec", "design", "task", "actor", "coding_agent", "pattern", "test_case", "api_contract", "context", "ui_component", "action", "requirement", "scenario", "scenario_step", "constitution"]
          },
          "content": {
            "type": "object",
            "description": "Artifact-specific content"
          }
        },
        "required": ["artifact_type", "content"]
      }
    }
  },
  "required": ["change_id", "artifacts"]
}`)
}

func (t *SpecBatchArtifact) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p specBatchArtifactParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}

	if p.ChangeID == "" {
		return mcp.ErrorResult("change_id is required"), nil
	}
	if len(p.Artifacts) == 0 {
		return mcp.ErrorResult("artifacts array is required and must not be empty"), nil
	}

	results := make([]map[string]any, 0, len(p.Artifacts))
	for i, a := range p.Artifacts {
		// Build the single-artifact params and delegate to the existing tool
		singleParams := specArtifactParams{
			ChangeID:     p.ChangeID,
			ArtifactType: a.ArtifactType,
			Content:      a.Content,
		}
		raw, err := json.Marshal(singleParams)
		if err != nil {
			return nil, fmt.Errorf("marshaling artifact %d: %w", i, err)
		}

		result, err := t.single.Execute(ctx, raw)
		if err != nil {
			return nil, fmt.Errorf("artifact %d (%s %q): %w", i, a.ArtifactType, getString(a.Content, "name"), err)
		}

		// Parse the single result to include in batch results
		var parsed any
		if len(result.Content) > 0 {
			text := result.Content[0].Text
			if err := json.Unmarshal([]byte(text), &parsed); err != nil {
				parsed = text
			}
		}

		entry := map[string]any{
			"index":         i,
			"artifact_type": a.ArtifactType,
			"name":          getString(a.Content, "name"),
			"result":        parsed,
		}

		// Check if the single result was an error
		if result.IsError {
			entry["error"] = true
			results = append(results, entry)
			// Return partial results up to the error
			return jsonResult(map[string]any{
				"completed": i,
				"total":     len(p.Artifacts),
				"error":     fmt.Sprintf("artifact %d failed", i),
				"results":   results,
			})
		}

		results = append(results, entry)
	}

	return jsonResult(map[string]any{
		"completed": len(p.Artifacts),
		"total":     len(p.Artifacts),
		"results":   results,
	})
}
