package store

import (
	"context"

	"github.com/diane-assistant/diane/internal/db"
)

// ContextStore defines the interface for context, context server, and tool override storage operations
type ContextStore interface {
	// Context operations
	ListContexts(ctx context.Context) ([]db.Context, error)
	GetContext(ctx context.Context, name string) (*db.Context, error)
	GetDefaultContext(ctx context.Context) (*db.Context, error)
	CreateContext(ctx context.Context, c *db.Context) error
	UpdateContext(ctx context.Context, c *db.Context) error
	DeleteContext(ctx context.Context, name string) error
	SetDefaultContext(ctx context.Context, name string) error

	// ContextServer operations
	GetServersForContext(ctx context.Context, contextName string) ([]db.ContextServer, error)
	AddServerToContext(ctx context.Context, contextName, serverName string, enabled bool) error
	RemoveServerFromContext(ctx context.Context, contextName, serverName string) error
	SetServerEnabledInContext(ctx context.Context, contextName, serverName string, enabled bool) error

	// ContextServerTool operations
	GetToolsForContextServer(ctx context.Context, contextName, serverName string) (map[string]bool, error)
	SetToolEnabled(ctx context.Context, contextName, serverName, toolName string, enabled bool) error
	BulkSetToolsEnabled(ctx context.Context, contextName, serverName string, tools map[string]bool) error

	// Context queries (used by mcpproxy)
	GetContextDetail(ctx context.Context, contextName string) (*db.ContextDetail, error)
	IsToolEnabledInContext(ctx context.Context, contextName, serverName, toolName string) (bool, error)
	GetEnabledServersForContext(ctx context.Context, contextName string) ([]db.MCPServer, error)
}
