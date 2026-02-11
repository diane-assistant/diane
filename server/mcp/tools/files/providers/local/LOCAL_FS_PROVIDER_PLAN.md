# Local Filesystem Provider

> **Note**: This is a **Stage 2** specification. In Stage 1, the Document Manager is a passive index - it doesn't have active providers. The LLM uses existing shell commands (find, stat, file, sha256sum) to discover files and then registers them with the Document Manager. This provider spec is for the future Stage 2 implementation where the Document Manager can actively scan the filesystem.

## Overview

The Local Filesystem Provider is a Source Provider implementation for the Document Manager that handles indexing and accessing files on the local filesystem. It implements the `SourceProvider` interface defined in the Document Manager architecture.

This provider is designed to work in **both stages** of the Document Manager architecture:
- **Stage 1**: Internal Go provider called directly by Document Manager
- **Stage 2**: Standalone MCP with standardized tools that can be called via MCP protocol

The implementation uses the same core code for both stages - only the interface layer differs.

---

## Stage 1 vs Stage 2

### Stage 1: Internal Provider

```
Document Manager
       │
       ▼ (direct Go call)
┌──────────────────┐
│  Local Provider  │
│  (Go interface)  │
└──────────────────┘
       │
       ▼
  [Filesystem]
```

The provider implements the `SourceProvider` Go interface:
```go
provider.List(path, opts)
provider.GetMetadata(id)
provider.GetContent(id)
provider.ComputeHash(id)
```

### Stage 2: MCP Provider

```
Document Manager
       │
       ▼ (MCP protocol call)
┌──────────────────┐
│  Local FS MCP    │
│  (MCP tools)     │
└──────────────────┘
       │
       ▼
  [Filesystem]
```

The provider exposes MCP tools:
```
fs_list, fs_get, fs_read, fs_hash, fs_roots, fs_capabilities
fs_delete, fs_move, fs_copy, fs_extract_text (optional)
```

### Code Reuse Strategy

```go
// Core implementation (shared)
package local

type Provider struct { ... }

func (p *Provider) List(path string, opts ListOptions) ([]SourceFile, error) { ... }
func (p *Provider) GetMetadata(id string) (*FileMetadata, error) { ... }
// ... other methods

// Stage 1: Used directly by Document Manager
// (no additional wrapper needed)

// Stage 2: MCP tool wrapper
package localfs

func init() {
    RegisterTool("fs_list", handleList)
    RegisterTool("fs_get", handleGet)
    // ...
}

func handleList(args map[string]any) (any, error) {
    provider := GetProvider()
    opts := parseListOptions(args)
    return provider.List(args["path"].(string), opts)
}
```

---

## Stage 2: MCP Tool Definitions

When running as a standalone MCP, the Local FS provider exposes these tools:

### fs_list
List files in a directory.

```yaml
name: fs_list
description: List files in a directory on the local filesystem

input:
  path: string          # Directory path to list (required)
  recursive: boolean    # Include subdirectories (default: false)
  include_hidden: bool  # Include hidden files (default: false)
  limit: integer        # Max results (default: 1000)
  offset: integer       # Pagination offset
  include_patterns:     # Glob patterns to include
    type: array
    items: string
  exclude_patterns:     # Glob patterns to exclude
    type: array
    items: string

output:
  files:
    type: array
    items:
      id: string          # File path (for local, path = id)
      path: string        # Full absolute path
      name: string        # Filename
      is_directory: boolean
      size: integer       # Size in bytes
      mime_type: string
      modified_at: string # ISO 8601 datetime
      created_at: string  # ISO 8601 datetime (if available)
```

### fs_get
Get detailed metadata for a file.

```yaml
name: fs_get
description: Get detailed metadata for a file

input:
  id: string            # File path (required)

output:
  id: string
  path: string
  name: string
  is_directory: boolean
  size: integer
  mime_type: string
  modified_at: string
  created_at: string
  permissions: string   # e.g., "-rw-r--r--"
  owner: string         # Username
  is_symlink: boolean
  symlink_target: string  # If symlink
  category: string      # document, image, video, etc.
  subcategory: string   # invoice, screenshot, etc.
```

### fs_read
Read file content.

```yaml
name: fs_read
description: Read file content

input:
  id: string            # File path (required)
  encoding: string      # 'base64' (default), 'utf8'
  max_size: integer     # Max bytes to read (default: 10MB)
  offset: integer       # Byte offset to start reading
  length: integer       # Number of bytes to read

output:
  content: string       # Base64 or UTF-8 encoded content
  size: integer         # Actual bytes returned
  total_size: integer   # Total file size
  truncated: boolean    # True if content was truncated
  encoding: string      # Encoding used
```

### fs_hash
Compute content hash.

```yaml
name: fs_hash
description: Compute SHA-256 hash of file content

input:
  id: string            # File path (required)
  algorithm: string     # 'sha256' (default), 'md5'
  partial: boolean      # Only hash first 64KB (default: false)

output:
  hash: string          # Full content hash
  partial_hash: string  # First 64KB hash (always computed)
  algorithm: string
  size: integer         # File size in bytes
```

### fs_roots
List indexable root directories.

```yaml
name: fs_roots
description: List common root directories for indexing

input: {}

output:
  roots:
    type: array
    items:
      id: string        # Directory path
      path: string      # Same as id for local
      name: string      # Display name (e.g., "Documents")
      type: string      # Always "directory" for local
      is_default: boolean
```

### fs_capabilities
Get provider capabilities.

```yaml
name: fs_capabilities
description: Get provider capabilities

input: {}

output:
  can_list: boolean          # true
  can_read: boolean          # true
  can_write: boolean         # true
  can_delete: boolean        # true
  can_trash: boolean         # true (macOS)
  can_watch: boolean         # true
  can_extract_text: boolean  # true
  can_compute_hash: boolean  # true
  supports_versions: boolean # false
  supports_sharing: boolean  # false
  is_remote: boolean         # false
  requires_auth: boolean     # false
```

### fs_delete (Optional)
Delete a file.

```yaml
name: fs_delete
description: Delete a file (with optional trash)

input:
  id: string            # File path (required)
  trash: boolean        # Move to trash instead of delete (default: true)

output:
  deleted: boolean
  path: string
  trash_location: string  # If moved to trash
```

### fs_move (Optional)
Move or rename a file.

```yaml
name: fs_move
description: Move or rename a file

input:
  id: string            # Source file path (required)
  destination: string   # Destination path (required)
  overwrite: boolean    # Overwrite if exists (default: false)

output:
  success: boolean
  old_path: string
  new_path: string
```

### fs_copy (Optional)
Copy a file.

```yaml
name: fs_copy
description: Copy a file

input:
  id: string            # Source file path (required)
  destination: string   # Destination path (required)
  overwrite: boolean    # Overwrite if exists (default: false)

output:
  success: boolean
  source_path: string
  destination_path: string
  size: integer
```

### fs_extract_text (Optional)
Extract text content from a document.

```yaml
name: fs_extract_text
description: Extract plain text from documents (PDF, DOCX, etc.)

input:
  id: string            # File path (required)
  max_length: integer   # Max characters to extract (default: 100000)

output:
  text: string          # Extracted plain text
  mime_type: string     # Original file type
  truncated: boolean
  extractor: string     # Which extractor was used (e.g., "pdftotext")
```

---

## Provider Identity

| Property | Value |
|----------|-------|
| Name | `local` |
| Display Name | `Local Filesystem` |
| Type | `local` |
| Is Remote | No |
| Requires Auth | No |

## Capabilities

```go
ProviderCapabilities{
    CanList:          true,
    CanRead:          true,
    CanWrite:         true,
    CanDelete:        true,
    CanTrash:         true,   // macOS Trash support
    CanWatch:         true,   // fsnotify
    CanExtractText:   true,
    CanComputeHash:   true,
    SupportsVersions: false,  // No native versioning
    SupportsSharing:  false,
    IsRemote:         false,
    RequiresAuth:     false,
}
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Local Filesystem Provider                     │
│                                                                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐ │
│  │   Walker    │  │   Hasher    │  │   Content Extractor     │ │
│  │  (listing)  │  │  (SHA-256)  │  │  (text from documents)  │ │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘ │
│                                                                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐ │
│  │   Watcher   │  │   Trash     │  │   MIME Detector         │ │
│  │  (fsnotify) │  │  (macOS)    │  │  (file type detection)  │ │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌───────────────────┐
                    │  Local Filesystem │
                    └───────────────────┘
```

## Implementation

### Provider Structure

```go
package local

import (
    "crypto/sha256"
    "io"
    "io/fs"
    "os"
    "path/filepath"
    "time"
    
    "github.com/fsnotify/fsnotify"
)

type Provider struct {
    // Configuration
    config       Config
    
    // Exclude patterns (compiled)
    excludeRules []glob.Glob
    
    // Watcher state
    watcher      *fsnotify.Watcher
    watchPaths   map[string]WatchHandle
    
    // Content extractors
    extractors   map[string]ContentExtractor
}

type Config struct {
    // Default exclude patterns
    ExcludePatterns []string
    
    // Content extraction settings
    ExtractMaxSize   int64   // Max file size for content extraction
    PreviewLength    int     // Length of content preview
    
    // Hash settings
    PartialHashSize  int64   // Bytes for partial hash
    
    // Performance
    MaxConcurrency   int     // Max concurrent operations
}

func New(cfg Config) (*Provider, error) {
    p := &Provider{
        config:     cfg,
        watchPaths: make(map[string]WatchHandle),
        extractors: make(map[string]ContentExtractor),
    }
    
    // Compile exclude patterns
    for _, pattern := range cfg.ExcludePatterns {
        g, err := glob.Compile(pattern)
        if err != nil {
            continue
        }
        p.excludeRules = append(p.excludeRules, g)
    }
    
    // Register default extractors
    p.registerExtractors()
    
    return p, nil
}
```

### Interface Implementation

```go
// Identity
func (p *Provider) Name() string        { return "local" }
func (p *Provider) DisplayName() string { return "Local Filesystem" }

func (p *Provider) Capabilities() ProviderCapabilities {
    return ProviderCapabilities{
        CanList:        true,
        CanRead:        true,
        CanWrite:       true,
        CanDelete:      true,
        CanTrash:       true,
        CanWatch:       true,
        CanExtractText: true,
        CanComputeHash: true,
        IsRemote:       false,
        RequiresAuth:   false,
    }
}

// Connection (always connected for local)
func (p *Provider) IsConnected() bool { return true }
func (p *Provider) Connect() error    { return nil }

// ListRoots returns the user's home and common directories
func (p *Provider) ListRoots() ([]SourceRoot, error) {
    home, _ := os.UserHomeDir()
    
    roots := []SourceRoot{
        {ID: home, Path: home, Name: "Home", Type: "directory", IsDefault: true},
    }
    
    // Add common directories if they exist
    commonDirs := []struct{ name, subpath string }{
        {"Documents", "Documents"},
        {"Downloads", "Downloads"},
        {"Desktop", "Desktop"},
        {"Pictures", "Pictures"},
    }
    
    for _, dir := range commonDirs {
        path := filepath.Join(home, dir.subpath)
        if info, err := os.Stat(path); err == nil && info.IsDir() {
            roots = append(roots, SourceRoot{
                ID:   path,
                Path: path,
                Name: dir.name,
                Type: "directory",
            })
        }
    }
    
    return roots, nil
}
```

### File Listing

```go
func (p *Provider) List(path string, opts ListOptions) ([]SourceFile, error) {
    var files []SourceFile
    
    // Resolve path
    absPath, err := filepath.Abs(path)
    if err != nil {
        return nil, err
    }
    
    walkFn := func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            return nil // Skip inaccessible files
        }
        
        // Skip excluded patterns
        if p.shouldExclude(path) {
            if d.IsDir() {
                return fs.SkipDir
            }
            return nil
        }
        
        // Skip hidden files if not requested
        if !opts.IncludeHidden && isHidden(path) {
            if d.IsDir() {
                return fs.SkipDir
            }
            return nil
        }
        
        // Apply include patterns
        if len(opts.IncludePatterns) > 0 && !matchesAny(path, opts.IncludePatterns) {
            return nil
        }
        
        info, err := d.Info()
        if err != nil {
            return nil
        }
        
        sf := p.infoToSourceFile(path, info)
        files = append(files, sf)
        
        // Limit results
        if opts.Limit > 0 && len(files) >= opts.Limit {
            return fs.SkipAll
        }
        
        // Non-recursive: skip subdirectories
        if !opts.Recursive && d.IsDir() && path != absPath {
            return fs.SkipDir
        }
        
        return nil
    }
    
    err = filepath.WalkDir(absPath, walkFn)
    return files, err
}

func (p *Provider) infoToSourceFile(path string, info fs.FileInfo) SourceFile {
    return SourceFile{
        ID:          path,           // For local, path is the ID
        Path:        path,
        Name:        info.Name(),
        IsDirectory: info.IsDir(),
        Size:        info.Size(),
        MimeType:    detectMimeType(path, info),
        ModifiedAt:  info.ModTime(),
        CreatedAt:   getBirthTime(info),
        Metadata:    getFileMetadata(path, info),
    }
}

func (p *Provider) shouldExclude(path string) bool {
    for _, rule := range p.excludeRules {
        if rule.Match(path) {
            return true
        }
    }
    return false
}
```

### Metadata and Content

```go
func (p *Provider) GetMetadata(id string) (*FileMetadata, error) {
    path := id // For local provider, ID is the path
    
    info, err := os.Stat(path)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, ErrNotFound
        }
        return nil, err
    }
    
    sf := p.infoToSourceFile(path, info)
    
    return &FileMetadata{
        SourceFile:  sf,
        Permissions: info.Mode().String(),
        Owner:       getFileOwner(info),
    }, nil
}

func (p *Provider) GetContent(id string) (io.ReadCloser, error) {
    return os.Open(id)
}

func (p *Provider) ComputeHash(id string) (string, error) {
    f, err := os.Open(id)
    if err != nil {
        return "", err
    }
    defer f.Close()
    
    h := sha256.New()
    if _, err := io.Copy(h, f); err != nil {
        return "", err
    }
    
    return hex.EncodeToString(h.Sum(nil)), nil
}

func (p *Provider) ComputePartialHash(id string) (string, error) {
    f, err := os.Open(id)
    if err != nil {
        return "", err
    }
    defer f.Close()
    
    h := sha256.New()
    if _, err := io.CopyN(h, f, p.config.PartialHashSize); err != nil && err != io.EOF {
        return "", err
    }
    
    return hex.EncodeToString(h.Sum(nil)), nil
}
```

### Content Extraction

```go
// ContentExtractor interface for extracting text from files
type ContentExtractor interface {
    // SupportedTypes returns MIME types this extractor handles
    SupportedTypes() []string
    
    // Extract extracts text content from a file
    Extract(path string) (string, error)
}

func (p *Provider) registerExtractors() {
    // Plain text
    p.extractors["text/plain"] = &PlainTextExtractor{}
    
    // Markdown
    p.extractors["text/markdown"] = &PlainTextExtractor{}
    
    // PDF
    p.extractors["application/pdf"] = &PDFExtractor{}
    
    // Office documents
    p.extractors["application/vnd.openxmlformats-officedocument.wordprocessingml.document"] = &DocxExtractor{}
    
    // Code files (treat as plain text)
    codeTypes := []string{
        "text/x-go", "text/x-python", "text/javascript",
        "text/typescript", "text/x-rust", "text/x-c",
    }
    for _, t := range codeTypes {
        p.extractors[t] = &PlainTextExtractor{}
    }
}

func (p *Provider) ExtractText(id string) (string, error) {
    info, err := os.Stat(id)
    if err != nil {
        return "", err
    }
    
    // Check file size
    if info.Size() > p.config.ExtractMaxSize {
        return "", ErrFileTooLarge
    }
    
    mimeType := detectMimeType(id, info)
    
    extractor, ok := p.extractors[mimeType]
    if !ok {
        return "", ErrUnsupportedType
    }
    
    return extractor.Extract(id)
}

// PlainTextExtractor for text files
type PlainTextExtractor struct{}

func (e *PlainTextExtractor) SupportedTypes() []string {
    return []string{"text/plain", "text/markdown"}
}

func (e *PlainTextExtractor) Extract(path string) (string, error) {
    content, err := os.ReadFile(path)
    if err != nil {
        return "", err
    }
    return string(content), nil
}

// PDFExtractor uses pdftotext (poppler)
type PDFExtractor struct{}

func (e *PDFExtractor) SupportedTypes() []string {
    return []string{"application/pdf"}
}

func (e *PDFExtractor) Extract(path string) (string, error) {
    cmd := exec.Command("pdftotext", "-layout", path, "-")
    output, err := cmd.Output()
    if err != nil {
        return "", fmt.Errorf("pdftotext failed: %w", err)
    }
    return string(output), nil
}

// DocxExtractor for Word documents
type DocxExtractor struct{}

func (e *DocxExtractor) Extract(path string) (string, error) {
    // Open docx (it's a zip file)
    r, err := zip.OpenReader(path)
    if err != nil {
        return "", err
    }
    defer r.Close()
    
    // Find document.xml
    for _, f := range r.File {
        if f.Name == "word/document.xml" {
            rc, err := f.Open()
            if err != nil {
                return "", err
            }
            defer rc.Close()
            
            // Parse XML and extract text
            return extractTextFromWordXML(rc)
        }
    }
    
    return "", ErrNoContent
}
```

### MIME Type Detection

```go
// detectMimeType detects file MIME type
func detectMimeType(path string, info fs.FileInfo) string {
    if info.IsDir() {
        return "inode/directory"
    }
    
    // First try extension-based detection
    ext := strings.ToLower(filepath.Ext(path))
    if mimeType, ok := extensionMimeTypes[ext]; ok {
        return mimeType
    }
    
    // Fall back to content detection
    f, err := os.Open(path)
    if err != nil {
        return "application/octet-stream"
    }
    defer f.Close()
    
    // Read first 512 bytes for detection
    buffer := make([]byte, 512)
    n, _ := f.Read(buffer)
    
    return http.DetectContentType(buffer[:n])
}

var extensionMimeTypes = map[string]string{
    // Documents
    ".pdf":  "application/pdf",
    ".doc":  "application/msword",
    ".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
    ".xls":  "application/vnd.ms-excel",
    ".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    ".ppt":  "application/vnd.ms-powerpoint",
    ".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
    ".odt":  "application/vnd.oasis.opendocument.text",
    ".ods":  "application/vnd.oasis.opendocument.spreadsheet",
    ".rtf":  "application/rtf",
    
    // Text
    ".txt":  "text/plain",
    ".md":   "text/markdown",
    ".rst":  "text/x-rst",
    ".log":  "text/plain",
    ".csv":  "text/csv",
    
    // Code
    ".go":   "text/x-go",
    ".py":   "text/x-python",
    ".js":   "text/javascript",
    ".ts":   "text/typescript",
    ".jsx":  "text/javascript",
    ".tsx":  "text/typescript",
    ".rs":   "text/x-rust",
    ".c":    "text/x-c",
    ".cpp":  "text/x-c++",
    ".h":    "text/x-c",
    ".java": "text/x-java",
    ".rb":   "text/x-ruby",
    ".php":  "text/x-php",
    ".swift":"text/x-swift",
    ".kt":   "text/x-kotlin",
    ".sh":   "text/x-shellscript",
    ".bash": "text/x-shellscript",
    ".zsh":  "text/x-shellscript",
    ".sql":  "text/x-sql",
    
    // Config
    ".json": "application/json",
    ".yaml": "application/x-yaml",
    ".yml":  "application/x-yaml",
    ".toml": "application/toml",
    ".xml":  "application/xml",
    ".ini":  "text/plain",
    
    // Images
    ".jpg":  "image/jpeg",
    ".jpeg": "image/jpeg",
    ".png":  "image/png",
    ".gif":  "image/gif",
    ".webp": "image/webp",
    ".svg":  "image/svg+xml",
    ".ico":  "image/x-icon",
    ".bmp":  "image/bmp",
    ".tiff": "image/tiff",
    ".heic": "image/heic",
    ".heif": "image/heif",
    ".raw":  "image/x-raw",
    ".cr2":  "image/x-canon-cr2",
    ".nef":  "image/x-nikon-nef",
    
    // Video
    ".mp4":  "video/mp4",
    ".mov":  "video/quicktime",
    ".avi":  "video/x-msvideo",
    ".mkv":  "video/x-matroska",
    ".webm": "video/webm",
    ".wmv":  "video/x-ms-wmv",
    ".flv":  "video/x-flv",
    ".m4v":  "video/x-m4v",
    
    // Audio
    ".mp3":  "audio/mpeg",
    ".wav":  "audio/wav",
    ".flac": "audio/flac",
    ".aac":  "audio/aac",
    ".ogg":  "audio/ogg",
    ".m4a":  "audio/mp4",
    ".wma":  "audio/x-ms-wma",
    
    // Archives
    ".zip":  "application/zip",
    ".tar":  "application/x-tar",
    ".gz":   "application/gzip",
    ".bz2":  "application/x-bzip2",
    ".xz":   "application/x-xz",
    ".7z":   "application/x-7z-compressed",
    ".rar":  "application/vnd.rar",
    ".dmg":  "application/x-apple-diskimage",
    
    // Executables
    ".exe":  "application/x-msdownload",
    ".app":  "application/x-apple-app",
    ".bin":  "application/octet-stream",
}
```

### File Watching

```go
func (p *Provider) Watch(path string, callback WatchCallback) (WatchHandle, error) {
    if p.watcher == nil {
        w, err := fsnotify.NewWatcher()
        if err != nil {
            return nil, err
        }
        p.watcher = w
        go p.watchLoop()
    }
    
    // Add path to watcher
    if err := p.watcher.Add(path); err != nil {
        return nil, err
    }
    
    handle := &localWatchHandle{
        path:     path,
        callback: callback,
        provider: p,
    }
    
    p.watchPaths[path] = handle
    return handle, nil
}

func (p *Provider) watchLoop() {
    for {
        select {
        case event, ok := <-p.watcher.Events:
            if !ok {
                return
            }
            p.handleWatchEvent(event)
            
        case err, ok := <-p.watcher.Errors:
            if !ok {
                return
            }
            slog.Error("watcher error", "error", err)
        }
    }
}

func (p *Provider) handleWatchEvent(event fsnotify.Event) {
    var eventType string
    switch {
    case event.Op&fsnotify.Create != 0:
        eventType = "created"
    case event.Op&fsnotify.Write != 0:
        eventType = "modified"
    case event.Op&fsnotify.Remove != 0:
        eventType = "deleted"
    case event.Op&fsnotify.Rename != 0:
        eventType = "moved"
    default:
        return
    }
    
    // Find matching watch handles
    for watchPath, handle := range p.watchPaths {
        if strings.HasPrefix(event.Name, watchPath) {
            var sf *SourceFile
            if eventType != "deleted" {
                if info, err := os.Stat(event.Name); err == nil {
                    file := p.infoToSourceFile(event.Name, info)
                    sf = &file
                }
            }
            
            handle.callback(WatchEvent{
                Type: eventType,
                Path: event.Name,
                File: sf,
            })
        }
    }
}

type localWatchHandle struct {
    path     string
    callback WatchCallback
    provider *Provider
}

func (h *localWatchHandle) Stop() error {
    delete(h.provider.watchPaths, h.path)
    return h.provider.watcher.Remove(h.path)
}
```

### File Operations (Mutations)

```go
func (p *Provider) Delete(id string, trash bool) error {
    if trash {
        return p.moveToTrash(id)
    }
    return os.RemoveAll(id)
}

func (p *Provider) moveToTrash(path string) error {
    // macOS-specific: use osascript to move to trash
    // This preserves the "put back" functionality
    script := fmt.Sprintf(`
        tell application "Finder"
            delete POSIX file %q
        end tell
    `, path)
    
    cmd := exec.Command("osascript", "-e", script)
    return cmd.Run()
}

func (p *Provider) Move(id string, newPath string) error {
    // Ensure parent directory exists
    if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
        return err
    }
    return os.Rename(id, newPath)
}

func (p *Provider) Copy(id string, destPath string) error {
    src, err := os.Open(id)
    if err != nil {
        return err
    }
    defer src.Close()
    
    // Ensure parent directory exists
    if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
        return err
    }
    
    dst, err := os.Create(destPath)
    if err != nil {
        return err
    }
    defer dst.Close()
    
    _, err = io.Copy(dst, src)
    return err
}
```

### Category Classification

```go
// classifyFile determines category and subcategory
func classifyFile(path string, mimeType string) (category, subcategory string) {
    // Category based on MIME type
    switch {
    case strings.HasPrefix(mimeType, "text/"):
        if strings.Contains(mimeType, "x-") {
            category = "code"
        } else {
            category = "document"
        }
    case strings.HasPrefix(mimeType, "image/"):
        category = "image"
    case strings.HasPrefix(mimeType, "video/"):
        category = "video"
    case strings.HasPrefix(mimeType, "audio/"):
        category = "audio"
    case strings.HasPrefix(mimeType, "application/"):
        switch {
        case strings.Contains(mimeType, "pdf"):
            category = "document"
        case strings.Contains(mimeType, "word"), strings.Contains(mimeType, "document"):
            category = "document"
        case strings.Contains(mimeType, "sheet"), strings.Contains(mimeType, "excel"):
            category = "spreadsheet"
        case strings.Contains(mimeType, "presentation"), strings.Contains(mimeType, "powerpoint"):
            category = "presentation"
        case strings.Contains(mimeType, "zip"), strings.Contains(mimeType, "tar"),
             strings.Contains(mimeType, "compressed"), strings.Contains(mimeType, "archive"):
            category = "archive"
        case strings.Contains(mimeType, "json"), strings.Contains(mimeType, "xml"),
             strings.Contains(mimeType, "yaml"):
            category = "data"
        case strings.Contains(mimeType, "executable"), strings.Contains(mimeType, "x-msdownload"):
            category = "executable"
        default:
            category = "other"
        }
    default:
        category = "other"
    }
    
    // Subcategory based on filename patterns and path
    filename := strings.ToLower(filepath.Base(path))
    pathLower := strings.ToLower(path)
    
    switch category {
    case "document":
        if strings.Contains(filename, "invoice") || strings.Contains(filename, "faktura") {
            subcategory = "invoice"
        } else if strings.Contains(filename, "receipt") || strings.Contains(filename, "paragon") {
            subcategory = "receipt"
        } else if strings.Contains(filename, "contract") || strings.Contains(filename, "umowa") {
            subcategory = "contract"
        } else if strings.Contains(filename, "resume") || strings.Contains(filename, "cv") {
            subcategory = "resume"
        }
        
    case "image":
        if strings.Contains(pathLower, "screenshot") || strings.HasPrefix(filename, "screen") {
            subcategory = "screenshot"
        } else if strings.Contains(pathLower, "/dcim/") || strings.Contains(pathLower, "/photos/") {
            subcategory = "photo"
        }
        
    case "code":
        // Detect config files
        configPatterns := []string{
            "config", "settings", ".rc", "makefile", "dockerfile",
            "package.json", "go.mod", "cargo.toml", "requirements.txt",
        }
        for _, pattern := range configPatterns {
            if strings.Contains(filename, pattern) {
                subcategory = "config"
                break
            }
        }
    }
    
    return category, subcategory
}
```

### Platform-Specific Utilities

```go
// isHidden checks if file is hidden
func isHidden(path string) bool {
    // Unix: starts with dot
    if strings.HasPrefix(filepath.Base(path), ".") {
        return true
    }
    
    // Check parent directories
    for _, part := range strings.Split(path, string(filepath.Separator)) {
        if strings.HasPrefix(part, ".") {
            return true
        }
    }
    
    return false
}

// getBirthTime gets file creation time (macOS/BSD)
func getBirthTime(info fs.FileInfo) *time.Time {
    // Platform-specific implementation
    // On macOS, use syscall to get birth time
    if stat, ok := info.Sys().(*syscall.Stat_t); ok {
        sec := stat.Birthtimespec.Sec
        nsec := stat.Birthtimespec.Nsec
        t := time.Unix(sec, nsec)
        return &t
    }
    return nil
}

// getFileOwner gets file owner (Unix)
func getFileOwner(info fs.FileInfo) string {
    if stat, ok := info.Sys().(*syscall.Stat_t); ok {
        uid := stat.Uid
        if u, err := user.LookupId(fmt.Sprintf("%d", uid)); err == nil {
            return u.Username
        }
    }
    return ""
}

// getFileMetadata extracts platform-specific metadata
func getFileMetadata(path string, info fs.FileInfo) map[string]any {
    meta := make(map[string]any)
    
    // Symlink info
    if info.Mode()&os.ModeSymlink != 0 {
        if target, err := os.Readlink(path); err == nil {
            meta["symlink_target"] = target
        }
    }
    
    // Extended attributes (macOS)
    if attrs, err := xattr.List(path); err == nil && len(attrs) > 0 {
        meta["xattrs"] = attrs
    }
    
    return meta
}
```

## Configuration

### Default Configuration

```go
var DefaultConfig = Config{
    ExcludePatterns: []string{
        // System files
        "**/.DS_Store",
        "**/Thumbs.db",
        "**/.Spotlight-*",
        "**/.fseventsd",
        "**/.Trashes",
        
        // Version control
        "**/.git/**",
        "**/.svn/**",
        "**/.hg/**",
        
        // Dependencies
        "**/node_modules/**",
        "**/vendor/**",
        "**/__pycache__/**",
        "**/.venv/**",
        "**/venv/**",
        
        // Build outputs
        "**/build/**",
        "**/dist/**",
        "**/target/**",
        "**/*.o",
        "**/*.pyc",
        
        // IDE
        "**/.idea/**",
        "**/.vscode/**",
        "**/*.swp",
        
        // Trash
        "**/.Trash/**",
        "**/Trash/**",
    },
    
    ExtractMaxSize:  10 * 1024 * 1024, // 10 MB
    PreviewLength:   500,
    PartialHashSize: 64 * 1024,        // 64 KB
    MaxConcurrency:  4,
}
```

## File Structure

### Stage 1: Internal Provider

```
server/mcp/tools/documents/providers/local/
├── provider.go           # Main provider implementation
├── walk.go               # Directory walking
├── hash.go               # Hashing utilities
├── mime.go               # MIME type detection
├── classify.go           # Category classification
├── watch.go              # File system watching
├── extract.go            # Content extraction coordinator
├── operations.go         # Delete, move, copy operations
├── platform_darwin.go    # macOS-specific (trash, birth time)
├── platform_linux.go     # Linux-specific
├── extractors/
│   ├── text.go           # Plain text extractor
│   ├── pdf.go            # PDF extractor (pdftotext)
│   ├── docx.go           # Word document extractor
│   └── xlsx.go           # Excel extractor
└── LOCAL_FS_PROVIDER_PLAN.md
```

### Stage 2: Standalone MCP (Additional Files)

```
server/mcp/tools/localfs/               # NEW: Standalone MCP
├── localfs.go            # MCP tool definitions and registration
├── handlers.go           # Tool handlers (wraps provider methods)
└── README.md

# The provider code stays in documents/providers/local/
# The MCP just wraps it with tool handlers
```

## Implementation Phases

### Stage 1 Phases (Internal Provider)

#### Phase 1.1: Core Provider
- [ ] Provider structure and interface
- [ ] ListRoots implementation
- [ ] List with basic walking
- [ ] GetMetadata
- [ ] GetContent

#### Phase 1.2: MIME and Classification
- [ ] Extension-based MIME detection
- [ ] Content-based MIME detection
- [ ] Category classification
- [ ] Subcategory detection

#### Phase 1.3: Hashing
- [ ] Full content hash
- [ ] Partial hash (first 64KB)
- [ ] Concurrent hashing

#### Phase 1.4: Content Extraction
- [ ] Plain text extractor
- [ ] PDF extractor (pdftotext integration)
- [ ] DOCX extractor
- [ ] Code file handling

#### Phase 1.5: File Watching
- [ ] fsnotify integration
- [ ] Watch handle management
- [ ] Event dispatching

#### Phase 1.6: File Operations
- [ ] Delete (direct)
- [ ] Move to Trash (macOS)
- [ ] Move/rename
- [ ] Copy

#### Phase 1.7: Platform Features
- [ ] macOS: Birth time, xattrs
- [ ] Linux compatibility
- [ ] Symlink handling

### Stage 2 Phases (MCP Provider)

#### Phase 2.1: MCP Wrapper
- [ ] Create localfs MCP package
- [ ] Register MCP tools
- [ ] Create tool handlers that wrap provider methods

#### Phase 2.2: Required Tools
- [ ] fs_list handler
- [ ] fs_get handler
- [ ] fs_read handler
- [ ] fs_hash handler
- [ ] fs_roots handler
- [ ] fs_capabilities handler

#### Phase 2.3: Optional Tools
- [ ] fs_delete handler
- [ ] fs_move handler
- [ ] fs_copy handler
- [ ] fs_extract_text handler

#### Phase 2.4: Integration
- [ ] Register with Document Manager as MCP source
- [ ] Test MCP-to-MCP communication
- [ ] Performance comparison with Stage 1

## Testing

### Unit Tests

```go
func TestProvider_List(t *testing.T) {
    // Create temp directory with test files
    tmpDir := t.TempDir()
    createTestFiles(tmpDir)
    
    p, _ := New(DefaultConfig)
    
    files, err := p.List(tmpDir, ListOptions{Recursive: true})
    require.NoError(t, err)
    assert.Len(t, files, expectedCount)
}

func TestProvider_ComputeHash(t *testing.T) {
    // Create file with known content
    tmpFile := filepath.Join(t.TempDir(), "test.txt")
    os.WriteFile(tmpFile, []byte("hello world"), 0644)
    
    p, _ := New(DefaultConfig)
    hash, err := p.ComputeHash(tmpFile)
    
    require.NoError(t, err)
    assert.Equal(t, expectedSHA256, hash)
}

func TestProvider_ExtractText_PDF(t *testing.T) {
    // Requires pdftotext to be installed
    if _, err := exec.LookPath("pdftotext"); err != nil {
        t.Skip("pdftotext not installed")
    }
    
    p, _ := New(DefaultConfig)
    text, err := p.ExtractText("testdata/sample.pdf")
    
    require.NoError(t, err)
    assert.Contains(t, text, "expected content")
}
```

### Integration Tests

```go
func TestProvider_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    
    p, _ := New(DefaultConfig)
    
    // Test with real home directory
    roots, err := p.ListRoots()
    require.NoError(t, err)
    assert.NotEmpty(t, roots)
    
    // List documents
    home, _ := os.UserHomeDir()
    docs := filepath.Join(home, "Documents")
    if _, err := os.Stat(docs); err == nil {
        files, err := p.List(docs, ListOptions{Limit: 10})
        require.NoError(t, err)
        t.Logf("Found %d files in Documents", len(files))
    }
}
```

## Performance Considerations

1. **Concurrent walking**: Use worker pool for large directories
2. **Hash caching**: Cache partial hashes during listing
3. **Lazy content extraction**: Only extract when requested
4. **Batch operations**: Group file operations for efficiency
5. **Exclude early**: Apply exclude patterns during walk, not after

## Error Handling

| Error | Handling |
|-------|----------|
| Permission denied | Skip file, log warning, continue |
| File too large | Skip content extraction, index metadata only |
| Symlink loop | Detect and skip |
| File disappeared | Return ErrNotFound |
| Disk full | Return error for write operations |
| pdftotext not found | Return ErrUnsupportedType for PDFs |

## Dependencies

| Dependency | Purpose | Required |
|------------|---------|----------|
| `github.com/fsnotify/fsnotify` | File system watching | Yes |
| `github.com/gobwas/glob` | Glob pattern matching | Yes |
| `github.com/pkg/xattr` | Extended attributes | Optional |
| `pdftotext` (poppler) | PDF text extraction | Optional |
