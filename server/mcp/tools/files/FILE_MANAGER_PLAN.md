# File Registry Architecture

## Overview

A unified file indexing and management layer that stores metadata about files from multiple sources (local filesystem, Google Drive, Gmail, Dropbox, etc.) into a single searchable index. This layer handles tagging, classification, cross-source duplicate detection, and source-agnostic operations.

**Key Principle**: The File Registry is a **passive index**. It doesn't discover or fetch files itself — it stores references and metadata that are provided to it.

**Backend**: [Emergent Knowledge Base](https://emergentintegrations.ai) — a graph database with built-in vector search, full-text search, and tagging via labels. All file objects are stored as Emergent graph objects of type `"file"`.

**Provider Name**: `file_registry` (all tool names use the `file_registry_` prefix).

## Goals

1. **Unified Search**: Find files across all sources with one query
2. **Cross-Source Organization**: Tag and classify files regardless of location
3. **Duplicate Detection**: Find duplicates across different sources (same file on Drive and local)
4. **Source Abstraction**: AI doesn't need to know where a file lives to work with it
5. **Extensibility**: Easy to add new sources as simple string identifiers
6. **Batch Efficiency**: Register, retrieve, tag, and manage files in bulk

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                                  LLM                                    │
│                                                                         │
│   Discovers files using existing tools:                                 │
│   - Shell: find ~/Downloads -name "*.pdf"                               │
│   - Shell: sha256sum /path/to/file                                      │
│   - Shell: file /path/to/file (for mime type)                           │
│   - Shell: stat /path/to/file (for size, dates)                         │
│   - Drive MCP: google_drive_list, google_drive_search                   │
│   - Gmail MCP: gmail_list_attachments                                   │
│                                                                         │
│   Then registers with File Registry:                                    │
│   - file_registry_register(source="local", path="...", ...)             │
│   - file_registry_batch_register(files=[{...}, {...}, ...])             │
│   - file_registry_tag(id="...", tags=["invoice", "2024"])               │
│   - file_registry_batch_tag(ids=["...","..."], tags=["reviewed"])       │
│                                                                         │
└──────────────────────────────────┬──────────────────────────────────────┘
                                   ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                      File Registry MCP Provider                         │
│                                                                         │
│  Passive index — stores metadata provided by LLM:                       │
│  - File references (source + path)                                      │
│  - Metadata (size, type, dates, hashes)                                 │
│  - Tags via Emergent labels                                             │
│  - Duplicate detection by content_hash                                  │
│  - Semantic search via Emergent vector embeddings                       │
│  - Full-text search via Emergent FTS                                    │
│                                                                         │
└──────────────────────────────────┬──────────────────────────────────────┘
                                   ▼
                        [Emergent Knowledge Base]
```

**Characteristics:**
- File Registry has no knowledge of how to access files
- Source is just a string identifier (e.g., "local", "gdrive", "gmail")
- LLM is responsible for gathering metadata before registration
- `content_hash` is required for registration (enables duplicate detection)
- Batch operations process up to 50 items (register) or 100 items (get) with 10 concurrent workers

---

## MCP Tools (18 total)

### Single-Item Tools (13)

| Tool | Description |
|------|-------------|
| `file_registry_register` | Register a file with metadata. `source`, `path`, and `content_hash` are required. |
| `file_registry_get` | Get file details by `id` or `source`+`path`. |
| `file_registry_search` | Full-text search with filters (source, tags, category, etc.). Browse mode when no query provided. |
| `file_registry_semantic_search` | Semantic/vector search using natural language queries. |
| `file_registry_tag` | Add tags to a single file by `id`. |
| `file_registry_untag` | Remove tags from a single file by `id`. |
| `file_registry_tags` | List all tags (labels) with usage counts. |
| `file_registry_duplicates` | Find duplicate files by `content_hash` or list all duplicate groups. |
| `file_registry_remove` | Soft-delete a file from the index (not from actual source). |
| `file_registry_verify` | Mark a file as verified (updates `verified_at` timestamp). |
| `file_registry_stats` | Get aggregate statistics (total files, breakdown by source). |
| `file_registry_recent` | List recently indexed files. |
| `file_registry_similar` | Find semantically similar files using vector embeddings. |

### Batch Tools (5)

All batch tools use 10 concurrent worker goroutines. Each item succeeds or fails independently with per-item results and aggregate succeeded/failed counts.

| Tool | Description | Max Items |
|------|-------------|-----------|
| `file_registry_batch_register` | Register multiple files in one call. Each file needs `source`, `path`, `content_hash`. | 50 |
| `file_registry_batch_get` | Get details for multiple files by `ids` and/or `keys` (source+path pairs). | 100 |
| `file_registry_batch_tag` | Add same tags to multiple files by ID. | unlimited |
| `file_registry_batch_untag` | Remove same tags from multiple files by ID. | unlimited |
| `file_registry_batch_remove` | Soft-delete multiple files by ID. | unlimited |

### Batch Response Format

All batch tools return:
```json
{
  "total": 10,
  "succeeded": 9,
  "failed": 1,
  "results": [
    {"index": 0, "id": "uuid", "status": "registered", "key": "local:/path/to/file"},
    {"index": 1, "id": "", "status": "error", "error": "content_hash is required"}
  ]
}
```

---

## Object Model (Emergent)

Files are stored as Emergent graph objects with:
- **Type**: `"file"`
- **Status**: `"active"` (default)
- **Key**: `"{source}:{path}"` (unique identifier)
- **Labels**: Used for tags (e.g., `["invoice", "2024", "important"]`)
- **Properties**: Metadata fields stored as a flat map

### Properties

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `source` | string | yes | Source identifier: `"local"`, `"gdrive"`, `"gmail"`, etc. |
| `path` | string | yes | Full path within the source |
| `content_hash` | string | yes | SHA-256 of full file content (for duplicate detection) |
| `filename` | string | auto | Base filename (extracted from path if not provided) |
| `extension` | string | auto | Lowercase extension without dot |
| `source_file_id` | string | no | Provider-specific file ID |
| `size` | integer | no | File size in bytes |
| `mime_type` | string | no | MIME type |
| `is_directory` | boolean | no | Whether this is a directory |
| `created_at` | string | no | Creation timestamp (ISO 8601) |
| `modified_at` | string | no | Modification timestamp (ISO 8601) |
| `partial_hash` | string | no | SHA-256 of first 64KB (for quick comparison) |
| `content_text` | string | no | Extracted text content (enables FTS and embeddings) |
| `content_preview` | string | no | First ~500 chars preview |
| `category` | string | no | `document`, `image`, `video`, `audio`, `code`, `archive`, `data`, `other` |
| `subcategory` | string | no | More specific: `invoice`, `receipt`, `photo`, `screenshot`, etc. |

---

## File Structure

```
server/mcp/tools/files/
├── files.go                    # Provider: tool definitions, implementations, batch operations
├── files_integration_test.go   # Integration tests (requires live Emergent instance)
└── FILE_MANAGER_PLAN.md        # This file
```

---

## Example Workflows

### 1. Index Local Directory (Batch)

```
User: "Index my Documents folder"

LLM steps:
1. $ find ~/Documents -type f | head -100
2. For each file, gather metadata:
   $ stat -f "%z %m" /path/to/file
   $ file --mime-type /path/to/file
   $ sha256sum /path/to/file
3. file_registry_batch_register(files=[
     {source: "local", path: "/Users/me/Documents/report.pdf", content_hash: "abc...", size: 45231, ...},
     {source: "local", path: "/Users/me/Documents/notes.md", content_hash: "def...", size: 1234, ...},
     ...
   ])
4. Report summary: "Registered 47 files (3 failures)"
```

### 2. Find Cross-Source Duplicates

```
User: "Do I have any files on Google Drive that are also on my local machine?"

LLM steps:
1. Search Drive with existing Drive MCP:
   google_drive_list(path="/")
2. Batch register Drive files:
   file_registry_batch_register(files=[...])
3. Query for duplicates:
   file_registry_duplicates()
4. "Found 5 duplicate groups across local and Google Drive"
```

### 3. Bulk Tag and Organize

```
User: "Tag all my invoices as reviewed"

LLM steps:
1. file_registry_search(tags=["invoice"])
2. Collect IDs from results
3. file_registry_batch_tag(ids=[...], tags=["reviewed"])
4. "Tagged 23 invoices as reviewed"
```

---

## Future Considerations

### Stage 2: Active Providers

Providers could become standalone MCPs that the File Registry calls via MCP protocol, enabling:
- Automatic file discovery and syncing
- Providers running on remote machines
- Language-agnostic provider implementation
- Dynamic provider registration

---

## Security Considerations

- Never store file contents in the index (only metadata + extracted text)
- The LLM is responsible for access control — File Registry trusts what it's told
- Require confirmation for destructive operations
- Activity logging via Emergent's built-in audit trail
