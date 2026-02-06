# Diane

Your personal AI assistant for automating life's tedious tasks.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/diane-assistant/diane/main/install.sh | sh
```

Works on macOS and Linux. The installer auto-detects your platform and installs to `~/.diane/bin/`.

---

## What can Diane do?

Diane connects to your AI tools (OpenCode, Claude Desktop, Cursor) and gives them superpowers to help with your daily life:

| | What Diane helps with |
|---|---|
| **Email & Docs** | Read, search, send emails. Create and edit Google Docs, Sheets, Slides. |
| **Calendar** | Check your schedule, create events, find free time. |
| **Money** | Connect to your bank accounts (PSD2), track spending, sync with budgets. |
| **Reminders** | Create and manage Apple Reminders (macOS). |
| **Contacts** | Look up people in your address book. |
| **Weather** | Get forecasts for any location. |
| **Places** | Find restaurants, shops, services nearby. |
| **Smart Home** | Control Home Assistant devices, send notifications. |
| **Infrastructure** | Manage Cloudflare DNS, GitHub repos. |

69+ tools, native Go performance, single binary.

## Connect to your AI

Diane speaks MCP (Model Context Protocol), so it works with any compatible client.

### OpenCode

Add to `opencode.json`:

```json
{
  "mcp": {
    "diane": {
      "type": "local",
      "command": ["~/.diane/bin/diane"]
    }
  }
}
```

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "diane": {
      "command": "~/.diane/bin/diane"
    }
  }
}
```

## Setup

Each capability requires its own credentials. Place config files in `~/.diane/secrets/`:

| Capability | Config File | Notes |
|------------|-------------|-------|
| Google (Gmail, Drive, Calendar) | — | Uses [gog](https://github.com/diane-assistant/gog) CLI for auth |
| Apple (Reminders, Contacts) | — | macOS only, uses `remindctl` and `contacts-cli` |
| Banking | `enablebanking-config.json` | PSD2 Open Banking credentials |
| Budgets | `actualbudget-config.json` | Actual Budget server URL |
| Discord | `discord-config.json` | Bot token |
| Cloudflare | `cloudflare-config.json` | API token |
| Places | `google-places-config.json` | Google Places API key |
| Home Assistant | `homeassistant-config.json` | URL and webhook token |
| GitHub Bot | `github-bot-private-key.pem` | GitHub App private key |

See [INSTALL.md](INSTALL.md) for detailed setup instructions.

## Manual Install

| Platform | Architecture | Download |
|----------|--------------|----------|
| macOS | Apple Silicon | [diane-darwin-arm64.tar.gz](https://github.com/diane-assistant/diane/releases/latest/download/diane-darwin-arm64.tar.gz) |
| macOS | Intel | [diane-darwin-amd64.tar.gz](https://github.com/diane-assistant/diane/releases/latest/download/diane-darwin-amd64.tar.gz) |
| Linux | x64 | [diane-linux-amd64.tar.gz](https://github.com/diane-assistant/diane/releases/latest/download/diane-linux-amd64.tar.gz) |
| Linux | ARM64 | [diane-linux-arm64.tar.gz](https://github.com/diane-assistant/diane/releases/latest/download/diane-linux-arm64.tar.gz) |

```bash
tar xzf diane-*.tar.gz
mkdir -p ~/.diane/bin && mv diane ~/.diane/bin/
```

## Build from Source

```bash
git clone https://github.com/diane-assistant/diane.git
cd diane
make build && make install
```

Requires Go 1.23+.

## License

MIT
