// Package files provides MCP tools for unified file indexing and management.
// This is a passive index - files are registered by the LLM using metadata
// gathered from existing tools (shell commands, Drive MCP, Gmail MCP, etc.)
//
// Backend: Emergent Knowledge Base (graph objects + vector search)
package files

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	sdk "github.com/emergent-company/emergent/apps/server-go/pkg/sdk"
	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"
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
// Configuration is read from environment variables:
//   - EMERGENT_BASE_URL (default: http://localhost:3002)
//   - EMERGENT_API_KEY (required)
//   - EMERGENT_PROJECT_ID (required)
func NewProvider() (*Provider, error) {
	baseURL := os.Getenv("EMERGENT_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:3002"
	}

	apiKey := os.Getenv("EMERGENT_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("EMERGENT_API_KEY environment variable is required")
	}

	projectID := os.Getenv("EMERGENT_PROJECT_ID")
	if projectID == "" {
		return nil, fmt.Errorf("EMERGENT_PROJECT_ID environment variable is required")
	}

	client, err := sdk.New(sdk.Config{
		ServerURL: baseURL,
		Auth: sdk.AuthConfig{
			Mode:   "apikey",
			APIKey: apiKey,
		},
		ProjectID: projectID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Emergent client: %w", err)
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
	return "files"
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
			Name: "file_register",
			Description: `Register a file in the unified file index. Use this after discovering files via shell commands (find, stat, file, sha256sum) or other tools (Drive, Gmail). 
The LLM gathers file metadata and registers it here for unified search and management.
Required: source (local, gdrive, gmail, etc.) and path. Other fields are optional but improve search.`,
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
					"content_hash":    stringProperty("SHA-256 hash of full file content (for duplicate detection)"),
					"partial_hash":    stringProperty("SHA-256 hash of first 64KB (for quick duplicate detection)"),
					"content_text":    stringProperty("Extracted text content (for full-text search and embeddings)"),
					"content_preview": stringProperty("First ~500 chars preview"),
					"category":        stringProperty("File category: document, image, video, audio, code, archive, data, other"),
					"subcategory":     stringProperty("Subcategory: invoice, receipt, photo, screenshot, etc."),
					"tags":            arrayProperty("Tags/labels to apply to this file", "string"),
				},
				[]string{"source", "path"},
			),
		},
		{
			Name:        "file_get",
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
			Name:        "file_search",
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
			Name:        "file_semantic_search",
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
			Name:        "file_tag",
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
			Name:        "file_untag",
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
			Name:        "file_tags",
			Description: "List all tags/labels in the file index with usage information.",
			InputSchema: objectSchema(
				map[string]interface{}{},
				nil,
			),
		},
		{
			Name:        "file_duplicates",
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
			Name:        "file_remove",
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
			Name:        "file_verify",
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
			Name:        "file_stats",
			Description: "Get statistics about the file index: total files by category, by source, etc.",
			InputSchema: objectSchema(
				map[string]interface{}{},
				nil,
			),
		},
		{
			Name:        "file_recent",
			Description: "Get recently indexed or accessed files.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"limit": intProperty("Maximum number of items to return", 20),
				},
				nil,
			),
		},
		{
			Name:        "file_similar",
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
	}
}

// HasTool checks if a tool name belongs to this provider
func (p *Provider) HasTool(name string) bool {
	switch name {
	case "file_register", "file_get", "file_search", "file_semantic_search",
		"file_tag", "file_untag", "file_tags", "file_duplicates",
		"file_remove", "file_verify", "file_stats", "file_recent", "file_similar":
		return true
	}
	return false
}

// Call executes a file management tool
func (p *Provider) Call(name string, args map[string]interface{}) (interface{}, error) {
	switch name {
	case "file_register":
		return p.register(args)
	case "file_get":
		return p.get(args)
	case "file_search":
		return p.search(args)
	case "file_semantic_search":
		return p.semanticSearch(args)
	case "file_tag":
		return p.tag(args)
	case "file_untag":
		return p.untag(args)
	case "file_tags":
		return p.tags(args)
	case "file_duplicates":
		return p.duplicates(args)
	case "file_remove":
		return p.remove(args)
	case "file_verify":
		return p.verify(args)
	case "file_stats":
		return p.stats(args)
	case "file_recent":
		return p.recent(args)
	case "file_similar":
		return p.similar(args)
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
	if source == "" {
		return nil, fmt.Errorf("source is required")
	}
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}

	filename := getString(args, "filename")
	if filename == "" {
		filename = extractFilename(path)
	}
	extension := extractExtension(filename)

	// Build properties from file metadata
	properties := map[string]interface{}{
		"source":    source,
		"path":      path,
		"filename":  filename,
		"extension": extension,
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
	if v := getString(args, "content_hash"); v != "" {
		properties["content_hash"] = v
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
