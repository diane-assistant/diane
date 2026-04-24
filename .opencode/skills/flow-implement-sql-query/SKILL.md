---
name: flow-implement-sql-query
description: >-
  Implement a SQLQuery and its SourceFile during scenario execution — write the
  SQLC-annotated SQL query and migration that planned SQLQuery/SourceFile objects
  map to. Use whenever the user says "implement the SQL query for X", "write the
  SQLC query", "add the migration", "implement sq- component", "write the
  database query", "add a query to the sql file", or "what SQL do I write for this
  SQLQuery?". Covers reading name/file from the graph, SQLC annotation format,
  migration file conventions, running task sqlc and task migrate, and NullTime
  patterns.
author: emergent
version: "1.0"
---

> **CRITICAL: Never use the `memory` MCP tool directly.** All graph reads and writes MUST go
> through the `flow` CLI (`flow graph`, `flow query`, `flow audit`, `flow journal`, etc.).
> The `memory` tool is not available in this workflow. If you cannot find a `flow` command
> for something, use `flow --help` or `flow graph --help` — do not fall back to `memory`.

# Implement SQL Query

This skill covers implementing a **SQLQuery** and **SourceFile** pair during scenario execution.

## Implementation Steps

1. **Query the Graph**: Identify the `name`, `file`, and `domain` of the `SQLQuery` object.
2. **Write SQL**: Add the query to `internal/db/queries/<domain>.sql` using SQLC annotations.
   ```sql
   -- name: QueryName :one | :many | :exec | :execresult
   SELECT * FROM table WHERE id = $1;
   ```
3. **Add Migration**: If required, create `internal/db/migrations/NNNNNN_desc.up.sql`.
4. **Regenerate & Apply**:
   - Run `task sqlc` to regenerate Go types (applies NullTime patches).
   - Run `task migrate` to apply database changes.
5. **Verify**: Run `task build` to check for type errors in callers.

## Key Conventions

- **SQLC Types**: Use `sqlc.narg()` for nullable params; `pgtype.Timestamptz` for timestamps.
- **NullTime**: Never use `sql.NullTime`. The `task sqlc` command handles overrides.
- **Migrations**: Always include a corresponding `.down.sql` file.
- **Reference**: Use `db-schema` for table structures and `new-sqlc-query` for patterns.

## Related Skills
- `plan-sql-query` / `plan-source-file`: The planning counterparts.
- `new-sqlc-query`: Detailed SQLC syntax and patterns.
- `db-schema`: PostgreSQL schema reference.
- `implement-service-method`: The service method that consumes this query.
