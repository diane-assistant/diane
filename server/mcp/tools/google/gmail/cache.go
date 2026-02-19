// Package gmail provides Gmail API client with Emergent-backed caching
package gmail

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	sdk "github.com/emergent-company/emergent/apps/server-go/pkg/sdk"
	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"

	"github.com/diane-assistant/diane/internal/emergent"
)

const (
	emailObjectType     = "gmail_email"
	attachObjectType    = "gmail_attachment"
	syncStateObjectType = "gmail_sync_state"
)

// Cache manages Gmail cache data via the Emergent graph API.
type Cache struct {
	client *sdk.Client
}

// Email represents a cached email message
type Email struct {
	GmailID          string     `json:"gmail_id"`
	ThreadID         string     `json:"thread_id"`
	Subject          string     `json:"subject"`
	FromEmail        string     `json:"from_email"`
	FromName         string     `json:"from_name"`
	ToEmails         []string   `json:"to_emails"`
	CcEmails         []string   `json:"cc_emails"`
	Date             time.Time  `json:"date"`
	Snippet          string     `json:"snippet"`
	Labels           []string   `json:"labels"`
	HasAttachments   bool       `json:"has_attachments"`
	PlainText        *string    `json:"plain_text,omitempty"`
	JsonLD           []any      `json:"json_ld,omitempty"`
	MetadataCachedAt time.Time  `json:"metadata_cached_at"`
	ContentCachedAt  *time.Time `json:"content_cached_at,omitempty"`
	AccessedAt       time.Time  `json:"accessed_at"`
}

// Attachment represents a cached attachment reference
type Attachment struct {
	ID           int64      `json:"id"`
	GmailID      string     `json:"gmail_id"`
	AttachmentID string     `json:"attachment_id"`
	Filename     string     `json:"filename"`
	MimeType     string     `json:"mime_type"`
	Size         int64      `json:"size"`
	LocalPath    *string    `json:"local_path,omitempty"`
	DownloadedAt *time.Time `json:"downloaded_at,omitempty"`
}

// SenderStats represents aggregated sender statistics
type SenderStats struct {
	EmailPattern   string    `json:"email_pattern"`
	DisplayName    string    `json:"display_name"`
	MessageCount   int       `json:"message_count"`
	FirstSeen      time.Time `json:"first_seen"`
	LastSeen       time.Time `json:"last_seen"`
	CommonSubjects []string  `json:"common_subjects"`
	JsonLDTypes    []string  `json:"json_ld_types"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// SyncState represents sync state for an account
type SyncState struct {
	Account             string     `json:"account"`
	HistoryID           string     `json:"history_id"`
	LastFullSync        *time.Time `json:"last_full_sync,omitempty"`
	LastIncrementalSync *time.Time `json:"last_incremental_sync,omitempty"`
}

// NewCache creates a new Emergent-backed Gmail cache.
func NewCache() (*Cache, error) {
	client, err := emergent.GetClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get Emergent client for gmail cache: %w", err)
	}
	return &Cache{client: client}, nil
}

// Close is a no-op since we use the shared Emergent client singleton.
func (c *Cache) Close() error {
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func strPtr(s string) *string { return &s }

func ctx() context.Context { return context.Background() }

// lookupByKey returns the first object of the given type with the given key, or nil.
func (c *Cache) lookupByKey(objType, key string) (*graph.GraphObject, error) {
	resp, err := c.client.Graph.ListObjects(ctx(), &graph.ListObjectsOptions{
		Type:  objType,
		Key:   key,
		Limit: 1,
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Items) == 0 {
		return nil, nil
	}
	return resp.Items[0], nil
}

// listAllObjects fetches all objects of a given type, handling pagination.
func (c *Cache) listAllObjects(objType string) ([]*graph.GraphObject, error) {
	var all []*graph.GraphObject
	cursor := ""
	for {
		opts := &graph.ListObjectsOptions{
			Type:  objType,
			Limit: 1000,
		}
		if cursor != "" {
			opts.Cursor = cursor
		}
		resp, err := c.client.Graph.ListObjects(ctx(), opts)
		if err != nil {
			return nil, err
		}
		all = append(all, resp.Items...)
		if resp.NextCursor == nil || *resp.NextCursor == "" {
			break
		}
		cursor = *resp.NextCursor
	}
	return all, nil
}

// ---------------------------------------------------------------------------
// Email ↔ object conversion
// ---------------------------------------------------------------------------

func emailLabels(e *Email) []string {
	return []string{
		fmt.Sprintf("gmail_id:%s", e.GmailID),
		fmt.Sprintf("thread_id:%s", e.ThreadID),
		fmt.Sprintf("from_email:%s", e.FromEmail),
	}
}

func emailToProperties(e *Email) map[string]any {
	props := map[string]any{
		"gmail_id":           e.GmailID,
		"thread_id":          e.ThreadID,
		"subject":            e.Subject,
		"from_email":         e.FromEmail,
		"from_name":          e.FromName,
		"date":               e.Date.Format(time.RFC3339Nano),
		"snippet":            e.Snippet,
		"has_attachments":    e.HasAttachments,
		"metadata_cached_at": e.MetadataCachedAt.Format(time.RFC3339Nano),
		"accessed_at":        e.AccessedAt.Format(time.RFC3339Nano),
	}

	if b, err := json.Marshal(e.ToEmails); err == nil {
		props["to_emails"] = string(b)
	}
	if b, err := json.Marshal(e.CcEmails); err == nil {
		props["cc_emails"] = string(b)
	}
	if b, err := json.Marshal(e.Labels); err == nil {
		props["labels"] = string(b)
	}
	if e.PlainText != nil {
		props["plain_text"] = *e.PlainText
	}
	if len(e.JsonLD) > 0 {
		if b, err := json.Marshal(e.JsonLD); err == nil {
			props["json_ld"] = string(b)
		}
	}
	if e.ContentCachedAt != nil {
		props["content_cached_at"] = e.ContentCachedAt.Format(time.RFC3339Nano)
	}
	return props
}

func emailFromObject(obj *graph.GraphObject) *Email {
	e := &Email{}
	p := obj.Properties

	if v, ok := p["gmail_id"].(string); ok {
		e.GmailID = v
	}
	if v, ok := p["thread_id"].(string); ok {
		e.ThreadID = v
	}
	if v, ok := p["subject"].(string); ok {
		e.Subject = v
	}
	if v, ok := p["from_email"].(string); ok {
		e.FromEmail = v
	}
	if v, ok := p["from_name"].(string); ok {
		e.FromName = v
	}
	if v, ok := p["to_emails"].(string); ok {
		json.Unmarshal([]byte(v), &e.ToEmails)
	}
	if v, ok := p["cc_emails"].(string); ok {
		json.Unmarshal([]byte(v), &e.CcEmails)
	}
	if v, ok := p["date"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			e.Date = t
		}
	}
	if v, ok := p["snippet"].(string); ok {
		e.Snippet = v
	}
	if v, ok := p["labels"].(string); ok {
		json.Unmarshal([]byte(v), &e.Labels)
	}
	if v, ok := p["has_attachments"].(bool); ok {
		e.HasAttachments = v
	}
	if v, ok := p["plain_text"].(string); ok {
		e.PlainText = &v
	}
	if v, ok := p["json_ld"].(string); ok {
		json.Unmarshal([]byte(v), &e.JsonLD)
	}
	if v, ok := p["metadata_cached_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			e.MetadataCachedAt = t
		}
	}
	if v, ok := p["content_cached_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			e.ContentCachedAt = &t
		}
	}
	if v, ok := p["accessed_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			e.AccessedAt = t
		}
	}
	return e
}

// ---------------------------------------------------------------------------
// Attachment ↔ object conversion
// ---------------------------------------------------------------------------

func attachmentKey(gmailID, attachmentID string) string {
	return gmailID + ":" + attachmentID
}

func attachmentLabels(a *Attachment) []string {
	labels := []string{
		fmt.Sprintf("gmail_id:%s", a.GmailID),
	}
	if a.LocalPath != nil {
		labels = append(labels, "downloaded:true")
	} else {
		labels = append(labels, "downloaded:false")
	}
	return labels
}

func attachmentToProperties(a *Attachment) map[string]any {
	props := map[string]any{
		"gmail_id":      a.GmailID,
		"attachment_id": a.AttachmentID,
		"filename":      a.Filename,
		"mime_type":     a.MimeType,
		"size":          a.Size,
	}
	if a.LocalPath != nil {
		props["local_path"] = *a.LocalPath
	}
	if a.DownloadedAt != nil {
		props["downloaded_at"] = a.DownloadedAt.Format(time.RFC3339Nano)
	}
	return props
}

func attachmentFromObject(obj *graph.GraphObject) *Attachment {
	a := &Attachment{}
	p := obj.Properties

	if v, ok := p["gmail_id"].(string); ok {
		a.GmailID = v
	}
	if v, ok := p["attachment_id"].(string); ok {
		a.AttachmentID = v
	}
	if v, ok := p["filename"].(string); ok {
		a.Filename = v
	}
	if v, ok := p["mime_type"].(string); ok {
		a.MimeType = v
	}
	if v, ok := p["size"]; ok {
		a.Size = toInt64(v)
	}
	if v, ok := p["local_path"].(string); ok {
		a.LocalPath = &v
	}
	if v, ok := p["downloaded_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			a.DownloadedAt = &t
		}
	}
	return a
}

// toInt64 converts a JSON number (float64, json.Number, int64) to int64.
func toInt64(v interface{}) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case json.Number:
		id, _ := n.Int64()
		return id
	case int64:
		return n
	}
	return 0
}

// ---------------------------------------------------------------------------
// Email CRUD
// ---------------------------------------------------------------------------

// GetEmail retrieves an email from cache by Gmail ID.
func (c *Cache) GetEmail(gmailID string) (*Email, error) {
	obj, err := c.lookupByKey(emailObjectType, gmailID)
	if err != nil {
		return nil, fmt.Errorf("failed to get email: %w", err)
	}
	if obj == nil {
		return nil, nil
	}

	email := emailFromObject(obj)

	// Update accessed_at (fire-and-forget)
	now := time.Now()
	go func() {
		c.client.Graph.UpdateObject(ctx(), obj.ID, &graph.UpdateObjectRequest{
			Properties: map[string]any{
				"accessed_at": now.Format(time.RFC3339Nano),
			},
		})
	}()

	return email, nil
}

// SaveEmail saves or updates an email in the cache.
// Uses COALESCE-like logic: plain_text, json_ld, and content_cached_at
// are preserved from the existing record when the new values are nil.
func (c *Cache) SaveEmail(email *Email) error {
	existing, err := c.lookupByKey(emailObjectType, email.GmailID)
	if err != nil {
		return fmt.Errorf("failed to check existing email: %w", err)
	}

	props := emailToProperties(email)
	labels := emailLabels(email)

	if existing != nil {
		// COALESCE: keep existing values when new ones are nil
		if email.PlainText == nil {
			if v, ok := existing.Properties["plain_text"]; ok {
				props["plain_text"] = v
			}
		}
		if len(email.JsonLD) == 0 {
			if v, ok := existing.Properties["json_ld"]; ok {
				props["json_ld"] = v
			}
		}
		if email.ContentCachedAt == nil {
			if v, ok := existing.Properties["content_cached_at"]; ok {
				props["content_cached_at"] = v
			}
		}

		_, err = c.client.Graph.UpdateObject(ctx(), existing.ID, &graph.UpdateObjectRequest{
			Properties:    props,
			Labels:        labels,
			ReplaceLabels: true,
		})
		if err != nil {
			return fmt.Errorf("failed to update email: %w", err)
		}
	} else {
		_, err = c.client.Graph.CreateObject(ctx(), &graph.CreateObjectRequest{
			Type:       emailObjectType,
			Key:        strPtr(email.GmailID),
			Properties: props,
			Labels:     labels,
		})
		if err != nil {
			return fmt.Errorf("failed to create email: %w", err)
		}
	}
	return nil
}

// SaveEmailContent updates just the content fields (plain_text, json_ld).
func (c *Cache) SaveEmailContent(gmailID string, plainText string, jsonLD []any) error {
	existing, err := c.lookupByKey(emailObjectType, gmailID)
	if err != nil {
		return fmt.Errorf("failed to find email for content update: %w", err)
	}
	if existing == nil {
		return fmt.Errorf("email not found: %s", gmailID)
	}

	now := time.Now()
	props := map[string]any{
		"plain_text":        plainText,
		"content_cached_at": now.Format(time.RFC3339Nano),
		"accessed_at":       now.Format(time.RFC3339Nano),
	}
	if len(jsonLD) > 0 {
		if b, err := json.Marshal(jsonLD); err == nil {
			props["json_ld"] = string(b)
		}
	}

	_, err = c.client.Graph.UpdateObject(ctx(), existing.ID, &graph.UpdateObjectRequest{
		Properties: props,
	})
	if err != nil {
		return fmt.Errorf("failed to update email content: %w", err)
	}
	return nil
}

// DeleteEmail removes an email from the cache.
func (c *Cache) DeleteEmail(gmailID string) error {
	obj, err := c.lookupByKey(emailObjectType, gmailID)
	if err != nil {
		return fmt.Errorf("failed to find email for deletion: %w", err)
	}
	if obj == nil {
		return nil // Already deleted
	}

	if err := c.client.Graph.DeleteObject(ctx(), obj.ID); err != nil {
		return fmt.Errorf("failed to delete email: %w", err)
	}
	return nil
}

// SearchEmails searches cached emails by query (case-insensitive substring matching).
func (c *Cache) SearchEmails(fromPattern, subjectPattern string, limit int) ([]Email, error) {
	objs, err := c.listAllObjects(emailObjectType)
	if err != nil {
		return nil, fmt.Errorf("failed to search emails: %w", err)
	}

	fromLower := strings.ToLower(fromPattern)
	subjectLower := strings.ToLower(subjectPattern)

	var emails []Email
	for _, obj := range objs {
		e := emailFromObject(obj)

		// Apply from filter
		if fromPattern != "" {
			emailLower := strings.ToLower(e.FromEmail)
			nameLower := strings.ToLower(e.FromName)
			if !strings.Contains(emailLower, fromLower) && !strings.Contains(nameLower, fromLower) {
				continue
			}
		}

		// Apply subject filter
		if subjectPattern != "" {
			if !strings.Contains(strings.ToLower(e.Subject), subjectLower) {
				continue
			}
		}

		emails = append(emails, *e)
	}

	// Sort by date descending (matching SQL ORDER BY date DESC)
	sort.Slice(emails, func(i, j int) bool {
		return emails[i].Date.After(emails[j].Date)
	})

	// Apply limit
	if limit > 0 && len(emails) > limit {
		emails = emails[:limit]
	}

	return emails, nil
}

// ---------------------------------------------------------------------------
// Attachment CRUD
// ---------------------------------------------------------------------------

// GetAttachments retrieves attachments for an email.
func (c *Cache) GetAttachments(gmailID string) ([]Attachment, error) {
	resp, err := c.client.Graph.ListObjects(ctx(), &graph.ListObjectsOptions{
		Type:  attachObjectType,
		Label: fmt.Sprintf("gmail_id:%s", gmailID),
		Limit: 1000,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get attachments: %w", err)
	}

	var attachments []Attachment
	for _, obj := range resp.Items {
		a := attachmentFromObject(obj)
		attachments = append(attachments, *a)
	}
	return attachments, nil
}

// SaveAttachment saves an attachment reference.
// Uses COALESCE-like logic for local_path and downloaded_at.
func (c *Cache) SaveAttachment(a *Attachment) error {
	key := attachmentKey(a.GmailID, a.AttachmentID)
	existing, err := c.lookupByKey(attachObjectType, key)
	if err != nil {
		return fmt.Errorf("failed to check existing attachment: %w", err)
	}

	props := attachmentToProperties(a)
	labels := attachmentLabels(a)

	if existing != nil {
		// COALESCE: keep existing local_path/downloaded_at when new ones are nil
		if a.LocalPath == nil {
			if v, ok := existing.Properties["local_path"]; ok {
				props["local_path"] = v
				// If existing had local_path, keep downloaded label
				labels = []string{
					fmt.Sprintf("gmail_id:%s", a.GmailID),
					"downloaded:true",
				}
			}
		}
		if a.DownloadedAt == nil {
			if v, ok := existing.Properties["downloaded_at"]; ok {
				props["downloaded_at"] = v
			}
		}

		_, err = c.client.Graph.UpdateObject(ctx(), existing.ID, &graph.UpdateObjectRequest{
			Properties:    props,
			Labels:        labels,
			ReplaceLabels: true,
		})
		if err != nil {
			return fmt.Errorf("failed to update attachment: %w", err)
		}
	} else {
		_, err = c.client.Graph.CreateObject(ctx(), &graph.CreateObjectRequest{
			Type:       attachObjectType,
			Key:        strPtr(key),
			Properties: props,
			Labels:     labels,
		})
		if err != nil {
			return fmt.Errorf("failed to create attachment: %w", err)
		}
	}
	return nil
}

// UpdateAttachmentLocalPath updates the local path after download.
func (c *Cache) UpdateAttachmentLocalPath(gmailID, attachmentID, localPath string) error {
	key := attachmentKey(gmailID, attachmentID)
	existing, err := c.lookupByKey(attachObjectType, key)
	if err != nil {
		return fmt.Errorf("failed to find attachment for path update: %w", err)
	}
	if existing == nil {
		return fmt.Errorf("attachment not found: %s", key)
	}

	now := time.Now()
	_, err = c.client.Graph.UpdateObject(ctx(), existing.ID, &graph.UpdateObjectRequest{
		Properties: map[string]any{
			"local_path":    localPath,
			"downloaded_at": now.Format(time.RFC3339Nano),
		},
		Labels: []string{
			fmt.Sprintf("gmail_id:%s", gmailID),
			"downloaded:true",
		},
		ReplaceLabels: true,
	})
	if err != nil {
		return fmt.Errorf("failed to update attachment local path: %w", err)
	}
	return nil
}

// ListDownloadedAttachments returns all locally downloaded attachments.
func (c *Cache) ListDownloadedAttachments() ([]Attachment, error) {
	resp, err := c.client.Graph.ListObjects(ctx(), &graph.ListObjectsOptions{
		Type:  attachObjectType,
		Label: "downloaded:true",
		Limit: 1000,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list downloaded attachments: %w", err)
	}

	var result []Attachment
	for _, obj := range resp.Items {
		a := attachmentFromObject(obj)
		if a.LocalPath != nil {
			result = append(result, *a)
		}
	}

	// Sort by downloaded_at descending (matching SQL ORDER BY downloaded_at DESC)
	sort.Slice(result, func(i, j int) bool {
		if result[i].DownloadedAt == nil {
			return false
		}
		if result[j].DownloadedAt == nil {
			return true
		}
		return result[i].DownloadedAt.After(*result[j].DownloadedAt)
	})

	return result, nil
}

// ---------------------------------------------------------------------------
// Sender Stats
// ---------------------------------------------------------------------------

// GetSenderStats retrieves sender statistics by pattern.
// Note: sender_stats were never actively populated in the SQLite version,
// so this always returns nil. Retained for API compatibility.
func (c *Cache) GetSenderStats(pattern string) (*SenderStats, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Sync State
// ---------------------------------------------------------------------------

// GetSyncState retrieves sync state for an account.
func (c *Cache) GetSyncState(account string) (*SyncState, error) {
	obj, err := c.lookupByKey(syncStateObjectType, account)
	if err != nil {
		return nil, fmt.Errorf("failed to get sync state: %w", err)
	}
	if obj == nil {
		return nil, nil
	}

	state := &SyncState{}
	p := obj.Properties

	if v, ok := p["account"].(string); ok {
		state.Account = v
	}
	if v, ok := p["history_id"].(string); ok {
		state.HistoryID = v
	}
	if v, ok := p["last_full_sync"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			state.LastFullSync = &t
		}
	}
	if v, ok := p["last_incremental_sync"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			state.LastIncrementalSync = &t
		}
	}
	return state, nil
}

// SaveSyncState saves sync state for an account.
// Uses COALESCE-like logic for last_full_sync and last_incremental_sync.
func (c *Cache) SaveSyncState(state *SyncState) error {
	existing, err := c.lookupByKey(syncStateObjectType, state.Account)
	if err != nil {
		return fmt.Errorf("failed to check existing sync state: %w", err)
	}

	props := map[string]any{
		"account":    state.Account,
		"history_id": state.HistoryID,
	}
	if state.LastFullSync != nil {
		props["last_full_sync"] = state.LastFullSync.Format(time.RFC3339Nano)
	}
	if state.LastIncrementalSync != nil {
		props["last_incremental_sync"] = state.LastIncrementalSync.Format(time.RFC3339Nano)
	}
	labels := []string{
		fmt.Sprintf("account:%s", state.Account),
	}

	if existing != nil {
		// COALESCE: keep existing values when new ones are nil
		if state.LastFullSync == nil {
			if v, ok := existing.Properties["last_full_sync"]; ok {
				props["last_full_sync"] = v
			}
		}
		if state.LastIncrementalSync == nil {
			if v, ok := existing.Properties["last_incremental_sync"]; ok {
				props["last_incremental_sync"] = v
			}
		}

		_, err = c.client.Graph.UpdateObject(ctx(), existing.ID, &graph.UpdateObjectRequest{
			Properties:    props,
			Labels:        labels,
			ReplaceLabels: true,
		})
		if err != nil {
			return fmt.Errorf("failed to update sync state: %w", err)
		}
	} else {
		_, err = c.client.Graph.CreateObject(ctx(), &graph.CreateObjectRequest{
			Type:       syncStateObjectType,
			Key:        strPtr(state.Account),
			Properties: props,
			Labels:     labels,
		})
		if err != nil {
			return fmt.Errorf("failed to create sync state: %w", err)
		}
	}
	return nil
}

// DeleteSyncState removes sync state for an account.
func (c *Cache) DeleteSyncState(account string) error {
	obj, err := c.lookupByKey(syncStateObjectType, account)
	if err != nil {
		return fmt.Errorf("failed to find sync state for deletion: %w", err)
	}
	if obj == nil {
		return nil // Already deleted
	}

	if err := c.client.Graph.DeleteObject(ctx(), obj.ID); err != nil {
		return fmt.Errorf("failed to delete sync state: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Cache Stats (used by Service.GetCacheStats)
// ---------------------------------------------------------------------------

// GetCacheStats computes statistics about cached data.
func (c *Cache) GetCacheStats(account string) (*CacheStats, error) {
	stats := &CacheStats{}

	// Fetch all emails
	emailObjs, err := c.listAllObjects(emailObjectType)
	if err != nil {
		return nil, fmt.Errorf("failed to list emails for stats: %w", err)
	}
	stats.TotalEmails = len(emailObjs)

	var oldest, newest *time.Time
	for _, obj := range emailObjs {
		// Count emails with content
		if _, ok := obj.Properties["content_cached_at"]; ok {
			stats.EmailsWithContent++
		}
		// Track date range
		if v, ok := obj.Properties["date"].(string); ok {
			if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
				if oldest == nil || t.Before(*oldest) {
					cpy := t
					oldest = &cpy
				}
				if newest == nil || t.After(*newest) {
					cpy := t
					newest = &cpy
				}
			}
		}
	}
	stats.OldestEmail = oldest
	stats.NewestEmail = newest

	// Fetch all attachments
	attachObjs, err := c.listAllObjects(attachObjectType)
	if err != nil {
		return nil, fmt.Errorf("failed to list attachments for stats: %w", err)
	}
	stats.TotalAttachments = len(attachObjs)
	for _, obj := range attachObjs {
		if _, ok := obj.Properties["local_path"]; ok {
			stats.DownloadedAttachments++
		}
	}

	// Get sync state
	state, _ := c.GetSyncState(account)
	if state != nil {
		stats.LastFullSync = state.LastFullSync
		stats.LastIncrementalSync = state.LastIncrementalSync
		stats.HistoryID = state.HistoryID
	}

	return stats, nil
}
