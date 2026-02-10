package gmail

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/api/gmail/v1"
)

// Service provides high-level Gmail operations with caching
type Service struct {
	client  *Client
	cache   *Cache
	account string
}

// NewService creates a new Gmail service with caching
func NewService(account string) (*Service, error) {
	client, err := NewClient(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gmail client: %w", err)
	}

	cache, err := NewCache()
	if err != nil {
		// Log warning but continue without cache
		// The service can still work, just without caching
		return &Service{client: client, account: account}, nil
	}

	return &Service{
		client:  client,
		cache:   cache,
		account: account,
	}, nil
}

// Close closes the service and its resources
func (s *Service) Close() error {
	if s.cache != nil {
		return s.cache.Close()
	}
	return nil
}

// SearchMessages searches for emails and returns cached metadata
// Results are cached for future queries
func (s *Service) SearchMessages(query string, maxResults int64) ([]Email, error) {
	// Fetch message list from API
	messages, err := s.client.ListMessages(query, maxResults)
	if err != nil {
		return nil, err
	}

	if len(messages) == 0 {
		return []Email{}, nil
	}

	// Check cache for each message
	ids := make([]string, 0, len(messages))
	cached := make(map[string]*Email)

	for _, msg := range messages {
		if s.cache != nil {
			email, err := s.cache.GetEmail(msg.Id)
			if err == nil && email != nil {
				cached[msg.Id] = email
				continue
			}
		}
		ids = append(ids, msg.Id)
	}

	// Fetch uncached messages
	if len(ids) > 0 {
		fetched, err := s.client.BatchGetMessages(ids, "metadata")
		if err != nil {
			return nil, fmt.Errorf("failed to fetch messages: %w", err)
		}

		for _, msg := range fetched {
			if msg == nil {
				continue
			}
			email := s.gmailMessageToEmail(msg, false)
			if s.cache != nil {
				s.cache.SaveEmail(email)
			}
			cached[msg.Id] = email
		}
	}

	// Build result in original order
	result := make([]Email, 0, len(messages))
	for _, msg := range messages {
		if email, ok := cached[msg.Id]; ok {
			result = append(result, *email)
		}
	}

	return result, nil
}

// GetMessage retrieves a single message with full content
// Uses cache if available, fetches and caches if not
func (s *Service) GetMessage(id string, withContent bool) (*Email, error) {
	// Check cache
	if s.cache != nil {
		email, err := s.cache.GetEmail(id)
		if err == nil && email != nil {
			// If we need content and it's cached, return it
			if !withContent || email.ContentCachedAt != nil {
				return email, nil
			}
		}
	}

	// Fetch from API
	format := "metadata"
	if withContent {
		format = "full"
	}

	msg, err := s.client.GetMessage(id, format)
	if err != nil {
		return nil, err
	}

	email := s.gmailMessageToEmail(msg, withContent)

	// Cache the result
	if s.cache != nil {
		s.cache.SaveEmail(email)
	}

	return email, nil
}

// GetMessageContent fetches and extracts content for a message
// Returns plain text and JSON-LD data
func (s *Service) GetMessageContent(id string) (*Email, error) {
	// Check if we already have content cached
	if s.cache != nil {
		email, err := s.cache.GetEmail(id)
		if err == nil && email != nil && email.ContentCachedAt != nil {
			return email, nil
		}
	}

	// Fetch full message
	msg, err := s.client.GetMessage(id, "full")
	if err != nil {
		return nil, err
	}

	// Extract content
	content, err := ExtractContent(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to extract content: %w", err)
	}

	email := s.gmailMessageToEmail(msg, true)
	email.PlainText = &content.PlainText
	email.JsonLD = content.JsonLD

	// Cache with content
	if s.cache != nil {
		s.cache.SaveEmail(email)
	}

	return email, nil
}

// BatchGetMessages retrieves multiple messages efficiently
func (s *Service) BatchGetMessages(ids []string, withContent bool) ([]*Email, error) {
	// Check cache first
	uncached := make([]string, 0)
	cached := make(map[string]*Email)

	for _, id := range ids {
		if s.cache != nil {
			email, err := s.cache.GetEmail(id)
			if err == nil && email != nil {
				if !withContent || email.ContentCachedAt != nil {
					cached[id] = email
					continue
				}
			}
		}
		uncached = append(uncached, id)
	}

	// Fetch uncached messages
	if len(uncached) > 0 {
		format := "metadata"
		if withContent {
			format = "full"
		}

		messages, err := s.client.BatchGetMessages(uncached, format)
		if err != nil {
			return nil, err
		}

		for _, msg := range messages {
			if msg == nil {
				continue
			}
			email := s.gmailMessageToEmail(msg, withContent)
			if s.cache != nil {
				s.cache.SaveEmail(email)
			}
			cached[msg.Id] = email
		}
	}

	// Build result in original order
	result := make([]*Email, len(ids))
	for i, id := range ids {
		result[i] = cached[id]
	}

	return result, nil
}

// ModifyLabels adds/removes labels from messages
func (s *Service) ModifyLabels(ids []string, addLabels, removeLabels []string) error {
	err := s.client.ModifyLabels(ids, addLabels, removeLabels)
	if err != nil {
		return err
	}

	// Update cached labels (if we have the messages cached)
	if s.cache != nil {
		for _, id := range ids {
			email, err := s.cache.GetEmail(id)
			if err != nil || email == nil {
				continue
			}

			// Update labels
			newLabels := make([]string, 0)
			removeSet := make(map[string]bool)
			for _, l := range removeLabels {
				removeSet[l] = true
			}

			for _, l := range email.Labels {
				if !removeSet[l] {
					newLabels = append(newLabels, l)
				}
			}
			newLabels = append(newLabels, addLabels...)
			email.Labels = newLabels

			s.cache.SaveEmail(email)
		}
	}

	return nil
}

// ListLabels returns all labels for the account
func (s *Service) ListLabels() ([]*gmail.Label, error) {
	return s.client.ListLabels()
}

// CreateLabel creates a new label
func (s *Service) CreateLabel(name string) (*gmail.Label, error) {
	return s.client.CreateLabel(name)
}

// GetSenderStats returns aggregated statistics for a sender
func (s *Service) GetSenderStats(senderPattern string, maxEmails int) (*SenderStats, error) {
	// Check cache first
	if s.cache != nil {
		stats, err := s.cache.GetSenderStats(senderPattern)
		if err == nil && stats != nil {
			return stats, nil
		}
	}

	// Search for emails from this sender
	query := fmt.Sprintf("from:%s", senderPattern)
	emails, err := s.SearchMessages(query, int64(maxEmails))
	if err != nil {
		return nil, err
	}

	if len(emails) == 0 {
		return nil, nil
	}

	// Aggregate stats
	stats := &SenderStats{
		EmailPattern: senderPattern,
		MessageCount: len(emails),
		UpdatedAt:    time.Now(),
	}

	subjectCounts := make(map[string]int)
	jsonLDTypes := make(map[string]bool)

	for _, email := range emails {
		// Track first/last seen
		if stats.FirstSeen.IsZero() || email.Date.Before(stats.FirstSeen) {
			stats.FirstSeen = email.Date
		}
		if stats.LastSeen.IsZero() || email.Date.After(stats.LastSeen) {
			stats.LastSeen = email.Date
		}

		// Track display name (use most recent)
		if email.FromName != "" {
			stats.DisplayName = email.FromName
		}

		// Track subject patterns
		subject := normalizeSubject(email.Subject)
		subjectCounts[subject]++

		// Track JSON-LD types
		if len(email.JsonLD) > 0 {
			for _, t := range GetJsonLDTypes(email.JsonLD) {
				jsonLDTypes[t] = true
			}
		}
	}

	// Get top subjects
	stats.CommonSubjects = getTopItems(subjectCounts, 5)

	// Get JSON-LD types
	for t := range jsonLDTypes {
		stats.JsonLDTypes = append(stats.JsonLDTypes, t)
	}

	// Cache the stats
	// Note: We'd need to add SaveSenderStats method to cache
	// For now, just return the computed stats

	return stats, nil
}

// gmailMessageToEmail converts a Gmail API message to our Email type
func (s *Service) gmailMessageToEmail(msg *gmail.Message, hasContent bool) *Email {
	headers := ExtractHeaders(msg)
	fromName, fromEmail := ParseFromHeader(headers["from"])

	// Parse date
	var date time.Time
	if dateStr := headers["date"]; dateStr != "" {
		// Try common date formats
		formats := []string{
			time.RFC1123Z,
			time.RFC1123,
			"Mon, 2 Jan 2006 15:04:05 -0700",
			"Mon, 2 Jan 2006 15:04:05 MST",
			"2 Jan 2006 15:04:05 -0700",
		}
		for _, f := range formats {
			if t, err := time.Parse(f, dateStr); err == nil {
				date = t
				break
			}
		}
	}
	if date.IsZero() && msg.InternalDate > 0 {
		date = time.UnixMilli(msg.InternalDate)
	}

	// Parse recipients
	var toEmails, ccEmails []string
	if to := headers["to"]; to != "" {
		toEmails = parseAddressList(to)
	}
	if cc := headers["cc"]; cc != "" {
		ccEmails = parseAddressList(cc)
	}

	// Check for attachments
	hasAttachments := false
	if msg.Payload != nil {
		hasAttachments = checkForAttachments(msg.Payload)
	}

	now := time.Now()
	email := &Email{
		GmailID:          msg.Id,
		ThreadID:         msg.ThreadId,
		Subject:          headers["subject"],
		FromEmail:        fromEmail,
		FromName:         fromName,
		ToEmails:         toEmails,
		CcEmails:         ccEmails,
		Date:             date,
		Snippet:          msg.Snippet,
		Labels:           msg.LabelIds,
		HasAttachments:   hasAttachments,
		MetadataCachedAt: now,
		AccessedAt:       now,
	}

	// If we have content, extract it
	if hasContent && msg.Payload != nil {
		content, err := ExtractContent(msg)
		if err == nil {
			email.PlainText = &content.PlainText
			email.JsonLD = content.JsonLD
			email.ContentCachedAt = &now
		}
	}

	return email
}

// Helper functions

func parseAddressList(addresses string) []string {
	var result []string
	// Simple split on comma, not handling all edge cases
	for _, addr := range strings.Split(addresses, ",") {
		addr = strings.TrimSpace(addr)
		if addr != "" {
			_, email := ParseFromHeader(addr)
			if email != "" {
				result = append(result, email)
			}
		}
	}
	return result
}

func checkForAttachments(part *gmail.MessagePart) bool {
	if part.Filename != "" && part.Body != nil && part.Body.AttachmentId != "" {
		return true
	}
	for _, p := range part.Parts {
		if checkForAttachments(p) {
			return true
		}
	}
	return false
}

func normalizeSubject(subject string) string {
	// Remove Re:, Fwd:, etc. prefixes
	subject = strings.TrimSpace(subject)
	prefixes := []string{"Re:", "RE:", "Fwd:", "FWD:", "Fw:"}
	for _, prefix := range prefixes {
		for strings.HasPrefix(subject, prefix) {
			subject = strings.TrimSpace(strings.TrimPrefix(subject, prefix))
		}
	}
	return subject
}

func getTopItems(counts map[string]int, n int) []string {
	if len(counts) == 0 {
		return nil
	}

	// Simple implementation - just get n items
	result := make([]string, 0, n)
	for item := range counts {
		result = append(result, item)
		if len(result) >= n {
			break
		}
	}
	return result
}

// SearchResult represents a search result with summary info
type SearchResult struct {
	ID             string    `json:"id"`
	ThreadID       string    `json:"thread_id"`
	Subject        string    `json:"subject"`
	From           string    `json:"from"`
	Date           time.Time `json:"date"`
	Snippet        string    `json:"snippet"`
	Labels         []string  `json:"labels,omitempty"`
	HasAttachments bool      `json:"has_attachments"`
}

// ToSearchResults converts emails to search result format
func ToSearchResults(emails []Email) []SearchResult {
	results := make([]SearchResult, len(emails))
	for i, e := range emails {
		from := e.FromEmail
		if e.FromName != "" {
			from = fmt.Sprintf("%s <%s>", e.FromName, e.FromEmail)
		}
		results[i] = SearchResult{
			ID:             e.GmailID,
			ThreadID:       e.ThreadID,
			Subject:        e.Subject,
			From:           from,
			Date:           e.Date,
			Snippet:        e.Snippet,
			Labels:         e.Labels,
			HasAttachments: e.HasAttachments,
		}
	}
	return results
}

// ToJSON converts an object to JSON string
func ToJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	return string(b)
}
