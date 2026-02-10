// Package google provides MCP tools for Google services (Gmail, Drive, Sheets, Calendar)
package google

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/diane-assistant/diane/mcp/tools/google/gmail"
)

// --- Helper Functions ---

func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" {
			return "", fmt.Errorf("%s: %s", err, stderrStr)
		}
		return "", err
	}

	return strings.TrimSpace(stdout.String()), nil
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

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

// CheckDependencies verifies required binaries exist
func (p *Provider) CheckDependencies() error {
	if !commandExists("gog") {
		return fmt.Errorf("gog CLI not found. Install it to use Google tools")
	}
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

	cmdArgs := []string{"drive", "search", query, fmt.Sprintf("--max=%d", max), "--json"}
	if account != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--account=%s", account))
	}

	output, err := runCommand("gog", cmdArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to search files: %w", err)
	}

	return textContent(output), nil
}

func (p *Provider) listFiles(args map[string]interface{}) (interface{}, error) {
	max := getInt(args, "max", 20)
	account := getString(args, "account")

	// List all files using a query that matches everything
	cmdArgs := []string{"drive", "search", "name contains ''", fmt.Sprintf("--max=%d", max), "--json"}
	if account != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--account=%s", account))
	}

	output, err := runCommand("gog", cmdArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	return textContent(output), nil
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

	cmdArgs := []string{"sheets", "get", sheetId, rangeArg, "--json"}
	if account != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--account=%s", account))
	}

	output, err := runCommand("gog", cmdArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to get sheet: %w", err)
	}

	return textContent(output), nil
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
	values, err := getStringRequired(args, "values")
	if err != nil {
		return nil, err
	}

	account := getString(args, "account")

	cmdArgs := []string{"sheets", "update", sheetId, rangeArg, fmt.Sprintf("--values-json=%s", values), "--input=USER_ENTERED"}
	if account != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--account=%s", account))
	}

	output, err := runCommand("gog", cmdArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to update sheet: %w", err)
	}

	return textContent(output), nil
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
	values, err := getStringRequired(args, "values")
	if err != nil {
		return nil, err
	}

	account := getString(args, "account")

	cmdArgs := []string{"sheets", "append", sheetId, rangeArg, fmt.Sprintf("--values-json=%s", values), "--insert=INSERT_ROWS"}
	if account != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--account=%s", account))
	}

	output, err := runCommand("gog", cmdArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to append to sheet: %w", err)
	}

	return textContent(output), nil
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

	cmdArgs := []string{"sheets", "clear", sheetId, rangeArg}
	if account != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--account=%s", account))
	}

	output, err := runCommand("gog", cmdArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to clear sheet: %w", err)
	}

	return textContent(output), nil
}

func (p *Provider) getSheetMetadata(args map[string]interface{}) (interface{}, error) {
	sheetId, err := getStringRequired(args, "sheetId")
	if err != nil {
		return nil, err
	}

	account := getString(args, "account")

	cmdArgs := []string{"sheets", "metadata", sheetId, "--json"}
	if account != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--account=%s", account))
	}

	output, err := runCommand("gog", cmdArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to get sheet metadata: %w", err)
	}

	return textContent(output), nil
}

// --- Calendar Tools ---

func (p *Provider) listCalendars(args map[string]interface{}) (interface{}, error) {
	account := getString(args, "account")

	cmdArgs := []string{"calendar", "calendars", "--json"}
	if account != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--account=%s", account))
	}

	output, err := runCommand("gog", cmdArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to list calendars: %w", err)
	}

	return textContent(output), nil
}

func (p *Provider) listEvents(args map[string]interface{}) (interface{}, error) {
	cmdArgs := []string{"calendar", "events"}

	// Add calendar ID or --all flag
	if getBool(args, "all") {
		cmdArgs = append(cmdArgs, "--all")
	} else {
		calendarId := getString(args, "calendar_id")
		if calendarId == "" {
			calendarId = "primary"
		}
		cmdArgs = append(cmdArgs, calendarId)
	}

	// Add optional flags
	if account := getString(args, "account"); account != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--account=%s", account))
	}
	if from := getString(args, "from"); from != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--from=%s", from))
	}
	if to := getString(args, "to"); to != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--to=%s", to))
	}
	if getBool(args, "today") {
		cmdArgs = append(cmdArgs, "--today")
	}
	if getBool(args, "tomorrow") {
		cmdArgs = append(cmdArgs, "--tomorrow")
	}
	if getBool(args, "week") {
		cmdArgs = append(cmdArgs, "--week")
	}
	if days := getInt(args, "days", 0); days > 0 {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--days=%d", days))
	}
	if max := getInt(args, "max", 0); max > 0 {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--max=%d", max))
	}
	if query := getString(args, "query"); query != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--query=%s", query))
	}

	cmdArgs = append(cmdArgs, "--json")

	output, err := runCommand("gog", cmdArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to list events: %w", err)
	}

	return textContent(output), nil
}

func (p *Provider) getEvent(args map[string]interface{}) (interface{}, error) {
	calendarId, err := getStringRequired(args, "calendar_id")
	if err != nil {
		return nil, err
	}
	eventId, err := getStringRequired(args, "event_id")
	if err != nil {
		return nil, err
	}

	account := getString(args, "account")

	cmdArgs := []string{"calendar", "event", calendarId, eventId, "--json"}
	if account != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--account=%s", account))
	}

	output, err := runCommand("gog", cmdArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to get event: %w", err)
	}

	return textContent(output), nil
}

func (p *Provider) createEvent(args map[string]interface{}) (interface{}, error) {
	calendarId, err := getStringRequired(args, "calendar_id")
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

	cmdArgs := []string{"calendar", "create", calendarId,
		fmt.Sprintf("--summary=%s", summary),
		fmt.Sprintf("--from=%s", from),
		fmt.Sprintf("--to=%s", to),
	}

	// Optional flags
	if account := getString(args, "account"); account != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--account=%s", account))
	}
	if description := getString(args, "description"); description != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--description=%s", description))
	}
	if location := getString(args, "location"); location != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--location=%s", location))
	}
	if attendees := getString(args, "attendees"); attendees != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--attendees=%s", attendees))
	}
	if getBool(args, "all_day") {
		cmdArgs = append(cmdArgs, "--all-day")
	}
	if reminder := getString(args, "reminder"); reminder != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--reminder=%s", reminder))
	}
	if getBool(args, "with_meet") {
		cmdArgs = append(cmdArgs, "--with-meet")
	}
	if visibility := getString(args, "visibility"); visibility != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--visibility=%s", visibility))
	}
	if transparency := getString(args, "transparency"); transparency != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--transparency=%s", transparency))
	}

	cmdArgs = append(cmdArgs, "--json")

	output, err := runCommand("gog", cmdArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to create event: %w", err)
	}

	return textContent(output), nil
}

func (p *Provider) updateEvent(args map[string]interface{}) (interface{}, error) {
	calendarId, err := getStringRequired(args, "calendar_id")
	if err != nil {
		return nil, err
	}
	eventId, err := getStringRequired(args, "event_id")
	if err != nil {
		return nil, err
	}

	cmdArgs := []string{"calendar", "update", calendarId, eventId}

	// Optional flags
	if account := getString(args, "account"); account != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--account=%s", account))
	}
	if summary := getString(args, "summary"); summary != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--summary=%s", summary))
	}
	if from := getString(args, "from"); from != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--from=%s", from))
	}
	if to := getString(args, "to"); to != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--to=%s", to))
	}
	if description := getString(args, "description"); description != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--description=%s", description))
	}
	if location := getString(args, "location"); location != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--location=%s", location))
	}

	cmdArgs = append(cmdArgs, "--json")

	output, err := runCommand("gog", cmdArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to update event: %w", err)
	}

	return textContent(output), nil
}

func (p *Provider) deleteEvent(args map[string]interface{}) (interface{}, error) {
	calendarId, err := getStringRequired(args, "calendar_id")
	if err != nil {
		return nil, err
	}
	eventId, err := getStringRequired(args, "event_id")
	if err != nil {
		return nil, err
	}

	account := getString(args, "account")

	cmdArgs := []string{"calendar", "delete", calendarId, eventId, "--force", "--json"}
	if account != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--account=%s", account))
	}

	output, err := runCommand("gog", cmdArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to delete event: %w", err)
	}

	return textContent(output), nil
}

func (p *Provider) checkFreebusy(args map[string]interface{}) (interface{}, error) {
	calendarIds, err := getStringRequired(args, "calendar_ids")
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

	cmdArgs := []string{"calendar", "freebusy", calendarIds,
		fmt.Sprintf("--from=%s", from),
		fmt.Sprintf("--to=%s", to),
		"--json",
	}
	if account != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--account=%s", account))
	}

	output, err := runCommand("gog", cmdArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to check free/busy: %w", err)
	}

	return textContent(output), nil
}
