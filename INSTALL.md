# Installing Diane

Your personal AI assistant for automating life's tedious tasks.

## Quick Install

```bash
curl -fsSL https://raw.githubusercontent.com/diane-assistant/diane/main/install.sh | sh
```

This installs the latest version to `~/.diane/bin/diane`.

### Options

```bash
# Install specific version
DIANE_VERSION=v1.0.0 curl -fsSL https://raw.githubusercontent.com/diane-assistant/diane/main/install.sh | sh

# Install to custom directory
DIANE_DIR=/opt/diane curl -fsSL https://raw.githubusercontent.com/diane-assistant/diane/main/install.sh | sh
```

## Manual Download

Download binaries from [GitHub Releases](https://github.com/diane-assistant/diane/releases):

| Platform | Architecture | Download |
|----------|--------------|----------|
| macOS    | Apple Silicon (M1/M2/M3) | `diane-darwin-arm64.tar.gz` |
| macOS    | Intel | `diane-darwin-amd64.tar.gz` |
| Linux    | x86_64 | `diane-linux-amd64.tar.gz` |
| Linux    | ARM64 | `diane-linux-arm64.tar.gz` |

## Connect to Your AI

Diane speaks MCP, so it works with any compatible AI client.

### OpenCode

Add to your `opencode.json`:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "diane-personal": {
      "type": "remote",
      "url": "http://localhost:8765/mcp/sse?context=personal",
      "oauth": false
    }
  }
}
```

Or install automatically with:

```bash
diane mcp install opencode
```

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "diane": {
      "command": "/Users/YOUR_USERNAME/.diane/bin/diane"
    }
  }
}
```

## Available Tools (69+)

### Apple (macOS only)
- `reminders_list_reminders` - List Apple Reminders
- `reminders_add_reminder` - Add reminder with due date
- `contacts_list_all_contacts` - List all contacts
- `contacts_search_contacts` - Search contacts

### Google Workspace
- `gmail_search_emails` - Search Gmail
- `gmail_read_email` - Read email by ID
- `drive_search_files` - Search Google Drive
- `drive_list_files` - List recent files
- `sheets_get_sheet` - Read spreadsheet data
- `sheets_update_sheet` - Update cells
- `sheets_append_sheet` - Append rows
- `calendar_list_events` - List calendar events
- `calendar_create_event` - Create events
- And more...

### Finance
- `enablebanking_list_banks` - List available banks (PSD2)
- `enablebanking_get_transactions` - Fetch bank transactions
- `actualbudget_import_transactions` - Import to Actual Budget
- `banksync_sync_all_accounts` - Sync bank to budget
- And 24 more finance tools...

### Weather
- `weather_get_weather` - Get forecast by coordinates
- `weather_search_location_weather` - Search location + forecast

### GitHub Bot
- `github-bot_comment_as_bot` - Comment as Diane bot
- `github-bot_react_as_bot` - Add reactions
- `github-bot_manage_labels` - Manage issue labels

### Google Places
- `google-places_search_places` - Search for places
- `google-places_get_place_details` - Get place details
- `google-places_find_nearby_places` - Find nearby places

### Notifications
- `discord_send_notification` - Send Discord message
- `discord_send_embed` - Rich Discord embeds
- `homeassistant_send_notification` - Home Assistant alerts

### Infrastructure
- `cloudflare_list_zones` - List domains
- `cloudflare_list_dns_records` - List DNS records
- `cloudflare_create_dns_record` - Create DNS record
- And 4 more Cloudflare tools...

### Cron Jobs
- `job_list` - List scheduled jobs
- `job_add` - Create new job
- `job_enable` / `job_disable` - Toggle jobs
- `job_logs` - View execution logs

## Proxy Other Tools

Diane can also proxy other MCP servers. Configure them in `~/.diane/mcp-config.json`:

```json
{
  "servers": {
    "context7": {
      "command": ["npx", "-y", "@context7/mcp-server"]
    },
    "infakt": {
      "command": ["uvx", "infakt-mcp"],
      "env": {
        "INFAKT_API_KEY": "your-key"
      }
    }
  }
}
```

## Building from Source

```bash
git clone https://github.com/diane-assistant/diane.git
cd diane
make build      # Build for current platform
make install    # Install to ~/.diane/bin/
make build-all  # Build for all platforms
```

## Uninstall

```bash
curl -fsSL https://raw.githubusercontent.com/diane-assistant/diane/main/install.sh | sh -s uninstall
# Or manually:
rm -rf ~/.diane
```

## License

MIT
