// Package drive provides native Google Drive API client
package drive

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/diane-assistant/diane/mcp/tools/google/auth"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// Client wraps the Google Drive API service
type Client struct {
	srv     *drive.Service
	account string
}

// FileInfo represents a simplified file info for JSON output
type FileInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	MimeType     string `json:"mimeType"`
	ModifiedTime string `json:"modifiedTime,omitempty"`
	Size         int64  `json:"size,omitempty"`
	Shared       bool   `json:"shared,omitempty"`
	WebViewLink  string `json:"webViewLink,omitempty"`
}

// NewClient creates a new Google Drive API client
func NewClient(account string) (*Client, error) {
	if account == "" {
		account = "default"
	}

	ctx := context.Background()

	tokenSource, err := auth.GetTokenSource(ctx, account, drive.DriveReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("failed to get token source: %w", err)
	}

	srv, err := drive.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, fmt.Errorf("failed to create Drive service: %w", err)
	}

	return &Client{srv: srv, account: account}, nil
}

// SearchFiles searches for files matching a query
func (c *Client) SearchFiles(query string, maxResults int64) ([]FileInfo, error) {
	if maxResults <= 0 {
		maxResults = 10
	}

	var files []FileInfo
	pageToken := ""

	for {
		req := c.srv.Files.List().
			Q(query).
			PageSize(maxResults).
			Fields("nextPageToken, files(id, name, mimeType, modifiedTime, size, shared, webViewLink)")

		if pageToken != "" {
			req = req.PageToken(pageToken)
		}

		resp, err := req.Do()
		if err != nil {
			return nil, fmt.Errorf("failed to search files: %w", err)
		}

		for _, f := range resp.Files {
			files = append(files, FileInfo{
				ID:           f.Id,
				Name:         f.Name,
				MimeType:     f.MimeType,
				ModifiedTime: f.ModifiedTime,
				Size:         f.Size,
				Shared:       f.Shared,
				WebViewLink:  f.WebViewLink,
			})
		}

		// Stop if we have enough results or no more pages
		if int64(len(files)) >= maxResults || resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	// Trim to maxResults
	if int64(len(files)) > maxResults {
		files = files[:maxResults]
	}

	return files, nil
}

// ListRecentFiles lists recent files sorted by modification time
func (c *Client) ListRecentFiles(maxResults int64) ([]FileInfo, error) {
	if maxResults <= 0 {
		maxResults = 20
	}

	resp, err := c.srv.Files.List().
		OrderBy("modifiedTime desc").
		PageSize(maxResults).
		Fields("files(id, name, mimeType, modifiedTime, size, shared, webViewLink)").
		Do()

	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	files := make([]FileInfo, len(resp.Files))
	for i, f := range resp.Files {
		files[i] = FileInfo{
			ID:           f.Id,
			Name:         f.Name,
			MimeType:     f.MimeType,
			ModifiedTime: f.ModifiedTime,
			Size:         f.Size,
			Shared:       f.Shared,
			WebViewLink:  f.WebViewLink,
		}
	}

	return files, nil
}

// ToJSON converts files to JSON string
func ToJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	return string(b)
}
