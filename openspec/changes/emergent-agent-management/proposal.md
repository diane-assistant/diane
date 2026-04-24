## Why

Emergent v0.21.6 exposes a full admin API for managing agents, runs, MCP servers, and workspace images. Diane currently only supports *creating* agents (emergent-custom-agents) and *monitoring* running ones (emergent-agent-monitoring). The API has much more: full CRUD on agents, manual triggering, run history, cancellation, reaction-agent pending-events, MCP server registry management, and workspace image management. Exposing this in Diane gives developers a complete control plane for their Emergent backend from within the app.

## What Changes

- Add **Agent Management** view: list, update, delete agents; trigger/cancel runs; view run history.
- Add **Pending Events** panel for reaction agents: inspect unprocessed graph objects, batch-trigger.
- Add **MCP Server Registry** management: list, register, update, delete MCP servers (full CRUD).
- Add **Workspace Images** management: list, register (docker ref), delete custom images.
- Add **EmergentAdminClient** API layer covering all new endpoints (agents CRUD, runs, mcp-servers, workspace-images, agent-definitions workspace-config).

## Capabilities

### New Capabilities
- `emergent-agent-management`: Full CRUD + execution control for agents (update, delete, trigger, cancel run, view run history).
- `emergent-reaction-agent-events`: View and batch-trigger pending events for reaction agents.
- `emergent-mcp-server-registry`: Register, list, update, delete MCP servers tied to a project.
- `emergent-workspace-images`: List, register, and delete custom workspace images.

### Modified Capabilities
- `emergent-agent-monitoring`: Extend existing monitoring view with trigger/cancel actions and run history tab.
- `emergent-custom-agents`: Wire agent-definitions workspace-config GET/PUT into the existing configuration form.

## API Surface (from http://mcj-emergent:3002/openapi.json)

### Agents
- `GET /api/admin/agents` — list all agents (X-Project-ID header)
- `POST /api/admin/agents` — create agent
- `GET /api/admin/agents/{id}` — get agent
- `PATCH /api/admin/agents/{id}` — update agent (partial)
- `DELETE /api/admin/agents/{id}` — delete agent
- `POST /api/admin/agents/{id}/trigger` — manual run trigger
- `GET /api/admin/agents/{id}/runs` — run history (limit 1–100, default 10)
- `POST /api/admin/agents/{id}/runs/{runId}/cancel` — cancel run
- `GET /api/admin/agents/{id}/pending-events` — pending events for reaction agents (limit 1–100)
- `POST /api/admin/agents/{id}/batch-trigger` — batch trigger (up to 100 objectIds)

### Agent Definitions
- `GET /api/admin/agent-definitions/{id}/workspace-config`
- `PUT /api/admin/agent-definitions/{id}/workspace-config`

### MCP Servers
- `GET /api/admin/mcp-servers` — list
- `POST /api/admin/mcp-servers` — register
- `GET /api/admin/mcp-servers/{id}` — get detail
- `PATCH /api/admin/mcp-servers/{id}` — update
- `DELETE /api/admin/mcp-servers/{id}` — delete (204)

### Workspace Images
- `GET /api/admin/workspace-images` — list (built-in + custom)
- `POST /api/admin/workspace-images` — register (triggers background docker pull)
- `GET /api/admin/workspace-images/{id}` — get
- `DELETE /api/admin/workspace-images/{id}` — delete custom images only (204)

### Projects
- `POST /api/admin/projects/{projectId}/install-default-agents` — idempotent default agent install

## Impact

- **API Layer**: `EmergentAdminClient.swift` — new methods for all endpoints above; bearer auth via `X-Project-ID` header for project-scoped resources.
- **UI**: New view for agent management (extends monitoring), new sheet/list for MCP server registry, new sheet/list for workspace images; all using existing MasterDetailView, DetailSection, InfoRow, SummaryCard components.
- **State**: New `@Observable` stores for mcp-servers and workspace-images; extend existing agent store with run history and pending events.
- **Non-goals**: No support for `install-default-agents` in UI (admin-only CLI operation). No real-time streaming of run logs (covered by existing monitoring spec).
