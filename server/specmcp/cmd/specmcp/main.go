// Command specmcp runs the SpecMCP MCP server.
//
// It communicates over stdio using JSON-RPC 2.0 (MCP protocol)
// and delegates all persistence to the Emergent graph API.
//
// Required environment variables:
//
//	EMERGENT_TOKEN        - Project-scoped token (emt_*) for Emergent API
//
// Optional environment variables:
//
//	EMERGENT_URL          - Emergent server URL (default: http://localhost:3002)
//	SPECMCP_LOG_LEVEL     - Log level: debug, info, warn, error (default: info)
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/diane-assistant/diane/server/specmcp/internal/config"
	"github.com/diane-assistant/diane/server/specmcp/internal/content"
	"github.com/diane-assistant/diane/server/specmcp/internal/emergent"
	"github.com/diane-assistant/diane/server/specmcp/internal/mcp"
	"github.com/diane-assistant/diane/server/specmcp/internal/tools/constitution"
	"github.com/diane-assistant/diane/server/specmcp/internal/tools/patterns"
	"github.com/diane-assistant/diane/server/specmcp/internal/tools/query"
	gosync "github.com/diane-assistant/diane/server/specmcp/internal/tools/sync"
	"github.com/diane-assistant/diane/server/specmcp/internal/tools/tasks"
	"github.com/diane-assistant/diane/server/specmcp/internal/tools/workflow"
)

// Version is set via ldflags at build time.
var Version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "specmcp: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Set up structured logging to stderr (stdout is for MCP protocol)
	logLevel := parseLogLevel(cfg.Log.Level)
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	}))

	version := cfg.Server.Version
	if Version != "dev" {
		version = Version
	}

	logger.Info("starting specmcp",
		"version", version,
		"emergent_url", cfg.Emergent.URL,
	)

	// Set up signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Create tool registry and register tools
	registry := mcp.NewRegistry()

	// Create Emergent client
	emClient, err := emergent.New(&cfg.Emergent, logger)
	if err != nil {
		return fmt.Errorf("creating emergent client: %w", err)
	}

	// Register workflow tools
	specArtifact := workflow.NewSpecArtifact(emClient)
	registry.Register(workflow.NewSpecNew(emClient))
	registry.Register(specArtifact)
	registry.Register(workflow.NewSpecBatchArtifact(specArtifact))
	registry.Register(workflow.NewSpecArchive(emClient))
	registry.Register(workflow.NewSpecVerify(emClient))

	// Register query tools
	registry.Register(query.NewListChanges(emClient))
	registry.Register(query.NewGetChange(emClient))
	registry.Register(query.NewGetContext(emClient))
	registry.Register(query.NewGetComponent(emClient))
	registry.Register(query.NewGetAction(emClient))
	registry.Register(query.NewGetDataModel(emClient))
	registry.Register(query.NewGetService(emClient))
	registry.Register(query.NewGetScenario(emClient))
	registry.Register(query.NewGetPatterns(emClient))
	registry.Register(query.NewImpactAnalysis(emClient))
	registry.Register(query.NewSearch(emClient))

	// Register task management tools
	registry.Register(tasks.NewGenerateTasks(emClient))
	registry.Register(tasks.NewGetAvailableTasks(emClient))
	registry.Register(tasks.NewAssignTask(emClient))
	registry.Register(tasks.NewCompleteTask(emClient))
	registry.Register(tasks.NewGetCriticalPath(emClient))

	// Register pattern tools
	registry.Register(patterns.NewSuggestPatterns(emClient))
	registry.Register(patterns.NewApplyPattern(emClient))
	registry.Register(patterns.NewSeedPatterns(emClient))

	// Register constitution tools
	registry.Register(constitution.NewValidateConstitution(emClient))

	// Register sync tools
	registry.Register(gosync.NewSyncStatus(emClient))
	registry.Register(gosync.NewSync(emClient))
	registry.Register(gosync.NewGraphSummary(emClient))

	// Register prompts
	registry.RegisterPrompt(&content.GuidePrompt{})
	registry.RegisterPrompt(&content.WorkflowPrompt{})

	// Register resources
	registry.RegisterResource(&content.EntityModelResource{})
	registry.RegisterResource(&content.GuardrailsResource{})
	registry.RegisterResource(&content.ToolReferenceResource{})

	// Create and run MCP server
	server := mcp.NewServer(registry, mcp.ServerInfo{
		Name:    cfg.Server.Name,
		Version: version,
	}, logger)

	return server.Run(ctx)
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
