package google

import (
	"fmt"
	"strings"

	"github.com/diane-assistant/diane/mcp/tools"
)

// Prompts returns all prompts provided by the Google provider
func (p *Provider) Prompts() []tools.Prompt {
	return []tools.Prompt{
		{
			Name:        "gmail_inbox_triage",
			Description: "Analyze inbox and categorize emails by priority and action needed. Helps process unread emails efficiently.",
			Arguments: []tools.PromptArgument{
				{Name: "max_emails", Description: "Maximum emails to analyze (default: 20)", Required: false},
				{Name: "labels", Description: "Comma-separated labels to filter (default: INBOX,UNREAD)", Required: false},
			},
		},
		{
			Name:        "gmail_sender_analysis",
			Description: "Analyze emails from a specific sender to understand patterns, frequency, and content types.",
			Arguments: []tools.PromptArgument{
				{Name: "sender", Description: "Email address or partial match (e.g., 'amazon', 'newsletter@')", Required: true},
				{Name: "max_emails", Description: "Maximum emails to analyze (default: 50)", Required: false},
			},
		},
		{
			Name:        "gmail_daily_digest",
			Description: "Create a summary of today's emails with key highlights and action items.",
			Arguments: []tools.PromptArgument{
				{Name: "include_read", Description: "Include already read emails (default: false)", Required: false},
			},
		},
		{
			Name:        "gmail_cleanup_suggestions",
			Description: "Analyze mailbox for cleanup opportunities: unsubscribe candidates, bulk deletions, archiving suggestions.",
			Arguments: []tools.PromptArgument{
				{Name: "days", Description: "Analyze emails from last N days (default: 30)", Required: false},
			},
		},
		{
			Name:        "gmail_order_tracking",
			Description: "Find and summarize all pending orders and shipments from email (uses JSON-LD structured data).",
			Arguments: []tools.PromptArgument{
				{Name: "days", Description: "Look back N days (default: 30)", Required: false},
			},
		},
		{
			Name:        "calendar_schedule_review",
			Description: "Review upcoming calendar events and identify potential conflicts or preparation needed.",
			Arguments: []tools.PromptArgument{
				{Name: "days", Description: "Days to look ahead (default: 7)", Required: false},
				{Name: "calendar", Description: "Calendar ID (default: primary)", Required: false},
			},
		},
	}
}

// GetPrompt returns a prompt with arguments substituted
func (p *Provider) GetPrompt(name string, args map[string]string) ([]tools.PromptMessage, error) {
	switch name {
	case "gmail_inbox_triage":
		return p.promptInboxTriage(args), nil
	case "gmail_sender_analysis":
		return p.promptSenderAnalysis(args), nil
	case "gmail_daily_digest":
		return p.promptDailyDigest(args), nil
	case "gmail_cleanup_suggestions":
		return p.promptCleanupSuggestions(args), nil
	case "gmail_order_tracking":
		return p.promptOrderTracking(args), nil
	case "calendar_schedule_review":
		return p.promptScheduleReview(args), nil
	default:
		return nil, fmt.Errorf("prompt not found: %s", name)
	}
}

func (p *Provider) promptInboxTriage(args map[string]string) []tools.PromptMessage {
	maxEmails := getArgOrDefault(args, "max_emails", "20")
	labels := getArgOrDefault(args, "labels", "INBOX,UNREAD")

	query := buildLabelQuery(labels)

	return []tools.PromptMessage{
		{
			Role: "user",
			Content: tools.PromptContent{
				Type: "text",
				Text: fmt.Sprintf(`Help me triage my inbox efficiently.

**Instructions:**
1. First, use google_search_emails with query="%s" and max=%s to get my emails
2. Analyze each email and categorize into:
   - **Urgent**: Requires immediate response (today)
   - **Action Needed**: Requires response but not urgent (this week)
   - **FYI**: Informational, no action needed
   - **Archive/Delete**: Newsletters, promotions, outdated

3. For each email, provide:
   - Subject and sender
   - Category and why
   - Suggested action (reply, archive, delete, label)

4. At the end, summarize:
   - Total emails processed
   - Count by category
   - Top 3 priorities to handle first

If you find emails that should be batch-processed (e.g., all from same sender), suggest using gmail_batch_modify_labels to handle them efficiently.`, query, maxEmails),
			},
		},
	}
}

func (p *Provider) promptSenderAnalysis(args map[string]string) []tools.PromptMessage {
	sender := args["sender"]
	if sender == "" {
		sender = "[SENDER_EMAIL]"
	}
	maxEmails := getArgOrDefault(args, "max_emails", "50")

	return []tools.PromptMessage{
		{
			Role: "user",
			Content: tools.PromptContent{
				Type: "text",
				Text: fmt.Sprintf(`Analyze my emails from "%s" to understand the pattern.

**Instructions:**
1. Use google_search_emails with query="from:%s" and max=%s
2. For the results, analyze:
   - **Frequency**: How often do they email? (daily, weekly, monthly)
   - **Content types**: Orders, newsletters, notifications, personal?
   - **JSON-LD data**: Use gmail_get_content on a few samples to check for structured data (orders, shipments, etc.)

3. Provide insights:
   - Is this sender important or noise?
   - Should emails from this sender be auto-archived or labeled?
   - Are there actionable items (orders, bills, etc.)?

4. Recommend actions:
   - Create a filter/label suggestion
   - Unsubscribe recommendation if it's marketing
   - Cleanup suggestion if there are many old emails`, sender, sender, maxEmails),
			},
		},
	}
}

func (p *Provider) promptDailyDigest(args map[string]string) []tools.PromptMessage {
	includeRead := getArgOrDefault(args, "include_read", "false")

	query := "newer_than:1d"
	if includeRead != "true" {
		query = "newer_than:1d is:unread"
	}

	return []tools.PromptMessage{
		{
			Role: "user",
			Content: tools.PromptContent{
				Type: "text",
				Text: fmt.Sprintf(`Create a daily digest of my emails from today.

**Instructions:**
1. Use google_search_emails with query="%s" and max=50
2. Group emails by category:
   - **Personal**: From known contacts
   - **Work**: Business-related
   - **Orders & Shipping**: Purchase confirmations, tracking (check JSON-LD)
   - **Newsletters & Updates**: Subscriptions
   - **Notifications**: Automated alerts

3. For each category, summarize:
   - Number of emails
   - Key highlights (important subjects)
   - Any action items

4. Create an executive summary:
   - Most important emails to read
   - Quick wins (things to archive/delete)
   - Pending actions

5. If there are order/shipping updates, use gmail_get_content to extract tracking info from JSON-LD.`, query),
			},
		},
	}
}

func (p *Provider) promptCleanupSuggestions(args map[string]string) []tools.PromptMessage {
	days := getArgOrDefault(args, "days", "30")

	return []tools.PromptMessage{
		{
			Role: "user",
			Content: tools.PromptContent{
				Type: "text",
				Text: fmt.Sprintf(`Analyze my mailbox and suggest cleanup actions.

**Instructions:**
1. Use google_search_emails with query="older_than:%sd" and max=100
2. Identify cleanup opportunities:

   **Unsubscribe Candidates:**
   - Newsletters you never read
   - Marketing emails
   - Notifications that are no longer relevant

   **Bulk Delete Candidates:**
   - Old promotional emails
   - Expired deals/coupons
   - Outdated notifications

   **Archive Candidates:**
   - Old but potentially useful emails
   - Completed order confirmations
   - Past event invitations

3. Group by sender and count emails from each
4. Suggest batch operations using gmail_batch_modify_labels:
   - Which messages to archive (remove INBOX label)
   - Which labels to create for organization
   - Which senders to consider unsubscribing from

5. Estimate impact:
   - How many emails could be cleaned up
   - Space/clutter reduction`, days),
			},
		},
	}
}

func (p *Provider) promptOrderTracking(args map[string]string) []tools.PromptMessage {
	days := getArgOrDefault(args, "days", "30")

	return []tools.PromptMessage{
		{
			Role: "user",
			Content: tools.PromptContent{
				Type: "text",
				Text: fmt.Sprintf(`Find and summarize all my pending orders and shipments.

**Instructions:**
1. Use google_search_emails with query="newer_than:%sd (order OR shipped OR tracking OR delivery)" and max=50
2. For each result that looks like an order/shipping email, use gmail_get_content to extract JSON-LD data
3. Look for these JSON-LD types:
   - **Order**: Purchase confirmations with order numbers
   - **ParcelDelivery**: Shipping notifications with tracking
   - **TrackAction**: Tracking links

4. Create a summary table:
   | Order/Tracking | Merchant | Status | Expected Date |
   |----------------|----------|--------|---------------|

5. Highlight:
   - Orders that should have arrived but no delivery confirmation
   - Packages currently in transit
   - Recent orders not yet shipped

6. If tracking numbers are found, note them for easy lookup.`, days),
			},
		},
	}
}

func (p *Provider) promptScheduleReview(args map[string]string) []tools.PromptMessage {
	days := getArgOrDefault(args, "days", "7")
	calendar := getArgOrDefault(args, "calendar", "primary")

	return []tools.PromptMessage{
		{
			Role: "user",
			Content: tools.PromptContent{
				Type: "text",
				Text: fmt.Sprintf(`Review my calendar for the next %s days and help me prepare.

**Instructions:**
1. Use google_list_events with calendar_id="%s" and days=%s
2. Analyze the schedule:

   **Conflicts:**
   - Overlapping events
   - Back-to-back meetings with no buffer
   - Events at unusual times

   **Preparation Needed:**
   - Meetings requiring prep work
   - Events with unclear locations
   - Recurring meetings that might need agenda

   **Optimization Suggestions:**
   - Meetings that could be shorter
   - Events that could be rescheduled for better flow
   - Gaps that could be used productively

3. Create a day-by-day summary:
   - Key events each day
   - Busiest/lightest days
   - Any free time blocks

4. Flag any events that might need follow-up:
   - Missing attendee responses
   - No video link for remote meetings
   - Events without clear purpose`, days, calendar, days),
			},
		},
	}
}

// Helper functions

func getArgOrDefault(args map[string]string, key, defaultVal string) string {
	if val, ok := args[key]; ok && val != "" {
		return val
	}
	return defaultVal
}

func buildLabelQuery(labels string) string {
	parts := strings.Split(labels, ",")
	var queryParts []string
	for _, label := range parts {
		label = strings.TrimSpace(label)
		if label != "" {
			queryParts = append(queryParts, fmt.Sprintf("label:%s", strings.ToLower(label)))
		}
	}
	return strings.Join(queryParts, " ")
}
