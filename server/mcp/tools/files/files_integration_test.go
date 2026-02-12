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
	// 1. file_register
	// ----------------------------------------------------------------
	t.Run("file_register", func(t *testing.T) {
		result, err := p.Call("file_register", map[string]interface{}{
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
			t.Fatalf("file_register failed: %v", err)
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
	// 2. file_get by source+path
	// ----------------------------------------------------------------
	var fileID string
	t.Run("file_get_by_path", func(t *testing.T) {
		result, err := p.Call("file_get", map[string]interface{}{
			"source": "local",
			"path":   testPath,
		})
		if err != nil {
			t.Fatalf("file_get failed: %v", err)
		}
		data := extractResult(t, result)
		fileID, _ = data["id"].(string)
		if fileID == "" {
			t.Fatal("expected non-empty id from file_get")
		}
		if data["type"] != "file" {
			t.Errorf("expected type=file, got %v", data["type"])
		}
		t.Logf("Got file by path, ID: %s", fileID)
	})

	// ----------------------------------------------------------------
	// 3. file_get by ID
	// ----------------------------------------------------------------
	t.Run("file_get_by_id", func(t *testing.T) {
		if fileID == "" {
			t.Skip("no fileID from previous test")
		}
		result, err := p.Call("file_get", map[string]interface{}{
			"id": fileID,
		})
		if err != nil {
			t.Fatalf("file_get by id failed: %v", err)
		}
		data := extractResult(t, result)
		if data["id"] != fileID {
			t.Errorf("expected id=%s, got %v", fileID, data["id"])
		}
	})

	// ----------------------------------------------------------------
	// 4. file_search (browse mode — no query)
	// ----------------------------------------------------------------
	t.Run("file_search_browse", func(t *testing.T) {
		result, err := p.Call("file_search", map[string]interface{}{
			"limit": float64(10),
		})
		if err != nil {
			t.Fatalf("file_search browse failed: %v", err)
		}
		data := extractResult(t, result)
		results, ok := data["results"].([]interface{})
		if !ok {
			t.Fatalf("expected results array, got %T", data["results"])
		}
		t.Logf("Browse returned %d results", len(results))
	})

	// ----------------------------------------------------------------
	// 5. file_search (FTS with query)
	// ----------------------------------------------------------------
	t.Run("file_search_fts", func(t *testing.T) {
		// Give Emergent a moment to index
		time.Sleep(2 * time.Second)

		result, err := p.Call("file_search", map[string]interface{}{
			"query": "artificial intelligence",
			"limit": float64(10),
		})
		if err != nil {
			t.Fatalf("file_search FTS failed: %v", err)
		}
		data := extractResult(t, result)
		t.Logf("FTS search returned total=%v", data["total"])
	})

	// ----------------------------------------------------------------
	// 6. file_semantic_search
	// ----------------------------------------------------------------
	t.Run("file_semantic_search", func(t *testing.T) {
		result, err := p.Call("file_semantic_search", map[string]interface{}{
			"query": "machine learning AI research",
			"limit": float64(5),
		})
		if err != nil {
			t.Fatalf("file_semantic_search failed: %v", err)
		}
		data := extractResult(t, result)
		t.Logf("Semantic search returned total=%v", data["total"])
	})

	// ----------------------------------------------------------------
	// 7. file_tag
	// ----------------------------------------------------------------
	t.Run("file_tag", func(t *testing.T) {
		if fileID == "" {
			t.Skip("no fileID")
		}
		result, err := p.Call("file_tag", map[string]interface{}{
			"id":   fileID,
			"tags": []interface{}{"important", "reviewed"},
		})
		if err != nil {
			t.Fatalf("file_tag failed: %v", err)
		}
		data := extractResult(t, result)
		if data["status"] != "tagged" {
			t.Errorf("expected status=tagged, got %v", data["status"])
		}
	})

	// ----------------------------------------------------------------
	// 8. file_untag
	// ----------------------------------------------------------------
	t.Run("file_untag", func(t *testing.T) {
		if fileID == "" {
			t.Skip("no fileID")
		}
		result, err := p.Call("file_untag", map[string]interface{}{
			"id":   fileID,
			"tags": []interface{}{"reviewed"},
		})
		if err != nil {
			t.Fatalf("file_untag failed: %v", err)
		}
		data := extractResult(t, result)
		if data["status"] != "untagged" {
			t.Errorf("expected status=untagged, got %v", data["status"])
		}
	})

	// ----------------------------------------------------------------
	// 9. file_tags
	// ----------------------------------------------------------------
	t.Run("file_tags", func(t *testing.T) {
		result, err := p.Call("file_tags", map[string]interface{}{})
		if err != nil {
			t.Fatalf("file_tags failed: %v", err)
		}
		data := extractResult(t, result)
		t.Logf("Tags: total=%v", data["total"])
	})

	// ----------------------------------------------------------------
	// 10. file_verify
	// ----------------------------------------------------------------
	t.Run("file_verify", func(t *testing.T) {
		if fileID == "" {
			t.Skip("no fileID")
		}
		result, err := p.Call("file_verify", map[string]interface{}{
			"id": fileID,
		})
		if err != nil {
			t.Fatalf("file_verify failed: %v", err)
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
	// 11. file_stats
	// ----------------------------------------------------------------
	t.Run("file_stats", func(t *testing.T) {
		result, err := p.Call("file_stats", map[string]interface{}{})
		if err != nil {
			t.Fatalf("file_stats failed: %v", err)
		}
		data := extractResult(t, result)
		total, _ := data["total"].(float64)
		if total < 1 {
			t.Errorf("expected total >= 1, got %v", total)
		}
		t.Logf("Stats: total=%v, by_source=%v", data["total"], data["by_source"])
	})

	// ----------------------------------------------------------------
	// 12. file_recent
	// ----------------------------------------------------------------
	t.Run("file_recent", func(t *testing.T) {
		result, err := p.Call("file_recent", map[string]interface{}{
			"limit": float64(5),
		})
		if err != nil {
			t.Fatalf("file_recent failed: %v", err)
		}
		data := extractResult(t, result)
		t.Logf("Recent files: total=%v", data["total"])
	})

	// ----------------------------------------------------------------
	// 13. file_duplicates (by specific hash)
	// ----------------------------------------------------------------
	t.Run("file_duplicates", func(t *testing.T) {
		result, err := p.Call("file_duplicates", map[string]interface{}{
			"content_hash": "abc123hash" + uniqueSuffix,
		})
		if err != nil {
			t.Fatalf("file_duplicates failed: %v", err)
		}
		data := extractResult(t, result)
		count, _ := data["count"].(float64)
		if count < 1 {
			t.Errorf("expected at least 1 file with this hash, got %v", count)
		}
		t.Logf("Duplicates for hash: count=%v", count)
	})

	// ----------------------------------------------------------------
	// 14. file_similar
	// ----------------------------------------------------------------
	t.Run("file_similar", func(t *testing.T) {
		if fileID == "" {
			t.Skip("no fileID")
		}
		result, err := p.Call("file_similar", map[string]interface{}{
			"id":    fileID,
			"limit": float64(5),
		})
		if err != nil {
			// FindSimilar may fail if no embeddings exist yet — log but don't fail hard
			t.Logf("file_similar returned error (may be expected if no embeddings): %v", err)
			return
		}
		data := extractResult(t, result)
		t.Logf("Similar files: total=%v", data["total"])
	})

	// ----------------------------------------------------------------
	// 15. Register a second file (for duplicate detection test)
	// ----------------------------------------------------------------
	t.Run("register_second_file", func(t *testing.T) {
		_, err := p.Call("file_register", map[string]interface{}{
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
	// 16. file_duplicates (all groups)
	// ----------------------------------------------------------------
	t.Run("file_duplicates_all", func(t *testing.T) {
		result, err := p.Call("file_duplicates", map[string]interface{}{
			"limit": float64(10),
		})
		if err != nil {
			t.Fatalf("file_duplicates all failed: %v", err)
		}
		data := extractResult(t, result)
		t.Logf("Duplicate groups: total_groups=%v", data["total_groups"])
	})

	// ----------------------------------------------------------------
	// 17. file_search with source filter
	// ----------------------------------------------------------------
	t.Run("file_search_with_source_filter", func(t *testing.T) {
		result, err := p.Call("file_search", map[string]interface{}{
			"sources": []interface{}{"gdrive"},
			"limit":   float64(10),
		})
		if err != nil {
			t.Fatalf("file_search with source filter failed: %v", err)
		}
		data := extractResult(t, result)
		t.Logf("Search with source=gdrive: total=%v", data["total"])
	})

	// ----------------------------------------------------------------
	// 18. file_search with tag filter
	// ----------------------------------------------------------------
	t.Run("file_search_with_tag_filter", func(t *testing.T) {
		result, err := p.Call("file_search", map[string]interface{}{
			"tags":  []interface{}{"important"},
			"limit": float64(10),
		})
		if err != nil {
			t.Fatalf("file_search with tag filter failed: %v", err)
		}
		data := extractResult(t, result)
		t.Logf("Search with tags=[important]: total=%v", data["total"])
	})

	// ----------------------------------------------------------------
	// 19. file_remove (cleanup both test files)
	// ----------------------------------------------------------------
	t.Run("file_remove", func(t *testing.T) {
		if fileID == "" {
			t.Skip("no fileID")
		}
		result, err := p.Call("file_remove", map[string]interface{}{
			"id": fileID,
		})
		if err != nil {
			t.Fatalf("file_remove failed: %v", err)
		}
		data := extractResult(t, result)
		if data["status"] != "removed" {
			t.Errorf("expected status=removed, got %v", data["status"])
		}
	})

	t.Run("file_remove_second", func(t *testing.T) {
		result, err := p.Call("file_remove", map[string]interface{}{
			"source": "gdrive",
			"path":   "/My Drive/test-copy-" + uniqueSuffix + ".txt",
		})
		if err != nil {
			t.Fatalf("file_remove second failed: %v", err)
		}
		data := extractResult(t, result)
		if data["status"] != "removed" {
			t.Errorf("expected status=removed, got %v", data["status"])
		}
	})
}
