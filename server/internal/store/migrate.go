package store

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/diane-assistant/diane/internal/db"
)

// MigrateFromSQLite performs a one-time migration of MCP servers (with placements)
// and contexts (with server associations and tool overrides) from SQLite to
// Emergent-backed stores.
//
// Idempotency: if Emergent already contains any MCP server objects the migration
// is skipped entirely — this function is safe to call on every startup.
func MigrateFromSQLite(database *db.DB, mcpServerStore MCPServerStore, contextStore ContextStore) error {
	ctx := context.Background()

	// -----------------------------------------------------------------------
	// Guard: skip if Emergent already has data
	// -----------------------------------------------------------------------
	existingCount, err := mcpServerStore.CountMCPServers(ctx)
	if err != nil {
		return fmt.Errorf("migration: failed to check existing MCP servers: %w", err)
	}
	if existingCount > 0 {
		slog.Info("Migration skipped: Emergent already has MCP server data", "count", existingCount)
		return nil
	}

	slog.Info("Starting one-time SQLite → Emergent migration")

	// -----------------------------------------------------------------------
	// Phase 1: MCP Servers + Placements
	// -----------------------------------------------------------------------
	servers, err := database.ListMCPServers()
	if err != nil {
		return fmt.Errorf("migration: failed to list SQLite MCP servers: %w", err)
	}

	serversMigrated := 0
	placementsMigrated := 0
	for _, srv := range servers {
		// Create the server in Emergent (preserving legacy_id)
		srvCopy := srv // copy for pointer
		if err := mcpServerStore.CreateMCPServer(ctx, &srvCopy); err != nil {
			return fmt.Errorf("migration: failed to create MCP server %q: %w", srv.Name, err)
		}
		serversMigrated++

		// Migrate placements for this server
		placements, err := database.GetPlacementsForServer(srv.ID)
		if err != nil {
			slog.Warn("migration: failed to get placements for server, skipping placements",
				"server", srv.Name, "error", err)
			continue
		}
		for _, p := range placements {
			if err := mcpServerStore.UpsertPlacement(ctx, srvCopy.ID, p.HostID, p.Enabled); err != nil {
				slog.Warn("migration: failed to create placement",
					"server", srv.Name, "host", p.HostID, "error", err)
				continue
			}
			placementsMigrated++
		}
	}
	slog.Info("Migration: MCP servers migrated",
		"servers", serversMigrated, "placements", placementsMigrated)

	// -----------------------------------------------------------------------
	// Phase 2: Contexts + ContextServers + Tool Overrides
	// -----------------------------------------------------------------------
	contexts, err := database.ListContexts()
	if err != nil {
		return fmt.Errorf("migration: failed to list SQLite contexts: %w", err)
	}

	contextsMigrated := 0
	contextServersMigrated := 0
	toolOverridesMigrated := 0
	for _, c := range contexts {
		// Create context in Emergent (preserving legacy_id)
		cCopy := c
		if err := contextStore.CreateContext(ctx, &cCopy); err != nil {
			return fmt.Errorf("migration: failed to create context %q: %w", c.Name, err)
		}
		contextsMigrated++

		// Get servers for this context
		contextServers, err := database.GetServersForContext(c.Name)
		if err != nil {
			slog.Warn("migration: failed to get servers for context, skipping",
				"context", c.Name, "error", err)
			continue
		}

		for _, cs := range contextServers {
			// Add server to context
			if err := contextStore.AddServerToContext(ctx, c.Name, cs.ServerName, cs.Enabled); err != nil {
				slog.Warn("migration: failed to add server to context",
					"context", c.Name, "server", cs.ServerName, "error", err)
				continue
			}
			contextServersMigrated++

			// Get tool overrides for this context-server pair
			tools, err := database.GetToolsForContextServer(c.Name, cs.ServerName)
			if err != nil {
				slog.Warn("migration: failed to get tool overrides",
					"context", c.Name, "server", cs.ServerName, "error", err)
				continue
			}
			if len(tools) > 0 {
				if err := contextStore.BulkSetToolsEnabled(ctx, c.Name, cs.ServerName, tools); err != nil {
					slog.Warn("migration: failed to set tool overrides",
						"context", c.Name, "server", cs.ServerName, "error", err)
					continue
				}
				toolOverridesMigrated += len(tools)
			}
		}
	}

	slog.Info("Migration: contexts migrated",
		"contexts", contextsMigrated,
		"context_servers", contextServersMigrated,
		"tool_overrides", toolOverridesMigrated)

	slog.Info("One-time SQLite → Emergent migration complete",
		"servers", serversMigrated,
		"placements", placementsMigrated,
		"contexts", contextsMigrated,
		"context_servers", contextServersMigrated,
		"tool_overrides", toolOverridesMigrated)

	return nil
}
