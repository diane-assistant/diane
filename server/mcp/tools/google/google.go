// Package google provides MCP tools for Google services (Gmail, Drive, Sheets, Calendar)
package google

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
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
			Description: "Search Gmail messages using Gmail search syntax. Returns a list of matched email threads.",
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
			Description: "Get full content of a specific Gmail message by its ID.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"id":      stringProperty("The message or thread ID to retrieve"),
					"account": stringProperty("Email account to use (optional, uses default if omitted)"),
				},
				[]string{"id"},
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

	cmdArgs := []string{"gmail", "search", query, fmt.Sprintf("--max=%d", max), "--json"}
	if account != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--account=%s", account))
	}

	output, err := runCommand("gog", cmdArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to search emails: %w", err)
	}

	return textContent(output), nil
}

func (p *Provider) readEmail(args map[string]interface{}) (interface{}, error) {
	id, err := getStringRequired(args, "id")
	if err != nil {
		return nil, err
	}

	account := getString(args, "account")

	cmdArgs := []string{"gmail", "get", id, "--format=full", "--json"}
	if account != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--account=%s", account))
	}

	output, err := runCommand("gog", cmdArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to read email: %w", err)
	}

	return textContent(output), nil
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
