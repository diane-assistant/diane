// Command seed registers the SpecMCP template pack with an Emergent project.
//
// Usage:
//
//	EMERGENT_TOKEN=emt_... go run ./scripts/seed.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/diane-assistant/diane/server/specmcp/internal/config"
	sdk "github.com/emergent-company/emergent/apps/server-go/pkg/sdk"
	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/templatepacks"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("seed failed: %v", err)
	}
}

func run() error {
	ctx := context.Background()

	// Load config from environment
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Initialize SDK client
	client, err := sdk.New(sdk.Config{
		ServerURL: cfg.Emergent.URL,
		Auth: sdk.AuthConfig{
			Mode:   "apikey",
			APIKey: cfg.Emergent.Token,
		},
	})
	if err != nil {
		return fmt.Errorf("initializing SDK client: %w", err)
	}

	// Read template pack JSON
	packPath := findPackPath()
	packData, err := os.ReadFile(packPath)
	if err != nil {
		return fmt.Errorf("reading template pack: %w", err)
	}
	log.Printf("loaded template pack from %s (%d bytes)", packPath, len(packData))

	// Parse the JSON to extract fields for CreatePackRequest
	var pack struct {
		Name                    string          `json:"name"`
		Version                 string          `json:"version"`
		Description             string          `json:"description"`
		Author                  string          `json:"author"`
		ObjectTypeSchemas       json.RawMessage `json:"object_type_schemas"`
		RelationshipTypeSchemas json.RawMessage `json:"relationship_type_schemas"`
		UIConfigs               json.RawMessage `json:"ui_configs"`
		ExtractionPrompts       json.RawMessage `json:"extraction_prompts"`
	}
	if err := json.Unmarshal(packData, &pack); err != nil {
		return fmt.Errorf("parsing template pack JSON: %w", err)
	}

	// Create the template pack
	log.Printf("creating template pack %q v%s...", pack.Name, pack.Version)
	created, err := client.TemplatePacks.CreatePack(ctx, &templatepacks.CreatePackRequest{
		Name:                    pack.Name,
		Version:                 pack.Version,
		Description:             strPtr(pack.Description),
		Author:                  strPtr(pack.Author),
		ObjectTypeSchemas:       pack.ObjectTypeSchemas,
		RelationshipTypeSchemas: pack.RelationshipTypeSchemas,
		UIConfigs:               pack.UIConfigs,
		ExtractionPrompts:       pack.ExtractionPrompts,
	})
	if err != nil {
		return fmt.Errorf("creating template pack: %w", err)
	}
	log.Printf("template pack created: id=%s", created.ID)

	// Assign the pack to the project
	log.Println("assigning template pack to project...")
	assignment, err := client.TemplatePacks.AssignPack(ctx, &templatepacks.AssignPackRequest{
		TemplatePackID: created.ID,
	})
	if err != nil {
		return fmt.Errorf("assigning template pack: %w", err)
	}
	log.Printf("template pack assigned: assignment_id=%s", assignment.ID)

	// Verify by listing compiled types
	log.Println("verifying compiled types...")
	compiled, err := client.TemplatePacks.GetCompiledTypes(ctx)
	if err != nil {
		return fmt.Errorf("getting compiled types: %w", err)
	}
	log.Printf("compiled types: %d object types, %d relationship types",
		len(compiled.ObjectTypes), len(compiled.RelationshipTypes))

	for _, ot := range compiled.ObjectTypes {
		if ot.PackID == created.ID {
			log.Printf("  object type: %s (pack: %s)", ot.Name, ot.PackName)
		}
	}
	for _, rt := range compiled.RelationshipTypes {
		if rt.PackID == created.ID {
			log.Printf("  relationship type: %s (pack: %s)", rt.Name, rt.PackName)
		}
	}

	log.Println("seed complete")
	return nil
}

// findPackPath locates the template pack JSON file relative to this script.
func findPackPath() string {
	// Try relative to CWD first
	candidates := []string{
		"templates/specmcp-pack.json",
		"server/specmcp/templates/specmcp-pack.json",
	}

	// Also try relative to this source file
	_, thisFile, _, ok := runtime.Caller(0)
	if ok {
		dir := filepath.Dir(thisFile)
		candidates = append(candidates, filepath.Join(dir, "..", "templates", "specmcp-pack.json"))
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			abs, _ := filepath.Abs(p)
			return abs
		}
	}

	// Default
	return "templates/specmcp-pack.json"
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
