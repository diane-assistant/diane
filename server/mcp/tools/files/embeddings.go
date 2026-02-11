package files

import (
	"database/sql"
	"encoding/json"
	"fmt"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

func init() {
	// Register sqlite-vec extension
	sqlite_vec.Auto()
}

// EmbeddingDimension is the dimension for Vertex AI text-embedding-005
const EmbeddingDimension = 768

// FileEmbedding represents a file's vector embedding
type FileEmbedding struct {
	FileID    int64
	Embedding []float32
	Model     string // Model used to generate embedding
}

// InitVectorTable creates the vector table for file embeddings
func (db *DB) InitVectorTable() error {
	// Create the vec0 virtual table for embeddings
	_, err := db.conn.Exec(fmt.Sprintf(`
		CREATE VIRTUAL TABLE IF NOT EXISTS file_embeddings USING vec0(
			file_id INTEGER PRIMARY KEY,
			embedding FLOAT[%d]
		)`, EmbeddingDimension))
	if err != nil {
		return fmt.Errorf("failed to create vector table: %w", err)
	}

	// Track which model was used for each embedding
	_, err = db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS file_embedding_meta (
			file_id INTEGER PRIMARY KEY,
			model TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE
		)`)
	if err != nil {
		return fmt.Errorf("failed to create embedding metadata table: %w", err)
	}

	return nil
}

// UpsertEmbedding stores or updates a file's embedding
func (db *DB) UpsertEmbedding(e *FileEmbedding) error {
	if len(e.Embedding) != EmbeddingDimension {
		return fmt.Errorf("embedding dimension mismatch: got %d, expected %d", len(e.Embedding), EmbeddingDimension)
	}

	// Serialize embedding to JSON for sqlite-vec
	embeddingJSON, err := json.Marshal(e.Embedding)
	if err != nil {
		return fmt.Errorf("failed to serialize embedding: %w", err)
	}

	// Use a transaction for the upsert
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete existing embedding if any
	_, err = tx.Exec("DELETE FROM file_embeddings WHERE file_id = ?", e.FileID)
	if err != nil {
		return fmt.Errorf("failed to delete existing embedding: %w", err)
	}

	// Insert new embedding
	_, err = tx.Exec(
		"INSERT INTO file_embeddings (file_id, embedding) VALUES (?, ?)",
		e.FileID, string(embeddingJSON),
	)
	if err != nil {
		return fmt.Errorf("failed to insert embedding: %w", err)
	}

	// Upsert metadata
	_, err = tx.Exec(`
		INSERT INTO file_embedding_meta (file_id, model, created_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(file_id) DO UPDATE SET model = excluded.model, created_at = CURRENT_TIMESTAMP`,
		e.FileID, e.Model,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert embedding metadata: %w", err)
	}

	return tx.Commit()
}

// DeleteEmbedding removes a file's embedding
func (db *DB) DeleteEmbedding(fileID int64) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec("DELETE FROM file_embeddings WHERE file_id = ?", fileID)
	if err != nil {
		return err
	}

	_, err = tx.Exec("DELETE FROM file_embedding_meta WHERE file_id = ?", fileID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// VectorSearchResult represents a similarity search result
type VectorSearchResult struct {
	FileID   int64
	Distance float64
	File     *File // Optionally populated
}

// VectorSearch performs similarity search using the given embedding
func (db *DB) VectorSearch(embedding []float32, limit int) ([]*VectorSearchResult, error) {
	if len(embedding) != EmbeddingDimension {
		return nil, fmt.Errorf("embedding dimension mismatch: got %d, expected %d", len(embedding), EmbeddingDimension)
	}

	if limit <= 0 {
		limit = 10
	}

	// Serialize query embedding
	embeddingJSON, err := json.Marshal(embedding)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize query embedding: %w", err)
	}

	rows, err := db.conn.Query(`
		SELECT file_id, distance
		FROM file_embeddings
		WHERE embedding MATCH ?
		ORDER BY distance
		LIMIT ?`,
		string(embeddingJSON), limit,
	)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}
	defer rows.Close()

	var results []*VectorSearchResult
	for rows.Next() {
		r := &VectorSearchResult{}
		if err := rows.Scan(&r.FileID, &r.Distance); err != nil {
			return nil, err
		}
		results = append(results, r)
	}

	return results, rows.Err()
}

// VectorSearchWithFiles performs similarity search and includes file metadata
func (db *DB) VectorSearchWithFiles(embedding []float32, limit int) ([]*VectorSearchResult, error) {
	results, err := db.VectorSearch(embedding, limit)
	if err != nil {
		return nil, err
	}

	// Fetch file details for each result
	for _, r := range results {
		file, err := db.GetFile(r.FileID)
		if err != nil {
			return nil, err
		}
		r.File = file
	}

	return results, nil
}

// GetFilesWithoutEmbeddings returns files that need embeddings generated
func (db *DB) GetFilesWithoutEmbeddings(limit int) ([]*File, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := db.conn.Query(`
		SELECT f.id, f.source, f.source_file_id, f.path, f.filename, f.extension,
			f.size, f.mime_type, f.is_directory, f.created_at, f.modified_at,
			f.content_hash, f.partial_hash, f.content_text, f.content_preview,
			f.category, f.subcategory, f.indexed_at, f.status
		FROM files f
		LEFT JOIN file_embedding_meta em ON em.file_id = f.id
		WHERE f.status = 'active'
			AND f.is_directory = 0
			AND f.content_text IS NOT NULL
			AND f.content_text != ''
			AND em.file_id IS NULL
		ORDER BY f.modified_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*File
	for rows.Next() {
		f := &File{}
		if err := rows.Scan(
			&f.ID, &f.Source, &f.SourceFileID, &f.Path, &f.Filename, &f.Extension,
			&f.Size, &f.MimeType, &f.IsDirectory, &f.CreatedAt, &f.ModifiedAt,
			&f.ContentHash, &f.PartialHash, &f.ContentText, &f.ContentPreview,
			&f.Category, &f.Subcategory, &f.IndexedAt, &f.Status,
		); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// GetEmbeddingStats returns statistics about embeddings
type EmbeddingStats struct {
	TotalEmbeddings    int64
	FilesWithContent   int64
	FilesNeedEmbedding int64
	EmbeddingsByModel  map[string]int64
}

func (db *DB) GetEmbeddingStats() (*EmbeddingStats, error) {
	stats := &EmbeddingStats{
		EmbeddingsByModel: make(map[string]int64),
	}

	// Total embeddings
	err := db.conn.QueryRow("SELECT COUNT(*) FROM file_embeddings").Scan(&stats.TotalEmbeddings)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	// Files with extractable content
	err = db.conn.QueryRow(`
		SELECT COUNT(*) FROM files
		WHERE status = 'active' AND is_directory = 0
		AND content_text IS NOT NULL AND content_text != ''`,
	).Scan(&stats.FilesWithContent)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	// Files needing embeddings
	err = db.conn.QueryRow(`
		SELECT COUNT(*) FROM files f
		LEFT JOIN file_embedding_meta em ON em.file_id = f.id
		WHERE f.status = 'active' AND f.is_directory = 0
		AND f.content_text IS NOT NULL AND f.content_text != ''
		AND em.file_id IS NULL`,
	).Scan(&stats.FilesNeedEmbedding)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	// By model
	rows, err := db.conn.Query("SELECT model, COUNT(*) FROM file_embedding_meta GROUP BY model")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var model string
		var count int64
		if err := rows.Scan(&model, &count); err != nil {
			return nil, err
		}
		stats.EmbeddingsByModel[model] = count
	}

	return stats, rows.Err()
}
