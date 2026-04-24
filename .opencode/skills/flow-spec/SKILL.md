---
name: flow-spec
description: >-
  Inspect the Swagger/OpenAPI contract for a planned APIEndpoint and generate
  dependency graph comment blocks for any graph object using `flow spec` and
  `flow graph comment`. Use whenever the user says "show me the spec for X",
  "what parameters does this endpoint take", "what's the swagger contract for Y",
  "generate a comment for Z", "show the dependency tree for this component",
  "what uses this object", "what does this endpoint depend on", or
  "is this endpoint already in the swagger spec".
metadata:
  author: emergent
  version: "1.0"
---

> **CRITICAL: Never use the `memory` MCP tool directly.** All graph reads and writes MUST go
> through the `flow` CLI (`flow graph`, `flow query`, `flow audit`, `flow journal`, etc.).
> The `memory` tool is not available in this workflow. If you cannot find a `flow` command
> for something, use `flow --help` or `flow graph --help` — do not fall back to `memory`.

# flow-spec skill

Two complementary inspection commands for understanding what's already wired
in the graph and what's already implemented in the API.

---

## `flow spec` — Swagger/OpenAPI contract viewer

### What it does

`flow spec` looks up an `APIEndpoint` graph object and renders its full Swagger
contract: HTTP method, path, summary, parameters, and response codes. It reads
`docs/swagger/swagger.json` in the repo root.

### When to use it

- During planning: verify that a planned endpoint **already exists** in the
  Swagger spec before writing new code.
- During implementation: confirm the expected parameters and response shape
  before writing the handler test.
- During review: spot discrepancies between what's planned in the graph and
  what's documented in the spec.

### Usage

```bash
# Look up by graph key
flow spec --key ep-cases-create

# Look up by HTTP method + path
flow spec --method POST --path /api/v1/cases

# If the endpoint is not in the spec yet, it prints "not found in swagger"
```

### Output format

```
ep-cases-create  [APIEndpoint]  planned
  file:    internal/handler/cases/handler.go
  name:    CreateCase

  ── Swagger contract ───────────────────────────────────────
  POST /api/v1/cases
  Summary: Create a new case

  Parameters:
    bearer  (header, required)  Bearer token

  Responses:
    201  Case created successfully
    400  Bad request
    401  Unauthorized
    500  Internal server error
```

### Reading the output

- **"not found in swagger"** → the endpoint is planned but not yet implemented.
  Check whether a migration or a new handler is needed.
- **Parameters** — note which are `required` vs optional and where they come
  from (`header`, `path`, `query`, `body`).
- **Responses** — use these as the source of truth when writing tests.

---

## `flow graph comment` — Dependency tree block

### What it does

`flow graph comment` generates a formatted comment block showing:

1. **Depends on** (downstream): what this object calls/uses
   `APIEndpoint → calls → ServiceMethod → uses → SQLQuery → uses → SourceFile`
2. **Used by** (upstream): which Contexts require this object, and which
   ScenarioSteps occur in those Contexts
3. **Co-located**: other components wired into the same Context(s)

### When to use it

- Before creating a new component: check if a similar one is already wired in
  the graph for the same Context.
- During planning: understand the full blast radius of a planned endpoint.
- Generating paste-ready Go doc comments for a handler or service method.

### Usage

```bash
# Single object — plain output, no prefix (default)
flow graph tree ep-cases-create
flow graph tree ui-cases-add-modal
flow graph tree sq-get-case-inbox

# All objects of a type
flow graph tree --type APIEndpoint
flow graph tree --type UIComponent
flow graph tree --type ServiceMethod
flow graph tree --type SQLQuery

# Go style: adds // prefix, paste directly above a handler func
flow graph tree ep-cases-create --style go

# Hash style: adds # prefix, for SQL/markdown files
flow graph tree sq-get-case-inbox --style hash

# Disable color for piping / clipboard
flow graph tree ep-cases-create --no-color
```

### Single-object output example

```
┌─ Component: ep-cases-create  [APIEndpoint]
│  file: internal/handler/cases/handler.go
│
│  Depends on:
│    └── svc-cases-create  internal/service/cases.go
│
│  Used by:
│  └── ctx-cases-new-modal  [Context]
│       ├── Step 2: "Fill in case details"  [s-create-a-case]
│
│  Co-located in same Context:
│       └── ui-cases-new-form
│
└─────────────────────────────────────────────────────────────────
```

With `--style go` the output has `//` prefix — paste directly above a handler:

```go
// ┌─ Component: ep-cases-create  [APIEndpoint]
// │  ...
func (h *Handler) CreateCase(w http.ResponseWriter, r *http.Request) {
```

### Type-list output example (`--type APIEndpoint`)

```
── APIEndpoint (30 total)
──────────────────────────────────────────────────────────────────────
  ep-cases-create                                   POST /api/v1/cases → svc-cases-create
  ep-mail-get-case-inbox                            GET /api/v1/cases/{caseId}/inbox → svc-mail-get-case-inbox
  ep-employees-delete                                 [not_existing]
──────────────────────────────────────────────────────────────────────
```

`[not_existing]` = planned but not yet implemented. No tag = implemented.

---

## Workflow: "Is this endpoint already done?"

```bash
# 1. Check what's planned in the graph
flow graph get ep-cases-create

# 2. Check if it's in the swagger spec
flow spec --key ep-cases-create

# 3. Understand its full dependency tree
flow graph comment ep-cases-create
```

If `flow spec` shows the contract and `flow verify --key <scenario>` shows ✓
for the endpoint row, it's implemented. If `flow spec` prints "not found", the
handler hasn't been written yet.

---

## Integration with `flow verify`

`flow verify` now inlines a compact spec token in the APIEndpoint detail column:

```
ep-cases-create   spec: POST /api/v1/cases | params: bearer | → 200 401
```

This lets you see at a glance whether the spec matches what's planned, without
leaving the verify output. Use `flow spec --key <k>` when you need the full
parameter list.

---

## DB schema diff in `flow verify`

`flow verify` also runs a schema check on `SQLQuery` objects. It parses all
`db/migrations/*.up.sql` files and checks whether the tables referenced in
the query's SQL exist in the schema.

```
sq-insert-case    schema✗ (tables missing: cases)
```

When you see `schema✗`, the migration for that table hasn't been written yet.
Set `requires_migration: true` on the SQLQuery:

```bash
flow graph update --id <sq-entity-id> --properties '{"requires_migration": true}'
```

`flow verify` will show a lint warning if a SQLQuery description mentions
"new table", "add column", or "migration" but `requires_migration` is not set.
