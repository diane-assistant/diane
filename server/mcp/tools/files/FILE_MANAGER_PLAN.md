# File Manager Architecture

## Overview

A unified file indexing and management layer that stores metadata about files from multiple sources (local filesystem, Google Drive, Gmail, remote servers) into a single searchable index. This layer handles tagging, classification, cross-source duplicate detection, and source-agnostic operations.

**Key Principle**: The File Manager is a **passive index**. It doesn't discover or fetch files itself - it stores references and metadata that are provided to it.

## Goals

1. **Unified Search**: Find files across all sources with one query
2. **Cross-Source Organization**: Tag and classify files regardless of location
3. **Duplicate Detection**: Find duplicates across different sources (same file on Drive and local)
4. **Source Abstraction**: AI doesn't need to know where a file lives to work with it
5. **Extensibility**: Easy to add new sources as simple string identifiers

---

## Evolution Strategy: Two-Stage Architecture

This system will be built in two stages, allowing us to ship functionality quickly while building toward a more powerful architecture.

### Stage 1: Passive Index (Initial Implementation)

In this stage, the File Manager is a **passive metadata store**. It does NOT have internal providers that fetch files. Instead:

- The LLM discovers files using **existing tools** (shell commands, Google Drive MCP, Gmail MCP, etc.)
- The LLM then **registers** those files with the File Manager, providing metadata
- The File Manager just stores references, tags, and relationships

```
┌─────────────────────────────────────────────────────────────────────────┐
│                                  LLM                                     │
│                                                                          │
│   Discovers files using existing tools:                                 │
│   - Shell: find ~/Downloads -name "*.pdf"                               │
│   - Shell: sha256sum /path/to/file                                      │
│   - Shell: file /path/to/file (for mime type)                           │
│   - Shell: stat /path/to/file (for size, dates)                         │
│   - Drive MCP: google_drive_list, google_drive_search                   │
│   - Gmail MCP: gmail_list_attachments                                   │
│                                                                          │
│   Then registers with File Manager:                                     │
│   - file_register(source="local", path="...", size=X, hash="...", ...)  │
│   - file_tag(id=123, add_tags=["invoice", "2024"])                      │
│                                                                          │
└──────────────────────────────────┬──────────────────────────────────────┘
                                   ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                        File Manager MCP                                  │
│                                                                          │
│  Passive index - stores metadata provided by LLM:                       │
│  - File references (source + path)                                      │
│  - Metadata (size, type, dates, hash)                                   │
│  - Tags and categories                                                  │
│  - Duplicate detection by hash                                          │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
                           [SQLite: files.db]
```

**Characteristics:**
- File Manager has no knowledge of how to access files
- Source is just a string identifier (e.g., "local", "gdrive", "gmail")
- LLM is responsible for gathering metadata before registration
- Simple to implement - just CRUD operations on SQLite
- No provider code, no API integrations

### Stage 2: Active Providers (Future)

In this stage, providers become **standalone MCPs** that the File Manager can optionally call via MCP protocol. This enables:

- Automatic file discovery and syncing
- Providers running on remote machines
- Language-agnostic provider implementation
- Dynamic provider registration

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              Diane Server                                │
│                                                                          │
│  ┌───────────────────────────────────────────────────────────────────┐ │
│  │                        File Manager MCP                            │ │
│  │                                                                    │ │
│  │   ┌────────────────────────────────────────────────────────────┐  │ │
│  │   │              MCP Client (internal)                          │  │ │
│  │   │         Calls source MCPs via MCP protocol                  │  │ │
│  │   └────────────────────────────────────────────────────────────┘  │ │
│  │                              │                                     │ │
│  │         ┌────────────────────┼────────────────────┐               │ │
│  │         ▼                    ▼                    ▼               │ │
│  │   [MCP call]           [MCP call]           [MCP call]           │ │
│  └─────────│────────────────────│────────────────────│───────────────┘ │
│            │                    │                    │                  │
│            ▼                    ▼                    ▼                  │
│  ┌──────────────────┐ ┌──────────────────┐ ┌──────────────────┐       │
│  │  Local FS MCP    │ │  Google Drive    │ │  Remote Server   │       │
│  │  (builtin)       │ │  MCP (builtin)   │ │  MCP (external)  │       │
│  └──────────────────┘ └──────────────────┘ └──────────────────┘       │
│            │                    │                    │                  │
└────────────│────────────────────│────────────────────│──────────────────┘
             ▼                    ▼                    ▼
       [Filesystem]          [Drive API]         [SSH/Remote]
```

**Characteristics:**
- Providers are MCPs implementing a standard tool interface
- Communication via MCP protocol (JSON-RPC)
- Can be builtin or external
- Dynamic registration at runtime
- Language-agnostic (any MCP-compatible implementation)

---

## Stage 1: Passive Index Details

### Workflow Example: Organizing Downloads

```
User: "Find all invoices in my Downloads and tag them"

LLM:
1. Discover files with shell commands:
   $ find ~/Downloads -type f -name "*invoice*" -o -name "*faktura*"
   $ find ~/Downloads -type f -name "*.pdf"

2. For each file, gather metadata:
   $ file /path/to/file                    → gets mime type
   $ stat -f "%z %m" /path/to/file         → gets size, mtime
   $ sha256sum /path/to/file               → gets hash

3. Register with File Manager:
   file_register(
     source="local",
     path="/Users/me/Downloads/invoice-2024.pdf",
     size=45231,
     mime_type="application/pdf",
     hash="abc123...",
     modified_at="2024-01-15T10:30:00Z"
   )

4. Tag the file:
   file_tag(
     path="/Users/me/Downloads/invoice-2024.pdf",
     add_tags=["invoice", "2024", "to-review"]
   )

5. Repeat for all files

6. "Registered and tagged 12 invoice files"
```

### Workflow Example: Cross-Source Duplicates

```
User: "Do I have any files on Google Drive that are also on my local machine?"

LLM:
1. Search Drive with existing Drive MCP:
   google_drive_list(path="/")
   → Gets list of files with metadata including md5Checksum

2. For each Drive file, register with File Manager:
   file_register(
     source="gdrive",
     source_file_id="1abc...",
     path="/My Drive/Documents/report.pdf",
     size=123456,
     hash="sha256:...",  # Convert from md5 or compute
     ...
   )

3. Query for duplicates:
   file_duplicates()
   → Returns groups of files with same hash across sources

4. "Found 5 duplicate groups across local and Google Drive"
```

---

## Database Schema (Stage 1)

```sql
-- ~/.diane/files.db

-- Unified file index
-- Source is just a string, not a foreign key
CREATE TABLE files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    
    -- Source reference (simple string)
    source TEXT NOT NULL,             -- "local", "gdrive", "gmail", etc.
    source_file_id TEXT,              -- Provider-specific file ID (optional)
    
    -- Universal identifiers
    path TEXT NOT NULL,               -- Full path within source
    filename TEXT NOT NULL,           -- Base filename
    extension TEXT,                   -- Lowercase, no dot
    
    -- File attributes
    size INTEGER NOT NULL,
    mime_type TEXT,
    is_directory INTEGER DEFAULT 0,
    
    -- Timestamps (from source)
    created_at DATETIME,
    modified_at DATETIME NOT NULL,
    
    -- Content identification (for cross-source duplicate detection)
    content_hash TEXT,                -- SHA-256 of full content
    partial_hash TEXT,                -- SHA-256 of first 64KB (for quick comparison)
    
    -- Extracted content
    content_text TEXT,                -- Extracted plain text
    content_preview TEXT,             -- First ~500 chars
    content_indexed_at DATETIME,
    
    -- Classification
    category TEXT,                    -- document, image, video, audio, code, archive, data, other
    subcategory TEXT,                 -- invoice, receipt, photo, screenshot, etc.
    
    -- AI-generated metadata
    ai_summary TEXT,
    ai_tags TEXT,                     -- JSON: AI-suggested tags
    ai_analyzed_at DATETIME,
    
    -- Embeddings (for semantic search)
    embedding BLOB,                   -- Vector embedding of content
    embedding_model TEXT,             -- Model used to generate embedding
    embedding_at DATETIME,
    
    -- Index metadata
    indexed_at DATETIME NOT NULL,
    verified_at DATETIME,
    status TEXT DEFAULT 'active',     -- active, missing, moved, deleted
    
    UNIQUE(source, path)
);

-- Indexes
CREATE INDEX idx_files_source ON files(source);
CREATE INDEX idx_files_path ON files(path);
CREATE INDEX idx_files_filename ON files(filename);
CREATE INDEX idx_files_extension ON files(extension);
CREATE INDEX idx_files_size ON files(size);
CREATE INDEX idx_files_modified ON files(modified_at);
CREATE INDEX idx_files_category ON files(category);
CREATE INDEX idx_files_content_hash ON files(content_hash);
CREATE INDEX idx_files_partial_hash ON files(partial_hash);
CREATE INDEX idx_files_status ON files(status);

-- Full-text search
CREATE VIRTUAL TABLE files_fts USING fts5(
    path,
    filename,
    content_text,
    content='files',
    content_rowid='id'
);

-- FTS sync triggers
CREATE TRIGGER files_fts_insert AFTER INSERT ON files BEGIN
    INSERT INTO files_fts(rowid, path, filename, content_text)
    VALUES (new.id, new.path, new.filename, new.content_text);
END;

CREATE TRIGGER files_fts_delete AFTER DELETE ON files BEGIN
    INSERT INTO files_fts(files_fts, rowid, path, filename, content_text)
    VALUES('delete', old.id, old.path, old.filename, old.content_text);
END;

CREATE TRIGGER files_fts_update AFTER UPDATE ON files BEGIN
    INSERT INTO files_fts(files_fts, rowid, path, filename, content_text)
    VALUES('delete', old.id, old.path, old.filename, old.content_text);
    INSERT INTO files_fts(rowid, path, filename, content_text)
    VALUES (new.id, new.path, new.filename, new.content_text);
END;

-- User-defined tags
CREATE TABLE tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    color TEXT,
    description TEXT,
    parent_id INTEGER,
    created_at DATETIME NOT NULL,
    usage_count INTEGER DEFAULT 0,
    
    FOREIGN KEY (parent_id) REFERENCES tags(id) ON DELETE SET NULL
);

CREATE INDEX idx_tags_name ON tags(name);
CREATE INDEX idx_tags_parent ON tags(parent_id);

-- File-tag associations
CREATE TABLE file_tags (
    file_id INTEGER NOT NULL,
    tag_id INTEGER NOT NULL,
    tagged_at DATETIME NOT NULL,
    tagged_by TEXT,                   -- 'user', 'auto', 'ai'
    confidence REAL,
    
    PRIMARY KEY (file_id, tag_id),
    FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);

CREATE INDEX idx_file_tags_file ON file_tags(file_id);
CREATE INDEX idx_file_tags_tag ON file_tags(tag_id);

-- Duplicate groups (cross-source)
CREATE TABLE duplicate_groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    content_hash TEXT UNIQUE NOT NULL,
    file_count INTEGER DEFAULT 0,
    total_wasted_size INTEGER DEFAULT 0,
    detected_at DATETIME NOT NULL,
    resolved INTEGER DEFAULT 0,
    resolution_notes TEXT,
    kept_file_id INTEGER,
    
    FOREIGN KEY (kept_file_id) REFERENCES files(id) ON DELETE SET NULL
);

CREATE INDEX idx_dup_groups_hash ON duplicate_groups(content_hash);
CREATE INDEX idx_dup_groups_resolved ON duplicate_groups(resolved);

-- File relationships
CREATE TABLE file_relations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_file_id INTEGER NOT NULL,
    target_file_id INTEGER NOT NULL,
    relation_type TEXT NOT NULL,      -- 'duplicate', 'version', 'derived', 'related', 'similar'
    confidence REAL,
    metadata TEXT,
    created_at DATETIME NOT NULL,
    
    FOREIGN KEY (source_file_id) REFERENCES files(id) ON DELETE CASCADE,
    FOREIGN KEY (target_file_id) REFERENCES files(id) ON DELETE CASCADE,
    UNIQUE(source_file_id, target_file_id, relation_type)
);

CREATE INDEX idx_relations_source ON file_relations(source_file_id);
CREATE INDEX idx_relations_target ON file_relations(target_file_id);
CREATE INDEX idx_relations_type ON file_relations(relation_type);

-- Cross-references to other Diane data
CREATE TABLE file_xrefs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_id INTEGER NOT NULL,
    xref_type TEXT NOT NULL,          -- 'gmail_attachment', 'gmail_download', 'job_output'
    xref_source TEXT NOT NULL,
    xref_id TEXT NOT NULL,
    metadata TEXT,
    created_at DATETIME NOT NULL,
    
    FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE,
    UNIQUE(file_id, xref_type, xref_id)
);

CREATE INDEX idx_xrefs_file ON file_xrefs(file_id);
CREATE INDEX idx_xrefs_lookup ON file_xrefs(xref_type, xref_id);

-- Activity log
CREATE TABLE file_activity (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_id INTEGER,
    source TEXT,
    path TEXT NOT NULL,
    action TEXT NOT NULL,             -- 'registered', 'updated', 'tagged', 'removed', 'verified'
    details TEXT,                     -- JSON with action-specific details
    performed_at DATETIME NOT NULL,
    performed_by TEXT                 -- 'llm', 'user', 'system'
);

CREATE INDEX idx_activity_file ON file_activity(file_id);
CREATE INDEX idx_activity_source ON file_activity(source);
CREATE INDEX idx_activity_action ON file_activity(action);
CREATE INDEX idx_activity_time ON file_activity(performed_at);
```

---

## MCP Tools (Stage 1)

### file_register
Register a file in the index with metadata provided by the LLM.

```yaml
Input:
  # Required
  source: string              # Source identifier: "local", "gdrive", "gmail", etc.
  path: string                # Full path within the source
  size: integer               # File size in bytes
  modified_at: string         # ISO 8601 datetime
  
  # Optional
  source_file_id: string      # Provider-specific file ID
  filename: string            # Override filename (extracted from path if omitted)
  mime_type: string           # MIME type
  is_directory: boolean       # Default: false
  created_at: string          # ISO 8601 datetime
  
  # Content identification
  content_hash: string        # SHA-256 of full content
  partial_hash: string        # SHA-256 of first 64KB
  
  # Content
  content_text: string        # Extracted plain text
  content_preview: string     # First ~500 chars (auto-generated if content_text provided)
  
  # Classification
  category: string            # document, image, video, audio, code, archive, data, other
  subcategory: string         # More specific type

Output:
  id: integer                 # File ID
  path: string
  source: string
  is_new: boolean             # True if newly registered, false if updated
  duplicate_of: integer       # File ID if content_hash matches existing
```

### file_update
Update an existing file's metadata.

```yaml
Input:
  # Identification (one required)
  id: integer                 # File ID
  # OR
  source: string              # Source + path
  path: string

  # Updatable fields
  size: integer
  modified_at: string
  mime_type: string
  content_hash: string
  partial_hash: string
  content_text: string
  category: string
  subcategory: string
  status: string              # 'active', 'missing', 'moved', 'deleted'

Output:
  id: integer
  updated_fields: string[]
```

### file_remove
Remove a file from the index (does not delete actual file).

```yaml
Input:
  # Identification (one required)
  id: integer
  # OR
  source: string
  path: string
  
  # Options
  cascade: boolean            # Remove related records (default: true)

Output:
  removed: boolean
  id: integer
  path: string
```

### file_get
Get detailed file information.

```yaml
Input:
  # Identification (one required)
  id: integer
  # OR
  source: string
  path: string
  
  # Options
  include_tags: boolean       # Default: true
  include_duplicates: boolean # Default: true
  include_relations: boolean  # Default: false
  include_content: boolean    # Include content_text (default: false)

Output:
  id: integer
  source: string
  source_file_id: string
  path: string
  filename: string
  extension: string
  size: integer
  size_human: string          # "1.2 MB"
  mime_type: string
  is_directory: boolean
  created_at: string
  modified_at: string
  content_hash: string
  category: string
  subcategory: string
  indexed_at: string
  status: string
  
  # Optional fields
  tags: [{name, color, tagged_at, tagged_by}]
  duplicates: [{id, source, path, size}]
  relations: [{id, source, path, relation_type}]
  content_preview: string
  content_text: string
```

### file_search
Search files across all sources.

```yaml
Input:
  # Text search
  query: string               # Full-text search query
  
  # Filters
  source: string[]            # Limit to sources
  path: string                # Path pattern (glob-style)
  filename: string            # Filename pattern
  extension: string[]         # Extensions (without dot)
  category: string[]          # Categories
  mime_type: string[]         # MIME types
  
  # Tag filters
  tags: string[]              # Required tags (AND)
  any_tags: string[]          # Any of these tags (OR)
  exclude_tags: string[]      # Exclude these tags
  
  # Size and date filters
  min_size: integer
  max_size: integer
  modified_after: string
  modified_before: string
  
  # Status
  status: string[]            # Default: ['active']
  has_duplicates: boolean
  
  # Pagination
  limit: integer              # Default: 50
  offset: integer
  order_by: string            # 'modified', 'size', 'name', 'relevance', 'indexed'
  order_dir: string           # 'asc', 'desc'

Output:
  total: integer
  files: [{
    id, source, path, filename,
    size, size_human, mime_type, category,
    modified_at, indexed_at,
    tags: [string],
    content_preview,
    has_duplicates, duplicate_count
  }]
```

### file_tag
Add or remove tags from files.

```yaml
Input:
  # Target (one required)
  ids: integer[]              # File IDs
  # OR
  source: string              # Source + path
  path: string
  # OR
  query: object               # Search query to match files
  
  # Tag operations
  add_tags: string[]
  remove_tags: string[]
  
  # Options
  create_missing: boolean     # Create tags that don't exist (default: true)
  tagged_by: string           # 'user', 'ai', 'auto' (default: 'user')

Output:
  modified: integer           # Number of files modified
  tags_created: string[]      # Newly created tags
  errors: string[]
```

### file_tags
Manage tag definitions.

```yaml
Input:
  action: string              # 'list', 'create', 'update', 'delete', 'merge'
  
  # For create/update
  name: string
  color: string               # Hex color code
  description: string
  parent: string              # Parent tag name for hierarchy
  
  # For delete
  force: boolean              # Delete even if in use
  
  # For merge
  source_tag: string          # Tag to merge from
  target_tag: string          # Tag to merge into

Output:
  list: [{id, name, color, description, parent, usage_count}]
  created: {id, name}
  merged: {files_updated: integer}
```

### file_duplicates
Find and manage duplicate files across sources.

```yaml
Input:
  action: string              # 'list', 'analyze', 'mark_resolved', 'find_for_file'
  
  # For list/analyze
  min_size: integer           # Minimum file size
  source: string[]            # Limit to sources
  unresolved_only: boolean    # Default: true
  limit: integer
  
  # For mark_resolved
  group_id: integer           # Duplicate group ID
  kept_id: integer            # File ID to keep
  notes: string               # Resolution notes
  
  # For find_for_file
  id: integer                 # File ID
  # OR
  content_hash: string        # Content hash

Output:
  groups: [{
    id: integer,
    content_hash: string,
    file_count: integer,
    wasted_size: integer,
    wasted_size_human: string,
    resolved: boolean,
    files: [{id, source, path, size, modified_at}]
  }]
  
  # For analyze
  summary: {
    total_groups: integer,
    total_duplicates: integer,
    total_wasted: integer,
    wasted_human: string,
    by_source: [{source, count, wasted}]
  }
```

### file_analyze
Get insights and statistics about indexed files.

```yaml
Input:
  type: string                # 'overview', 'by_source', 'by_category', 'by_extension',
                              # 'by_tag', 'large_files', 'old_files', 'recent'
  
  # Filters
  source: string[]
  path: string
  
  # For large_files/old_files/recent
  limit: integer              # Default: 20
  min_size: integer           # For large_files
  older_than: string          # For old_files (ISO date or "30d", "1y")
  newer_than: string          # For recent

Output:
  # Varies by type
  
  overview: {
    total_files: integer,
    total_size: integer,
    total_size_human: string,
    by_source: [{source, count, size}],
    by_category: [{category, count, size}],
    duplicate_groups: integer,
    wasted_space: integer
  }
  
  by_source/by_category/by_extension/by_tag: [{
    name: string,
    count: integer,
    size: integer,
    size_human: string
  }]
  
  large_files/old_files/recent: [{
    id, source, path, size, size_human, modified_at
  }]
```

### file_verify
Verify that indexed files still exist and update status.

```yaml
Input:
  # Scope (optional - defaults to all)
  source: string              # Limit to source
  path: string                # Path prefix
  ids: integer[]              # Specific file IDs
  
  # Options
  check_hash: boolean         # Re-verify content hash (default: false)
  update_metadata: boolean    # Update size/mtime if changed (default: true)
  mark_missing: boolean       # Mark missing files (default: true)

Output:
  verified: integer
  missing: integer
  changed: integer
  errors: [{id, path, error}]
```

---

## MCP Resources

```
files://stats                 # Overall statistics
files://tags                  # All tags with usage counts
files://duplicates            # Duplicate summary
files://recent                # Recently indexed files
files://guide                 # Usage documentation
```

---

## File Structure (Stage 1)

```
server/mcp/tools/files/
├── files.go                  # MCP tool definitions
├── manager.go                # File Manager core
├── db.go                     # SQLite operations
├── search.go                 # Search implementation
├── tags.go                   # Tag management
├── duplicates.go             # Duplicate detection
├── embeddings.go             # Vector embeddings (for semantic search)
├── embedding_service.go      # Embedding service interface
├── vertex_embeddings.go      # Google Vertex AI embeddings
├── FILE_MANAGER_PLAN.md      # This file
│
└── providers/                # Stage 2 provider specs (for future reference)
    ├── local/
    │   └── LOCAL_FS_PROVIDER_PLAN.md
    └── gdrive/
        └── GDRIVE_PROVIDER_PLAN.md
```

---

## Configuration

```yaml
files:
  db_path: ~/.diane/files.db
  
  # Content extraction settings
  preview_length: 500         # Characters for content_preview
  
  # Embedding settings
  embedding_enabled: true
  embedding_model: "text-embedding-004"  # Vertex AI model
  embedding_dimensions: 768
  
  # Duplicate detection
  detect_duplicates: true
  
  # Default filters
  default_status: ["active"]
  default_limit: 50
  max_limit: 500
```

---

## Implementation Roadmap

### Stage 1: Passive Index

#### Phase 1.1: Core Infrastructure
- [x] Database schema and migrations (`db.go`)
- [ ] Basic CRUD operations
- [ ] File registration (`file_register`)
- [ ] File retrieval (`file_get`)

#### Phase 1.2: Search
- [ ] Full-text search setup (FTS5)
- [ ] `file_search` implementation
- [ ] Filter combinations
- [ ] Pagination

#### Phase 1.3: Tagging
- [ ] Tag CRUD (`tags.go`)
- [ ] `file_tag` - tag files
- [ ] `file_tags` - manage tags
- [ ] Tag-based search filters

#### Phase 1.4: Duplicate Detection
- [ ] Hash-based grouping (`duplicates.go`)
- [ ] Auto-detect on registration
- [ ] `file_duplicates` tool
- [ ] Duplicate group management

#### Phase 1.5: Analysis & Verification
- [ ] `file_analyze` - statistics
- [ ] `file_verify` - check file existence
- [ ] Activity logging

#### Phase 1.6: Embeddings (Optional)
- [x] Embedding service interface (`embedding_service.go`)
- [x] Vertex AI integration (`vertex_embeddings.go`)
- [ ] Semantic search
- [ ] Similar file finding

### Stage 2: Active Providers (Future)

See provider specs in `providers/` directory:
- `providers/local/LOCAL_FS_PROVIDER_PLAN.md`
- `providers/gdrive/GDRIVE_PROVIDER_PLAN.md`

---

## Security Considerations

- Never store file contents in the index (only metadata + extracted text)
- The LLM is responsible for access control - File Manager trusts what it's told
- Require confirmation for destructive operations
- Activity logging for audit trails

---

## Example LLM Workflows

### 1. Index Local Directory

```
User: "Index my Documents folder"

LLM steps:
1. $ find ~/Documents -type f | head -100  # Get list of files
2. For each file:
   $ stat -f "%z %m" /path/to/file
   $ file --mime-type /path/to/file
3. file_register(source="local", path="...", size=X, mime_type="...", ...)
4. Report summary
```

### 2. Find PDFs Across Sources

```
User: "Find all PDFs I have"

LLM steps:
1. file_search(extension=["pdf"])
2. If few results, also check sources not yet indexed:
   $ find ~/Documents ~/Downloads -name "*.pdf"
   google_drive_search(query="mimeType='application/pdf'")
3. Register any new finds
4. Return combined results
```

### 3. Tag Files by Content

```
User: "Tag my invoices appropriately"

LLM steps:
1. file_search(tags=["invoice"])  # Find already tagged
2. file_search(filename="*invoice*")  # Find by name
3. For untagged PDFs, extract text and analyze:
   $ pdftotext /path/to/file -
   # AI analysis of content
4. file_tag(ids=[...], add_tags=["invoice", "2024", "vendor:acme"])
```

### 4. Find Cross-Source Duplicates

```
User: "Do I have duplicates between local and Drive?"

LLM steps:
1. Ensure both sources are indexed with hashes
2. file_duplicates(action="analyze")
3. file_duplicates(action="list", source=["local", "gdrive"])
4. Present results with recommendations
```
