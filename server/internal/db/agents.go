package db

import (
	"database/sql"
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
func (db *DB) CreateAgent(name, url, agentType string) (*Agent, error) {
	if agentType == "" {
		agentType = "acp"
	}

	query := `
		INSERT INTO agents (name, url, type, enabled, created_at, updated_at)
		VALUES (?, ?, ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id, created_at, updated_at
	`

	agent := &Agent{
		Name:    name,
		URL:     url,
		Type:    agentType,
		Enabled: true,
	}

	err := db.conn.QueryRow(query, name, url, agentType).Scan(&agent.ID, &agent.CreatedAt, &agent.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return agent, nil
}

// GetAgent retrieves an agent by ID
func (db *DB) GetAgent(id int64) (*Agent, error) {
	query := `
		SELECT id, name, type, url, enabled, created_at, updated_at
		FROM agents
		WHERE id = ?
	`

	agent := &Agent{}
	var enabled int
	err := db.conn.QueryRow(query, id).Scan(
		&agent.ID,
		&agent.Name,
		&agent.Type,
		&agent.URL,
		&enabled,
		&agent.CreatedAt,
		&agent.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	agent.Enabled = enabled == 1
	return agent, nil
}

// GetAgentByName retrieves an agent by name
func (db *DB) GetAgentByName(name string) (*Agent, error) {
	query := `
		SELECT id, name, type, url, enabled, created_at, updated_at
		FROM agents
		WHERE name = ?
	`

	agent := &Agent{}
	var enabled int
	err := db.conn.QueryRow(query, name).Scan(
		&agent.ID,
		&agent.Name,
		&agent.Type,
		&agent.URL,
		&enabled,
		&agent.CreatedAt,
		&agent.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	agent.Enabled = enabled == 1
	return agent, nil
}

// ListAgents returns all agents
func (db *DB) ListAgents() ([]*Agent, error) {
	query := `
		SELECT id, name, type, url, enabled, created_at, updated_at
		FROM agents
		ORDER BY created_at DESC
	`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*Agent
	for rows.Next() {
		agent := &Agent{}
		var enabled int
		if err := rows.Scan(
			&agent.ID,
			&agent.Name,
			&agent.Type,
			&agent.URL,
			&enabled,
			&agent.CreatedAt,
			&agent.UpdatedAt,
		); err != nil {
			return nil, err
		}
		agent.Enabled = enabled == 1
		agents = append(agents, agent)
	}

	return agents, nil
}

// DeleteAgent deletes an agent by ID
func (db *DB) DeleteAgent(id int64) error {
	_, err := db.conn.Exec("DELETE FROM agents WHERE id = ?", id)
	return err
}

// ToggleAgent enables or disables an agent
func (db *DB) ToggleAgent(id int64, enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	_, err := db.conn.Exec("UPDATE agents SET enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", val, id)
	return err
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
func (db *DB) CreateAgentLog(agentName, direction, messageType string, content, errMsg *string, durationMs *int) (*AgentLog, error) {
	query := `
		INSERT INTO agent_logs (agent_name, direction, message_type, content, error, duration_ms, created_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		RETURNING id, created_at
	`

	log := &AgentLog{
		AgentName:   agentName,
		Direction:   direction,
		MessageType: messageType,
		Content:     content,
		Error:       errMsg,
		DurationMs:  durationMs,
	}

	err := db.conn.QueryRow(query, agentName, direction, messageType, content, errMsg, durationMs).Scan(&log.ID, &log.CreatedAt)
	if err != nil {
		return nil, err
	}

	return log, nil
}

// ListAgentLogs returns agent logs, optionally filtered by agent name
func (db *DB) ListAgentLogs(agentName *string, limit, offset int) ([]*AgentLog, error) {
	var query string
	var args []interface{}

	if agentName != nil && *agentName != "" {
		query = `
			SELECT id, agent_name, direction, message_type, content, error, duration_ms, created_at
			FROM agent_logs
			WHERE agent_name = ?
			ORDER BY created_at DESC
			LIMIT ? OFFSET ?
		`
		args = []interface{}{*agentName, limit, offset}
	} else {
		query = `
			SELECT id, agent_name, direction, message_type, content, error, duration_ms, created_at
			FROM agent_logs
			ORDER BY created_at DESC
			LIMIT ? OFFSET ?
		`
		args = []interface{}{limit, offset}
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*AgentLog
	for rows.Next() {
		log := &AgentLog{}
		if err := rows.Scan(
			&log.ID,
			&log.AgentName,
			&log.Direction,
			&log.MessageType,
			&log.Content,
			&log.Error,
			&log.DurationMs,
			&log.CreatedAt,
		); err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// DeleteOldAgentLogs deletes logs older than the specified duration
func (db *DB) DeleteOldAgentLogs(olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := db.conn.Exec("DELETE FROM agent_logs WHERE created_at < ?", cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
