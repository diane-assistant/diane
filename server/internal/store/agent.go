package store

import (
	"context"
	"time"

	"github.com/diane-assistant/diane/internal/db"
)

// AgentStore defines the interface for agent and agent-log storage operations.
// "Agent" here refers to Diane's registered remote agents (db.Agent), NOT ACP agents.
type AgentStore interface {
	// --- Agent CRUD ---

	// CreateAgent registers a new agent.
	CreateAgent(ctx context.Context, name, url, agentType string) (*db.Agent, error)

	// GetAgent retrieves an agent by its legacy ID.
	GetAgent(ctx context.Context, id int64) (*db.Agent, error)

	// GetAgentByName retrieves an agent by its unique name.
	GetAgentByName(ctx context.Context, name string) (*db.Agent, error)

	// ListAgents returns all registered agents.
	ListAgents(ctx context.Context) ([]*db.Agent, error)

	// DeleteAgent removes an agent by its legacy ID.
	DeleteAgent(ctx context.Context, id int64) error

	// ToggleAgent enables or disables an agent.
	ToggleAgent(ctx context.Context, id int64, enabled bool) error

	// --- Agent log operations ---

	// CreateAgentLog creates a log entry for agent communication.
	CreateAgentLog(ctx context.Context, agentName, direction, messageType string, content, errMsg *string, durationMs *int) (*db.AgentLog, error)

	// ListAgentLogs returns agent logs, optionally filtered by agent name.
	ListAgentLogs(ctx context.Context, agentName *string, limit, offset int) ([]*db.AgentLog, error)

	// DeleteOldAgentLogs removes logs older than the given duration. Returns the count deleted.
	DeleteOldAgentLogs(ctx context.Context, olderThan time.Duration) (int64, error)
}
