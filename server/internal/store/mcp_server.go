package store

import (
	"context"

	"github.com/diane-assistant/diane/internal/db"
)

// MCPServerStore defines the interface for MCP server and placement storage operations
type MCPServerStore interface {
	// MCPServer operations
	ListMCPServers(ctx context.Context) ([]db.MCPServer, error)
	GetMCPServer(ctx context.Context, name string) (*db.MCPServer, error)
	GetMCPServerByID(ctx context.Context, id int64) (*db.MCPServer, error)
	CreateMCPServer(ctx context.Context, server *db.MCPServer) error
	UpdateMCPServer(ctx context.Context, server *db.MCPServer) error
	DeleteMCPServer(ctx context.Context, name string) error
	CountMCPServers(ctx context.Context) (int, error)
	EnsureBuiltinServers(ctx context.Context) error

	// MCPServerPlacement operations
	GetPlacementsForHost(ctx context.Context, hostID string) ([]db.PlacementWithServer, error)
	GetPlacementsForServer(ctx context.Context, serverID int64) ([]db.MCPServerPlacement, error)
	GetPlacement(ctx context.Context, serverID int64, hostID string) (*db.MCPServerPlacement, error)
	CreatePlacement(ctx context.Context, placement *db.MCPServerPlacement) error
	UpsertPlacement(ctx context.Context, serverID int64, hostID string, enabled bool) error
	SetPlacementEnabled(ctx context.Context, serverID int64, hostID string, enabled bool) error
	DeletePlacement(ctx context.Context, serverID int64, hostID string) error
	DeletePlacementsForServer(ctx context.Context, serverID int64) error
	DeletePlacementsForHost(ctx context.Context, hostID string) error
	GetEnabledServersForHost(ctx context.Context, hostID string) ([]db.MCPServer, error)
	IsServerEnabledOnHost(ctx context.Context, serverID int64, hostID string) (bool, error)
	BulkSetPlacements(ctx context.Context, serverID int64, placements map[string]bool) error
	EnsurePlacementExists(ctx context.Context, serverID int64, hostID string) error
	EnsurePlacementsForAllHosts(ctx context.Context) error
}
