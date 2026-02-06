# Diane - Personal AI Assistant

A high-performance MCP (Model Context Protocol) server providing 69+ native Go tools for personal productivity automation.

## Features

- **Native Go Performance** - Single binary, no runtime dependencies
- **69+ Tools** - Comprehensive coverage for personal automation
- **MCP Protocol** - Works with any MCP-compatible AI client (OpenCode, Claude Desktop, etc.)
- **Multi-Platform** - macOS (Intel/Apple Silicon) and Linux (x64/ARM64)

## Tool Categories

| Provider | Tools | Description |
|----------|-------|-------------|
| **Apple** | 4 | Reminders, Contacts (macOS only) |
| **Google** | 16 | Gmail, Drive, Sheets, Calendar |
| **Finance** | 28 | Enable Banking, Actual Budget, Bank Sync |
| **Discord** | 4 | Notifications via Discord bot |
| **Weather** | 2 | yr.no forecasts (no API key needed) |
| **Google Places** | 3 | Search, details, nearby places |
| **GitHub Bot** | 3 | Comments, reactions, labels as bot |
| **Infrastructure** | 7 | Cloudflare DNS management |
| **Home Assistant** | 3 | Notifications and commands |

## Quick Install

```bash
# Using curl (when repo is public)
curl -fsSL https://raw.githubusercontent.com/Emergent-Comapny/diane/main/install.sh | sh

# Using GitHub CLI
gh release download --repo Emergent-Comapny/diane --pattern "diane-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m | sed 's/x86_64/amd64/; s/aarch64/arm64/').tar.gz" -O - | tar xz
mkdir -p ~/.diane/bin && mv diane-mcp ~/.diane/bin/
```

## Manual Download

Download the latest release for your platform:

| Platform | Architecture | Download |
|----------|--------------|----------|
| macOS | Apple Silicon (M1/M2/M3) | [diane-darwin-arm64.tar.gz](https://github.com/Emergent-Comapny/diane/releases/latest/download/diane-darwin-arm64.tar.gz) |
| macOS | Intel | [diane-darwin-amd64.tar.gz](https://github.com/Emergent-Comapny/diane/releases/latest/download/diane-darwin-amd64.tar.gz) |
| Linux | x64 | [diane-linux-amd64.tar.gz](https://github.com/Emergent-Comapny/diane/releases/latest/download/diane-linux-amd64.tar.gz) |
| Linux | ARM64 | [diane-linux-arm64.tar.gz](https://github.com/Emergent-Comapny/diane/releases/latest/download/diane-linux-arm64.tar.gz) |

## Configuration

### OpenCode

Add to your `opencode.json`:

```json
{
  "mcp": {
    "diane": {
      "type": "local",
      "command": ["~/.diane/bin/diane-mcp"]
    }
  }
}
```

### Claude Desktop

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "diane": {
      "command": "~/.diane/bin/diane-mcp"
    }
  }
}
```

## Tool Configuration

Each tool category requires its own configuration. Create config files in `~/.diane/secrets/` or the directory where diane-mcp is running.

### Required Config Files

| Tool Category | Config File | Description |
|---------------|-------------|-------------|
| Google (Gmail, Drive, Sheets, Calendar) | Uses `gog` CLI | Install [gog](https://github.com/your-gog-repo) and authenticate |
| Apple (Reminders, Contacts) | macOS only | Uses native `remindctl` and `contacts-cli` |
| Enable Banking | `enablebanking-config.json` | PSD2 Open Banking credentials |
| Actual Budget | `actualbudget-config.json` | Server URL and sync credentials |
| Discord | `discord-config.json` | Bot token for notifications |
| Cloudflare | `cloudflare-config.json` | API token for DNS management |
| Google Places | `google-places-config.json` | Google Places API key |
| Home Assistant | `homeassistant-config.json` | HA URL and webhook token |
| GitHub Bot | `github-bot-private-key.pem` | GitHub App private key |

### Example Config (Enable Banking)

```json
{
  "app_id": "your-app-id",
  "private_key_path": "~/.diane/secrets/enablebanking-private.pem"
}
```

## Building from Source

```bash
# Clone the repository
git clone https://github.com/Emergent-Comapny/diane.git
cd diane

# Build for current platform
make build

# Build for all platforms
make build-all

# Install locally
make install
```

## Requirements

### macOS
- Go 1.23+ (for building)
- `remindctl` - Apple Reminders CLI (`brew install keith/formulae/remindctl`)
- `contacts-cli` - Apple Contacts CLI (`brew install keith/formulae/contacts-cli`)
- `gog` - Google Workspace CLI (for Gmail, Drive, Sheets, Calendar)

### Linux
- Go 1.23+ (for building)
- `gog` - Google Workspace CLI

## License

MIT License - See [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! Please open an issue or pull request.
