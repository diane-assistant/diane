package google

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/diane-assistant/diane/mcp/tools"
	"github.com/diane-assistant/diane/mcp/tools/google/gmail"
)

// Resources returns all resources provided by the Google provider
func (p *Provider) Resources() []tools.Resource {
	return []tools.Resource{
		{
			URI:         "google://gmail/cache/stats",
			Name:        "Gmail Cache Statistics",
			Description: "Current state of the Gmail cache: email count, date range, sync status, attachment storage",
			MimeType:    "application/json",
		},
		{
			URI:         "google://gmail/labels",
			Name:        "Gmail Labels",
			Description: "All Gmail labels for the account with their IDs and types",
			MimeType:    "application/json",
		},
		{
			URI:         "google://gmail/sync/status",
			Name:        "Gmail Sync Status",
			Description: "Last sync time, history ID, and sync health",
			MimeType:    "application/json",
		},
		{
			URI:         "google://gmail/guide",
			Name:        "Gmail Tools Guide",
			Description: "Documentation for using Gmail tools effectively: search syntax, batch operations, JSON-LD extraction",
			MimeType:    "text/markdown",
		},
		{
			URI:         "google://calendar/guide",
			Name:        "Calendar Tools Guide",
			Description: "Documentation for calendar operations: time formats, recurring events, free/busy queries",
			MimeType:    "text/markdown",
		},
	}
}

// ReadResource returns the content of a resource
func (p *Provider) ReadResource(uri string) (*tools.ResourceContent, error) {
	switch uri {
	case "google://gmail/cache/stats":
		return p.resourceCacheStats()
	case "google://gmail/labels":
		return p.resourceLabels()
	case "google://gmail/sync/status":
		return p.resourceSyncStatus()
	case "google://gmail/guide":
		return p.resourceGmailGuide()
	case "google://calendar/guide":
		return p.resourceCalendarGuide()
	default:
		return nil, fmt.Errorf("resource not found: %s", uri)
	}
}

func (p *Provider) resourceCacheStats() (*tools.ResourceContent, error) {
	svc, err := gmail.NewService("")
	if err != nil {
		return nil, fmt.Errorf("failed to create gmail service: %w", err)
	}
	defer svc.Close()

	stats, err := svc.GetCacheStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get cache stats: %w", err)
	}

	if stats == nil {
		return &tools.ResourceContent{
			URI:      "google://gmail/cache/stats",
			MimeType: "application/json",
			Text:     `{"status": "cache_not_initialized", "message": "Run gmail_sync to initialize the cache"}`,
		}, nil
	}

	data, _ := json.MarshalIndent(stats, "", "  ")
	return &tools.ResourceContent{
		URI:      "google://gmail/cache/stats",
		MimeType: "application/json",
		Text:     string(data),
	}, nil
}

func (p *Provider) resourceLabels() (*tools.ResourceContent, error) {
	svc, err := gmail.NewService("")
	if err != nil {
		return nil, fmt.Errorf("failed to create gmail service: %w", err)
	}
	defer svc.Close()

	labels, err := svc.ListLabels()
	if err != nil {
		return nil, fmt.Errorf("failed to list labels: %w", err)
	}

	// Simplify label data
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

	data, _ := json.MarshalIndent(result, "", "  ")
	return &tools.ResourceContent{
		URI:      "google://gmail/labels",
		MimeType: "application/json",
		Text:     string(data),
	}, nil
}

func (p *Provider) resourceSyncStatus() (*tools.ResourceContent, error) {
	svc, err := gmail.NewService("")
	if err != nil {
		return nil, fmt.Errorf("failed to create gmail service: %w", err)
	}
	defer svc.Close()

	state, err := svc.GetSyncState()
	if err != nil {
		return nil, fmt.Errorf("failed to get sync state: %w", err)
	}

	if state == nil {
		return &tools.ResourceContent{
			URI:      "google://gmail/sync/status",
			MimeType: "application/json",
			Text:     `{"status": "never_synced", "message": "Run gmail_sync to start syncing"}`,
		}, nil
	}

	data, _ := json.MarshalIndent(state, "", "  ")
	return &tools.ResourceContent{
		URI:      "google://gmail/sync/status",
		MimeType: "application/json",
		Text:     string(data),
	}, nil
}

func (p *Provider) resourceGmailGuide() (*tools.ResourceContent, error) {
	guide := strings.TrimSpace(`
# Gmail Tools Guide

## Available Tools

### Search & Read
- **google_search_emails**: Search emails using Gmail query syntax
- **google_read_email**: Get full email content by ID
- **gmail_get_content**: Get extracted plain text and JSON-LD structured data

### Batch Operations
- **gmail_batch_get_messages**: Fetch multiple emails efficiently
- **gmail_batch_modify_labels**: Add/remove labels from multiple emails

### Labels
- **gmail_list_labels**: List all labels with IDs
- **gmail_create_label**: Create new labels

### Attachments
- **gmail_list_attachments**: List attachments for an email
- **gmail_download_attachment**: Download attachment to local storage

### Cache & Sync
- **gmail_sync**: Sync emails to local cache (incremental or full)
- **gmail_cache_stats**: View cache statistics

## Gmail Search Syntax

### Common Operators
| Operator | Example | Description |
|----------|---------|-------------|
| from: | from:amazon | Emails from sender |
| to: | to:me@email.com | Emails to recipient |
| subject: | subject:invoice | Subject contains word |
| label: | label:inbox | Has specific label |
| is: | is:unread, is:starred | Email state |
| has: | has:attachment | Has attachments |
| newer_than: | newer_than:7d | Within last N days |
| older_than: | older_than:30d | Older than N days |
| after: | after:2024/01/01 | After date |
| before: | before:2024/12/31 | Before date |

### Combining Operators
- **AND**: from:amazon subject:order (implicit AND)
- **OR**: from:amazon OR from:ebay
- **NOT**: -label:archived (exclude)
- **Group**: (from:amazon OR from:ebay) subject:shipped

### Examples
` + "```" + `
# Unread emails from last week
is:unread newer_than:7d

# Order confirmations
subject:(order confirmation) OR subject:(your order)

# Emails with attachments from specific sender
from:company.com has:attachment

# Archive old newsletters
from:newsletter older_than:30d
` + "```" + `

## JSON-LD Structured Data

Many transactional emails contain JSON-LD structured data that can be extracted:

### Common Types
- **Order**: Purchase confirmations with order numbers, items, prices
- **ParcelDelivery**: Shipping notifications with tracking numbers
- **FlightReservation**: Flight bookings with times, confirmation codes
- **EventReservation**: Event tickets with venue, time
- **LodgingReservation**: Hotel bookings

### Usage
Use gmail_get_content to extract JSON-LD from emails. The response includes:
- plain_text: Readable text content
- json_ld: Array of structured data objects
- json_ld_types: List of @type values found

## Batch Operations

For efficiency, use batch operations when working with multiple emails:

` + "```" + `
# Archive multiple emails
gmail_batch_modify_labels ids="id1,id2,id3" remove="INBOX"

# Add label to multiple emails
gmail_batch_modify_labels ids="id1,id2,id3" add="MyLabel"

# Fetch multiple emails at once
gmail_batch_get_messages ids="id1,id2,id3" format="metadata"
` + "```" + `

## Local Cache

The Gmail cache stores email metadata locally for fast access:
- Stored in ~/.diane/gmail.db
- Attachments in ~/.diane/attachments/
- Use gmail_sync for incremental updates
- Use gmail_cache_stats to check cache health
`)

	return &tools.ResourceContent{
		URI:      "google://gmail/guide",
		MimeType: "text/markdown",
		Text:     guide,
	}, nil
}

func (p *Provider) resourceCalendarGuide() (*tools.ResourceContent, error) {
	guide := strings.TrimSpace(`
# Calendar Tools Guide

## Available Tools

### Calendars
- **google_list_calendars**: List all calendars for the account

### Events
- **google_list_events**: List events with flexible time filtering
- **google_get_event**: Get details of a specific event
- **google_create_event**: Create a new event
- **google_update_event**: Update an existing event
- **google_delete_event**: Delete an event

### Scheduling
- **google_check_freebusy**: Check free/busy status for calendars

## Time Formats

### RFC3339 Format
Standard format for precise times with timezone:
` + "```" + `
2026-02-10T15:30:00+01:00  # 3:30 PM in UTC+1
2026-02-10T09:00:00-05:00  # 9:00 AM in EST
2026-02-10T14:00:00Z       # 2:00 PM in UTC
` + "```" + `

### Date-Only Format (All-Day Events)
` + "```" + `
2026-02-10  # All-day event on Feb 10
` + "```" + `

### Relative Time Shortcuts
The google_list_events tool supports shortcuts:
- **today**: Today's events
- **tomorrow**: Tomorrow's events
- **week**: This week (Monday-Sunday)
- **days=N**: Next N days

## Listing Events

### Examples
` + "```" + `
# Today's events
google_list_events today=true

# This week across all calendars
google_list_events week=true all=true

# Next 14 days from primary calendar
google_list_events days=14 calendar_id="primary"

# Events matching a query
google_list_events query="meeting" days=7
` + "```" + `

## Creating Events

### Required Parameters
- calendar_id: Usually "primary"
- summary: Event title
- from: Start time
- to: End time

### Optional Parameters
- description: Event description
- location: Event location
- attendees: Comma-separated email list
- all_day: true for all-day events
- with_meet: true to add Google Meet link
- reminder: e.g., "popup:30m", "email:1d"
- visibility: default, public, private
- transparency: "busy" or "free"

### Example
` + "```" + `
google_create_event \
  calendar_id="primary" \
  summary="Team Standup" \
  from="2026-02-10T09:00:00+01:00" \
  to="2026-02-10T09:30:00+01:00" \
  description="Daily sync" \
  with_meet=true \
  attendees="alice@company.com,bob@company.com" \
  reminder="popup:10m"
` + "```" + `

## Free/Busy Queries

Check availability before scheduling:
` + "```" + `
google_check_freebusy \
  calendar_ids="primary,team@company.com" \
  from="2026-02-10T08:00:00+01:00" \
  to="2026-02-10T18:00:00+01:00"
` + "```" + `

Returns busy time blocks for each calendar.

## Common Calendar IDs
- **primary**: User's main calendar
- **email@domain.com**: Shared calendar by email
- Use google_list_calendars to see all available calendars
`)

	return &tools.ResourceContent{
		URI:      "google://calendar/guide",
		MimeType: "text/markdown",
		Text:     guide,
	}, nil
}
