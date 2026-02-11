package files

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the SQLite database connection for the file index
type DB struct {
	conn *sql.DB
	path string
}

// File represents a file in the unified index
// This is a passive index - files are registered by the LLM using metadata
// gathered from existing tools (shell commands, Drive MCP, Gmail MCP, etc.)
type File struct {
	ID           int64
	Source       string // Simple string identifier: "local", "gdrive", "gmail", etc.
	SourceFileID string // Provider-specific file ID (optional)
	Path         string // Full path within the source
	Filename     string // Base filename
	Extension    string // Lowercase, no dot
	Size         int64
	MimeType     string
	IsDirectory  bool
	CreatedAt    *time.Time
	ModifiedAt   time.Time

	// Content identification (for cross-source duplicate detection)
	ContentHash string // SHA-256 of full content
	PartialHash string // SHA-256 of first 64KB

	// Extracted content
	ContentText      string
	ContentPreview   string
	ContentIndexedAt *time.Time

	// Classification
	Category    string // document, image, video, audio, code, archive, data, other
	Subcategory string

	// AI metadata
	AISummary    string
	AITags       string // JSON array
	AIAnalyzedAt *time.Time

	// Embeddings
	Embedding      []byte
	EmbeddingModel string
	EmbeddingAt    *time.Time

	// Index metadata
	IndexedAt  time.Time
	VerifiedAt *time.Time
	Status     string // active, missing, moved, deleted
}

// Tag represents a user-defined tag
type Tag struct {
	ID          int64
	Name        string
	Color       string
	Description string
	ParentID    *int64
	CreatedAt   time.Time
	UsageCount  int
}

// FileTag represents a file-tag association
type FileTag struct {
	FileID     int64
	TagID      int64
	TaggedAt   time.Time
	TaggedBy   string // 'user', 'auto', 'ai'
	Confidence float64
}

// DuplicateGroup represents a group of duplicate files
type DuplicateGroup struct {
	ID              int64
	ContentHash     string
	FileCount       int
	TotalWastedSize int64
	DetectedAt      time.Time
	Resolved        bool
	ResolutionNotes string
	KeptFileID      *int64
}

// FileRelation represents a relationship between files
type FileRelation struct {
	ID           int64
	SourceFileID int64
	TargetFileID int64
	RelationType string // 'duplicate', 'version', 'derived', 'related', 'similar'
	Confidence   float64
	Metadata     string
	CreatedAt    time.Time
}

// FileXRef represents a cross-reference to other Diane data
type FileXRef struct {
	ID         int64
	FileID     int64
	XRefType   string // 'gmail_attachment', 'gmail_download', 'job_output'
	XRefSource string
	XRefID     string
	Metadata   string
	CreatedAt  time.Time
}

// Activity represents a file activity log entry
type Activity struct {
	ID          int64
	FileID      *int64
	Source      string
	Path        string
	Action      string // 'registered', 'updated', 'tagged', 'removed', 'verified'
	Details     string // JSON with action-specific details
	PerformedAt time.Time
	PerformedBy string // 'llm', 'user', 'system'
}

// NewDB creates a new database connection for the file index
// If path is empty, uses ~/.diane/files.db
func NewDB(path string) (*DB, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		dianeDir := filepath.Join(home, ".diane")
		if err := os.MkdirAll(dianeDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create .diane directory: %w", err)
		}
		path = filepath.Join(dianeDir, "files.db")
	}

	conn, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_synchronous=NORMAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys
	if _, err := conn.Exec("PRAGMA foreign_keys = ON"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
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
	-- Unified file index
	-- Source is just a simple string identifier, not a foreign key
	CREATE TABLE IF NOT EXISTS files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		
		-- Source reference (simple string)
		source TEXT NOT NULL,                 -- "local", "gdrive", "gmail", etc.
		source_file_id TEXT,                  -- Provider-specific file ID (optional)
		
		-- Universal identifiers
		path TEXT NOT NULL,                   -- Full path within source
		filename TEXT NOT NULL,               -- Base filename
		extension TEXT,                       -- Lowercase, no dot
		
		-- File attributes
		size INTEGER NOT NULL,
		mime_type TEXT,
		is_directory INTEGER DEFAULT 0,
		
		-- Timestamps (from source)
		created_at DATETIME,
		modified_at DATETIME NOT NULL,
		
		-- Content identification (for cross-source duplicate detection)
		content_hash TEXT,                    -- SHA-256 of full content
		partial_hash TEXT,                    -- SHA-256 of first 64KB
		
		-- Extracted content
		content_text TEXT,                    -- Extracted plain text
		content_preview TEXT,                 -- First ~500 chars
		content_indexed_at DATETIME,
		
		-- Classification
		category TEXT,                        -- document, image, video, audio, code, archive, data, other
		subcategory TEXT,                     -- invoice, receipt, photo, screenshot, etc.
		
		-- AI-generated metadata
		ai_summary TEXT,
		ai_tags TEXT,                         -- JSON: AI-suggested tags
		ai_analyzed_at DATETIME,
		
		-- Embeddings
		embedding BLOB,
		embedding_model TEXT,
		embedding_at DATETIME,
		
		-- Index metadata
		indexed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		verified_at DATETIME,
		status TEXT DEFAULT 'active',         -- active, missing, moved, deleted
		
		UNIQUE(source, path)
	);

	CREATE INDEX IF NOT EXISTS idx_files_source ON files(source);
	CREATE INDEX IF NOT EXISTS idx_files_path ON files(path);
	CREATE INDEX IF NOT EXISTS idx_files_filename ON files(filename);
	CREATE INDEX IF NOT EXISTS idx_files_extension ON files(extension);
	CREATE INDEX IF NOT EXISTS idx_files_size ON files(size);
	CREATE INDEX IF NOT EXISTS idx_files_modified ON files(modified_at);
	CREATE INDEX IF NOT EXISTS idx_files_category ON files(category);
	CREATE INDEX IF NOT EXISTS idx_files_content_hash ON files(content_hash);
	CREATE INDEX IF NOT EXISTS idx_files_partial_hash ON files(partial_hash);
	CREATE INDEX IF NOT EXISTS idx_files_status ON files(status);

	-- User-defined tags
	CREATE TABLE IF NOT EXISTS tags (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		color TEXT,
		description TEXT,
		parent_id INTEGER,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		usage_count INTEGER DEFAULT 0,
		FOREIGN KEY (parent_id) REFERENCES tags(id) ON DELETE SET NULL
	);

	CREATE INDEX IF NOT EXISTS idx_tags_name ON tags(name);
	CREATE INDEX IF NOT EXISTS idx_tags_parent ON tags(parent_id);

	-- File-tag associations
	CREATE TABLE IF NOT EXISTS file_tags (
		file_id INTEGER NOT NULL,
		tag_id INTEGER NOT NULL,
		tagged_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		tagged_by TEXT,                       -- 'user', 'auto', 'ai'
		confidence REAL,
		PRIMARY KEY (file_id, tag_id),
		FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE,
		FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_file_tags_file ON file_tags(file_id);
	CREATE INDEX IF NOT EXISTS idx_file_tags_tag ON file_tags(tag_id);

	-- Duplicate groups
	CREATE TABLE IF NOT EXISTS duplicate_groups (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		content_hash TEXT UNIQUE NOT NULL,
		file_count INTEGER DEFAULT 0,
		total_wasted_size INTEGER DEFAULT 0,
		detected_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		resolved INTEGER DEFAULT 0,
		resolution_notes TEXT,
		kept_file_id INTEGER,
		FOREIGN KEY (kept_file_id) REFERENCES files(id) ON DELETE SET NULL
	);

	CREATE INDEX IF NOT EXISTS idx_dup_groups_hash ON duplicate_groups(content_hash);
	CREATE INDEX IF NOT EXISTS idx_dup_groups_resolved ON duplicate_groups(resolved);

	-- File relationships
	CREATE TABLE IF NOT EXISTS file_relations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source_file_id INTEGER NOT NULL,
		target_file_id INTEGER NOT NULL,
		relation_type TEXT NOT NULL,          -- 'duplicate', 'version', 'derived', 'related', 'similar'
		confidence REAL,
		metadata TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (source_file_id) REFERENCES files(id) ON DELETE CASCADE,
		FOREIGN KEY (target_file_id) REFERENCES files(id) ON DELETE CASCADE,
		UNIQUE(source_file_id, target_file_id, relation_type)
	);

	CREATE INDEX IF NOT EXISTS idx_relations_source ON file_relations(source_file_id);
	CREATE INDEX IF NOT EXISTS idx_relations_target ON file_relations(target_file_id);
	CREATE INDEX IF NOT EXISTS idx_relations_type ON file_relations(relation_type);

	-- Cross-references to other Diane data
	CREATE TABLE IF NOT EXISTS file_xrefs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		file_id INTEGER NOT NULL,
		xref_type TEXT NOT NULL,              -- 'gmail_attachment', 'gmail_download', 'job_output'
		xref_source TEXT NOT NULL,
		xref_id TEXT NOT NULL,
		metadata TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE,
		UNIQUE(file_id, xref_type, xref_id)
	);

	CREATE INDEX IF NOT EXISTS idx_xrefs_file ON file_xrefs(file_id);
	CREATE INDEX IF NOT EXISTS idx_xrefs_lookup ON file_xrefs(xref_type, xref_id);

	-- Activity log
	CREATE TABLE IF NOT EXISTS file_activity (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		file_id INTEGER,
		source TEXT,
		path TEXT NOT NULL,
		action TEXT NOT NULL,                 -- 'registered', 'updated', 'tagged', 'removed', 'verified'
		details TEXT,                         -- JSON with action-specific details
		performed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		performed_by TEXT                     -- 'llm', 'user', 'system'
	);

	CREATE INDEX IF NOT EXISTS idx_activity_file ON file_activity(file_id);
	CREATE INDEX IF NOT EXISTS idx_activity_source ON file_activity(source);
	CREATE INDEX IF NOT EXISTS idx_activity_action ON file_activity(action);
	CREATE INDEX IF NOT EXISTS idx_activity_time ON file_activity(performed_at);
	`

	if _, err := db.conn.Exec(schema); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Create FTS table if not exists
	ftsSchema := `
	CREATE VIRTUAL TABLE IF NOT EXISTS files_fts USING fts5(
		path,
		filename,
		content_text,
		content='files',
		content_rowid='id'
	);
	`
	if _, err := db.conn.Exec(ftsSchema); err != nil {
		return fmt.Errorf("failed to create FTS table: %w", err)
	}

	// Create FTS sync triggers
	triggers := `
	CREATE TRIGGER IF NOT EXISTS files_fts_insert AFTER INSERT ON files BEGIN
		INSERT INTO files_fts(rowid, path, filename, content_text)
		VALUES (new.id, new.path, new.filename, new.content_text);
	END;

	CREATE TRIGGER IF NOT EXISTS files_fts_delete AFTER DELETE ON files BEGIN
		INSERT INTO files_fts(files_fts, rowid, path, filename, content_text)
		VALUES('delete', old.id, old.path, old.filename, old.content_text);
	END;

	CREATE TRIGGER IF NOT EXISTS files_fts_update AFTER UPDATE ON files BEGIN
		INSERT INTO files_fts(files_fts, rowid, path, filename, content_text)
		VALUES('delete', old.id, old.path, old.filename, old.content_text);
		INSERT INTO files_fts(rowid, path, filename, content_text)
		VALUES (new.id, new.path, new.filename, new.content_text);
	END;
	`
	if _, err := db.conn.Exec(triggers); err != nil {
		// Triggers may already exist, that's fine
	}

	return nil
}

// =============================================================================
// File CRUD Operations
// =============================================================================

// RegisterFile inserts or updates a file in the index
// Returns the file ID and whether it was newly created
func (db *DB) RegisterFile(f *File) (id int64, isNew bool, err error) {
	// Check if file exists
	var existingID int64
	err = db.conn.QueryRow("SELECT id FROM files WHERE source = ? AND path = ?", f.Source, f.Path).Scan(&existingID)
	if err != nil && err != sql.ErrNoRows {
		return 0, false, err
	}

	isNew = err == sql.ErrNoRows

	if isNew {
		// Insert new file
		result, err := db.conn.Exec(`
			INSERT INTO files (
				source, source_file_id, path, filename, extension, size, mime_type,
				is_directory, created_at, modified_at, content_hash, partial_hash,
				content_text, content_preview, category, subcategory, indexed_at, status
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			f.Source, f.SourceFileID, f.Path, f.Filename, f.Extension, f.Size, f.MimeType,
			f.IsDirectory, f.CreatedAt, f.ModifiedAt, f.ContentHash, f.PartialHash,
			f.ContentText, f.ContentPreview, f.Category, f.Subcategory, time.Now(), "active",
		)
		if err != nil {
			return 0, false, err
		}
		id, err = result.LastInsertId()
		if err != nil {
			return 0, false, err
		}
	} else {
		// Update existing file
		_, err = db.conn.Exec(`
			UPDATE files SET
				source_file_id = COALESCE(?, source_file_id),
				filename = ?,
				extension = ?,
				size = ?,
				mime_type = COALESCE(?, mime_type),
				is_directory = ?,
				created_at = COALESCE(?, created_at),
				modified_at = ?,
				content_hash = COALESCE(?, content_hash),
				partial_hash = COALESCE(?, partial_hash),
				content_text = COALESCE(?, content_text),
				content_preview = COALESCE(?, content_preview),
				category = COALESCE(?, category),
				subcategory = COALESCE(?, subcategory),
				verified_at = CURRENT_TIMESTAMP,
				status = 'active'
			WHERE id = ?`,
			f.SourceFileID, f.Filename, f.Extension, f.Size, f.MimeType,
			f.IsDirectory, f.CreatedAt, f.ModifiedAt, f.ContentHash, f.PartialHash,
			f.ContentText, f.ContentPreview, f.Category, f.Subcategory, existingID,
		)
		if err != nil {
			return 0, false, err
		}
		id = existingID
	}

	// Log activity
	action := "registered"
	if !isNew {
		action = "updated"
	}
	db.LogActivity(id, f.Source, f.Path, action, "", "llm")

	return id, isNew, nil
}

// GetFile retrieves a file by ID
func (db *DB) GetFile(id int64) (*File, error) {
	f := &File{}
	err := db.conn.QueryRow(`
		SELECT id, source, source_file_id, path, filename, extension, size, mime_type,
			is_directory, created_at, modified_at, content_hash, partial_hash,
			content_text, content_preview, content_indexed_at, category, subcategory,
			ai_summary, ai_tags, ai_analyzed_at, embedding, embedding_model, embedding_at,
			indexed_at, verified_at, status
		FROM files WHERE id = ?`, id,
	).Scan(
		&f.ID, &f.Source, &f.SourceFileID, &f.Path, &f.Filename, &f.Extension, &f.Size, &f.MimeType,
		&f.IsDirectory, &f.CreatedAt, &f.ModifiedAt, &f.ContentHash, &f.PartialHash,
		&f.ContentText, &f.ContentPreview, &f.ContentIndexedAt, &f.Category, &f.Subcategory,
		&f.AISummary, &f.AITags, &f.AIAnalyzedAt, &f.Embedding, &f.EmbeddingModel, &f.EmbeddingAt,
		&f.IndexedAt, &f.VerifiedAt, &f.Status,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return f, err
}

// GetFileByPath retrieves a file by source and path
func (db *DB) GetFileByPath(source, path string) (*File, error) {
	f := &File{}
	err := db.conn.QueryRow(`
		SELECT id, source, source_file_id, path, filename, extension, size, mime_type,
			is_directory, created_at, modified_at, content_hash, partial_hash,
			content_text, content_preview, content_indexed_at, category, subcategory,
			ai_summary, ai_tags, ai_analyzed_at, embedding, embedding_model, embedding_at,
			indexed_at, verified_at, status
		FROM files WHERE source = ? AND path = ?`, source, path,
	).Scan(
		&f.ID, &f.Source, &f.SourceFileID, &f.Path, &f.Filename, &f.Extension, &f.Size, &f.MimeType,
		&f.IsDirectory, &f.CreatedAt, &f.ModifiedAt, &f.ContentHash, &f.PartialHash,
		&f.ContentText, &f.ContentPreview, &f.ContentIndexedAt, &f.Category, &f.Subcategory,
		&f.AISummary, &f.AITags, &f.AIAnalyzedAt, &f.Embedding, &f.EmbeddingModel, &f.EmbeddingAt,
		&f.IndexedAt, &f.VerifiedAt, &f.Status,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return f, err
}

// RemoveFile removes a file from the index
func (db *DB) RemoveFile(id int64) error {
	// Get file info for logging
	var source, path string
	db.conn.QueryRow("SELECT source, path FROM files WHERE id = ?", id).Scan(&source, &path)

	_, err := db.conn.Exec("DELETE FROM files WHERE id = ?", id)
	if err != nil {
		return err
	}

	if source != "" {
		db.LogActivity(nil, source, path, "removed", "", "llm")
	}
	return nil
}

// RemoveFileByPath removes a file by source and path
func (db *DB) RemoveFileByPath(source, path string) error {
	result, err := db.conn.Exec("DELETE FROM files WHERE source = ? AND path = ?", source, path)
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected > 0 {
		db.LogActivity(nil, source, path, "removed", "", "llm")
	}
	return nil
}

// SearchOptions for searching files
type SearchOptions struct {
	Query           string   // Full-text search query
	Sources         []string // Filter by sources
	PathPattern     string   // Path pattern (supports % wildcards)
	FilenamePattern string   // Filename pattern
	Extensions      []string // Filter by extensions
	Categories      []string // Filter by categories
	MimeTypes       []string // Filter by MIME types

	// Tag filters
	RequiredTags []string // All these tags must be present (AND)
	AnyTags      []string // Any of these tags (OR)
	ExcludeTags  []string // None of these tags

	// Size and date filters
	MinSize        int64
	MaxSize        int64
	ModifiedAfter  *time.Time
	ModifiedBefore *time.Time

	// Status
	Statuses      []string // Filter by status (default: ['active'])
	HasDuplicates *bool    // Filter by has duplicates

	// Pagination
	Limit    int
	Offset   int
	OrderBy  string // 'modified', 'size', 'name', 'relevance', 'indexed'
	OrderDir string // 'asc', 'desc'
}

// SearchResult represents a search result with extra info
type SearchResult struct {
	File
	Tags           []string
	HasDuplicates  bool
	DuplicateCount int
}

// SearchFiles performs a search on files
func (db *DB) SearchFiles(opts SearchOptions) (total int, results []*SearchResult, err error) {
	if opts.Limit == 0 {
		opts.Limit = 50
	}
	if len(opts.Statuses) == 0 {
		opts.Statuses = []string{"active"}
	}
	if opts.OrderBy == "" {
		opts.OrderBy = "modified"
	}
	if opts.OrderDir == "" {
		opts.OrderDir = "desc"
	}

	// Build query
	selectCols := `f.id, f.source, f.source_file_id, f.path, f.filename, f.extension,
		f.size, f.mime_type, f.is_directory, f.created_at, f.modified_at,
		f.content_hash, f.partial_hash, f.content_preview, f.category, f.subcategory,
		f.indexed_at, f.status`

	query := "SELECT " + selectCols + " FROM files f"
	countQuery := "SELECT COUNT(*) FROM files f"

	var args []any
	var conditions []string

	// Full-text search
	if opts.Query != "" {
		query += " JOIN files_fts ON files_fts.rowid = f.id"
		countQuery += " JOIN files_fts ON files_fts.rowid = f.id"
		conditions = append(conditions, "files_fts MATCH ?")
		args = append(args, opts.Query)
	}

	// Source filter
	if len(opts.Sources) > 0 {
		placeholders := make([]string, len(opts.Sources))
		for i := range opts.Sources {
			placeholders[i] = "?"
			args = append(args, opts.Sources[i])
		}
		conditions = append(conditions, "f.source IN ("+strings.Join(placeholders, ",")+")")
	}

	// Path pattern
	if opts.PathPattern != "" {
		conditions = append(conditions, "f.path LIKE ?")
		args = append(args, opts.PathPattern)
	}

	// Filename pattern
	if opts.FilenamePattern != "" {
		conditions = append(conditions, "f.filename LIKE ?")
		args = append(args, opts.FilenamePattern)
	}

	// Extensions
	if len(opts.Extensions) > 0 {
		placeholders := make([]string, len(opts.Extensions))
		for i, ext := range opts.Extensions {
			placeholders[i] = "?"
			args = append(args, strings.TrimPrefix(ext, "."))
		}
		conditions = append(conditions, "f.extension IN ("+strings.Join(placeholders, ",")+")")
	}

	// Categories
	if len(opts.Categories) > 0 {
		placeholders := make([]string, len(opts.Categories))
		for i := range opts.Categories {
			placeholders[i] = "?"
			args = append(args, opts.Categories[i])
		}
		conditions = append(conditions, "f.category IN ("+strings.Join(placeholders, ",")+")")
	}

	// MIME types
	if len(opts.MimeTypes) > 0 {
		placeholders := make([]string, len(opts.MimeTypes))
		for i := range opts.MimeTypes {
			placeholders[i] = "?"
			args = append(args, opts.MimeTypes[i])
		}
		conditions = append(conditions, "f.mime_type IN ("+strings.Join(placeholders, ",")+")")
	}

	// Status
	if len(opts.Statuses) > 0 {
		placeholders := make([]string, len(opts.Statuses))
		for i := range opts.Statuses {
			placeholders[i] = "?"
			args = append(args, opts.Statuses[i])
		}
		conditions = append(conditions, "f.status IN ("+strings.Join(placeholders, ",")+")")
	}

	// Size filters
	if opts.MinSize > 0 {
		conditions = append(conditions, "f.size >= ?")
		args = append(args, opts.MinSize)
	}
	if opts.MaxSize > 0 {
		conditions = append(conditions, "f.size <= ?")
		args = append(args, opts.MaxSize)
	}

	// Date filters
	if opts.ModifiedAfter != nil {
		conditions = append(conditions, "f.modified_at >= ?")
		args = append(args, opts.ModifiedAfter)
	}
	if opts.ModifiedBefore != nil {
		conditions = append(conditions, "f.modified_at <= ?")
		args = append(args, opts.ModifiedBefore)
	}

	// Tag filters (using subqueries)
	if len(opts.RequiredTags) > 0 {
		for _, tag := range opts.RequiredTags {
			conditions = append(conditions, `EXISTS (
				SELECT 1 FROM file_tags ft
				JOIN tags t ON t.id = ft.tag_id
				WHERE ft.file_id = f.id AND t.name = ?
			)`)
			args = append(args, tag)
		}
	}
	if len(opts.AnyTags) > 0 {
		placeholders := make([]string, len(opts.AnyTags))
		for i := range opts.AnyTags {
			placeholders[i] = "?"
			args = append(args, opts.AnyTags[i])
		}
		conditions = append(conditions, `EXISTS (
			SELECT 1 FROM file_tags ft
			JOIN tags t ON t.id = ft.tag_id
			WHERE ft.file_id = f.id AND t.name IN (`+strings.Join(placeholders, ",")+`)
		)`)
	}
	if len(opts.ExcludeTags) > 0 {
		placeholders := make([]string, len(opts.ExcludeTags))
		for i := range opts.ExcludeTags {
			placeholders[i] = "?"
			args = append(args, opts.ExcludeTags[i])
		}
		conditions = append(conditions, `NOT EXISTS (
			SELECT 1 FROM file_tags ft
			JOIN tags t ON t.id = ft.tag_id
			WHERE ft.file_id = f.id AND t.name IN (`+strings.Join(placeholders, ",")+`)
		)`)
	}

	// Has duplicates filter
	if opts.HasDuplicates != nil {
		if *opts.HasDuplicates {
			conditions = append(conditions, `f.content_hash IS NOT NULL AND EXISTS (
				SELECT 1 FROM files f2 WHERE f2.content_hash = f.content_hash AND f2.id != f.id
			)`)
		} else {
			conditions = append(conditions, `f.content_hash IS NULL OR NOT EXISTS (
				SELECT 1 FROM files f2 WHERE f2.content_hash = f.content_hash AND f2.id != f.id
			)`)
		}
	}

	// Build WHERE clause
	if len(conditions) > 0 {
		whereClause := " WHERE " + strings.Join(conditions, " AND ")
		query += whereClause
		countQuery += whereClause
	}

	// Get total count
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	if err = db.conn.QueryRow(countQuery, countArgs...).Scan(&total); err != nil {
		return 0, nil, err
	}

	// Order by
	orderCol := "f.modified_at"
	switch opts.OrderBy {
	case "size":
		orderCol = "f.size"
	case "name":
		orderCol = "f.filename"
	case "indexed":
		orderCol = "f.indexed_at"
	case "relevance":
		if opts.Query != "" {
			orderCol = "rank"
		}
	}
	orderDir := "DESC"
	if opts.OrderDir == "asc" {
		orderDir = "ASC"
	}
	query += fmt.Sprintf(" ORDER BY %s %s", orderCol, orderDir)

	// Pagination
	query += " LIMIT ? OFFSET ?"
	args = append(args, opts.Limit, opts.Offset)

	// Execute query
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()

	results = make([]*SearchResult, 0)
	for rows.Next() {
		r := &SearchResult{}
		if err := rows.Scan(
			&r.ID, &r.Source, &r.SourceFileID, &r.Path, &r.Filename, &r.Extension,
			&r.Size, &r.MimeType, &r.IsDirectory, &r.CreatedAt, &r.ModifiedAt,
			&r.ContentHash, &r.PartialHash, &r.ContentPreview, &r.Category, &r.Subcategory,
			&r.IndexedAt, &r.Status,
		); err != nil {
			return 0, nil, err
		}
		results = append(results, r)
	}

	// Fetch tags and duplicate info for results
	for _, r := range results {
		// Get tags
		tags, _ := db.GetFileTags(r.ID)
		r.Tags = make([]string, len(tags))
		for i, t := range tags {
			r.Tags[i] = t.Name
		}

		// Check duplicates
		if r.ContentHash != "" {
			var count int
			db.conn.QueryRow(`
				SELECT COUNT(*) FROM files
				WHERE content_hash = ? AND id != ? AND status = 'active'`,
				r.ContentHash, r.ID,
			).Scan(&count)
			r.HasDuplicates = count > 0
			r.DuplicateCount = count
		}
	}

	return total, results, nil
}

// UpdateFile updates specific fields of a file
func (db *DB) UpdateFile(id int64, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}

	var setClauses []string
	var args []any

	allowedFields := map[string]bool{
		"size": true, "mime_type": true, "modified_at": true,
		"content_hash": true, "partial_hash": true, "content_text": true,
		"content_preview": true, "category": true, "subcategory": true,
		"status": true, "ai_summary": true, "ai_tags": true,
	}

	for field, value := range updates {
		if !allowedFields[field] {
			continue
		}
		setClauses = append(setClauses, field+" = ?")
		args = append(args, value)
	}

	if len(setClauses) == 0 {
		return nil
	}

	args = append(args, id)
	query := "UPDATE files SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"

	_, err := db.conn.Exec(query, args...)
	return err
}

// UpdateFileContent updates the extracted content for a file
func (db *DB) UpdateFileContent(id int64, contentText, contentPreview string) error {
	_, err := db.conn.Exec(`
		UPDATE files SET content_text = ?, content_preview = ?, content_indexed_at = ?
		WHERE id = ?`,
		contentText, contentPreview, time.Now(), id,
	)
	return err
}

// UpdateFileHashes updates the content hashes for a file
func (db *DB) UpdateFileHashes(id int64, contentHash, partialHash string) error {
	_, err := db.conn.Exec(`
		UPDATE files SET content_hash = ?, partial_hash = ? WHERE id = ?`,
		contentHash, partialHash, id,
	)
	return err
}

// UpdateFileStatus updates the status of a file
func (db *DB) UpdateFileStatus(id int64, status string) error {
	_, err := db.conn.Exec("UPDATE files SET status = ? WHERE id = ?", status, id)
	return err
}

// VerifyFile marks a file as verified (still exists)
func (db *DB) VerifyFile(id int64) error {
	_, err := db.conn.Exec("UPDATE files SET verified_at = CURRENT_TIMESTAMP WHERE id = ?", id)
	return err
}

// =============================================================================
// Tag Operations
// =============================================================================

// CreateTag creates a new tag
func (db *DB) CreateTag(t *Tag) (int64, error) {
	result, err := db.conn.Exec(`
		INSERT INTO tags (name, color, description, parent_id, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		t.Name, t.Color, t.Description, t.ParentID, time.Now(),
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// GetTag retrieves a tag by ID
func (db *DB) GetTag(id int64) (*Tag, error) {
	t := &Tag{}
	err := db.conn.QueryRow(`
		SELECT id, name, color, description, parent_id, created_at, usage_count
		FROM tags WHERE id = ?`, id,
	).Scan(&t.ID, &t.Name, &t.Color, &t.Description, &t.ParentID, &t.CreatedAt, &t.UsageCount)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return t, err
}

// GetTagByName retrieves a tag by name
func (db *DB) GetTagByName(name string) (*Tag, error) {
	t := &Tag{}
	err := db.conn.QueryRow(`
		SELECT id, name, color, description, parent_id, created_at, usage_count
		FROM tags WHERE name = ?`, name,
	).Scan(&t.ID, &t.Name, &t.Color, &t.Description, &t.ParentID, &t.CreatedAt, &t.UsageCount)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return t, err
}

// ListTags returns all tags
func (db *DB) ListTags() ([]*Tag, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, color, description, parent_id, created_at, usage_count
		FROM tags ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []*Tag
	for rows.Next() {
		t := &Tag{}
		if err := rows.Scan(&t.ID, &t.Name, &t.Color, &t.Description, &t.ParentID, &t.CreatedAt, &t.UsageCount); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// UpdateTag updates a tag
func (db *DB) UpdateTag(id int64, name, color, description string, parentID *int64) error {
	_, err := db.conn.Exec(`
		UPDATE tags SET name = ?, color = ?, description = ?, parent_id = ?
		WHERE id = ?`,
		name, color, description, parentID, id,
	)
	return err
}

// DeleteTag deletes a tag
func (db *DB) DeleteTag(id int64) error {
	_, err := db.conn.Exec("DELETE FROM tags WHERE id = ?", id)
	return err
}

// MergeTags merges source tag into target tag
func (db *DB) MergeTags(sourceID, targetID int64) (int, error) {
	// Update document_tags to use target tag
	result, err := db.conn.Exec(`
		UPDATE OR IGNORE document_tags SET tag_id = ? WHERE tag_id = ?`,
		targetID, sourceID,
	)
	if err != nil {
		return 0, err
	}

	affected, _ := result.RowsAffected()

	// Delete orphaned document_tags (duplicates that couldn't be updated)
	db.conn.Exec("DELETE FROM document_tags WHERE tag_id = ?", sourceID)

	// Update usage count
	db.conn.Exec(`
		UPDATE tags SET usage_count = (
			SELECT COUNT(*) FROM file_tags WHERE tag_id = ?
		) WHERE id = ?`, targetID, targetID)

	// Delete source tag
	db.conn.Exec("DELETE FROM tags WHERE id = ?", sourceID)

	return int(affected), nil
}

// GetOrCreateTag gets an existing tag or creates it
func (db *DB) GetOrCreateTag(name string) (*Tag, error) {
	tag, err := db.GetTagByName(name)
	if err != nil {
		return nil, err
	}
	if tag != nil {
		return tag, nil
	}

	// Create new tag
	t := &Tag{Name: name}
	id, err := db.CreateTag(t)
	if err != nil {
		return nil, err
	}
	t.ID = id
	t.CreatedAt = time.Now()
	return t, nil
}

// TagFile adds a tag to a file
func (db *DB) TagFile(fileID, tagID int64, taggedBy string, confidence float64) error {
	_, err := db.conn.Exec(`
		INSERT INTO file_tags (file_id, tag_id, tagged_at, tagged_by, confidence)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(file_id, tag_id) DO UPDATE SET
			tagged_by = excluded.tagged_by,
			confidence = excluded.confidence`,
		fileID, tagID, time.Now(), taggedBy, confidence,
	)
	if err != nil {
		return err
	}

	// Update usage count
	_, err = db.conn.Exec("UPDATE tags SET usage_count = usage_count + 1 WHERE id = ?", tagID)
	return err
}

// UntagFile removes a tag from a file
func (db *DB) UntagFile(fileID, tagID int64) error {
	result, err := db.conn.Exec("DELETE FROM file_tags WHERE file_id = ? AND tag_id = ?", fileID, tagID)
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected > 0 {
		_, err = db.conn.Exec("UPDATE tags SET usage_count = usage_count - 1 WHERE id = ? AND usage_count > 0", tagID)
	}
	return err
}

// GetFileTags returns all tags for a file
func (db *DB) GetFileTags(fileID int64) ([]*Tag, error) {
	rows, err := db.conn.Query(`
		SELECT t.id, t.name, t.color, t.description, t.parent_id, t.created_at, t.usage_count
		FROM tags t
		JOIN file_tags ft ON ft.tag_id = t.id
		WHERE ft.file_id = ?
		ORDER BY t.name`, fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []*Tag
	for rows.Next() {
		t := &Tag{}
		if err := rows.Scan(&t.ID, &t.Name, &t.Color, &t.Description, &t.ParentID, &t.CreatedAt, &t.UsageCount); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// =============================================================================
// Duplicate Detection
// =============================================================================

// FindDuplicatesByHash finds files with the same content hash
func (db *DB) FindDuplicatesByHash(contentHash string) ([]*File, error) {
	rows, err := db.conn.Query(`
		SELECT id, source, source_file_id, path, filename, size, modified_at
		FROM files WHERE content_hash = ? AND status = 'active'
		ORDER BY indexed_at`, contentHash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*File
	for rows.Next() {
		f := &File{}
		if err := rows.Scan(&f.ID, &f.Source, &f.SourceFileID, &f.Path, &f.Filename, &f.Size, &f.ModifiedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// DuplicateGroupInfo contains info about a duplicate group
type DuplicateGroupInfo struct {
	ContentHash string
	FileCount   int
	TotalSize   int64
	WastedSize  int64
	Files       []*File
}

// GetDuplicateGroups returns all duplicate groups
func (db *DB) GetDuplicateGroups(minSize int64, sources []string, limit int) ([]*DuplicateGroupInfo, error) {
	if limit == 0 {
		limit = 100
	}

	query := `
		SELECT content_hash, COUNT(*) as cnt, SUM(size) as total, SUM(size) - MIN(size) as wasted
		FROM files
		WHERE content_hash IS NOT NULL AND content_hash != '' AND status = 'active'`

	var args []any

	if minSize > 0 {
		query += " AND size >= ?"
		args = append(args, minSize)
	}

	if len(sources) > 0 {
		placeholders := make([]string, len(sources))
		for i := range sources {
			placeholders[i] = "?"
			args = append(args, sources[i])
		}
		query += " AND source IN (" + strings.Join(placeholders, ",") + ")"
	}

	query += `
		GROUP BY content_hash
		HAVING cnt > 1
		ORDER BY wasted DESC
		LIMIT ?`
	args = append(args, limit)

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []*DuplicateGroupInfo
	for rows.Next() {
		g := &DuplicateGroupInfo{}
		if err := rows.Scan(&g.ContentHash, &g.FileCount, &g.TotalSize, &g.WastedSize); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}

	// Fetch files for each group
	for _, g := range groups {
		g.Files, _ = db.FindDuplicatesByHash(g.ContentHash)
	}

	return groups, rows.Err()
}

// GetDuplicateSummary returns summary statistics about duplicates
type DuplicateSummary struct {
	TotalGroups      int
	TotalDuplicates  int
	TotalWastedSpace int64
	BySource         map[string]struct {
		Count  int
		Wasted int64
	}
}

func (db *DB) GetDuplicateSummary() (*DuplicateSummary, error) {
	summary := &DuplicateSummary{
		BySource: make(map[string]struct {
			Count  int
			Wasted int64
		}),
	}

	// Get total groups and wasted space
	err := db.conn.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(wasted), 0) FROM (
			SELECT SUM(size) - MIN(size) as wasted
			FROM files
			WHERE content_hash IS NOT NULL AND content_hash != '' AND status = 'active'
			GROUP BY content_hash
			HAVING COUNT(*) > 1
		)`,
	).Scan(&summary.TotalGroups, &summary.TotalWastedSpace)
	if err != nil {
		return nil, err
	}

	// Get total duplicate count
	err = db.conn.QueryRow(`
		SELECT COALESCE(SUM(cnt - 1), 0) FROM (
			SELECT COUNT(*) as cnt
			FROM files
			WHERE content_hash IS NOT NULL AND content_hash != '' AND status = 'active'
			GROUP BY content_hash
			HAVING cnt > 1
		)`,
	).Scan(&summary.TotalDuplicates)
	if err != nil {
		return nil, err
	}

	return summary, nil
}

// =============================================================================
// Activity Log
// =============================================================================

// LogActivity logs an activity
func (db *DB) LogActivity(fileID any, source, path, action, details, performedBy string) error {
	var fID *int64
	switch v := fileID.(type) {
	case int64:
		fID = &v
	case *int64:
		fID = v
	}

	_, err := db.conn.Exec(`
		INSERT INTO file_activity (file_id, source, path, action, details, performed_at, performed_by)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		fID, source, path, action, details, time.Now(), performedBy,
	)
	return err
}

// GetRecentActivity returns recent activity
func (db *DB) GetRecentActivity(limit int) ([]*Activity, error) {
	if limit == 0 {
		limit = 50
	}

	rows, err := db.conn.Query(`
		SELECT id, file_id, source, path, action, details, performed_at, performed_by
		FROM file_activity
		ORDER BY performed_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var activities []*Activity
	for rows.Next() {
		a := &Activity{}
		if err := rows.Scan(&a.ID, &a.FileID, &a.Source, &a.Path, &a.Action, &a.Details, &a.PerformedAt, &a.PerformedBy); err != nil {
			return nil, err
		}
		activities = append(activities, a)
	}
	return activities, rows.Err()
}

// =============================================================================
// Stats
// =============================================================================

// Stats represents file index statistics
type Stats struct {
	TotalFiles      int64
	TotalSize       int64
	ByCategory      map[string]CategoryStats
	BySource        map[string]SourceStats
	DuplicateGroups int64
	WastedSpace     int64
}

type CategoryStats struct {
	Count int64
	Size  int64
}

type SourceStats struct {
	Count int64
	Size  int64
}

// GetStats returns statistics about the file index
func (db *DB) GetStats() (*Stats, error) {
	stats := &Stats{
		ByCategory: make(map[string]CategoryStats),
		BySource:   make(map[string]SourceStats),
	}

	// Total files and size
	err := db.conn.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(size), 0)
		FROM files WHERE status = 'active'`,
	).Scan(&stats.TotalFiles, &stats.TotalSize)
	if err != nil {
		return nil, err
	}

	// By category
	rows, err := db.conn.Query(`
		SELECT COALESCE(category, 'unknown'), COUNT(*), COALESCE(SUM(size), 0)
		FROM files WHERE status = 'active'
		GROUP BY category`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var cat string
		var cs CategoryStats
		if err := rows.Scan(&cat, &cs.Count, &cs.Size); err != nil {
			rows.Close()
			return nil, err
		}
		stats.ByCategory[cat] = cs
	}
	rows.Close()

	// By source
	rows, err = db.conn.Query(`
		SELECT source, COUNT(*), COALESCE(SUM(size), 0)
		FROM files WHERE status = 'active'
		GROUP BY source`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var src string
		var ss SourceStats
		if err := rows.Scan(&src, &ss.Count, &ss.Size); err != nil {
			rows.Close()
			return nil, err
		}
		stats.BySource[src] = ss
	}
	rows.Close()

	// Duplicates
	err = db.conn.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(wasted), 0) FROM (
			SELECT SUM(size) - MIN(size) as wasted
			FROM files
			WHERE content_hash IS NOT NULL AND content_hash != '' AND status = 'active'
			GROUP BY content_hash
			HAVING COUNT(*) > 1
		)`,
	).Scan(&stats.DuplicateGroups, &stats.WastedSpace)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// GetLargeFiles returns the largest files
func (db *DB) GetLargeFiles(limit int, minSize int64) ([]*File, error) {
	if limit == 0 {
		limit = 20
	}

	query := `
		SELECT id, source, source_file_id, path, filename, size, modified_at
		FROM files WHERE status = 'active'`

	var args []any
	if minSize > 0 {
		query += " AND size >= ?"
		args = append(args, minSize)
	}

	query += " ORDER BY size DESC LIMIT ?"
	args = append(args, limit)

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*File
	for rows.Next() {
		f := &File{}
		if err := rows.Scan(&f.ID, &f.Source, &f.SourceFileID, &f.Path, &f.Filename, &f.Size, &f.ModifiedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// GetRecentFiles returns recently indexed files
func (db *DB) GetRecentFiles(limit int) ([]*File, error) {
	if limit == 0 {
		limit = 20
	}

	rows, err := db.conn.Query(`
		SELECT id, source, source_file_id, path, filename, size, modified_at, indexed_at
		FROM files WHERE status = 'active'
		ORDER BY indexed_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*File
	for rows.Next() {
		f := &File{}
		if err := rows.Scan(&f.ID, &f.Source, &f.SourceFileID, &f.Path, &f.Filename, &f.Size, &f.ModifiedAt, &f.IndexedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}
