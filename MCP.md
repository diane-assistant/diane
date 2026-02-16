# Diane MCP Configuration

Diane acts as both an MCP server and an MCP proxy, allowing you to:
1. **Use Diane as an MCP server** - Connect your AI tools to Diane's 69+ tools
2. **Use Diane as an MCP proxy** - Connect to other MCP servers through Diane

## Using Diane as an MCP Server

Diane supports multiple connection methods:
- **stdio** - For local tools like OpenCode, Claude Desktop, Cursor (default)
- **HTTP Streamable** - For network-based clients (port 8765)
- **SSE** - For Server-Sent Events transport (port 8765)

### stdio (Local Tools)

#### OpenCode

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

**Note:** This connects to a running Diane instance via SSE. Make sure Diane is running (via Diane or command line) before starting OpenCode.

#### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `~/.config/claude/claude_desktop_config.json` (Linux):

```json
{
  "mcpServers": {
    "diane": {
      "command": "/Users/YOUR_USERNAME/.diane/bin/diane"
    }
  }
}
```

#### Cursor

Add to your Cursor MCP settings:

```json
{
  "mcpServers": {
    "diane": {
      "command": "/Users/YOUR_USERNAME/.diane/bin/diane"
    }
  }
}
```

#### Windsurf / Continue

```json
{
  "mcpServers": {
    "diane": {
      "command": "/Users/YOUR_USERNAME/.diane/bin/diane"
    }
  }
}
```

### HTTP Streamable (Network Clients)

When Diane is running, it exposes an HTTP Streamable MCP endpoint on port 8765.

#### Configuration for Network Clients

```json
{
  "mcpServers": {
    "diane": {
      "type": "http",
      "url": "http://localhost:8765/mcp"
    }
  }
}
```

#### Manual Testing

```bash
# Initialize a session
curl -X POST http://localhost:8765/mcp \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}'

# The response includes MCP-Session-Id header for subsequent requests
```

### SSE (Server-Sent Events)

Diane also supports SSE transport on the same port. Note: The SSE connection must remain open for the session to stay valid.

```bash
# Connect to SSE endpoint (connection must stay open)
curl -N http://localhost:8765/mcp/sse

# Response:
# event: endpoint
# data: /mcp/message?session=<session-id>
```

Send messages to the returned endpoint (while SSE connection is open):

```bash
curl -X POST "http://localhost:8765/mcp/message?session=<session-id>" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{...}}'
```

**Note:** For most clients, we recommend using **HTTP Streamable** (`/mcp`) rather than SSE, as it's simpler and doesn't require maintaining an open connection.

### Health Check

```bash
curl http://localhost:8765/health
# {"status":"ok"}
```

---

## Multiple AI Clients (Multi-Consumer Setup)

When using multiple AI tools (OpenCode, Claude Desktop, Cursor, etc.) simultaneously, you'll encounter a conflict because Diane only allows one instance to run at a time. The **stdio transport** spawns a new Diane process for each client, but the second attempt fails due to Diane's single-instance lock.

**Solution: Use HTTP/SSE transport for additional clients.**

### How It Works

1. **First client** or the **Diane app** starts Diane (via stdio or directly)
2. **Additional clients** connect via HTTP to the already-running Diane instance
3. All clients share the same Diane instance but get independent sessions

### Recommended Setup

#### Option 1: All Clients via HTTP (Recommended)

Start Diane once (via Diane or command line), then configure all clients to use HTTP:

**OpenCode** (`opencode.json`):
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

**Claude Desktop** (`claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "diane": {
      "type": "http",
      "url": "http://localhost:8765/mcp"
    }
  }
}
```

**Cursor** (MCP settings):
```json
{
  "mcpServers": {
    "diane": {
      "type": "http",
      "url": "http://localhost:8765/mcp"
    }
  }
}
```

#### Option 2: Hybrid Setup

One primary client starts Diane via stdio, others connect via HTTP:

**Primary Client (e.g., Claude Desktop)** - starts Diane:
```json
{
  "mcpServers": {
    "diane": {
      "command": "/Users/YOUR_USERNAME/.diane/bin/diane"
    }
  }
}
```

**Secondary Clients (e.g., OpenCode, Cursor)** - connect via HTTP:
```json
{
  "mcp": {
    "diane-personal": {
      "type": "remote",
      "url": "http://localhost:8765/mcp/sse?context=personal",
      "oauth": false
    }
  }
}
```

### Checking Connection Status

Verify Diane is running and accepting HTTP connections:

```bash
# Check health
curl http://localhost:8765/health
# {"status":"ok"}

# Test MCP connection
curl -X POST http://localhost:8765/mcp \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}'
```

### Troubleshooting Multi-Client Issues

| Issue | Solution |
|-------|----------|
| "Another instance of Diane is already running" | This is expected for stdio! Use HTTP transport instead. |
| "Connection refused" on port 8765 | Diane is not running. Start it via Diane or a primary stdio client. |
| Tools not appearing in secondary client | Ensure the client supports HTTP/remote MCP servers. Check client logs. |

---

## Diane as MCP Proxy

Diane can connect to external MCP servers and expose their tools alongside its builtin tools. This is useful for:
- Aggregating multiple MCP servers into one connection
- Adding authentication/headers to MCP connections
- Using MCP servers that require SSE or HTTP transport

### Configuration

MCP servers are configured through the Diane app UI or API. All configuration is stored in the SQLite database at `~/.diane/cron.db`.

### Supported Transport Types

#### stdio (default)

Spawn an MCP server as a subprocess. Most common for CLI-based servers.

```json
{
  "name": "filesystem",
  "enabled": true,
  "type": "stdio",
  "command": "npx",
  "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"],
  "env": {}
}
```

#### http

Connect to an MCP server via HTTP Streamable transport (MCP 2025-06-18+).

```json
{
  "name": "remote-server",
  "enabled": true,
  "type": "http",
  "url": "https://api.example.com/mcp",
  "headers": {
    "Authorization": "Bearer your-token",
    "X-Project-Id": "project-123"
  }
}
```

#### sse

Connect to an MCP server via Server-Sent Events transport.

```json
{
  "name": "sse-server",
  "enabled": true,
  "type": "sse",
  "url": "https://api.example.com/mcp/sse",
  "headers": {
    "X-API-Key": "your-key"
  }
}
```

---

## Example Configurations

### Context7 (Documentation Search)

```json
{
  "name": "context7",
  "enabled": true,
  "type": "stdio",
  "command": "npx",
  "args": ["-y", "@upstash/context7-mcp@latest"],
  "env": {}
}
```

### Filesystem Access

```json
{
  "name": "filesystem",
  "enabled": true,
  "type": "stdio",
  "command": "npx",
  "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/allowed/dir"],
  "env": {}
}
```

### Emergent Knowledge Graph (HTTP)

```json
{
  "name": "emergent",
  "enabled": true,
  "type": "http",
  "url": "http://localhost:3002/api/mcp",
  "headers": {
    "X-API-Key": "your-api-key",
    "X-Project-Id": "your-project-id"
  }
}
```

### Brave Search

```json
{
  "name": "brave-search",
  "enabled": true,
  "type": "stdio",
  "command": "npx",
  "args": ["-y", "@anthropics/mcp-server-brave-search"],
  "env": {
    "BRAVE_API_KEY": "your-brave-api-key"
  }
}
```

---

## Managing MCP Servers

### Reload Configuration

After adding or updating MCP server configuration, reload without restarting:

```bash
diane reload
```

### Check Server Status

```bash
diane mcp-servers
```

### Restart a Specific Server

```bash
diane restart <server-name>
```

---

## Tool Naming

When proxying external MCP servers, Diane prefixes tool names with the server name to avoid conflicts:

- Server `context7` tool `resolve-library-id` becomes `context7_resolve-library-id`
- Server `filesystem` tool `read_file` becomes `filesystem_read_file`

Builtin Diane tools (apple, google, weather, etc.) are not prefixed.

---

## Troubleshooting

### Server shows disconnected

Check the error message in the Diane app or via:

```bash
diane mcp-servers
```

Common issues:
- **"exec: no command"** - Missing `command` field for stdio type
- **"failed to initialize"** - Server started but MCP handshake failed
- **"connection refused"** - HTTP/SSE server not reachable

### View server logs

Stderr from stdio servers is logged to `~/.diane/server.log`.

### Test HTTP connectivity

```bash
curl -X POST "https://your-server/mcp" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}'
```
