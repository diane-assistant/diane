---
name: flow-add-pattern
description: >-
  Add a new Pattern to the legalplant-api planning graph. Use whenever the
  user says "add a pattern", "record this pattern", "we have a pattern for
  X", "add this as a pattern", "document this approach as a pattern",
  "add a UI pattern", "add a DB pattern", "add an API pattern", "add a
  service pattern", "this is a recurring pattern", "capture this pattern",
  "there's a pattern we follow for X", "flow-add-pattern", or
  "run flow-add-pattern". Covers Pattern object creation,
  key naming (pat-<category>-<slug>), allowed categories (ui/api/service/db),
  required vs optional properties, and running flow patterns to confirm.
metadata:
  author: emergent
  version: "1.0"
---

# Add Pattern

A **Pattern** is a named, reusable design or implementation solution that
captures *how* things are done in this codebase. Patterns are categorised by
layer (ui / api / service / db) and browsed with `flow patterns`.

Patterns differ from Rules: a Pattern describes a recommended solution; a
Rule is a non-negotiable constraint. A Rule might *enforce* that a Pattern
is followed, but they are separate objects.

---

## Pattern object schema

| Property | Required | Description |
|---|---|---|
| `name` | yes | Short human label, e.g. `"HTMX Partial Swap"` |
| `description` | yes | What this pattern is, when to use it, and what it covers |
| `category` | yes | `ui` \| `api` \| `service` \| `db` |
| `rationale` | no | Why this pattern was chosen over alternatives |
| `example` | no | Brief code snippet or usage example |

**Key naming:** `pat-<category>-<slug>`
- `pat-ui-htmx-partial-swap`
- `pat-api-paginated-list`
- `pat-service-error-wrap`
- `pat-db-nulltime`

---

## Categories

| Category | Layer | Examples |
|---|---|---|
| `ui` | Templ / HTMX / DaisyUI | HTMX swap, modal rendering, empty states, form error display |
| `api` | HTTP handlers / Swagger | Paginated list shape, soft delete, error response format |
| `service` | Go service layer | Error wrapping, context propagation, transaction handling |
| `db` | SQLC / PostgreSQL | NullTime, soft delete column, JSONB usage, cursor pagination |

---

## Workflow

### 1. Check for duplicates first

```bash
flow patterns
# or filtered:
flow patterns --category ui
```

If a similar pattern exists, update it instead of creating a new one:
```bash
flow graph update --key pat-ui-htmx-partial-swap \
  --properties '{"example": "updated example"}'
```

### 2. Create the Pattern object

```bash
flow graph create --type Pattern \
  --key pat-<category>-<slug> \
  --properties '{
    "name": "...",
    "description": "...",
    "category": "<ui|api|service|db>",
    "rationale": "...",
    "example": "..."
  }'
```

### 3. Confirm

```bash
flow patterns --category <category>
```

The new pattern should appear in the table. Use `flow graph get --key pat-<key>` to see the full detail including description, rationale, and example.

---

## Graph relationships (optional — wire after creating)

The schema supports these relationships for patterns:

| Relationship | Direction | Meaning |
|---|---|---|
| `follows` | Module → Pattern | A module/package follows this pattern |
| `extends` | Pattern → Pattern | This pattern is a specialisation of another |
| `exemplifies` | SourceFile → Pattern | A source file is a good example of this pattern |
| `counter_exemplifies` | SourceFile → Pattern | A source file is an example of what NOT to do |

To wire a source file as an example:
```bash
flow graph relate --type exemplifies \
  --from <sourcefile-entity-id> \
  --to <pattern-entity-id>
```

These relationships are optional but useful — they allow `flow graph tree pat-<key>`
to show which files implement or violate the pattern.

---

## Examples

### UI pattern: form error display

```bash
flow graph create --type Pattern \
  --key pat-ui-form-error-inline \
  --properties '{
    "name": "Inline Form Error Display",
    "category": "ui",
    "description": "Validation errors are rendered inline beneath each field using a DaisyUI label with text-error class. Never use toast notifications for field-level errors. The Templ component receives an errors map[string]string and renders each error next to its field.",
    "rationale": "Inline errors are faster to scan and require no JS. Toast errors disappear before the user reads them.",
    "example": "if err, ok := errors[\"email\"]; ok { <label class=\"label\"><span class=\"label-text-alt text-error\">{err}</span></label> }"
  }'
```

### DB pattern: cursor pagination SQL

```bash
flow graph create --type Pattern \
  --key pat-db-cursor-pagination \
  --properties '{
    "name": "Cursor Pagination",
    "category": "db",
    "description": "List queries use keyset/cursor pagination: WHERE id > $cursor ORDER BY id LIMIT $limit. The cursor is the last seen id, encoded as base64 for the API layer. Never use OFFSET — it degrades at scale.",
    "rationale": "OFFSET pagination is O(n) at the DB. Keyset pagination is O(log n) with an index on id.",
    "example": "-- name: ListCasesCursor :many\nSELECT * FROM cases WHERE id > @cursor ORDER BY id LIMIT @limit"
  }'
```
