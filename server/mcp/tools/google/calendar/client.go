// Package calendar provides native Google Calendar API client
package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/diane-assistant/diane/mcp/tools/google/auth"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// Client wraps the Google Calendar API service
type Client struct {
	srv     *calendar.Service
	account string
}

// CalendarInfo represents a calendar entry
type CalendarInfo struct {
	ID              string `json:"id"`
	Summary         string `json:"summary"`
	Description     string `json:"description,omitempty"`
	TimeZone        string `json:"timeZone,omitempty"`
	Primary         bool   `json:"primary,omitempty"`
	BackgroundColor string `json:"backgroundColor,omitempty"`
	AccessRole      string `json:"accessRole,omitempty"`
}

// EventInfo represents a calendar event
type EventInfo struct {
	ID           string         `json:"id"`
	Summary      string         `json:"summary"`
	Description  string         `json:"description,omitempty"`
	Location     string         `json:"location,omitempty"`
	Start        string         `json:"start"`
	End          string         `json:"end"`
	AllDay       bool           `json:"allDay,omitempty"`
	Status       string         `json:"status,omitempty"`
	HtmlLink     string         `json:"htmlLink,omitempty"`
	HangoutLink  string         `json:"hangoutLink,omitempty"`
	Attendees    []AttendeeInfo `json:"attendees,omitempty"`
	Organizer    *OrganizerInfo `json:"organizer,omitempty"`
	Reminders    []ReminderInfo `json:"reminders,omitempty"`
	Recurrence   []string       `json:"recurrence,omitempty"`
	Visibility   string         `json:"visibility,omitempty"`
	Transparency string         `json:"transparency,omitempty"`
	CalendarID   string         `json:"calendarId,omitempty"`
}

// AttendeeInfo represents an event attendee
type AttendeeInfo struct {
	Email          string `json:"email"`
	DisplayName    string `json:"displayName,omitempty"`
	ResponseStatus string `json:"responseStatus,omitempty"`
	Organizer      bool   `json:"organizer,omitempty"`
	Self           bool   `json:"self,omitempty"`
}

// OrganizerInfo represents event organizer
type OrganizerInfo struct {
	Email       string `json:"email"`
	DisplayName string `json:"displayName,omitempty"`
	Self        bool   `json:"self,omitempty"`
}

// ReminderInfo represents a reminder
type ReminderInfo struct {
	Method  string `json:"method"`
	Minutes int64  `json:"minutes"`
}

// FreeBusyInfo represents free/busy info for a calendar
type FreeBusyInfo struct {
	CalendarID string       `json:"calendarId"`
	Busy       []BusyPeriod `json:"busy,omitempty"`
	Errors     []string     `json:"errors,omitempty"`
}

// BusyPeriod represents a busy time period
type BusyPeriod struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// ListEventsOptions contains options for listing events
type ListEventsOptions struct {
	TimeMin    time.Time
	TimeMax    time.Time
	MaxResults int64
	Query      string
}

// NewClient creates a new Google Calendar API client
func NewClient(account string) (*Client, error) {
	if account == "" {
		account = "default"
	}

	ctx := context.Background()

	tokenSource, err := auth.GetTokenSource(ctx, account, calendar.CalendarScope)
	if err != nil {
		return nil, fmt.Errorf("failed to get token source: %w", err)
	}

	srv, err := calendar.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, fmt.Errorf("failed to create Calendar service: %w", err)
	}

	return &Client{srv: srv, account: account}, nil
}

// ListCalendars lists all calendars for the account
func (c *Client) ListCalendars() ([]CalendarInfo, error) {
	resp, err := c.srv.CalendarList.List().Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list calendars: %w", err)
	}

	calendars := make([]CalendarInfo, len(resp.Items))
	for i, cal := range resp.Items {
		calendars[i] = CalendarInfo{
			ID:              cal.Id,
			Summary:         cal.Summary,
			Description:     cal.Description,
			TimeZone:        cal.TimeZone,
			Primary:         cal.Primary,
			BackgroundColor: cal.BackgroundColor,
			AccessRole:      cal.AccessRole,
		}
	}

	return calendars, nil
}

// ListEvents lists events from a calendar
func (c *Client) ListEvents(calendarID string, opts ListEventsOptions) ([]EventInfo, error) {
	if calendarID == "" {
		calendarID = "primary"
	}

	req := c.srv.Events.List(calendarID).
		SingleEvents(true).
		OrderBy("startTime")

	if !opts.TimeMin.IsZero() {
		req = req.TimeMin(opts.TimeMin.Format(time.RFC3339))
	}
	if !opts.TimeMax.IsZero() {
		req = req.TimeMax(opts.TimeMax.Format(time.RFC3339))
	}
	if opts.MaxResults > 0 {
		req = req.MaxResults(opts.MaxResults)
	}
	if opts.Query != "" {
		req = req.Q(opts.Query)
	}

	resp, err := req.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list events: %w", err)
	}

	events := make([]EventInfo, 0, len(resp.Items))
	for _, evt := range resp.Items {
		events = append(events, eventToInfo(evt, calendarID))
	}

	return events, nil
}

// ListAllCalendarEvents lists events from all calendars
func (c *Client) ListAllCalendarEvents(opts ListEventsOptions) ([]EventInfo, error) {
	calendars, err := c.ListCalendars()
	if err != nil {
		return nil, err
	}

	var allEvents []EventInfo
	for _, cal := range calendars {
		events, err := c.ListEvents(cal.ID, opts)
		if err != nil {
			// Skip calendars with errors (e.g., shared calendars without access)
			continue
		}
		allEvents = append(allEvents, events...)
	}

	// Sort by start time
	// Simple bubble sort for now (could use sort.Slice for efficiency)
	for i := 0; i < len(allEvents); i++ {
		for j := i + 1; j < len(allEvents); j++ {
			if allEvents[i].Start > allEvents[j].Start {
				allEvents[i], allEvents[j] = allEvents[j], allEvents[i]
			}
		}
	}

	// Apply max results limit
	if opts.MaxResults > 0 && int64(len(allEvents)) > opts.MaxResults {
		allEvents = allEvents[:opts.MaxResults]
	}

	return allEvents, nil
}

// GetEvent gets a specific event
func (c *Client) GetEvent(calendarID, eventID string) (*EventInfo, error) {
	if calendarID == "" {
		calendarID = "primary"
	}

	evt, err := c.srv.Events.Get(calendarID, eventID).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get event: %w", err)
	}

	info := eventToInfo(evt, calendarID)
	return &info, nil
}

// CreateEvent creates a new event
func (c *Client) CreateEvent(calendarID string, event *calendar.Event) (*EventInfo, error) {
	if calendarID == "" {
		calendarID = "primary"
	}

	created, err := c.srv.Events.Insert(calendarID, event).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create event: %w", err)
	}

	info := eventToInfo(created, calendarID)
	return &info, nil
}

// UpdateEvent updates an existing event
func (c *Client) UpdateEvent(calendarID, eventID string, event *calendar.Event) (*EventInfo, error) {
	if calendarID == "" {
		calendarID = "primary"
	}

	updated, err := c.srv.Events.Update(calendarID, eventID, event).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to update event: %w", err)
	}

	info := eventToInfo(updated, calendarID)
	return &info, nil
}

// DeleteEvent deletes an event
func (c *Client) DeleteEvent(calendarID, eventID string) error {
	if calendarID == "" {
		calendarID = "primary"
	}

	err := c.srv.Events.Delete(calendarID, eventID).Do()
	if err != nil {
		return fmt.Errorf("failed to delete event: %w", err)
	}

	return nil
}

// FreeBusy queries free/busy information for calendars
func (c *Client) FreeBusy(calendarIDs []string, timeMin, timeMax time.Time) ([]FreeBusyInfo, error) {
	items := make([]*calendar.FreeBusyRequestItem, len(calendarIDs))
	for i, id := range calendarIDs {
		items[i] = &calendar.FreeBusyRequestItem{Id: id}
	}

	req := &calendar.FreeBusyRequest{
		TimeMin: timeMin.Format(time.RFC3339),
		TimeMax: timeMax.Format(time.RFC3339),
		Items:   items,
	}

	resp, err := c.srv.Freebusy.Query(req).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to query free/busy: %w", err)
	}

	result := make([]FreeBusyInfo, 0, len(calendarIDs))
	for calID, cal := range resp.Calendars {
		info := FreeBusyInfo{
			CalendarID: calID,
		}

		for _, busy := range cal.Busy {
			info.Busy = append(info.Busy, BusyPeriod{
				Start: busy.Start,
				End:   busy.End,
			})
		}

		for _, err := range cal.Errors {
			info.Errors = append(info.Errors, err.Reason)
		}

		result = append(result, info)
	}

	return result, nil
}

// Helper functions

func eventToInfo(evt *calendar.Event, calendarID string) EventInfo {
	info := EventInfo{
		ID:           evt.Id,
		Summary:      evt.Summary,
		Description:  evt.Description,
		Location:     evt.Location,
		Status:       evt.Status,
		HtmlLink:     evt.HtmlLink,
		HangoutLink:  evt.HangoutLink,
		Recurrence:   evt.Recurrence,
		Visibility:   evt.Visibility,
		Transparency: evt.Transparency,
		CalendarID:   calendarID,
	}

	// Parse start/end times
	if evt.Start != nil {
		if evt.Start.Date != "" {
			info.Start = evt.Start.Date
			info.AllDay = true
		} else {
			info.Start = evt.Start.DateTime
		}
	}
	if evt.End != nil {
		if evt.End.Date != "" {
			info.End = evt.End.Date
		} else {
			info.End = evt.End.DateTime
		}
	}

	// Parse attendees
	if len(evt.Attendees) > 0 {
		info.Attendees = make([]AttendeeInfo, len(evt.Attendees))
		for i, att := range evt.Attendees {
			info.Attendees[i] = AttendeeInfo{
				Email:          att.Email,
				DisplayName:    att.DisplayName,
				ResponseStatus: att.ResponseStatus,
				Organizer:      att.Organizer,
				Self:           att.Self,
			}
		}
	}

	// Parse organizer
	if evt.Organizer != nil {
		info.Organizer = &OrganizerInfo{
			Email:       evt.Organizer.Email,
			DisplayName: evt.Organizer.DisplayName,
			Self:        evt.Organizer.Self,
		}
	}

	// Parse reminders
	if evt.Reminders != nil && len(evt.Reminders.Overrides) > 0 {
		info.Reminders = make([]ReminderInfo, len(evt.Reminders.Overrides))
		for i, rem := range evt.Reminders.Overrides {
			info.Reminders[i] = ReminderInfo{
				Method:  rem.Method,
				Minutes: rem.Minutes,
			}
		}
	}

	return info
}

// ParseTimeArg parses flexible time arguments
// Supports: RFC3339, dates (YYYY-MM-DD), and relative (today, tomorrow, monday, etc)
func ParseTimeArg(arg string, isEnd bool) (time.Time, error) {
	if arg == "" {
		return time.Time{}, nil
	}

	now := time.Now()
	arg = strings.ToLower(strings.TrimSpace(arg))

	// Handle relative times
	switch arg {
	case "today":
		t := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		if isEnd {
			t = t.Add(24 * time.Hour)
		}
		return t, nil
	case "tomorrow":
		t := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		if isEnd {
			t = t.Add(24 * time.Hour)
		}
		return t, nil
	case "yesterday":
		t := time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, now.Location())
		if isEnd {
			t = t.Add(24 * time.Hour)
		}
		return t, nil
	}

	// Handle weekdays
	weekdays := map[string]time.Weekday{
		"sunday":    time.Sunday,
		"monday":    time.Monday,
		"tuesday":   time.Tuesday,
		"wednesday": time.Wednesday,
		"thursday":  time.Thursday,
		"friday":    time.Friday,
		"saturday":  time.Saturday,
	}
	if targetDay, ok := weekdays[arg]; ok {
		daysUntil := int(targetDay) - int(now.Weekday())
		if daysUntil <= 0 {
			daysUntil += 7
		}
		t := time.Date(now.Year(), now.Month(), now.Day()+daysUntil, 0, 0, 0, 0, now.Location())
		if isEnd {
			t = t.Add(24 * time.Hour)
		}
		return t, nil
	}

	// Try RFC3339
	if t, err := time.Parse(time.RFC3339, arg); err == nil {
		return t, nil
	}

	// Try date only (YYYY-MM-DD)
	if t, err := time.ParseInLocation("2006-01-02", arg, now.Location()); err == nil {
		if isEnd {
			t = t.Add(24 * time.Hour)
		}
		return t, nil
	}

	return time.Time{}, fmt.Errorf("cannot parse time: %s", arg)
}

// ToJSON converts an object to JSON string
func ToJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	return string(b)
}
