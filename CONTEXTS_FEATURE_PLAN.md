# Diane Contexts Feature - Implementation Plan

## Overview

Add context-based MCP aggregation to Diane, allowing users to group MCP servers into contexts (e.g., `personal`, `work`, `dev`) with granular tool-level control. Consumers (OpenCode, Claude, Cursor) can specify which context to use when connecting.

## Goals

1. Define contexts that group MCP servers together
2. Enable/disable entire MCPs per context
3. Enable/disable individual tools per MCP per context
4. Provide ready-to-copy connection instructions per context
5. Migrate MCP configuration from JSON file to SQLite database
6. Build UI in Diane for context management

---

## Phase 1: Database Schema Migration

### 1.1 Create Migration File

**File:** `server/internal/db/migrations/003_contexts.sql`

```sql
-- MCP Servers (source of truth for available servers)
CREATE TABLE IF NOT EXISTS mcp_servers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    enabled BOOLEAN DEFAULT true,
    type TEXT NOT NULL,              -- 'stdio', 'sse', 'http', 'builtin'
    command TEXT,
    args TEXT,                       -- JSON array
    env TEXT,                        -- JSON object  
    url TEXT,
    headers TEXT,                    -- JSON object
    oauth TEXT,                      -- JSON object
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Contexts
CREATE TABLE IF NOT EXISTS contexts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    description TEXT,
    is_default BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Context-Server relationship
CREATE TABLE IF NOT EXISTS context_servers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    context_id INTEGER NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    server_id INTEGER NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(context_id, server_id)
);

-- Tool overrides per context-server
CREATE TABLE IF NOT EXISTS context_server_tools (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    context_server_id INTEGER NOT NULL REFERENCES context_servers(id) ON DELETE CASCADE,
    tool_name TEXT NOT NULL,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(context_server_id, tool_name)
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_context_servers_context ON context_servers(context_id);
CREATE INDEX IF NOT EXISTS idx_context_servers_server ON context_servers(server_id);
CREATE INDEX IF NOT EXISTS idx_context_server_tools_cs ON context_server_tools(context_server_id);

-- Insert default context
INSERT OR IGNORE INTO contexts (name, description, is_default) 
VALUES ('personal', 'Personal productivity tools', true);
```

### 1.2 Update DB Initialization

**File:** `server/internal/db/db.go`

- Add migration runner for `003_contexts.sql`
- Ensure migrations run on startup

### 1.3 Tasks

- [ ] Create migration SQL file
- [ ] Update `db.go` to run migration
- [ ] Test migration on fresh DB
- [ ] Test migration on existing DB with jobs

---

## Phase 2: Database Access Layer

### 2.1 MCP Server Operations

**File:** `server/internal/db/mcp_servers.go`

```go
type MCPServerRow struct {
    ID        int64             `json:"id"`
    Name      string            `json:"name"`
    Enabled   bool              `json:"enabled"`
    Type      string            `json:"type"`
    Command   string            `json:"command,omitempty"`
    Args      []string          `json:"args,omitempty"`
    Env       map[string]string `json:"env,omitempty"`
    URL       string            `json:"url,omitempty"`
    Headers   map[string]string `json:"headers,omitempty"`
    OAuth     *OAuthConfig      `json:"oauth,omitempty"`
    CreatedAt time.Time         `json:"created_at"`
    UpdatedAt time.Time         `json:"updated_at"`
}

func (db *DB) ListMCPServers() ([]MCPServerRow, error)
func (db *DB) GetMCPServer(name string) (*MCPServerRow, error)
func (db *DB) CreateMCPServer(server *MCPServerRow) error
func (db *DB) UpdateMCPServer(server *MCPServerRow) error
func (db *DB) DeleteMCPServer(name string) error
func (db *DB) ImportMCPServersFromJSON(path string) error
```

### 2.2 Context Operations

**File:** `server/internal/db/contexts.go`

```go
type ContextRow struct {
    ID          int64     `json:"id"`
    Name        string    `json:"name"`
    Description string    `json:"description,omitempty"`
    IsDefault   bool      `json:"is_default"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

type ContextServerRow struct {
    ID         int64  `json:"id"`
    ContextID  int64  `json:"context_id"`
    ServerID   int64  `json:"server_id"`
    ServerName string `json:"server_name"`
    Enabled    bool   `json:"enabled"`
}

type ContextServerToolRow struct {
    ID              int64  `json:"id"`
    ContextServerID int64  `json:"context_server_id"`
    ToolName        string `json:"tool_name"`
    Enabled         bool   `json:"enabled"`
}

// Context CRUD
func (db *DB) ListContexts() ([]ContextRow, error)
func (db *DB) GetContext(name string) (*ContextRow, error)
func (db *DB) GetDefaultContext() (*ContextRow, error)
func (db *DB) CreateContext(ctx *ContextRow) error
func (db *DB) UpdateContext(ctx *ContextRow) error
func (db *DB) DeleteContext(name string) error
func (db *DB) SetDefaultContext(name string) error

// Context-Server relationships
func (db *DB) GetServersForContext(contextName string) ([]ContextServerRow, error)
func (db *DB) AddServerToContext(contextName, serverName string, enabled bool) error
func (db *DB) RemoveServerFromContext(contextName, serverName string) error
func (db *DB) SetServerEnabledInContext(contextName, serverName string, enabled bool) error

// Tool toggles
func (db *DB) GetToolsForContextServer(contextName, serverName string) ([]ContextServerToolRow, error)
func (db *DB) SetToolEnabled(contextName, serverName, toolName string, enabled bool) error
func (db *DB) BulkSetToolsEnabled(contextName, serverName string, tools map[string]bool) error

// Aggregated queries
func (db *DB) GetFullContextConfig(contextName string) (*FullContextConfig, error)
func (db *DB) IsToolEnabledInContext(contextName, serverName, toolName string) (bool, error)
```

### 2.3 Tasks

- [ ] Create `mcp_servers.go` with CRUD operations
- [ ] Create `contexts.go` with CRUD operations
- [ ] Add JSON marshal/unmarshal for Args, Env, Headers, OAuth fields
- [ ] Write unit tests for DB operations
- [ ] Implement `ImportMCPServersFromJSON` for migration from existing config

---

## Phase 3: API Endpoints

### 3.1 MCP Servers API

**File:** `server/internal/api/mcp_servers_api.go`

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/mcp-servers` | List all MCP servers |
| GET | `/api/mcp-servers/{name}` | Get server details |
| POST | `/api/mcp-servers` | Create new server |
| PUT | `/api/mcp-servers/{name}` | Update server |
| DELETE | `/api/mcp-servers/{name}` | Delete server |
| POST | `/api/mcp-servers/import` | Import from JSON file |
| POST | `/api/mcp-servers/{name}/restart` | Restart server |

### 3.2 Contexts API

**File:** `server/internal/api/contexts_api.go`

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/contexts` | List all contexts |
| GET | `/api/contexts/{name}` | Get context with servers & tools |
| POST | `/api/contexts` | Create new context |
| PUT | `/api/contexts/{name}` | Update context metadata |
| DELETE | `/api/contexts/{name}` | Delete context |
| POST | `/api/contexts/{name}/default` | Set as default context |
| GET | `/api/contexts/{name}/connect` | Get connection instructions |

### 3.3 Context-Server API

**File:** `server/internal/api/context_servers_api.go`

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/contexts/{ctx}/servers` | List servers in context |
| PUT | `/api/contexts/{ctx}/servers/{server}` | Add/update server in context |
| DELETE | `/api/contexts/{ctx}/servers/{server}` | Remove server from context |
| GET | `/api/contexts/{ctx}/servers/{server}/tools` | List tools with enabled status |
| PUT | `/api/contexts/{ctx}/servers/{server}/tools` | Bulk update tool toggles |
| PUT | `/api/contexts/{ctx}/servers/{server}/tools/{tool}` | Toggle single tool |

### 3.4 Response Types

```go
// GET /api/contexts/{name}
type ContextDetailResponse struct {
    Context  ContextRow                    `json:"context"`
    Servers  []ContextServerDetailResponse `json:"servers"`
    Summary  ContextSummary                `json:"summary"`
}

type ContextServerDetailResponse struct {
    Server      MCPServerRow `json:"server"`
    Enabled     bool         `json:"enabled"`
    Tools       []ToolStatus `json:"tools"`
    ToolsActive int          `json:"tools_active"`
    ToolsTotal  int          `json:"tools_total"`
}

type ToolStatus struct {
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
    Enabled     bool   `json:"enabled"`
}

type ContextSummary struct {
    ServersEnabled int `json:"servers_enabled"`
    ServersTotal   int `json:"servers_total"`
    ToolsActive    int `json:"tools_active"`
    ToolsTotal     int `json:"tools_total"`
}

// GET /api/contexts/{name}/connect
type ConnectInstructionsResponse struct {
    ContextName string            `json:"context_name"`
    Endpoint    string            `json:"endpoint"`
    Snippets    map[string]string `json:"snippets"`
}
```

### 3.5 Tasks

- [ ] Create `mcp_servers_api.go` with handlers
- [ ] Create `contexts_api.go` with handlers
- [ ] Create `context_servers_api.go` with handlers
- [ ] Register routes in `api.go`
- [ ] Add connection instructions generator
- [ ] Write API integration tests

---

## Phase 4: Update MCP Proxy

### 4.1 Modify Proxy to Use Database

**File:** `server/internal/mcpproxy/proxy.go`

```go
type Proxy struct {
    db         *db.DB
    clients    map[string]Client
    mu         sync.RWMutex
    notifyChan chan string
    initErrors map[string]string
}

// NewProxy creates proxy from database
func NewProxy(database *db.DB) (*Proxy, error)

// LoadServers loads enabled servers from database
func (p *Proxy) LoadServers() error

// Context-aware tool listing
func (p *Proxy) ListToolsForContext(contextName string) ([]map[string]interface{}, error)

// Context-aware tool calling
func (p *Proxy) CallToolForContext(contextName, toolName string, args map[string]interface{}) (json.RawMessage, error)

// Check if tool is enabled in context
func (p *Proxy) isToolEnabledInContext(contextName, serverName, toolName string) bool
```

### 4.2 Update MCP Server

**File:** `server/mcp/server.go`

- Accept context from connection (query param, header, or env var)
- Store context in session state
- Pass context to `listTools()` and `callTool()`

```go
type Session struct {
    ID      string
    Context string  // NEW: active context for this session
    // ...
}

func (s *MCPServer) listTools(session *Session) ([]Tool, error) {
    // Get tools filtered by session.Context
}

func (s *MCPServer) callTool(session *Session, name string, args map[string]interface{}) (interface{}, error) {
    // Verify tool is enabled in session.Context before calling
}
```

### 4.3 Context Resolution Order

1. Query parameter: `?context=work`
2. Header: `X-Diane-Context: work`
3. Environment variable: `DIANE_CONTEXT=work` (stdio mode)
4. Default context from database

### 4.4 Tasks

- [ ] Refactor `Proxy` to accept `*db.DB` instead of config path
- [ ] Implement `ListToolsForContext()`
- [ ] Implement `CallToolForContext()`
- [ ] Add context resolution in HTTP handler
- [ ] Add context resolution in stdio mode
- [ ] Update session to track context
- [ ] Send `notifications/tools/list_changed` when context config changes
- [ ] Write integration tests for context filtering

---

## Phase 5: Builtin Tools Context Support

### 5.1 Register Builtin Tools in Database

On startup, register builtin tool providers as special MCP servers:

```go
builtinServers := []MCPServerRow{
    {Name: "apple", Type: "builtin", Enabled: true},
    {Name: "google", Type: "builtin", Enabled: true},
    {Name: "weather", Type: "builtin", Enabled: true},
    {Name: "notifications", Type: "builtin", Enabled: true},
    {Name: "finance", Type: "builtin", Enabled: true},
    {Name: "infrastructure", Type: "builtin", Enabled: true},
    {Name: "places", Type: "builtin", Enabled: true},
    {Name: "github", Type: "builtin", Enabled: true},
    {Name: "downloads", Type: "builtin", Enabled: true},
}
```

### 5.2 Filter Builtin Tools by Context

**File:** `server/mcp/server.go`

```go
func (s *MCPServer) listTools(session *Session) ([]Tool, error) {
    var tools []Tool
    
    // Builtin tools - filter by context
    for _, provider := range s.providers {
        if !s.isServerEnabledInContext(session.Context, provider.Name()) {
            continue
        }
        for _, tool := range provider.ListTools() {
            if s.isToolEnabledInContext(session.Context, provider.Name(), tool.Name) {
                tools = append(tools, tool)
            }
        }
    }
    
    // Proxied tools - already context-filtered
    proxiedTools, _ := s.proxy.ListToolsForContext(session.Context)
    tools = append(tools, proxiedTools...)
    
    return tools, nil
}
```

### 5.3 Tasks

- [ ] Create builtin server entries in DB on first run
- [ ] Modify tool listing to filter builtins by context
- [ ] Modify tool calling to verify builtin access by context
- [ ] Ensure builtin tools appear in UI with their tool lists

---

## Phase 6: Diane UI

### 6.1 New Views

**Files to create in `Diane/Diane/Views/`:**

| File | Description |
|------|-------------|
| `ContextsView.swift` | Main view with context tabs |
| `ContextDetailView.swift` | Single context configuration |
| `ContextServerRowView.swift` | Expandable MCP server row |
| `ContextToolRowView.swift` | Tool toggle checkbox |
| `ConnectionInstructionsView.swift` | Copy-able connection snippets |
| `AddContextSheet.swift` | Modal for creating new context |
| `AddServerToContextSheet.swift` | Modal for adding server to context |

### 6.2 New Models

**Files to create in `Diane/Diane/Models/`:**

| File | Description |
|------|-------------|
| `Context.swift` | Context model matching API |
| `ContextServer.swift` | Server-in-context model |
| `ContextTool.swift` | Tool toggle model |
| `ConnectionInstructions.swift` | Connection snippets model |

### 6.3 Update DianeClient

**File:** `Diane/Diane/Services/DianeClient.swift`

Add methods:
```swift
// Contexts
func listContexts() async throws -> [Context]
func getContext(name: String) async throws -> ContextDetail
func createContext(name: String, description: String) async throws -> Context
func deleteContext(name: String) async throws
func setDefaultContext(name: String) async throws

// Context servers
func setServerEnabled(context: String, server: String, enabled: Bool) async throws
func setToolEnabled(context: String, server: String, tool: String, enabled: Bool) async throws
func bulkSetTools(context: String, server: String, tools: [String: Bool]) async throws

// Connection
func getConnectionInstructions(context: String) async throws -> ConnectionInstructions
```

### 6.4 UI Wireframes

#### Contexts Tab (Main View)
```
┌────────────────────────────────────────────────┐
│ Contexts                              [+ Add]  │
├────────────────────────────────────────────────┤
│                                                │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐          │
│  │personal │ │  work   │ │   dev   │          │
│  │   ✓     │ │         │ │         │          │
│  └────┬────┘ └─────────┘ └─────────┘          │
│       │                                        │
│  ═════╧════════════════════════════════════   │
│                                                │
│  Context: personal (default)                   │
│  Personal productivity tools                   │
│                                                │
│  [📋 Connection Instructions]  [Set Default]   │
│                                                │
│  ──────────────────────────────────────────   │
│                                                │
│  MCP Servers                        [+ Add]    │
│                                                │
│  ┌──────────────────────────────────────────┐ │
│  │ [✓] gmail         builtin   5/6 tools  ▶ │ │
│  │ [✓] context7      stdio     2/2 tools  ▶ │ │
│  │ [ ] slack-work    stdio     disabled   ▶ │ │
│  │ [✓] github        stdio     8/10 tools ▶ │ │
│  └──────────────────────────────────────────┘ │
│                                                │
│  Summary: 3 MCPs enabled, 15 tools active     │
│                                                │
└────────────────────────────────────────────────┘
```

#### Expanded Server View
```
┌──────────────────────────────────────────────┐
│ [✓] gmail                              ▼     │
├──────────────────────────────────────────────┤
│   Tools:                    [All] [None]     │
│   [✓] gmail_send         Send emails         │
│   [✓] gmail_search       Search inbox        │
│   [ ] gmail_draft        Create drafts       │
│   [✓] gmail_read         Read emails         │
│   [✓] gmail_list_labels  List labels         │
│   [ ] gmail_delete       Delete emails  ⚠️   │
└──────────────────────────────────────────────┘
```

#### Connection Instructions Sheet
```
┌────────────────────────────────────────────────┐
│ Connect to: personal                      ✕    │
├────────────────────────────────────────────────┤
│                                                │
│  HTTP Endpoint:                                │
│  ┌──────────────────────────────────────────┐ │
│  │ http://localhost:8765/mcp?context=...  📋│ │
│  └──────────────────────────────────────────┘ │
│                                                │
│  ┌─────────────────────────────────────────┐  │
│  │ OpenCode          │ Claude │ Cursor │   │  │
│  └─────────────────────────────────────────┘  │
│                                                │
│  ┌──────────────────────────────────────────┐ │
│  │ {                                        │ │
│  │   "$schema": "https://opencode.ai/...",  │ │
│  │   "mcp": {                               │ │
│  │     "diane-personal": {                  │ │
│  │       "type": "remote",                  │ │
│  │       "url": "http://localhost:8765/...   │ │
│  │       "oauth": false                     │ │
│  │     }                                    │ │
│  │   }                                      │ │
│  │ }                                        │ │
│  └──────────────────────────────────────────┘ │
│                                         [📋]  │
│                                                │
└────────────────────────────────────────────────┘
```

### 6.5 Tasks

- [ ] Create `Context.swift` model
- [ ] Create `ContextServer.swift` model  
- [ ] Create `ContextTool.swift` model
- [ ] Create `ConnectionInstructions.swift` model
- [ ] Add context API methods to `DianeClient.swift`
- [ ] Create `ContextsView.swift` main view
- [ ] Create `ContextDetailView.swift`
- [ ] Create `ContextServerRowView.swift` with expand/collapse
- [ ] Create `ContextToolRowView.swift`
- [ ] Create `ConnectionInstructionsView.swift` sheet
- [ ] Create `AddContextSheet.swift`
- [ ] Add Contexts tab to main navigation
- [ ] Test UI with mock data
- [ ] Test UI with live API

---

## Phase 7: Migration & Backward Compatibility

### 7.1 Import Existing Config

On first run after update:
1. Check if `mcp_servers` table is empty
2. If empty and `~/.diane/mcp-servers.json` exists, import servers
3. Add all imported servers to default "personal" context
4. Log migration result

```go
func (db *DB) MigrateFromJSONIfNeeded(jsonPath string) error {
    count, _ := db.CountMCPServers()
    if count > 0 {
        return nil // Already migrated
    }
    
    if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
        return nil // No JSON to migrate
    }
    
    servers, err := loadServersFromJSON(jsonPath)
    if err != nil {
        return err
    }
    
    for _, s := range servers {
        db.CreateMCPServer(&s)
        db.AddServerToContext("personal", s.Name, s.Enabled)
    }
    
    slog.Info("Migrated MCP servers from JSON", "count", len(servers))
    return nil
}
```

### 7.2 Deprecate JSON Config

- Keep `~/.diane/mcp-servers.json` as optional import source
- Add `diane-ctl mcp import <path>` command
- Add `diane-ctl mcp export <path>` command
- Update documentation

### 7.3 Tasks

- [ ] Implement `MigrateFromJSONIfNeeded()`
- [ ] Call migration on server startup
- [ ] Add `diane-ctl mcp import` command
- [ ] Add `diane-ctl mcp export` command
- [ ] Update `MCP.md` documentation
- [ ] Add migration notes to README

---

## Phase 8: Testing

### 8.1 Unit Tests

| Test File | Coverage |
|-----------|----------|
| `db/mcp_servers_test.go` | CRUD for MCP servers |
| `db/contexts_test.go` | CRUD for contexts |
| `mcpproxy/proxy_context_test.go` | Context filtering |
| `api/contexts_api_test.go` | API endpoints |

### 8.2 Integration Tests

| Test | Description |
|------|-------------|
| Context tool filtering | Verify only enabled tools are listed |
| Tool call rejection | Verify disabled tools cannot be called |
| Context switching | Verify different contexts return different tools |
| Migration | Verify JSON import works correctly |

### 8.3 Manual Testing Checklist

- [ ] Create new context in UI
- [ ] Add servers to context
- [ ] Toggle individual tools
- [ ] Copy connection instructions
- [ ] Connect with OpenCode using context
- [ ] Verify only context tools are visible
- [ ] Switch contexts in UI
- [ ] Delete context
- [ ] Restart Diane, verify persistence

---

## Implementation Order

### Week 1: Database & Core
1. Phase 1: Database schema migration
2. Phase 2: Database access layer
3. Phase 4.1-4.2: Update proxy to use DB

### Week 2: API & Proxy
4. Phase 3: API endpoints
5. Phase 4.3-4.4: Context resolution & filtering
6. Phase 5: Builtin tools context support

### Week 3: UI
7. Phase 6: Diane UI implementation
8. Phase 7: Migration & backward compatibility

### Week 4: Polish
9. Phase 8: Testing
10. Documentation updates
11. Bug fixes and edge cases

---

## File Changes Summary

### New Files

| Path | Description |
|------|-------------|
| `server/internal/db/migrations/003_contexts.sql` | Database migration |
| `server/internal/db/mcp_servers.go` | MCP server DB operations |
| `server/internal/db/contexts.go` | Context DB operations |
| `server/internal/api/mcp_servers_api.go` | MCP server API handlers |
| `server/internal/api/contexts_api.go` | Context API handlers |
| `server/internal/api/context_servers_api.go` | Context-server API handlers |
| `Diane/Diane/Models/Context.swift` | Context model |
| `Diane/Diane/Models/ContextServer.swift` | Context server model |
| `Diane/Diane/Models/ContextTool.swift` | Tool model |
| `Diane/Diane/Models/ConnectionInstructions.swift` | Connection model |
| `Diane/Diane/Views/ContextsView.swift` | Main contexts view |
| `Diane/Diane/Views/ContextDetailView.swift` | Context detail view |
| `Diane/Diane/Views/ContextServerRowView.swift` | Server row view |
| `Diane/Diane/Views/ContextToolRowView.swift` | Tool row view |
| `Diane/Diane/Views/ConnectionInstructionsView.swift` | Instructions sheet |
| `Diane/Diane/Views/AddContextSheet.swift` | Add context modal |

### Modified Files

| Path | Changes |
|------|---------|
| `server/internal/db/db.go` | Add migration runner |
| `server/internal/mcpproxy/proxy.go` | Use DB, add context filtering |
| `server/internal/api/api.go` | Register new routes |
| `server/mcp/server.go` | Context-aware tool listing/calling |
| `server/cmd/diane-ctl/main.go` | Add import/export commands |
| `Diane/Diane/Services/DianeClient.swift` | Add context API methods |
| `MCP.md` | Document contexts feature |

---

## Success Criteria

1. ✅ User can create/edit/delete contexts in Diane
2. ✅ User can enable/disable MCPs per context
3. ✅ User can enable/disable individual tools per context
4. ✅ Connection instructions show correct context URL
5. ✅ OpenCode/Claude/Cursor can connect with specific context
6. ✅ Only tools enabled in context are visible to consumer
7. ✅ Disabled tools return error if called directly
8. ✅ Existing `mcp-servers.json` is migrated automatically
9. ✅ Changes persist across Diane restarts
10. ✅ UI updates in real-time when config changes
