// Integration tests for the files provider against a live Emergent instance.
// Run with: EMERGENT_API_KEY=... EMERGENT_PROJECT_ID=... go test -v -run TestIntegration ./mcp/tools/files/
package files

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

func mustProvider(t *testing.T) *Provider {
	t.Helper()
	if os.Getenv("EMERGENT_API_KEY") == "" || os.Getenv("EMERGENT_PROJECT_ID") == "" {
		t.Skip("EMERGENT_API_KEY and EMERGENT_PROJECT_ID required for integration tests")
	}
	p, err := NewProvider()
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	return p
}

// extractResult parses the textContent response wrapper and returns the inner data map.
func extractResult(t *testing.T, result interface{}) map[string]interface{} {
	t.Helper()
	wrapper, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map wrapper, got %T", result)
	}
	contentArr, ok := wrapper["content"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected content array, got %T", wrapper["content"])
	}
	if len(contentArr) == 0 {
		t.Fatal("empty content array")
	}
	text, ok := contentArr[0]["text"].(string)
	if !ok {
		t.Fatalf("expected text string, got %T", contentArr[0]["text"])
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		t.Fatalf("failed to parse result JSON: %v\nraw: %s", err, text)
	}
	return data
}

func TestIntegration_FullLifecycle(t *testing.T) {
	p := mustProvider(t)

	uniqueSuffix := fmt.Sprintf("%d", time.Now().UnixNano())
	testPath := "/tmp/test-file-" + uniqueSuffix + ".txt"

	// ----------------------------------------------------------------
	// 1. file_registry_register
	// ----------------------------------------------------------------
	t.Run("file_registry_register", func(t *testing.T) {
		result, err := p.Call("file_registry_register", map[string]interface{}{
			"source":       "local",
			"path":         testPath,
			"filename":     "test-file-" + uniqueSuffix + ".txt",
			"size":         float64(1234),
			"mime_type":    "text/plain",
			"content_hash": "abc123hash" + uniqueSuffix,
			"content_text": "This is a test document about artificial intelligence and machine learning for integration testing.",
			"category":     "document",
			"subcategory":  "test",
			"tags":         []interface{}{"test", "integration"},
		})
		if err != nil {
			t.Fatalf("file_registry_register failed: %v", err)
		}
		data := extractResult(t, result)
		if data["status"] != "registered" {
			t.Errorf("expected status=registered, got %v", data["status"])
		}
		if data["id"] == nil || data["id"] == "" {
			t.Error("expected non-empty id")
		}
		t.Logf("Registered file ID: %v", data["id"])
	})

	// ----------------------------------------------------------------
	// 2. file_registry_get by source+path
	// ----------------------------------------------------------------
	var fileID string
	t.Run("file_registry_get_by_path", func(t *testing.T) {
		result, err := p.Call("file_registry_get", map[string]interface{}{
			"source": "local",
			"path":   testPath,
		})
		if err != nil {
			t.Fatalf("file_registry_get failed: %v", err)
		}
		data := extractResult(t, result)
		fileID, _ = data["id"].(string)
		if fileID == "" {
			t.Fatal("expected non-empty id from file_registry_get")
		}
		if data["type"] != "file" {
			t.Errorf("expected type=file, got %v", data["type"])
		}
		t.Logf("Got file by path, ID: %s", fileID)
	})

	// ----------------------------------------------------------------
	// 3. file_registry_get by ID
	// ----------------------------------------------------------------
	t.Run("file_registry_get_by_id", func(t *testing.T) {
		if fileID == "" {
			t.Skip("no fileID from previous test")
		}
		result, err := p.Call("file_registry_get", map[string]interface{}{
			"id": fileID,
		})
		if err != nil {
			t.Fatalf("file_registry_get by id failed: %v", err)
		}
		data := extractResult(t, result)
		if data["id"] != fileID {
			t.Errorf("expected id=%s, got %v", fileID, data["id"])
		}
	})

	// ----------------------------------------------------------------
	// 4. file_registry_search (browse mode — no query)
	// ----------------------------------------------------------------
	t.Run("file_registry_search_browse", func(t *testing.T) {
		result, err := p.Call("file_registry_search", map[string]interface{}{
			"limit": float64(10),
		})
		if err != nil {
			t.Fatalf("file_registry_search browse failed: %v", err)
		}
		data := extractResult(t, result)
		results, ok := data["results"].([]interface{})
		if !ok {
			t.Fatalf("expected results array, got %T", data["results"])
		}
		t.Logf("Browse returned %d results", len(results))
	})

	// ----------------------------------------------------------------
	// 5. file_registry_search (FTS with query)
	// ----------------------------------------------------------------
	t.Run("file_registry_search_fts", func(t *testing.T) {
		// Give Emergent a moment to index
		time.Sleep(2 * time.Second)

		result, err := p.Call("file_registry_search", map[string]interface{}{
			"query": "artificial intelligence",
			"limit": float64(10),
		})
		if err != nil {
			t.Fatalf("file_registry_search FTS failed: %v", err)
		}
		data := extractResult(t, result)
		t.Logf("FTS search returned total=%v", data["total"])
	})

	// ----------------------------------------------------------------
	// 6. file_registry_semantic_search
	// ----------------------------------------------------------------
	t.Run("file_registry_semantic_search", func(t *testing.T) {
		result, err := p.Call("file_registry_semantic_search", map[string]interface{}{
			"query": "machine learning AI research",
			"limit": float64(5),
		})
		if err != nil {
			t.Fatalf("file_registry_semantic_search failed: %v", err)
		}
		data := extractResult(t, result)
		t.Logf("Semantic search returned total=%v", data["total"])
	})

	// ----------------------------------------------------------------
	// 7. file_registry_tag
	// ----------------------------------------------------------------
	t.Run("file_registry_tag", func(t *testing.T) {
		if fileID == "" {
			t.Skip("no fileID")
		}
		result, err := p.Call("file_registry_tag", map[string]interface{}{
			"id":   fileID,
			"tags": []interface{}{"important", "reviewed"},
		})
		if err != nil {
			t.Fatalf("file_registry_tag failed: %v", err)
		}
		data := extractResult(t, result)
		if data["status"] != "tagged" {
			t.Errorf("expected status=tagged, got %v", data["status"])
		}
	})

	// ----------------------------------------------------------------
	// 8. file_registry_untag
	// ----------------------------------------------------------------
	t.Run("file_registry_untag", func(t *testing.T) {
		if fileID == "" {
			t.Skip("no fileID")
		}
		result, err := p.Call("file_registry_untag", map[string]interface{}{
			"id":   fileID,
			"tags": []interface{}{"reviewed"},
		})
		if err != nil {
			t.Fatalf("file_registry_untag failed: %v", err)
		}
		data := extractResult(t, result)
		if data["status"] != "untagged" {
			t.Errorf("expected status=untagged, got %v", data["status"])
		}
	})

	// ----------------------------------------------------------------
	// 9. file_registry_tags
	// ----------------------------------------------------------------
	t.Run("file_registry_tags", func(t *testing.T) {
		result, err := p.Call("file_registry_tags", map[string]interface{}{})
		if err != nil {
			t.Fatalf("file_registry_tags failed: %v", err)
		}
		data := extractResult(t, result)
		t.Logf("Tags: total=%v", data["total"])
	})

	// ----------------------------------------------------------------
	// 10. file_registry_verify
	// ----------------------------------------------------------------
	t.Run("file_registry_verify", func(t *testing.T) {
		if fileID == "" {
			t.Skip("no fileID")
		}
		result, err := p.Call("file_registry_verify", map[string]interface{}{
			"id": fileID,
		})
		if err != nil {
			t.Fatalf("file_registry_verify failed: %v", err)
		}
		data := extractResult(t, result)
		if data["status"] != "verified" {
			t.Errorf("expected status=verified, got %v", data["status"])
		}
		if data["verified_at"] == nil {
			t.Error("expected verified_at timestamp")
		}
	})

	// ----------------------------------------------------------------
	// 11. file_registry_stats
	// ----------------------------------------------------------------
	t.Run("file_registry_stats", func(t *testing.T) {
		result, err := p.Call("file_registry_stats", map[string]interface{}{})
		if err != nil {
			t.Fatalf("file_registry_stats failed: %v", err)
		}
		data := extractResult(t, result)
		total, _ := data["total"].(float64)
		if total < 1 {
			t.Errorf("expected total >= 1, got %v", total)
		}
		t.Logf("Stats: total=%v, by_source=%v", data["total"], data["by_source"])
	})

	// ----------------------------------------------------------------
	// 12. file_registry_recent
	// ----------------------------------------------------------------
	t.Run("file_registry_recent", func(t *testing.T) {
		result, err := p.Call("file_registry_recent", map[string]interface{}{
			"limit": float64(5),
		})
		if err != nil {
			t.Fatalf("file_registry_recent failed: %v", err)
		}
		data := extractResult(t, result)
		t.Logf("Recent files: total=%v", data["total"])
	})

	// ----------------------------------------------------------------
	// 13. file_registry_duplicates (by specific hash)
	// ----------------------------------------------------------------
	t.Run("file_registry_duplicates", func(t *testing.T) {
		result, err := p.Call("file_registry_duplicates", map[string]interface{}{
			"content_hash": "abc123hash" + uniqueSuffix,
		})
		if err != nil {
			t.Fatalf("file_registry_duplicates failed: %v", err)
		}
		data := extractResult(t, result)
		count, _ := data["count"].(float64)
		if count < 1 {
			t.Errorf("expected at least 1 file with this hash, got %v", count)
		}
		t.Logf("Duplicates for hash: count=%v", count)
	})

	// ----------------------------------------------------------------
	// 14. file_registry_similar
	// ----------------------------------------------------------------
	t.Run("file_registry_similar", func(t *testing.T) {
		if fileID == "" {
			t.Skip("no fileID")
		}
		result, err := p.Call("file_registry_similar", map[string]interface{}{
			"id":    fileID,
			"limit": float64(5),
		})
		if err != nil {
			// FindSimilar may fail if no embeddings exist yet — log but don't fail hard
			t.Logf("file_registry_similar returned error (may be expected if no embeddings): %v", err)
			return
		}
		data := extractResult(t, result)
		t.Logf("Similar files: total=%v", data["total"])
	})

	// ----------------------------------------------------------------
	// 15. Register a second file (for duplicate detection test)
	// ----------------------------------------------------------------
	t.Run("register_second_file", func(t *testing.T) {
		_, err := p.Call("file_registry_register", map[string]interface{}{
			"source":       "gdrive",
			"path":         "/My Drive/test-copy-" + uniqueSuffix + ".txt",
			"size":         float64(1234),
			"mime_type":    "text/plain",
			"content_hash": "abc123hash" + uniqueSuffix, // same hash = duplicate
			"content_text": "This is a duplicate test document about artificial intelligence.",
			"category":     "document",
			"tags":         []interface{}{"test", "gdrive-copy"},
		})
		if err != nil {
			t.Fatalf("register second file failed: %v", err)
		}
	})

	// ----------------------------------------------------------------
	// 16. file_registry_duplicates (all groups)
	// ----------------------------------------------------------------
	t.Run("file_registry_duplicates_all", func(t *testing.T) {
		result, err := p.Call("file_registry_duplicates", map[string]interface{}{
			"limit": float64(10),
		})
		if err != nil {
			t.Fatalf("file_registry_duplicates all failed: %v", err)
		}
		data := extractResult(t, result)
		t.Logf("Duplicate groups: total_groups=%v", data["total_groups"])
	})

	// ----------------------------------------------------------------
	// 17. file_registry_search with source filter
	// ----------------------------------------------------------------
	t.Run("file_registry_search_with_source_filter", func(t *testing.T) {
		result, err := p.Call("file_registry_search", map[string]interface{}{
			"sources": []interface{}{"gdrive"},
			"limit":   float64(10),
		})
		if err != nil {
			t.Fatalf("file_registry_search with source filter failed: %v", err)
		}
		data := extractResult(t, result)
		t.Logf("Search with source=gdrive: total=%v", data["total"])
	})

	// ----------------------------------------------------------------
	// 18. file_registry_search with tag filter
	// ----------------------------------------------------------------
	t.Run("file_registry_search_with_tag_filter", func(t *testing.T) {
		result, err := p.Call("file_registry_search", map[string]interface{}{
			"tags":  []interface{}{"important"},
			"limit": float64(10),
		})
		if err != nil {
			t.Fatalf("file_registry_search with tag filter failed: %v", err)
		}
		data := extractResult(t, result)
		t.Logf("Search with tags=[important]: total=%v", data["total"])
	})

	// ----------------------------------------------------------------
	// 19. file_registry_remove (cleanup both test files)
	// ----------------------------------------------------------------
	t.Run("file_registry_remove", func(t *testing.T) {
		if fileID == "" {
			t.Skip("no fileID")
		}
		result, err := p.Call("file_registry_remove", map[string]interface{}{
			"id": fileID,
		})
		if err != nil {
			t.Fatalf("file_registry_remove failed: %v", err)
		}
		data := extractResult(t, result)
		if data["status"] != "removed" {
			t.Errorf("expected status=removed, got %v", data["status"])
		}
	})

	t.Run("file_registry_remove_second", func(t *testing.T) {
		result, err := p.Call("file_registry_remove", map[string]interface{}{
			"source": "gdrive",
			"path":   "/My Drive/test-copy-" + uniqueSuffix + ".txt",
		})
		if err != nil {
			t.Fatalf("file_registry_remove second failed: %v", err)
		}
		data := extractResult(t, result)
		if data["status"] != "removed" {
			t.Errorf("expected status=removed, got %v", data["status"])
		}
	})
}

// TestIntegration_BatchLifecycle exercises all 5 batch tools end-to-end:
// batch_register -> batch_get -> batch_tag -> batch_untag -> batch_remove.
func TestIntegration_BatchLifecycle(t *testing.T) {
	p := mustProvider(t)

	uniqueSuffix := fmt.Sprintf("%d", time.Now().UnixNano())

	// We'll register 3 files and track their IDs for subsequent batch operations.
	var batchIDs []string

	// ----------------------------------------------------------------
	// 1. file_registry_batch_register — register 3 files at once
	// ----------------------------------------------------------------
	t.Run("file_registry_batch_register", func(t *testing.T) {
		files := []interface{}{
			map[string]interface{}{
				"source":       "local",
				"path":         "/tmp/batch-a-" + uniqueSuffix + ".txt",
				"filename":     "batch-a-" + uniqueSuffix + ".txt",
				"size":         float64(100),
				"mime_type":    "text/plain",
				"content_hash": "batchhashA" + uniqueSuffix,
				"content_text": "Batch test document alpha about neural networks.",
				"category":     "document",
				"tags":         []interface{}{"batch-test"},
			},
			map[string]interface{}{
				"source":       "local",
				"path":         "/tmp/batch-b-" + uniqueSuffix + ".md",
				"filename":     "batch-b-" + uniqueSuffix + ".md",
				"size":         float64(200),
				"mime_type":    "text/markdown",
				"content_hash": "batchhashB" + uniqueSuffix,
				"content_text": "Batch test document bravo about deep learning.",
				"category":     "document",
				"tags":         []interface{}{"batch-test"},
			},
			map[string]interface{}{
				"source":       "gdrive",
				"path":         "/My Drive/batch-c-" + uniqueSuffix + ".pdf",
				"filename":     "batch-c-" + uniqueSuffix + ".pdf",
				"size":         float64(300),
				"mime_type":    "application/pdf",
				"content_hash": "batchhashC" + uniqueSuffix,
				"content_text": "Batch test document charlie about computer vision.",
				"category":     "document",
				"tags":         []interface{}{"batch-test"},
			},
		}

		result, err := p.Call("file_registry_batch_register", map[string]interface{}{
			"files": files,
		})
		if err != nil {
			t.Fatalf("batch_register failed: %v", err)
		}
		data := extractResult(t, result)

		// Check aggregate counts
		total, _ := data["total"].(float64)
		succeeded, _ := data["succeeded"].(float64)
		failed, _ := data["failed"].(float64)
		if total != 3 {
			t.Errorf("expected total=3, got %v", total)
		}
		if succeeded != 3 {
			t.Errorf("expected succeeded=3, got %v", succeeded)
		}
		if failed != 0 {
			t.Errorf("expected failed=0, got %v", failed)
		}

		// Collect IDs from results
		results, ok := data["results"].([]interface{})
		if !ok {
			t.Fatalf("expected results array, got %T", data["results"])
		}
		for _, r := range results {
			item, _ := r.(map[string]interface{})
			id, _ := item["id"].(string)
			if id == "" {
				t.Errorf("expected non-empty id in batch result, got item: %v", item)
			}
			batchIDs = append(batchIDs, id)
		}
		t.Logf("Batch registered %d files: %v", len(batchIDs), batchIDs)
	})

	// ----------------------------------------------------------------
	// 2. file_registry_batch_register — validation: missing content_hash
	// ----------------------------------------------------------------
	t.Run("file_registry_batch_register_missing_hash", func(t *testing.T) {
		files := []interface{}{
			map[string]interface{}{
				"source": "local",
				"path":   "/tmp/no-hash-" + uniqueSuffix + ".txt",
				// content_hash intentionally omitted
			},
		}
		result, err := p.Call("file_registry_batch_register", map[string]interface{}{
			"files": files,
		})
		if err != nil {
			t.Fatalf("batch_register call failed: %v", err)
		}
		data := extractResult(t, result)
		failed, _ := data["failed"].(float64)
		if failed != 1 {
			t.Errorf("expected failed=1 for missing content_hash, got %v", failed)
		}
		results, _ := data["results"].([]interface{})
		if len(results) > 0 {
			item, _ := results[0].(map[string]interface{})
			if item["status"] != "error" {
				t.Errorf("expected status=error, got %v", item["status"])
			}
			t.Logf("Expected error: %v", item["error"])
		}
	})

	// ----------------------------------------------------------------
	// 3. file_registry_batch_get — by IDs
	// ----------------------------------------------------------------
	t.Run("file_registry_batch_get_by_ids", func(t *testing.T) {
		if len(batchIDs) == 0 {
			t.Skip("no batch IDs from previous test")
		}
		idsArg := make([]interface{}, len(batchIDs))
		for i, id := range batchIDs {
			idsArg[i] = id
		}

		result, err := p.Call("file_registry_batch_get", map[string]interface{}{
			"ids": idsArg,
		})
		if err != nil {
			t.Fatalf("batch_get by ids failed: %v", err)
		}
		data := extractResult(t, result)
		found, _ := data["found"].(float64)
		if found != float64(len(batchIDs)) {
			t.Errorf("expected found=%d, got %v", len(batchIDs), found)
		}
		t.Logf("Batch get by IDs: found=%v, not_found=%v", data["found"], data["not_found"])
	})

	// ----------------------------------------------------------------
	// 4. file_registry_batch_get — by keys (source+path pairs)
	// ----------------------------------------------------------------
	t.Run("file_registry_batch_get_by_keys", func(t *testing.T) {
		keys := []interface{}{
			map[string]interface{}{
				"source": "local",
				"path":   "/tmp/batch-a-" + uniqueSuffix + ".txt",
			},
			map[string]interface{}{
				"source": "gdrive",
				"path":   "/My Drive/batch-c-" + uniqueSuffix + ".pdf",
			},
		}

		result, err := p.Call("file_registry_batch_get", map[string]interface{}{
			"keys": keys,
		})
		if err != nil {
			t.Fatalf("batch_get by keys failed: %v", err)
		}
		data := extractResult(t, result)
		found, _ := data["found"].(float64)
		if found != 2 {
			t.Errorf("expected found=2, got %v", found)
		}
		t.Logf("Batch get by keys: found=%v, not_found=%v", data["found"], data["not_found"])
	})

	// ----------------------------------------------------------------
	// 5. file_registry_batch_tag — add tags to all 3 files
	// ----------------------------------------------------------------
	t.Run("file_registry_batch_tag", func(t *testing.T) {
		if len(batchIDs) == 0 {
			t.Skip("no batch IDs")
		}
		idsArg := make([]interface{}, len(batchIDs))
		for i, id := range batchIDs {
			idsArg[i] = id
		}

		result, err := p.Call("file_registry_batch_tag", map[string]interface{}{
			"ids":  idsArg,
			"tags": []interface{}{"reviewed", "important"},
		})
		if err != nil {
			t.Fatalf("batch_tag failed: %v", err)
		}
		data := extractResult(t, result)
		succeeded, _ := data["succeeded"].(float64)
		if succeeded != float64(len(batchIDs)) {
			t.Errorf("expected succeeded=%d, got %v", len(batchIDs), succeeded)
		}
		t.Logf("Batch tag: succeeded=%v, failed=%v", data["succeeded"], data["failed"])
	})

	// ----------------------------------------------------------------
	// 6. file_registry_batch_untag — remove one tag from all 3 files
	// ----------------------------------------------------------------
	t.Run("file_registry_batch_untag", func(t *testing.T) {
		if len(batchIDs) == 0 {
			t.Skip("no batch IDs")
		}
		idsArg := make([]interface{}, len(batchIDs))
		for i, id := range batchIDs {
			idsArg[i] = id
		}

		result, err := p.Call("file_registry_batch_untag", map[string]interface{}{
			"ids":  idsArg,
			"tags": []interface{}{"reviewed"},
		})
		if err != nil {
			t.Fatalf("batch_untag failed: %v", err)
		}
		data := extractResult(t, result)
		succeeded, _ := data["succeeded"].(float64)
		if succeeded != float64(len(batchIDs)) {
			t.Errorf("expected succeeded=%d, got %v", len(batchIDs), succeeded)
		}
		t.Logf("Batch untag: succeeded=%v, failed=%v", data["succeeded"], data["failed"])
	})

	// ----------------------------------------------------------------
	// 7. Verify tags were applied/removed correctly via batch_get
	// ----------------------------------------------------------------
	t.Run("verify_batch_tag_results", func(t *testing.T) {
		if len(batchIDs) == 0 {
			t.Skip("no batch IDs")
		}
		// Get the first file and check its tags
		result, err := p.Call("file_registry_get", map[string]interface{}{
			"id": batchIDs[0],
		})
		if err != nil {
			t.Fatalf("get after batch tag/untag failed: %v", err)
		}
		data := extractResult(t, result)
		tags, _ := data["tags"].([]interface{})
		// Should still have: batch-test, important (reviewed was removed)
		tagSet := make(map[string]bool)
		for _, tag := range tags {
			tagSet[fmt.Sprintf("%v", tag)] = true
		}
		if !tagSet["important"] {
			t.Errorf("expected 'important' tag to be present, got tags: %v", tags)
		}
		if tagSet["reviewed"] {
			t.Errorf("expected 'reviewed' tag to be removed, but it's still present: %v", tags)
		}
		t.Logf("Tags on file after batch tag/untag: %v", tags)
	})

	// ----------------------------------------------------------------
	// 8. file_registry_batch_remove — cleanup all 3 files
	// ----------------------------------------------------------------
	t.Run("file_registry_batch_remove", func(t *testing.T) {
		if len(batchIDs) == 0 {
			t.Skip("no batch IDs")
		}
		idsArg := make([]interface{}, len(batchIDs))
		for i, id := range batchIDs {
			idsArg[i] = id
		}

		result, err := p.Call("file_registry_batch_remove", map[string]interface{}{
			"ids": idsArg,
		})
		if err != nil {
			t.Fatalf("batch_remove failed: %v", err)
		}
		data := extractResult(t, result)
		succeeded, _ := data["succeeded"].(float64)
		failed, _ := data["failed"].(float64)
		if succeeded != float64(len(batchIDs)) {
			t.Errorf("expected succeeded=%d, got %v", len(batchIDs), succeeded)
		}
		if failed != 0 {
			t.Errorf("expected failed=0, got %v", failed)
		}
		t.Logf("Batch remove: succeeded=%v, failed=%v", data["succeeded"], data["failed"])
	})
}

// TestIntegration_CrawlLifecycle exercises the file_registry_crawl tool:
// creates a temp directory with test files, crawls it (dry_run + real),
// verifies skip-on-recrawl, and cleans up.
func TestIntegration_CrawlLifecycle(t *testing.T) {
	p := mustProvider(t)

	uniqueSuffix := fmt.Sprintf("%d", time.Now().UnixNano())

	// Create a temp directory with test files
	tmpDir := fmt.Sprintf("/tmp/crawl-test-%s", uniqueSuffix)
	if err := os.MkdirAll(tmpDir+"/subdir", 0755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	// Create test files
	testFiles := map[string]string{
		"hello.txt":        "Hello, world!",
		"report.pdf":       "fake pdf content for testing",
		"data.csv":         "name,value\nfoo,42",
		".hidden":          "hidden file",
		"subdir/nested.md": "# Nested doc",
		"subdir/image.png": "fake png content",
		"ignore.tmp":       "should be excluded",
	}
	for name, content := range testFiles {
		path := tmpDir + "/" + name
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create test file %s: %v", name, err)
		}
	}

	// ----------------------------------------------------------------
	// 1. Dry run — should report files without registering
	// ----------------------------------------------------------------
	t.Run("crawl_dry_run", func(t *testing.T) {
		result, err := p.Call("file_registry_crawl", map[string]interface{}{
			"path":    tmpDir,
			"dry_run": true,
		})
		if err != nil {
			t.Fatalf("crawl dry_run failed: %v", err)
		}
		data := extractResult(t, result)
		if data["status"] != "dry_run" {
			t.Errorf("expected status=dry_run, got %v", data["status"])
		}
		totalFound, _ := data["total_found"].(float64)
		// 6 visible files (hidden excluded by default)
		if totalFound < 5 {
			t.Errorf("expected at least 5 files found (hidden excluded), got %v", totalFound)
		}
		t.Logf("Dry run: total_found=%v, by_category=%v", data["total_found"], data["by_category"])
	})

	// ----------------------------------------------------------------
	// 2. Crawl with pattern filter — only .txt and .csv
	// ----------------------------------------------------------------
	t.Run("crawl_with_pattern", func(t *testing.T) {
		result, err := p.Call("file_registry_crawl", map[string]interface{}{
			"path":    tmpDir,
			"pattern": `\.(txt|csv)$`,
			"dry_run": true,
		})
		if err != nil {
			t.Fatalf("crawl with pattern failed: %v", err)
		}
		data := extractResult(t, result)
		totalFound, _ := data["total_found"].(float64)
		if totalFound != 2 {
			t.Errorf("expected 2 files matching pattern, got %v", totalFound)
		}
	})

	// ----------------------------------------------------------------
	// 3. Crawl with exclude_pattern — exclude .tmp
	// ----------------------------------------------------------------
	t.Run("crawl_with_exclude", func(t *testing.T) {
		result, err := p.Call("file_registry_crawl", map[string]interface{}{
			"path":            tmpDir,
			"exclude_pattern": `\.tmp$`,
			"dry_run":         true,
		})
		if err != nil {
			t.Fatalf("crawl with exclude failed: %v", err)
		}
		data := extractResult(t, result)
		totalFound, _ := data["total_found"].(float64)
		// All non-hidden minus .tmp = 5
		if totalFound != 5 {
			t.Errorf("expected 5 files (exclude .tmp), got %v", totalFound)
		}
	})

	// ----------------------------------------------------------------
	// 4. Crawl with max_depth=0 — only root, no subdir
	// ----------------------------------------------------------------
	t.Run("crawl_max_depth_0", func(t *testing.T) {
		result, err := p.Call("file_registry_crawl", map[string]interface{}{
			"path":      tmpDir,
			"max_depth": float64(0),
			"dry_run":   true,
		})
		if err != nil {
			t.Fatalf("crawl max_depth=0 failed: %v", err)
		}
		data := extractResult(t, result)
		totalFound, _ := data["total_found"].(float64)
		// Root files only: hello.txt, report.pdf, data.csv, ignore.tmp = 4
		if totalFound != 4 {
			t.Errorf("expected 4 root-level files, got %v", totalFound)
		}
	})

	// ----------------------------------------------------------------
	// 5. Crawl with include_hidden
	// ----------------------------------------------------------------
	t.Run("crawl_include_hidden", func(t *testing.T) {
		result, err := p.Call("file_registry_crawl", map[string]interface{}{
			"path":           tmpDir,
			"include_hidden": true,
			"dry_run":        true,
		})
		if err != nil {
			t.Fatalf("crawl include_hidden failed: %v", err)
		}
		data := extractResult(t, result)
		totalFound, _ := data["total_found"].(float64)
		// All 7 files including .hidden
		if totalFound != 7 {
			t.Errorf("expected 7 files (including hidden), got %v", totalFound)
		}
	})

	// ----------------------------------------------------------------
	// 6. Real crawl — register files (exclude .tmp)
	// ----------------------------------------------------------------
	t.Run("crawl_register", func(t *testing.T) {
		result, err := p.Call("file_registry_crawl", map[string]interface{}{
			"path":            tmpDir,
			"exclude_pattern": `\.tmp$`,
			"tags":            []interface{}{"crawl-test", uniqueSuffix},
		})
		if err != nil {
			t.Fatalf("crawl register failed: %v", err)
		}
		data := extractResult(t, result)
		if data["status"] != "completed" {
			t.Errorf("expected status=completed, got %v", data["status"])
		}
		registered, _ := data["registered"].(float64)
		if registered < 4 {
			t.Errorf("expected at least 4 registered, got %v", registered)
		}
		failed, _ := data["failed"].(float64)
		if failed != 0 {
			t.Errorf("expected 0 failures, got %v", failed)
		}
		t.Logf("Crawl register: total=%v, registered=%v, skipped=%v, failed=%v",
			data["total"], data["registered"], data["skipped"], data["failed"])
	})

	// ----------------------------------------------------------------
	// 7. Re-crawl — should skip already registered files
	// ----------------------------------------------------------------
	t.Run("crawl_skip_existing", func(t *testing.T) {
		result, err := p.Call("file_registry_crawl", map[string]interface{}{
			"path":            tmpDir,
			"exclude_pattern": `\.tmp$`,
		})
		if err != nil {
			t.Fatalf("re-crawl failed: %v", err)
		}
		data := extractResult(t, result)
		skipped, _ := data["skipped"].(float64)
		registered, _ := data["registered"].(float64)
		if skipped < 4 {
			t.Errorf("expected at least 4 skipped on re-crawl, got %v", skipped)
		}
		if registered != 0 {
			t.Errorf("expected 0 new registrations on re-crawl, got %v", registered)
		}
		t.Logf("Re-crawl: total=%v, registered=%v, skipped=%v", data["total"], registered, skipped)
	})

	// ----------------------------------------------------------------
	// 8. Verify crawled files are searchable by tag
	// ----------------------------------------------------------------
	t.Run("crawl_verify_searchable", func(t *testing.T) {
		result, err := p.Call("file_registry_search", map[string]interface{}{
			"tags":  []interface{}{uniqueSuffix},
			"limit": float64(50),
		})
		if err != nil {
			t.Fatalf("search by crawl tag failed: %v", err)
		}
		data := extractResult(t, result)
		results, _ := data["results"].([]interface{})
		if len(results) < 4 {
			t.Errorf("expected at least 4 files found by tag, got %d", len(results))
		}
		t.Logf("Search by crawl tag: found %d files", len(results))
	})

	// ----------------------------------------------------------------
	// 9. Cleanup — batch remove all crawled files
	// ----------------------------------------------------------------
	t.Run("crawl_cleanup", func(t *testing.T) {
		// Find all files with our unique tag
		result, err := p.Call("file_registry_search", map[string]interface{}{
			"tags":  []interface{}{uniqueSuffix},
			"limit": float64(100),
		})
		if err != nil {
			t.Fatalf("search for cleanup failed: %v", err)
		}
		data := extractResult(t, result)
		results, _ := data["results"].([]interface{})

		if len(results) == 0 {
			t.Log("No files to clean up")
			return
		}

		ids := make([]interface{}, 0, len(results))
		for _, r := range results {
			item, _ := r.(map[string]interface{})
			if id, ok := item["id"].(string); ok && id != "" {
				ids = append(ids, id)
			}
		}

		if len(ids) == 0 {
			t.Log("No file IDs found for cleanup")
			return
		}

		removeResult, err := p.Call("file_registry_batch_remove", map[string]interface{}{
			"ids": ids,
		})
		if err != nil {
			t.Fatalf("batch remove cleanup failed: %v", err)
		}
		removeData := extractResult(t, removeResult)
		t.Logf("Cleanup: removed %v files", removeData["succeeded"])
	})
}
