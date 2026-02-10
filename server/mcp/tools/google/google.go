// Package google provides MCP tools for Google services (Gmail, Drive, Sheets, Calendar)
package google

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/diane-assistant/diane/mcp/tools/google/calendar"
	"github.com/diane-assistant/diane/mcp/tools/google/drive"
	"github.com/diane-assistant/diane/mcp/tools/google/gmail"
	"github.com/diane-assistant/diane/mcp/tools/google/sheets"
	gcal "google.golang.org/api/calendar/v3"
)

// --- Helper Functions ---

func getString(args map[string]interface{}, key string) string {
	if val, ok := args[key].(string); ok {
		return val
	}
	return ""
}

func getStringRequired(args map[string]interface{}, key string) (string, error) {
	if val, ok := args[key].(string); ok && val != "" {
		return val, nil
	}
	return "", fmt.Errorf("missing required argument: %s", key)
}

func getInt(args map[string]interface{}, key string, defaultVal int) int {
	if val, ok := args[key].(float64); ok {
		return int(val)
	}
	return defaultVal
}

func getBool(args map[string]interface{}, key string) bool {
	if val, ok := args[key].(bool); ok {
		return val
	}
	return false
}

func textContent(text string) map[string]interface{} {
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": text,
			},
		},
	}
}

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

func numberProperty(description string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "number",
		"description": description,
	}
}

func boolProperty(description string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "boolean",
		"description": description,
	}
}

// --- Tool Definition ---

// Tool represents an MCP tool definition
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// Provider implements ToolProvider for Google services
type Provider struct{}

// NewProvider creates a new Google tools provider
func NewProvider() *Provider {
	return &Provider{}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "google"
}

// CheckDependencies verifies required dependencies exist.
// Since all Google services now use native SDK, no external dependencies are required.
func (p *Provider) CheckDependencies() error {
	// No external dependencies - all services use native Google Go SDK
	return nil
}

// Tools returns all Google tools
func (p *Provider) Tools() []Tool {
	return []Tool{
		// Gmail tools
		{
			Name:        "google_search_emails",
			Description: "Search Gmail messages using Gmail search syntax. Returns metadata (id, subject, from, date, snippet, labels). For classification workflows, prefer gmail_search_and_fetch which returns the same data but is optimized for batch operations.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"query":   stringProperty("Gmail search query (e.g., 'from:alice', 'is:unread', 'label:inbox', 'subject:meeting')"),
					"max":     numberProperty("Maximum number of results to return (default: 10)"),
					"account": stringProperty("Email account to search (optional, uses default if omitted)"),
				},
				[]string{"query"},
			),
		},
		{
			Name:        "google_read_email",
			Description: "Get full content of a specific Gmail message by its ID. Returns complete message with body, headers, and attachments info.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"id":      stringProperty("The message ID to retrieve"),
					"account": stringProperty("Email account to use (optional, uses default if omitted)"),
				},
				[]string{"id"},
			),
		},
		{
			Name:        "gmail_batch_get_messages",
			Description: "Get metadata or full content for multiple Gmail messages in parallel. Uses 10 concurrent requests for speed. Supports up to 100 IDs per call efficiently.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"ids":     stringProperty("Comma-separated list of message IDs to retrieve"),
					"format":  stringProperty("Message format: 'metadata' (headers only, fast) or 'full' (with body). Default: metadata"),
					"account": stringProperty("Email account to use (optional)"),
				},
				[]string{"ids"},
			),
		},
		{
			Name:        "gmail_batch_modify_labels",
			Description: "Add or remove labels from multiple Gmail messages in one API call. Handles up to 1000 IDs per call (Gmail API limit). For bulk operations by query, use gmail_batch_label_by_query instead.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"ids":     stringProperty("Comma-separated list of message IDs to modify"),
					"add":     stringProperty("Comma-separated labels to add (e.g., 'diane-processed,Important')"),
					"remove":  stringProperty("Comma-separated labels to remove (e.g., 'INBOX' to archive, 'UNREAD' to mark read)"),
					"account": stringProperty("Email account to use (optional)"),
				},
				[]string{"ids"},
			),
		},
		{
			Name:        "gmail_list_labels",
			Description: "List all Gmail labels for an account. Returns label IDs, names, and types. Use this to map existing Gmail organization.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"account": stringProperty("Email account to use (optional)"),
				},
				nil,
			),
		},
		{
			Name:        "gmail_create_label",
			Description: "Create a new Gmail label. Use for creating organizational labels like 'diane-processed' or 'diane-ignored'.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"name":    stringProperty("Name of the label to create (can include / for nested labels)"),
					"account": stringProperty("Email account to use (optional)"),
				},
				[]string{"name"},
			),
		},
		{
			Name:        "gmail_analyze_sender",
			Description: "Get statistical profile of a sender: email count, date range, common subjects. Useful for understanding email patterns. Supports flexible matching for forwarded/aliased emails.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"sender":  stringProperty("Email address or partial address to analyze (e.g., 'allegro@allegro.pl' or just 'allegro')"),
					"query":   stringProperty("Custom Gmail search query instead of auto-generated from: query. Use this for complex cases like forwarded emails."),
					"max":     numberProperty("Maximum emails to analyze (default: 100)"),
					"account": stringProperty("Email account to use (optional)"),
				},
				nil, // No required fields - either sender or query must be provided
			),
		},
		// New cached Gmail tools
		{
			Name:        "gmail_sync",
			Description: "Sync Gmail messages to local cache. Uses History API for efficient incremental updates. Run this periodically to keep cache fresh.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"max":     numberProperty("Maximum messages to sync on full sync (default: 500)"),
					"force":   boolProperty("Force a full sync even if incremental is available"),
					"account": stringProperty("Email account to use (optional)"),
				},
				nil,
			),
		},
		{
			Name:        "gmail_cache_stats",
			Description: "Get statistics about the local Gmail cache: message count, date range, sync status.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"account": stringProperty("Email account to use (optional)"),
				},
				nil,
			),
		},
		{
			Name:        "gmail_list_attachments",
			Description: "List attachments for a Gmail message. Returns attachment metadata including ID, filename, MIME type, size, and download status.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"message_id": stringProperty("Gmail message ID"),
					"account":    stringProperty("Email account to use (optional)"),
				},
				[]string{"message_id"},
			),
		},
		{
			Name:        "gmail_download_attachment",
			Description: "Download an attachment from a Gmail message to local storage. Returns the local file path.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"message_id":    stringProperty("Gmail message ID"),
					"attachment_id": stringProperty("Attachment ID (from gmail_list_attachments)"),
					"account":       stringProperty("Email account to use (optional)"),
				},
				[]string{"message_id", "attachment_id"},
			),
		},
		{
			Name:        "gmail_get_content",
			Description: "Get the full content of an email including extracted plain text and JSON-LD structured data (orders, shipping, etc).",
			InputSchema: objectSchema(
				map[string]interface{}{
					"message_id": stringProperty("Gmail message ID"),
					"account":    stringProperty("Email account to use (optional)"),
				},
				[]string{"message_id"},
			),
		},
		// New composite tools for speed optimization
		{
			Name:        "gmail_search_and_fetch",
			Description: "Search emails AND return full metadata in one call. Returns: id, subject, from, date, snippet, labels, has_attachments. Use this instead of search + batch_get for classification workflows. Eliminates the 2-step bottleneck.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"query":   stringProperty("Gmail search query (e.g., 'is:unread newer_than:7d', 'from:newsletter@example.com')"),
					"max":     numberProperty("Maximum results (default: 50, max: 500)"),
					"account": stringProperty("Email account to use (optional)"),
				},
				[]string{"query"},
			),
		},
		{
			Name:        "gmail_batch_label_by_query",
			Description: "Apply labels to ALL emails matching a search query in one call. Use for bulk operations like 'label all newsletters as Ignored'. Automatically handles pagination and chunking.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"query":   stringProperty("Gmail search query (e.g., 'from:newsletter@spam.com', 'is:unread older_than:30d')"),
					"add":     stringProperty("Labels to add (comma-separated, e.g., 'diane-processed,Archived')"),
					"remove":  stringProperty("Labels to remove (comma-separated, e.g., 'INBOX,UNREAD')"),
					"max":     numberProperty("Safety limit on messages to modify (default: 500, max: 5000)"),
					"account": stringProperty("Email account to use (optional)"),
				},
				[]string{"query"},
			),
		},
		// Drive tools
		{
			Name:        "google_search_files",
			Description: "Search Google Drive for files and folders using query syntax. Returns file metadata including ID, name, mimeType, and shared status.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"query":   stringProperty("Search query (e.g., 'name contains \"report\"', 'mimeType = \"application/vnd.google-apps.spreadsheet\"')"),
					"max":     numberProperty("Maximum number of results (default: 10)"),
					"account": stringProperty("Google account email to use (optional)"),
				},
				[]string{"query"},
			),
		},
		{
			Name:        "google_list_files",
			Description: "List recent files from Google Drive with optional filtering. Simpler than search for basic listing.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"max":     numberProperty("Maximum number of results (default: 20)"),
					"account": stringProperty("Google account email to use (optional)"),
				},
				nil,
			),
		},
		// Sheets tools
		{
			Name:        "google_get_sheet",
			Description: "Get data from a Google Sheet range. Returns cell values in JSON format.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"sheetId": stringProperty("The Google Sheets ID (from the URL)"),
					"range":   stringProperty("Range in A1 notation (e.g., 'Sheet1!A1:D10' or 'Tab!A:C')"),
					"account": stringProperty("Google account email to use (optional)"),
				},
				[]string{"sheetId", "range"},
			),
		},
		{
			Name:        "google_update_sheet",
			Description: "Update data in a Google Sheet range. Overwrites existing values.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"sheetId": stringProperty("The Google Sheets ID (from the URL)"),
					"range":   stringProperty("Range in A1 notation (e.g., 'Sheet1!A1:B2')"),
					"values":  stringProperty("JSON array of arrays with cell values (e.g., '[[\"A\",\"B\"],[\"1\",\"2\"]]')"),
					"account": stringProperty("Google account email to use (optional)"),
				},
				[]string{"sheetId", "range", "values"},
			),
		},
		{
			Name:        "google_append_sheet",
			Description: "Append data to a Google Sheet. Adds new rows at the end of the range.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"sheetId": stringProperty("The Google Sheets ID (from the URL)"),
					"range":   stringProperty("Range in A1 notation specifying columns (e.g., 'Sheet1!A:C')"),
					"values":  stringProperty("JSON array of arrays with row values (e.g., '[[\"x\",\"y\",\"z\"]]')"),
					"account": stringProperty("Google account email to use (optional)"),
				},
				[]string{"sheetId", "range", "values"},
			),
		},
		{
			Name:        "google_clear_sheet",
			Description: "Clear data from a Google Sheet range without deleting the cells.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"sheetId": stringProperty("The Google Sheets ID (from the URL)"),
					"range":   stringProperty("Range in A1 notation (e.g., 'Sheet1!A2:Z' or 'Tab!B5:D10')"),
					"account": stringProperty("Google account email to use (optional)"),
				},
				[]string{"sheetId", "range"},
			),
		},
		{
			Name:        "google_get_sheet_metadata",
			Description: "Get metadata about a Google Sheet including sheet tabs, properties, and structure.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"sheetId": stringProperty("The Google Sheets ID (from the URL)"),
					"account": stringProperty("Google account email to use (optional)"),
				},
				[]string{"sheetId"},
			),
		},
		// Calendar tools
		{
			Name:        "google_list_calendars",
			Description: "List all calendars for a Google account",
			InputSchema: objectSchema(
				map[string]interface{}{
					"account": stringProperty("Account email (optional - if omitted, uses primary account)"),
				},
				nil,
			),
		},
		{
			Name:        "google_list_events",
			Description: "List events from a Google Calendar with flexible time filtering",
			InputSchema: objectSchema(
				map[string]interface{}{
					"calendar_id": stringProperty("Calendar ID (use 'primary' for main calendar, or specific calendar ID)"),
					"account":     stringProperty("Account email (optional)"),
					"from":        stringProperty("Start time (RFC3339, date YYYY-MM-DD, or relative: today, tomorrow, monday)"),
					"to":          stringProperty("End time (RFC3339, date YYYY-MM-DD, or relative)"),
					"today":       boolProperty("Show only today's events (timezone-aware)"),
					"tomorrow":    boolProperty("Show only tomorrow's events (timezone-aware)"),
					"week":        boolProperty("Show this week's events (Monday-Sunday)"),
					"days":        numberProperty("Show next N days of events (timezone-aware)"),
					"max":         numberProperty("Maximum number of events to return (default: 10)"),
					"query":       stringProperty("Free text search query to filter events"),
					"all":         boolProperty("Fetch events from all calendars (not just one)"),
				},
				nil,
			),
		},
		{
			Name:        "google_get_event",
			Description: "Get details of a specific calendar event",
			InputSchema: objectSchema(
				map[string]interface{}{
					"calendar_id": stringProperty("Calendar ID (use 'primary' for main calendar)"),
					"event_id":    stringProperty("Event ID"),
					"account":     stringProperty("Account email (optional)"),
				},
				[]string{"calendar_id", "event_id"},
			),
		},
		{
			Name:        "google_create_event",
			Description: "Create a new event in Google Calendar",
			InputSchema: objectSchema(
				map[string]interface{}{
					"calendar_id":  stringProperty("Calendar ID (use 'primary' for main calendar)"),
					"summary":      stringProperty("Event title/summary"),
					"from":         stringProperty("Start time (RFC3339 format: 2026-02-04T15:30:00+01:00 or date for all-day: 2026-02-04)"),
					"to":           stringProperty("End time (RFC3339 format or date for all-day)"),
					"account":      stringProperty("Account email (optional)"),
					"description":  stringProperty("Event description"),
					"location":     stringProperty("Event location"),
					"attendees":    stringProperty("Comma-separated list of attendee emails"),
					"all_day":      boolProperty("Is this an all-day event? (use date-only format in from/to)"),
					"reminder":     stringProperty("Reminder in format 'method:duration' (e.g., 'popup:30m', 'email:1d')"),
					"with_meet":    boolProperty("Create a Google Meet video conference link"),
					"visibility":   stringProperty("Event visibility: default, public, private, confidential"),
					"transparency": stringProperty("Show as busy (opaque) or free (transparent). Use: busy or free"),
				},
				[]string{"calendar_id", "summary", "from", "to"},
			),
		},
		{
			Name:        "google_update_event",
			Description: "Update an existing calendar event",
			InputSchema: objectSchema(
				map[string]interface{}{
					"calendar_id": stringProperty("Calendar ID"),
					"event_id":    stringProperty("Event ID to update"),
					"account":     stringProperty("Account email (optional)"),
					"summary":     stringProperty("New event title/summary"),
					"from":        stringProperty("New start time (RFC3339 format)"),
					"to":          stringProperty("New end time (RFC3339 format)"),
					"description": stringProperty("New event description"),
					"location":    stringProperty("New event location"),
				},
				[]string{"calendar_id", "event_id"},
			),
		},
		{
			Name:        "google_delete_event",
			Description: "Delete a calendar event",
			InputSchema: objectSchema(
				map[string]interface{}{
					"calendar_id": stringProperty("Calendar ID"),
					"event_id":    stringProperty("Event ID to delete"),
					"account":     stringProperty("Account email (optional)"),
				},
				[]string{"calendar_id", "event_id"},
			),
		},
		{
			Name:        "google_check_freebusy",
			Description: "Check free/busy status for one or more calendars",
			InputSchema: objectSchema(
				map[string]interface{}{
					"calendar_ids": stringProperty("Comma-separated list of calendar IDs to check"),
					"from":         stringProperty("Start time (RFC3339 format)"),
					"to":           stringProperty("End time (RFC3339 format)"),
					"account":      stringProperty("Account email (optional)"),
				},
				[]string{"calendar_ids", "from", "to"},
			),
		},
	}
}

// HasTool checks if a tool name belongs to this provider
func (p *Provider) HasTool(name string) bool {
	for _, tool := range p.Tools() {
		if tool.Name == name {
			return true
		}
	}
	return false
}

// Call executes a tool by name
func (p *Provider) Call(name string, args map[string]interface{}) (interface{}, error) {
	switch name {
	// Gmail
	case "google_search_emails":
		return p.searchEmails(args)
	case "google_read_email":
		return p.readEmail(args)
	case "gmail_batch_get_messages":
		return p.batchGetMessages(args)
	case "gmail_batch_modify_labels":
		return p.batchModifyLabels(args)
	case "gmail_list_labels":
		return p.listLabels(args)
	case "gmail_create_label":
		return p.createLabel(args)
	case "gmail_analyze_sender":
		return p.analyzeSender(args)
	// New cached Gmail tools
	case "gmail_sync":
		return p.gmailSync(args)
	case "gmail_cache_stats":
		return p.gmailCacheStats(args)
	case "gmail_list_attachments":
		return p.gmailListAttachments(args)
	case "gmail_download_attachment":
		return p.gmailDownloadAttachment(args)
	case "gmail_get_content":
		return p.gmailGetContent(args)
	// New composite tools
	case "gmail_search_and_fetch":
		return p.gmailSearchAndFetch(args)
	case "gmail_batch_label_by_query":
		return p.gmailBatchLabelByQuery(args)
	// Drive
	case "google_search_files":
		return p.searchFiles(args)
	case "google_list_files":
		return p.listFiles(args)
	// Sheets
	case "google_get_sheet":
		return p.getSheet(args)
	case "google_update_sheet":
		return p.updateSheet(args)
	case "google_append_sheet":
		return p.appendSheet(args)
	case "google_clear_sheet":
		return p.clearSheet(args)
	case "google_get_sheet_metadata":
		return p.getSheetMetadata(args)
	// Calendar
	case "google_list_calendars":
		return p.listCalendars(args)
	case "google_list_events":
		return p.listEvents(args)
	case "google_get_event":
		return p.getEvent(args)
	case "google_create_event":
		return p.createEvent(args)
	case "google_update_event":
		return p.updateEvent(args)
	case "google_delete_event":
		return p.deleteEvent(args)
	case "google_check_freebusy":
		return p.checkFreebusy(args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// --- Gmail Tools ---

func (p *Provider) searchEmails(args map[string]interface{}) (interface{}, error) {
	query, err := getStringRequired(args, "query")
	if err != nil {
		return nil, err
	}

	max := getInt(args, "max", 10)
	account := getString(args, "account")

	svc, err := gmail.NewService(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create gmail service: %w", err)
	}
	defer svc.Close()

	emails, err := svc.SearchMessages(query, int64(max))
	if err != nil {
		return nil, fmt.Errorf("failed to search emails: %w", err)
	}

	results := gmail.ToSearchResults(emails)
	return textContent(gmail.ToJSON(results)), nil
}

func (p *Provider) readEmail(args map[string]interface{}) (interface{}, error) {
	id, err := getStringRequired(args, "id")
	if err != nil {
		return nil, err
	}

	account := getString(args, "account")

	svc, err := gmail.NewService(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create gmail service: %w", err)
	}
	defer svc.Close()

	email, err := svc.GetMessage(id, true) // withContent=true
	if err != nil {
		return nil, fmt.Errorf("failed to read email: %w", err)
	}

	return textContent(gmail.ToJSON(email)), nil
}

func (p *Provider) batchGetMessages(args map[string]interface{}) (interface{}, error) {
	idsStr, err := getStringRequired(args, "ids")
	if err != nil {
		return nil, err
	}

	format := getString(args, "format")
	if format == "" {
		format = "metadata"
	}
	if format != "metadata" && format != "full" {
		return nil, fmt.Errorf("format must be 'metadata' or 'full', got: %s", format)
	}

	account := getString(args, "account")

	svc, err := gmail.NewService(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create gmail service: %w", err)
	}
	defer svc.Close()

	// Split IDs and fetch messages using parallel SDK
	ids := strings.Split(idsStr, ",")
	cleanIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			cleanIDs = append(cleanIDs, id)
		}
	}

	if len(cleanIDs) == 0 {
		return textContent("[]"), nil
	}

	withContent := format == "full"
	emails, err := svc.BatchGetMessages(cleanIDs, withContent)
	if err != nil {
		return nil, fmt.Errorf("failed to batch get messages: %w", err)
	}

	return textContent(gmail.ToJSON(emails)), nil
}

func (p *Provider) batchModifyLabels(args map[string]interface{}) (interface{}, error) {
	idsStr, err := getStringRequired(args, "ids")
	if err != nil {
		return nil, err
	}

	addLabelsStr := getString(args, "add")
	removeLabelsStr := getString(args, "remove")
	account := getString(args, "account")

	if addLabelsStr == "" && removeLabelsStr == "" {
		return nil, fmt.Errorf("at least one of 'add' or 'remove' must be specified")
	}

	// Parse comma-separated IDs
	ids := strings.Split(idsStr, ",")
	cleanIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			cleanIDs = append(cleanIDs, id)
		}
	}

	if len(cleanIDs) == 0 {
		return nil, fmt.Errorf("no valid message IDs provided")
	}

	// Parse labels
	var addLabels, removeLabels []string
	if addLabelsStr != "" {
		for _, l := range strings.Split(addLabelsStr, ",") {
			if l = strings.TrimSpace(l); l != "" {
				addLabels = append(addLabels, l)
			}
		}
	}
	if removeLabelsStr != "" {
		for _, l := range strings.Split(removeLabelsStr, ",") {
			if l = strings.TrimSpace(l); l != "" {
				removeLabels = append(removeLabels, l)
			}
		}
	}

	svc, err := gmail.NewService(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create gmail service: %w", err)
	}
	defer svc.Close()

	// Batch modify in chunks of 1000 (Gmail API limit)
	const chunkSize = 1000
	modified := 0
	for i := 0; i < len(cleanIDs); i += chunkSize {
		end := i + chunkSize
		if end > len(cleanIDs) {
			end = len(cleanIDs)
		}
		chunk := cleanIDs[i:end]

		if err := svc.ModifyLabels(chunk, addLabels, removeLabels); err != nil {
			return nil, fmt.Errorf("failed to modify labels: %w", err)
		}
		modified += len(chunk)
	}

	result := map[string]interface{}{
		"modified": modified,
		"added":    addLabels,
		"removed":  removeLabels,
	}
	return textContent(gmail.ToJSON(result)), nil
}

func (p *Provider) listLabels(args map[string]interface{}) (interface{}, error) {
	account := getString(args, "account")

	svc, err := gmail.NewService(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create gmail service: %w", err)
	}
	defer svc.Close()

	labels, err := svc.ListLabels()
	if err != nil {
		return nil, fmt.Errorf("failed to list labels: %w", err)
	}

	// Convert to simpler format for JSON output
	type LabelInfo struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	}
	result := make([]LabelInfo, len(labels))
	for i, l := range labels {
		result[i] = LabelInfo{
			ID:   l.Id,
			Name: l.Name,
			Type: l.Type,
		}
	}

	return textContent(gmail.ToJSON(result)), nil
}

func (p *Provider) createLabel(args map[string]interface{}) (interface{}, error) {
	name, err := getStringRequired(args, "name")
	if err != nil {
		return nil, err
	}

	account := getString(args, "account")

	svc, err := gmail.NewService(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create gmail service: %w", err)
	}
	defer svc.Close()

	label, err := svc.CreateLabel(name)
	if err != nil {
		return nil, fmt.Errorf("failed to create label: %w", err)
	}

	result := map[string]string{
		"id":   label.Id,
		"name": label.Name,
		"type": label.Type,
	}
	return textContent(gmail.ToJSON(result)), nil
}

func (p *Provider) analyzeSender(args map[string]interface{}) (interface{}, error) {
	sender := getString(args, "sender")
	customQuery := getString(args, "query")
	max := getInt(args, "max", 100)
	account := getString(args, "account")

	if sender == "" && customQuery == "" {
		return nil, fmt.Errorf("either 'sender' or 'query' must be provided")
	}

	svc, err := gmail.NewService(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create gmail service: %w", err)
	}
	defer svc.Close()

	// Use custom query or build from sender
	senderPattern := sender
	if sender == "" {
		senderPattern = customQuery // Use query as pattern for stats
	}

	stats, err := svc.GetSenderStats(senderPattern, max)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze sender: %w", err)
	}

	if stats == nil {
		return textContent(`{"message": "No emails found for this sender"}`), nil
	}

	return textContent(gmail.ToJSON(stats)), nil
}

// --- New Cached Gmail Tools ---

func (p *Provider) gmailSync(args map[string]interface{}) (interface{}, error) {
	account := getString(args, "account")
	maxMessages := int64(getInt(args, "max", 500))
	force := getBool(args, "force")

	svc, err := gmail.NewService(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create gmail service: %w", err)
	}
	defer svc.Close()

	var result *gmail.SyncResult
	if force {
		result, err = svc.ForceFullSync(maxMessages)
	} else {
		result, err = svc.Sync(maxMessages)
	}
	if err != nil {
		return nil, fmt.Errorf("sync failed: %w", err)
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return textContent(string(output)), nil
}

func (p *Provider) gmailCacheStats(args map[string]interface{}) (interface{}, error) {
	account := getString(args, "account")

	svc, err := gmail.NewService(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create gmail service: %w", err)
	}
	defer svc.Close()

	stats, err := svc.GetCacheStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get cache stats: %w", err)
	}

	if stats == nil {
		return textContent(`{"error": "cache not available"}`), nil
	}

	output, _ := json.MarshalIndent(stats, "", "  ")
	return textContent(string(output)), nil
}

func (p *Provider) gmailListAttachments(args map[string]interface{}) (interface{}, error) {
	messageID, err := getStringRequired(args, "message_id")
	if err != nil {
		return nil, err
	}
	account := getString(args, "account")

	svc, err := gmail.NewService(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create gmail service: %w", err)
	}
	defer svc.Close()

	attachments, err := svc.GetAttachmentInfo(messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to list attachments: %w", err)
	}

	output, _ := json.MarshalIndent(attachments, "", "  ")
	return textContent(string(output)), nil
}

func (p *Provider) gmailDownloadAttachment(args map[string]interface{}) (interface{}, error) {
	messageID, err := getStringRequired(args, "message_id")
	if err != nil {
		return nil, err
	}
	attachmentID, err := getStringRequired(args, "attachment_id")
	if err != nil {
		return nil, err
	}
	account := getString(args, "account")

	svc, err := gmail.NewService(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create gmail service: %w", err)
	}
	defer svc.Close()

	localPath, err := svc.DownloadAttachment(messageID, attachmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to download attachment: %w", err)
	}

	result := map[string]string{
		"message_id":    messageID,
		"attachment_id": attachmentID,
		"local_path":    localPath,
	}
	output, _ := json.MarshalIndent(result, "", "  ")
	return textContent(string(output)), nil
}

func (p *Provider) gmailGetContent(args map[string]interface{}) (interface{}, error) {
	messageID, err := getStringRequired(args, "message_id")
	if err != nil {
		return nil, err
	}
	account := getString(args, "account")

	svc, err := gmail.NewService(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create gmail service: %w", err)
	}
	defer svc.Close()

	email, err := svc.GetMessageContent(messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get message content: %w", err)
	}

	// Return a focused view of the content
	type ContentResult struct {
		ID          string   `json:"id"`
		Subject     string   `json:"subject"`
		From        string   `json:"from"`
		Date        string   `json:"date"`
		PlainText   string   `json:"plain_text,omitempty"`
		JsonLD      []any    `json:"json_ld,omitempty"`
		JsonLDTypes []string `json:"json_ld_types,omitempty"`
		Labels      []string `json:"labels,omitempty"`
	}

	result := ContentResult{
		ID:      email.GmailID,
		Subject: email.Subject,
		From:    fmt.Sprintf("%s <%s>", email.FromName, email.FromEmail),
		Date:    email.Date.Format("2006-01-02 15:04:05"),
		JsonLD:  email.JsonLD,
		Labels:  email.Labels,
	}

	if email.PlainText != nil {
		result.PlainText = *email.PlainText
	}
	if len(email.JsonLD) > 0 {
		result.JsonLDTypes = gmail.GetJsonLDTypes(email.JsonLD)
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return textContent(string(output)), nil
}

// --- New Composite Gmail Tools ---

func (p *Provider) gmailSearchAndFetch(args map[string]interface{}) (interface{}, error) {
	query, err := getStringRequired(args, "query")
	if err != nil {
		return nil, err
	}

	max := getInt(args, "max", 50)
	if max > 500 {
		max = 500
	}
	account := getString(args, "account")

	svc, err := gmail.NewService(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create gmail service: %w", err)
	}
	defer svc.Close()

	// Search and fetch metadata in one operation (uses cache when available)
	emails, err := svc.SearchMessages(query, int64(max))
	if err != nil {
		return nil, fmt.Errorf("failed to search and fetch emails: %w", err)
	}

	// Convert to search result format with full metadata
	results := gmail.ToSearchResults(emails)

	return textContent(gmail.ToJSON(results)), nil
}

func (p *Provider) gmailBatchLabelByQuery(args map[string]interface{}) (interface{}, error) {
	query, err := getStringRequired(args, "query")
	if err != nil {
		return nil, err
	}

	addLabelsStr := getString(args, "add")
	removeLabelsStr := getString(args, "remove")
	account := getString(args, "account")

	if addLabelsStr == "" && removeLabelsStr == "" {
		return nil, fmt.Errorf("at least one of 'add' or 'remove' must be specified")
	}

	max := getInt(args, "max", 500)
	if max > 5000 {
		max = 5000
	}

	// Parse labels
	var addLabels, removeLabels []string
	if addLabelsStr != "" {
		for _, l := range strings.Split(addLabelsStr, ",") {
			if l = strings.TrimSpace(l); l != "" {
				addLabels = append(addLabels, l)
			}
		}
	}
	if removeLabelsStr != "" {
		for _, l := range strings.Split(removeLabelsStr, ",") {
			if l = strings.TrimSpace(l); l != "" {
				removeLabels = append(removeLabels, l)
			}
		}
	}

	svc, err := gmail.NewService(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create gmail service: %w", err)
	}
	defer svc.Close()

	// Search for all matching messages
	emails, err := svc.SearchMessages(query, int64(max))
	if err != nil {
		return nil, fmt.Errorf("failed to search emails: %w", err)
	}

	if len(emails) == 0 {
		result := map[string]interface{}{
			"modified": 0,
			"message":  "No emails matched the query",
			"query":    query,
		}
		return textContent(gmail.ToJSON(result)), nil
	}

	// Collect IDs
	ids := make([]string, len(emails))
	for i, e := range emails {
		ids[i] = e.GmailID
	}

	// Batch modify in chunks of 1000 (Gmail API limit)
	const chunkSize = 1000
	modified := 0
	for i := 0; i < len(ids); i += chunkSize {
		end := i + chunkSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[i:end]

		if err := svc.ModifyLabels(chunk, addLabels, removeLabels); err != nil {
			return nil, fmt.Errorf("failed to modify labels (modified %d before error): %w", modified, err)
		}
		modified += len(chunk)
	}

	result := map[string]interface{}{
		"modified": modified,
		"query":    query,
		"added":    addLabels,
		"removed":  removeLabels,
	}
	return textContent(gmail.ToJSON(result)), nil
}

// --- Drive Tools ---

func (p *Provider) searchFiles(args map[string]interface{}) (interface{}, error) {
	query, err := getStringRequired(args, "query")
	if err != nil {
		return nil, err
	}

	max := getInt(args, "max", 10)
	account := getString(args, "account")

	client, err := drive.NewClient(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create Drive client: %w", err)
	}

	files, err := client.SearchFiles(query, int64(max))
	if err != nil {
		return nil, fmt.Errorf("failed to search files: %w", err)
	}

	return textContent(drive.ToJSON(files)), nil
}

func (p *Provider) listFiles(args map[string]interface{}) (interface{}, error) {
	max := getInt(args, "max", 20)
	account := getString(args, "account")

	client, err := drive.NewClient(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create Drive client: %w", err)
	}

	files, err := client.ListRecentFiles(int64(max))
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	return textContent(drive.ToJSON(files)), nil
}

// --- Sheets Tools ---

func (p *Provider) getSheet(args map[string]interface{}) (interface{}, error) {
	sheetId, err := getStringRequired(args, "sheetId")
	if err != nil {
		return nil, err
	}
	rangeArg, err := getStringRequired(args, "range")
	if err != nil {
		return nil, err
	}

	account := getString(args, "account")

	client, err := sheets.NewClient(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create Sheets client: %w", err)
	}

	values, err := client.GetRange(sheetId, rangeArg)
	if err != nil {
		return nil, fmt.Errorf("failed to get sheet: %w", err)
	}

	return textContent(sheets.ToJSON(values)), nil
}

func (p *Provider) updateSheet(args map[string]interface{}) (interface{}, error) {
	sheetId, err := getStringRequired(args, "sheetId")
	if err != nil {
		return nil, err
	}
	rangeArg, err := getStringRequired(args, "range")
	if err != nil {
		return nil, err
	}
	valuesStr, err := getStringRequired(args, "values")
	if err != nil {
		return nil, err
	}

	account := getString(args, "account")

	// Parse JSON values
	var values [][]interface{}
	if err := json.Unmarshal([]byte(valuesStr), &values); err != nil {
		return nil, fmt.Errorf("failed to parse values JSON: %w", err)
	}

	client, err := sheets.NewClient(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create Sheets client: %w", err)
	}

	result, err := client.UpdateRange(sheetId, rangeArg, values)
	if err != nil {
		return nil, fmt.Errorf("failed to update sheet: %w", err)
	}

	return textContent(sheets.ToJSON(result)), nil
}

func (p *Provider) appendSheet(args map[string]interface{}) (interface{}, error) {
	sheetId, err := getStringRequired(args, "sheetId")
	if err != nil {
		return nil, err
	}
	rangeArg, err := getStringRequired(args, "range")
	if err != nil {
		return nil, err
	}
	valuesStr, err := getStringRequired(args, "values")
	if err != nil {
		return nil, err
	}

	account := getString(args, "account")

	// Parse JSON values
	var values [][]interface{}
	if err := json.Unmarshal([]byte(valuesStr), &values); err != nil {
		return nil, fmt.Errorf("failed to parse values JSON: %w", err)
	}

	client, err := sheets.NewClient(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create Sheets client: %w", err)
	}

	result, err := client.AppendRows(sheetId, rangeArg, values)
	if err != nil {
		return nil, fmt.Errorf("failed to append to sheet: %w", err)
	}

	return textContent(sheets.ToJSON(result)), nil
}

func (p *Provider) clearSheet(args map[string]interface{}) (interface{}, error) {
	sheetId, err := getStringRequired(args, "sheetId")
	if err != nil {
		return nil, err
	}
	rangeArg, err := getStringRequired(args, "range")
	if err != nil {
		return nil, err
	}

	account := getString(args, "account")

	client, err := sheets.NewClient(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create Sheets client: %w", err)
	}

	result, err := client.ClearRange(sheetId, rangeArg)
	if err != nil {
		return nil, fmt.Errorf("failed to clear sheet: %w", err)
	}

	return textContent(sheets.ToJSON(result)), nil
}

func (p *Provider) getSheetMetadata(args map[string]interface{}) (interface{}, error) {
	sheetId, err := getStringRequired(args, "sheetId")
	if err != nil {
		return nil, err
	}

	account := getString(args, "account")

	client, err := sheets.NewClient(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create Sheets client: %w", err)
	}

	metadata, err := client.GetMetadata(sheetId)
	if err != nil {
		return nil, fmt.Errorf("failed to get sheet metadata: %w", err)
	}

	return textContent(sheets.ToJSON(metadata)), nil
}

// --- Calendar Tools ---

func (p *Provider) listCalendars(args map[string]interface{}) (interface{}, error) {
	account := getString(args, "account")

	client, err := calendar.NewClient(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create Calendar client: %w", err)
	}

	calendars, err := client.ListCalendars()
	if err != nil {
		return nil, fmt.Errorf("failed to list calendars: %w", err)
	}

	return textContent(calendar.ToJSON(calendars)), nil
}

func (p *Provider) listEvents(args map[string]interface{}) (interface{}, error) {
	account := getString(args, "account")
	calendarID := getString(args, "calendar_id")
	if calendarID == "" {
		calendarID = "primary"
	}

	client, err := calendar.NewClient(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create Calendar client: %w", err)
	}

	// Build options
	opts := calendar.ListEventsOptions{
		MaxResults: int64(getInt(args, "max", 10)),
		Query:      getString(args, "query"),
	}

	now := time.Now()

	// Handle time range options
	if getBool(args, "today") {
		opts.TimeMin = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		opts.TimeMax = opts.TimeMin.Add(24 * time.Hour)
	} else if getBool(args, "tomorrow") {
		opts.TimeMin = time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		opts.TimeMax = opts.TimeMin.Add(24 * time.Hour)
	} else if getBool(args, "week") {
		// Find Monday of current week
		daysUntilMonday := int(now.Weekday()) - int(time.Monday)
		if daysUntilMonday < 0 {
			daysUntilMonday += 7
		}
		opts.TimeMin = time.Date(now.Year(), now.Month(), now.Day()-daysUntilMonday, 0, 0, 0, 0, now.Location())
		opts.TimeMax = opts.TimeMin.Add(7 * 24 * time.Hour)
	} else if days := getInt(args, "days", 0); days > 0 {
		opts.TimeMin = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		opts.TimeMax = opts.TimeMin.Add(time.Duration(days) * 24 * time.Hour)
	} else {
		// Parse from/to if provided
		if from := getString(args, "from"); from != "" {
			t, err := calendar.ParseTimeArg(from, false)
			if err != nil {
				return nil, fmt.Errorf("invalid 'from' time: %w", err)
			}
			opts.TimeMin = t
		}
		if to := getString(args, "to"); to != "" {
			t, err := calendar.ParseTimeArg(to, true)
			if err != nil {
				return nil, fmt.Errorf("invalid 'to' time: %w", err)
			}
			opts.TimeMax = t
		}
	}

	// Handle --all flag to fetch from all calendars
	var events []calendar.EventInfo
	if getBool(args, "all") {
		events, err = client.ListAllCalendarEvents(opts)
	} else {
		events, err = client.ListEvents(calendarID, opts)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list events: %w", err)
	}

	return textContent(calendar.ToJSON(events)), nil
}

func (p *Provider) getEvent(args map[string]interface{}) (interface{}, error) {
	calendarID, err := getStringRequired(args, "calendar_id")
	if err != nil {
		return nil, err
	}
	eventID, err := getStringRequired(args, "event_id")
	if err != nil {
		return nil, err
	}

	account := getString(args, "account")

	client, err := calendar.NewClient(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create Calendar client: %w", err)
	}

	event, err := client.GetEvent(calendarID, eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to get event: %w", err)
	}

	return textContent(calendar.ToJSON(event)), nil
}

func (p *Provider) createEvent(args map[string]interface{}) (interface{}, error) {
	calendarID, err := getStringRequired(args, "calendar_id")
	if err != nil {
		return nil, err
	}
	summary, err := getStringRequired(args, "summary")
	if err != nil {
		return nil, err
	}
	from, err := getStringRequired(args, "from")
	if err != nil {
		return nil, err
	}
	to, err := getStringRequired(args, "to")
	if err != nil {
		return nil, err
	}

	account := getString(args, "account")

	client, err := calendar.NewClient(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create Calendar client: %w", err)
	}

	// Build the event
	event := &gcal.Event{
		Summary: summary,
	}

	// Handle all-day events vs timed events
	allDay := getBool(args, "all_day")
	if allDay {
		event.Start = &gcal.EventDateTime{Date: from}
		event.End = &gcal.EventDateTime{Date: to}
	} else {
		event.Start = &gcal.EventDateTime{DateTime: from}
		event.End = &gcal.EventDateTime{DateTime: to}
	}

	// Optional fields
	if description := getString(args, "description"); description != "" {
		event.Description = description
	}
	if location := getString(args, "location"); location != "" {
		event.Location = location
	}
	if visibility := getString(args, "visibility"); visibility != "" {
		event.Visibility = visibility
	}
	if transparency := getString(args, "transparency"); transparency != "" {
		// Map "busy"/"free" to "opaque"/"transparent"
		switch transparency {
		case "busy":
			event.Transparency = "opaque"
		case "free":
			event.Transparency = "transparent"
		default:
			event.Transparency = transparency
		}
	}

	// Handle attendees
	if attendeesStr := getString(args, "attendees"); attendeesStr != "" {
		attendeeEmails := strings.Split(attendeesStr, ",")
		event.Attendees = make([]*gcal.EventAttendee, len(attendeeEmails))
		for i, email := range attendeeEmails {
			event.Attendees[i] = &gcal.EventAttendee{Email: strings.TrimSpace(email)}
		}
	}

	// Handle reminder
	if reminder := getString(args, "reminder"); reminder != "" {
		// Parse reminder format: method:duration (e.g., "popup:30m", "email:1d")
		parts := strings.SplitN(reminder, ":", 2)
		if len(parts) == 2 {
			method := parts[0]
			durationStr := parts[1]
			minutes := parseReminderDuration(durationStr)
			if minutes > 0 {
				event.Reminders = &gcal.EventReminders{
					UseDefault: false,
					Overrides: []*gcal.EventReminder{
						{Method: method, Minutes: minutes},
					},
				}
			}
		}
	}

	// Handle Google Meet
	if getBool(args, "with_meet") {
		event.ConferenceData = &gcal.ConferenceData{
			CreateRequest: &gcal.CreateConferenceRequest{
				RequestId: fmt.Sprintf("meet-%d", time.Now().UnixNano()),
				ConferenceSolutionKey: &gcal.ConferenceSolutionKey{
					Type: "hangoutsMeet",
				},
			},
		}
	}

	created, err := client.CreateEvent(calendarID, event)
	if err != nil {
		return nil, fmt.Errorf("failed to create event: %w", err)
	}

	return textContent(calendar.ToJSON(created)), nil
}

func (p *Provider) updateEvent(args map[string]interface{}) (interface{}, error) {
	calendarID, err := getStringRequired(args, "calendar_id")
	if err != nil {
		return nil, err
	}
	eventID, err := getStringRequired(args, "event_id")
	if err != nil {
		return nil, err
	}

	account := getString(args, "account")

	client, err := calendar.NewClient(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create Calendar client: %w", err)
	}

	// First get the existing event
	existing, err := client.GetEvent(calendarID, eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing event: %w", err)
	}

	// Build the update event from existing
	event := &gcal.Event{
		Summary:     existing.Summary,
		Description: existing.Description,
		Location:    existing.Location,
	}

	// Handle times based on whether it's all-day
	if existing.AllDay {
		event.Start = &gcal.EventDateTime{Date: existing.Start}
		event.End = &gcal.EventDateTime{Date: existing.End}
	} else {
		event.Start = &gcal.EventDateTime{DateTime: existing.Start}
		event.End = &gcal.EventDateTime{DateTime: existing.End}
	}

	// Apply updates
	if summary := getString(args, "summary"); summary != "" {
		event.Summary = summary
	}
	if description := getString(args, "description"); description != "" {
		event.Description = description
	}
	if location := getString(args, "location"); location != "" {
		event.Location = location
	}
	if from := getString(args, "from"); from != "" {
		if existing.AllDay {
			event.Start = &gcal.EventDateTime{Date: from}
		} else {
			event.Start = &gcal.EventDateTime{DateTime: from}
		}
	}
	if to := getString(args, "to"); to != "" {
		if existing.AllDay {
			event.End = &gcal.EventDateTime{Date: to}
		} else {
			event.End = &gcal.EventDateTime{DateTime: to}
		}
	}

	updated, err := client.UpdateEvent(calendarID, eventID, event)
	if err != nil {
		return nil, fmt.Errorf("failed to update event: %w", err)
	}

	return textContent(calendar.ToJSON(updated)), nil
}

func (p *Provider) deleteEvent(args map[string]interface{}) (interface{}, error) {
	calendarID, err := getStringRequired(args, "calendar_id")
	if err != nil {
		return nil, err
	}
	eventID, err := getStringRequired(args, "event_id")
	if err != nil {
		return nil, err
	}

	account := getString(args, "account")

	client, err := calendar.NewClient(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create Calendar client: %w", err)
	}

	err = client.DeleteEvent(calendarID, eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete event: %w", err)
	}

	result := map[string]string{
		"status":      "deleted",
		"calendar_id": calendarID,
		"event_id":    eventID,
	}
	return textContent(calendar.ToJSON(result)), nil
}

func (p *Provider) checkFreebusy(args map[string]interface{}) (interface{}, error) {
	calendarIDsStr, err := getStringRequired(args, "calendar_ids")
	if err != nil {
		return nil, err
	}
	fromStr, err := getStringRequired(args, "from")
	if err != nil {
		return nil, err
	}
	toStr, err := getStringRequired(args, "to")
	if err != nil {
		return nil, err
	}

	account := getString(args, "account")

	client, err := calendar.NewClient(account)
	if err != nil {
		return nil, fmt.Errorf("failed to create Calendar client: %w", err)
	}

	// Parse calendar IDs
	calendarIDs := strings.Split(calendarIDsStr, ",")
	for i, id := range calendarIDs {
		calendarIDs[i] = strings.TrimSpace(id)
	}

	// Parse times
	timeMin, err := calendar.ParseTimeArg(fromStr, false)
	if err != nil {
		return nil, fmt.Errorf("invalid 'from' time: %w", err)
	}
	timeMax, err := calendar.ParseTimeArg(toStr, true)
	if err != nil {
		return nil, fmt.Errorf("invalid 'to' time: %w", err)
	}

	freeBusy, err := client.FreeBusy(calendarIDs, timeMin, timeMax)
	if err != nil {
		return nil, fmt.Errorf("failed to check free/busy: %w", err)
	}

	return textContent(calendar.ToJSON(freeBusy)), nil
}

// parseReminderDuration parses duration strings like "30m", "1h", "1d" to minutes
// Also accepts bare numbers (e.g., "15") which are treated as minutes
func parseReminderDuration(s string) int64 {
	s = strings.TrimSpace(strings.ToLower(s))
	if len(s) == 0 {
		return 0
	}

	// Check if the string is all digits (bare number = minutes)
	allDigits := true
	for _, c := range s {
		if c < '0' || c > '9' {
			allDigits = false
			break
		}
	}
	if allDigits {
		var value int
		fmt.Sscanf(s, "%d", &value)
		return int64(value)
	}

	// Need at least 2 chars for value + unit
	if len(s) < 2 {
		return 0
	}

	unit := s[len(s)-1]
	valueStr := s[:len(s)-1]

	var value int
	fmt.Sscanf(valueStr, "%d", &value)

	switch unit {
	case 'm':
		return int64(value)
	case 'h':
		return int64(value * 60)
	case 'd':
		return int64(value * 60 * 24)
	case 'w':
		return int64(value * 60 * 24 * 7)
	default:
		return 0 // invalid unit
	}
}
