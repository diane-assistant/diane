package db

import (
	"database/sql"
	"fmt"
	"time"
)

// Usage represents a single API usage record
type Usage struct {
	ID           int64
	ProviderID   int64
	ProviderName string // for display
	Service      string // e.g., "vertex_ai_llm", "openai"
	Model        string
	InputTokens  int
	OutputTokens int
	CachedTokens int
	Cost         float64 // calculated cost in USD
	Metadata     string  // JSON metadata (e.g., request details)
	CreatedAt    time.Time
}

// UsageSummary represents aggregated usage stats
type UsageSummary struct {
	ProviderID    int64
	ProviderName  string
	Service       string
	Model         string
	TotalRequests int
	TotalInput    int
	TotalOutput   int
	TotalCached   int
	TotalCost     float64
}

// RecordUsage records a new usage entry
func (db *DB) RecordUsage(u *Usage) (int64, error) {
	result, err := db.conn.Exec(`
		INSERT INTO usage (provider_id, service, model, input_tokens, output_tokens, cached_tokens, cost, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ProviderID, u.Service, u.Model, u.InputTokens, u.OutputTokens, u.CachedTokens, u.Cost, u.Metadata, time.Now(),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to record usage: %w", err)
	}
	return result.LastInsertId()
}

// GetUsageByProvider returns usage records for a specific provider
func (db *DB) GetUsageByProvider(providerID int64, from, to time.Time, limit int) ([]*Usage, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := db.conn.Query(`
		SELECT u.id, u.provider_id, p.name, u.service, u.model, u.input_tokens, u.output_tokens, u.cached_tokens, u.cost, u.metadata, u.created_at
		FROM usage u
		LEFT JOIN providers p ON u.provider_id = p.id
		WHERE u.provider_id = ? AND u.created_at >= ? AND u.created_at <= ?
		ORDER BY u.created_at DESC
		LIMIT ?`,
		providerID, from, to, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanUsageRows(rows)
}

// GetUsageByService returns usage records for a specific service type
func (db *DB) GetUsageByService(service string, from, to time.Time, limit int) ([]*Usage, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := db.conn.Query(`
		SELECT u.id, u.provider_id, p.name, u.service, u.model, u.input_tokens, u.output_tokens, u.cached_tokens, u.cost, u.metadata, u.created_at
		FROM usage u
		LEFT JOIN providers p ON u.provider_id = p.id
		WHERE u.service = ? AND u.created_at >= ? AND u.created_at <= ?
		ORDER BY u.created_at DESC
		LIMIT ?`,
		service, from, to, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanUsageRows(rows)
}

// GetAllUsage returns all usage records within a time range
func (db *DB) GetAllUsage(from, to time.Time, limit int) ([]*Usage, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := db.conn.Query(`
		SELECT u.id, u.provider_id, COALESCE(p.name, ''), u.service, u.model, u.input_tokens, u.output_tokens, u.cached_tokens, u.cost, u.metadata, u.created_at
		FROM usage u
		LEFT JOIN providers p ON u.provider_id = p.id
		WHERE u.created_at >= ? AND u.created_at <= ?
		ORDER BY u.created_at DESC
		LIMIT ?`,
		from, to, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanUsageRows(rows)
}

func scanUsageRows(rows *sql.Rows) ([]*Usage, error) {
	var usages []*Usage
	for rows.Next() {
		u := &Usage{}
		var metadata sql.NullString
		if err := rows.Scan(&u.ID, &u.ProviderID, &u.ProviderName, &u.Service, &u.Model,
			&u.InputTokens, &u.OutputTokens, &u.CachedTokens, &u.Cost, &metadata, &u.CreatedAt); err != nil {
			return nil, err
		}
		u.Metadata = metadata.String
		usages = append(usages, u)
	}
	return usages, rows.Err()
}

// GetUsageSummary returns aggregated usage stats grouped by provider and model
func (db *DB) GetUsageSummary(from, to time.Time) ([]*UsageSummary, error) {
	rows, err := db.conn.Query(`
		SELECT u.provider_id, COALESCE(p.name, '') as provider_name, u.service, u.model,
			COUNT(*) as total_requests,
			SUM(u.input_tokens) as total_input,
			SUM(u.output_tokens) as total_output,
			SUM(u.cached_tokens) as total_cached,
			SUM(u.cost) as total_cost
		FROM usage u
		LEFT JOIN providers p ON u.provider_id = p.id
		WHERE u.created_at >= ? AND u.created_at <= ?
		GROUP BY u.provider_id, u.service, u.model
		ORDER BY total_cost DESC`,
		from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []*UsageSummary
	for rows.Next() {
		s := &UsageSummary{}
		if err := rows.Scan(&s.ProviderID, &s.ProviderName, &s.Service, &s.Model,
			&s.TotalRequests, &s.TotalInput, &s.TotalOutput, &s.TotalCached, &s.TotalCost); err != nil {
			return nil, err
		}
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}

// GetTotalCost returns the total cost within a time range
func (db *DB) GetTotalCost(from, to time.Time) (float64, error) {
	var total sql.NullFloat64
	err := db.conn.QueryRow(`
		SELECT SUM(cost) FROM usage WHERE created_at >= ? AND created_at <= ?`,
		from, to,
	).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total.Float64, nil
}

// DeleteOldUsage removes usage records older than the given duration
func (db *DB) DeleteOldUsage(olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := db.conn.Exec("DELETE FROM usage WHERE created_at < ?", cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
