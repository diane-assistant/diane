---
name: flow-plan-ui-component
description: >-
  Plan and wire UIComponent objects in the Memory graph. Use whenever the user says "add a UI component", "wire a Templ file", "add a page component", "what properties does UIComponent have?", "add a button component", "wire ui- to a context", "what's the key format for UI components?", "is this a new file or existing?", or "how do I attach a UI component to a context?". Covers UIComponent creation, requires wiring from Context, key naming (ui-<domain>-<description>), and reuse across scenarios.
metadata:
  author: emergent
  version: "1.0"
---

> **CRITICAL: Never use the `memory` MCP tool directly.** All graph reads and writes MUST go
> through the `flow` CLI (`flow graph`, `flow query`, `flow audit`, `flow journal`, etc.).
> The `memory` tool is not available in this workflow. If you cannot find a `flow` command
> for something, use `flow --help` or `flow graph --help` — do not fall back to `memory`.

# Plan UI Component

A **UIComponent** represents a **Templ file or UI handler route** that needs to be created or modified. It is wired to a **Context** via a `requires` relationship.

**One UIComponent = one file.** A single `.templ` file contains all the buttons, tables, checkboxes, avatars, and form fields for that screen — you do not create a separate UIComponent for each widget. The planning question is: *which files need to change?*

A Context typically requires 1–3 UIComponent entries:
- The `.templ` file for the screen itself (e.g. `verifications.templ`)
- The UI handler route file if a new route is needed (e.g. `handler.go`)
- A separate modal `.templ` if the screen opens a modal defined in its own file

## Key Naming

Three key namespaces exist in the graph:

| Prefix | What it represents | Example |
|--------|-------------------|---------|
| `ui-cmp-<ComponentName>` | A named shared component (`templ Foo(...)` in `internal/ui/components/`) | `ui-cmp-Button`, `ui-cmp-Table`, `ui-cmp-FormModal` |
| `ui-page-<ComponentName>` | A named page-level component (`templ Foo(...)` in `internal/ui/pages/`) | `ui-page-CasesListPage`, `ui-page-CaseDetailPage` |
| `ui-<domain>-<description>` | A scenario-specific file that needs to be **created or modified** | `ui-compliance-verifications-tab`, `ui-cases-add-property-modal` |

**When planning a scenario:**
- Use `ui-cmp-*` / `ui-page-*` keys to reference **existing** shared components the scenario depends on
- Use `ui-<domain>-<description>` keys for **new files** or **modified files** specific to the scenario

**When wiring a Context:**
- Wire `requires` to the scenario-specific `ui-<domain>-*` node (the file being changed)
- Wire `contains` from that node to the `ui-cmp-*` / `ui-page-*` nodes it uses

Example — Cases List Page (real composition from `cases/list.templ` + `cases/table.templ`):
```
Context: Cases List
  └── requires → ui-page-CasesListPage              (existing: cases/list.templ)
                    └── contains → ui-cmp-Page            layout.Page
                    └── contains → ui-cmp-AppShell         layout.AppShell
                    └── contains → ui-cmp-PageHeader       nav.PageHeader
                    └── contains → ui-cmp-StatCards        ui.StatCards
                    └── contains → ui-cmp-ListArea         table.ListArea
                    └── contains → ui-cmp-TableEmpty       table.TableEmpty
                    └── contains → ui-cmp-Avatar           ui.Avatar
```

Example — new file scenario (verifications tab doesn't exist yet):
```
Context: Case Verifications Tab
  └── requires → ui-compliance-verifications-tab    (new file: cases/tabs/verifications.templ)
                    └── contains → ui-cmp-Table
                    └── contains → ui-cmp-Checkbox
                    └── contains → ui-cmp-Button
                    └── contains → ui-cmp-FilterCard
                    └── contains → ui-cmp-Pagination
```

## Properties
- `name`: Human-readable display label (e.g., `"Case Detail Archive Button"`)
- `component_name`: **The Templ function name as it appears in code** (e.g., `"CaseDetailArchiveBtn"`). Use PascalCase matching the `templ` function definition. Set this whenever you know the function name.
- `file`: Relative path (e.g., `internal/ui/pages/cases/detail.templ`)
- `description`: **Required.** Structured summary of what this file renders and what interactive elements it must contain for the scenario to work. See format below.

⛔ Do NOT add `domain` — domain is expressed via the Scenario's relationship to a Domain object, not as a property on components.
⛔ Do NOT add `change_type` — whether a file is new or modified is derivable from git diff, not stored in the graph.
⛔ Do NOT add `new_file` — this property does not exist and will be rejected with `Error: forbidden properties: new_file`. Whether a file is new or modified is **not stored as a property**. Convey it in the `description` field instead (e.g. "New file. Renders the…" or "Modifies existing detail.templ to add…").

## Description Format

The `description` field must list the **required elements** — the specific HTML structures, interactive controls, and dynamic behaviours the implementer needs to build inside this file. Without this, the implementer has to re-derive the UI from the step narrative alone.

Format:
```
<One sentence: what this file renders overall.>

Required elements:
- <element 1> — <purpose / behaviour>
- <element 2> — <purpose / behaviour>
- ...
```

Example for a verifications tab:
```
Renders the list of verification slots for a case.

Required elements:
- Checkbox per slot row (value=slot_id) — allows multi-select
- Toolbar (hidden by default) — appears when ≥1 checkbox is checked, contains Bulk Reset button
- Selected count badge — shows how many slots are selected
- Bulk Reset button (in toolbar) — triggers confirmation modal via hx-get
```

Example for a modal:
```
Confirmation modal for bulk-resetting selected verification slots.

Required elements:
- Selected slot count display — shows how many slots will be reset
- Confirm button — submits POST /api/v1/cases/{caseID}/verifications/bulk-reset via hx-post
- Cancel button — closes modal
```

**Rules:**
- Every UIComponent **must** have a `description`. Empty descriptions are not acceptable.
- Focus on elements that are **specific to this scenario** — don't list generic chrome (nav, breadcrumbs).
- When a UIComponent is reused across scenarios, **append** new required elements rather than overwriting.

## Commands
```bash
# Create UIComponent (prints entity_id)
UI_ID=$(flow graph create --type UIComponent --key ui-case-detail-archive-btn \
  --properties '{
    "name": "Archive Button",
    "file": "internal/ui/pages/cases/detail.templ",
    "description": "Case detail page header.\n\nRequired elements:\n- Archive button (in page header) — visible when case is active, triggers POST /app/cases/{id}/archive\n- Unarchive button (in page header) — visible when case is archived, triggers POST /app/cases/{id}/unarchive"
  }')
echo $UI_ID

# Wire Context → UIComponent
flow graph relate --type requires --from <ctx-id> --to $UI_ID

# Optional Component (renders as ○ in verify tree)
flow graph relate --type requires --from <ctx-id> --to $UI_ID --properties '{"optional": true}'
```

## Workflow

1. **Check for existing UIComponent** — search by file path or key before creating:
   ```bash
   flow graph list --type UIComponent --json \
     | python3 -c "import sys,json; [print(i['id'],'|',i.get('key',''),'|',i['properties'].get('file','')) for i in json.load(sys.stdin)['items'] if 'cases/detail' in i['properties'].get('file','')]"
   ```
   Replace the `if` filter with the file path or keyword you're looking for. If an object with the same `file` already exists, **reuse it** — just wire the new `requires` relationship from the Context to the existing object. Do not create a duplicate.
2. **Create** (only if no match found) using the command above.
3. **Wire** `Context` → `UIComponent` via `requires`.

## UIComponent → UIComponent dependencies (`uses`)

Files depend on other files. A tab `.templ` calls shared primitives; a handler route triggers API endpoints.

```
Context
  └── requires → UIComponent (verifications.templ)        ← the screen file
                    └── uses → UIComponent (table.templ)   ← shared table primitive
                    └── uses → UIComponent (avatar.templ)  ← shared avatar primitive
                    └── calls → APIEndpoint                ← route handler triggers API
```

**Wire a `uses` edge** when the dependency file is **not yet built** and must be created as part of this scenario:
```bash
flow graph relate \
  --type uses --from <screen-ui-id> --to <shared-ui-id>
```

**Wire a `calls` edge** when a UI handler route file triggers an API endpoint:
```bash
flow graph relate \
  --type calls --from <handler-ui-id> --to <api-endpoint-id>
```

**Do NOT model shared components that already exist** — if the file is already in `internal/ui/components/ui/` (button, avatar, table, badge, filter, pagination, modal, form, nav), just use it in code. Only add a `uses` edge if the shared file itself needs to be created or modified by this scenario.

## Related Skills
- `plan-context`: Create the Context this component attaches to.
- `plan-api-endpoint`: Plan the API side of the same Context.
