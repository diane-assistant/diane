# Gmail Cache & Processing Architecture

## Overview

A local caching layer for Gmail that provides fast access to email metadata, extracted content, and attachments. The cache lives in a separate SQLite database (`~/.diane/gmail.db`) and stores processed/extracted data rather than raw email content.

## Goals

1. **Speed**: Replace slow `gog` CLI calls with direct Google API + local cache
2. **Token efficiency**: Return plain text instead of 300KB HTML to AI
3. **Structured data**: Extract and cache JSON-LD for classification
4. **Attachments**: Download on-demand, store locally, make accessible to other tools
5. **Sender analytics**: Pre-computed stats for pattern analysis

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         MCP Tools                                │
│  gmail_search, gmail_read, gmail_download_attachment, etc.      │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Gmail Service Layer                         │
│               (server/mcp/tools/google/gmail/)                   │
│  - Manages cache reads/writes                                    │
│  - Handles Google API calls                                      │
│  - Extracts plain text and JSON-LD                              │
└─────────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┴───────────────┐
              ▼                               ▼
┌─────────────────────────┐     ┌─────────────────────────────────┐
│   Gmail Cache (SQLite)  │     │        Google Gmail API          │
│   ~/.diane/gmail.db     │     │   (using gog credentials)        │
└─────────────────────────┘     └─────────────────────────────────┘
              │
              ▼
┌─────────────────────────┐
│   Attachments (Files)   │
│ ~/.diane/attachments/   │
└─────────────────────────┘
```

## Database Schema

```sql
-- ~/.diane/gmail.db

-- Core email metadata (always cached on search/access)
CREATE TABLE emails (
    gmail_id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL,
    subject TEXT,
    from_email TEXT,
    from_name TEXT,
    to_emails TEXT,           -- JSON array
    cc_emails TEXT,           -- JSON array
    date DATETIME,
    snippet TEXT,             -- Gmail's ~100 char preview
    labels TEXT,              -- JSON array of label IDs
    has_attachments INTEGER DEFAULT 0,
    
    -- Extracted content (populated on-demand)
    plain_text TEXT,          -- Extracted plain text (nullable)
    json_ld TEXT,             -- Extracted JSON-LD array (nullable)
    
    -- Cache metadata
    metadata_cached_at DATETIME NOT NULL,
    content_cached_at DATETIME,  -- When plain_text/json_ld were extracted
    accessed_at DATETIME NOT NULL
);

CREATE INDEX idx_emails_thread ON emails(thread_id);
CREATE INDEX idx_emails_from ON emails(from_email);
CREATE INDEX idx_emails_date ON emails(date);

-- Attachment references (populated when email is accessed)
CREATE TABLE attachments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    gmail_id TEXT NOT NULL,
    attachment_id TEXT NOT NULL,  -- Gmail's attachment ID for fetching
    filename TEXT NOT NULL,
    mime_type TEXT,
    size INTEGER,
    
    -- Local storage (set when downloaded)
    local_path TEXT,              -- Full path to downloaded file
    downloaded_at DATETIME,
    
    FOREIGN KEY (gmail_id) REFERENCES emails(gmail_id) ON DELETE CASCADE,
    UNIQUE(gmail_id, attachment_id)
);

CREATE INDEX idx_attachments_gmail ON attachments(gmail_id);

-- Pre-computed sender statistics
CREATE TABLE sender_stats (
    email_pattern TEXT PRIMARY KEY,  -- Email or pattern like "%@allegro.pl"
    display_name TEXT,               -- Most common display name
    message_count INTEGER DEFAULT 0,
    first_seen DATETIME,
    last_seen DATETIME,
    common_subjects TEXT,            -- JSON array of common subject patterns
    json_ld_types TEXT,              -- JSON array of JSON-LD @type values seen
    updated_at DATETIME NOT NULL
);

-- Sync state for incremental updates
CREATE TABLE sync_state (
    account TEXT PRIMARY KEY,
    history_id TEXT,                 -- Gmail history ID for incremental sync
    last_full_sync DATETIME,
    last_incremental_sync DATETIME
);
```

## File Storage

```
~/.diane/
├── gmail.db                          # SQLite cache database
└── attachments/
    └── {gmail_id}/
        ├── invoice.pdf
        ├── image.jpg
        └── ...
```

## MCP Tools

### gmail_search
Search emails with caching of metadata.

```
Input:
  query: string       # Gmail search query (from:, subject:, label:, etc.)
  max: number         # Max results (default: 20)
  account: string     # Optional account

Output:
  Array of:
    - gmail_id, thread_id
    - subject, from_email, from_name, date
    - snippet
    - labels
    - has_attachments
    - has_json_ld (boolean - indicates structured data available)

Behavior:
  1. Execute Gmail API search
  2. Cache metadata for all results
  3. Return cached data
```

### gmail_read
Read email content with format options.

```
Input:
  id: string          # Gmail message ID
  format: string      # "plain" (default), "jsonld", "headers", "raw"
  account: string     # Optional

Output (varies by format):
  "plain":   { headers, plain_text, json_ld_summary }
  "jsonld":  { json_ld array or null }
  "headers": { from, to, subject, date, labels, attachments }
  "raw":     { full HTML/MIME content - not cached }

Behavior:
  1. Check cache for requested format
  2. If not cached, fetch from API
  3. Extract plain text (strip HTML)
  4. Extract JSON-LD from <script type="application/ld+json">
  5. Cache extracted content
  6. Return requested format
```

### gmail_list_attachments
List attachments for an email.

```
Input:
  id: string          # Gmail message ID
  account: string     # Optional

Output:
  Array of:
    - attachment_id
    - filename
    - mime_type
    - size
    - local_path (if downloaded)
    - downloaded_at (if downloaded)
```

### gmail_download_attachment
Download attachment to local filesystem.

```
Input:
  email_id: string        # Gmail message ID
  attachment_id: string   # Attachment ID from list
  account: string         # Optional

Output:
  - local_path: string    # Full path to downloaded file
  - filename: string
  - mime_type: string
  - size: number

Behavior:
  1. Create directory ~/.diane/attachments/{email_id}/
  2. Download attachment from Gmail API
  3. Save to {directory}/{filename}
  4. Update attachments table with local_path
  5. Return file info
```

### gmail_sender_stats
Get statistics about a sender or pattern.

```
Input:
  pattern: string     # Email address or pattern (e.g., "allegro", "%@newsletter.%")
  account: string     # Optional

Output:
  - email_pattern
  - display_name
  - message_count
  - first_seen, last_seen
  - common_subjects (array)
  - json_ld_types (array) - e.g., ["Order", "ParcelDelivery"]
```

### gmail_sync
Trigger cache sync (manual or can be scheduled).

```
Input:
  mode: string        # "incremental" (default) or "full"
  days: number        # For full sync, how many days back (default: 30)
  account: string     # Optional

Output:
  - messages_synced: number
  - new_messages: number
  - updated_messages: number

Behavior:
  - incremental: Use Gmail History API from last historyId
  - full: Fetch all messages from last N days, update cache
```

### gmail_batch_modify (existing, enhanced)
Modify labels on multiple messages.

```
Input:
  ids: string         # Comma-separated message IDs
  add: string         # Labels to add
  remove: string      # Labels to remove

Behavior:
  - Call Gmail API
  - Update cached labels in gmail.db
```

## JSON-LD Extraction

### What We Extract

From `<script type="application/ld+json">` blocks in email HTML:

```json
{
  "@type": "Order",
  "orderNumber": "ABC123",
  "merchant": {"name": "Allegro"},
  "orderedItem": [...],
  "orderStatus": "OrderProcessing"
}
```

Common types in emails:
- `Order` - Purchase confirmations
- `ParcelDelivery` - Shipping notifications
- `FlightReservation` - Flight bookings
- `EventReservation` - Event tickets
- `FoodEstablishmentReservation` - Restaurant bookings
- `LodgingReservation` - Hotel bookings

### How We Use It

1. **Classification**: `json_ld_types` in sender_stats tells us "emails from X are usually Orders"
2. **Quick lookup**: AI asks "what orders do I have?" → query emails WHERE json_ld LIKE '%"@type":"Order"%'
3. **Structured access**: Return JSON-LD directly instead of AI parsing HTML

## Credential Sharing with gog

The `gog` CLI stores OAuth tokens in `~/.config/gog/`:
```
~/.config/gog/
├── config.yaml
└── tokens/
    └── {account}.json   # OAuth token
```

Our Go Gmail client reads the same token files - no duplicate authentication.

## Implementation Phases

### Phase 1: Foundation (Core Infrastructure) ✅ COMPLETE
- [x] Create `gmail.db` schema and migrations
- [x] Build Go Gmail API client (reusing gog tokens)
- [x] Implement basic cache read/write operations
- [x] Create `gmail_search` with metadata caching

### Phase 2: Content Extraction ✅ COMPLETE
- [x] Implement HTML → plain text extraction
- [x] Implement JSON-LD extraction from HTML
- [x] Create `gmail_read` with format options
- [x] Cache extracted content

### Phase 3: Attachments ✅ COMPLETE
- [x] Create attachment directory structure
- [x] Implement `gmail_list_attachments`
- [x] Implement `gmail_download_attachment`
- [x] Update attachment references in cache

### Phase 4: Analytics & Sync ✅ COMPLETE
- [x] Implement sender_stats aggregation
- [x] Create `gmail_sender_stats` tool
- [x] Implement incremental sync with History API
- [x] Create `gmail_sync` tool

### Phase 5: Integration & Migration ✅ COMPLETE
- [x] Update existing google.go to use new Gmail service
- [ ] Deprecate old gog-based implementations (kept for backward compat)
- [x] Add cache management tools (stats)
- [x] Documentation

## File Structure

```
server/mcp/tools/google/
├── google.go                 # Main provider (updated with new tools)
├── gmail/
│   ├── cache.go             # SQLite cache operations ✅
│   ├── client.go            # Google API client ✅
│   ├── extract.go           # Plain text & JSON-LD extraction ✅
│   ├── attachments.go       # Attachment handling ✅
│   ├── sync.go              # Sync operations ✅
│   └── service.go           # High-level service layer ✅
└── GMAIL_CACHE_PLAN.md      # This file
```

## Configuration

No new configuration required. Uses:
- `~/.diane/gmail.db` for cache (auto-created)
- `~/.diane/attachments/` for files (auto-created)
- `~/.diane/secrets/google/` for OAuth credentials

### Secrets Location

```
~/.diane/
├── secrets/
│   ├── google/
│   │   ├── credentials.json     # OAuth client credentials
│   │   └── token_{account}.json # Per-account OAuth tokens
│   ├── github/
│   │   └── ...
│   └── {other-mcp}/
│       └── ...
├── gmail.db
├── attachments/
└── cron.db
```

**Migration**: On first run, we check for existing gog tokens in `~/.config/gog/tokens/` 
and copy them to the new location for backward compatibility.

## Cache Invalidation

- **Metadata**: Considered stale after 1 hour (configurable)
- **Content**: Never expires (email content doesn't change)
- **Attachments**: Never deleted automatically
- **Manual clear**: Tool to clear cache by age or pattern

## Error Handling

- API failures: Return cached data if available, with warning
- Missing credentials: Clear error message pointing to `gog auth`
- Cache corruption: Auto-rebuild on schema mismatch
