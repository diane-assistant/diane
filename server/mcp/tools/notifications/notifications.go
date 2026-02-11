// Package notifications provides MCP tools for sending notifications (Discord, Home Assistant)
package notifications

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/diane-assistant/diane/mcp/tools"
	_ "github.com/mattn/go-sqlite3"
)

// --- Configuration ---

type discordConfig struct {
	BotToken  string
	ChannelID string
}

type homeAssistantConfig struct {
	ServerURL     string `json:"server_url"`
	AccessToken   string `json:"access_token"`
	NotifyService string `json:"notify_service"`
}

type channelMappings map[string]string

func getDiscordBotToken() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	dbPath := filepath.Join(home, ".kimaki", "discord-sessions.db")
	db, err := sql.Open("sqlite3", dbPath+"?mode=ro")
	if err != nil {
		return "", fmt.Errorf("failed to open Kimaki database: %w", err)
	}
	defer db.Close()

	var token string
	err = db.QueryRow("SELECT token FROM bot_tokens LIMIT 1").Scan(&token)
	if err != nil {
		return "", fmt.Errorf("no Discord bot token found in Kimaki database: %w", err)
	}

	return token, nil
}

func loadChannelMappings() (channelMappings, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	// Try DIANE config directories
	paths := []string{
		filepath.Join(home, ".diane", "secrets", "discord-channels.json"),
		filepath.Join(home, ".opencode", "secrets", "discord-channels.json"),
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var mappings channelMappings
		if err := json.Unmarshal(data, &mappings); err != nil {
			continue
		}
		return mappings, nil
	}

	return channelMappings{}, nil
}

func getDefaultChannelID() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	dbPath := filepath.Join(home, ".kimaki", "discord-sessions.db")
	db, err := sql.Open("sqlite3", dbPath+"?mode=ro")
	if err != nil {
		return "", fmt.Errorf("failed to open Kimaki database: %w", err)
	}
	defer db.Close()

	// Look for diane channel in Kimaki database
	var channelID string
	err = db.QueryRow(
		"SELECT channel_id FROM channel_directories WHERE directory LIKE '%diane%' ORDER BY id LIMIT 1",
	).Scan(&channelID)
	if err != nil {
		return "", fmt.Errorf("no Discord channel mapping found for diane: %w", err)
	}

	return channelID, nil
}

func getChannelID(channelNameOrID string) (string, error) {
	// If no channel specified, use default
	if channelNameOrID == "" {
		return getDefaultChannelID()
	}

	// If it looks like a channel ID (all digits), use directly
	if matched, _ := regexp.MatchString(`^\d+$`, channelNameOrID); matched {
		return channelNameOrID, nil
	}

	// Otherwise, look up in config
	mappings, err := loadChannelMappings()
	if err != nil {
		return "", err
	}

	channelID, ok := mappings[channelNameOrID]
	if !ok {
		keys := make([]string, 0, len(mappings))
		for k := range mappings {
			keys = append(keys, k)
		}
		return "", fmt.Errorf("channel %q not found in discord-channels.json. Available: %s", channelNameOrID, strings.Join(keys, ", "))
	}

	return channelID, nil
}

func getHomeAssistantConfig() (*homeAssistantConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(home, ".diane", "secrets", "homeassistant-config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("Home Assistant config not found. Create %s with server_url, access_token, notify_service", configPath)
	}

	var config homeAssistantConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse Home Assistant config: %w", err)
	}

	return &config, nil
}

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

// --- Tool Definition ---

// Tool represents an MCP tool definition
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// Provider implements ToolProvider for notification services
type Provider struct {
	discordAvailable       bool
	homeAssistantAvailable bool
}

// NewProvider creates a new notifications tools provider
func NewProvider() *Provider {
	return &Provider{}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "notifications"
}

// CheckDependencies verifies at least one notification service is available
func (p *Provider) CheckDependencies() error {
	// Check Discord
	_, err := getDiscordBotToken()
	p.discordAvailable = err == nil

	// Check Home Assistant
	_, err = getHomeAssistantConfig()
	p.homeAssistantAvailable = err == nil

	if !p.discordAvailable && !p.homeAssistantAvailable {
		return fmt.Errorf("no notification services available (Discord or Home Assistant)")
	}

	return nil
}

// Tools returns all notification tools
func (p *Provider) Tools() []Tool {
	var tools []Tool

	// Discord tools
	if p.discordAvailable {
		tools = append(tools, []Tool{
			{
				Name: "discord_send_notification",
				Description: `Send a notification message to Discord channel via the Kimaki bot.

Uses the Discord bot credentials stored in Kimaki database.
Perfect for automation notifications, cron job results, and alerts.`,
				InputSchema: objectSchema(
					map[string]interface{}{
						"message":      stringProperty("Message content to send"),
						"title":        stringProperty("Optional title/header (will be bolded)"),
						"channel_name": stringProperty("Channel name from config, channel ID (digits only), or default #diane"),
					},
					[]string{"message"},
				),
			},
			{
				Name: "discord_send_embed",
				Description: `Send a rich embed notification to Discord channel.

Rich embeds support:
- Color coding (success=green, error=red, warning=yellow, info=blue)
- Title and description
- Multiple fields (key-value pairs)
- Footer text`,
				InputSchema: objectSchema(
					map[string]interface{}{
						"title":        stringProperty("Embed title"),
						"description":  stringProperty("Embed description text"),
						"color":        stringProperty("Color: success, error, warning, info (default: info)"),
						"fields":       stringProperty("JSON array of fields: [{name: string, value: string, inline?: boolean}]"),
						"footer":       stringProperty("Footer text"),
						"channel_name": stringProperty("Channel name from config, channel ID, or default"),
					},
					[]string{"title"},
				),
			},
			{
				Name: "discord_send_reaction",
				Description: `Add a reaction emoji to a Discord message.

Useful for acknowledging task completion, status indicators, and quick feedback.
Common emojis: ✅ ❌ ⚠️ ℹ️ 🔄 ⏳ 💰 📊 🚀`,
				InputSchema: objectSchema(
					map[string]interface{}{
						"message_id":   stringProperty("Discord message ID to react to"),
						"emoji":        stringProperty("Emoji to use (unicode emoji like ✅ or custom :emoji_name:)"),
						"channel_name": stringProperty("Channel name from config, channel ID, or default"),
					},
					[]string{"message_id", "emoji"},
				),
			},
			{
				Name: "discord_send_message_with_buttons",
				Description: `Send a message or embed with interactive buttons to Discord.

Buttons allow users to click and respond to questions or options.
Use for yes/no questions, multiple choice, approval workflows.`,
				InputSchema: objectSchema(
					map[string]interface{}{
						"message":           stringProperty("Message text (optional if using embed)"),
						"embed_title":       stringProperty("Embed title (optional)"),
						"embed_description": stringProperty("Embed description (optional)"),
						"embed_color":       stringProperty("Embed color: success, error, warning, info"),
						"buttons":           stringProperty("JSON array of buttons: [{label: string, custom_id: string, style?: number}]. Styles: 1=Primary(blue), 2=Secondary(gray), 3=Success(green), 4=Danger(red)"),
						"channel_name":      stringProperty("Channel name from config, channel ID, or default"),
					},
					[]string{"buttons"},
				),
			},
		}...)
	}

	// Home Assistant tools
	if p.homeAssistantAvailable {
		tools = append(tools, []Tool{
			{
				Name:        "homeassistant_send_notification",
				Description: "Send a notification to Home Assistant companion app. Use for cron job summaries, alerts, and important updates.",
				InputSchema: objectSchema(
					map[string]interface{}{
						"message": stringProperty("Notification message text"),
						"title":   stringProperty("Notification title (optional)"),
						"data":    stringProperty("Additional notification data as JSON string (optional)"),
					},
					[]string{"message"},
				),
			},
			{
				Name:        "homeassistant_send_actionable_notification",
				Description: "Send an actionable notification with buttons/actions to Home Assistant companion app",
				InputSchema: objectSchema(
					map[string]interface{}{
						"message": stringProperty("Notification message text"),
						"title":   stringProperty("Notification title"),
						"actions": stringProperty("Actions as JSON array: [{action: string, title: string}]"),
					},
					[]string{"message", "actions"},
				),
			},
			{
				Name:        "homeassistant_send_command",
				Description: "Send a command to Home Assistant companion app (e.g., update_sensors, request_location_update, command_screen_on)",
				InputSchema: objectSchema(
					map[string]interface{}{
						"command": stringProperty("Command message (e.g., 'command_update_sensors', 'request_location_update')"),
						"data":    stringProperty("Additional command data as JSON string (optional)"),
					},
					[]string{"command"},
				),
			},
		}...)
	}

	return tools
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
	// Discord tools
	case "discord_send_notification":
		return p.discordSendNotification(args)
	case "discord_send_embed":
		return p.discordSendEmbed(args)
	case "discord_send_reaction":
		return p.discordSendReaction(args)
	case "discord_send_message_with_buttons":
		return p.discordSendMessageWithButtons(args)
	// Home Assistant tools
	case "homeassistant_send_notification":
		return p.haSendNotification(args)
	case "homeassistant_send_actionable_notification":
		return p.haSendActionableNotification(args)
	case "homeassistant_send_command":
		return p.haSendCommand(args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// --- Discord Tool Implementations ---

func (p *Provider) discordSendNotification(args map[string]interface{}) (interface{}, error) {
	message, err := getStringRequired(args, "message")
	if err != nil {
		return nil, err
	}
	title := getString(args, "title")
	channelName := getString(args, "channel_name")

	botToken, err := getDiscordBotToken()
	if err != nil {
		return nil, err
	}

	channelID, err := getChannelID(channelName)
	if err != nil {
		return nil, err
	}

	content := message
	if title != "" {
		content = fmt.Sprintf("**%s**\n%s", title, message)
	}

	payload := map[string]interface{}{
		"content": content,
	}

	result, err := discordAPICall(botToken, channelID, "messages", "POST", payload)
	if err != nil {
		return nil, fmt.Errorf("failed to send Discord notification: %w", err)
	}

	response := map[string]interface{}{
		"success":    true,
		"message_id": result["id"],
		"channel_id": channelID,
		"timestamp":  result["timestamp"],
	}
	output, _ := json.MarshalIndent(response, "", "  ")
	return textContent(string(output)), nil
}

func (p *Provider) discordSendEmbed(args map[string]interface{}) (interface{}, error) {
	title, err := getStringRequired(args, "title")
	if err != nil {
		return nil, err
	}
	description := getString(args, "description")
	colorName := getString(args, "color")
	fieldsJSON := getString(args, "fields")
	footer := getString(args, "footer")
	channelName := getString(args, "channel_name")

	botToken, err := getDiscordBotToken()
	if err != nil {
		return nil, err
	}

	channelID, err := getChannelID(channelName)
	if err != nil {
		return nil, err
	}

	// Map color names to Discord color values
	colorMap := map[string]int{
		"success": 0x00ff00, // Green
		"error":   0xff0000, // Red
		"warning": 0xffff00, // Yellow
		"info":    0x0099ff, // Blue
	}
	color := colorMap["info"]
	if c, ok := colorMap[colorName]; ok {
		color = c
	}

	// Parse fields if provided
	var fields []map[string]interface{}
	if fieldsJSON != "" {
		if err := json.Unmarshal([]byte(fieldsJSON), &fields); err != nil {
			return nil, fmt.Errorf("invalid fields JSON: %w", err)
		}
	}

	embed := map[string]interface{}{
		"title":       title,
		"description": description,
		"color":       color,
		"fields":      fields,
	}
	if footer != "" {
		embed["footer"] = map[string]string{"text": footer}
	}

	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{embed},
	}

	result, err := discordAPICall(botToken, channelID, "messages", "POST", payload)
	if err != nil {
		return nil, fmt.Errorf("failed to send Discord embed: %w", err)
	}

	response := map[string]interface{}{
		"success":    true,
		"message_id": result["id"],
		"channel_id": channelID,
		"timestamp":  result["timestamp"],
	}
	output, _ := json.MarshalIndent(response, "", "  ")
	return textContent(string(output)), nil
}

func (p *Provider) discordSendReaction(args map[string]interface{}) (interface{}, error) {
	messageID, err := getStringRequired(args, "message_id")
	if err != nil {
		return nil, err
	}
	emoji, err := getStringRequired(args, "emoji")
	if err != nil {
		return nil, err
	}
	channelName := getString(args, "channel_name")

	botToken, err := getDiscordBotToken()
	if err != nil {
		return nil, err
	}

	channelID, err := getChannelID(channelName)
	if err != nil {
		return nil, err
	}

	// URL-encode the emoji
	encodedEmoji := url.PathEscape(emoji)

	// Add reaction via Discord API
	apiURL := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages/%s/reactions/%s/@me",
		channelID, messageID, encodedEmoji)

	req, err := http.NewRequest("PUT", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bot "+botToken)
	req.Header.Set("Content-Length", "0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Discord API error: %s - %s", resp.Status, string(body))
	}

	response := map[string]interface{}{
		"success":    true,
		"message_id": messageID,
		"emoji":      emoji,
	}
	output, _ := json.MarshalIndent(response, "", "  ")
	return textContent(string(output)), nil
}

func (p *Provider) discordSendMessageWithButtons(args map[string]interface{}) (interface{}, error) {
	buttonsJSON, err := getStringRequired(args, "buttons")
	if err != nil {
		return nil, err
	}
	message := getString(args, "message")
	embedTitle := getString(args, "embed_title")
	embedDescription := getString(args, "embed_description")
	embedColor := getString(args, "embed_color")
	channelName := getString(args, "channel_name")

	botToken, err := getDiscordBotToken()
	if err != nil {
		return nil, err
	}

	channelID, err := getChannelID(channelName)
	if err != nil {
		return nil, err
	}

	// Parse buttons
	var buttons []map[string]interface{}
	if err := json.Unmarshal([]byte(buttonsJSON), &buttons); err != nil {
		return nil, fmt.Errorf("invalid buttons JSON: %w", err)
	}

	// Build button components
	buttonComponents := make([]map[string]interface{}, len(buttons))
	for i, btn := range buttons {
		style := 1 // Default to Primary (blue)
		if s, ok := btn["style"].(float64); ok {
			style = int(s)
		}
		buttonComponents[i] = map[string]interface{}{
			"type":      2, // Button
			"label":     btn["label"],
			"custom_id": btn["custom_id"],
			"style":     style,
		}
		if url, ok := btn["url"].(string); ok {
			buttonComponents[i]["url"] = url
		}
	}

	components := []map[string]interface{}{
		{
			"type":       1, // Action Row
			"components": buttonComponents,
		},
	}

	payload := map[string]interface{}{
		"components": components,
	}

	// Add embed if provided
	if embedTitle != "" || embedDescription != "" {
		colorMap := map[string]int{
			"success": 0x00ff00,
			"error":   0xff0000,
			"warning": 0xffff00,
			"info":    0x0099ff,
		}
		color := colorMap["info"]
		if c, ok := colorMap[embedColor]; ok {
			color = c
		}

		payload["embeds"] = []map[string]interface{}{
			{
				"title":       embedTitle,
				"description": embedDescription,
				"color":       color,
			},
		}
	}

	// Add plain text message if provided
	if message != "" {
		payload["content"] = message
	}

	result, err := discordAPICall(botToken, channelID, "messages", "POST", payload)
	if err != nil {
		return nil, fmt.Errorf("failed to send message with buttons: %w", err)
	}

	response := map[string]interface{}{
		"success":    true,
		"message_id": result["id"],
		"channel_id": channelID,
		"timestamp":  result["timestamp"],
		"note":       "Button interactions need to be handled by Kimaki bot",
	}
	output, _ := json.MarshalIndent(response, "", "  ")
	return textContent(string(output)), nil
}

func discordAPICall(botToken, channelID, endpoint, method string, payload interface{}) (map[string]interface{}, error) {
	apiURL := fmt.Sprintf("https://discord.com/api/v10/channels/%s/%s", channelID, endpoint)

	var reqBody io.Reader
	if payload != nil {
		jsonBody, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, apiURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bot "+botToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Discord API error: %s - %s", resp.Status, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

// --- Home Assistant Tool Implementations ---

func (p *Provider) haSendNotification(args map[string]interface{}) (interface{}, error) {
	message, err := getStringRequired(args, "message")
	if err != nil {
		return nil, err
	}
	title := getString(args, "title")
	dataJSON := getString(args, "data")

	config, err := getHomeAssistantConfig()
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"message": message,
	}
	if title != "" {
		payload["title"] = title
	}
	if dataJSON != "" {
		var data interface{}
		if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
			return nil, fmt.Errorf("invalid data JSON: %w", err)
		}
		payload["data"] = data
	}

	if err := haAPICall(config, payload); err != nil {
		return nil, fmt.Errorf("failed to send notification: %w", err)
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Notification sent successfully",
		"title":   title,
		"sent_to": config.NotifyService,
	}
	output, _ := json.MarshalIndent(response, "", "  ")
	return textContent(string(output)), nil
}

func (p *Provider) haSendActionableNotification(args map[string]interface{}) (interface{}, error) {
	message, err := getStringRequired(args, "message")
	if err != nil {
		return nil, err
	}
	actionsJSON, err := getStringRequired(args, "actions")
	if err != nil {
		return nil, err
	}
	title := getString(args, "title")

	config, err := getHomeAssistantConfig()
	if err != nil {
		return nil, err
	}

	var actions interface{}
	if err := json.Unmarshal([]byte(actionsJSON), &actions); err != nil {
		return nil, fmt.Errorf("invalid actions JSON: %w", err)
	}

	payload := map[string]interface{}{
		"message": message,
		"title":   title,
		"data": map[string]interface{}{
			"actions": actions,
		},
	}

	if err := haAPICall(config, payload); err != nil {
		return nil, fmt.Errorf("failed to send actionable notification: %w", err)
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Actionable notification sent successfully",
		"actions": actions,
	}
	output, _ := json.MarshalIndent(response, "", "  ")
	return textContent(string(output)), nil
}

func (p *Provider) haSendCommand(args map[string]interface{}) (interface{}, error) {
	command, err := getStringRequired(args, "command")
	if err != nil {
		return nil, err
	}
	dataJSON := getString(args, "data")

	config, err := getHomeAssistantConfig()
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"message": command,
	}
	if dataJSON != "" {
		var data interface{}
		if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
			return nil, fmt.Errorf("invalid data JSON: %w", err)
		}
		payload["data"] = data
	}

	if err := haAPICall(config, payload); err != nil {
		return nil, fmt.Errorf("failed to send command: %w", err)
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Command sent successfully",
		"command": command,
	}
	output, _ := json.MarshalIndent(response, "", "  ")
	return textContent(string(output)), nil
}

func haAPICall(config *homeAssistantConfig, payload interface{}) error {
	// Build the API endpoint
	// notify.mobile_app_xxx -> notify/mobile_app_xxx
	service := strings.Replace(config.NotifyService, "notify.", "", 1)
	service = strings.Replace(service, "mobile_app_", "notify/mobile_app_", 1)
	apiURL := fmt.Sprintf("%s/api/services/%s", config.ServerURL, service)

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+config.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Home Assistant API error (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// --- Prompts ---

// Prompts returns all prompts provided by the Discord/notifications provider
func (p *Provider) Prompts() []tools.Prompt {
	var prompts []tools.Prompt

	if p.discordAvailable {
		prompts = append(prompts, []tools.Prompt{
			{
				Name:        "discord_notify_job_result",
				Description: "Send a formatted job execution result to Discord with appropriate styling based on success/failure",
				Arguments: []tools.PromptArgument{
					{Name: "job_name", Description: "Name of the job that ran", Required: true},
					{Name: "success", Description: "Whether the job succeeded (true/false)", Required: true},
					{Name: "output", Description: "Job output or error message", Required: false},
					{Name: "channel", Description: "Discord channel name (default: diane)", Required: false},
				},
			},
			{
				Name:        "discord_daily_summary",
				Description: "Send a daily summary notification to Discord with key metrics and status",
				Arguments: []tools.PromptArgument{
					{Name: "title", Description: "Summary title (e.g., 'Daily Report')", Required: true},
					{Name: "metrics", Description: "JSON object with key metrics to display", Required: false},
				},
			},
			{
				Name:        "discord_ask_confirmation",
				Description: "Send an interactive message with Yes/No buttons for user confirmation",
				Arguments: []tools.PromptArgument{
					{Name: "question", Description: "The question to ask", Required: true},
					{Name: "context", Description: "Additional context for the question", Required: false},
				},
			},
		}...)
	}

	return prompts
}

// GetPrompt returns a prompt with arguments substituted
func (p *Provider) GetPrompt(name string, args map[string]string) ([]tools.PromptMessage, error) {
	switch name {
	case "discord_notify_job_result":
		return p.promptNotifyJobResult(args), nil
	case "discord_daily_summary":
		return p.promptDailySummary(args), nil
	case "discord_ask_confirmation":
		return p.promptAskConfirmation(args), nil
	default:
		return nil, fmt.Errorf("prompt not found: %s", name)
	}
}

func getArgOrDefault(args map[string]string, key, defaultVal string) string {
	if val, ok := args[key]; ok && val != "" {
		return val
	}
	return defaultVal
}

func (p *Provider) promptNotifyJobResult(args map[string]string) []tools.PromptMessage {
	jobName := getArgOrDefault(args, "job_name", "unknown")
	success := getArgOrDefault(args, "success", "true")
	output := getArgOrDefault(args, "output", "")
	channel := getArgOrDefault(args, "channel", "diane")

	color := "success"
	statusEmoji := "checkmark"
	statusText := "Success"
	if success != "true" {
		color = "error"
		statusEmoji = "x"
		statusText = "Failed"
	}

	outputSection := ""
	if output != "" {
		outputSection = fmt.Sprintf(`
4. If there is output, add it as a field:
   - name: "Output"
   - value: %q (truncate to 1000 chars if longer)`, output)
	}

	return []tools.PromptMessage{
		{
			Role: "user",
			Content: tools.PromptContent{
				Type: "text",
				Text: fmt.Sprintf(`Send a Discord notification about job execution result.

**Job Details:**
- Job Name: %s
- Status: %s
- Channel: %s

**Instructions:**
1. Use discord_send_embed to send a formatted notification
2. Set color to "%s"
3. Title should be: "Job: %s - %s"
%s
5. Add a footer with the current timestamp

This notification should be clear and scannable at a glance.`, jobName, statusText, channel, color, jobName, statusEmoji, outputSection),
			},
		},
	}
}

func (p *Provider) promptDailySummary(args map[string]string) []tools.PromptMessage {
	title := getArgOrDefault(args, "title", "Daily Summary")
	metrics := getArgOrDefault(args, "metrics", "{}")

	return []tools.PromptMessage{
		{
			Role: "user",
			Content: tools.PromptContent{
				Type: "text",
				Text: fmt.Sprintf(`Send a daily summary notification to Discord.

**Summary Details:**
- Title: %s
- Metrics: %s

**Instructions:**
1. Use discord_send_embed to create a rich summary
2. Set color to "info" (blue)
3. Parse the metrics JSON and create fields for each key-value pair
4. Make numeric values stand out
5. Add a footer with "Generated at [current time]"

The summary should be visually organized and easy to scan.`, title, metrics),
			},
		},
	}
}

func (p *Provider) promptAskConfirmation(args map[string]string) []tools.PromptMessage {
	question := getArgOrDefault(args, "question", "Do you want to proceed?")
	context := getArgOrDefault(args, "context", "")

	contextSection := ""
	if context != "" {
		contextSection = fmt.Sprintf("\n- Context: %s", context)
	}

	return []tools.PromptMessage{
		{
			Role: "user",
			Content: tools.PromptContent{
				Type: "text",
				Text: fmt.Sprintf(`Send an interactive confirmation request to Discord.

**Question Details:**
- Question: %s%s

**Instructions:**
1. Use discord_send_message_with_buttons
2. Set embed_title to the question
3. If context is provided, set embed_description to the context
4. Set embed_color to "warning" (yellow) to draw attention
5. Add two buttons:
   - label: "Yes", custom_id: "confirm_yes", style: 3 (green)
   - label: "No", custom_id: "confirm_no", style: 4 (red)

The message should clearly present the choice to the user.`, question, contextSection),
			},
		},
	}
}
