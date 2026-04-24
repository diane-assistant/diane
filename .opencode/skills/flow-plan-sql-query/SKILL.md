---
name: flow-plan-sql-query
description: >-
  Plan and wire SQLQuery objects in the Memory graph. Use whenever the user says "add a SQL query", "wire a SQLC query", "add sq- component", "what properties does SQLQuery have?", "wire uses relationship to source file", "what's the key format for SQL queries?", "link service method to query", or "add a query to the graph". Covers SQLQuery creation, uses wiring from ServiceMethod, uses wiring to SourceFile, key naming (sq-<verb>-<domain>), and the name/file property schema.
metadata:
  author: emergent
  version: "1.0"
---

> **CRITICAL: Never use the `memory` MCP tool directly.** All graph reads and writes MUST go
> through the `flow` CLI (`flow graph`, `flow query`, `flow audit`, `flow journal`, etc.).
> The `memory` tool is not available in this workflow. If you cannot find a `flow` command
> for something, use `flow --help` or `flow graph --help` — do not fall back to `memory`.

# plan-sql-query

Plan and wire **SQLQuery** (SQLC query in a `.sql` file) in the Memory graph.

## Component Chain
`ServiceMethod` ── uses ──> `SQLQuery` ── uses ──> `SourceFile` (migration)

## Properties
| Property | Required | Description |
|---|---|---|
| `name` | yes | SQLC query name (e.g. `"ArchiveCase"`) |
| `file` | yes | Path (e.g. `internal/db/queries/cases.sql`) |

⛔ Do NOT add `domain` — domain is expressed via the Scenario's relationship to a Domain object, not as a property on components.

## Key naming: `sq-<verb>-<domain>`
Examples: `sq-archive-case`, `sq-unarchive-case`, `sq-archive-company`.

## Commands
```bash
# Create SQLQuery (prints entity_id)
SQ_ID=$(flow graph create --type SQLQuery --key sq-archive-case \
  --properties '{"name": "ArchiveCase", "file": "internal/db/queries/cases.sql"}')
echo $SQ_ID

# Wire ServiceMethod → SQLQuery (uses)
flow graph relate --type uses --from <sm-id> --to $SQ_ID

# Wire SQLQuery → SourceFile (uses)
flow graph relate --type uses --from $SQ_ID --to <sf-id>
```

## Notes
- `name` must match SQLC `-- name:` annotation exactly.
- After adding to graph, regenerate with `task sqlc`.

## Workflow

1. **Check for existing SQLQuery** — search by key or `name` property before creating:
   ```bash
    flow graph list --type SQLQuery
   ```
   If a SQLQuery with the same `name` and `file` already exists, **reuse it** — just wire the `uses` relationship from the ServiceMethod to the existing object. Do not create a duplicate.
2. **Create** (only if no match found) using the command above.
3. **Wire** `ServiceMethod` → `SQLQuery` (`uses`), then continue to `SourceFile`.
