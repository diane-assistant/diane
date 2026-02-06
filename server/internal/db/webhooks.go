package db

import (
	"database/sql"
	"time"
)

// Webhook represents a webhook configuration
type Webhook struct {
	ID        int64
	AgentID   *int64
	Path      string
	Prompt    string
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreateWebhook creates a new webhook
func (db *DB) CreateWebhook(path, prompt string, agentID *int64) (*Webhook, error) {
	query := `
		INSERT INTO webhooks (path, prompt, agent_id, enabled, created_at, updated_at)
		VALUES (?, ?, ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id, created_at, updated_at
	`

	webhook := &Webhook{
		Path:    path,
		Prompt:  prompt,
		AgentID: agentID,
		Enabled: true,
	}

	err := db.conn.QueryRow(query, path, prompt, agentID).Scan(&webhook.ID, &webhook.CreatedAt, &webhook.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return webhook, nil
}

// GetWebhookByPath retrieves a webhook by its path
func (db *DB) GetWebhookByPath(path string) (*Webhook, error) {
	query := `
		SELECT id, path, prompt, agent_id, enabled, created_at, updated_at
		FROM webhooks
		WHERE path = ?
	`

	webhook := &Webhook{}
	var enabled int
	err := db.conn.QueryRow(query, path).Scan(
		&webhook.ID,
		&webhook.Path,
		&webhook.Prompt,
		&webhook.AgentID,
		&enabled,
		&webhook.CreatedAt,
		&webhook.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	webhook.Enabled = enabled == 1
	return webhook, nil
}

// GetWebhook retrieves a webhook by its ID
func (db *DB) GetWebhook(id int64) (*Webhook, error) {
	query := `
		SELECT id, path, prompt, agent_id, enabled, created_at, updated_at
		FROM webhooks
		WHERE id = ?
	`

	webhook := &Webhook{}
	var enabled int
	err := db.conn.QueryRow(query, id).Scan(
		&webhook.ID,
		&webhook.Path,
		&webhook.Prompt,
		&webhook.AgentID,
		&enabled,
		&webhook.CreatedAt,
		&webhook.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	webhook.Enabled = enabled == 1
	return webhook, nil
}

// ListWebhooks returns all webhooks
func (db *DB) ListWebhooks() ([]*Webhook, error) {
	query := `
		SELECT id, path, prompt, agent_id, enabled, created_at, updated_at
		FROM webhooks
		ORDER BY created_at DESC
	`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var webhooks []*Webhook
	for rows.Next() {
		webhook := &Webhook{}
		var enabled int
		if err := rows.Scan(
			&webhook.ID,
			&webhook.Path,
			&webhook.Prompt,
			&webhook.AgentID,
			&enabled,
			&webhook.CreatedAt,
			&webhook.UpdatedAt,
		); err != nil {
			return nil, err
		}
		webhook.Enabled = enabled == 1
		webhooks = append(webhooks, webhook)
	}

	return webhooks, nil
}

// DeleteWebhook deletes a webhook by ID
func (db *DB) DeleteWebhook(id int64) error {
	_, err := db.conn.Exec("DELETE FROM webhooks WHERE id = ?", id)
	return err
}

// ToggleWebhook enables or disables a webhook
func (db *DB) ToggleWebhook(id int64, enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	_, err := db.conn.Exec("UPDATE webhooks SET enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", val, id)
	return err
}
