---
name: flow-plan-scenario
description: >-
  Plan a new scenario on the plan/next-gen Memory branch. Creates the full implementation
  blueprint: Scenario + ScenarioSteps, Contexts, Actions, all required components (migration,
  SQL, service, API endpoints, UI), and wired relationships — ready for a future implementation
  session. Use whenever the user says "plan a scenario", "add a scenario to next-gen", "create
  a plan for X", "define the implementation plan for Y", "plan how to implement Z", "what would
  it take to build X — plan it in the graph", "add scenario to the branch", "blueprint X",
  "are you aware of the plan", "what's the plan for implementing X", "let's plan X",
  "I want to implement X — plan it", "what do we need to build for X", "design the
  implementation for X", "map out the scenario for X", "what's the next-gen plan for X",
  "check the plan/next-gen branch for X", or "look up the scenario plan for X".
metadata:
  author: emergent
  version: "10.1"
---

> **Turn budget:** A planning session should complete in ≤20 turns. An implementation session for a 3–5 component scenario should complete in ≤30 turns. If you exceed this, stop iterating and summarize what is blocking you rather than continuing to loop.

> **At turn 15:** If planning is not complete, stop adding new objects. Summarize what is done, what is missing, and why you are blocked. Do not continue past turn 20 under any circumstances.

> **CRITICAL: Never use the `memory` MCP tool directly.** All graph reads and writes MUST go
> through the `flow` CLI (`flow graph`, `flow query`, `flow audit`, `flow journal`, etc.).
> The `memory` tool is not available in this workflow. If you cannot find a `flow` command
> for something, use `flow --help` or `flow graph --help` — do not fall back to `memory`.
>
> **CRITICAL: Never read `.env.local`, `.env`, or any config file to extract tokens.**
> Never use `MEMORY_PROJECT_TOKEN` or any API token directly via `curl` or HTTP calls.
> All graph operations MUST go through `flow graph` commands. Raw API calls bypass branch
> isolation and will corrupt the graph.

> **If the skill tool fails to load:** Stop immediately and report the error — do NOT search the filesystem for `.md` files as a fallback. Do NOT read operator skills (`flow-session-start`, `flow-session-resume`, etc.) — those are for a different agent role. If skill load fails, ask the operator to run `flow install-skills --pack agent --force --prune` and restart the server.

> **CRITICAL: Memory First.** Query the graph before reading any source files. The graph contains file paths, handler names, component locations, and domain metadata. Only read source files to verify implementation details *after* the graph has been consulted. Do not grep handlers, list UI files, or read SQL files until you have exhausted what the graph already tells you. Agents that invert this order waste 10–15 turns.

---

## Step 0 — Mandatory state audit (do this before anything else)

Run these three commands in a **single parallel batch** before touching the graph or reading any files:

```bash
BRANCH="87b21b07-90cf-4738-b0c3-e2a0698bbcf0"

# 1. Check verify status for this scenario key (shows existing steps, contexts, components)
flow verify --key s-<domain>-<slug>

# 2. List all Scenario objects on the branch (detect duplicates / partial state)
flow graph list --type Scenario --branch "$BRANCH" --json \
  | python3 -c "import json,sys; [print(o['key'], '|', o['id']) for o in json.load(sys.stdin)['items']]"

# 3. Check step_order on the scenario if it already exists
flow graph get --key s-<domain>-<slug> --json 2>/dev/null \
  | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('properties',{}).get('step_order','not set'))"
```

**Decision tree based on what you find:**

| Situation | Action |
|---|---|
| Scenario does not exist | Proceed to Phase 1 |
| Scenario exists, fully planned (verify shows 0 warnings, all steps have Context + Action + UIComponent) | Skip to Phase 5 (validate) — do NOT re-create anything |
| Scenario exists, partially planned | List missing objects from verify output, create only those — never re-create existing objects |
| Scenario has orphaned/superseded objects | Delete them with `flow graph delete --id <id> --branch "$BRANCH"` before adding new ones |
| `modify_existing: true` on a UIComponent but the file doesn't exist on disk | Override: treat as `new_file` — set `modify_existing: false` |

**Validate `step_order` immediately** after reading existing steps. The `step_order` array on the Scenario must list all step entity IDs in the correct sequence. Mismatches cause verify to report steps out of order. Fix with:
```bash
flow graph update --id <scenario-id> --branch "$BRANCH" \
  --properties '{"step_order": ["<step-id-1>", "<step-id-2>", ...]}'
```


## Loading this skill

Load this skill using the `skill` tool with `name="flow-plan-scenario"` before doing anything else.
If you are reading this file directly from disk, you loaded it correctly — proceed.

Do NOT search for this skill via `glob`, `read`, or filesystem commands.

# Skill: flow-plan-scenario

Plan a new scenario on the `plan/next-gen` Memory branch. The user describes what they want;
you produce a complete, layered blueprint — anchored in a clear proposal, grounded in research,
expressed as graph objects that double as an implementation checklist.

**This skill is for planning only.** Once the plan is on the graph, hand off to
`implement-scenario` for execution, verification, and promotion.

**Branch ID (plan/next-gen):** `87b21b07-90cf-4738-b0c3-e2a0698bbcf0`

The structure mirrors OpenSpec's philosophy: **why → what → how → tasks**.
Each phase gates the next. The graph objects are the tasks — checkable when implemented.

---

## Object Model

```
Scenario
  └── has ──► ScenarioStep  (ordered: what happens, from the user's perspective)
```

**Allowed properties per type (updated):**

| Type | Allowed Properties |
|---|---|
| `Scenario` | `description`, `name`, `given`, `when`, `then`, `and_also`, `step_order`, `user_value`, `ux_notes`, `acceptance_criteria` |
| `ScenarioStep` | `description` |
| `Context` | `description` |
| `Action` | `description` |
| `ServiceMethod` | `description`, `file`, `signature`, `name`, `domain` |
| `SQLQuery` | `description`, `file`, `name`, `domain` |
| `APIEndpoint` | `description`, `path`, `handler`, `method`, `file`, `domain` |
| `SourceFile` | `description`, `path`, `file`, `name`, `domain`, `signature` |

**Relationship types (updated):**

| Operation | Works? |
|---|---|
| `Scenario --has--> ScenarioStep` | YES |
| `Scenario --requires--> Component` | YES |
| `Scenario --calls--> Component` | YES |
| `Scenario --has_step--> ScenarioStep` | YES |
| `Context --contains--> Action` | YES |
| `ScenarioStep --occurs_in--> Context` | YES |
| `Feature --available_in--> Context` | YES |
| `Context --navigates_to--> Context` | YES |
| Creating `ServiceMethod` objects | YES — now fully supported |

### Component manifest pattern

The component manifest (`components: key1, key2` in the Scenario description) is kept for backward compatibility and as a human-readable summary. For new scenarios, prefer real `requires` relationships to link components to the Scenario — this enables graph traversal and better tooling support. Both coexist fine.

The `flow verify --key <key>` command parses this manifest to discover
components when BFS relationship traversal yields only ScenarioSteps.

### Structured metadata in descriptions

**Backward compatibility:** Old objects (created before issues #121, #127 were fixed) encoded `file`, `name`, `signature` as pipe-delimited pairs in `properties.description`. The verify/tree scripts still parse this format. For new objects, use first-class properties directly.

---

## Phase 1 — Proposal (why + what)

Before touching the graph, write a short internal proposal. This anchors everything that follows.

Answer three questions:
1. **Why does this scenario matter?** What problem does it solve for the user?
2. **What is missing?** Which capability gap does it fill (relative to stated requirements)?
3. **What is NOT in scope?** What related things are explicitly excluded?

State the scenario name, domain, and a one-sentence user-facing description.
Derive a stable key: `s-<domain>-<slug>` (e.g. `s-company-archive-restore`).

> This isn't a document you write — it's the thinking you do before picking up the graph CLI.
> If you can't answer these three questions, ask the user before proceeding.

---

## Phase 2 — Research (understand existing context)

> **Mandatory opening batch:** At the very start of Phase 2, issue ALL discovery queries (A–F below) in a single parallel batch — one message, all queries simultaneously. Do not wait for one result before issuing the next. Do not issue them sequentially across multiple turns. This is the single most important rule for keeping sessions short.

**Don't re-query what you already have.** Before issuing a graph lookup, check your conversation history. If you fetched this object or list in a previous turn, reuse that result — do not query again unless you have reason to believe it changed.

**First: check if the scenario already exists on the branch.**

```bash
BRANCH="87b21b07-90cf-4738-b0c3-e2a0698bbcf0"

# Does this scenario already exist on plan/next-gen?
flow graph list --type Scenario --branch "$BRANCH" --json 2>&1 | \
  python3 -c "import json,sys; objs=json.load(sys.stdin)['items']; [print(o['key'], o.get('properties',{}).get('status')) for o in objs if '<slug>' in o.get('key','').lower()]"

# Also check main branch by key
flow graph list --type Scenario --key "s-<domain>-<slug>" --json
```

If it exists on the branch: run `flow verify --key <scenario-key>` to see what is already wired. Only create objects that are missing — never re-create objects that already exist. Use `--upsert` for idempotent re-runs.
If it exists on main (status=verified): the scenario is already implemented — confirm with the user before proceeding.
If it doesn't exist anywhere: proceed to create it.

---

**Lookup rule:** Use `flow graph list --type X --json` for structured lookups when you know the type. Use `flow query` only for open-ended semantic search when the type or key is unknown. Never use both for the same lookup.

**If you know the key, use `flow graph get --key <key>` directly.** Only use `flow graph list` + filtering when you don't know the key. Never parse list output with python3 when a direct key lookup is available.

> **Verify-first rule:** `flow verify --key <scenario-key>` output contains all step keys, context keys, and component keys already wired. Extract what you need from this output before issuing any `flow graph get` calls. Only query the graph for objects not shown in verify output.

> **Wiring check:** Before wiring any relationship, run `flow graph rels <object-id>` to check what is already connected. Never wire a relationship that already exists — use `--upsert` for idempotent re-runs.

Run all remaining queries in a **single parallel batch — one turn, all results**. Do not issue follow-up queries unless a specific gap is found after reviewing the batch output.

```bash
# A. What already exists for this domain?
flow query "what API endpoints, service methods, SQL queries, and UI components exist for <domain>?"

# B. What Contexts and Actions exist for related scenarios?
flow query "what Context and Action objects exist for <domain> or <related domain>?"

# C. What is the closest analogous feature? (find the reference pattern)
flow query "how is <most similar feature> implemented? what API, service, SQL, UI?"

# D. DB schema — does the table have the columns we need?
# Schema discovery: prefer flow query before grepping raw SQL files
flow query "what columns and tables exist for <domain>?"
# Only grep migrations when the graph query returns insufficient detail:
grep -A 40 'CREATE TABLE' internal/db/migrations/000001_baseline.up.sql
grep -rn "<column_name>" internal/db/migrations/   # check later migrations too (007+)

# E. Handler and service — what methods already exist?
grep -n "^func" internal/handler/<domain>/handler.go internal/service/<service>.go 2>/dev/null

# F. UI — which pages and handlers exist?
ls internal/ui/pages/<domain>/ internal/ui/handler/<domain>/ 2>/dev/null
```

From this research, produce a **design classification** — every component sorted into:

| Classification | Meaning |
|---|---|
| **reuse** | Exists on main, no changes needed — just reference it |
| **modify** | Exists on main, needs additions — create a branch object flagging the change |
| **new** | Does not exist — must be created from scratch |

---

## Phase 3 — Design (how + which files)

Before creating any graph objects, resolve:

1. **Step map** — list 3–5 ScenarioSteps. Format: `"<Actor> <action>. System <reaction>."`
2. **Context map** — for each step, which screen/modal/page is the actor on? Does it already exist in the graph?
3. **Action map** — for each context, what action does the actor perform? Does it already exist?

   > **Sub-skill rule:** When wiring Actions, load the `flow-plan-action` skill. When wiring Contexts, load the `flow-plan-context` skill. Do not attempt to wire these objects from memory alone — the sub-skills contain the exact property schemas and wiring patterns.

4. **Component map** — for each action, what must be built/changed? (migration, SQL, service, API, UI)

   > **modify_existing validation:** When planning a UIComponent as `modify_existing: true`, confirm the file actually exists in the repo (`ls <path>`) before marking it as such. If unsure, default to `modify_existing: false` (new file). A wrong `modify_existing` flag causes multi-turn confusion during implementation.
5. **Reference pattern** — which existing feature is the canonical model to mirror?
   Be specific: `mirror ArchiveContact in contacts.sql`, not just "follow contacts pattern".
6. **Ordering** — migration → SQL → service → API → UI

The design classification from Phase 2 becomes the explicit checklist for Phase 4.

---

## Phase 4a — Batch create via `flow graph` (preferred)

Use `flow graph create` and `flow graph relate` to create all objects and wire relationships.
For bulk creation, compose a shell script or run commands sequentially — `flow graph` handles
upsert, branch scoping, and first-class property storage automatically.

```bash
BRANCH="87b21b07-90cf-4738-b0c3-e2a0698bbcf0"

# Create the Scenario
SCENARIO_ID=$(flow graph create \
  --type Scenario \
  --key "s-<domain>-<slug>" \
  --status not_existing \
  --branch "$BRANCH" \
  --properties '{
    "description": "<One sentence. User can X. System does Y.>",
    "name": "<Scenario Name>"
  }' --json | python3 -c "import json,sys; print(json.load(sys.stdin)['entity_id'])")
# Domain is expressed via belongs_to relationship, not a property

# Create a ScenarioStep
STEP_ID=$(flow graph create \
  --type ScenarioStep \
  --key "s-<domain>-<slug>-step-1" \
  --status not_existing \
  --branch "$BRANCH" \
  --properties '{"description": "<Actor does X. System does Y.>"}' \
  --json | python3 -c "import json,sys; print(json.load(sys.stdin)['entity_id'])")

# Wire Scenario → Step
flow graph relate --type has --from "$SCENARIO_ID" --to "$STEP_ID" --branch "$BRANCH"

# Create a component
COMP_ID=$(flow graph create \
  --type SQLQuery \
  --key "sq-<verb>-<entity>" \
  --status not_existing \
  --branch "$BRANCH" \
  --properties '{
    "description": "<what the query does>",
    "file": "internal/db/queries/<domain>.sql",
    "name": "<QueryName>",
    "domain": "<domain>"
  }' --json | python3 -c "import json,sys; print(json.load(sys.stdin)['entity_id'])")

# Wire Scenario → Component
flow graph relate --type requires --from "$SCENARIO_ID" --to "$COMP_ID" --branch "$BRANCH"

> **Batching rule:** Create objects in groups, not one at a time.
> - **One call:** Create Scenario + all ScenarioSteps together (capture all IDs)
> - **One call:** Create all Contexts together
> - **One call:** Create all Actions together
> - **One call:** Wire all `has` (Scenario→Step), `occurs_in` (Step→Context), `has_action` (Step→Action) relationships
> - **One call:** Create all implementation components (SQLQuery, ServiceMethod, APIEndpoint, UIComponent, SourceFile)
> - **One call:** Wire all `requires` / `calls` / `uses` relationships
>
> Do NOT create one object, then wire it, then create the next. Batch creates, then batch wires.

**After the full batch of creates/wires:** run `flow verify --key <scenario-key>` once and confirm the tree reflects everything you added before proceeding.
```

The flow graph commands:
- Store `file`, `name`, `signature`, `domain` as first-class properties
- Use `--upsert` flag for idempotent re-runs (add to any `flow graph create` call)
- Scope writes to the correct branch via `--branch`

---

## Phase 4b — Manual CLI (fallback)

If you need fine-grained control or are creating/modifying individual objects,
use the CLI directly.

Use `--branch 87b21b07-90cf-4738-b0c3-e2a0698bbcf0` on **every** write.

```bash
BRANCH="87b21b07-90cf-4738-b0c3-e2a0698bbcf0"
```

### 4a. Scenario + ScenarioSteps

```bash
SCENARIO=$(flow graph create \
  --type Scenario \
  --key "s-<domain>-<slug>" \
  --status not_existing \
  --branch "$BRANCH" \
  --properties '{
    "description": "<One sentence. User can X. System does Y.> components: <comma-separated component keys>",
    "name": "<Scenario Name>"
  }' --json | python3 -c "import json,sys; print(json.load(sys.stdin)['entity_id'])")
# Domain is expressed via belongs_to relationship, not a property
```

### 4b. Contexts (where each step happens)

One `Context` per distinct screen, modal, or UI surface. Check if it already exists first.

```bash
CTX1=$(flow graph create \
  --type Context \
  --key "ctx-<domain>-<slug>" \
  --status not_existing \
  --branch "$BRANCH" \
  --properties '{
    "description": "<What the user sees on this screen>"
  }' --json | python3 -c "import json,sys; print(json.load(sys.stdin)['entity_id'])")
```

### 4c. Actions (what happens in each context)

One `Action` per discrete user action or system operation within a context.

```bash
ACT1=$(flow graph create \
  --type Action \
  --key "act-<domain>-<verb>-<slug>" \
  --status not_existing \
  --branch "$BRANCH" \
  --properties '{
    "description": "<What the user or system does. What API call fires. What state changes.>"
  }' --json | python3 -c "import json,sys; print(json.load(sys.stdin)['entity_id'])")
```

### 4d. Implementation components

One object per work item. Encode metadata in allowed properties.

| Layer | Type | Key pattern | Allowed props (updated) |
|---|---|---|---|
| DB migration | `SourceFile` | `sf-migration-<slug>` | `path`, `file`, `name` |
| SQL query | `SQLQuery` | `sq-<verb>-<entity>` | `file`, `name`, `domain` |
| Service method | `ServiceMethod` | `sm-<domain>-<verb>` | `file`, `signature`, `name`, `domain` |
| API endpoint | `APIEndpoint` | `ep-<domain>-<verb>` | `method`, `path`, `handler`, `file`, `domain` |
| Source file | `SourceFile` | `sf-<slug>` | `path`, `file`, `name`, `domain`, `signature` |
| UI change | `SourceFile` | `ui-<domain>-<slug>` | `path`, `file`, `name`, `domain` |

**Remember to add all component keys to the Scenario's description manifest:**
```
"description": "... components: sq-verb-entity, ep-domain-verb, sf-migration-slug"
```

Capture every `entity_id` from `--json` immediately.

### 4e. Wire all relationships

**Scenario hierarchy**

```bash
# Scenario has ScenarioStep (canonical)
flow graph relate --type has --from "$SCENARIO" --to "$S1" --branch "$BRANCH" --upsert

# Component linkage
flow graph relate --type requires --from "$SCENARIO" --to "$COMP_ID" --branch "$BRANCH" --upsert

# Full hierarchy (now all work)
flow graph relate --type occurs_in --from "$STEP_ID" --to "$CTX_ID" --branch "$BRANCH" --upsert
flow graph relate --type contains --from "$CTX_ID" --to "$ACT_ID" --branch "$BRANCH" --upsert
flow graph relate --type navigates_to --from "$CTX_FROM" --to "$CTX_TO" --branch "$BRANCH" --upsert
```

---

## Phase 5 — Validate the plan

Before moving to implementation, run `flow audit` to catch errors early:

```bash
# Validate a single scenario (checks required properties, key naming, planning patterns)
flow audit --key s-employee-soft-delete

# Force re-audit even if a recent one exists
flow audit --key s-employee-soft-delete --force

# JSON output
flow audit --key s-employee-soft-delete --json
```

The audit checks:
- Required property presence on all component objects
- Key naming conventions (auto_check rules)
- Planning-phase patterns that apply to each component type
- All constitution rules with `applies_to` matching a component type

To check naming/convention rules across all objects of a type:

```bash
flow check APIEndpoint
flow check UIComponent
flow check Scenario
flow check SQLQuery
```

Fix any errors before handing off to `implement-scenario`.

**Mandatory closing checklist — run this before declaring the plan complete:**

```bash
# 1. Full verify — must show 0 warnings
flow verify --key <key>

# 2. Per-step relationship completeness
flow verify --key <key> --json | python3 -c "
import json, sys
data = json.load(sys.stdin)
for step in data.get('steps', []):
    key = step.get('key', '?')
    ctx = step.get('context')
    action = step.get('action')
    ui = step.get('ui_components', [])
    print(f'Step {key}: context={bool(ctx)} action={bool(action)} ui={len(ui)}')"
```

Confirm for every step:
- [ ] `occurs_in` → Context is wired
- [ ] `has_action` → Action is wired (required on all steps except the last)
- [ ] Context `requires` ≥1 UIComponent
- [ ] `step_order` on the Scenario matches the intended sequence

A plan with any unresolved `⚠` warnings will block the implementation session from starting cleanly.

---

## Phase 6 — Journal note (durable record)

```bash
flow journal note "## Plan: <Scenario Name> (<YYYY-MM-DD>)

Scenario key: <key> | Domain: <domain> | Branch: plan/next-gen

### Proposal
Why: <one sentence — what user problem this solves>
Gap: <what is missing from the current implementation>
Out of scope: <what is explicitly excluded>

### Design
Reference pattern: <the analogous feature and its key files>
Implementation order: migration → SQL → service → API → UI

### Step → Context → Action map
1. Step: <step name>
   Context: <screen/modal name> (<new or existing>)
   Action: <what the user does>
   Fires: <API call>

### New / Modified (all not_existing on branch)
1. Migration: <file> — <ALTER TABLE description>
2. SQL: <query names> — <brief SQL intent>
3. Service: <method signatures> — added to <ServiceName> in <file>
4. API: <METHOD path> — <handler name> in <file>
5. UI: <what changes in which files>"
```

---

## Phase 7 — Present the plan

Output a clean summary, then tell the user to run `implement-scenario` to begin execution.

```markdown
## <Scenario Name> — Plan on `plan/next-gen`

**Why:** <one sentence from the proposal>
**Reference pattern:** mirrors `<analogous feature>` — see `<key files>`

### Component map

| Layer | What | File | Status |
|---|---|---|---|
| Migration | `ADD COLUMN archivedAt TIMESTAMPTZ` | `000010_company_archive.up.sql` | not_existing |
| SQL | `ArchiveCompany`, `UnarchiveCompany` | `contacts.sql` | not_existing |
| ... | ... | ... | not_existing |

### Next: implement

View the dependency tree:
  flow graph tree <key>

Start execution:
  flow verify --key <key> --init-branch

### Gotchas

- **Turn budget: ≤20 turns for a planning session.** If you exceed this, stop and summarize what is blocking you rather than continuing to loop. Common causes: not running queries in parallel, re-querying data already fetched, creating objects that already exist.

- **Use `flow verify --key <key> --summary` when iterating on fixes — it shows only warnings without reprinting the full tree. Use full `flow verify` only for initial state inspection.**

- **Never resolve `step_order` IDs via `flow graph list` or `flow graph get`** — these are internal version IDs that cannot be looked up directly. To get step entity IDs, run `flow verify --key <scenario-key>` and read the `[id]` shown next to each step in the output.

- **Always check existence before `flow graph create`** — use `flow verify` or `flow graph list --type <Type> --key <key>` first. Creating an object that already exists causes an error; use `--upsert` for idempotent creation.

- **Check for duplicate `occurs_in` before wiring** — before assigning a new `occurs_in` relationship to a step, check if one already exists via `flow graph rels <step-id>`. Delete the old one first if present to prevent duplicate context paths.

- **Never read `.env.local` or use tokens directly** — all graph operations go through `flow graph`. Reading env files and calling the memory API directly bypasses branch isolation and corrupts the graph.

- **Do not use `todowrite`** — it adds no value to the graph and inflates session length. Track progress mentally or in assistant text between tool calls. Use `flow journal note` for durable progress records if needed.
- **Never pass `--cursor ""`** — an empty cursor string silently breaks JSON output. Omit `--cursor` entirely on the first page:
  ```bash
  flow graph list --type Scenario --json           # ✓ first page — no --cursor
  flow graph list --type Scenario --json --cursor <token>  # ✓ subsequent pages
  flow graph list --type Scenario --json --cursor ""       # ✗ breaks output silently
  ```
- **`flow graph get <key>` now searches all types automatically** — but if lookup fails, add `--type <Type>`:
  ```bash
  flow graph get s-my-scenario              # ✓ Scenario — always works
  flow graph get ctx-settings-team          # ✓ now searches all types
  flow graph get ctx-settings-team --type Context   # ✓ explicit fallback if needed
  ```
- **All relationship types now work on branches — the full `Scenario → Step → Context → Action → Component` hierarchy can be expressed as real graph edges**
- **Use `--upsert` on all `relationships create` calls for idempotent re-runs**
- **Description metadata encoding (`file: path | name: func`) is supported for backward compatibility** in old objects but is no longer needed for new objects.
- **`--name` flag maps to `properties.name`** which is rejected for `Scenario` type — don't use it.
- **Cross-branch relationships fail** — if you need to reference a main-branch object, put its key/name in the description instead.
- **Check if Context/Action already exists** — query before creating. Don't duplicate objects.
- **Check all migrations** — a column might exist in a later migration (000007+), not the baseline.
- **Out of scope section matters** — it prevents scope creep in the implementation session.
- **Each graph object = one implementation task** — keep them granular enough to be individually checkable.
- **Always include the component manifest** — without it, `flow verify` cannot discover components for new scenarios.
- **Status is on the top-level object** — use `--status`, not `properties.status`.

---

## CLI Commands

Planning tasks use the `flow` CLI directly:

### Batch object creation

```bash
flow graph create --type Scenario --key <key> --description "<desc>" --upsert
flow graph create --type ScenarioStep --key <key> --description "<desc>" --upsert
flow graph relate --src <key> --rel implements --dst <key> --upsert
```

Handles: object creation with `--upsert` for idempotent re-runs, and relationships between plan objects.

### Validate plan integrity

```bash
flow audit --key <key> --stage plan    # planning-phase checks only (no code/file checks)
flow check Scenario       # validate all scenarios
```

Checks: manifest presence, key resolution, property presence,
type-specific required fields, and file existence warnings.
