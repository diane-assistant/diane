package gmail

import (
	"fmt"
	"strconv"
	"time"
)

// SyncResult contains the results of a sync operation
type SyncResult struct {
	NewMessages      int      `json:"new_messages"`
	UpdatedMessages  int      `json:"updated_messages"`
	DeletedMessages  int      `json:"deleted_messages"`
	MessagesWithData []string `json:"messages_with_data,omitempty"`
	HistoryID        string   `json:"history_id"`
	SyncType         string   `json:"sync_type"` // "full" or "incremental"
	Duration         string   `json:"duration"`
}

// Sync performs an incremental sync using the History API
// If no history ID is stored, performs a full sync of recent messages
func (s *Service) Sync(maxMessages int64) (*SyncResult, error) {
	if s.cache == nil {
		return nil, fmt.Errorf("sync requires cache to be enabled")
	}

	start := time.Now()

	// Get current sync state
	state, err := s.cache.GetSyncState(s.account)
	if err != nil {
		return nil, fmt.Errorf("failed to get sync state: %w", err)
	}

	// Get current profile to get latest history ID
	profile, err := s.client.GetProfile()
	if err != nil {
		return nil, fmt.Errorf("failed to get profile: %w", err)
	}

	result := &SyncResult{
		HistoryID: fmt.Sprintf("%d", profile.HistoryId),
	}

	if state == nil || state.HistoryID == "" {
		// No previous sync, do a full sync
		err = s.fullSync(maxMessages, result)
		result.SyncType = "full"
	} else {
		// Try incremental sync
		historyID, _ := strconv.ParseUint(state.HistoryID, 10, 64)
		err = s.incrementalSync(historyID, result)
		result.SyncType = "incremental"
	}

	if err != nil {
		return nil, err
	}

	// Update sync state
	now := time.Now()
	newState := &SyncState{
		Account:   s.account,
		HistoryID: result.HistoryID,
	}
	if result.SyncType == "full" {
		newState.LastFullSync = &now
	} else {
		newState.LastIncrementalSync = &now
	}
	s.cache.SaveSyncState(newState)

	result.Duration = time.Since(start).String()
	return result, nil
}

// fullSync fetches recent messages and caches their metadata
func (s *Service) fullSync(maxMessages int64, result *SyncResult) error {
	// Search for all recent messages
	messages, err := s.client.ListMessages("", maxMessages)
	if err != nil {
		return fmt.Errorf("failed to list messages: %w", err)
	}

	if len(messages) == 0 {
		return nil
	}

	// Fetch metadata for all messages
	ids := make([]string, len(messages))
	for i, msg := range messages {
		ids[i] = msg.Id
	}

	fetched, err := s.client.BatchGetMessages(ids, "metadata")
	if err != nil {
		return fmt.Errorf("failed to fetch messages: %w", err)
	}

	for _, msg := range fetched {
		if msg == nil {
			continue
		}
		email := s.gmailMessageToEmail(msg, false)
		if err := s.cache.SaveEmail(email); err == nil {
			result.NewMessages++
			result.MessagesWithData = append(result.MessagesWithData, msg.Id)
		}
	}

	return nil
}

// incrementalSync uses the History API to get changes since last sync
func (s *Service) incrementalSync(startHistoryID uint64, result *SyncResult) error {
	histories, err := s.client.GetHistory(startHistoryID)
	if err != nil {
		// History may be expired (after ~7 days), fall back to full sync
		// This is indicated by a 404 error from the API
		// For now, just return the error
		return fmt.Errorf("failed to get history (may need full sync): %w", err)
	}

	addedIDs := make(map[string]bool)
	deletedIDs := make(map[string]bool)
	modifiedIDs := make(map[string]bool)

	for _, h := range histories {
		// Track additions
		for _, msg := range h.MessagesAdded {
			if msg.Message != nil {
				addedIDs[msg.Message.Id] = true
			}
		}

		// Track deletions
		for _, msg := range h.MessagesDeleted {
			if msg.Message != nil {
				deletedIDs[msg.Message.Id] = true
				delete(addedIDs, msg.Message.Id) // Remove from added if deleted
			}
		}

		// Track label changes (modifications)
		for _, msg := range h.LabelsAdded {
			if msg.Message != nil && !addedIDs[msg.Message.Id] {
				modifiedIDs[msg.Message.Id] = true
			}
		}
		for _, msg := range h.LabelsRemoved {
			if msg.Message != nil && !addedIDs[msg.Message.Id] {
				modifiedIDs[msg.Message.Id] = true
			}
		}
	}

	// Fetch new and modified messages
	idsToFetch := make([]string, 0, len(addedIDs)+len(modifiedIDs))
	for id := range addedIDs {
		idsToFetch = append(idsToFetch, id)
	}
	for id := range modifiedIDs {
		idsToFetch = append(idsToFetch, id)
	}

	if len(idsToFetch) > 0 {
		fetched, err := s.client.BatchGetMessages(idsToFetch, "metadata")
		if err != nil {
			return fmt.Errorf("failed to fetch new messages: %w", err)
		}

		for _, msg := range fetched {
			if msg == nil {
				continue
			}
			email := s.gmailMessageToEmail(msg, false)
			if err := s.cache.SaveEmail(email); err == nil {
				if addedIDs[msg.Id] {
					result.NewMessages++
				} else {
					result.UpdatedMessages++
				}
				result.MessagesWithData = append(result.MessagesWithData, msg.Id)
			}
		}
	}

	// Handle deletions
	for id := range deletedIDs {
		if err := s.deleteFromCache(id); err == nil {
			result.DeletedMessages++
		}
	}

	return nil
}

// deleteFromCache removes a message from the cache
func (s *Service) deleteFromCache(messageID string) error {
	if s.cache == nil {
		return nil
	}

	_, err := s.cache.db.Exec("DELETE FROM emails WHERE gmail_id = ?", messageID)
	return err
}

// GetSyncState returns the current sync state
func (s *Service) GetSyncState() (*SyncState, error) {
	if s.cache == nil {
		return nil, nil
	}
	return s.cache.GetSyncState(s.account)
}

// ForceFullSync clears sync state and performs a full sync
func (s *Service) ForceFullSync(maxMessages int64) (*SyncResult, error) {
	if s.cache == nil {
		return nil, fmt.Errorf("sync requires cache to be enabled")
	}

	// Clear sync state
	_, err := s.cache.db.Exec("DELETE FROM sync_state WHERE account = ?", s.account)
	if err != nil {
		return nil, fmt.Errorf("failed to clear sync state: %w", err)
	}

	return s.Sync(maxMessages)
}

// GetCacheStats returns statistics about the cached data
func (s *Service) GetCacheStats() (*CacheStats, error) {
	if s.cache == nil {
		return nil, nil
	}

	stats := &CacheStats{}

	// Count emails
	row := s.cache.db.QueryRow("SELECT COUNT(*) FROM emails")
	row.Scan(&stats.TotalEmails)

	// Count emails with content
	row = s.cache.db.QueryRow("SELECT COUNT(*) FROM emails WHERE content_cached_at IS NOT NULL")
	row.Scan(&stats.EmailsWithContent)

	// Count attachments
	row = s.cache.db.QueryRow("SELECT COUNT(*) FROM attachments")
	row.Scan(&stats.TotalAttachments)

	// Count downloaded attachments
	row = s.cache.db.QueryRow("SELECT COUNT(*) FROM attachments WHERE local_path IS NOT NULL")
	row.Scan(&stats.DownloadedAttachments)

	// Get sync state
	state, _ := s.cache.GetSyncState(s.account)
	if state != nil {
		stats.LastFullSync = state.LastFullSync
		stats.LastIncrementalSync = state.LastIncrementalSync
		stats.HistoryID = state.HistoryID
	}

	// Get date range
	row = s.cache.db.QueryRow("SELECT MIN(date), MAX(date) FROM emails")
	row.Scan(&stats.OldestEmail, &stats.NewestEmail)

	return stats, nil
}

// CacheStats contains statistics about the local cache
type CacheStats struct {
	TotalEmails           int        `json:"total_emails"`
	EmailsWithContent     int        `json:"emails_with_content"`
	TotalAttachments      int        `json:"total_attachments"`
	DownloadedAttachments int        `json:"downloaded_attachments"`
	OldestEmail           *time.Time `json:"oldest_email,omitempty"`
	NewestEmail           *time.Time `json:"newest_email,omitempty"`
	LastFullSync          *time.Time `json:"last_full_sync,omitempty"`
	LastIncrementalSync   *time.Time `json:"last_incremental_sync,omitempty"`
	HistoryID             string     `json:"history_id,omitempty"`
}

// PrefetchContent fetches and caches content for messages that only have metadata
func (s *Service) PrefetchContent(messageIDs []string) (int, error) {
	if s.cache == nil {
		return 0, nil
	}

	prefetched := 0

	for _, id := range messageIDs {
		// Check if already has content
		email, err := s.cache.GetEmail(id)
		if err != nil || email == nil || email.ContentCachedAt != nil {
			continue
		}

		// Fetch content
		_, err = s.GetMessageContent(id)
		if err == nil {
			prefetched++
		}
	}

	return prefetched, nil
}

// PrefetchContentForQuery fetches content for all messages matching a query
func (s *Service) PrefetchContentForQuery(query string, maxMessages int64) (int, error) {
	messages, err := s.client.ListMessages(query, maxMessages)
	if err != nil {
		return 0, err
	}

	ids := make([]string, len(messages))
	for i, msg := range messages {
		ids[i] = msg.Id
	}

	return s.PrefetchContent(ids)
}
