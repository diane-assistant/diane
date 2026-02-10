// Package downloads provides file download management tools for the MCP server.
// Downloads files to ~/.diane/downloads/ with background processing and status tracking.
package downloads

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/diane-assistant/diane/mcp/tools"
)

const (
	userAgent = "diane github.com/diane-assistant/diane"
)

// DownloadStatus represents the current state of a download
type DownloadStatus string

const (
	StatusPending    DownloadStatus = "pending"
	StatusInProgress DownloadStatus = "in_progress"
	StatusCompleted  DownloadStatus = "completed"
	StatusFailed     DownloadStatus = "failed"
)

// Download represents a single download task
type Download struct {
	ID           string         `json:"id"`
	URL          string         `json:"url"`
	Filename     string         `json:"filename"`
	FilePath     string         `json:"file_path,omitempty"`
	Status       DownloadStatus `json:"status"`
	BytesTotal   int64          `json:"bytes_total,omitempty"`
	BytesWritten int64          `json:"bytes_written"`
	StartedAt    time.Time      `json:"started_at"`
	CompletedAt  *time.Time     `json:"completed_at,omitempty"`
	Error        string         `json:"error,omitempty"`
}

// Tool represents an MCP tool definition
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// Provider implements download management tools
type Provider struct {
	downloadDir string
	downloads   map[string]*Download
	mu          sync.RWMutex
	httpClient  *http.Client
}

// NewProvider creates a new downloads provider
func NewProvider() (*Provider, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	downloadDir := filepath.Join(home, ".diane", "downloads")
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create downloads directory: %w", err)
	}

	return &Provider{
		downloadDir: downloadDir,
		downloads:   make(map[string]*Download),
		httpClient: &http.Client{
			Timeout: 30 * time.Minute, // Long timeout for large files
		},
	}, nil
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "downloads"
}

// CheckDependencies verifies requirements (no external deps needed)
func (p *Provider) CheckDependencies() error {
	return nil
}

// Tools returns the list of download tools
func (p *Provider) Tools() []Tool {
	return []Tool{
		{
			Name:        "downloads_start",
			Description: "Start downloading a file from a URL. Downloads run asynchronously in the background - this tool returns immediately with a download ID. Use downloads_status with the returned ID to check progress and completion. Files are saved to ~/.diane/downloads/. Only HTTP and HTTPS URLs are supported.",
			InputSchema: map[string]interface{}{
				"type":     "object",
				"required": []string{"url"},
				"properties": map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "The URL of the file to download. Must be http:// or https://",
					},
					"filename": map[string]interface{}{
						"type":        "string",
						"description": "Custom filename for the downloaded file. If not provided, the filename is extracted from the URL path or Content-Disposition header",
					},
				},
			},
		},
		{
			Name:        "downloads_status",
			Description: "Check the status of downloads. Returns status, progress (bytes_written/bytes_total), and file path when complete. Status values: 'pending' (queued), 'in_progress' (downloading), 'completed' (finished, file_path available), 'failed' (error field contains reason).",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "The download ID returned by downloads_start. If omitted, returns status of all downloads from this session",
					},
				},
			},
		},
		{
			Name:        "downloads_list",
			Description: "List all files currently stored in the downloads directory (~/.diane/downloads/). Returns file names, sizes, modification times, and full paths. This lists files on disk, not active download tasks - use downloads_status to check active downloads.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "downloads_delete",
			Description: "Delete a file from the downloads directory. Only accepts filenames (not paths) to prevent deleting files outside the downloads directory. Cannot delete directories.",
			InputSchema: map[string]interface{}{
				"type":     "object",
				"required": []string{"filename"},
				"properties": map[string]interface{}{
					"filename": map[string]interface{}{
						"type":        "string",
						"description": "The filename to delete. Use the name from downloads_list, not a full path",
					},
				},
			},
		},
	}
}

// HasTool checks if a tool name belongs to this provider
func (p *Provider) HasTool(name string) bool {
	switch name {
	case "downloads_start", "downloads_status", "downloads_list", "downloads_delete":
		return true
	}
	return false
}

// Call executes a download tool
func (p *Provider) Call(name string, args map[string]interface{}) (interface{}, error) {
	switch name {
	case "downloads_start":
		return p.startDownload(args)
	case "downloads_status":
		return p.getStatus(args)
	case "downloads_list":
		return p.listFiles(args)
	case "downloads_delete":
		return p.deleteFile(args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// startDownload initiates a background download
func (p *Provider) startDownload(args map[string]interface{}) (interface{}, error) {
	downloadURL, ok := args["url"].(string)
	if !ok || downloadURL == "" {
		return nil, fmt.Errorf("url is required")
	}

	// Validate URL
	parsedURL, err := url.Parse(downloadURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("only http and https URLs are supported")
	}

	// Generate download ID
	id := generateID()

	// Determine filename
	filename, _ := args["filename"].(string)
	if filename == "" {
		filename = extractFilenameFromURL(parsedURL)
	}

	// Create download record
	download := &Download{
		ID:        id,
		URL:       downloadURL,
		Filename:  filename,
		Status:    StatusPending,
		StartedAt: time.Now(),
	}

	p.mu.Lock()
	p.downloads[id] = download
	p.mu.Unlock()

	// Start download in background
	go p.performDownload(download)

	return textContent(map[string]interface{}{
		"message":      "Download started",
		"id":           id,
		"filename":     filename,
		"status":       StatusPending,
		"download_dir": p.downloadDir,
	}), nil
}

// performDownload executes the actual download
func (p *Provider) performDownload(download *Download) {
	p.updateStatus(download.ID, StatusInProgress, "", 0)

	// Create HTTP request
	req, err := http.NewRequest("GET", download.URL, nil)
	if err != nil {
		p.updateStatus(download.ID, StatusFailed, fmt.Sprintf("failed to create request: %v", err), 0)
		return
	}
	req.Header.Set("User-Agent", userAgent)

	// Execute request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		p.updateStatus(download.ID, StatusFailed, fmt.Sprintf("download failed: %v", err), 0)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		p.updateStatus(download.ID, StatusFailed, fmt.Sprintf("server returned %s", resp.Status), 0)
		return
	}

	// Update filename from Content-Disposition if available and not custom
	p.mu.RLock()
	currentFilename := download.Filename
	p.mu.RUnlock()

	if currentFilename == "download" || currentFilename == "" {
		if cd := resp.Header.Get("Content-Disposition"); cd != "" {
			if fn := extractFilenameFromContentDisposition(cd); fn != "" {
				p.mu.Lock()
				download.Filename = fn
				currentFilename = fn
				p.mu.Unlock()
			}
		}
	}

	// Update total size
	if resp.ContentLength > 0 {
		p.mu.Lock()
		download.BytesTotal = resp.ContentLength
		p.mu.Unlock()
	}

	// Sanitize filename and create file path
	safeFilename := sanitizeFilename(currentFilename)
	filePath := filepath.Join(p.downloadDir, safeFilename)

	// Handle duplicate filenames
	filePath = p.getUniqueFilePath(filePath)

	// Create file
	file, err := os.Create(filePath)
	if err != nil {
		p.updateStatus(download.ID, StatusFailed, fmt.Sprintf("failed to create file: %v", err), 0)
		return
	}
	defer file.Close()

	// Copy with progress tracking
	var bytesWritten int64
	buf := make([]byte, 32*1024) // 32KB buffer

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			written, writeErr := file.Write(buf[:n])
			if writeErr != nil {
				p.updateStatus(download.ID, StatusFailed, fmt.Sprintf("write error: %v", writeErr), bytesWritten)
				return
			}
			bytesWritten += int64(written)

			// Update progress periodically
			p.mu.Lock()
			download.BytesWritten = bytesWritten
			p.mu.Unlock()
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			p.updateStatus(download.ID, StatusFailed, fmt.Sprintf("read error: %v", readErr), bytesWritten)
			return
		}
	}

	// Mark as completed
	now := time.Now()
	p.mu.Lock()
	download.Status = StatusCompleted
	download.FilePath = filePath
	download.CompletedAt = &now
	download.BytesWritten = bytesWritten
	download.Filename = filepath.Base(filePath)
	p.mu.Unlock()

	slog.Info("Download completed", "id", download.ID, "file", filePath, "bytes", bytesWritten)
}

// updateStatus updates download status
func (p *Provider) updateStatus(id string, status DownloadStatus, errorMsg string, bytesWritten int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if download, ok := p.downloads[id]; ok {
		download.Status = status
		download.Error = errorMsg
		if bytesWritten > 0 {
			download.BytesWritten = bytesWritten
		}
		if status == StatusCompleted || status == StatusFailed {
			now := time.Now()
			download.CompletedAt = &now
		}
	}
}

// getStatus returns status of downloads
func (p *Provider) getStatus(args map[string]interface{}) (interface{}, error) {
	id, _ := args["id"].(string)

	p.mu.RLock()
	defer p.mu.RUnlock()

	if id != "" {
		// Get specific download
		download, ok := p.downloads[id]
		if !ok {
			return nil, fmt.Errorf("download not found: %s", id)
		}
		return textContent(download), nil
	}

	// Get all downloads
	var downloads []*Download
	for _, d := range p.downloads {
		downloads = append(downloads, d)
	}

	if len(downloads) == 0 {
		return textContent(map[string]interface{}{
			"message":   "No downloads in progress or recent history",
			"downloads": []interface{}{},
		}), nil
	}

	return textContent(map[string]interface{}{
		"count":     len(downloads),
		"downloads": downloads,
	}), nil
}

// listFiles lists all files in the downloads directory
func (p *Provider) listFiles(args map[string]interface{}) (interface{}, error) {
	entries, err := os.ReadDir(p.downloadDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read downloads directory: %w", err)
	}

	type FileInfo struct {
		Name        string    `json:"name"`
		Size        int64     `json:"size"`
		ModifiedAt  time.Time `json:"modified_at"`
		FullPath    string    `json:"full_path"`
		IsDirectory bool      `json:"is_directory"`
	}

	var files []FileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, FileInfo{
			Name:        entry.Name(),
			Size:        info.Size(),
			ModifiedAt:  info.ModTime(),
			FullPath:    filepath.Join(p.downloadDir, entry.Name()),
			IsDirectory: entry.IsDir(),
		})
	}

	return textContent(map[string]interface{}{
		"download_dir": p.downloadDir,
		"count":        len(files),
		"files":        files,
	}), nil
}

// deleteFile deletes a file from the downloads directory
func (p *Provider) deleteFile(args map[string]interface{}) (interface{}, error) {
	filename, ok := args["filename"].(string)
	if !ok || filename == "" {
		return nil, fmt.Errorf("filename is required")
	}

	// Sanitize to prevent path traversal
	filename = filepath.Base(filename)
	filePath := filepath.Join(p.downloadDir, filename)

	// Check if file exists
	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: %s", filename)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Don't allow deleting directories for safety
	if info.IsDir() {
		return nil, fmt.Errorf("cannot delete directories, only files")
	}

	// Delete the file
	if err := os.Remove(filePath); err != nil {
		return nil, fmt.Errorf("failed to delete file: %w", err)
	}

	// Also remove from downloads map if present
	p.mu.Lock()
	for id, d := range p.downloads {
		if d.FilePath == filePath || d.Filename == filename {
			delete(p.downloads, id)
			break
		}
	}
	p.mu.Unlock()

	return textContent(map[string]interface{}{
		"message":  "File deleted successfully",
		"filename": filename,
	}), nil
}

// Helper functions

func generateID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func extractFilenameFromURL(u *url.URL) string {
	path := u.Path
	if path == "" || path == "/" {
		return "download"
	}

	// Get the last segment
	segments := strings.Split(path, "/")
	for i := len(segments) - 1; i >= 0; i-- {
		if segments[i] != "" {
			// URL decode the filename
			decoded, err := url.QueryUnescape(segments[i])
			if err != nil {
				return segments[i]
			}
			return decoded
		}
	}
	return "download"
}

func extractFilenameFromContentDisposition(cd string) string {
	// Simple parsing of Content-Disposition header
	// Format: attachment; filename="example.pdf" or filename*=UTF-8''example.pdf
	parts := strings.Split(cd, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(strings.ToLower(part), "filename=") {
			filename := strings.TrimPrefix(part, "filename=")
			filename = strings.TrimPrefix(filename, "\"")
			filename = strings.TrimSuffix(filename, "\"")
			return filename
		}
		if strings.HasPrefix(strings.ToLower(part), "filename*=") {
			// UTF-8 encoded filename
			filename := strings.TrimPrefix(part, "filename*=")
			if idx := strings.Index(filename, "''"); idx != -1 {
				filename = filename[idx+2:]
				decoded, err := url.QueryUnescape(filename)
				if err == nil {
					return decoded
				}
			}
		}
	}
	return ""
}

func sanitizeFilename(filename string) string {
	// Remove path separators and null bytes
	filename = strings.ReplaceAll(filename, "/", "_")
	filename = strings.ReplaceAll(filename, "\\", "_")
	filename = strings.ReplaceAll(filename, "\x00", "")

	// Remove leading/trailing dots and spaces
	filename = strings.Trim(filename, ". ")

	// If empty after sanitization, use default
	if filename == "" {
		filename = "download"
	}

	// Limit length
	if len(filename) > 255 {
		ext := filepath.Ext(filename)
		base := filename[:255-len(ext)]
		filename = base + ext
	}

	return filename
}

func (p *Provider) getUniqueFilePath(filePath string) string {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return filePath
	}

	ext := filepath.Ext(filePath)
	base := strings.TrimSuffix(filePath, ext)

	for i := 1; i < 1000; i++ {
		newPath := fmt.Sprintf("%s_%d%s", base, i, ext)
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newPath
		}
	}

	// Fallback with timestamp
	return fmt.Sprintf("%s_%d%s", base, time.Now().UnixNano(), ext)
}

// textContent formats result as MCP text content
func textContent(data interface{}) map[string]interface{} {
	jsonBytes, _ := json.MarshalIndent(data, "", "  ")
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": string(jsonBytes),
			},
		},
	}
}

// --- MCP Resources ---

// Resources returns all resources provided by the Downloads provider
func (p *Provider) Resources() []tools.Resource {
	return []tools.Resource{
		{
			URI:         "downloads://guide",
			Name:        "Downloads Tools Guide",
			Description: "Documentation for using file download tools: async download pattern, status checking, file management",
			MimeType:    "text/markdown",
		},
	}
}

// ReadResource returns the content of a resource
func (p *Provider) ReadResource(uri string) (*tools.ResourceContent, error) {
	switch uri {
	case "downloads://guide":
		return p.resourceGuide()
	default:
		return nil, fmt.Errorf("resource not found: %s", uri)
	}
}

func (p *Provider) resourceGuide() (*tools.ResourceContent, error) {
	guide := strings.TrimSpace(`
# Downloads Tools Guide

File download management for Diane. Downloads files from URLs to a local directory with background processing and status tracking.

## Storage Location

All downloaded files are saved to: ` + "`~/.diane/downloads/`" + `

## Available Tools

| Tool | Purpose |
|------|---------|
| downloads_start | Start a new download (async) |
| downloads_status | Check download progress/completion |
| downloads_list | List files in downloads directory |
| downloads_delete | Remove a downloaded file |

## Async Download Pattern

Downloads run in the background. The typical workflow is:

1. **Start the download:**
` + "```" + `
downloads_start url="https://example.com/file.pdf"
` + "```" + `
Response:
` + "```json" + `
{
  "id": "a1b2c3d4e5f6",
  "filename": "file.pdf",
  "status": "pending"
}
` + "```" + `

2. **Check status (poll if needed):**
` + "```" + `
downloads_status id="a1b2c3d4e5f6"
` + "```" + `
Response while downloading:
` + "```json" + `
{
  "id": "a1b2c3d4e5f6",
  "status": "in_progress",
  "bytes_written": 1048576,
  "bytes_total": 5242880
}
` + "```" + `
Response when complete:
` + "```json" + `
{
  "id": "a1b2c3d4e5f6",
  "status": "completed",
  "file_path": "/Users/name/.diane/downloads/file.pdf",
  "bytes_written": 5242880
}
` + "```" + `

## Status Values

| Status | Meaning |
|--------|---------|
| pending | Download queued, about to start |
| in_progress | Currently downloading (check bytes_written for progress) |
| completed | Download finished successfully (file_path is available) |
| failed | Download failed (error field contains reason) |

## Listing and Managing Files

**List all downloaded files:**
` + "```" + `
downloads_list
` + "```" + `
Returns files with name, size, modification time, and full path.

**Delete a file:**
` + "```" + `
downloads_delete filename="file.pdf"
` + "```" + `
Use the filename only (not the full path). This prevents accidental deletion of files outside the downloads directory.

## Filename Handling

- If no custom filename is provided, it's extracted from the URL path
- The Content-Disposition header is respected if the URL doesn't have a clear filename
- Duplicate filenames get a numeric suffix (e.g., file_1.pdf, file_2.pdf)
- Filenames are sanitized to remove unsafe characters

## Limitations

- Only HTTP and HTTPS URLs are supported
- Download status is tracked in memory (session only) - restarting Diane clears the download history
- Files on disk persist across restarts (use downloads_list to see them)
- Maximum download timeout is 30 minutes per file

## Example Use Cases

### Download and verify a file
` + "```" + `
1. downloads_start url="https://example.com/data.csv"
2. downloads_status id="<returned_id>"  # repeat until completed
3. downloads_list  # verify file exists
` + "```" + `

### Clean up old downloads
` + "```" + `
1. downloads_list  # see all files
2. downloads_delete filename="old-file.pdf"
` + "```" + `

### Download with custom filename
` + "```" + `
downloads_start url="https://example.com/download?id=123" filename="report-2024.pdf"
` + "```" + `
`)

	return &tools.ResourceContent{
		URI:      "downloads://guide",
		MimeType: "text/markdown",
		Text:     guide,
	}, nil
}
