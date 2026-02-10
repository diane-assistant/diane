# Extended File System Architecture

## Overview

A local indexing and metadata layer for filesystem files that provides fast search, tagging, classification, and AI-accessible queries. The index lives in a SQLite database (`~/.diane/files.db`) and stores metadata, tags, and references to files rather than file content.

**Key Principle**: We store *references* to files, not copies. The original files remain in place on the filesystem.

## Goals

1. **Search**: Find files by name, type, tags, content patterns, or metadata
2. **Organization**: Tag and classify files without moving them
3. **Cleanup**: Identify duplicates, large files, old files, empty directories
4. **AI-friendly**: Provide structured data for AI agents to query and manage files
5. **Token efficiency**: Return metadata and summaries instead of full file contents
6. **Cross-reference**: Link files to other Diane data (emails, attachments, etc.)

## Use Cases

### Primary Use Cases
1. **Find files**: "Where is that PDF I downloaded last week?"
2. **Organize projects**: Tag files across different directories as belonging to a project
3. **Cleanup assistance**: "Show me files larger than 100MB that I haven't accessed in a year"
4. **Document search**: Find documents containing specific text or patterns
5. **Duplicate detection**: Find duplicate files wasting disk space
6. **Quick access**: Get file paths for commonly accessed files

### Example Queries AI Can Handle
- "Find all invoices from 2024"
- "Show me large video files I haven't watched"
- "What documents are related to Project X?"
- "Find duplicate photos in my Downloads folder"
- "Clean up old downloads older than 6 months"
- "What files did I work on yesterday?"

## Architecture

```
+------------------------------------------------------------------+
|                          MCP Tools                                |
|  fs_search, fs_index, fs_tag, fs_analyze, fs_cleanup, etc.       |
+------------------------------------------------------------------+
                              |
                              v
+------------------------------------------------------------------+
|                    File System Service Layer                      |
|                (server/mcp/tools/filesystem/)                     |
|  - Manages index reads/writes                                     |
|  - Handles filesystem operations                                  |
|  - Extracts metadata and content                                  |
|  - Computes hashes for duplicate detection                        |
+------------------------------------------------------------------+
                              |
              +---------------+---------------+
              v                               v
+---------------------------+     +---------------------------+
|   Files Index (SQLite)    |     |      Local Filesystem     |
|   ~/.diane/files.db       |     |    (actual files)         |
+---------------------------+     +---------------------------+
```

## Database Schema

```sql
-- ~/.diane/files.db

-- Core file metadata (indexed files)
CREATE TABLE files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path TEXT UNIQUE NOT NULL,        -- Absolute canonical path
    filename TEXT NOT NULL,           -- Base filename
    extension TEXT,                   -- File extension (lowercase, no dot)
    
    -- File attributes
    size INTEGER NOT NULL,            -- Size in bytes
    mime_type TEXT,                   -- Detected MIME type
    is_directory INTEGER DEFAULT 0,
    is_hidden INTEGER DEFAULT 0,      -- Starts with . or in hidden dir
    is_symlink INTEGER DEFAULT 0,
    symlink_target TEXT,              -- If symlink, where it points
    
    -- Timestamps (from filesystem)
    created_at DATETIME,              -- Birth time (if available)
    modified_at DATETIME NOT NULL,    -- Last modification time
    accessed_at DATETIME,             -- Last access time
    
    -- Content identification
    content_hash TEXT,                -- SHA-256 of content (for duplicate detection)
    partial_hash TEXT,                -- Quick hash (first 64KB) for fast comparison
    
    -- Extracted content (for searchable documents)
    content_text TEXT,                -- Extracted plain text (for PDFs, docs, etc.)
    content_preview TEXT,             -- First ~500 chars for quick display
    
    -- Classification
    category TEXT,                    -- Auto-detected: document, image, video, audio, archive, code, data, other
    subcategory TEXT,                 -- More specific: invoice, receipt, photo, screenshot, etc.
    
    -- Index metadata
    indexed_at DATETIME NOT NULL,     -- When we indexed this file
    content_indexed_at DATETIME,      -- When we extracted content (null if not extracted)
    verified_at DATETIME,             -- Last time we verified file still exists
    
    -- Status
    status TEXT DEFAULT 'active',     -- active, missing, moved, deleted
    notes TEXT                        -- User notes about the file
);

CREATE INDEX idx_files_path ON files(path);
CREATE INDEX idx_files_filename ON files(filename);
CREATE INDEX idx_files_extension ON files(extension);
CREATE INDEX idx_files_size ON files(size);
CREATE INDEX idx_files_modified ON files(modified_at);
CREATE INDEX idx_files_category ON files(category);
CREATE INDEX idx_files_hash ON files(content_hash);
CREATE INDEX idx_files_partial_hash ON files(partial_hash);
CREATE INDEX idx_files_status ON files(status);

-- Full-text search on content
CREATE VIRTUAL TABLE files_fts USING fts5(
    path,
    filename, 
    content_text,
    content='files',
    content_rowid='id'
);

-- Triggers to keep FTS in sync
CREATE TRIGGER files_ai AFTER INSERT ON files BEGIN
    INSERT INTO files_fts(rowid, path, filename, content_text) 
    VALUES (new.id, new.path, new.filename, new.content_text);
END;

CREATE TRIGGER files_ad AFTER DELETE ON files BEGIN
    INSERT INTO files_fts(files_fts, rowid, path, filename, content_text) 
    VALUES('delete', old.id, old.path, old.filename, old.content_text);
END;

CREATE TRIGGER files_au AFTER UPDATE ON files BEGIN
    INSERT INTO files_fts(files_fts, rowid, path, filename, content_text) 
    VALUES('delete', old.id, old.path, old.filename, old.content_text);
    INSERT INTO files_fts(rowid, path, filename, content_text) 
    VALUES (new.id, new.path, new.filename, new.content_text);
END;

-- User-defined tags
CREATE TABLE tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,        -- Tag name (lowercase, normalized)
    color TEXT,                       -- Optional color for UI (#RRGGBB)
    description TEXT,                 -- What this tag represents
    created_at DATETIME NOT NULL,
    usage_count INTEGER DEFAULT 0     -- Denormalized for quick stats
);

CREATE INDEX idx_tags_name ON tags(name);

-- File-tag associations (many-to-many)
CREATE TABLE file_tags (
    file_id INTEGER NOT NULL,
    tag_id INTEGER NOT NULL,
    tagged_at DATETIME NOT NULL,
    tagged_by TEXT,                   -- 'user', 'auto', 'ai'
    confidence REAL,                  -- For auto-tags, confidence score (0-1)
    
    PRIMARY KEY (file_id, tag_id),
    FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);

CREATE INDEX idx_file_tags_file ON file_tags(file_id);
CREATE INDEX idx_file_tags_tag ON file_tags(tag_id);

-- Watched directories (for auto-indexing)
CREATE TABLE watched_dirs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path TEXT UNIQUE NOT NULL,        -- Directory path to watch
    recursive INTEGER DEFAULT 1,      -- Watch subdirectories
    include_hidden INTEGER DEFAULT 0, -- Include hidden files
    include_patterns TEXT,            -- JSON array of glob patterns to include
    exclude_patterns TEXT,            -- JSON array of glob patterns to exclude
    auto_tag TEXT,                    -- JSON: tags to auto-apply to files in this dir
    index_content INTEGER DEFAULT 0,  -- Extract text content from files
    enabled INTEGER DEFAULT 1,
    last_scan DATETIME,
    created_at DATETIME NOT NULL
);

-- Duplicate groups (files with same content)
CREATE TABLE duplicate_groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    content_hash TEXT UNIQUE NOT NULL,
    file_count INTEGER DEFAULT 0,
    total_size INTEGER DEFAULT 0,     -- Total wasted space (size * (count-1))
    detected_at DATETIME NOT NULL,
    resolved INTEGER DEFAULT 0,       -- User marked as handled
    kept_file_id INTEGER,             -- Which file the user wants to keep
    
    FOREIGN KEY (kept_file_id) REFERENCES files(id) ON DELETE SET NULL
);

CREATE INDEX idx_duplicates_hash ON duplicate_groups(content_hash);
CREATE INDEX idx_duplicates_resolved ON duplicate_groups(resolved);

-- File relationships (for linking related files)
CREATE TABLE file_relations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_file_id INTEGER NOT NULL,
    target_file_id INTEGER NOT NULL,
    relation_type TEXT NOT NULL,      -- 'duplicate', 'version', 'derived', 'related', 'attachment'
    metadata TEXT,                    -- JSON: additional info about the relation
    created_at DATETIME NOT NULL,
    
    FOREIGN KEY (source_file_id) REFERENCES files(id) ON DELETE CASCADE,
    FOREIGN KEY (target_file_id) REFERENCES files(id) ON DELETE CASCADE,
    UNIQUE(source_file_id, target_file_id, relation_type)
);

CREATE INDEX idx_relations_source ON file_relations(source_file_id);
CREATE INDEX idx_relations_target ON file_relations(target_file_id);

-- Cross-references to other Diane data
CREATE TABLE file_xrefs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_id INTEGER NOT NULL,
    xref_type TEXT NOT NULL,          -- 'gmail_attachment', 'gmail_download', 'job_output'
    xref_id TEXT NOT NULL,            -- ID in the other system (gmail_id, job_id, etc.)
    metadata TEXT,                    -- JSON: additional context
    created_at DATETIME NOT NULL,
    
    FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE,
    UNIQUE(file_id, xref_type, xref_id)
);

CREATE INDEX idx_xrefs_file ON file_xrefs(file_id);
CREATE INDEX idx_xrefs_type ON file_xrefs(xref_type, xref_id);

-- Index statistics and state
CREATE TABLE index_state (
    key TEXT PRIMARY KEY,
    value TEXT,
    updated_at DATETIME NOT NULL
);

-- Activity log for tracking file operations
CREATE TABLE file_activity (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_id INTEGER,                  -- May be null if file was deleted
    path TEXT NOT NULL,               -- Preserved even if file deleted
    action TEXT NOT NULL,             -- 'indexed', 'updated', 'deleted', 'moved', 'tagged', 'untagged'
    details TEXT,                     -- JSON: action-specific details
    performed_at DATETIME NOT NULL
);

CREATE INDEX idx_activity_file ON file_activity(file_id);
CREATE INDEX idx_activity_action ON file_activity(action);
CREATE INDEX idx_activity_time ON file_activity(performed_at);
```

## File Storage

```
~/.diane/
+-- files.db                          # SQLite index database
+-- files/
|   +-- thumbnails/                   # Generated thumbnails for images
|   |   +-- {file_id}.jpg
|   +-- previews/                     # Generated previews for documents
|   |   +-- {file_id}.txt
|   +-- exports/                      # Exported file lists, reports
+-- gmail.db
+-- attachments/
+-- cron.db
```

**Note**: Original files stay in their locations. We only store references.

## MCP Tools

### fs_index
Index files in a directory or specific paths.

```
Input:
  paths: string[]         # Paths to index (files or directories)
  recursive: boolean      # For directories, include subdirectories (default: true)
  include_hidden: boolean # Include hidden files (default: false)
  extract_content: boolean # Extract text from documents (default: false)
  compute_hash: boolean   # Compute content hash for duplicates (default: true)

Output:
  - indexed: number       # Files newly indexed
  - updated: number       # Files with updated metadata
  - skipped: number       # Files skipped (already indexed, unchanged)
  - errors: number        # Files that failed to index
  - duration: string

Behavior:
  1. Walk directory tree (if directory)
  2. For each file:
     a. Check if already indexed (by path)
     b. Compare mtime - skip if unchanged
     c. Stat file for metadata
     d. Detect MIME type
     e. Compute partial hash (first 64KB)
     f. Optionally compute full hash
     g. Optionally extract text content
     h. Categorize file
     i. Insert/update in database
```

### fs_search
Search indexed files with flexible criteria.

```
Input:
  query: string           # Full-text search query (optional)
  path: string            # Path pattern (glob or prefix)
  filename: string        # Filename pattern
  extension: string[]     # File extensions
  category: string[]      # Categories: document, image, video, audio, code, etc.
  tags: string[]          # Required tags (AND)
  any_tags: string[]      # Any of these tags (OR)
  min_size: number        # Minimum size in bytes
  max_size: number        # Maximum size in bytes
  modified_after: string  # ISO date
  modified_before: string # ISO date
  include_missing: boolean # Include files marked as missing (default: false)
  limit: number           # Max results (default: 50)
  order_by: string        # 'modified', 'size', 'name', 'accessed' (default: 'modified')

Output:
  Array of:
    - id, path, filename
    - size, size_human (e.g., "4.2 MB")
    - extension, mime_type
    - category, subcategory
    - modified_at
    - tags (array)
    - content_preview (if available)
    - status

Behavior:
  1. Build SQL query from criteria
  2. Use FTS5 for text search
  3. Return paginated results
```

### fs_get
Get detailed information about a specific file.

```
Input:
  path: string            # File path
  -- OR --
  id: number              # File ID from index

Output:
  - All file metadata
  - Tags with tag metadata
  - Relations (duplicates, versions, etc.)
  - Cross-references (gmail attachments, etc.)
  - Content preview
  - Duplicate info (if duplicates exist)

Behavior:
  1. Look up file by path or ID
  2. Verify file still exists (update status if not)
  3. Gather related data
  4. Return comprehensive info
```

### fs_tag
Add or remove tags from files.

```
Input:
  paths: string[]         # File paths to tag
  -- OR --
  ids: number[]           # File IDs to tag
  -- OR --
  query: object           # Search query (same as fs_search) to tag matching files
  
  add_tags: string[]      # Tags to add
  remove_tags: string[]   # Tags to remove
  create_missing: boolean # Create tags that don't exist (default: true)

Output:
  - modified: number      # Files modified
  - tags_created: string[] # New tags created
  - errors: string[]      # Any errors

Behavior:
  1. Resolve files (by path, ID, or query)
  2. Create missing tags if needed
  3. Add/remove tag associations
  4. Update tag usage counts
```

### fs_tags
List and manage tags.

```
Input:
  action: string          # 'list', 'create', 'update', 'delete', 'merge'
  
  # For list:
  include_unused: boolean # Include tags with 0 files (default: false)
  
  # For create/update:
  name: string
  color: string
  description: string
  
  # For delete:
  name: string
  
  # For merge:
  source: string          # Tag to merge from
  target: string          # Tag to merge into

Output:
  Varies by action:
  - list: Array of tags with usage counts
  - create/update: Tag object
  - delete: { deleted: boolean }
  - merge: { files_retagged: number }
```

### fs_analyze
Analyze indexed files for insights.

```
Input:
  type: string            # 'overview', 'large_files', 'old_files', 'duplicates', 
                          # 'by_extension', 'by_category', 'by_tag', 'empty_dirs'
  path: string            # Limit to path prefix (optional)
  limit: number           # For lists (default: 20)
  
  # For large_files:
  min_size: number        # Minimum size to consider
  
  # For old_files:
  days: number            # Files not modified in N days
  
  # For duplicates:
  min_size: number        # Minimum file size to check (default: 1KB)

Output (varies by type):
  overview:
    - total_files, total_size
    - by_category: { category: { count, size } }
    - by_extension: { ext: { count, size } } (top 10)
    - largest_files: [{ path, size }] (top 5)
    - oldest_files: [{ path, modified_at }] (top 5)
    - duplicate_count, duplicate_wasted_size
  
  large_files:
    - [{ path, size, modified_at, accessed_at }]
  
  old_files:
    - [{ path, size, modified_at, accessed_at, days_old }]
  
  duplicates:
    - [{ hash, count, size, wasted, files: [{ path, modified_at }] }]
  
  empty_dirs:
    - [{ path, has_hidden_files }]
```

### fs_cleanup
Suggest or perform cleanup operations.

```
Input:
  action: string          # 'suggest', 'preview', 'execute'
  type: string            # 'duplicates', 'old_files', 'empty_dirs', 'large_files'
  
  # For suggest/preview:
  path: string            # Limit to path prefix
  
  # For duplicates:
  keep: string            # 'newest', 'oldest', 'shortest_path', 'ask'
  
  # For old_files:
  days: number            # Files older than N days
  
  # For execute:
  file_ids: number[]      # Specific files to delete
  confirm: boolean        # Must be true to actually delete
  trash: boolean          # Move to trash instead of delete (default: true)

Output:
  suggest/preview:
    - files: [{ id, path, size, reason }]
    - total_size: number
    - count: number
  
  execute:
    - deleted: number
    - freed_size: number
    - errors: [{ path, error }]

Behavior:
  - suggest: Return recommendations without changes
  - preview: Show exactly what would be deleted
  - execute: Actually perform deletion (requires confirm=true)
  - Always moves to Trash unless trash=false
```

### fs_watch
Manage watched directories for auto-indexing.

```
Input:
  action: string          # 'list', 'add', 'update', 'remove', 'scan'
  
  # For add/update:
  path: string            # Directory path
  recursive: boolean      # Watch subdirectories
  include_hidden: boolean
  include_patterns: string[] # Glob patterns
  exclude_patterns: string[] # Glob patterns
  auto_tags: string[]     # Tags to auto-apply
  index_content: boolean  # Extract text content
  
  # For remove:
  path: string
  
  # For scan:
  path: string            # Specific directory to scan (or all if omitted)
  full: boolean           # Force full re-index

Output:
  list: Array of watched directories with last scan time
  add/update: Watched directory config
  remove: { removed: boolean }
  scan: { indexed, updated, removed, duration }
```

### fs_relate
Create or query relationships between files.

```
Input:
  action: string          # 'create', 'delete', 'find'
  
  # For create:
  source: string          # Source file path
  target: string          # Target file path
  type: string            # 'version', 'derived', 'related', 'attachment'
  metadata: object        # Additional context
  
  # For delete:
  source: string
  target: string
  type: string
  
  # For find:
  path: string            # Find relations for this file
  type: string            # Filter by relation type

Output:
  create: { created: boolean }
  delete: { deleted: boolean }
  find: [{ source, target, type, metadata }]
```

### fs_verify
Verify indexed files still exist and are unchanged.

```
Input:
  path: string            # Path prefix to verify (or all if omitted)
  fix: boolean            # Update status for missing files (default: true)

Output:
  - verified: number      # Files verified as present
  - missing: number       # Files no longer exist
  - changed: number       # Files with different mtime/size
  - errors: number

Behavior:
  1. Iterate indexed files
  2. Stat each file
  3. Compare mtime/size
  4. Update status or metadata as needed
```

### fs_export
Export file lists or reports.

```
Input:
  type: string            # 'list', 'duplicates', 'cleanup_report', 'tags'
  format: string          # 'json', 'csv', 'markdown', 'text'
  query: object           # Search criteria (optional)
  output: string          # Output path (optional, returns content if omitted)

Output:
  - path: string          # If written to file
  - content: string       # If no output path specified
  - count: number         # Records exported
```

## Content Extraction

### Supported Document Types

| Category | Extensions | Extraction Method |
|----------|------------|-------------------|
| Text | txt, md, rst, log | Direct read |
| Documents | pdf | pdftotext (poppler) |
| Documents | docx | unzip + XML parsing |
| Documents | odt | unzip + XML parsing |
| Spreadsheets | xlsx, csv | First sheet/rows preview |
| Code | go, py, js, ts, etc. | Direct read |
| Config | json, yaml, toml, xml | Structured preview |

### Extraction Limits
- Maximum file size for extraction: 10 MB
- Maximum extracted text: 100 KB per file
- Preview length: 500 characters

### MIME Type Detection
- Use file extension mapping first
- Fall back to `net/http.DetectContentType()` for binary files
- Use magic bytes for common formats (PDF, images, archives)

## Category Classification

### Auto-Categories

| Category | Extensions | Description |
|----------|------------|-------------|
| document | pdf, doc, docx, odt, rtf, txt, md | Text documents |
| spreadsheet | xls, xlsx, csv, ods | Tabular data |
| presentation | ppt, pptx, odp, key | Slides |
| image | jpg, png, gif, webp, svg, raw | Images |
| video | mp4, mov, avi, mkv, webm | Video files |
| audio | mp3, wav, flac, m4a, ogg | Audio files |
| archive | zip, tar, gz, 7z, rar | Compressed files |
| code | go, py, js, ts, rs, c, cpp, java | Source code |
| data | json, xml, yaml, sql, db | Data files |
| executable | exe, app, sh, bin | Executables |
| other | * | Everything else |

### Auto-Subcategories (Examples)
- document/invoice (PDF with "invoice" in name or content)
- document/receipt
- image/screenshot (detected by dimensions/path)
- image/photo (EXIF data present)
- code/config (package.json, Makefile, etc.)

## Duplicate Detection

### Algorithm
1. **Quick filter**: Group files by size
2. **Partial hash**: For files with same size, compute SHA-256 of first 64KB
3. **Full hash**: For files with same partial hash, compute full SHA-256
4. **Group**: Files with identical full hash are duplicates

### Hash Computation
```go
func partialHash(path string) (string, error) {
    f, _ := os.Open(path)
    defer f.Close()
    
    h := sha256.New()
    io.CopyN(h, f, 64*1024) // First 64KB
    return hex.EncodeToString(h.Sum(nil)), nil
}
```

### Duplicate Resolution
- AI can suggest which copy to keep (newest, in best location, etc.)
- User confirms before any deletion
- Deleted files go to Trash by default

## Cross-References

### Gmail Attachments
When `gmail_download_attachment` saves a file:
1. Index the file in files.db
2. Create xref: `{ file_id, xref_type: 'gmail_attachment', xref_id: gmail_id }`
3. AI can later find "files from emails" or "attachments from sender X"

### Job Outputs
When a cron job produces output files:
1. Index the output files
2. Create xref: `{ file_id, xref_type: 'job_output', xref_id: job_id }`

## Implementation Phases

### Phase 1: Foundation (Core Infrastructure)
- [ ] Create `files.db` schema and migrations
- [ ] Build file indexing engine (stat, hash, categorize)
- [ ] Implement `fs_index` with basic metadata
- [ ] Implement `fs_search` with path/name/extension filters
- [ ] Implement `fs_get` for single file info

### Phase 2: Tagging System
- [ ] Implement tag CRUD operations
- [ ] Implement `fs_tag` for adding/removing tags
- [ ] Implement `fs_tags` for tag management
- [ ] Add tag-based search to `fs_search`

### Phase 3: Content & Classification
- [ ] Implement MIME type detection
- [ ] Implement auto-categorization
- [ ] Add content extraction for text documents
- [ ] Add content extraction for PDFs (pdftotext)
- [ ] Set up FTS5 for content search

### Phase 4: Duplicate Detection
- [ ] Implement partial and full hashing
- [ ] Create duplicate detection algorithm
- [ ] Implement `fs_analyze` with duplicates
- [ ] Add duplicate info to `fs_get`

### Phase 5: Cleanup & Analysis
- [ ] Implement `fs_analyze` (overview, large_files, old_files)
- [ ] Implement `fs_cleanup` (suggest, preview, execute)
- [ ] Implement empty directory detection
- [ ] Add Trash integration for safe deletion

### Phase 6: Watching & Automation
- [ ] Implement watched directories table
- [ ] Implement `fs_watch` for directory management
- [ ] Add background scanning (optional, can be cron-based)
- [ ] Implement auto-tagging rules

### Phase 7: Relations & Integration
- [ ] Implement file relations
- [ ] Implement `fs_relate`
- [ ] Add cross-references to Gmail attachments
- [ ] Implement `fs_verify` for index maintenance
- [ ] Implement `fs_export`

## File Structure

```
server/mcp/tools/filesystem/
+-- fs.go                 # Main provider, tool definitions
+-- index.go              # File indexing engine
+-- cache.go              # SQLite cache operations
+-- search.go             # Search implementation
+-- tags.go               # Tag management
+-- analyze.go            # Analysis and statistics
+-- cleanup.go            # Cleanup operations
+-- extract.go            # Content extraction
+-- hash.go               # Hashing utilities
+-- watch.go              # Directory watching
+-- relations.go          # File relationships
+-- FILESYSTEM_PLAN.md    # This file
```

## Configuration

### Default Settings (in Diane config)

```yaml
filesystem:
  # Database location
  db_path: ~/.diane/files.db
  
  # Content extraction
  extract_max_size: 10485760  # 10 MB
  preview_length: 500
  
  # Hashing
  partial_hash_size: 65536    # 64 KB
  
  # Cleanup
  use_trash: true             # Move to Trash instead of delete
  
  # Default exclude patterns
  exclude_patterns:
    - "*.DS_Store"
    - "*.Spotlight-*"
    - "*/.git/*"
    - "*/.svn/*"
    - "*/node_modules/*"
    - "*/__pycache__/*"
    - "*.pyc"
    - "*/.Trash/*"
```

## MCP Resources

```
filesystem://index/stats     # Index statistics
filesystem://tags            # All tags with usage counts
filesystem://watched         # Watched directories
filesystem://duplicates      # Duplicate summary
filesystem://guide           # Usage documentation
```

## MCP Prompts

### fs_organize
```
Help me organize files in a directory by suggesting tags and categories.

Arguments:
  path: Directory to analyze

The prompt will:
1. Scan the directory
2. Analyze file types and patterns
3. Suggest organizational structure
4. Propose tags based on content/names
```

### fs_cleanup_review
```
Review files suggested for cleanup and help decide what to keep or delete.

Arguments:
  type: 'duplicates' | 'old_files' | 'large_files'

The prompt will:
1. Get cleanup suggestions
2. Present each file with context
3. Ask for user decision
4. Summarize cleanup plan
```

### fs_find_similar
```
Find files similar to a given file.

Arguments:
  path: Reference file

The prompt will:
1. Analyze the file
2. Search for similar files (by name, type, content, location)
3. Present matches with similarity reasoning
```

## Error Handling

- **Permission denied**: Skip file, log warning, continue
- **File too large**: Skip content extraction, index metadata only
- **Symlink loop**: Detect and skip
- **Missing file**: Update status to 'missing' in index
- **Database corruption**: Auto-rebuild from scratch

## Security Considerations

- Only index files the user has read access to
- Never modify or delete files without explicit confirmation
- Store only file references, not content copies
- Exclude sensitive paths by default (~/.ssh, ~/.gnupg, etc.)
- Support exclude patterns for sensitive directories

## Performance Considerations

- Use partial hashing for quick duplicate detection
- Batch database operations (insert 100 files per transaction)
- Index content asynchronously (don't block on slow PDF extraction)
- Use FTS5 for efficient text search
- Cache frequently accessed paths in memory
- Limit depth for recursive operations

## Future Enhancements

1. **macOS Integration**
   - Read Finder tags
   - Spotlight metadata
   - Quick Look previews

2. **Image Analysis**
   - EXIF extraction
   - Face detection
   - OCR for images

3. **Smart Folders**
   - Saved searches as virtual folders
   - Dynamic file collections

4. **File Sync Status**
   - iCloud Drive sync status
   - Dropbox sync status
   - Git status for code files

5. **Activity Timeline**
   - Track file access patterns
   - "Files I worked on this week"
   - Recent file history
