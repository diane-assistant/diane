// Package gmail provides Gmail API client with local caching
package gmail

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Cache manages the local Gmail cache database
type Cache struct {
	db   *sql.DB
	path string
}

// Email represents a cached email message
type Email struct {
	GmailID          string     `json:"gmail_id"`
	ThreadID         string     `json:"thread_id"`
	Subject          string     `json:"subject"`
	FromEmail        string     `json:"from_email"`
	FromName         string     `json:"from_name"`
	ToEmails         []string   `json:"to_emails"`
	CcEmails         []string   `json:"cc_emails"`
	Date             time.Time  `json:"date"`
	Snippet          string     `json:"snippet"`
	Labels           []string   `json:"labels"`
	HasAttachments   bool       `json:"has_attachments"`
	PlainText        *string    `json:"plain_text,omitempty"`
	JsonLD           []any      `json:"json_ld,omitempty"`
	MetadataCachedAt time.Time  `json:"metadata_cached_at"`
	ContentCachedAt  *time.Time `json:"content_cached_at,omitempty"`
	AccessedAt       time.Time  `json:"accessed_at"`
}

// Attachment represents a cached attachment reference
type Attachment struct {
	ID           int64      `json:"id"`
	GmailID      string     `json:"gmail_id"`
	AttachmentID string     `json:"attachment_id"`
	Filename     string     `json:"filename"`
	MimeType     string     `json:"mime_type"`
	Size         int64      `json:"size"`
	LocalPath    *string    `json:"local_path,omitempty"`
	DownloadedAt *time.Time `json:"downloaded_at,omitempty"`
}

// SenderStats represents aggregated sender statistics
type SenderStats struct {
	EmailPattern   string    `json:"email_pattern"`
	DisplayName    string    `json:"display_name"`
	MessageCount   int       `json:"message_count"`
	FirstSeen      time.Time `json:"first_seen"`
	LastSeen       time.Time `json:"last_seen"`
	CommonSubjects []string  `json:"common_subjects"`
	JsonLDTypes    []string  `json:"json_ld_types"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// SyncState represents sync state for an account
type SyncState struct {
	Account             string     `json:"account"`
	HistoryID           string     `json:"history_id"`
	LastFullSync        *time.Time `json:"last_full_sync,omitempty"`
	LastIncrementalSync *time.Time `json:"last_incremental_sync,omitempty"`
}

// NewCache creates or opens the Gmail cache database
func NewCache() (*Cache, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	dianeDir := filepath.Join(home, ".diane")
	if err := os.MkdirAll(dianeDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create .diane directory: %w", err)
	}

	dbPath := filepath.Join(dianeDir, "gmail.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open gmail cache database: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	cache := &Cache{db: db, path: dbPath}

	if err := cache.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return cache, nil
}

// Close closes the database connection
func (c *Cache) Close() error {
	return c.db.Close()
}

// migrate creates the database schema
func (c *Cache) migrate() error {
	schema := `
	-- Core email metadata
	CREATE TABLE IF NOT EXISTS emails (
		gmail_id TEXT PRIMARY KEY,
		thread_id TEXT NOT NULL,
		subject TEXT,
		from_email TEXT,
		from_name TEXT,
		to_emails TEXT,
		cc_emails TEXT,
		date DATETIME,
		snippet TEXT,
		labels TEXT,
		has_attachments INTEGER DEFAULT 0,
		plain_text TEXT,
		json_ld TEXT,
		metadata_cached_at DATETIME NOT NULL,
		content_cached_at DATETIME,
		accessed_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_emails_thread ON emails(thread_id);
	CREATE INDEX IF NOT EXISTS idx_emails_from ON emails(from_email);
	CREATE INDEX IF NOT EXISTS idx_emails_date ON emails(date);

	-- Attachment references
	CREATE TABLE IF NOT EXISTS attachments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		gmail_id TEXT NOT NULL,
		attachment_id TEXT NOT NULL,
		filename TEXT NOT NULL,
		mime_type TEXT,
		size INTEGER,
		local_path TEXT,
		downloaded_at DATETIME,
		FOREIGN KEY (gmail_id) REFERENCES emails(gmail_id) ON DELETE CASCADE,
		UNIQUE(gmail_id, attachment_id)
	);

	CREATE INDEX IF NOT EXISTS idx_attachments_gmail ON attachments(gmail_id);

	-- Pre-computed sender statistics
	CREATE TABLE IF NOT EXISTS sender_stats (
		email_pattern TEXT PRIMARY KEY,
		display_name TEXT,
		message_count INTEGER DEFAULT 0,
		first_seen DATETIME,
		last_seen DATETIME,
		common_subjects TEXT,
		json_ld_types TEXT,
		updated_at DATETIME NOT NULL
	);

	-- Sync state for incremental updates
	CREATE TABLE IF NOT EXISTS sync_state (
		account TEXT PRIMARY KEY,
		history_id TEXT,
		last_full_sync DATETIME,
		last_incremental_sync DATETIME
	);
	`

	_, err := c.db.Exec(schema)
	return err
}

// GetEmail retrieves an email from cache by ID
func (c *Cache) GetEmail(gmailID string) (*Email, error) {
	row := c.db.QueryRow(`
		SELECT gmail_id, thread_id, subject, from_email, from_name,
		       to_emails, cc_emails, date, snippet, labels, has_attachments,
		       plain_text, json_ld, metadata_cached_at, content_cached_at, accessed_at
		FROM emails WHERE gmail_id = ?
	`, gmailID)

	var email Email
	var toEmailsJSON, ccEmailsJSON, labelsJSON sql.NullString
	var jsonLDJSON sql.NullString
	var contentCachedAt sql.NullTime

	err := row.Scan(
		&email.GmailID, &email.ThreadID, &email.Subject, &email.FromEmail, &email.FromName,
		&toEmailsJSON, &ccEmailsJSON, &email.Date, &email.Snippet, &labelsJSON, &email.HasAttachments,
		&email.PlainText, &jsonLDJSON, &email.MetadataCachedAt, &contentCachedAt, &email.AccessedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get email: %w", err)
	}

	// Parse JSON arrays
	if toEmailsJSON.Valid {
		json.Unmarshal([]byte(toEmailsJSON.String), &email.ToEmails)
	}
	if ccEmailsJSON.Valid {
		json.Unmarshal([]byte(ccEmailsJSON.String), &email.CcEmails)
	}
	if labelsJSON.Valid {
		json.Unmarshal([]byte(labelsJSON.String), &email.Labels)
	}
	if jsonLDJSON.Valid {
		json.Unmarshal([]byte(jsonLDJSON.String), &email.JsonLD)
	}
	if contentCachedAt.Valid {
		email.ContentCachedAt = &contentCachedAt.Time
	}

	// Update accessed_at
	c.db.Exec("UPDATE emails SET accessed_at = ? WHERE gmail_id = ?", time.Now(), gmailID)

	return &email, nil
}

// SaveEmail saves or updates an email in the cache
func (c *Cache) SaveEmail(email *Email) error {
	toEmailsJSON, _ := json.Marshal(email.ToEmails)
	ccEmailsJSON, _ := json.Marshal(email.CcEmails)
	labelsJSON, _ := json.Marshal(email.Labels)

	var jsonLDJSON []byte
	if len(email.JsonLD) > 0 {
		jsonLDJSON, _ = json.Marshal(email.JsonLD)
	}

	_, err := c.db.Exec(`
		INSERT INTO emails (
			gmail_id, thread_id, subject, from_email, from_name,
			to_emails, cc_emails, date, snippet, labels, has_attachments,
			plain_text, json_ld, metadata_cached_at, content_cached_at, accessed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(gmail_id) DO UPDATE SET
			thread_id = excluded.thread_id,
			subject = excluded.subject,
			from_email = excluded.from_email,
			from_name = excluded.from_name,
			to_emails = excluded.to_emails,
			cc_emails = excluded.cc_emails,
			date = excluded.date,
			snippet = excluded.snippet,
			labels = excluded.labels,
			has_attachments = excluded.has_attachments,
			plain_text = COALESCE(excluded.plain_text, emails.plain_text),
			json_ld = COALESCE(excluded.json_ld, emails.json_ld),
			metadata_cached_at = excluded.metadata_cached_at,
			content_cached_at = COALESCE(excluded.content_cached_at, emails.content_cached_at),
			accessed_at = excluded.accessed_at
	`,
		email.GmailID, email.ThreadID, email.Subject, email.FromEmail, email.FromName,
		string(toEmailsJSON), string(ccEmailsJSON), email.Date, email.Snippet, string(labelsJSON), email.HasAttachments,
		email.PlainText, nullableString(jsonLDJSON), email.MetadataCachedAt, email.ContentCachedAt, email.AccessedAt,
	)

	return err
}

// SaveEmailContent updates just the content fields (plain_text, json_ld)
func (c *Cache) SaveEmailContent(gmailID string, plainText string, jsonLD []any) error {
	var jsonLDJSON []byte
	if len(jsonLD) > 0 {
		jsonLDJSON, _ = json.Marshal(jsonLD)
	}

	now := time.Now()
	_, err := c.db.Exec(`
		UPDATE emails SET 
			plain_text = ?,
			json_ld = ?,
			content_cached_at = ?,
			accessed_at = ?
		WHERE gmail_id = ?
	`, plainText, nullableString(jsonLDJSON), now, now, gmailID)

	return err
}

// GetAttachments retrieves attachments for an email
func (c *Cache) GetAttachments(gmailID string) ([]Attachment, error) {
	rows, err := c.db.Query(`
		SELECT id, gmail_id, attachment_id, filename, mime_type, size, local_path, downloaded_at
		FROM attachments WHERE gmail_id = ?
	`, gmailID)
	if err != nil {
		return nil, fmt.Errorf("failed to get attachments: %w", err)
	}
	defer rows.Close()

	var attachments []Attachment
	for rows.Next() {
		var a Attachment
		var downloadedAt sql.NullTime
		err := rows.Scan(&a.ID, &a.GmailID, &a.AttachmentID, &a.Filename, &a.MimeType, &a.Size, &a.LocalPath, &downloadedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan attachment: %w", err)
		}
		if downloadedAt.Valid {
			a.DownloadedAt = &downloadedAt.Time
		}
		attachments = append(attachments, a)
	}

	return attachments, nil
}

// SaveAttachment saves an attachment reference
func (c *Cache) SaveAttachment(a *Attachment) error {
	_, err := c.db.Exec(`
		INSERT INTO attachments (gmail_id, attachment_id, filename, mime_type, size, local_path, downloaded_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(gmail_id, attachment_id) DO UPDATE SET
			filename = excluded.filename,
			mime_type = excluded.mime_type,
			size = excluded.size,
			local_path = COALESCE(excluded.local_path, attachments.local_path),
			downloaded_at = COALESCE(excluded.downloaded_at, attachments.downloaded_at)
	`, a.GmailID, a.AttachmentID, a.Filename, a.MimeType, a.Size, a.LocalPath, a.DownloadedAt)

	return err
}

// UpdateAttachmentLocalPath updates the local path after download
func (c *Cache) UpdateAttachmentLocalPath(gmailID, attachmentID, localPath string) error {
	now := time.Now()
	_, err := c.db.Exec(`
		UPDATE attachments SET local_path = ?, downloaded_at = ?
		WHERE gmail_id = ? AND attachment_id = ?
	`, localPath, now, gmailID, attachmentID)

	return err
}

// GetSenderStats retrieves sender statistics by pattern
func (c *Cache) GetSenderStats(pattern string) (*SenderStats, error) {
	row := c.db.QueryRow(`
		SELECT email_pattern, display_name, message_count, first_seen, last_seen,
		       common_subjects, json_ld_types, updated_at
		FROM sender_stats WHERE email_pattern LIKE ?
	`, pattern)

	var stats SenderStats
	var commonSubjectsJSON, jsonLDTypesJSON sql.NullString

	err := row.Scan(
		&stats.EmailPattern, &stats.DisplayName, &stats.MessageCount,
		&stats.FirstSeen, &stats.LastSeen, &commonSubjectsJSON, &jsonLDTypesJSON, &stats.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get sender stats: %w", err)
	}

	if commonSubjectsJSON.Valid {
		json.Unmarshal([]byte(commonSubjectsJSON.String), &stats.CommonSubjects)
	}
	if jsonLDTypesJSON.Valid {
		json.Unmarshal([]byte(jsonLDTypesJSON.String), &stats.JsonLDTypes)
	}

	return &stats, nil
}

// GetSyncState retrieves sync state for an account
func (c *Cache) GetSyncState(account string) (*SyncState, error) {
	row := c.db.QueryRow(`
		SELECT account, history_id, last_full_sync, last_incremental_sync
		FROM sync_state WHERE account = ?
	`, account)

	var state SyncState
	var lastFullSync, lastIncrementalSync sql.NullTime

	err := row.Scan(&state.Account, &state.HistoryID, &lastFullSync, &lastIncrementalSync)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get sync state: %w", err)
	}

	if lastFullSync.Valid {
		state.LastFullSync = &lastFullSync.Time
	}
	if lastIncrementalSync.Valid {
		state.LastIncrementalSync = &lastIncrementalSync.Time
	}

	return &state, nil
}

// SaveSyncState saves sync state for an account
func (c *Cache) SaveSyncState(state *SyncState) error {
	_, err := c.db.Exec(`
		INSERT INTO sync_state (account, history_id, last_full_sync, last_incremental_sync)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(account) DO UPDATE SET
			history_id = excluded.history_id,
			last_full_sync = COALESCE(excluded.last_full_sync, sync_state.last_full_sync),
			last_incremental_sync = COALESCE(excluded.last_incremental_sync, sync_state.last_incremental_sync)
	`, state.Account, state.HistoryID, state.LastFullSync, state.LastIncrementalSync)

	return err
}

// SearchEmails searches cached emails by query (simple LIKE matching)
func (c *Cache) SearchEmails(fromPattern, subjectPattern string, limit int) ([]Email, error) {
	query := "SELECT gmail_id, thread_id, subject, from_email, from_name, date, snippet, labels, has_attachments FROM emails WHERE 1=1"
	args := []any{}

	if fromPattern != "" {
		query += " AND (from_email LIKE ? OR from_name LIKE ?)"
		args = append(args, "%"+fromPattern+"%", "%"+fromPattern+"%")
	}
	if subjectPattern != "" {
		query += " AND subject LIKE ?"
		args = append(args, "%"+subjectPattern+"%")
	}

	query += " ORDER BY date DESC LIMIT ?"
	args = append(args, limit)

	rows, err := c.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search emails: %w", err)
	}
	defer rows.Close()

	var emails []Email
	for rows.Next() {
		var e Email
		var labelsJSON sql.NullString
		err := rows.Scan(&e.GmailID, &e.ThreadID, &e.Subject, &e.FromEmail, &e.FromName, &e.Date, &e.Snippet, &labelsJSON, &e.HasAttachments)
		if err != nil {
			return nil, fmt.Errorf("failed to scan email: %w", err)
		}
		if labelsJSON.Valid {
			json.Unmarshal([]byte(labelsJSON.String), &e.Labels)
		}
		emails = append(emails, e)
	}

	return emails, nil
}

// helper to convert []byte to nullable string for SQL
func nullableString(b []byte) sql.NullString {
	if len(b) == 0 {
		return sql.NullString{}
	}
	return sql.NullString{String: string(b), Valid: true}
}
