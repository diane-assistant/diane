---
name: flow-implement-service-method
description: >-
  Implement a ServiceMethod during scenario execution — write the Go service
  method that a planned ServiceMethod maps to. Use whenever the user says
  "implement the service method for X", "write the Go service for Y", "add the
  business logic", "implement svc- component", "write the service layer", "add a
  method to the service", or "what Go code do I write for this ServiceMethod?".
  Covers reading name/file from the graph, service struct pattern,
  context.Context, error wrapping, calling SQLC queries via s.db, and running
  task build.
metadata:
  author: emergent
  version: "1.0"
---

> **CRITICAL: Never use the `memory` MCP tool directly.** All graph reads and writes MUST go
> through the `flow` CLI (`flow graph`, `flow query`, `flow audit`, `flow journal`, etc.).
> The `memory` tool is not available in this workflow. If you cannot find a `flow` command
> for something, use `flow --help` or `flow graph --help` — do not fall back to `memory`.

# Implement ServiceMethod

This skill covers writing the Go code for a **ServiceMethod** during scenario implementation.

## Where it fits
`APIEndpoint` → calls → `ServiceMethod` (this skill) → uses → `SQLQuery`

## Implementation Steps

1.  **Check the graph**: Read the `ServiceMethod` to find its `file` and `name`.
2.  **Add the method**: Update `internal/service/<domain>.go`.
3.  **Signature**: `func (s *XxxService) MethodName(ctx context.Context, ...) (..., error)`
4.  **SQLC Call**: Execute queries via `s.db.QueryName(ctx, params)`.
5.  **Error Handling**: Wrap all errors: `fmt.Errorf("ServiceName.MethodName: %w", err)`.
6.  **Verify**: Run `task build` to check for compilation errors.

## Key Conventions

- **Struct Pattern**: Services use `db *sqlcdb.Queries` for DB access.
- **Context**: Always pass `context.Context` as the first parameter.
- **Domain Focus**: Logic stays within `internal/service/`; handlers stay in `internal/handler/`.

## Example

```go
func (s *OrgService) GetOrgByID(ctx context.Context, id uuid.UUID) (*sqlcdb.Org, error) {
    org, err := s.db.GetOrgByID(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("OrgService.GetOrgByID: %w", err)
    }
    return &org, nil
}
```

## Related Skills
- `plan-service-method`: Adding ServiceMethods to the graph.
- `backend-dev-guidelines`: General Go conventions and error patterns.
- `implement-api-endpoint`: The handler that calls this service.
- `implement-sql-query`: The underlying SQLC query.
