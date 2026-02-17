package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// DB represents the database connection
type DB struct {
	conn *sql.DB
	path string
}

// Job represents a scheduled job in the database
type Job struct {
	ID         int64
	Name       string
	Command    string
	Schedule   string
	Enabled    bool
	ActionType string  // "shell" (default) or "agent"
	AgentName  *string // Agent name for agent actions
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// JobExecution represents a job execution log entry
type JobExecution struct {
	ID        int64
	JobID     int64
	StartedAt time.Time
	EndedAt   *time.Time
	ExitCode  *int
	Stdout    string
	Stderr    string
	Error     *string
}

// New creates a new database connection
// If path is empty, uses ~/.diane/cron.db
func New(path string) (*DB, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		dianeDir := filepath.Join(home, ".diane")
		if err := os.MkdirAll(dianeDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create .diane directory: %w", err)
		}
		path = filepath.Join(dianeDir, "cron.db")
	}

	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool limits for SQLite (single writer)
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	conn.SetConnMaxLifetime(0)

	// Enable foreign keys
	if _, err := conn.Exec("PRAGMA foreign_keys = ON"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Set busy timeout (5 seconds) to handle concurrent access
	if _, err := conn.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to set busy timeout: %w", err)
	}

	db := &DB{conn: conn, path: path}

	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// migrate creates the database schema
func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS jobs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		command TEXT NOT NULL,
		schedule TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1,
		action_type TEXT NOT NULL DEFAULT 'shell',
		agent_name TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS job_executions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		job_id INTEGER NOT NULL,
		started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		ended_at DATETIME,
		exit_code INTEGER,
		stdout TEXT,
		stderr TEXT,
		error TEXT,
		FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_job_executions_job_id ON job_executions(job_id);
	CREATE INDEX IF NOT EXISTS idx_job_executions_started_at ON job_executions(started_at);

	CREATE TABLE IF NOT EXISTS agents (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		type TEXT NOT NULL DEFAULT 'acp',
		url TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS webhooks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		agent_id INTEGER,
		path TEXT NOT NULL UNIQUE,
		prompt TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE SET NULL
	);

	CREATE TABLE IF NOT EXISTS agent_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		agent_name TEXT NOT NULL,
		direction TEXT NOT NULL,
		message_type TEXT NOT NULL,
		content TEXT,
		error TEXT,
		duration_ms INTEGER,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_agent_logs_agent_name ON agent_logs(agent_name);
	CREATE INDEX IF NOT EXISTS idx_agent_logs_created_at ON agent_logs(created_at);

	-- MCP Servers (source of truth for available servers)
	CREATE TABLE IF NOT EXISTS mcp_servers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		enabled INTEGER NOT NULL DEFAULT 1,
		type TEXT NOT NULL,
		command TEXT,
		args TEXT,
		env TEXT,
		url TEXT,
		headers TEXT,
		oauth TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	-- Contexts for grouping MCP servers
	CREATE TABLE IF NOT EXISTS contexts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		description TEXT,
		is_default INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	-- Context-Server relationship (which servers are in which context)
	CREATE TABLE IF NOT EXISTS context_servers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		context_id INTEGER NOT NULL,
		server_id INTEGER NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (context_id) REFERENCES contexts(id) ON DELETE CASCADE,
		FOREIGN KEY (server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE,
		UNIQUE(context_id, server_id)
	);

	-- Tool overrides per context-server
	CREATE TABLE IF NOT EXISTS context_server_tools (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		context_server_id INTEGER NOT NULL,
		tool_name TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (context_server_id) REFERENCES context_servers(id) ON DELETE CASCADE,
		UNIQUE(context_server_id, tool_name)
	);

	CREATE INDEX IF NOT EXISTS idx_context_servers_context ON context_servers(context_id);
	CREATE INDEX IF NOT EXISTS idx_context_servers_server ON context_servers(server_id);
	CREATE INDEX IF NOT EXISTS idx_context_server_tools_cs ON context_server_tools(context_server_id);

	-- AI/Service Providers (embedding, LLM, storage providers)
	CREATE TABLE IF NOT EXISTS providers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		type TEXT NOT NULL,
		service TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1,
		is_default INTEGER NOT NULL DEFAULT 0,
		auth_type TEXT NOT NULL DEFAULT 'none',
		auth_config TEXT,
		config TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_providers_type ON providers(type);
	CREATE INDEX IF NOT EXISTS idx_providers_service ON providers(service);
	CREATE INDEX IF NOT EXISTS idx_providers_enabled ON providers(enabled);

	-- Usage tracking for AI providers
	CREATE TABLE IF NOT EXISTS usage (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		provider_id INTEGER,
		service TEXT NOT NULL,
		model TEXT NOT NULL,
		input_tokens INTEGER NOT NULL DEFAULT 0,
		output_tokens INTEGER NOT NULL DEFAULT 0,
		cached_tokens INTEGER NOT NULL DEFAULT 0,
		cost REAL NOT NULL DEFAULT 0,
		metadata TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE SET NULL
	);

	CREATE INDEX IF NOT EXISTS idx_usage_provider ON usage(provider_id);
	CREATE INDEX IF NOT EXISTS idx_usage_service ON usage(service);
	CREATE INDEX IF NOT EXISTS idx_usage_model ON usage(model);
	CREATE INDEX IF NOT EXISTS idx_usage_created_at ON usage(created_at);

	-- Slave servers for distributed MCP
	CREATE TABLE IF NOT EXISTS slave_servers (
		id TEXT PRIMARY KEY,
		host_id TEXT UNIQUE NOT NULL,
		cert_serial TEXT NOT NULL,
		issued_at DATETIME NOT NULL,
		expires_at DATETIME NOT NULL,
		last_seen DATETIME,
		enabled INTEGER NOT NULL DEFAULT 1,
		platform TEXT DEFAULT '',
		version TEXT DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_slave_servers_host_id ON slave_servers(host_id);
	CREATE INDEX IF NOT EXISTS idx_slave_servers_enabled ON slave_servers(enabled);

	-- Revoked slave credentials
	CREATE TABLE IF NOT EXISTS revoked_slave_credentials (
		id TEXT PRIMARY KEY,
		host_id TEXT NOT NULL,
		cert_serial TEXT NOT NULL,
		revoked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		reason TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_revoked_creds_host_id ON revoked_slave_credentials(host_id);
	CREATE INDEX IF NOT EXISTS idx_revoked_creds_serial ON revoked_slave_credentials(cert_serial);

	-- Pending pairing requests (optional persistence)
	CREATE TABLE IF NOT EXISTS pairing_requests (
		id TEXT PRIMARY KEY,
		host_id TEXT NOT NULL,
		pairing_code TEXT NOT NULL,
		csr TEXT NOT NULL,
		platform TEXT DEFAULT '',
		requested_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending'
	);

	CREATE INDEX IF NOT EXISTS idx_pairing_requests_host_id ON pairing_requests(host_id);
	CREATE INDEX IF NOT EXISTS idx_pairing_requests_status ON pairing_requests(status);
	`

	_, err := db.conn.Exec(schema)
	if err != nil {
		return err
	}

	// Migration: Add certificate column to pairing_requests if missing
	// This handles the case where the table was created before this column was added
	db.conn.Exec(`ALTER TABLE pairing_requests ADD COLUMN certificate TEXT`)

	// Migration: Add platform column to pairing_requests if missing
	db.conn.Exec(`ALTER TABLE pairing_requests ADD COLUMN platform TEXT DEFAULT ''`)

	// Migration: Add platform column to slave_servers if missing
	db.conn.Exec(`ALTER TABLE slave_servers ADD COLUMN platform TEXT DEFAULT ''`)

	// Migration: Add version column to slave_servers if missing
	db.conn.Exec(`ALTER TABLE slave_servers ADD COLUMN version TEXT DEFAULT ''`)

	// Migration: Add node-aware columns to mcp_servers if missing
	db.conn.Exec(`ALTER TABLE mcp_servers ADD COLUMN node_id TEXT`)
	db.conn.Exec(`ALTER TABLE mcp_servers ADD COLUMN node_mode TEXT DEFAULT 'master'`)

	// Create index for efficient node-based queries
	db.conn.Exec(`CREATE INDEX IF NOT EXISTS idx_mcp_servers_node ON mcp_servers(node_id, node_mode)`)

	// Migration: Create mcp_server_placements table for host-based MCP deployment
	db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS mcp_server_placements (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			server_id INTEGER NOT NULL,
			host_id TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE,
			UNIQUE(server_id, host_id)
		)
	`)
	db.conn.Exec(`CREATE INDEX IF NOT EXISTS idx_placements_host ON mcp_server_placements(host_id)`)
	db.conn.Exec(`CREATE INDEX IF NOT EXISTS idx_placements_server ON mcp_server_placements(server_id)`)

	// Migrate existing node_mode/node_id data to placements
	if err := db.migratePlacementsFromNodeMode(); err != nil {
		return fmt.Errorf("failed to migrate placements: %w", err)
	}

	// Ensure default context exists
	return db.ensureDefaultContext()
}

// ensureDefaultContext creates the default "personal" context if no contexts exist
func (db *DB) ensureDefaultContext() error {
	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM contexts").Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		_, err = db.conn.Exec(`
			INSERT INTO contexts (name, description, is_default) 
			VALUES ('personal', 'Personal productivity tools', 1)
		`)
		return err
	}
	return nil
}

// migratePlacementsFromNodeMode migrates existing node_mode/node_id data to the placements table
func (db *DB) migratePlacementsFromNodeMode() error {
	// Check if we've already migrated by seeing if placements exist
	var existingCount int
	if err := db.conn.QueryRow("SELECT COUNT(*) FROM mcp_server_placements").Scan(&existingCount); err != nil {
		return fmt.Errorf("failed to check existing placements: %w", err)
	}

	if existingCount > 0 {
		// Already migrated, skip
		return nil
	}

	// Get all MCP servers with their node configuration
	rows, err := db.conn.Query(`
		SELECT id, name, type, enabled, node_mode, node_id 
		FROM mcp_servers
	`)
	if err != nil {
		return fmt.Errorf("failed to query mcp_servers: %w", err)
	}
	defer rows.Close()

	type serverInfo struct {
		id       int64
		name     string
		typ      string
		enabled  bool
		nodeMode *string
		nodeID   *string
	}

	var servers []serverInfo
	for rows.Next() {
		var s serverInfo
		if err := rows.Scan(&s.id, &s.name, &s.typ, &s.enabled, &s.nodeMode, &s.nodeID); err != nil {
			return fmt.Errorf("failed to scan server row: %w", err)
		}
		servers = append(servers, s)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating servers: %w", err)
	}

	// Migrate each server to placements
	for _, s := range servers {
		// Determine which hosts this server should be placed on
		var hostIDs []string

		// If node_mode is not set or empty, default to "master" for backwards compatibility
		nodeMode := "master"
		if s.nodeMode != nil && *s.nodeMode != "" {
			nodeMode = *s.nodeMode
		}

		switch nodeMode {
		case "master":
			// Place on master node only
			hostIDs = []string{"master"}
		case "slave":
			// Place on specific slave node
			if s.nodeID != nil && *s.nodeID != "" {
				hostIDs = []string{*s.nodeID}
			}
		case "all":
			// Place on master and all slaves
			hostIDs = []string{"master"}
			// Get all slave IDs
			slaveRows, err := db.conn.Query("SELECT id FROM slave_servers")
			if err != nil {
				return fmt.Errorf("failed to query slaves: %w", err)
			}
			for slaveRows.Next() {
				var slaveID string
				if err := slaveRows.Scan(&slaveID); err != nil {
					slaveRows.Close()
					return fmt.Errorf("failed to scan slave ID: %w", err)
				}
				hostIDs = append(hostIDs, slaveID)
			}
			slaveRows.Close()
		}

		// Create placement for each host
		// Preserve existing enabled state during migration: if the server was enabled,
		// its master placement should also be enabled to maintain existing behavior.
		// New servers added later will default to disabled (secure by default).
		for _, hostID := range hostIDs {
			enabled := 0

			// Preserve previous enabled state on master during migration
			if s.enabled && hostID == "master" {
				enabled = 1
			}

			_, err := db.conn.Exec(`
				INSERT INTO mcp_server_placements (server_id, host_id, enabled)
				VALUES (?, ?, ?)
			`, s.id, hostID, enabled)

			if err != nil {
				return fmt.Errorf("failed to create placement for server %s on host %s: %w", s.name, hostID, err)
			}
		}
	}

	return nil
}
