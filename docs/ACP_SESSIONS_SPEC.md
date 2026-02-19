# ACP Multi-Turn Sessions - Specification

**Change ID**: `acp-multi-turn-sessions`  
**Status**: Design Phase  
**Priority**: Medium  
**Complexity**: Medium

## Overview

Extend Diane's ACP agent integration to support **persistent, multi-turn conversations** with ACP agents. Currently, each `RunAgent()` call spawns a fresh subprocess, sends a single prompt, collects output, and kills the process. This spec adds session tracking so users can continue conversations with the same agent across multiple turns, preserving full context.

---

## Problem Statement

Today's flow (in `acp/config.go:RunAgent`):

```
spawn process → initialize → session/new → prompt (1x) → collect output → close/kill
```

Every interaction is stateless. The agent subprocess is destroyed after each call. If a user asks "refactor auth module" and then wants to say "also add tests for that", the agent has no memory of the first request.

The ACP protocol already supports multi-turn via `session/prompt` (can be called repeatedly on the same `sessionId`), but Diane doesn't use this capability.

---

## Design Goals

1. **Keep agent processes alive** between turns to preserve conversation context
2. **Track sessions** in Emergent so they survive Diane restarts (agent reconnection via `loadSession`)
3. **Expose sessions** through the API so the UI can list, resume, and close conversations
4. **Automatic cleanup** of idle sessions to prevent resource leaks
5. **Backward compatible** — single-shot `RunAgent()` still works unchanged

---

## Architecture

### Component Overview

```
┌──────────────┐         ┌────────────────────┐         ┌──────────────────┐
│   Diane UI   │  HTTP   │    API Server       │         │   ACP Agent      │
│  (Frontend)  │ ──────> │  /agents/:name/     │         │  (subprocess)    │
│              │         │    sessions/...      │         │                  │
└──────────────┘         └────────┬─────────────┘         └──────────────────┘
                                  │                              ▲
                                  │                              │ JSON-RPC
                                  v                              │ over stdio
                         ┌────────────────────┐                  │
                         │   acp.Manager      │──────────────────┘
                         │                    │
                         │  sessions map      │
                         │  stdioClients map  │
                         └────────┬───────────┘
                                  │
                                  │ persist session metadata
                                  v
                         ┌────────────────────┐
                         │   Emergent Graph    │
                         │   type: acp_session │
                         └────────────────────┘
```

### Key Design Decisions

1. **Sessions are agent-scoped**: A session belongs to one agent. Different agents have independent sessions.
2. **One active StdioClient per agent**: The subprocess stays alive and can hold multiple sessions (though typically one active session per agent).
3. **Session metadata in Emergent**: Session ID, agent name, creation time, last activity, and turn count are stored as graph objects. Message history is NOT stored (the agent holds that in-process).
4. **Idle timeout**: Sessions are reaped after a configurable idle period (default: 30 minutes). The subprocess is killed when all its sessions are closed.
5. **Crash recovery**: If the agent process dies, the session becomes `"disconnected"`. If the agent supports `loadSession`, Diane can attempt to restore it on a new subprocess.

---

## Data Model

### Session Metadata (Emergent Graph Object)

```
Graph Object Type: "acp_session"

Properties:
  session_id:     string    // ACP session ID returned by session/new
  agent_name:     string    // Name of the ACP agent (e.g., "opencode")
  agent_key:      string    // Agent unique key (name@workdir)
  workdir:        string    // Working directory for the session
  status:         string    // "active", "idle", "disconnected", "closed"
  turn_count:     int       // Number of prompt/response turns completed
  created_at:     string    // ISO 8601 timestamp
  last_active_at: string    // ISO 8601 timestamp of last prompt/response
  model_id:       string    // Current model ID (if agent exposes models)
  mode_id:        string    // Current mode ID (if agent exposes modes)
  title:          string    // User-assigned or auto-generated session title
  summary:        string    // Brief summary of conversation (optional, set by user or agent)

Labels:
  agent:{agentName}
  status:{status}
  workdir:{workdirHash}     // For filtering sessions by workspace
```

### In-Memory State (acp.Manager)

```go
// SessionState holds the runtime state for an active session
type SessionState struct {
    SessionID    string
    AgentName    string
    AgentKey     string
    WorkDir      string
    Client       *StdioClient    // Live connection to agent subprocess
    Status       SessionStatus
    TurnCount    int
    CreatedAt    time.Time
    LastActiveAt time.Time
    ModelID      string
    ModeID       string
    Title        string
}

type SessionStatus string

const (
    SessionActive       SessionStatus = "active"        // Subprocess alive, session usable
    SessionIdle         SessionStatus = "idle"           // Subprocess alive, no recent activity
    SessionDisconnected SessionStatus = "disconnected"   // Subprocess died, may be restorable
    SessionClosed       SessionStatus = "closed"         // Explicitly closed by user
)
```

---

## Manager Changes

### New Fields on `acp.Manager`

```go
type Manager struct {
    // ... existing fields ...
    sessions     map[string]*SessionState  // keyed by sessionID
    sessionsMu   sync.RWMutex
    idleTimeout  time.Duration             // default 30m
    sessionStore store.ACPSessionStore     // Emergent-backed persistence
}
```

### New Methods

```go
// StartSession creates a new multi-turn session with an agent.
// It spawns the subprocess (or reuses an existing one), initializes,
// creates a session, and returns the session state.
func (m *Manager) StartSession(agentName string, workDir string) (*SessionState, error)

// PromptSession sends a prompt to an existing session and returns the response.
// This is the multi-turn entry point — it reuses the existing subprocess and session.
func (m *Manager) PromptSession(sessionID string, prompt string) (*Run, error)

// ListSessions returns all sessions, optionally filtered by agent name or status.
func (m *Manager) ListSessions(agentName string, status SessionStatus) []*SessionState

// GetSession returns a specific session by ID.
func (m *Manager) GetSession(sessionID string) (*SessionState, error)

// CloseSession gracefully closes a session.
// If this is the last session on the subprocess, kills the subprocess.
func (m *Manager) CloseSession(sessionID string) error

// SetSessionConfig changes a configuration option (model, mode) for a session.
func (m *Manager) SetSessionConfig(sessionID string, configID string, value string) error

// CleanupIdleSessions is called periodically to reap sessions
// that have exceeded the idle timeout.
func (m *Manager) CleanupIdleSessions()
```

### Modified Methods

```go
// RunAgent remains unchanged for backward compatibility.
// It still does the single-shot spawn→prompt→kill flow.
// New callers should use StartSession + PromptSession instead.
func (m *Manager) RunAgent(name, prompt string) (*Run, error)
```

---

## Session Lifecycle

### Creating a Session

```
User: POST /agents/opencode/sessions  {"workdir": "/code/myproject"}

Manager.StartSession("opencode", "/code/myproject"):
  1. Look up agent config
  2. Check if a StdioClient already exists for this agent+workdir
     a. If yes, reuse it
     b. If no, spawn new subprocess, call Initialize()
  3. Call client.NewSessionWithInfo() → get sessionID, models, modes
  4. Create SessionState in memory
  5. Persist session metadata to Emergent
  6. Return session state (includes sessionID, available models, etc.)
```

### Sending a Follow-up Prompt

```
User: POST /agents/opencode/sessions/{sessionID}/prompt  {"prompt": "add tests"}

Manager.PromptSession(sessionID, "add tests"):
  1. Look up SessionState by sessionID
  2. Verify status is "active" or "idle"
  3. If status is "disconnected":
     a. Check if agent supports loadSession
     b. If yes: spawn new subprocess, initialize, call loadSession(sessionID)
     c. If no: return error "session lost, start a new one"
  4. Call client.Prompt(sessionID, prompt, updateHandler)
  5. Collect streaming output
  6. Increment turn count, update last_active_at
  7. Persist updated metadata to Emergent
  8. Return Run with collected output
```

### Closing a Session

```
User: DELETE /agents/opencode/sessions/{sessionID}

Manager.CloseSession(sessionID):
  1. Look up SessionState
  2. Mark status = "closed"
  3. Update Emergent graph object
  4. Check if the StdioClient has any other active sessions
     a. If no: call client.Close() (kills subprocess)
     b. If yes: leave subprocess running
  5. Remove from in-memory sessions map
```

### Idle Cleanup (background goroutine)

```
Manager.CleanupIdleSessions() — runs every 5 minutes:
  1. For each session where time.Since(LastActiveAt) > idleTimeout:
     a. Mark status = "closed"
     b. Update Emergent
     c. Close subprocess if no other sessions
     d. Remove from memory
```

### Crash Recovery

```
On Manager startup (NewManager or Reload):
  1. Load all sessions with status != "closed" from Emergent
  2. Mark them as "disconnected" (subprocess is gone after restart)
  3. They remain visible in ListSessions
  4. On next PromptSession, attempt loadSession if agent supports it
  5. If loadSession fails, auto-close and tell user to start fresh
```

---

## API Endpoints

### Session Management

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/agents/{name}/sessions` | Start a new multi-turn session |
| `GET` | `/agents/{name}/sessions` | List sessions for an agent |
| `GET` | `/agents/{name}/sessions/{id}` | Get session details |
| `POST` | `/agents/{name}/sessions/{id}/prompt` | Send a prompt to a session |
| `POST` | `/agents/{name}/sessions/{id}/cancel` | Cancel an in-progress prompt |
| `POST` | `/agents/{name}/sessions/{id}/config` | Set model/mode configuration |
| `DELETE` | `/agents/{name}/sessions/{id}` | Close a session |
| `GET` | `/sessions` | List all sessions across all agents |

### Request/Response Examples

**Start Session:**

```
POST /agents/opencode/sessions
{
  "workdir": "/Users/mcj/code/myproject",
  "title": "Auth refactor"
}

→ 201 Created
{
  "session_id": "abc123",
  "agent_name": "opencode",
  "status": "active",
  "workdir": "/Users/mcj/code/myproject",
  "title": "Auth refactor",
  "turn_count": 0,
  "created_at": "2026-02-19T10:00:00Z",
  "models": {
    "current_model_id": "anthropic/claude-sonnet-4-20250514",
    "available_models": [...]
  },
  "modes": {
    "current_mode_id": "default",
    "available_modes": [...]
  }
}
```

**Send Prompt (multi-turn):**

```
POST /agents/opencode/sessions/abc123/prompt
{
  "prompt": "refactor the auth middleware to use JWT"
}

→ 200 OK
{
  "run_id": "run_456",
  "agent_name": "opencode",
  "session_id": "abc123",
  "status": "completed",
  "output": [
    {
      "role": "agent",
      "parts": [{"content_type": "text/plain", "content": "I've refactored..."}]
    }
  ],
  "turn_number": 1,
  "created_at": "...",
  "finished_at": "..."
}
```

**Follow-up (same session):**

```
POST /agents/opencode/sessions/abc123/prompt
{
  "prompt": "now add unit tests for the changes you made"
}

→ 200 OK
{
  "run_id": "run_789",
  "session_id": "abc123",
  "turn_number": 2,
  ...
}
```

**List Sessions:**

```
GET /agents/opencode/sessions?status=active

→ 200 OK
[
  {
    "session_id": "abc123",
    "agent_name": "opencode",
    "status": "active",
    "workdir": "/Users/mcj/code/myproject",
    "title": "Auth refactor",
    "turn_count": 2,
    "last_active_at": "2026-02-19T10:05:00Z",
    "model_id": "anthropic/claude-sonnet-4-20250514"
  }
]
```

**Set Config:**

```
POST /agents/opencode/sessions/abc123/config
{
  "config_id": "model",
  "value": "anthropic/claude-opus-4-20250514"
}

→ 200 OK
{
  "status": "ok",
  "config_options": [...]
}
```

---

## Store Interface

```go
// ACPSessionStore persists session metadata to Emergent
type ACPSessionStore interface {
    CreateSession(ctx context.Context, session *ACPSession) error
    GetSession(ctx context.Context, sessionID string) (*ACPSession, error)
    ListSessions(ctx context.Context, agentName string, status string) ([]*ACPSession, error)
    UpdateSession(ctx context.Context, sessionID string, updates map[string]interface{}) error
    DeleteSession(ctx context.Context, sessionID string) error
    ListAllSessions(ctx context.Context, status string) ([]*ACPSession, error)
}

type ACPSession struct {
    SessionID    string    `json:"session_id"`
    AgentName    string    `json:"agent_name"`
    AgentKey     string    `json:"agent_key"`
    WorkDir      string    `json:"workdir"`
    Status       string    `json:"status"`
    TurnCount    int       `json:"turn_count"`
    CreatedAt    time.Time `json:"created_at"`
    LastActiveAt time.Time `json:"last_active_at"`
    ModelID      string    `json:"model_id,omitempty"`
    ModeID       string    `json:"mode_id,omitempty"`
    Title        string    `json:"title,omitempty"`
    Summary      string    `json:"summary,omitempty"`
}
```

---

## MCP Tool Exposure

New MCP tools for AI agents (Claude, etc.) to use sessions:

| Tool | Description |
|------|-------------|
| `agent_session_start` | Start a new multi-turn session with an ACP agent |
| `agent_session_prompt` | Send a follow-up prompt to an existing session |
| `agent_session_list` | List active sessions |
| `agent_session_close` | Close a session |

This enables AI-to-AI delegation with context: "Use OpenCode to refactor this, then ask it to also write tests for what it changed."

---

## Streaming Support (Future)

The current implementation collects all output before returning. A future enhancement could support Server-Sent Events (SSE) for real-time streaming:

```
POST /agents/opencode/sessions/abc123/prompt?stream=true
Accept: text/event-stream

→ 200 OK
Content-Type: text/event-stream

event: agent_message_chunk
data: {"content": {"type": "text", "text": "I'll start by..."}}

event: tool_call
data: {"title": "Reading auth.go", "status": "running"}

event: agent_message_chunk
data: {"content": {"type": "text", "text": "I've updated..."}}

event: done
data: {"run_id": "run_456", "stop_reason": "end_turn", "turn_number": 1}
```

This maps directly to the `session/update` notifications already received from ACP agents via `updateHandler` in `StdioClient.Prompt()`.

---

## Implementation Plan

### Phase 1: Core Session Management
1. Add `SessionState` type and session map to `acp.Manager`
2. Implement `StartSession()` — reuse or spawn subprocess, create session
3. Implement `PromptSession()` — send prompt to existing session
4. Implement `CloseSession()` — graceful teardown
5. Add idle cleanup goroutine

### Phase 2: Persistence & API
1. Create `ACPSessionStore` interface and `EmergentACPSessionStore`
2. Persist session metadata on create/update/close
3. Load disconnected sessions on startup
4. Add HTTP API endpoints to `api.go`

### Phase 3: MCP Tools & Integration
1. Add `agent_session_*` MCP tools
2. Wire into job system (scheduled jobs can open sessions)
3. Update CLI commands for session management

### Phase 4: Streaming & Polish
1. SSE streaming for prompt responses
2. Session restoration via `loadSession`
3. Session title auto-generation from first prompt

---

## Files to Change

| File | Change |
|------|--------|
| `internal/acp/config.go` | Add session management methods to `Manager` |
| `internal/acp/config.go` | Add `SessionState` type, session map, idle cleanup |
| `internal/acp/client.go` | Add `Run.TurnNumber`, `Run.SessionID` fields |
| `internal/store/acp_session.go` | New: `ACPSessionStore` interface |
| `internal/store/acp_session_emergent.go` | New: Emergent-backed implementation |
| `internal/api/api.go` | Add session HTTP endpoints, wire routes |
| `mcp/server.go` | Add `agent_session_*` MCP tools |
| `internal/cli/agent.go` | Add `diane agent sessions` CLI commands |
