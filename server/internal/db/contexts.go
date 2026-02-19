package db

import (
	"database/sql"
	"time"
)

// Context represents a context for grouping MCP servers
type Context struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	IsDefault   bool      `json:"is_default"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ContextServer represents the relationship between a context and an MCP server
type ContextServer struct {
	ID         int64  `json:"id"`
	ContextID  int64  `json:"context_id"`
	ServerID   int64  `json:"server_id"`
	ServerName string `json:"server_name"`
	Enabled    bool   `json:"enabled"`
}

// ContextServerTool represents a tool override in a context-server relationship
type ContextServerTool struct {
	ID              int64  `json:"id"`
	ContextServerID int64  `json:"context_server_id"`
	ToolName        string `json:"tool_name"`
	Enabled         bool   `json:"enabled"`
}

// ContextServerDetail includes the server details and tool overrides
type ContextServerDetail struct {
	ContextServer
	Server MCPServer       `json:"server"`
	Tools  map[string]bool `json:"tools"` // tool_name -> enabled
}

// ContextDetail includes all information about a context
type ContextDetail struct {
	Context
	Servers []ContextServerDetail `json:"servers"`
}

// ListContexts returns all contexts
func (db *DB) ListContexts() ([]Context, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, description, is_default, created_at, updated_at
		FROM contexts
		ORDER BY is_default DESC, name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contexts []Context
	for rows.Next() {
		var c Context
		var desc sql.NullString
		err := rows.Scan(&c.ID, &c.Name, &desc, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
		if err != nil {
			return nil, err
		}
		if desc.Valid {
			c.Description = desc.String
		}
		contexts = append(contexts, c)
	}
	return contexts, rows.Err()
}

// GetContext returns a context by name
func (db *DB) GetContext(name string) (*Context, error) {
	var c Context
	var desc sql.NullString
	err := db.conn.QueryRow(`
		SELECT id, name, description, is_default, created_at, updated_at
		FROM contexts
		WHERE name = ?
	`, name).Scan(&c.ID, &c.Name, &desc, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if desc.Valid {
		c.Description = desc.String
	}
	return &c, nil
}

// GetDefaultContext returns the default context
func (db *DB) GetDefaultContext() (*Context, error) {
	var c Context
	var desc sql.NullString
	err := db.conn.QueryRow(`
		SELECT id, name, description, is_default, created_at, updated_at
		FROM contexts
		WHERE is_default = 1
		LIMIT 1
	`).Scan(&c.ID, &c.Name, &desc, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if desc.Valid {
		c.Description = desc.String
	}
	return &c, nil
}

// CreateContext creates a new context
func (db *DB) CreateContext(ctx *Context) error {
	result, err := db.conn.Exec(`
		INSERT INTO contexts (name, description, is_default)
		VALUES (?, ?, ?)
	`, ctx.Name, ctx.Description, ctx.IsDefault)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	ctx.ID = id
	return nil
}

// UpdateContext updates an existing context
func (db *DB) UpdateContext(ctx *Context) error {
	_, err := db.conn.Exec(`
		UPDATE contexts 
		SET description = ?, updated_at = CURRENT_TIMESTAMP
		WHERE name = ?
	`, ctx.Description, ctx.Name)
	return err
}

// DeleteContext deletes a context by name
func (db *DB) DeleteContext(name string) error {
	// Don't allow deleting the default context
	ctx, err := db.GetContext(name)
	if err != nil {
		return err
	}
	if ctx != nil && ctx.IsDefault {
		return ErrCannotDeleteDefault
	}

	_, err = db.conn.Exec("DELETE FROM contexts WHERE name = ?", name)
	return err
}

// SetDefaultContext sets a context as the default (and unsets others)
func (db *DB) SetDefaultContext(name string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Unset current default
	if _, err := tx.Exec("UPDATE contexts SET is_default = 0"); err != nil {
		return err
	}

	// Set new default
	if _, err := tx.Exec("UPDATE contexts SET is_default = 1, updated_at = CURRENT_TIMESTAMP WHERE name = ?", name); err != nil {
		return err
	}

	return tx.Commit()
}

// GetServersForContext returns all servers in a context with their enabled status
func (db *DB) GetServersForContext(contextName string) ([]ContextServer, error) {
	rows, err := db.conn.Query(`
		SELECT cs.id, cs.context_id, cs.server_id, s.name, cs.enabled
		FROM context_servers cs
		JOIN contexts c ON cs.context_id = c.id
		JOIN mcp_servers s ON cs.server_id = s.id
		WHERE c.name = ?
		ORDER BY s.name
	`, contextName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []ContextServer
	for rows.Next() {
		var cs ContextServer
		err := rows.Scan(&cs.ID, &cs.ContextID, &cs.ServerID, &cs.ServerName, &cs.Enabled)
		if err != nil {
			return nil, err
		}
		servers = append(servers, cs)
	}
	return servers, rows.Err()
}

// AddServerToContext adds a server to a context
func (db *DB) AddServerToContext(contextName, serverName string, enabled bool) error {
	_, err := db.conn.Exec(`
		INSERT INTO context_servers (context_id, server_id, enabled)
		SELECT c.id, s.id, ?
		FROM contexts c, mcp_servers s
		WHERE c.name = ? AND s.name = ?
		ON CONFLICT(context_id, server_id) DO UPDATE SET enabled = excluded.enabled
	`, enabled, contextName, serverName)
	return err
}

// RemoveServerFromContext removes a server from a context
func (db *DB) RemoveServerFromContext(contextName, serverName string) error {
	_, err := db.conn.Exec(`
		DELETE FROM context_servers
		WHERE context_id = (SELECT id FROM contexts WHERE name = ?)
		AND server_id = (SELECT id FROM mcp_servers WHERE name = ?)
	`, contextName, serverName)
	return err
}

// SetServerEnabledInContext updates whether a server is enabled in a context
func (db *DB) SetServerEnabledInContext(contextName, serverName string, enabled bool) error {
	result, err := db.conn.Exec(`
		UPDATE context_servers 
		SET enabled = ?
		WHERE context_id = (SELECT id FROM contexts WHERE name = ?)
		AND server_id = (SELECT id FROM mcp_servers WHERE name = ?)
	`, enabled, contextName, serverName)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		// Server not in context, add it
		return db.AddServerToContext(contextName, serverName, enabled)
	}
	return nil
}

// GetToolsForContextServer returns tool overrides for a server in a context
func (db *DB) GetToolsForContextServer(contextName, serverName string) (map[string]bool, error) {
	rows, err := db.conn.Query(`
		SELECT cst.tool_name, cst.enabled
		FROM context_server_tools cst
		JOIN context_servers cs ON cst.context_server_id = cs.id
		JOIN contexts c ON cs.context_id = c.id
		JOIN mcp_servers s ON cs.server_id = s.id
		WHERE c.name = ? AND s.name = ?
	`, contextName, serverName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tools := make(map[string]bool)
	for rows.Next() {
		var toolName string
		var enabled bool
		if err := rows.Scan(&toolName, &enabled); err != nil {
			return nil, err
		}
		tools[toolName] = enabled
	}
	return tools, rows.Err()
}

// SetToolEnabled sets whether a tool is enabled for a server in a context
func (db *DB) SetToolEnabled(contextName, serverName, toolName string, enabled bool) error {
	// First get the context_server_id
	var csID int64
	err := db.conn.QueryRow(`
		SELECT cs.id
		FROM context_servers cs
		JOIN contexts c ON cs.context_id = c.id
		JOIN mcp_servers s ON cs.server_id = s.id
		WHERE c.name = ? AND s.name = ?
	`, contextName, serverName).Scan(&csID)
	if err == sql.ErrNoRows {
		// Server not in context, add it first
		if err := db.AddServerToContext(contextName, serverName, true); err != nil {
			return err
		}
		// Get the ID again
		err = db.conn.QueryRow(`
			SELECT cs.id
			FROM context_servers cs
			JOIN contexts c ON cs.context_id = c.id
			JOIN mcp_servers s ON cs.server_id = s.id
			WHERE c.name = ? AND s.name = ?
		`, contextName, serverName).Scan(&csID)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	// Upsert the tool override
	_, err = db.conn.Exec(`
		INSERT INTO context_server_tools (context_server_id, tool_name, enabled)
		VALUES (?, ?, ?)
		ON CONFLICT(context_server_id, tool_name) DO UPDATE SET enabled = excluded.enabled
	`, csID, toolName, enabled)
	return err
}

// BulkSetToolsEnabled sets multiple tool enabled states at once
func (db *DB) BulkSetToolsEnabled(contextName, serverName string, tools map[string]bool) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get context_server_id
	var csID int64
	err = tx.QueryRow(`
		SELECT cs.id
		FROM context_servers cs
		JOIN contexts c ON cs.context_id = c.id
		JOIN mcp_servers s ON cs.server_id = s.id
		WHERE c.name = ? AND s.name = ?
	`, contextName, serverName).Scan(&csID)
	if err == sql.ErrNoRows {
		return ErrServerNotInContext
	}
	if err != nil {
		return err
	}

	// Clear existing tool overrides
	if _, err := tx.Exec("DELETE FROM context_server_tools WHERE context_server_id = ?", csID); err != nil {
		return err
	}

	// Insert new tool overrides
	stmt, err := tx.Prepare(`
		INSERT INTO context_server_tools (context_server_id, tool_name, enabled)
		VALUES (?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for toolName, enabled := range tools {
		if _, err := stmt.Exec(csID, toolName, enabled); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetContextDetail returns a context with all its servers and tool overrides
func (db *DB) GetContextDetail(contextName string) (*ContextDetail, error) {
	ctx, err := db.GetContext(contextName)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		return nil, nil
	}

	detail := &ContextDetail{
		Context: *ctx,
		Servers: []ContextServerDetail{},
	}

	// Get all servers in this context
	contextServers, err := db.GetServersForContext(contextName)
	if err != nil {
		return nil, err
	}

	for _, cs := range contextServers {
		server, err := db.GetMCPServerByID(cs.ServerID)
		if err != nil {
			return nil, err
		}
		if server == nil {
			continue
		}

		tools, err := db.GetToolsForContextServer(contextName, cs.ServerName)
		if err != nil {
			return nil, err
		}

		detail.Servers = append(detail.Servers, ContextServerDetail{
			ContextServer: cs,
			Server:        *server,
			Tools:         tools,
		})
	}

	return detail, nil
}

// IsToolEnabledInContext checks if a specific tool is enabled in a context
// Returns true if:
// - The server is in the context and enabled
// - AND either no tool override exists (defaults to enabled) or the override is enabled
func (db *DB) IsToolEnabledInContext(contextName, serverName, toolName string) (bool, error) {
	var serverEnabled bool
	var toolEnabled sql.NullBool

	err := db.conn.QueryRow(`
		SELECT cs.enabled, cst.enabled
		FROM context_servers cs
		JOIN contexts c ON cs.context_id = c.id
		JOIN mcp_servers s ON cs.server_id = s.id
		LEFT JOIN context_server_tools cst ON cst.context_server_id = cs.id AND cst.tool_name = ?
		WHERE c.name = ? AND s.name = ?
	`, toolName, contextName, serverName).Scan(&serverEnabled, &toolEnabled)

	if err == sql.ErrNoRows {
		// Server not in context
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if !serverEnabled {
		return false, nil
	}

	// If no tool override, default to enabled
	if !toolEnabled.Valid {
		return true, nil
	}

	return toolEnabled.Bool, nil
}

// GetEnabledServersForContext returns all enabled servers for a context
func (db *DB) GetEnabledServersForContext(contextName string) ([]MCPServer, error) {
	rows, err := db.conn.Query(`
		SELECT s.id, s.name, s.enabled, s.type, s.command, s.args, s.env, s.url, s.headers, s.oauth, s.created_at, s.updated_at
		FROM mcp_servers s
		JOIN context_servers cs ON s.id = cs.server_id
		JOIN contexts c ON cs.context_id = c.id
		WHERE c.name = ? AND cs.enabled = 1 AND s.enabled = 1
		ORDER BY s.name
	`, contextName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []MCPServer
	for rows.Next() {
		s, err := scanMCPServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, s)
	}
	return servers, rows.Err()
}

// Error types
var (
	ErrCannotDeleteDefault = &ContextError{"cannot delete default context"}
	ErrServerNotInContext  = &ContextError{"server not in context"}
)

// ContextError represents a context-related error
type ContextError struct {
	Message string
}

func (e *ContextError) Error() string {
	return e.Message
}
