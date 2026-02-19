package db

// TODO(emergent-migration): Agent entity must be migrated to Emergent.
// Steps:
//   1. Create AgentStore interface in internal/store/agent.go
//      Methods: CreateAgent, GetAgent, GetAgentByName, ListAgents, DeleteAgent, ToggleAgent
//   2. Create EmergentAgentStore in internal/store/agent_emergent.go
//      - Graph object type: "agent"
//      - Properties: name, type, url, enabled
//      - Labels: legacy_id:{id}, name:{name}
//   3. Wire AgentStore into API layer and ACP server
//   4. Also migrate AgentLog (related entity):
//      - Graph object type: "agent_log"
//      - Relationship: agent -> has_log -> agent_log
//      - Properties: direction, message_type, content, error, duration_ms
//      - DeleteOldAgentLogs maps to temporal filter + bulk delete
// Tracking: docs/EMERGENT_MIGRATION_PLAN.md

import (
	"fmt"
	"time"
)

// Agent represents a remote agent (e.g., OpenCode server)
type Agent struct {
	ID        int64
	Name      string
	Type      string
	URL       string
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreateAgent creates a new agent
// DEPRECATED: Stub for refactoring. Implement AgentStore interface instead.
func (db *DB) CreateAgent(name, url, agentType string) (*Agent, error) {
	return nil, fmt.Errorf("CreateAgent: not implemented - refactor to AgentStore")
}

// GetAgent retrieves an agent by ID
// DEPRECATED: Stub for refactoring. Implement AgentStore interface instead.
func (db *DB) GetAgent(id int64) (*Agent, error) {
	return nil, fmt.Errorf("GetAgent: not implemented - refactor to AgentStore")
}

// GetAgentByName retrieves an agent by name
// DEPRECATED: Stub for refactoring. Implement AgentStore interface instead.
func (db *DB) GetAgentByName(name string) (*Agent, error) {
	return nil, fmt.Errorf("GetAgentByName: not implemented - refactor to AgentStore")
}

// ListAgents returns all agents
// DEPRECATED: Stub for refactoring. Implement AgentStore interface instead.
func (db *DB) ListAgents() ([]*Agent, error) {
	return nil, fmt.Errorf("ListAgents: not implemented - refactor to AgentStore")
}

// DeleteAgent deletes an agent by ID
// DEPRECATED: Stub for refactoring. Implement AgentStore interface instead.
func (db *DB) DeleteAgent(id int64) error {
	return fmt.Errorf("DeleteAgent: not implemented - refactor to AgentStore")
}

// ToggleAgent enables or disables an agent
// DEPRECATED: Stub for refactoring. Implement AgentStore interface instead.
func (db *DB) ToggleAgent(id int64, enabled bool) error {
	return fmt.Errorf("ToggleAgent: not implemented - refactor to AgentStore")
}

// AgentLog represents a log entry for agent communication
type AgentLog struct {
	ID          int64
	AgentName   string
	Direction   string // "request" or "response"
	MessageType string // "ping", "run", "list", etc.
	Content     *string
	Error       *string
	DurationMs  *int
	CreatedAt   time.Time
}

// CreateAgentLog creates a new agent log entry
// DEPRECATED: Stub for refactoring. Implement AgentStore interface instead.
func (db *DB) CreateAgentLog(agentName, direction, messageType string, content, errMsg *string, durationMs *int) (*AgentLog, error) {
	return nil, fmt.Errorf("CreateAgentLog: not implemented - refactor to AgentStore")
}

// ListAgentLogs returns agent logs, optionally filtered by agent name
// DEPRECATED: Stub for refactoring. Implement AgentStore interface instead.
func (db *DB) ListAgentLogs(agentName *string, limit, offset int) ([]*AgentLog, error) {
	return nil, fmt.Errorf("ListAgentLogs: not implemented - refactor to AgentStore")
}

// DeleteOldAgentLogs deletes logs older than the specified duration
// DEPRECATED: Stub for refactoring. Implement AgentStore interface instead.
func (db *DB) DeleteOldAgentLogs(olderThan time.Duration) (int64, error) {
	return 0, fmt.Errorf("DeleteOldAgentLogs: not implemented - refactor to AgentStore")
}
