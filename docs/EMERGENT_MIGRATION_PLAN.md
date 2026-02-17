# Emergent Migration Plan: SQLite → Emergent Backend

**Status**: Planning Phase - Awaiting Per-Node MCP Server Configuration Feature  
**Priority**: High  
**Complexity**: High  
**Estimated Duration**: 3-4 weeks after per-node feature completion

---

## Overview

This document outlines the comprehensive plan to migrate Diane's local SQLite databases to Emergent as the primary backend. This migration will eliminate database duplication, leverage Emergent's advanced features (vector search, graph relationships, versioning), and consolidate data management.

---

## Table of Contents

1. [Strategic Decisions](#strategic-decisions)
2. [Current State Analysis](#current-state-analysis)
3. [Emergent Capabilities](#emergent-capabilities)
4. [Migration Architecture](#migration-architecture)
5. [Phased Implementation Plan](#phased-implementation-plan)
6. [Data Mapping Strategy](#data-mapping-strategy)
7. [Testing & Rollback Strategy](#testing--rollback-strategy)
8. [Timeline & Milestones](#timeline--milestones)

---

## Strategic Decisions

### Core Principles

✅ **Always-Online Assumption**  
- Diane assumes network connectivity to Emergent at all times
- No offline fallback or local caching required
- Emergent is the single source of truth

✅ **Secrets in Emergent**  
- Store API keys and sensitive provider configs in Emergent graph object properties
- Single source of truth for all credentials
- Leverage Emergent's security model

✅ **Schema Enforcement**  
- Create a Diane template pack in Emergent with strict JSON Schemas
- All object types have defined schemas: `job`, `agent`, `mcp_server`, `context`, `provider`, `webhook`, `slave_server`, `usage_record`, etc.

✅ **Phased Migration Approach**  
- Phase 1: Dual-write (SQLite + Emergent)
- Phase 2: Read migration (read from Emergent, continue dual-write)
- Phase 3: SQLite removal (Emergent-only)
- Repeat for each entity type

✅ **Simplified Configuration**  
- API key only (no ProjectID needed - automatically resolved by Emergent)
- Config file: `~/.diane/secrets/emergent-config.json`
- Environment variable fallback: `EMERGENT_BASE_URL`, `EMERGENT_API_KEY`

---

## Current State Analysis

### Diane's 3 SQLite Databases

#### 1. Primary Database: `~/.diane/cron.db` (14 tables)

**Job Management** (2 tables):
- `jobs` - Scheduled tasks with cron schedules
- `job_executions` - Execution history with stdout/stderr/exit codes

**Agent Management** (2 tables):
- `agents` - Remote OpenCode agents
- `agent_logs` - Agent communication logs

**MCP Server Configuration** (1 table):
- `mcp_servers` - MCP server configs (stdio/HTTP/SSE/builtin)
  - Includes: command, args, env, OAuth config, node routing

**Context Management** (3 tables):
- `contexts` - Context groupings for MCP servers
- `context_servers` - Many-to-many relationship (context ↔ servers)
- `context_server_tools` - Per-tool overrides within contexts

**Provider Management** (1 table):
- `providers` - AI/service provider configs (Vertex AI, OpenAI, Ollama)
  - Includes: API keys, OAuth tokens, service-specific settings

**Usage Tracking** (1 table):
- `usage` - Token usage and cost tracking per provider

**Webhook Management** (1 table):
- `webhooks` - Inbound webhook configurations with agent routing

**Distributed MCP** (3 tables):
- `slave_servers` - Remote Diane instances (distributed nodes)
- `revoked_slave_credentials` - Certificate revocation list
- `pairing_requests` - Pending slave pairing requests

#### 2. Gmail Cache: `~/.diane/gmail.db` (4 tables)

**Email Storage**:
- `emails` - Cached Gmail messages with content
- `attachments` - Email attachment references
- `sender_stats` - Pre-computed sender statistics
- `sync_state` - Incremental sync state tracking

**Migration Priority**: Lower (defer until primary database complete)

#### 3. Discord Sessions: `~/.kimaki/discord-sessions.db`

**Ownership**: External (Kimaki Discord bot)  
**Access**: Read-only by Diane  
**Migration**: **NOT INCLUDED** - keep as-is

---

## Emergent Capabilities

### Existing Integration

**Current Usage**: `server/mcp/tools/files/files.go`
- Emergent SDK: `github.com/emergent-company/emergent/apps/server-go/pkg/sdk v0.8.9`
- Already using Graph API: `CreateObject`, `GetObject`, `ListObjects`, `UpdateObject`, `DeleteObject`, `FTSSearch`, `HybridSearch`, `FindSimilar`
- Reference config: `~/.diane/secrets/emergent-config.json`

### Key Features for Migration

**Graph Objects**:
- Flexible typed entities with JSONB properties
- Labels for tagging and filtering
- Soft-delete support (`deleted_at`)
- Automatic timestamps (`created_at`, `updated_at`)
- Versioning support

**Graph Relationships**:
- Typed directed edges between objects
- Edge properties (JSONB)
- Enables complex many-to-many relationships

**Search Capabilities**:
- Full-text search (FTS)
- Vector search (embeddings)
- Hybrid search (lexical + vector)
- Filter by type, labels, properties

**Built-in Features**:
- User management (orgs/projects/invites)
- Chat history
- LLM call logs
- Notifications
- Audit logs
- Template packs (schema enforcement)
- Extraction jobs
- Integrations

---

## Migration Architecture

### Configuration Model

**Simplified Config** (API key automatically determines project):

```json
{
  "base_url": "https://api.emergent.ai",
  "api_key": "emt_abc123..."
}
```

**Environment Variables** (fallback):
```bash
EMERGENT_BASE_URL=https://api.emergent.ai
EMERGENT_API_KEY=emt_abc123...
```

**No `project_id` needed!** ✅

### Shared Client Package

**Location**: `server/internal/emergent/`

**Files**:
- `config.go` - Simplified config loading (no ProjectID)
- `client.go` - Singleton SDK client
- `config_test.go` - Unit tests for config loading

**Usage**:
```go
import "github.com/diane-assistant/diane/server/internal/emergent"

client, err := emergent.GetClient()
if err != nil {
    return fmt.Errorf("failed to get Emergent client: %w", err)
}

// Use client.Graph.* methods
```

### Repository Pattern

**Interface-Based Design**:
```go
type MCPServerRepository interface {
    List() ([]MCPServer, error)
    Get(name string) (*MCPServer, error)
    Create(server *MCPServer) error
    Update(server *MCPServer) error
    Delete(name string) error
}
```

**Implementations**:
- `mcp_servers_sqlite.go` - Wraps existing `internal/db` methods
- `mcp_servers_emergent.go` - Emergent graph API implementation
- `mcp_servers_dual.go` - Dual-write wrapper (SQLite + Emergent)

---

## Phased Implementation Plan

### Phase 0: Foundation Setup ⚙️

**Goal**: Create shared infrastructure and template pack

**Tasks**:
1. ✅ Create `server/internal/emergent/` package
   - `config.go` - Config loading
   - `client.go` - Singleton SDK client
   - `config_test.go` - Unit tests

2. ✅ Update Files Provider
   - Refactor to use `emergent.GetClient()`
   - Remove ProjectID references
   - Update documentation

3. ✅ Create Template Pack Script
   - `server/cmd/create-template-pack/main.go`
   - Define JSON Schemas for all Diane entity types
   - Upload via SDK API
   - Verify creation

4. ✅ Documentation
   - Add `emergent-config.json` to `.gitignore`
   - Document config format
   - Migration guide for existing users

**Deliverables**:
- Shared Emergent client package
- Files provider using shared client
- Template pack created in Emergent
- Updated documentation

**Estimated Duration**: 2-3 days

---

### Phase 1-3: MCP Servers Migration (First Entity) 🔄

**Why MCP Servers First?**
- Self-contained (7 CRUD operations)
- No complex relationships initially
- Good test case for migration pattern
- **CRITICAL**: Depends on per-node configuration feature completion

**Current Schema**:
```go
type MCPServer struct {
    ID        int64             // SQLite auto-increment
    Name      string            // Unique identifier
    Enabled   bool
    Type      string            // stdio, sse, http, builtin
    Command   string
    Args      []string
    Env       map[string]string
    URL       string
    Headers   map[string]string
    OAuth     *OAuthConfig
    NodeID    string            // Target slave hostname
    NodeMode  string            // "master", "specific", "any"
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### Phase 1: Dual Write

**Implementation Steps**:

1. **Create Repository Interface** (`server/internal/repository/mcp_servers.go`)
```go
type MCPServerRepository interface {
    List() ([]MCPServer, error)
    Get(name string) (*MCPServer, error)
    GetByID(id int64) (*MCPServer, error)
    Create(server *MCPServer) error
    Update(server *MCPServer) error
    Delete(name string) error
    GetOAuthToken(name string) (*OAuthToken, error)
    SaveOAuthToken(name string, token *OAuthToken) error
}
```

2. **SQLite Implementation** (`mcp_servers_sqlite.go`)
   - Wrap existing `internal/db` methods
   - Minimal changes

3. **Emergent Implementation** (`mcp_servers_emergent.go`)
   - Map to graph object type `"mcp_server"`
   - Store all fields in JSONB properties
   - Use labels for quick lookups: `"name:{server_name}"`
   - Store legacy ID: `properties.legacy_id`
   - Handle per-node configurations (design TBD after per-node feature)

4. **Dual-Write Wrapper** (`mcp_servers_dual.go`)
```go
type DualWriteMCPServerRepository struct {
    sqlite   MCPServerRepository
    emergent MCPServerRepository
}

// Write to BOTH, read from SQLite only
// Log Emergent write failures (don't block)
```

5. **Update API Handlers** (`server/internal/api/mcp_servers_api.go`)
   - Inject dual-write repository
   - Feature flag: `DIANE_USE_EMERGENT_MCP=dual-write`

**Testing**:
- Integration tests comparing responses
- Verify data consistency (SQLite vs Emergent)
- Test error handling (Emergent unavailable)

**Duration**: 3-4 days

#### Phase 2: Read Migration

**Goal**: Read from Emergent, continue dual-write

**Changes**:
- Update dual-write wrapper to read from Emergent
- Fallback to SQLite on error
- Monitor latency and errors
- Feature flag: `DIANE_USE_EMERGENT_MCP=dual-write-emergent-read`

**Testing**:
- Full test suite against Emergent reads
- Verify all operations work
- Test slave server node assignments
- OAuth flow testing

**Canary Period**: Run for X days/hours (configurable)

**Duration**: 2-3 days

#### Phase 3: SQLite Removal

**Goal**: Emergent-only for MCP servers

**Changes**:
- Remove dual-write wrapper
- Direct Emergent repository usage
- Keep `mcp_servers` table temporarily (backup)
- Feature flag: `DIANE_USE_EMERGENT_MCP=emergent-only`

**Migration Script**:
- One-time export: SQLite → Emergent
- Verify row counts match
- Spot-check random samples

**UI Updates**:
- Remove SQLite path references for MCP servers
- Add Emergent connection status

**Duration**: 1-2 days

---

### Phase 4-N: Remaining Entities 🔁

**Priority Order**:

1. ✅ **MCP Servers** (Phase 1-3 above)
2. **Contexts** (depends on MCP servers via relationships)
   - Many-to-many via graph edges
   - `context_servers` → edges between `context` and `mcp_server`
   - `context_server_tools` → edge properties
   - **Duration**: 2-3 days

3. **Providers** (secrets migration - critical!)
   - API keys in graph object properties
   - OAuth tokens in properties
   - Service-specific configs in properties
   - **Duration**: 2-3 days

4. **Jobs + Job Executions**
   - Parent-child relationship via graph edges
   - Agent relationships via edges
   - Execution history as related objects
   - **Duration**: 3-4 days

5. **Agents + Agent Logs**
   - Consider using Emergent's `llm_call_logs` for logs
   - Agent metadata in graph objects
   - **Duration**: 2-3 days

6. **Webhooks** (depends on agents)
   - Webhook configs with agent relationships
   - **Duration**: 1-2 days

7. **Usage Records**
   - Consider leveraging Emergent's `llm_call_logs`
   - Or custom graph objects type `"usage_record"`
   - **Duration**: 1-2 days

8. **Slave Servers + Pairing** (distributed node management)
   - Certificate management
   - Pairing request state
   - Revocation list
   - **Duration**: 2-3 days

**For Each Entity**:
- Repeat 3-phase approach (dual-write → read migration → SQLite removal)
- Write integration tests
- Update API handlers
- Update UI references

---

### Phase 5: Gmail Cache (Optional/Future) 📧

**Goal**: Migrate `~/.diane/gmail.db` to Emergent

**Approach**:
- **Emails** → Emergent documents with chunks
- **Attachments** → Document attachments API
- **Sender stats** → Graph objects with aggregations
- **Sync state** → Settings API (`/settings/{key}`)

**Decision Point**: Defer until primary database migration complete

**Estimated Duration**: 3-4 days

---

### Phase 6: Final Cleanup 🧹

**Goal**: Remove all SQLite dependencies

**Tasks**:
1. ✅ Drop SQLite tables from `internal/db/db.go`
2. ✅ Remove SQLite drivers from `go.mod`:
   - `modernc.org/sqlite`
   - `github.com/mattn/go-sqlite3` (keep if Gmail cache remains SQLite)
3. ✅ Delete `internal/db/` package (except `gmail/` if needed)
4. ✅ Update imports across 13+ consumer files
5. ✅ Update UI/documentation:
   - Remove `~/.diane/cron.db` references
   - Add Emergent configuration docs
6. ✅ Archive SQLite files (backup, don't delete immediately)

**Duration**: 1-2 days

---

## Data Mapping Strategy

### SQLite → Emergent Mapping

| SQLite Concept | Emergent Concept | Example |
|---|---|---|
| Table row | Graph object | `MCPServer` → object type `"mcp_server"` |
| Foreign key | Graph relationship (edge) | `job.agent_name` → edge to `agent` object |
| JSON column | JSONB properties | `MCPServer.Env` → `properties.env` |
| Many-to-many join table | Graph edges with properties | `context_servers` → edges between `context` and `mcp_server` |
| Auto-increment ID | Emergent object ID (UUID) | `MCPServer.ID` → `object.id` |
| Unique constraints | Emergent labels + queries | `name UNIQUE` → label `"name:{value}"` |
| Indexes | Emergent indexing (automatic) | FTS, property indexing |

### ID Migration Strategy

**Challenge**: SQLite uses `int64` IDs, Emergent uses UUIDs

**Solution**:
1. Store SQLite ID in Emergent properties: `properties.legacy_id`
2. Create Emergent label for lookups: `"legacy_id:{sqlite_id}"`
3. Update foreign key references to use Emergent object IDs
4. Provide helper methods: `GetByLegacyID(id int64)`

### Relationship Mapping

**Foreign Keys → Graph Edges**:

**Examples**:
- `job.agent_name` → graph edge: `job` --[uses_agent]--> `agent`
- `context_servers.context_id + server_id` → edge: `context` --[includes_server]--> `mcp_server`
- `context_server_tools` → edge properties: `{tool: "read_file", enabled: true}`
- `job_executions.job_id` → edge: `job` --[has_execution]--> `job_execution`

**Edge Naming Convention**:
- Use semantic names: `uses_agent`, `includes_server`, `executed_by`, `logs_for`, `has_execution`
- Store relationship metadata in edge properties

### OAuth Tokens & Secrets

**Storage Location**: Graph object properties (per requirements)

**Example** (MCP Server with OAuth):
```json
{
  "type": "mcp_server",
  "name": "github",
  "properties": {
    "enabled": true,
    "type": "stdio",
    "command": "/usr/local/bin/mcp-github",
    "args": [],
    "env": {
      "GITHUB_TOKEN": "ghp_..."
    },
    "oauth": {
      "provider": "github",
      "client_id": "...",
      "client_secret": "...",
      "access_token": "...",
      "refresh_token": "...",
      "expires_at": "2026-03-01T..."
    },
    "node_id": "workstation",
    "node_mode": "specific",
    "legacy_id": 42
  },
  "labels": [
    "name:github",
    "legacy_id:42",
    "node:workstation"
  ]
}
```

### Timestamps & Metadata

**Emergent Built-in Fields**:
- `created_at` (automatic)
- `updated_at` (automatic)
- `deleted_at` (soft delete)

**SQLite timestamps migrate directly** to Emergent's built-in fields.

---

## Testing & Rollback Strategy

### Unit Tests
- Mock Emergent SDK for repository tests
- Test data mapping (struct ↔ graph object)
- Test ID translation (legacy ID → UUID)
- Test relationship creation/traversal

### Integration Tests
- Spin up test Emergent project
- Full CRUD operations against live Emergent
- Verify data consistency (SQLite vs Emergent)
- Performance benchmarks (read/write latency)
- Test OAuth flows end-to-end

### Migration Tests
- Export SQLite → Emergent (one-time script)
- Verify row counts match
- Spot-check random samples for accuracy
- Test relationship integrity (foreign keys → edges)

### Rollback Plan

**At Each Phase**:
- **Dual-write phase**: Disable Emergent writes via feature flag
- **Read migration**: Revert to SQLite reads instantly
- **SQLite removal**: Keep backup of `~/.diane/cron.db` for 30 days

**Emergency Rollback**:
1. Set feature flag: `DIANE_USE_EMERGENT_{ENTITY}=sqlite-only`
2. Restart Diane
3. Investigate Emergent issue
4. Re-enable Emergent when resolved

**Feature Flags** (environment variables):
```bash
DIANE_USE_EMERGENT_MCP=dual-write              # Phase 1
DIANE_USE_EMERGENT_MCP=dual-write-emergent-read # Phase 2
DIANE_USE_EMERGENT_MCP=emergent-only           # Phase 3
DIANE_USE_EMERGENT_MCP=sqlite-only             # Rollback
```

---

## Timeline & Milestones

### Overall Estimate: 3-4 weeks

| Phase | Component | Duration | Status |
|---|---|---|---|
| **Phase 0** | Foundation Setup | 2-3 days | ⏳ Pending |
| **Phase 1-3** | MCP Servers | 6-9 days | ⏳ Blocked (awaiting per-node feature) |
| **Phase 4** | Contexts | 2-3 days | ⏳ Pending |
| **Phase 4** | Providers | 2-3 days | ⏳ Pending |
| **Phase 4** | Jobs + Executions | 3-4 days | ⏳ Pending |
| **Phase 4** | Agents + Logs | 2-3 days | ⏳ Pending |
| **Phase 4** | Webhooks | 1-2 days | ⏳ Pending |
| **Phase 4** | Usage | 1-2 days | ⏳ Pending |
| **Phase 4** | Slave Servers | 2-3 days | ⏳ Pending |
| **Phase 6** | Final Cleanup | 1-2 days | ⏳ Pending |
| **Testing/QA** | Full test suite | 2-3 days | ⏳ Pending |

**Total**: ~21-28 days (3-4 weeks)

### Success Criteria

**Phase 1-3 (MCP Servers) Complete When**:
- ✅ All MCP server CRUD operations work via Emergent
- ✅ OAuth token flow works (device auth, refresh)
- ✅ Slave server node assignments work
- ✅ Per-node configurations work (after feature completion)
- ✅ No SQLite reads for MCP servers
- ✅ Integration tests pass
- ✅ No errors in production logs for 48 hours

**Full Migration Complete When**:
- ✅ All 14 SQLite tables migrated
- ✅ `internal/db/` package deleted (except Gmail if deferred)
- ✅ SQLite drivers removed from `go.mod`
- ✅ UI updated (no `~/.diane/cron.db` references)
- ✅ All tests pass
- ✅ Documentation updated
- ✅ No performance regressions
- ✅ Successful production deployment

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| **Emergent downtime** | High | Feature flags for instant SQLite fallback |
| **Data inconsistency** | High | Dual-write phase validates sync before read migration |
| **Performance regression** | Medium | Benchmark each phase; Emergent has caching/indexing |
| **OAuth token migration** | High | Test with non-production credentials first |
| **Complex relationships** | Medium | Start with simple entities, validate patterns |
| **Slave server coordination** | Medium | Test distributed scenarios explicitly |
| **Per-node config changes** | High | **BLOCKING**: Wait for feature completion before starting |

---

## Blocking Dependencies

### ⚠️ CRITICAL: Per-Node MCP Server Configuration Feature

**Status**: In development  
**Impact**: Blocks Phase 1-3 (MCP Servers migration)

**Why Blocking?**:
- MCP servers are the first entity to migrate
- Current schema uses `node_id` + `node_mode` fields
- New feature may change how per-node configs are stored
- Migration strategy depends on final data model

**What We Know**:
- Feature involves per-node MCP server configurations
- Same logical server can have different configs on different nodes
- Distributed MCP architecture already in place (see `DISTRIBUTED_MCP_SPEC.md`)

**What We Need Before Proceeding**:
1. Final per-node configuration data model
2. Storage schema (separate table? JSON column? multiple server records?)
3. API contract for per-node CRUD operations
4. Understanding of how node routing works with new model

**Action Items**:
1. ✅ Document migration plan (this file)
2. ⏳ Wait for per-node feature completion
3. ⏳ Review final per-node implementation
4. ⏳ Update Phase 1-3 plan based on per-node design
5. ⏳ Begin Phase 0 (Foundation Setup) - can proceed independently

---

## References

### Documentation
- Emergent OpenAPI Spec: `https://github.com/emergent-company/emergent/main/openapi.yaml`
- Emergent Database Schema: `https://github.com/emergent-company/emergent/main/docs/database/schema.dbml`
- Emergent Agents Guide: `https://github.com/emergent-company/emergent/main/AGENTS.md`
- Distributed MCP Spec: `docs/DISTRIBUTED_MCP_SPEC.md`
- Distributed MCP Implementation: `docs/DISTRIBUTED_MCP_IMPLEMENTATION.md`

### Code References
- Emergent SDK: `github.com/emergent-company/emergent/apps/server-go/pkg/sdk v0.8.9`
- Files Provider (Reference): `server/mcp/tools/files/files.go`
- Database Layer: `server/internal/db/` (10 files)
- API Layer: `server/internal/api/` (6 API handlers)
- MCP Server: `server/mcp/server.go`

---

**Document Version**: 1.0  
**Last Updated**: 2026-02-17  
**Status**: Planning Phase - Blocked by Per-Node Configuration Feature  
**Next Action**: Complete per-node MCP server feature, then revisit Phase 0

---

## Appendix: Per-Node Configuration Design Considerations

### Possible Data Models (TBD)

**Option A: Separate Table**
```sql
CREATE TABLE mcp_server_node_configs (
    id INTEGER PRIMARY KEY,
    server_id INTEGER REFERENCES mcp_servers(id),
    node_id TEXT REFERENCES slave_servers(host_id),
    command TEXT,
    args TEXT,  -- JSON
    env TEXT,   -- JSON
    -- ... other overridable fields
);
```

**Option B: Embedded JSON**
```sql
ALTER TABLE mcp_servers ADD COLUMN node_configs TEXT; -- JSON map[node_id]->config
```

**Option C: Duplicate Server Records**
```sql
-- Create separate records: "github-mac", "github-linux"
-- No schema changes needed
```

**Emergent Mapping** (will depend on chosen option):
- **Option A**: Graph edges with properties: `mcp_server` --[configured_for {command, args, env}]--> `slave_server`
- **Option B**: JSONB property: `properties.node_configs: { "workstation": {...}, "server": {...} }`
- **Option C**: Multiple graph objects with label: `"server_name:github"`, distinguished by properties

**Decision**: To be determined after per-node feature implementation review.

---

## Change Log

| Date | Version | Changes | Author |
|---|---|---|---|
| 2026-02-17 | 1.0 | Initial migration plan | Assistant |

