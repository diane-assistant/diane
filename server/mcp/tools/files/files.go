// Package files provides MCP tools for unified file indexing and management.
// This is a passive index - files are registered by the LLM using metadata
// gathered from existing tools (shell commands, Drive MCP, Gmail MCP, etc.)
//
// Backend: Emergent Knowledge Base (graph objects + vector search)
package files

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	sdk "github.com/emergent-company/emergent/apps/server-go/pkg/sdk"
	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"

	"github.com/diane-assistant/diane/internal/emergent"
)

// Tool represents an MCP tool definition
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// Provider implements file management tools backed by Emergent
type Provider struct {
	client *sdk.Client
}

// NewProvider creates a new files provider using Emergent as the backend.
// Configuration is managed by the shared emergent package, reading from
// ~/.diane/secrets/emergent-config.json or environment variables
// (EMERGENT_BASE_URL, EMERGENT_API_KEY).
//
// The API key is project-scoped — no project ID is needed.
func NewProvider() (*Provider, error) {
	client, err := emergent.GetClient()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Emergent client: %w", err)
	}

	return &Provider{
		client: client,
	}, nil
}

// Close is a no-op for the Emergent-backed provider (HTTP client, no persistent connections)
func (p *Provider) Close() error {
	return nil
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "file_registry"
}

// CheckDependencies verifies Emergent configuration is present
func (p *Provider) CheckDependencies() error {
	if p.client == nil {
		return fmt.Errorf("Emergent client not initialized")
	}
	return nil
}

// --- Schema Helpers ---

func objectSchema(properties map[string]interface{}, required []string) map[string]interface{} {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func stringProperty(description string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "string",
		"description": description,
	}
}

func intProperty(description string, defaultVal int) map[string]interface{} {
	return map[string]interface{}{
		"type":        "integer",
		"description": description,
		"default":     defaultVal,
	}
}

func boolProperty(description string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "boolean",
		"description": description,
	}
}

func arrayProperty(description string, itemType string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "array",
		"description": description,
		"items":       map[string]interface{}{"type": itemType},
	}
}

func numberProperty(description string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "number",
		"description": description,
	}
}

// Tools returns the list of file management tools
func (p *Provider) Tools() []Tool {
	return []Tool{
		{
			Name: "file_registry_register",
			Description: `Register a file in the unified file index. Use this after discovering files via shell commands (find, stat, file, sha256sum) or other tools (Drive, Gmail). 
The LLM gathers file metadata and registers it here for unified search and management.
Required: source, path, and content_hash. Other fields are optional but improve search.`,
			InputSchema: objectSchema(
				map[string]interface{}{
					"source":          stringProperty("Source identifier: 'local', 'gdrive', 'gmail', 'dropbox', etc."),
					"path":            stringProperty("Full path within the source (e.g., '/Users/me/docs/file.pdf' for local)"),
					"source_file_id":  stringProperty("Provider-specific file ID (e.g., Google Drive file ID)"),
					"filename":        stringProperty("Base filename (extracted from path if not provided)"),
					"size":            intProperty("File size in bytes", 0),
					"mime_type":       stringProperty("MIME type (e.g., 'application/pdf')"),
					"is_directory":    boolProperty("True if this is a directory"),
					"created_at":      stringProperty("Creation timestamp (ISO 8601 format)"),
					"modified_at":     stringProperty("Modification timestamp (ISO 8601 format)"),
					"content_hash":    stringProperty("SHA-256 hash of full file content (required for duplicate detection)"),
					"partial_hash":    stringProperty("SHA-256 hash of first 64KB (for quick duplicate detection)"),
					"content_text":    stringProperty("Extracted text content (for full-text search and embeddings)"),
					"content_preview": stringProperty("First ~500 chars preview"),
					"category":        stringProperty("File category: document, image, video, audio, code, archive, data, other"),
					"subcategory":     stringProperty("Subcategory: invoice, receipt, photo, screenshot, etc."),
					"tags":            arrayProperty("Tags/labels to apply to this file", "string"),
				},
				[]string{"source", "path", "content_hash"},
			),
		},
		{
			Name:        "file_registry_get",
			Description: "Get details about a file by ID or by source+path. Returns all indexed metadata including tags.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"id":     stringProperty("File ID (UUID from search results or previous operations)"),
					"source": stringProperty("Source identifier (required if using path instead of id)"),
					"path":   stringProperty("Full path within source (required if using source instead of id)"),
				},
				nil,
			),
		},
		{
			Name:        "file_registry_search",
			Description: "Search for files using full-text search across filenames, paths, and content. Supports filtering by source, category, tags, and type. Returns paginated results.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"query":      stringProperty("Full-text search query (searches path, filename, content)"),
					"sources":    arrayProperty("Filter by sources (e.g., ['local', 'gdrive'])", "string"),
					"categories": arrayProperty("Filter by categories (e.g., ['document', 'image'])", "string"),
					"tags":       arrayProperty("Filter by tags/labels (files must have ALL these tags)", "string"),
					"any_tags":   arrayProperty("Filter by tags/labels (files must have ANY of these tags)", "string"),
					"status":     stringProperty("Filter by status: 'active', 'missing', 'deleted'"),
					"limit":      intProperty("Max results to return", 20),
					"cursor":     stringProperty("Pagination cursor from previous search results"),
					"order":      stringProperty("Sort direction: 'asc' or 'desc' (default: desc by updated_at)"),
				},
				nil,
			),
		},
		{
			Name:        "file_registry_semantic_search",
			Description: "Search for files using semantic similarity. Combines full-text and vector search (hybrid) to find files with similar meaning to the query, even if they don't contain the exact words. Emergent handles embedding generation automatically.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"query":          stringProperty("Natural language query describing what you're looking for"),
					"limit":          intProperty("Maximum number of results to return", 10),
					"sources":        arrayProperty("Filter by sources", "string"),
					"categories":     arrayProperty("Filter by categories", "string"),
					"tags":           arrayProperty("Filter by tags/labels", "string"),
					"lexical_weight": numberProperty("Weight for full-text component (0.0-1.0)"),
					"vector_weight":  numberProperty("Weight for vector/semantic component (0.0-1.0)"),
				},
				[]string{"query"},
			),
		},
		{
			Name:        "file_registry_tag",
			Description: "Add one or more tags/labels to a file.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"id":     stringProperty("File ID (UUID)"),
					"source": stringProperty("Source identifier (alternative to id)"),
					"path":   stringProperty("Path within source (alternative to id)"),
					"tags":   arrayProperty("Tags to add", "string"),
				},
				[]string{"tags"},
			),
		},
		{
			Name:        "file_registry_untag",
			Description: "Remove one or more tags/labels from a file.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"id":     stringProperty("File ID (UUID)"),
					"source": stringProperty("Source identifier (alternative to id)"),
					"path":   stringProperty("Path within source (alternative to id)"),
					"tags":   arrayProperty("Tags to remove", "string"),
				},
				[]string{"tags"},
			),
		},
		{
			Name:        "file_registry_tags",
			Description: "List all tags/labels in the file index with usage information.",
			InputSchema: objectSchema(
				map[string]interface{}{},
				nil,
			),
		},
		{
			Name:        "file_registry_duplicates",
			Description: "Find duplicate files based on content hash. Groups files with identical content across all sources.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"content_hash": stringProperty("Find duplicates of a specific content hash"),
					"limit":        intProperty("Max results to return", 20),
				},
				nil,
			),
		},
		{
			Name:        "file_registry_remove",
			Description: "Remove a file from the index (soft-delete). This only removes the file from the index, not from the actual source.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"id":     stringProperty("File ID (UUID)"),
					"source": stringProperty("Source identifier (alternative to id)"),
					"path":   stringProperty("Path within source (alternative to id)"),
				},
				nil,
			),
		},
		{
			Name:        "file_registry_verify",
			Description: "Mark a file as verified (still exists at source). Updates the verified_at timestamp.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"id":     stringProperty("File ID (UUID)"),
					"source": stringProperty("Source identifier (alternative to id)"),
					"path":   stringProperty("Path within source (alternative to id)"),
				},
				nil,
			),
		},
		{
			Name:        "file_registry_stats",
			Description: "Get statistics about the file index: total files by category, by source, etc.",
			InputSchema: objectSchema(
				map[string]interface{}{},
				nil,
			),
		},
		{
			Name:        "file_registry_recent",
			Description: "Get recently indexed or accessed files.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"limit": intProperty("Maximum number of items to return", 20),
				},
				nil,
			),
		},
		{
			Name:        "file_registry_similar",
			Description: "Find files similar to a given file using vector similarity.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"id":     stringProperty("File ID (UUID) to find similar files for"),
					"source": stringProperty("Source identifier (alternative to id)"),
					"path":   stringProperty("Path within source (alternative to id)"),
					"limit":  intProperty("Maximum number of results", 10),
				},
				nil,
			),
		},
		// --- Batch operations ---
		{
			Name:        "file_registry_batch_register",
			Description: "Register multiple files in one call. Much faster than individual file_registry_register calls. Each file needs source and path at minimum. Processes up to 50 files per call with 10 concurrent workers.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"files": map[string]interface{}{
						"type":        "array",
						"description": "Array of file objects to register. Each object has the same fields as file_registry_register (source, path, filename, size, mime_type, etc.)",
						"items": objectSchema(
							map[string]interface{}{
								"source":          stringProperty("Source identifier: 'local', 'gdrive', 'gmail', 'dropbox', etc."),
								"path":            stringProperty("Full path within the source"),
								"source_file_id":  stringProperty("Provider-specific file ID"),
								"filename":        stringProperty("Base filename (extracted from path if not provided)"),
								"size":            intProperty("File size in bytes", 0),
								"mime_type":       stringProperty("MIME type"),
								"is_directory":    boolProperty("True if this is a directory"),
								"created_at":      stringProperty("Creation timestamp (ISO 8601)"),
								"modified_at":     stringProperty("Modification timestamp (ISO 8601)"),
								"content_hash":    stringProperty("SHA-256 hash of file content (required for duplicate detection)"),
								"partial_hash":    stringProperty("SHA-256 hash of first 64KB"),
								"content_text":    stringProperty("Extracted text content"),
								"content_preview": stringProperty("First ~500 chars preview"),
								"category":        stringProperty("File category: document, image, video, audio, code, archive, data, other"),
								"subcategory":     stringProperty("Subcategory: invoice, receipt, photo, screenshot, etc."),
								"tags":            arrayProperty("Tags/labels", "string"),
							},
							[]string{"source", "path", "content_hash"},
						),
					},
				},
				[]string{"files"},
			),
		},
		{
			Name:        "file_registry_batch_get",
			Description: "Get details for multiple files in one call. Accepts a list of IDs or source+path pairs. Processes up to 100 files with 10 concurrent workers.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"ids": arrayProperty("File IDs (UUIDs) to retrieve", "string"),
					"keys": map[string]interface{}{
						"type":        "array",
						"description": "Array of {source, path} objects to look up (alternative to ids)",
						"items": objectSchema(
							map[string]interface{}{
								"source": stringProperty("Source identifier"),
								"path":   stringProperty("Path within source"),
							},
							[]string{"source", "path"},
						),
					},
				},
				nil,
			),
		},
		{
			Name:        "file_registry_batch_tag",
			Description: "Add tags to multiple files in one call. All specified files get the same set of tags added. Processes with 10 concurrent workers.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"ids":  arrayProperty("File IDs (UUIDs) to tag", "string"),
					"tags": arrayProperty("Tags to add to all specified files", "string"),
				},
				[]string{"ids", "tags"},
			),
		},
		{
			Name:        "file_registry_batch_untag",
			Description: "Remove tags from multiple files in one call. All specified files get the same set of tags removed. Processes with 10 concurrent workers.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"ids":  arrayProperty("File IDs (UUIDs) to untag", "string"),
					"tags": arrayProperty("Tags to remove from all specified files", "string"),
				},
				[]string{"ids", "tags"},
			),
		},
		{
			Name:        "file_registry_batch_remove",
			Description: "Remove multiple files from the index in one call (soft-delete). Only removes from the index, not from actual sources. Processes with 10 concurrent workers.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"ids": arrayProperty("File IDs (UUIDs) to remove", "string"),
				},
				[]string{"ids"},
			),
		},
		{
			Name:        "file_registry_crawl",
			Description: "Crawl a local directory and register all discovered files. Computes SHA-256 hashes, detects MIME types, and batch-registers files into the index. Skips files already registered with the same source:path key. Use this instead of manually running find/stat/sha256sum and calling batch_register.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"path":            stringProperty("Absolute path to the directory to crawl (e.g. /Users/me/Downloads)"),
					"pattern":         stringProperty("Regexp to match filenames to include (e.g. '\\.(pdf|docx|xlsx)$'). If omitted, all files are included."),
					"exclude_pattern": stringProperty("Regexp to match filenames to exclude (e.g. '\\.(DS_Store|tmp)$')"),
					"max_depth":       intProperty("Maximum directory depth to descend (0 = target dir only, -1 or omitted = unlimited)", -1),
					"include_hidden":  boolProperty("Include hidden files and directories (names starting with '.'). Default: false"),
					"dry_run":         boolProperty("If true, only report what would be registered without actually registering. Default: false"),
					"tags":            arrayProperty("Tags to apply to all crawled files", "string"),
				},
				[]string{"path"},
			),
		},
	}
}

// HasTool checks if a tool name belongs to this provider
func (p *Provider) HasTool(name string) bool {
	switch name {
	case "file_registry_register", "file_registry_get", "file_registry_search", "file_registry_semantic_search",
		"file_registry_tag", "file_registry_untag", "file_registry_tags", "file_registry_duplicates",
		"file_registry_remove", "file_registry_verify", "file_registry_stats", "file_registry_recent", "file_registry_similar",
		"file_registry_batch_register", "file_registry_batch_get", "file_registry_batch_tag",
		"file_registry_batch_untag", "file_registry_batch_remove",
		"file_registry_crawl":
		return true
	}
	return false
}

// Call executes a file management tool
func (p *Provider) Call(name string, args map[string]interface{}) (interface{}, error) {
	switch name {
	case "file_registry_register":
		return p.register(args)
	case "file_registry_get":
		return p.get(args)
	case "file_registry_search":
		return p.search(args)
	case "file_registry_semantic_search":
		return p.semanticSearch(args)
	case "file_registry_tag":
		return p.tag(args)
	case "file_registry_untag":
		return p.untag(args)
	case "file_registry_tags":
		return p.tags(args)
	case "file_registry_duplicates":
		return p.duplicates(args)
	case "file_registry_remove":
		return p.remove(args)
	case "file_registry_verify":
		return p.verify(args)
	case "file_registry_stats":
		return p.stats(args)
	case "file_registry_recent":
		return p.recent(args)
	case "file_registry_similar":
		return p.similar(args)
	case "file_registry_batch_register":
		return p.batchRegister(args)
	case "file_registry_batch_get":
		return p.batchGet(args)
	case "file_registry_batch_tag":
		return p.batchTag(args)
	case "file_registry_batch_untag":
		return p.batchUntag(args)
	case "file_registry_batch_remove":
		return p.batchRemove(args)
	case "file_registry_crawl":
		return p.crawl(args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// --- Arg Helpers ---

func getString(args map[string]interface{}, key string) string {
	if val, ok := args[key].(string); ok {
		return val
	}
	return ""
}

func getInt(args map[string]interface{}, key string, defaultVal int) int {
	if val, ok := args[key].(float64); ok {
		return int(val)
	}
	return defaultVal
}

func getFloat32(args map[string]interface{}, key string) *float32 {
	if val, ok := args[key].(float64); ok {
		f := float32(val)
		return &f
	}
	return nil
}

func getInt64(args map[string]interface{}, key string, defaultVal int64) int64 {
	if val, ok := args[key].(float64); ok {
		return int64(val)
	}
	return defaultVal
}

func getBool(args map[string]interface{}, key string) *bool {
	if val, ok := args[key].(bool); ok {
		return &val
	}
	return nil
}

func getStringArray(args map[string]interface{}, key string) []string {
	if val, ok := args[key].([]interface{}); ok {
		result := make([]string, 0, len(val))
		for _, v := range val {
			if s, ok := v.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

// --- Response Helpers ---

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

func strPtr(s string) *string {
	return &s
}

// graphObjectToMap converts a typed GraphObject to a display-friendly map.
func graphObjectToMap(obj *graph.GraphObject) map[string]interface{} {
	m := map[string]interface{}{
		"id":         obj.ID,
		"type":       obj.Type,
		"properties": obj.Properties,
		"labels":     obj.Labels,
		"version":    obj.Version,
		"created_at": obj.CreatedAt,
	}
	if obj.Key != nil {
		m["key"] = *obj.Key
	}
	if obj.Status != nil {
		m["status"] = *obj.Status
	}
	if obj.ExternalSource != nil {
		m["external_source"] = *obj.ExternalSource
	}
	if obj.ExternalID != nil {
		m["external_id"] = *obj.ExternalID
	}
	if obj.ExternalURL != nil {
		m["external_url"] = *obj.ExternalURL
	}
	if obj.DeletedAt != nil {
		m["deleted_at"] = *obj.DeletedAt
	}
	return m
}

// extractFilename extracts the base filename from a path
func extractFilename(path string) string {
	parts := strings.Split(path, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			return parts[i]
		}
	}
	return path
}

// extractExtension extracts the extension from a filename (lowercase, no dot)
func extractExtension(filename string) string {
	idx := strings.LastIndex(filename, ".")
	if idx == -1 || idx == len(filename)-1 {
		return ""
	}
	return strings.ToLower(filename[idx+1:])
}

// --- File Resolution ---

// resolveFileID resolves a file identifier from either a direct ID or source+path key lookup.
// Returns the object ID and the full GraphObject.
func (p *Provider) resolveFileID(ctx context.Context, id, source, path string) (string, *graph.GraphObject, error) {
	if id != "" {
		obj, err := p.client.Graph.GetObject(ctx, id)
		if err != nil {
			return "", nil, fmt.Errorf("file not found: %w", err)
		}
		return obj.ID, obj, nil
	}

	// Lookup by key (source:path)
	key := fmt.Sprintf("%s:%s", source, path)
	resp, err := p.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  "file",
		Key:   key,
		Limit: 1,
	})
	if err != nil {
		return "", nil, fmt.Errorf("file lookup failed: %w", err)
	}
	if len(resp.Items) == 0 {
		return "", nil, fmt.Errorf("file not found with key %s", key)
	}

	obj := resp.Items[0]
	return obj.ID, obj, nil
}

// --- Tool Implementations ---

func (p *Provider) register(args map[string]interface{}) (interface{}, error) {
	source := getString(args, "source")
	path := getString(args, "path")
	contentHash := getString(args, "content_hash")
	if source == "" {
		return nil, fmt.Errorf("source is required")
	}
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}
	if contentHash == "" {
		return nil, fmt.Errorf("content_hash is required")
	}

	filename := getString(args, "filename")
	if filename == "" {
		filename = extractFilename(path)
	}
	extension := extractExtension(filename)

	// Build properties from file metadata
	properties := map[string]interface{}{
		"source":       source,
		"path":         path,
		"filename":     filename,
		"extension":    extension,
		"content_hash": contentHash,
	}
	if v := getString(args, "source_file_id"); v != "" {
		properties["source_file_id"] = v
	}
	if v := getInt64(args, "size", 0); v > 0 {
		properties["size"] = v
	}
	if v := getString(args, "mime_type"); v != "" {
		properties["mime_type"] = v
	}
	if b := getBool(args, "is_directory"); b != nil {
		properties["is_directory"] = *b
	}
	if v := getString(args, "created_at"); v != "" {
		properties["created_at"] = v
	}
	if v := getString(args, "modified_at"); v != "" {
		properties["modified_at"] = v
	}
	if v := getString(args, "partial_hash"); v != "" {
		properties["partial_hash"] = v
	}
	if v := getString(args, "content_text"); v != "" {
		properties["content_text"] = v
	}
	if v := getString(args, "content_preview"); v != "" {
		properties["content_preview"] = v
	}
	if v := getString(args, "category"); v != "" {
		properties["category"] = v
	}
	if v := getString(args, "subcategory"); v != "" {
		properties["subcategory"] = v
	}

	key := fmt.Sprintf("%s:%s", source, path)
	tags := getStringArray(args, "tags")

	ctx := context.Background()
	obj, err := p.client.Graph.CreateObject(ctx, &graph.CreateObjectRequest{
		Type:       "file",
		Key:        &key,
		Status:     strPtr("active"),
		Properties: properties,
		Labels:     tags,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to register file: %w", err)
	}

	slog.Info("File registered", "key", key, "id", obj.ID)
	return textContent(map[string]interface{}{
		"status":  "registered",
		"id":      obj.ID,
		"key":     key,
		"message": fmt.Sprintf("File registered: %s", filename),
	}), nil
}

func (p *Provider) get(args map[string]interface{}) (interface{}, error) {
	id := getString(args, "id")
	source := getString(args, "source")
	path := getString(args, "path")

	if id == "" && (source == "" || path == "") {
		return nil, fmt.Errorf("either 'id' or both 'source' and 'path' are required")
	}

	ctx := context.Background()
	_, obj, err := p.resolveFileID(ctx, id, source, path)
	if err != nil {
		return nil, err
	}

	return textContent(graphObjectToMap(obj)), nil
}

func (p *Provider) search(args map[string]interface{}) (interface{}, error) {
	query := getString(args, "query")
	sources := getStringArray(args, "sources")
	categories := getStringArray(args, "categories")
	tags := getStringArray(args, "tags")
	status := getString(args, "status")
	limit := getInt(args, "limit", 20)
	cursor := getString(args, "cursor")
	order := getString(args, "order")

	ctx := context.Background()

	if query != "" {
		// Full-text search via FTS
		ftsOpts := &graph.FTSSearchOptions{
			Query:  query,
			Types:  []string{"file"},
			Labels: tags,
			Status: status,
			Limit:  limit,
		}
		ftsResp, err := p.client.Graph.FTSSearch(ctx, ftsOpts)
		if err != nil {
			return nil, fmt.Errorf("search failed: %w", err)
		}

		results := searchResultsToMaps(ftsResp.Data, sources, categories, tags)
		return textContent(map[string]interface{}{
			"results": results,
			"total":   len(results),
		}), nil
	}

	// No query — browse/filter mode
	listOpts := &graph.ListObjectsOptions{
		Type:   "file",
		Labels: tags,
		Status: status,
		Limit:  limit,
		Cursor: cursor,
		Order:  order,
	}
	resp, err := p.client.Graph.ListObjects(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	results := filterObjectResults(resp.Items, sources, categories, tags)
	var nextCursor *string
	if resp.NextCursor != nil {
		nextCursor = resp.NextCursor
	}

	response := map[string]interface{}{
		"results": results,
		"total":   len(results),
	}
	if nextCursor != nil {
		response["next_cursor"] = *nextCursor
	}
	return textContent(response), nil
}

// searchResultsToMaps converts search results to display maps with optional source/category/tag filtering.
func searchResultsToMaps(items []*graph.SearchResultItem, sources, categories, tags []string) []map[string]interface{} {
	sourceSet := toSet(sources)
	catSet := toSet(categories)
	tagSet := toSet(tags)

	var results []map[string]interface{}
	for _, item := range items {
		if item.Object == nil {
			continue
		}
		if !matchesFilters(item.Object.Properties, sourceSet, catSet) {
			continue
		}
		if !matchesTagFilter(item.Object.Labels, tagSet) {
			continue
		}
		m := graphObjectToMap(item.Object)
		m["score"] = item.Score
		results = append(results, m)
	}
	return results
}

// filterObjectResults filters GraphObjects by source, category, and tag properties.
func filterObjectResults(items []*graph.GraphObject, sources, categories, tags []string) []map[string]interface{} {
	sourceSet := toSet(sources)
	catSet := toSet(categories)
	tagSet := toSet(tags)

	var results []map[string]interface{}
	for _, obj := range items {
		if !matchesFilters(obj.Properties, sourceSet, catSet) {
			continue
		}
		if !matchesTagFilter(obj.Labels, tagSet) {
			continue
		}
		results = append(results, graphObjectToMap(obj))
	}
	return results
}

func toSet(items []string) map[string]bool {
	if len(items) == 0 {
		return nil
	}
	s := make(map[string]bool, len(items))
	for _, item := range items {
		s[item] = true
	}
	return s
}

func matchesFilters(props map[string]interface{}, sourceSet, catSet map[string]bool) bool {
	if len(sourceSet) > 0 {
		src, _ := props["source"].(string)
		if !sourceSet[src] {
			return false
		}
	}
	if len(catSet) > 0 {
		cat, _ := props["category"].(string)
		if !catSet[cat] {
			return false
		}
	}
	return true
}

// matchesTagFilter checks that the object has ALL required tags.
func matchesTagFilter(labels []string, tagSet map[string]bool) bool {
	if len(tagSet) == 0 {
		return true
	}
	labelSet := toSet(labels)
	for tag := range tagSet {
		if !labelSet[tag] {
			return false
		}
	}
	return true
}

func (p *Provider) semanticSearch(args map[string]interface{}) (interface{}, error) {
	query := getString(args, "query")
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	limit := getInt(args, "limit", 10)
	tags := getStringArray(args, "tags")
	sources := getStringArray(args, "sources")
	categories := getStringArray(args, "categories")

	ctx := context.Background()

	resp, err := p.client.Graph.HybridSearch(ctx, &graph.HybridSearchRequest{
		Query:         query,
		Types:         []string{"file"},
		Labels:        tags,
		LexicalWeight: getFloat32(args, "lexical_weight"),
		VectorWeight:  getFloat32(args, "vector_weight"),
		Limit:         limit,
	})
	if err != nil {
		return nil, fmt.Errorf("semantic search failed: %w", err)
	}

	results := searchResultsToMaps(resp.Data, sources, categories, tags)
	return textContent(map[string]interface{}{
		"results": results,
		"total":   len(results),
	}), nil
}

func (p *Provider) tag(args map[string]interface{}) (interface{}, error) {
	tags := getStringArray(args, "tags")
	if len(tags) == 0 {
		return nil, fmt.Errorf("tags array is required")
	}

	id := getString(args, "id")
	source := getString(args, "source")
	path := getString(args, "path")

	if id == "" && (source == "" || path == "") {
		return nil, fmt.Errorf("either 'id' or both 'source' and 'path' are required")
	}

	ctx := context.Background()
	fileID, _, err := p.resolveFileID(ctx, id, source, path)
	if err != nil {
		return nil, err
	}

	// Additive label update (ReplaceLabels defaults to false)
	_, err = p.client.Graph.UpdateObject(ctx, fileID, &graph.UpdateObjectRequest{
		Labels: tags,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to add tags: %w", err)
	}

	return textContent(map[string]interface{}{
		"status":  "tagged",
		"id":      fileID,
		"added":   tags,
		"message": fmt.Sprintf("Added %d tag(s) to file", len(tags)),
	}), nil
}

func (p *Provider) untag(args map[string]interface{}) (interface{}, error) {
	tagsToRemove := getStringArray(args, "tags")
	if len(tagsToRemove) == 0 {
		return nil, fmt.Errorf("tags array is required")
	}

	id := getString(args, "id")
	source := getString(args, "source")
	path := getString(args, "path")

	if id == "" && (source == "" || path == "") {
		return nil, fmt.Errorf("either 'id' or both 'source' and 'path' are required")
	}

	ctx := context.Background()
	fileID, obj, err := p.resolveFileID(ctx, id, source, path)
	if err != nil {
		return nil, err
	}

	// Compute remaining labels after removing the specified ones
	removeSet := toSet(tagsToRemove)
	var remaining []string
	for _, l := range obj.Labels {
		if !removeSet[l] {
			remaining = append(remaining, l)
		}
	}

	// Replace all labels with the filtered set
	_, err = p.client.Graph.UpdateObject(ctx, fileID, &graph.UpdateObjectRequest{
		Labels:        remaining,
		ReplaceLabels: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to remove tags: %w", err)
	}

	return textContent(map[string]interface{}{
		"status":  "untagged",
		"id":      fileID,
		"removed": tagsToRemove,
		"message": fmt.Sprintf("Removed %d tag(s) from file", len(tagsToRemove)),
	}), nil
}

func (p *Provider) tags(args map[string]interface{}) (interface{}, error) {
	ctx := context.Background()

	tagList, err := p.client.Graph.ListTags(ctx, &graph.ListTagsOptions{
		Type: "file",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	return textContent(map[string]interface{}{
		"tags":  tagList,
		"total": len(tagList),
	}), nil
}

func (p *Provider) duplicates(args map[string]interface{}) (interface{}, error) {
	contentHash := getString(args, "content_hash")
	limit := getInt(args, "limit", 20)

	ctx := context.Background()

	// Fetch file objects and group by content_hash
	resp, err := p.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  "file",
		Limit: 500,
	})
	if err != nil {
		return nil, fmt.Errorf("duplicate search failed: %w", err)
	}

	if contentHash != "" {
		// Find files with this specific content hash
		var matches []map[string]interface{}
		for _, obj := range resp.Items {
			hash, _ := obj.Properties["content_hash"].(string)
			if hash == contentHash {
				matches = append(matches, graphObjectToMap(obj))
			}
		}
		return textContent(map[string]interface{}{
			"content_hash": contentHash,
			"duplicates":   matches,
			"count":        len(matches),
		}), nil
	}

	// No specific hash — find all duplicate groups
	groups := make(map[string][]*graph.GraphObject)
	for _, obj := range resp.Items {
		hash, _ := obj.Properties["content_hash"].(string)
		if hash == "" {
			continue
		}
		groups[hash] = append(groups[hash], obj)
	}

	var duplicateGroups []map[string]interface{}
	for hash, files := range groups {
		if len(files) > 1 {
			fileMaps := make([]map[string]interface{}, len(files))
			for i, f := range files {
				fileMaps[i] = graphObjectToMap(f)
			}
			duplicateGroups = append(duplicateGroups, map[string]interface{}{
				"content_hash": hash,
				"files":        fileMaps,
				"count":        len(files),
			})
		}
		if len(duplicateGroups) >= limit {
			break
		}
	}

	return textContent(map[string]interface{}{
		"duplicate_groups": duplicateGroups,
		"total_groups":     len(duplicateGroups),
	}), nil
}

func (p *Provider) remove(args map[string]interface{}) (interface{}, error) {
	id := getString(args, "id")
	source := getString(args, "source")
	path := getString(args, "path")

	if id == "" && (source == "" || path == "") {
		return nil, fmt.Errorf("either 'id' or both 'source' and 'path' are required")
	}

	ctx := context.Background()
	fileID, _, err := p.resolveFileID(ctx, id, source, path)
	if err != nil {
		return nil, err
	}

	if err := p.client.Graph.DeleteObject(ctx, fileID); err != nil {
		return nil, fmt.Errorf("failed to remove file: %w", err)
	}

	slog.Info("File removed from index", "id", fileID)
	return textContent(map[string]interface{}{
		"status":  "removed",
		"id":      fileID,
		"message": "File removed from index (soft-deleted)",
	}), nil
}

func (p *Provider) verify(args map[string]interface{}) (interface{}, error) {
	id := getString(args, "id")
	source := getString(args, "source")
	path := getString(args, "path")

	if id == "" && (source == "" || path == "") {
		return nil, fmt.Errorf("either 'id' or both 'source' and 'path' are required")
	}

	ctx := context.Background()
	fileID, _, err := p.resolveFileID(ctx, id, source, path)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = p.client.Graph.UpdateObject(ctx, fileID, &graph.UpdateObjectRequest{
		Properties: map[string]interface{}{
			"verified_at": now,
		},
		Status: strPtr("active"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to verify file: %w", err)
	}

	return textContent(map[string]interface{}{
		"status":      "verified",
		"id":          fileID,
		"verified_at": now,
		"message":     "File verified as still existing at source",
	}), nil
}

func (p *Provider) stats(args map[string]interface{}) (interface{}, error) {
	ctx := context.Background()

	resp, err := p.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  "file",
		Limit: 1000,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	total := len(resp.Items)
	bySource := make(map[string]int)
	byCategory := make(map[string]int)
	byStatus := make(map[string]int)
	byExtension := make(map[string]int)

	for _, obj := range resp.Items {
		if s, _ := obj.Properties["source"].(string); s != "" {
			bySource[s]++
		}
		if c, _ := obj.Properties["category"].(string); c != "" {
			byCategory[c]++
		}
		if obj.Status != nil {
			byStatus[*obj.Status]++
		}
		if e, _ := obj.Properties["extension"].(string); e != "" {
			byExtension[e]++
		}
	}

	return textContent(map[string]interface{}{
		"total":        total,
		"by_source":    bySource,
		"by_category":  byCategory,
		"by_status":    byStatus,
		"by_extension": byExtension,
	}), nil
}

func (p *Provider) recent(args map[string]interface{}) (interface{}, error) {
	limit := getInt(args, "limit", 20)

	ctx := context.Background()

	resp, err := p.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  "file",
		Limit: limit,
		Order: "desc",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get recent files: %w", err)
	}

	results := make([]map[string]interface{}, len(resp.Items))
	for i, obj := range resp.Items {
		results[i] = graphObjectToMap(obj)
	}

	return textContent(map[string]interface{}{
		"files": results,
		"total": len(results),
	}), nil
}

func (p *Provider) similar(args map[string]interface{}) (interface{}, error) {
	id := getString(args, "id")
	source := getString(args, "source")
	path := getString(args, "path")

	if id == "" && (source == "" || path == "") {
		return nil, fmt.Errorf("either 'id' or both 'source' and 'path' are required")
	}

	limit := getInt(args, "limit", 10)

	ctx := context.Background()
	fileID, _, err := p.resolveFileID(ctx, id, source, path)
	if err != nil {
		return nil, err
	}

	results, err := p.client.Graph.FindSimilar(ctx, fileID, &graph.FindSimilarOptions{
		Limit: limit,
		Type:  "file",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find similar files: %w", err)
	}

	return textContent(map[string]interface{}{
		"source_id": fileID,
		"similar":   results,
		"total":     len(results),
	}), nil
}

// --- Batch Operations ---

const batchWorkers = 10

// batchResult tracks the outcome of a single item in a batch operation.
type batchResult struct {
	Index   int                    `json:"index"`
	ID      string                 `json:"id,omitempty"`
	Key     string                 `json:"key,omitempty"`
	Status  string                 `json:"status"`
	Error   string                 `json:"error,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// getObjectArray extracts an array of map objects from args.
func getObjectArray(args map[string]interface{}, key string) []map[string]interface{} {
	val, ok := args[key].([]interface{})
	if !ok {
		return nil
	}
	result := make([]map[string]interface{}, 0, len(val))
	for _, v := range val {
		if m, ok := v.(map[string]interface{}); ok {
			result = append(result, m)
		}
	}
	return result
}

func (p *Provider) batchRegister(args map[string]interface{}) (interface{}, error) {
	files := getObjectArray(args, "files")
	if len(files) == 0 {
		return nil, fmt.Errorf("files array is required and must not be empty")
	}
	if len(files) > 50 {
		return nil, fmt.Errorf("maximum 50 files per batch call, got %d", len(files))
	}

	ctx := context.Background()
	results := make([]batchResult, len(files))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, batchWorkers)

	for i, fileArgs := range files {
		wg.Add(1)
		go func(idx int, fa map[string]interface{}) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			source := getString(fa, "source")
			path := getString(fa, "path")
			contentHash := getString(fa, "content_hash")

			br := batchResult{Index: idx}

			if source == "" || path == "" {
				br.Status = "error"
				br.Error = "source and path are required"
				mu.Lock()
				results[idx] = br
				mu.Unlock()
				return
			}
			if contentHash == "" {
				br.Status = "error"
				br.Error = "content_hash is required"
				mu.Lock()
				results[idx] = br
				mu.Unlock()
				return
			}

			filename := getString(fa, "filename")
			if filename == "" {
				filename = extractFilename(path)
			}
			extension := extractExtension(filename)

			properties := map[string]interface{}{
				"source":       source,
				"path":         path,
				"filename":     filename,
				"extension":    extension,
				"content_hash": contentHash,
			}
			if v := getString(fa, "source_file_id"); v != "" {
				properties["source_file_id"] = v
			}
			if v := getInt64(fa, "size", 0); v > 0 {
				properties["size"] = v
			}
			if v := getString(fa, "mime_type"); v != "" {
				properties["mime_type"] = v
			}
			if b := getBool(fa, "is_directory"); b != nil {
				properties["is_directory"] = *b
			}
			if v := getString(fa, "created_at"); v != "" {
				properties["created_at"] = v
			}
			if v := getString(fa, "modified_at"); v != "" {
				properties["modified_at"] = v
			}
			if v := getString(fa, "partial_hash"); v != "" {
				properties["partial_hash"] = v
			}
			if v := getString(fa, "content_text"); v != "" {
				properties["content_text"] = v
			}
			if v := getString(fa, "content_preview"); v != "" {
				properties["content_preview"] = v
			}
			if v := getString(fa, "category"); v != "" {
				properties["category"] = v
			}
			if v := getString(fa, "subcategory"); v != "" {
				properties["subcategory"] = v
			}

			key := fmt.Sprintf("%s:%s", source, path)
			tags := getStringArray(fa, "tags")

			obj, err := p.client.Graph.CreateObject(ctx, &graph.CreateObjectRequest{
				Type:       "file",
				Key:        &key,
				Status:     strPtr("active"),
				Properties: properties,
				Labels:     tags,
			})
			if err != nil {
				br.Status = "error"
				br.Error = err.Error()
				br.Key = key
			} else {
				br.Status = "registered"
				br.ID = obj.ID
				br.Key = key
			}

			mu.Lock()
			results[idx] = br
			mu.Unlock()
		}(i, fileArgs)
	}

	wg.Wait()

	succeeded := 0
	failed := 0
	for _, r := range results {
		if r.Status == "registered" {
			succeeded++
		} else {
			failed++
		}
	}

	slog.Info("Batch register completed", "succeeded", succeeded, "failed", failed, "total", len(files))
	return textContent(map[string]interface{}{
		"status":    "completed",
		"total":     len(files),
		"succeeded": succeeded,
		"failed":    failed,
		"results":   results,
	}), nil
}

func (p *Provider) batchGet(args map[string]interface{}) (interface{}, error) {
	ids := getStringArray(args, "ids")
	keys := getObjectArray(args, "keys")

	if len(ids) == 0 && len(keys) == 0 {
		return nil, fmt.Errorf("either 'ids' or 'keys' is required")
	}

	totalItems := len(ids) + len(keys)
	if totalItems > 100 {
		return nil, fmt.Errorf("maximum 100 files per batch call, got %d", totalItems)
	}

	ctx := context.Background()
	results := make([]batchResult, totalItems)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, batchWorkers)

	// Process IDs
	for i, id := range ids {
		wg.Add(1)
		go func(idx int, fileID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			br := batchResult{Index: idx, ID: fileID}

			obj, err := p.client.Graph.GetObject(ctx, fileID)
			if err != nil {
				br.Status = "error"
				br.Error = fmt.Sprintf("file not found: %v", err)
			} else {
				br.Status = "found"
				br.Details = graphObjectToMap(obj)
			}

			mu.Lock()
			results[idx] = br
			mu.Unlock()
		}(i, id)
	}

	// Process source+path keys
	offset := len(ids)
	for i, keyObj := range keys {
		wg.Add(1)
		go func(idx int, source, path string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			key := fmt.Sprintf("%s:%s", source, path)
			br := batchResult{Index: idx, Key: key}

			_, obj, err := p.resolveFileID(ctx, "", source, path)
			if err != nil {
				br.Status = "error"
				br.Error = err.Error()
			} else {
				br.Status = "found"
				br.ID = obj.ID
				br.Details = graphObjectToMap(obj)
			}

			mu.Lock()
			results[idx] = br
			mu.Unlock()
		}(offset+i, getString(keyObj, "source"), getString(keyObj, "path"))
	}

	wg.Wait()

	found := 0
	notFound := 0
	for _, r := range results {
		if r.Status == "found" {
			found++
		} else {
			notFound++
		}
	}

	return textContent(map[string]interface{}{
		"total":     totalItems,
		"found":     found,
		"not_found": notFound,
		"results":   results,
	}), nil
}

func (p *Provider) batchTag(args map[string]interface{}) (interface{}, error) {
	ids := getStringArray(args, "ids")
	tags := getStringArray(args, "tags")

	if len(ids) == 0 {
		return nil, fmt.Errorf("ids array is required")
	}
	if len(tags) == 0 {
		return nil, fmt.Errorf("tags array is required")
	}

	ctx := context.Background()
	results := make([]batchResult, len(ids))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, batchWorkers)

	for i, id := range ids {
		wg.Add(1)
		go func(idx int, fileID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			br := batchResult{Index: idx, ID: fileID}

			_, err := p.client.Graph.UpdateObject(ctx, fileID, &graph.UpdateObjectRequest{
				Labels: tags,
			})
			if err != nil {
				br.Status = "error"
				br.Error = err.Error()
			} else {
				br.Status = "tagged"
			}

			mu.Lock()
			results[idx] = br
			mu.Unlock()
		}(i, id)
	}

	wg.Wait()

	succeeded := 0
	failed := 0
	for _, r := range results {
		if r.Status == "tagged" {
			succeeded++
		} else {
			failed++
		}
	}

	return textContent(map[string]interface{}{
		"status":    "completed",
		"total":     len(ids),
		"succeeded": succeeded,
		"failed":    failed,
		"tags":      tags,
		"results":   results,
	}), nil
}

func (p *Provider) batchUntag(args map[string]interface{}) (interface{}, error) {
	ids := getStringArray(args, "ids")
	tagsToRemove := getStringArray(args, "tags")

	if len(ids) == 0 {
		return nil, fmt.Errorf("ids array is required")
	}
	if len(tagsToRemove) == 0 {
		return nil, fmt.Errorf("tags array is required")
	}

	removeSet := toSet(tagsToRemove)

	ctx := context.Background()
	results := make([]batchResult, len(ids))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, batchWorkers)

	for i, id := range ids {
		wg.Add(1)
		go func(idx int, fileID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			br := batchResult{Index: idx, ID: fileID}

			// Get current labels
			obj, err := p.client.Graph.GetObject(ctx, fileID)
			if err != nil {
				br.Status = "error"
				br.Error = fmt.Sprintf("file not found: %v", err)
				mu.Lock()
				results[idx] = br
				mu.Unlock()
				return
			}

			// Filter out the tags to remove
			var remaining []string
			for _, l := range obj.Labels {
				if !removeSet[l] {
					remaining = append(remaining, l)
				}
			}

			_, err = p.client.Graph.UpdateObject(ctx, fileID, &graph.UpdateObjectRequest{
				Labels:        remaining,
				ReplaceLabels: true,
			})
			if err != nil {
				br.Status = "error"
				br.Error = err.Error()
			} else {
				br.Status = "untagged"
			}

			mu.Lock()
			results[idx] = br
			mu.Unlock()
		}(i, id)
	}

	wg.Wait()

	succeeded := 0
	failed := 0
	for _, r := range results {
		if r.Status == "untagged" {
			succeeded++
		} else {
			failed++
		}
	}

	return textContent(map[string]interface{}{
		"status":    "completed",
		"total":     len(ids),
		"succeeded": succeeded,
		"failed":    failed,
		"removed":   tagsToRemove,
		"results":   results,
	}), nil
}

func (p *Provider) batchRemove(args map[string]interface{}) (interface{}, error) {
	ids := getStringArray(args, "ids")

	if len(ids) == 0 {
		return nil, fmt.Errorf("ids array is required")
	}

	ctx := context.Background()
	results := make([]batchResult, len(ids))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, batchWorkers)

	for i, id := range ids {
		wg.Add(1)
		go func(idx int, fileID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			br := batchResult{Index: idx, ID: fileID}

			if err := p.client.Graph.DeleteObject(ctx, fileID); err != nil {
				br.Status = "error"
				br.Error = err.Error()
			} else {
				br.Status = "removed"
			}

			mu.Lock()
			results[idx] = br
			mu.Unlock()
		}(i, id)
	}

	wg.Wait()

	succeeded := 0
	failed := 0
	for _, r := range results {
		if r.Status == "removed" {
			succeeded++
		} else {
			failed++
		}
	}

	slog.Info("Batch remove completed", "succeeded", succeeded, "failed", failed, "total", len(ids))
	return textContent(map[string]interface{}{
		"status":    "completed",
		"total":     len(ids),
		"succeeded": succeeded,
		"failed":    failed,
		"results":   results,
	}), nil
}

// --- Crawl Implementation ---

// crawlFile holds metadata for a single discovered file.
type crawlFile struct {
	Path     string
	Info     fs.FileInfo
	RelDepth int
}

// mimeFromExtension returns a MIME type for common extensions, falling back to
// Go's built-in mime.TypeByExtension.
func mimeFromExtension(ext string) string {
	// Common types that Go's mime package sometimes misses or gets wrong
	overrides := map[string]string{
		".pdf":  "application/pdf",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xls":  "application/vnd.ms-excel",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".ppt":  "application/vnd.ms-powerpoint",
		".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		".zip":  "application/zip",
		".gz":   "application/gzip",
		".tar":  "application/x-tar",
		".dmg":  "application/x-apple-diskimage",
		".heic": "image/heic",
		".heif": "image/heif",
		".webp": "image/webp",
		".avif": "image/avif",
		".svg":  "image/svg+xml",
		".mp4":  "video/mp4",
		".mov":  "video/quicktime",
		".mkv":  "video/x-matroska",
		".mp3":  "audio/mpeg",
		".flac": "audio/flac",
		".wav":  "audio/wav",
		".m4a":  "audio/mp4",
		".json": "application/json",
		".yaml": "application/x-yaml",
		".yml":  "application/x-yaml",
		".toml": "application/toml",
		".csv":  "text/csv",
		".tsv":  "text/tab-separated-values",
		".md":   "text/markdown",
		".xml":  "application/xml",
		".stl":  "model/stl",
	}
	if t, ok := overrides[strings.ToLower(ext)]; ok {
		return t
	}
	if t := mime.TypeByExtension(ext); t != "" {
		return t
	}
	return "application/octet-stream"
}

// categoryFromMIME returns a broad category based on MIME type.
func categoryFromMIME(mimeType string) string {
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return "image"
	case strings.HasPrefix(mimeType, "video/"):
		return "video"
	case strings.HasPrefix(mimeType, "audio/"):
		return "audio"
	case strings.HasPrefix(mimeType, "text/"):
		return "document"
	case mimeType == "application/pdf",
		strings.Contains(mimeType, "document"),
		strings.Contains(mimeType, "spreadsheet"),
		strings.Contains(mimeType, "presentation"),
		mimeType == "application/msword",
		mimeType == "application/vnd.ms-excel",
		mimeType == "application/vnd.ms-powerpoint":
		return "document"
	case mimeType == "application/zip",
		mimeType == "application/gzip",
		mimeType == "application/x-tar",
		mimeType == "application/x-apple-diskimage",
		mimeType == "application/x-7z-compressed",
		mimeType == "application/x-rar-compressed":
		return "archive"
	case mimeType == "application/json",
		mimeType == "application/xml",
		mimeType == "text/csv",
		mimeType == "application/x-yaml",
		mimeType == "application/toml":
		return "data"
	default:
		return "other"
	}
}

// hashFile computes SHA-256 of the full file and a partial hash of the first 64KB.
func hashFile(path string) (fullHash, partialHash string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	const partialSize = 64 * 1024

	partialHasher := sha256.New()
	fullHasher := sha256.New()

	// Read first 64KB for partial hash, while also feeding into full hash
	lr := io.LimitReader(f, partialSize)
	n, err := io.Copy(io.MultiWriter(partialHasher, fullHasher), lr)
	if err != nil {
		return "", "", err
	}
	partialHash = hex.EncodeToString(partialHasher.Sum(nil))

	// If file was <= 64KB, full hash equals partial hash
	if n < partialSize {
		return partialHash, partialHash, nil
	}

	// Continue reading rest for full hash
	if _, err := io.Copy(fullHasher, f); err != nil {
		return "", "", err
	}
	fullHash = hex.EncodeToString(fullHasher.Sum(nil))
	return fullHash, partialHash, nil
}

func (p *Provider) crawl(args map[string]interface{}) (interface{}, error) {
	rootPath := getString(args, "path")
	if rootPath == "" {
		return nil, fmt.Errorf("path is required")
	}

	// Expand ~ to home directory
	if strings.HasPrefix(rootPath, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			rootPath = filepath.Join(home, rootPath[2:])
		}
	}

	// Resolve to absolute path
	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	// Verify directory exists
	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("path not accessible: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", absRoot)
	}

	// Parse options
	maxDepth := getInt(args, "max_depth", -1)
	includeHidden := false
	if b := getBool(args, "include_hidden"); b != nil {
		includeHidden = *b
	}
	dryRun := false
	if b := getBool(args, "dry_run"); b != nil {
		dryRun = *b
	}
	tags := getStringArray(args, "tags")

	var includeRe, excludeRe *regexp.Regexp
	if pat := getString(args, "pattern"); pat != "" {
		includeRe, err = regexp.Compile(pat)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern regexp: %w", err)
		}
	}
	if pat := getString(args, "exclude_pattern"); pat != "" {
		excludeRe, err = regexp.Compile(pat)
		if err != nil {
			return nil, fmt.Errorf("invalid exclude_pattern regexp: %w", err)
		}
	}

	slog.Info("Starting crawl", "path", absRoot, "max_depth", maxDepth, "dry_run", dryRun)

	// Phase 1: Walk the directory and collect matching files
	var files []crawlFile
	rootDepth := strings.Count(absRoot, string(filepath.Separator))

	err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			slog.Warn("Crawl: error accessing path", "path", path, "error", err)
			return nil // skip errors, continue crawling
		}

		name := d.Name()

		// Skip hidden files/dirs unless requested
		if !includeHidden && strings.HasPrefix(name, ".") && path != absRoot {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Check depth
		if maxDepth >= 0 {
			currentDepth := strings.Count(path, string(filepath.Separator)) - rootDepth
			if d.IsDir() && currentDepth > maxDepth {
				return filepath.SkipDir
			}
			if !d.IsDir() && currentDepth > maxDepth {
				return nil
			}
		}

		// Skip directories (we only register files)
		if d.IsDir() {
			return nil
		}

		// Apply include pattern
		if includeRe != nil && !includeRe.MatchString(name) {
			return nil
		}

		// Apply exclude pattern
		if excludeRe != nil && excludeRe.MatchString(name) {
			return nil
		}

		fi, err := d.Info()
		if err != nil {
			slog.Warn("Crawl: cannot stat file", "path", path, "error", err)
			return nil
		}

		// Skip non-regular files (symlinks, devices, etc.)
		if !fi.Mode().IsRegular() {
			return nil
		}

		depth := strings.Count(path, string(filepath.Separator)) - rootDepth
		files = append(files, crawlFile{Path: path, Info: fi, RelDepth: depth})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("crawl failed: %w", err)
	}

	slog.Info("Crawl: discovery complete", "files_found", len(files))

	if dryRun {
		// Return summary without registering
		byCategory := map[string]int{}
		byExtension := map[string]int{}
		var totalSize int64
		for _, f := range files {
			ext := filepath.Ext(f.Info.Name())
			mimeType := mimeFromExtension(ext)
			cat := categoryFromMIME(mimeType)
			byCategory[cat]++
			if ext != "" {
				byExtension[strings.ToLower(ext)]++
			} else {
				byExtension["(none)"]++
			}
			totalSize += f.Info.Size()
		}
		return textContent(map[string]interface{}{
			"status":       "dry_run",
			"path":         absRoot,
			"total_found":  len(files),
			"total_size":   totalSize,
			"by_category":  byCategory,
			"by_extension": byExtension,
		}), nil
	}

	// Phase 2: Hash files and register in batches using concurrent workers
	const crawlBatchSize = 50

	type crawlResult struct {
		Key    string `json:"key"`
		Status string `json:"status"`
		ID     string `json:"id,omitempty"`
		Error  string `json:"error,omitempty"`
	}

	ctx := context.Background()
	results := make([]crawlResult, len(files))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, batchWorkers)

	for i, cf := range files {
		wg.Add(1)
		go func(idx int, cf crawlFile) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			key := fmt.Sprintf("local:%s", cf.Path)
			cr := crawlResult{Key: key}

			// Check if already registered
			existResp, err := p.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
				Type:  "file",
				Key:   key,
				Limit: 1,
			})
			if err == nil && len(existResp.Items) > 0 {
				cr.Status = "skipped"
				cr.ID = existResp.Items[0].ID
				mu.Lock()
				results[idx] = cr
				mu.Unlock()
				return
			}

			// Compute hashes
			fullHash, partialHash, err := hashFile(cf.Path)
			if err != nil {
				cr.Status = "error"
				cr.Error = fmt.Sprintf("hash failed: %v", err)
				mu.Lock()
				results[idx] = cr
				mu.Unlock()
				return
			}

			// Build metadata
			filename := cf.Info.Name()
			ext := filepath.Ext(filename)
			mimeType := mimeFromExtension(ext)
			category := categoryFromMIME(mimeType)
			extension := extractExtension(filename)

			properties := map[string]interface{}{
				"source":       "local",
				"path":         cf.Path,
				"filename":     filename,
				"extension":    extension,
				"content_hash": fullHash,
				"partial_hash": partialHash,
				"size":         cf.Info.Size(),
				"mime_type":    mimeType,
				"category":     category,
				"modified_at":  cf.Info.ModTime().UTC().Format(time.RFC3339),
			}

			obj, err := p.client.Graph.CreateObject(ctx, &graph.CreateObjectRequest{
				Type:       "file",
				Key:        &key,
				Status:     strPtr("active"),
				Properties: properties,
				Labels:     tags,
			})
			if err != nil {
				cr.Status = "error"
				cr.Error = err.Error()
			} else {
				cr.Status = "registered"
				cr.ID = obj.ID
			}

			mu.Lock()
			results[idx] = cr
			mu.Unlock()
		}(i, cf)
	}

	wg.Wait()

	// Aggregate counts
	registered := 0
	skipped := 0
	failed := 0
	var errors []crawlResult
	for _, r := range results {
		switch r.Status {
		case "registered":
			registered++
		case "skipped":
			skipped++
		case "error":
			failed++
			errors = append(errors, r)
		}
	}

	slog.Info("Crawl completed",
		"path", absRoot,
		"total", len(files),
		"registered", registered,
		"skipped", skipped,
		"failed", failed,
	)

	response := map[string]interface{}{
		"status":     "completed",
		"path":       absRoot,
		"total":      len(files),
		"registered": registered,
		"skipped":    skipped,
		"failed":     failed,
	}
	if len(errors) > 0 {
		response["errors"] = errors
	}

	return textContent(response), nil
}
