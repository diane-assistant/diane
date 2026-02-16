package patterns

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/diane-assistant/diane/server/specmcp/internal/emergent"
	"github.com/diane-assistant/diane/server/specmcp/internal/mcp"
)

// patternSeed defines a pre-built pattern from the standard library.
type patternSeed struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	Scope         string `json:"scope"`
	Description   string `json:"description"`
	ExampleCode   string `json:"example_code"`
	UsageGuidance string `json:"usage_guidance"`
}

// standardPatterns is the built-in pattern library.
var standardPatterns = []patternSeed{
	// Naming patterns
	{
		Name:          "kebab-case-files",
		Type:          "naming",
		Scope:         "system",
		Description:   "Use kebab-case for all file names (e.g., user-profile.tsx, api-client.go)",
		UsageGuidance: "Apply to all new files. Rename existing files during refactoring.",
	},
	{
		Name:          "pascal-case-components",
		Type:          "naming",
		Scope:         "component",
		Description:   "Use PascalCase for component names matching their file name (e.g., UserProfile in user-profile.tsx)",
		UsageGuidance: "All React/SwiftUI/UI components must use PascalCase for their exported name.",
	},
	{
		Name:          "camel-case-functions",
		Type:          "naming",
		Scope:         "module",
		Description:   "Use camelCase for function and method names",
		UsageGuidance: "All exported functions should use camelCase (JS/TS) or follow language convention.",
	},

	// Structural patterns
	{
		Name:          "barrel-exports",
		Type:          "structural",
		Scope:         "module",
		Description:   "Use index.ts barrel files to re-export public API from directories",
		ExampleCode:   "// src/components/index.ts\nexport { UserProfile } from './user-profile';\nexport { Navigation } from './navigation';",
		UsageGuidance: "Create an index.ts in each feature directory that re-exports the public API.",
	},
	{
		Name:          "colocation",
		Type:          "structural",
		Scope:         "module",
		Description:   "Colocate related files (component, styles, tests, types) in the same directory",
		UsageGuidance: "Keep component.tsx, component.test.tsx, component.types.ts, and component.css together.",
	},
	{
		Name:          "feature-folders",
		Type:          "structural",
		Scope:         "system",
		Description:   "Organize code by feature/domain rather than by file type",
		UsageGuidance: "Use src/features/<feature-name>/ instead of src/components/, src/hooks/, etc.",
	},
	{
		Name:          "separation-of-concerns",
		Type:          "structural",
		Scope:         "component",
		Description:   "Separate business logic from presentation. Use hooks/services for logic, components for rendering.",
		ExampleCode:   "// useUserProfile.ts - logic\nconst useUserProfile = (id) => { ... };\n\n// UserProfile.tsx - presentation\nconst UserProfile = ({ id }) => {\n  const { data } = useUserProfile(id);\n  return <div>{data.name}</div>;\n};",
		UsageGuidance: "Extract business logic into custom hooks or service modules. Components should only handle rendering.",
	},

	// Behavioral patterns
	{
		Name:          "error-boundary",
		Type:          "behavioral",
		Scope:         "component",
		Description:   "Wrap major UI sections in error boundaries to prevent cascading failures",
		ExampleCode:   "<ErrorBoundary fallback={<ErrorFallback />}>\n  <UserProfile />\n</ErrorBoundary>",
		UsageGuidance: "Add error boundaries at route level and around critical interactive sections.",
	},
	{
		Name:          "optimistic-updates",
		Type:          "behavioral",
		Scope:         "component",
		Description:   "Update UI immediately on user action, then reconcile with server response",
		UsageGuidance: "Use for actions where perceived speed matters (likes, toggles, inline edits). Always handle rollback on error.",
	},
	{
		Name:          "loading-states",
		Type:          "behavioral",
		Scope:         "component",
		Description:   "Show appropriate loading indicators for all async operations",
		UsageGuidance: "Use skeleton screens for initial loads, spinners for actions, progress bars for uploads.",
	},
	{
		Name:          "retry-with-backoff",
		Type:          "behavioral",
		Scope:         "module",
		Description:   "Retry failed network requests with exponential backoff",
		ExampleCode:   "func retry(fn func() error, maxAttempts int) error {\n  for i := 0; i < maxAttempts; i++ {\n    if err := fn(); err == nil { return nil }\n    time.Sleep(time.Duration(1<<i) * time.Second)\n  }\n  return fmt.Errorf(\"max retries exceeded\")\n}",
		UsageGuidance: "Apply to all external API calls. Use 3 retries with 1s, 2s, 4s backoff. Don't retry 4xx errors.",
	},

	// Error handling patterns
	{
		Name:          "error-wrapping",
		Type:          "error_handling",
		Scope:         "system",
		Description:   "Wrap errors with context at each layer boundary for clear error chains",
		ExampleCode:   "if err != nil {\n  return fmt.Errorf(\"creating user: %w\", err)\n}",
		UsageGuidance: "Always wrap errors at function boundaries. Include the operation name. Use %w for Go, cause chains for JS.",
	},
	{
		Name:          "graceful-degradation",
		Type:          "error_handling",
		Scope:         "component",
		Description:   "Degrade gracefully when a non-critical feature fails rather than showing an error",
		UsageGuidance: "Hide optional features when their data fails to load. Show cached data when refresh fails.",
	},
	{
		Name:          "structured-logging",
		Type:          "error_handling",
		Scope:         "system",
		Description:   "Use structured logging with consistent fields for all log entries",
		ExampleCode:   "logger.Error(\"request failed\",\n  \"method\", r.Method,\n  \"path\", r.URL.Path,\n  \"status\", status,\n  \"error\", err,\n)",
		UsageGuidance: "Always include: operation, relevant IDs, error. Use slog or equivalent structured logger.",
	},
	{
		Name:          "input-validation",
		Type:          "error_handling",
		Scope:         "module",
		Description:   "Validate all external inputs at system boundaries before processing",
		UsageGuidance: "Validate at API handlers, form submissions, and config loading. Return clear validation error messages.",
	},
}

// --- spec_seed_patterns ---

type seedPatternsParams struct {
	Types []string `json:"types,omitempty"`
	Force bool     `json:"force,omitempty"`
}

// SeedPatterns creates standard patterns from the built-in pattern library.
type SeedPatterns struct {
	client *emergent.Client
}

func NewSeedPatterns(client *emergent.Client) *SeedPatterns {
	return &SeedPatterns{client: client}
}

func (t *SeedPatterns) Name() string { return "spec_seed_patterns" }
func (t *SeedPatterns) Description() string {
	return "Seed the graph with standard patterns from the built-in pattern library. Includes naming, structural, behavioral, and error_handling patterns. Skips patterns that already exist by name (unless force=true)."
}
func (t *SeedPatterns) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "types": {
      "type": "array",
      "items": {"type": "string", "enum": ["naming", "structural", "behavioral", "error_handling"]},
      "description": "Filter to specific pattern types (default: all)"
    },
    "force": {
      "type": "boolean",
      "description": "If true, recreate patterns even if they already exist (default: false)"
    }
  }
}`)
}

func (t *SeedPatterns) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p seedPatternsParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}

	// Build filter set
	typeFilter := make(map[string]bool)
	for _, typ := range p.Types {
		typeFilter[typ] = true
	}

	created := make([]map[string]any, 0)
	skipped := make([]string, 0)

	for _, seed := range standardPatterns {
		// Apply type filter
		if len(typeFilter) > 0 && !typeFilter[seed.Type] {
			continue
		}

		// Check if pattern already exists
		if !p.Force {
			existing, err := t.client.FindByTypeAndKey(ctx, emergent.TypePattern, seed.Name)
			if err == nil && existing != nil {
				skipped = append(skipped, seed.Name)
				continue
			}
		}

		// Create the pattern
		props := map[string]any{
			"name":        seed.Name,
			"type":        seed.Type,
			"scope":       seed.Scope,
			"description": seed.Description,
		}
		if seed.ExampleCode != "" {
			props["example_code"] = seed.ExampleCode
		}
		if seed.UsageGuidance != "" {
			props["usage_guidance"] = seed.UsageGuidance
		}

		key := seed.Name
		obj, err := t.client.CreateObject(ctx, emergent.TypePattern, &key, props, []string{
			fmt.Sprintf("pattern:%s", seed.Type),
			fmt.Sprintf("scope:%s", seed.Scope),
		})
		if err != nil {
			return nil, fmt.Errorf("creating pattern %q: %w", seed.Name, err)
		}

		created = append(created, map[string]any{
			"id":    obj.ID,
			"name":  seed.Name,
			"type":  seed.Type,
			"scope": seed.Scope,
		})
	}

	return mcp.JSONResult(map[string]any{
		"created":       created,
		"created_count": len(created),
		"skipped":       skipped,
		"skipped_count": len(skipped),
		"message":       fmt.Sprintf("Seeded %d patterns, skipped %d existing", len(created), len(skipped)),
	})
}
