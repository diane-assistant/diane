---
name: flow-plan-api-endpoint
description: >-
  Plan and wire APIEndpoint objects in the Memory graph. Use whenever the user says "add an API endpoint", "wire an endpoint", "what handler does this call?", "add a route", "what properties does APIEndpoint have?", "wire the calls relationship", "link endpoint to service method", "what's the key format for endpoints?", "add ep- component", or "how do I attach an endpoint to a context?". Covers APIEndpoint creation, requires wiring from Context, calls wiring to ServiceMethod, property schema (file/handler/method/path), key naming, and reuse across scenarios.
metadata:
  author: emergent
  version: "1.0"
---

> **CRITICAL: Never use the `memory` MCP tool directly.** All graph reads and writes MUST go
> through the `flow` CLI (`flow graph`, `flow query`, `flow audit`, `flow journal`, etc.).
> The `memory` tool is not available in this workflow. If you cannot find a `flow` command
> for something, use `flow --help` or `flow graph --help` — do not fall back to `memory`.

# Plan API Endpoint Skill

This skill covers planning and wiring `APIEndpoint` objects in the Memory graph.

## Where APIEndpoint fits

```
Context
  └── requires → APIEndpoint       (HTTP handler)
                     └── calls → ServiceMethod    (Go service method)
                                      └── uses → SQLQuery
                                                     └── uses → SourceFile
```

## ⛔ APIEndpoint is ONLY for `/api/v1/...` REST routes

`APIEndpoint` objects represent **JSON REST API routes** only — routes registered under `/api/v1/`.

**DO NOT** create an `APIEndpoint` for:
- UI handler routes (e.g. `GET /app/cases/wizard/{step}`) — these belong in a `UIComponent` description
- Server-rendered page routes (anything under `/app/`, `/auth/`, etc.)

If a scenario step involves a UI page load or HTMX partial that is served by a Go handler but returns HTML (not JSON), model it as a `UIComponent`, not an `APIEndpoint`.

## APIEndpoint properties

| Property | Required | Description |
|---|---|---|
| `file` | yes | Handler file path, e.g. `internal/handler/cases/handler.go` |
| `handler` | yes | Go function name, e.g. `HandleArchiveCase` |
| `method` | yes | HTTP method: `GET`, `POST`, `PATCH`, `DELETE` |
| `path` | yes | Route path, e.g. `/api/v1/cases/{id}/archive` |

⛔ Do NOT add `domain` — domain is expressed via the Scenario's relationship to a Domain object, not as a property on components.

**Key naming:** `ep-<domain>-<verb>` (e.g., `ep-case-archive`, `ep-employees-upload-avatar`).

## Commands

**Create APIEndpoint** (prints entity_id):
```bash
EP_ID=$(flow graph create --type APIEndpoint --key ep-case-archive \
  --properties '{"file": "internal/handler/cases/handler.go", "handler": "HandleArchiveCase", "method": "PATCH", "path": "/api/v1/cases/{id}/archive"}')
echo $EP_ID
```

**Wire Context → APIEndpoint (requires):**
```bash
flow graph relate --type requires --from <context-id> --to $EP_ID
```

**Wire APIEndpoint → ServiceMethod (calls):**
```bash
flow graph relate --type calls --from $EP_ID --to <servicemethod-id>
```

## Workflow

1. Check for existing endpoints with `flow graph list --type APIEndpoint`.
2. Create missing endpoints; wire `Context` → `APIEndpoint` (`requires`) and `APIEndpoint` → `ServiceMethod` (`calls`).
3. Continue the chain: `ServiceMethod` → `SQLQuery` (`uses`) → `SourceFile` (`uses`).
