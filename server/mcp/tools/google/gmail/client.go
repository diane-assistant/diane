package gmail

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/diane-assistant/diane/mcp/tools/google/auth"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// Client wraps the Gmail API service
type Client struct {
	srv     *gmail.Service
	account string
}

// NewClient creates a new Gmail API client
// It looks for credentials in:
// 1. ~/.diane/secrets/google/token_{account}.json
// 2. ~/.config/gog/tokens/{account}.json (backward compatibility)
func NewClient(account string) (*Client, error) {
	if account == "" {
		account = "default"
	}

	ctx := context.Background()

	// Get token source using shared auth package
	tokenSource, err := auth.GetTokenSource(ctx, account,
		gmail.GmailReadonlyScope,
		gmail.GmailModifyScope,
		gmail.GmailLabelsScope,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get token source: %w", err)
	}

	// Create Gmail service
	srv, err := gmail.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gmail service: %w", err)
	}

	return &Client{srv: srv, account: account}, nil
}

// ListMessages lists message IDs matching a query
func (c *Client) ListMessages(query string, maxResults int64) ([]*gmail.Message, error) {
	var messages []*gmail.Message
	pageToken := ""

	for {
		req := c.srv.Users.Messages.List("me").Q(query)
		if pageToken != "" {
			req = req.PageToken(pageToken)
		}
		if maxResults > 0 && int64(len(messages)) >= maxResults {
			break
		}

		resp, err := req.Do()
		if err != nil {
			return nil, fmt.Errorf("failed to list messages: %w", err)
		}

		messages = append(messages, resp.Messages...)

		if resp.NextPageToken == "" || (maxResults > 0 && int64(len(messages)) >= maxResults) {
			break
		}
		pageToken = resp.NextPageToken
	}

	// Trim to maxResults
	if maxResults > 0 && int64(len(messages)) > maxResults {
		messages = messages[:maxResults]
	}

	return messages, nil
}

// GetMessage retrieves a single message with specified format
func (c *Client) GetMessage(id string, format string) (*gmail.Message, error) {
	if format == "" {
		format = "full"
	}

	msg, err := c.srv.Users.Messages.Get("me", id).Format(format).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get message %s: %w", id, err)
	}

	return msg, nil
}

// BatchGetMessages retrieves multiple messages efficiently
// Gmail API doesn't have a true batch get, but we can use goroutines for parallel fetching
func (c *Client) BatchGetMessages(ids []string, format string) ([]*gmail.Message, error) {
	if format == "" {
		format = "metadata"
	}

	type result struct {
		index int
		msg   *gmail.Message
		err   error
	}

	results := make(chan result, len(ids))

	// Fetch in parallel (limit concurrency to avoid rate limits)
	sem := make(chan struct{}, 10) // max 10 concurrent requests

	for i, id := range ids {
		go func(idx int, msgID string) {
			sem <- struct{}{}        // acquire
			defer func() { <-sem }() // release

			msg, err := c.srv.Users.Messages.Get("me", msgID).Format(format).Do()
			results <- result{index: idx, msg: msg, err: err}
		}(i, id)
	}

	// Collect results
	messages := make([]*gmail.Message, len(ids))
	var firstErr error

	for i := 0; i < len(ids); i++ {
		r := <-results
		if r.err != nil && firstErr == nil {
			firstErr = r.err
		}
		messages[r.index] = r.msg
	}

	return messages, firstErr
}

// GetAttachment retrieves an attachment
func (c *Client) GetAttachment(messageID, attachmentID string) ([]byte, error) {
	att, err := c.srv.Users.Messages.Attachments.Get("me", messageID, attachmentID).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get attachment: %w", err)
	}

	// Decode base64url data
	data, err := decodeBase64URL(att.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode attachment data: %w", err)
	}

	return data, nil
}

// ModifyLabels adds/removes labels from messages
func (c *Client) ModifyLabels(ids []string, addLabels, removeLabels []string) error {
	req := &gmail.BatchModifyMessagesRequest{
		Ids:            ids,
		AddLabelIds:    addLabels,
		RemoveLabelIds: removeLabels,
	}

	err := c.srv.Users.Messages.BatchModify("me", req).Do()
	if err != nil {
		return fmt.Errorf("failed to batch modify labels: %w", err)
	}

	return nil
}

// ListLabels returns all labels for the account
func (c *Client) ListLabels() ([]*gmail.Label, error) {
	resp, err := c.srv.Users.Labels.List("me").Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list labels: %w", err)
	}

	return resp.Labels, nil
}

// CreateLabel creates a new label
func (c *Client) CreateLabel(name string) (*gmail.Label, error) {
	label := &gmail.Label{
		Name:                  name,
		LabelListVisibility:   "labelShow",
		MessageListVisibility: "show",
	}

	created, err := c.srv.Users.Labels.Create("me", label).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create label: %w", err)
	}

	return created, nil
}

// GetHistory retrieves history since a given historyId
func (c *Client) GetHistory(startHistoryID uint64) ([]*gmail.History, error) {
	var histories []*gmail.History
	pageToken := ""

	for {
		req := c.srv.Users.History.List("me").StartHistoryId(startHistoryID)
		if pageToken != "" {
			req = req.PageToken(pageToken)
		}

		resp, err := req.Do()
		if err != nil {
			return nil, fmt.Errorf("failed to get history: %w", err)
		}

		histories = append(histories, resp.History...)

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return histories, nil
}

// GetProfile returns the user's Gmail profile (includes current historyId)
func (c *Client) GetProfile() (*gmail.Profile, error) {
	profile, err := c.srv.Users.GetProfile("me").Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get profile: %w", err)
	}

	return profile, nil
}

// Helper functions

// decodeBase64URL decodes Gmail's base64url encoded data
func decodeBase64URL(data string) ([]byte, error) {
	// Gmail uses URL-safe base64 without padding
	data = strings.ReplaceAll(data, "-", "+")
	data = strings.ReplaceAll(data, "_", "/")

	// Add padding if needed
	switch len(data) % 4 {
	case 2:
		data += "=="
	case 3:
		data += "="
	}

	return io.ReadAll(strings.NewReader(data))
}

// ExtractHeaders extracts common headers from a message
func ExtractHeaders(msg *gmail.Message) map[string]string {
	headers := make(map[string]string)

	if msg.Payload == nil {
		return headers
	}

	for _, h := range msg.Payload.Headers {
		switch strings.ToLower(h.Name) {
		case "from", "to", "cc", "subject", "date", "message-id", "in-reply-to":
			headers[strings.ToLower(h.Name)] = h.Value
		}
	}

	return headers
}

// ParseFromHeader parses "Name <email>" format
func ParseFromHeader(from string) (name, email string) {
	if idx := strings.Index(from, "<"); idx >= 0 {
		name = strings.TrimSpace(from[:idx])
		email = strings.Trim(from[idx:], "<> ")
	} else {
		email = strings.TrimSpace(from)
	}
	// Remove quotes from name
	name = strings.Trim(name, `"'`)
	return
}
