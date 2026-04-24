---
name: flow-implement-api-endpoint
description: >-
  Implement an APIEndpoint during scenario execution — write the HTTP handler function, register the route, and add Swagger annotations. Use whenever the user says "implement the endpoint for X", "write the handler for Y", "add the route", "implement ep- component", "wire the HTTP handler", "add the API for this step", or "what handler do I write for this endpoint?". Covers reading handler/method/path from the graph, handler function pattern, route registration in server.go, Swagger annotations, and calling the ServiceMethod.
metadata:
  author: emergent
  version: "1.0"
---

> **CRITICAL: Never use the `memory` MCP tool directly.** All graph reads and writes MUST go
> through the `flow` CLI (`flow graph`, `flow query`, `flow audit`, `flow journal`, etc.).
> The `memory` tool is not available in this workflow. If you cannot find a `flow` command
> for something, use `flow --help` or `flow graph --help` — do not fall back to `memory`.

# implement-api-endpoint

This skill covers writing the actual code for an **APIEndpoint** — the HTTP handler function — during scenario implementation. It is the implementation counterpart to `plan-api-endpoint`.

## Where it fits
```
Context
  └── requires → APIEndpoint    ← this skill implements it
                     └── calls → ServiceMethod
```

## Implementation steps

1. **Read the APIEndpoint from the graph** — check `file`, `handler`, `method`, `path`, `domain`
2. **Add the handler function** to `internal/handler/<domain>/handler.go`
3. **Register the route** in `internal/server/server.go`
4. **Add Swagger annotations** above the handler — required by `task swagger:validate`
5. **Call the ServiceMethod** from the handler — inject via the handler struct
6. **Run `task build`** to verify compilation

## Key conventions
- Handler struct: `type Handler struct { svc *service.XxxService }`
- Handler func signature: `func (h *Handler) HandleXxx(w http.ResponseWriter, r *http.Request)`
- Route registration: `r.Method("/path", h.HandleXxx)` in server.go
- Swagger annotations are mandatory — see `backend-dev-guidelines` skill
- Error wrapping: `fmt.Errorf("context: %w", err)`
- Use `new-handler` skill if scaffolding a brand new handler domain

## Related skills
- `plan-api-endpoint` — how to add APIEndpoint to the graph
- `backend-dev-guidelines` — Go conventions, Swagger annotations, error wrapping
- `new-handler` — scaffold a full new handler domain
- `implement-service-method` — the ServiceMethod this endpoint calls
