# Google Drive Provider

> **Note**: This is a **Stage 2** specification. In Stage 1, the Document Manager is a passive index - it doesn't have active providers. The LLM uses the existing Google Drive MCP tools (google_drive_list, google_drive_search, etc.) to discover files and then registers them with the Document Manager. This provider spec is for the future Stage 2 implementation where the Document Manager can actively sync with Google Drive.

## Overview

The Google Drive Provider is a Source Provider implementation for the Document Manager that handles indexing and accessing files on Google Drive. It implements the same `SourceProvider` interface as the Local FS provider, ensuring consistent behavior across sources.

This provider is designed to work in **both stages** of the Document Manager architecture:
- **Stage 1**: Internal Go provider called directly by Document Manager
- **Stage 2**: Standalone MCP with standardized tools that can be called via MCP protocol

---

## Provider Identity

| Property | Value |
|----------|-------|
| Name | `gdrive` or `gdrive:{account}` |
| Display Name | `Google Drive` or `Google Drive ({account})` |
| Type | `gdrive` |
| Is Remote | Yes |
| Requires Auth | Yes (OAuth 2.0) |

## Capabilities

```go
ProviderCapabilities{
    CanList:          true,
    CanRead:          true,
    CanWrite:         true,   // Create, update files
    CanDelete:        true,
    CanTrash:         true,   // Drive has trash
    CanWatch:         true,   // Changes API
    CanExtractText:   true,   // Via Google's text extraction
    CanComputeHash:   true,   // Drive provides md5Checksum
    SupportsVersions: true,   // Drive keeps revisions
    SupportsSharing:  true,   // Drive sharing
    IsRemote:         true,
    RequiresAuth:     true,
}
```

---

## Architecture

### Stage 1: Internal Provider

```
Document Manager
       │
       ▼ (direct Go call)
┌──────────────────┐
│  GDrive Provider │
│  (Go interface)  │
└──────────────────┘
       │
       ▼ (HTTP/REST)
┌──────────────────┐
│  Google Drive    │
│  API v3          │
└──────────────────┘
```

### Stage 2: MCP Provider

```
Document Manager
       │
       ▼ (MCP protocol)
┌──────────────────┐
│  GDrive MCP      │
│  (MCP tools)     │
└──────────────────┘
       │
       ▼ (HTTP/REST)
┌──────────────────┐
│  Google Drive    │
│  API v3          │
└──────────────────┘
```

---

## Authentication

### OAuth 2.0 Flow

The provider uses Google OAuth 2.0 with the following scopes:
- `https://www.googleapis.com/auth/drive.readonly` (read-only operations)
- `https://www.googleapis.com/auth/drive` (full access, if mutations enabled)

### Token Storage

Tokens are stored in Diane's secrets directory:
```
~/.diane/secrets/google/
├── credentials.json          # OAuth client credentials
├── token_default.json        # Default account token
└── token_{account}.json      # Per-account tokens
```

### Integration with Existing Gmail Auth

The provider can reuse existing Google OAuth tokens from the Gmail integration:
```go
func (p *Provider) getToken(account string) (*oauth2.Token, error) {
    // Try Diane secrets first
    tokenPath := filepath.Join(secretsDir, "google", fmt.Sprintf("token_%s.json", account))
    if token, err := loadToken(tokenPath); err == nil {
        return token, nil
    }
    
    // Fall back to gog tokens (backward compatibility)
    gogPath := filepath.Join(os.Getenv("HOME"), ".config", "gog", "tokens", account+".json")
    return loadToken(gogPath)
}
```

---

## Stage 2: MCP Tool Definitions

When running as a standalone MCP, the Google Drive provider exposes these tools:

### gdrive_list
List files in a folder.

```yaml
name: gdrive_list
description: List files in a Google Drive folder

input:
  path: string          # Folder path or ID (default: root)
  recursive: boolean    # Include subfolders (default: false)
  include_trashed: bool # Include trashed files (default: false)
  limit: integer        # Max results (default: 100)
  page_token: string    # For pagination
  query: string         # Drive search query (optional)
  mime_types:           # Filter by MIME types
    type: array
    items: string

output:
  files:
    type: array
    items:
      id: string          # Drive file ID
      path: string        # Full path (constructed from parents)
      name: string        # File name
      is_directory: boolean
      size: integer       # Size in bytes (0 for folders)
      mime_type: string   # Drive MIME type
      modified_at: string # ISO 8601 datetime
      created_at: string  # ISO 8601 datetime
      md5_checksum: string # Drive's MD5 (for files)
      web_view_link: string
      shared: boolean
  next_page_token: string
```

### gdrive_get
Get detailed metadata for a file.

```yaml
name: gdrive_get
description: Get detailed metadata for a Drive file

input:
  id: string            # File ID (required)

output:
  id: string
  path: string          # Full path
  name: string
  is_directory: boolean
  size: integer
  mime_type: string
  modified_at: string
  created_at: string
  md5_checksum: string
  version: string       # Drive revision number
  web_view_link: string
  web_content_link: string  # Download link (if available)
  owners:
    type: array
    items:
      email: string
      name: string
  shared: boolean
  sharing_user: object  # Who shared with you
  permissions:
    type: array
    items:
      type: string      # user, group, domain, anyone
      role: string      # owner, writer, reader
      email: string
  parents:
    type: array
    items: string       # Parent folder IDs
  starred: boolean
  trashed: boolean
  capabilities:         # What you can do with this file
    can_edit: boolean
    can_share: boolean
    can_delete: boolean
```

### gdrive_read
Download file content.

```yaml
name: gdrive_read
description: Download file content from Google Drive

input:
  id: string            # File ID (required)
  encoding: string      # 'base64' (default), 'utf8'
  max_size: integer     # Max bytes (default: 10MB)
  export_format: string # For Google Docs: 'pdf', 'docx', 'txt', etc.

output:
  content: string       # Base64 or UTF-8 encoded
  size: integer
  mime_type: string     # Actual content type
  truncated: boolean
  exported: boolean     # True if Google Doc was exported
```

### gdrive_hash
Get content hash (Drive provides md5Checksum).

```yaml
name: gdrive_hash
description: Get file hash from Google Drive

input:
  id: string            # File ID (required)

output:
  hash: string          # SHA-256 (computed if needed)
  md5: string           # Drive's native MD5 checksum
  partial_hash: string  # First 64KB SHA-256 (computed)
  size: integer

# Note: For exact duplicate detection with local files,
# we compute SHA-256 by downloading. For quick checks,
# we can use Drive's md5Checksum.
```

### gdrive_roots
List available drives and shared locations.

```yaml
name: gdrive_roots
description: List available Drive roots

input: {}

output:
  roots:
    type: array
    items:
      id: string        # 'root', 'shared', or shared drive ID
      path: string      # Display path
      name: string      # "My Drive", "Shared with me", etc.
      type: string      # 'drive', 'shared_drive', 'shared_folder'
      is_default: boolean
```

### gdrive_capabilities
Get provider capabilities.

```yaml
name: gdrive_capabilities
description: Get Google Drive provider capabilities

input: {}

output:
  can_list: boolean          # true
  can_read: boolean          # true
  can_write: boolean         # true (if scope allows)
  can_delete: boolean        # true
  can_trash: boolean         # true
  can_watch: boolean         # true (Changes API)
  can_extract_text: boolean  # true (export as text)
  can_compute_hash: boolean  # true (md5Checksum)
  supports_versions: boolean # true
  supports_sharing: boolean  # true
  is_remote: boolean         # true
  requires_auth: boolean     # true
```

### gdrive_search
Native Drive search.

```yaml
name: gdrive_search
description: Search files using Google Drive query syntax

input:
  query: string         # Drive search query (required)
  # Supports: name contains 'x', mimeType='y', modifiedTime > 'z'
  limit: integer
  page_token: string

output:
  files: array          # Same as gdrive_list
  next_page_token: string
```

### gdrive_delete (Optional)
Trash or permanently delete a file.

```yaml
name: gdrive_delete
description: Delete a file from Google Drive

input:
  id: string            # File ID (required)
  trash: boolean        # Move to trash (default: true)
  permanent: boolean    # Permanently delete (requires trash=false)

output:
  deleted: boolean
  trashed: boolean
```

### gdrive_changes
Get changes since last sync.

```yaml
name: gdrive_changes
description: Get file changes for incremental sync

input:
  page_token: string    # Start token (required, get from gdrive_changes_start)
  limit: integer

output:
  changes:
    type: array
    items:
      type: string      # 'file', 'drive'
      file_id: string
      removed: boolean
      file: object      # File metadata if not removed
  new_start_page_token: string  # For next sync
  next_page_token: string       # For pagination within this call
```

---

## Implementation

### Provider Structure

```go
package gdrive

import (
    "context"
    "google.golang.org/api/drive/v3"
    "golang.org/x/oauth2"
)

type Provider struct {
    account     string
    config      Config
    client      *drive.Service
    pathCache   *PathCache  // Cache file ID -> path mappings
}

type Config struct {
    Account         string
    CredentialsPath string
    TokenPath       string
    ReadOnly        bool
}

func New(cfg Config) (*Provider, error) {
    token, err := loadToken(cfg.TokenPath)
    if err != nil {
        return nil, fmt.Errorf("no auth token: %w", err)
    }
    
    client := oauth2.NewClient(context.Background(), 
        oauth2.StaticTokenSource(token))
    
    srv, err := drive.NewService(context.Background(), 
        option.WithHTTPClient(client))
    if err != nil {
        return nil, err
    }
    
    return &Provider{
        account:   cfg.Account,
        config:    cfg,
        client:    srv,
        pathCache: NewPathCache(),
    }, nil
}
```

### Interface Implementation

```go
func (p *Provider) Name() string { 
    if p.account == "" || p.account == "default" {
        return "gdrive"
    }
    return fmt.Sprintf("gdrive:%s", p.account)
}

func (p *Provider) DisplayName() string {
    if p.account == "" || p.account == "default" {
        return "Google Drive"
    }
    return fmt.Sprintf("Google Drive (%s)", p.account)
}

func (p *Provider) Capabilities() ProviderCapabilities {
    return ProviderCapabilities{
        CanList:          true,
        CanRead:          true,
        CanWrite:         !p.config.ReadOnly,
        CanDelete:        !p.config.ReadOnly,
        CanTrash:         true,
        CanWatch:         true,
        CanExtractText:   true,
        CanComputeHash:   true,
        SupportsVersions: true,
        SupportsSharing:  true,
        IsRemote:         true,
        RequiresAuth:     true,
    }
}

func (p *Provider) IsConnected() bool {
    // Quick API call to verify connection
    _, err := p.client.About.Get().Fields("user").Do()
    return err == nil
}

func (p *Provider) Connect() error {
    return nil // Already connected in New()
}
```

### Listing Files

```go
func (p *Provider) ListRoots() ([]SourceRoot, error) {
    roots := []SourceRoot{
        {
            ID:        "root",
            Path:      "/My Drive",
            Name:      "My Drive",
            Type:      "drive",
            IsDefault: true,
        },
        {
            ID:   "shared",
            Path: "/Shared with me",
            Name: "Shared with me",
            Type: "shared_folder",
        },
    }
    
    // Add shared drives if user has access
    drives, err := p.client.Drives.List().Do()
    if err == nil {
        for _, d := range drives.Drives {
            roots = append(roots, SourceRoot{
                ID:   d.Id,
                Path: "/" + d.Name,
                Name: d.Name,
                Type: "shared_drive",
            })
        }
    }
    
    return roots, nil
}

func (p *Provider) List(path string, opts ListOptions) ([]SourceFile, error) {
    // Resolve path to folder ID
    folderID, err := p.resolvePath(path)
    if err != nil {
        return nil, err
    }
    
    query := fmt.Sprintf("'%s' in parents and trashed = false", folderID)
    
    var files []SourceFile
    pageToken := ""
    
    for {
        call := p.client.Files.List().
            Q(query).
            Fields("nextPageToken, files(id, name, mimeType, size, modifiedTime, createdTime, md5Checksum, parents, webViewLink)").
            PageSize(100)
        
        if pageToken != "" {
            call = call.PageToken(pageToken)
        }
        
        result, err := call.Do()
        if err != nil {
            return nil, err
        }
        
        for _, f := range result.Files {
            sf := p.driveFileToSourceFile(f, path)
            files = append(files, sf)
            
            if opts.Limit > 0 && len(files) >= opts.Limit {
                return files, nil
            }
        }
        
        if result.NextPageToken == "" {
            break
        }
        pageToken = result.NextPageToken
    }
    
    // Recursive listing
    if opts.Recursive {
        for _, f := range files {
            if f.IsDirectory {
                subFiles, err := p.List(f.Path, opts)
                if err != nil {
                    continue // Skip inaccessible folders
                }
                files = append(files, subFiles...)
            }
        }
    }
    
    return files, nil
}

func (p *Provider) driveFileToSourceFile(f *drive.File, parentPath string) SourceFile {
    isDir := f.MimeType == "application/vnd.google-apps.folder"
    
    path := parentPath
    if !strings.HasSuffix(path, "/") {
        path += "/"
    }
    path += f.Name
    
    // Cache path for later lookups
    p.pathCache.Set(f.Id, path)
    
    modified, _ := time.Parse(time.RFC3339, f.ModifiedTime)
    created, _ := time.Parse(time.RFC3339, f.CreatedTime)
    
    return SourceFile{
        ID:          f.Id,
        Path:        path,
        Name:        f.Name,
        IsDirectory: isDir,
        Size:        f.Size,
        MimeType:    f.MimeType,
        ModifiedAt:  modified,
        CreatedAt:   &created,
        Metadata: map[string]any{
            "md5Checksum": f.Md5Checksum,
            "webViewLink": f.WebViewLink,
        },
    }
}
```

### Reading Content

```go
func (p *Provider) GetContent(id string) (io.ReadCloser, error) {
    file, err := p.client.Files.Get(id).Fields("mimeType").Do()
    if err != nil {
        return nil, err
    }
    
    // Google Docs need to be exported
    if isGoogleDoc(file.MimeType) {
        exportMime := getExportMimeType(file.MimeType)
        resp, err := p.client.Files.Export(id, exportMime).Download()
        if err != nil {
            return nil, err
        }
        return resp.Body, nil
    }
    
    // Regular files can be downloaded directly
    resp, err := p.client.Files.Get(id).Download()
    if err != nil {
        return nil, err
    }
    return resp.Body, nil
}

func isGoogleDoc(mimeType string) bool {
    return strings.HasPrefix(mimeType, "application/vnd.google-apps.")
}

func getExportMimeType(googleMime string) string {
    exports := map[string]string{
        "application/vnd.google-apps.document":     "application/pdf",
        "application/vnd.google-apps.spreadsheet":  "text/csv",
        "application/vnd.google-apps.presentation": "application/pdf",
        "application/vnd.google-apps.drawing":      "image/png",
    }
    if export, ok := exports[googleMime]; ok {
        return export
    }
    return "application/pdf"
}
```

### Computing Hash

```go
func (p *Provider) ComputeHash(id string) (string, error) {
    // For non-Google-Doc files, Drive provides MD5
    // But for cross-source duplicate detection, we need SHA-256
    
    file, err := p.client.Files.Get(id).Fields("md5Checksum, size").Do()
    if err != nil {
        return "", err
    }
    
    // For small files or when we need exact match, download and hash
    if file.Size < 50*1024*1024 { // < 50MB
        content, err := p.GetContent(id)
        if err != nil {
            return "", err
        }
        defer content.Close()
        
        h := sha256.New()
        if _, err := io.Copy(h, content); err != nil {
            return "", err
        }
        return hex.EncodeToString(h.Sum(nil)), nil
    }
    
    // For large files, use MD5 (not ideal for cross-source matching)
    // Consider: download in chunks, or use partial hash
    return file.Md5Checksum, nil
}
```

### Incremental Sync

```go
func (p *Provider) GetChangeToken() (string, error) {
    token, err := p.client.Changes.GetStartPageToken().Do()
    if err != nil {
        return "", err
    }
    return token.StartPageToken, nil
}

func (p *Provider) GetChanges(pageToken string) ([]Change, string, error) {
    result, err := p.client.Changes.List(pageToken).
        Fields("nextPageToken, newStartPageToken, changes(fileId, removed, file)").
        Do()
    if err != nil {
        return nil, "", err
    }
    
    var changes []Change
    for _, c := range result.Changes {
        change := Change{
            FileID:  c.FileId,
            Removed: c.Removed,
        }
        if c.File != nil {
            sf := p.driveFileToSourceFile(c.File, "") // Path needs resolution
            change.File = &sf
        }
        changes = append(changes, change)
    }
    
    nextToken := result.NextPageToken
    if nextToken == "" {
        nextToken = result.NewStartPageToken
    }
    
    return changes, nextToken, nil
}
```

---

## Path Resolution

Google Drive uses file IDs, not paths. We need to resolve paths to IDs and vice versa.

```go
type PathCache struct {
    idToPath map[string]string
    pathToID map[string]string
    mu       sync.RWMutex
}

func (p *Provider) resolvePath(path string) (string, error) {
    // Check cache first
    if id, ok := p.pathCache.GetID(path); ok {
        return id, nil
    }
    
    // Handle special paths
    switch path {
    case "", "/", "/My Drive":
        return "root", nil
    case "/Shared with me":
        return "shared", nil
    }
    
    // Walk the path from root
    parts := strings.Split(strings.Trim(path, "/"), "/")
    if parts[0] == "My Drive" {
        parts = parts[1:]
    }
    
    currentID := "root"
    for _, part := range parts {
        query := fmt.Sprintf("name = '%s' and '%s' in parents and trashed = false", 
            escapeQuery(part), currentID)
        
        result, err := p.client.Files.List().Q(query).Fields("files(id)").Do()
        if err != nil {
            return "", err
        }
        if len(result.Files) == 0 {
            return "", ErrNotFound
        }
        currentID = result.Files[0].Id
    }
    
    // Cache the result
    p.pathCache.Set(currentID, path)
    return currentID, nil
}

func (p *Provider) resolveID(id string) (string, error) {
    // Check cache first
    if path, ok := p.pathCache.GetPath(id); ok {
        return path, nil
    }
    
    // Build path by walking up parents
    var parts []string
    currentID := id
    
    for currentID != "" && currentID != "root" {
        file, err := p.client.Files.Get(currentID).Fields("name, parents").Do()
        if err != nil {
            return "", err
        }
        parts = append([]string{file.Name}, parts...)
        
        if len(file.Parents) > 0 {
            currentID = file.Parents[0]
        } else {
            break
        }
    }
    
    path := "/My Drive/" + strings.Join(parts, "/")
    p.pathCache.Set(id, path)
    return path, nil
}
```

---

## Configuration

```go
var DefaultConfig = Config{
    Account:         "default",
    CredentialsPath: "~/.diane/secrets/google/credentials.json",
    TokenPath:       "~/.diane/secrets/google/token_default.json",
    ReadOnly:        false,
}
```

---

## File Structure

### Stage 1: Internal Provider

```
server/mcp/tools/documents/providers/gdrive/
├── provider.go           # Main provider implementation
├── client.go             # Drive API client wrapper
├── auth.go               # OAuth token management
├── path.go               # Path resolution and caching
├── sync.go               # Changes API / incremental sync
├── export.go             # Google Docs export handling
└── GDRIVE_PROVIDER_PLAN.md
```

### Stage 2: Standalone MCP

```
server/mcp/tools/gdrive/              # NEW: Standalone MCP
├── gdrive.go             # MCP tool definitions
├── handlers.go           # Tool handlers
└── README.md
```

---

## Implementation Phases

### Stage 1 Phases (Internal Provider)

#### Phase 1.1: Core Provider
- [ ] Provider structure
- [ ] OAuth token loading
- [ ] Drive API client setup
- [ ] ListRoots implementation

#### Phase 1.2: File Listing
- [ ] List files in folder
- [ ] Recursive listing
- [ ] Path resolution (path → ID)
- [ ] Path cache

#### Phase 1.3: File Access
- [ ] GetMetadata
- [ ] GetContent (regular files)
- [ ] GetContent (Google Docs export)
- [ ] ID to path resolution

#### Phase 1.4: Hashing
- [ ] Use Drive's md5Checksum
- [ ] Compute SHA-256 for small files
- [ ] Partial hash for large files

#### Phase 1.5: Sync
- [ ] Get change token
- [ ] Get changes since token
- [ ] Integration with Document Manager sync

#### Phase 1.6: Mutations
- [ ] Trash file
- [ ] Permanent delete
- [ ] Move file
- [ ] Copy file

### Stage 2 Phases (MCP Provider)

#### Phase 2.1: MCP Wrapper
- [ ] Create gdrive MCP package
- [ ] Register tools
- [ ] Tool handlers

#### Phase 2.2: All Tools
- [ ] gdrive_list, gdrive_get, gdrive_read
- [ ] gdrive_hash, gdrive_roots, gdrive_capabilities
- [ ] gdrive_search, gdrive_changes
- [ ] gdrive_delete (optional)

---

## Challenges and Solutions

### Challenge: Path vs ID

**Problem**: Drive uses IDs, not paths. Users think in paths.

**Solution**: 
- Maintain a path cache
- Build paths by walking parent chain
- Accept both paths and IDs in tools

### Challenge: Google Docs

**Problem**: Google Docs (Docs, Sheets, Slides) have no binary content.

**Solution**:
- Export to standard formats (PDF, DOCX, CSV)
- Default export format configurable
- Return exported MIME type in output

### Challenge: Large Files

**Problem**: Computing SHA-256 requires downloading entire file.

**Solution**:
- Use Drive's MD5 for quick checks
- Only compute SHA-256 when needed for exact matching
- Use partial hash (first 64KB) for quick elimination

### Challenge: Shared Files

**Problem**: Shared files don't have a path in "My Drive".

**Solution**:
- Create virtual root "/Shared with me"
- Use "sharedWithMe = true" query
- Track sharing metadata

---

## Error Handling

| Error | Handling |
|-------|----------|
| 401 Unauthorized | Token expired, trigger re-auth |
| 403 Forbidden | No access to file, skip and log |
| 404 Not Found | File deleted or moved |
| 429 Rate Limited | Exponential backoff |
| Network Error | Retry with backoff |
| Google Doc no export | Use best available format |

---

## Rate Limits

Google Drive API has rate limits:
- 1,000 queries per 100 seconds per user
- 10 queries per second per user

**Mitigation**:
- Batch requests where possible
- Cache aggressively
- Use fields parameter to limit response size
- Implement exponential backoff

---

## Dependencies

| Dependency | Purpose | Required |
|------------|---------|----------|
| `google.golang.org/api/drive/v3` | Drive API client | Yes |
| `golang.org/x/oauth2` | OAuth2 handling | Yes |
| `golang.org/x/oauth2/google` | Google OAuth config | Yes |
