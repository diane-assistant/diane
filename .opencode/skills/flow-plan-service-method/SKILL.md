---
name: flow-plan-service-method
description: >-
  Plan and wire ServiceMethod objects in the Memory graph. Use whenever the user says "add a service method", "wire a Go service", "what calls the SQL query?", "add svc- component", "wire calls relationship", "what properties does ServiceMethod have?", "link endpoint to service", or "what's the key format for service methods?". Covers ServiceMethod creation, calls wiring from APIEndpoint, uses wiring to SQLQuery, key naming (svc-<domain>-<verb>), and property schema.
metadata:
  author: emergent
  version: "1.0"
---

> **CRITICAL: Never use the `memory` MCP tool directly.** All graph reads and writes MUST go
> through the `flow` CLI (`flow graph`, `flow query`, `flow audit`, `flow journal`, etc.).
> The `memory` tool is not available in this workflow. If you cannot find a `flow` command
> for something, use `flow --help` or `flow graph --help` — do not fall back to `memory`.

# plan-service-method

ServiceMethod represents a Go method on a service struct, sitting between APIEndpoint and SQLQuery.

`APIEndpoint` ── calls → `ServiceMethod` ── uses → `SQLQuery`

## Properties
| Property | Required | Description |
|---|---|---|
| `name` | yes | Method name, e.g. `"ArchiveCase"` |
| `file` | yes | Relative path, e.g. `internal/service/cases.go` |

⛔ Do NOT add `domain` — domain is expressed via the Scenario's relationship to a Domain object, not as a property on components.

## Key Naming
`svc-<domain>-<verb>`, e.g.: `svc-cases-archive`, `svc-get-company-ownership-tree`.

## Commands
```bash
# Create (prints entity_id)
SVC_ID=$(flow graph create --type ServiceMethod --key svc-cases-archive \
  --properties '{"name": "ArchiveCase", "file": "internal/service/cases.go"}')
echo $SVC_ID

# Wire APIEndpoint → ServiceMethod (calls)
flow graph relate --type calls --from <endpoint-id> --to $SVC_ID

# Wire ServiceMethod → SQLQuery (uses)
flow graph relate --type uses --from $SVC_ID --to <sqlquery-id>
```

## Workflow

1. **Check the codebase first** — before creating a graph object, verify the method doesn't already exist in the service file:
   ```bash
   grep -r "func.*MethodName" internal/service/
   ```
   Replace `MethodName` with the method you're planning. If it exists in the code, a graph object should either already exist (reuse it) or needs to be created to represent the existing implementation — but do **not** invent a new method name that doesn't match the actual code.

2. **Check the graph** — search by key or `name` property before creating a new object:
   ```bash
   flow graph list --type ServiceMethod
   ```
   If a ServiceMethod with the same `name` and `file` already exists, **reuse it** — just wire the `calls` relationship from the APIEndpoint to the existing object. Do not create a duplicate.

3. **Create** (only if no match found in code or graph) using the command above.
4. **Wire** `APIEndpoint` → `ServiceMethod` (`calls`), then continue the chain to `SQLQuery`.

## Related Skills
- `plan-api-endpoint` — the APIEndpoint that calls this method
- `plan-sql-query` — the SQLQuery this method uses
