---
name: flow-implement-scenario
description: >-
  Implement a planned scenario: create execution branches, write code following the component
  graph, verify implementation against the plan, and promote to main when done. Includes the
  tree viewer (`flow verify --key K`) and verification script (`flow verify`). Use whenever the
  user says "implement scenario X", "start working on scenario X", "show me the tree for X",
  "verify scenario X", "what's the status of scenario X", "promote scenario X", "init branch
  for X", "check implementation progress", "show scenario tree", "run verify", "what needs to
  be built for X", "which components are done", "show the dependency graph", "what's left to
  implement", "advance the scenario", "execute scenario X", or "continue implementing X".
metadata:
  author: emergent
  version: "9.4"
---

> **CRITICAL: Never use the `memory` MCP tool directly.** All graph reads and writes MUST go
> through the `flow` CLI (`flow graph`, `flow query`, `flow audit`, `flow journal`, etc.).
> The `memory` tool is not available in this workflow. If you cannot find a `flow` command
> for something, use `flow --help` or `flow graph --help` — do not fall back to `memory`.

# Skill: implement-scenario

Execute a planned scenario — from plan/next-gen graph to shipped code on main.

**Turn budget:** An implementation session for a 3–5 component scenario should complete in ≤30 turns. If you exceed this, stop iterating and summarize what is blocking you rather than continuing to loop.

**MANDATORY SESSION CLOSE RULE:** Before declaring any session complete and closing it, you MUST run `flow verify --key <key>` and report the exit code and output in your closing summary. A session MUST NOT be marked as done without a final verify run. This is non-negotiable — not even for planning-only sessions.

**Prerequisite:** A scenario must already exist on plan/next-gen with a component graph
(created by `plan-scenario`). If it doesn't exist, use `plan-scenario` first.

---

## Graph model (canonical)

The Memory graph has a specific layered architecture. Components don't hang off
Scenario directly — they hang off Context, and Context is reached through ScenarioStep.

```
Scenario
  ├── has → ScenarioStep          (what happens, sequenced)
  ├── acted_by → Actor            (who performs it)
  └── belongs_to → Domain         (taxonomy)

ScenarioStep
  ├── occurs_in  → Context        (WHERE the step happens — a screen/view/page)
  └── has_action → Action         (WHAT triggers the transition to the next step)

Action                             (required on all steps except the last)
  properties: description, type (click|submit|navigate|system), label (UI text)

Context                            (central node, fans out in PARALLEL)
  ├── requires → UIComponent      (what renders on screen)
  └── requires → APIEndpoint      (what the backend serves)

APIEndpoint
  └── calls → ServiceMethod       (business logic)

ServiceMethod
  └── uses → SQLQuery             (data access)

SQLQuery
  └── uses → SourceFile           (migration/schema)
```

### Key design points

- **UIComponent and APIEndpoint are parallel siblings** under Context — NOT chained
- **Context is the central fan-out node** — it's a place (screen/view) where things happen
- **ScenarioStep describes WHAT happens**, not HOW — it's always happening ON a Context
- **Action describes the trigger** — the user gesture or system event that causes the transition to the next step. Required on all steps except the last.
- **Components are never directly attached to Scenario** — always reached through ScenarioStep → Context

### Step ordering

Step order is stored as `properties.step_order` on the **Scenario** object — an ordered array of ScenarioStep entity IDs:

```json
{ "step_order": ["<step-id-1>", "<step-id-2>", "<step-id-3>"] }
```

- **`has` edges are unordered** — they declare membership, not position
- **Reordering** = one `objects update` on the Scenario (`step_order` array)
- **Adding a step** = create the `has` edge + append the step ID to `step_order`
- **Removing a step** = delete the `has` edge + remove the step ID from `step_order`
- **Step keys are content-based**, not position-based: `step-employee-clicks-archive`, not `s-case-archive-restore-step-2`
- The verify command falls back to alphabetical key order for legacy scenarios that predate `step_order`

To set `step_order` on a scenario:

```bash
flow graph update --id <scenario-id> \
  --properties '{"step_order": ["<step-id-1>", "<step-id-2>", "<step-id-3>"]}'
```

### Relationship vocabulary

| Rel type | Direction | Meaning |
|---|---|---|
| `has` | Scenario → ScenarioStep | behavioral steps |
| `belongs_to` | Scenario → Domain | taxonomy |
| `acted_by` | Scenario → Actor | who does it |
| `occurs_in` | ScenarioStep → Context | where the step happens |
| `requires` | Context → UIComponent | screen needs this file |
| `requires` | Context → APIEndpoint | screen needs API |
| `uses` | UIComponent → UIComponent | this file imports a shared component file |
| `calls` | UIComponent → APIEndpoint | UI handler route triggers this endpoint |
| `calls` | APIEndpoint → ServiceMethod | endpoint calls service |
| `uses` | ServiceMethod → SQLQuery | service uses query |
| `uses` | SQLQuery → SourceFile | query uses migration |

### UIComponent composition

A **UIComponent** is a file (`new_file` or `modify_existing`). Files depend on other files:

- A tab `.templ` file calls shared primitives from `internal/ui/components/ui/` (buttons, tables, avatars, badges, modals). Model this with `uses`.
- A UI handler route file (`handler.go`) triggers API endpoints. Model this with `calls`.

```
Context
  └── requires → UIComponent (verifications.templ)       ← the screen file
                    └── uses → UIComponent (table.templ)  ← shared table primitive
                    └── uses → UIComponent (avatar.templ) ← shared avatar primitive
                    └── calls → APIEndpoint               ← route handler triggers API
```

**When to add `uses` edges:**

Only wire `uses` when the shared component file is **not yet implemented** and needs to be built as part of this scenario. If it already exists in `internal/ui/components/ui/`, there is no need to model it — just use it in code.

```bash
# Wire a tab file that depends on a shared table component
flow graph relate \
  --type uses --from <tab-ui-component-id> --to <table-ui-component-id>

# Wire a handler route that calls an API endpoint
flow graph relate \
  --type calls --from <handler-ui-component-id> --to <api-endpoint-id>
```

**Shared component files** (already exist — no need to model unless modifying):
```
internal/ui/components/ui/button.templ
internal/ui/components/ui/avatar.templ
internal/ui/components/ui/table/          ← table primitives
internal/ui/components/ui/badge-pill.templ
internal/ui/components/ui/filter.templ
internal/ui/components/ui/pagination.templ
internal/ui/components/modal/             ← modal shell
internal/ui/components/form/              ← form fields
internal/ui/components/nav/               ← navigation
```

---

## Planning tiers (`flow verify --list` output)

Branch membership is the source of truth — no stored status fields.

| Tier | Meaning | What's needed |
|---|---|---|
| `IN EXECUTION` | Has an `execute/<key>` Memory branch | Continue implementing |
| `READY` | Key + steps + components all present, no execute branch | Run `flow verify --key K --init-branch` to start |
| `NEEDS DESIGN` | Key + steps exist, but 0 components wired through Context | Create Context objects, wire ScenarioStep → Context → Components |
| `NEEDS KEY` | Has steps but no key assigned | Assign a key (s-xxx-yyy format) |
| `NEEDS STEPS` | Has key but no ScenarioSteps | Add ScenarioStep objects wired with `has` |

---

## Three-branch model

```
Memory: main              ← verified reality (what is actually shipped)
Memory: plan/next-gen     ← planning workspace (component trees, all not_existing)
Memory: execute/<key>     ← execution tracking per scenario (status advances here)

Git: master               ← production code (main checkout at the project directory)
Git: scenario/<key>       ← implementation work for one scenario
Worktree: /tmp/lp-work/<key>  ← isolated working copy for this scenario (use this!)
```

Always implement inside the worktree — never switch branches in the main checkout.

---

## ⛔ Never set `verdict` on a Scenario

The `verdict` field (`backend_only`, `full_stack`, `ui_only`) **must never be set manually** by a planning agent. It is a legacy free-text field that has caused agents to skip UIComponent planning entirely.

The `flow verify` command now derives scope automatically from what is actually wired in the graph:
- If a Context has UIComponents → UI work is needed
- If a Context has APIEndpoints → backend work is needed
- If a Context name contains `tab`, `modal`, `page`, `form`, `list`, etc. and has no UIComponent → the verifier warns

**Do not set `verdict` when planning a scenario.** Do not read it to decide whether to add UIComponents. Let the graph structure speak for itself.

---

## ⛔ CRITICAL: Always use `flow graph` for all graph operations

All graph reads and writes go through `flow graph`. Never use any other CLI tool to query or mutate graph objects.

```bash
# Find a Context by name (JSON filter)
flow graph list --type Context --json \
  | python3 -c "import sys,json; [print(i['id'],'|',i.get('key',''),'|',i['properties'].get('name','')) for i in json.load(sys.stdin)['items'] if 'email' in i['properties'].get('name','').lower()]"

# Find a UIComponent by file path
flow graph list --type UIComponent --json \
  | python3 -c "import sys,json; [print(i['id'],'|',i.get('key',''),'|',i['properties'].get('file','')) for i in json.load(sys.stdin)['items'] if 'cases' in i['properties'].get('file','')]"

# Get a specific object by key
flow graph get ctx-case-email-tab

# Get a specific object by entity_id
flow graph get <entity_id>
```

For file/code lookups use shell tools directly:

```bash
ls internal/handler/cases/          # list files in a directory
grep -r "FunctionName" internal/    # find a symbol
find . -name "*.go" -path "*/cases/*"  # find files by pattern
```

---

## ⛔ CRITICAL: All graph mutations and reads must use `flow graph`

Never bypass `flow graph` for direct API calls or other graph CLIs. `flow graph` enforces branch isolation, auditability, and consistent error handling.

```bash
# ✅ Correct — always use flow graph
flow graph create --type Context --key ctx-foo ...
flow graph relate --type requires --from <id> --to <id>
```

---

## ⛔ CRITICAL: Never use `cd <dir> && <cmd>`

Use the `workdir` bash tool parameter instead:

```bash
# ❌ Forbidden
cd /root/<project> && flow graph list --type Context

# ✅ Correct — use workdir parameter set to the project directory
flow graph list --type Context
```

---

## ℹ️ Don't load `plan-*` skills to resolve verify warnings

The `flow verify` output already shows the exact commands needed to fix each warning. Copy and run them directly — do not load `plan-context`, `plan-action`, or other `plan-*` skills mid-session just to get the command syntax.

---

## The flow binary

The `flow` binary automates the entire lifecycle. Always use it — don't manually update the Memory graph during implementation.

### flow next — find the next scenario to plan or implement

```bash
# Find the next scenario to plan (by step count, excludes scenarios with execute branch)
flow next --plan
flow next --plan --domain cases

# Find the next scenario to implement (fewest components first)
flow next --implement
```

> Branch membership is the status: `execute/<key>` branch exists = in execution.
> No claim/release lifecycle. No planning_status fields.

### flow verify — view, verify, and promote

```bash
# Orient: list all scenarios with status + branch info
flow verify --list

# Filter by tier
flow verify --list --tier planning        # only planning-phase scenarios
flow verify --list --tier implementation  # only scenarios with execute branches
flow verify --list --domain cases         # filter by domain

# View a scenario's full component tree with real-time codebase checks
flow verify --key s-company-archive-restore

# Create git branch + Memory execute branch (gates on user_value being set)
flow verify --key s-company-archive-restore --init-branch

# Promote: Dry-run first
flow verify --key <key> --promote

# Promote: Apply if no conflicts
flow verify --key <key> --promote --execute
```

**What `flow verify --key K` shows:**
- The full dependency tree: Scenario → Steps → Components
- Real codebase status per component (actual file checks, not stored status)
- Progress bar with percentage complete

---

## Workflow

### Step 0 — Orient in one pass (do this before anything else)

Run `flow verify --key <key>` **once** and read ALL component properties — `file`, `name`, `handler`, `path` — from the tree output before touching any files.

- Do not re-read source files between components — collect everything upfront in this single pass
- Use `flow verify --key <key> --json` to get structured component properties if you need machine-readable output
- Do not issue follow-up graph queries for data already visible in the tree

> **Graph-first file navigation:** Before grepping or reading files, check the graph object's `file`, `handler`, and `path` properties from the `flow verify --key <key>` output. These tell you exactly which file to open. Only fall back to grep/glob when the graph doesn't have the path. Never spend turns on broad codebase searches for information already in the graph.

> **Batch discovery rule:** In a single message at the start of the session, issue ALL graph reads (`flow graph get` for each component) and ALL file reads you can anticipate from the verify output. Do not issue them one at a time across multiple turns. Parallelise everything you know you'll need before writing any code.

> **Never use `find` or `ls` to locate flow config files or scenario state.** Use these flow commands instead:
> - `flow verify --list` — see all scenarios and their branch/implementation state
> - `flow worktree path <key>` — find the worktree directory for a scenario
> - `flow graph get --key <key>` — inspect any graph object by key
> - `flow verify --key <key>` — see full component tree for a scenario

> **Why:** Re-reading the same files across multiple turns is the #1 cause of inflated session length. One orientation pass, then implement.

### Step 1 — Orient

Always start a session by viewing the current state:

```bash
flow verify --key <key>
flow verify --list
```

**Memory graph freshness:** The graph is a planning snapshot — it may lag the codebase by a session or two. If a component's `file` or `name` property looks wrong (e.g. path doesn't exist on disk), fix it in the graph with `flow graph update` before proceeding.

### Step 2 — Init branch (first time only)

If the scenario has no git or execute branch yet:

> Before stashing or switching branches, run `git branch --show-current`. If already on `scenario/<key>`, no stash or checkout is needed.

```bash
flow verify --key <key> --init-branch
```

> **⚠️ Do NOT manually copy `.flow.yml`** — `flow verify --init-branch` creates it automatically on the scenario branch. If you copy `.flow.yml` from master before running this command, the command will fail with a wrong-branch error. If you see a `.flow.yml` error, verify you are running the command from the correct directory (the main checkout, not a worktree).

This creates:
- Git branch `scenario/<key>` from master (by `--init-branch`)
- A git worktree at `/tmp/lp-work/<key>` checked out to that branch (by `flow worktree add`)
- Memory branch `execute/<key>` forked from plan/next-gen (all objects copied atomically, canonical IDs preserved)

**Environment check:** Before running `task migrate` or any service command, confirm `.env.local` exists in the worktree. If missing, symlink it from the main checkout:
```bash
ln -sf /root/legalplant-api/.env.local /tmp/lp-work/<key>/.env.local
```

**All subsequent implementation work (file edits, builds, commits) must happen inside the worktree directory `/tmp/lp-work/<key>`.** This keeps the main checkout on `master` and allows multiple scenarios to run in parallel sessions without conflicts.

To get the worktree path for an already-initialised scenario:
```bash
flow worktree path <key>   # prints /tmp/lp-work/<key>
```

### Step 3 — Implement

Work through the components **bottom-up** following the dependency graph:

1. **Migration** — `internal/db/migrations/NNNNNN_<slug>.up.sql` (and `.down.sql`)
2. **SQL queries** — add to `.sql` files, run `task sqlc` to regenerate
3. **Service methods** — add to the service file, call the SQLC queries
4. **API handlers** — add handler funcs with Swagger annotations
5. **Routes** — register in `internal/server/server.go`
6. **UI** — update templ files and UI handlers

**Quality gates after each layer:**
- `task build` — must compile
- `task lint` — no new warnings
- `task test` — all tests pass

### Step 4 — Commit, then verify

**Always commit before running `flow verify`.** ScenarioSteps whose descriptions contain
neither UI keywords nor API keywords fall through to a git commit check — they will stay
`planned` forever until there is at least one commit on the branch vs master in `internal/`.

```bash
git add internal/
git commit -m "feat: <description>"
flow verify --key <key>
```

This checks every component against the actual codebase and shows the scan result table.
Safe to re-run at any time — read-only, no writes.

### Step 4b — Run constitution audit

Before promoting, run the audit to validate component completeness:

```bash
flow audit --key <key>
```

This checks required properties, key naming conventions, and planning-phase patterns.
`flow verify --promote` **gates on** the audit result — it will block if the audit is missing, stale (>24h), or has violations. Statuses:
- `✓ pass` — all checks passed
- `⚠ warn` — warnings only, promote allowed
- `✗ fail` — violations present, promote blocked

To force re-audit:
```bash
flow audit --key <key> --force
```

### Step 5 — Final check

When all components show `✓ implemented`:

```bash
task build && task lint && task test
flow verify --key <key>              # must exit 0
```

> **Trust verify output.** Once `flow verify` shows all steps `✓ implemented` and exits 0, proceed directly to promote. Do NOT re-read source files, git log, or handler code to "double-check" — that is wasted turns. Verify is the source of truth.

### Step 6 — Promote ← all commands are here, no need to reload the skill

### Before promoting — always check git state

```bash
git diff master...HEAD --stat
```

- **If output is empty:** The scenario was already implemented on master. Do NOT create a PR. Run `flow verify --promote --execute --allow-empty-diff` only if Memory needs syncing. Close the session with a summary.
- **If output shows changes:** Proceed with PR creation first, then promote after merge.

**Pre-promote checklist:**
1. Run `task build` — must exit 0. Do not promote if the build fails.
2. Run `flow verify --key <key>` — must exit 0 with all components `✓`.
3. Run `git diff master...HEAD --stat` — must show non-empty changes.

**Dry-run first**
```bash
flow verify --key <key> --promote
```

**Apply if no conflicts**
```bash
flow verify --key <key> --promote --execute
```

After promotion:
1. Execute branch merges to Memory main (the merge IS the signal — no status write needed)
2. Open a git PR (from the worktree):

```bash
# If still in worktree:
gh pr create --title "feat: <key>" --base master --head scenario/<key>

# If back in main checkout:
gh pr create --title "feat: <key>" --base master --head scenario/<key>
```

3. Clean up the worktree after the PR is merged:
```bash
flow worktree remove <key>
```

---

## What the flow verify command checks

### Planning completeness tree (`--key <key>` header)

When you run verify on a single scenario, it prints a nested planning tree that
walks the real graph model:

```
Planning completeness
├── ScenarioStep  ✓  3 steps
│   ├── Step 1: "Attorney opens case page"
│   │   └── Context  ✗  no context assigned (occurs_in)
│   ├── Step 2: "Attorney clicks Archive"
│   │   └── Context  ✓  Case Detail Page
│   │       ├── UIComponent   ✓  ui-case-archive-btn
│   │       └── APIEndpoint   ✓  ep-case-archive  PATCH /api/v1/cases/{id}/archive
│   │           └── ServiceMethod  ✓  svc-archive-case
│   │               └── SQLQuery   ✓  sq-soft-delete-case
│   │                   └── SourceFile ✓  sf-cases-migration
│   └── Step 3: "System confirms archive"
│       └── Context  ✗  no context assigned (occurs_in)
├── Actor         ✓  Attorney  (actor-attorney)
└── Domain        ✓  Cases  (domain-cases)
```

Each missing node shows actionable CLI guidance with exact entity IDs and
relationship types to wire.

### Per-component codebase checks

| Type | Check | `implemented` when |
|---|---|---|
| `SourceFile` | File exists on disk or git branch | Path exists |
| `SQLQuery` | `.sql` source exists + Go func in `sqlcdb/` | `func (q *Queries) Name(` found |
| `ServiceMethod` | File exists + signature/name in file text | Signature string found |
| `APIEndpoint` | File exists + handler func name in file | Handler func found |
| `UIComponent` | `.templ` or `_templ.go` file exists | File exists |
| `ScenarioStep` | Keyword match on description, then git commit fallback | See ScenarioStep check logic below |

### ScenarioStep check logic

`flow verify` uses this decision tree for each ScenarioStep, in order:

1. **UI keyword match** — if the step description contains any of: `navigate`, `profile page`, `upload button`, `ui`, `browser`, `click`, `opens`
   → walks `internal/ui/` for a `.templ` file whose name or content matches a domain word from the description
   → `verified` if found, `planned` if not

2. **API keyword match** — if the description contains any of: `/api/v1/`, `multipart`, `form-data`, `posts `, `request`, `response`, `returns`
   → checks `internal/server/server.go` for the route or a keyword from the description
   → `verified` if found, `planned` if not

3. **Git commit fallback** — if neither keyword set matches
   → checks `git log master..<branch> -- internal/` for any commits
   → `verified` if commits exist, `planned` if not

**Implication:** Steps that describe filtering, displaying, or system state changes (e.g. "Employee filters the list…", "System displays archived records") often fall into bucket 3. They will stay `planned` until you commit. **Always commit before verifying.**

### If `flow verify` output is unchanged after a fix attempt

**Stop. Do not re-run `flow verify` again.** Re-running verify with no code changes between runs produces identical output — it is a read-only command.

When verify shows a component still `planned` after you believed you fixed it:

1. **Read the component type** — which bucket is it? (SourceFile, SQLQuery, ServiceMethod, APIEndpoint, UIComponent, or ScenarioStep)
2. **For ScenarioStep** — check which bucket the description falls into (UI keywords? API keywords? Neither → git commit fallback). If bucket 3, you must commit before re-running verify.
3. **For other components** — the file or symbol the checker looks for is genuinely missing. Check the `file`, `name`, `handler`, or `path` property on the component and verify the file exists and contains the expected symbol.
4. **Fix the root cause**, then re-run verify once.

> **Never run `flow verify` more than once without making a code change or commit between runs.**

### Dependency enforcement (edge metadata)

Any relationship edge can declare itself a blocking dependency by setting
`properties.dependency = true` on the edge. Dependencies are declared in the graph,
not inferred from code. Only edges with `{"dependency": true}` are enforced.

When a dependency is unmet (destination object hasn't reached its DONE threshold per
`DONE_POLICY`), the source object is capped at `planned` status and the detail column
shows `blocked by: <key>(<status>)`.

### Optional component links (`optional` edge property)

A `requires` edge from Context → UIComponent or Context → APIEndpoint can be marked
optional by setting `properties.optional = true` on the edge. This means the component
is a soft expectation — the scenario can be complete without it (e.g. a static/text-only
screen may not need a UIComponent; a read-only view may not need an APIEndpoint).

- **Default (no property, or `optional: false`)** → required. Missing = warning `⚠` in the tree.
- **`optional: true`** → soft. Present components show `○` (grey) instead of `✓`. Missing = `—` (informational).

To mark a requires edge as optional:

```bash
# 1. Delete the existing edge
flow graph unrelate <rel-id>

# 2. Recreate with optional: true
flow graph relate \
  --type requires \
  --from <context-id> \
  --to <component-id> \
  --properties '{"optional": true}'
```

---

## Gotchas

- **`node_modules` is absent in git worktrees** — if the scenario requires frontend tooling (templ, bun, npm), run `bun install` (or `npm install`) in the worktree directory before running any build commands. The worktree does not inherit `node_modules` from the main checkout.
- **Turn budget: ≤30 turns for a 3–5 component scenario.** If you exceed this, stop implementing and summarize what is blocking you — do not continue looping. Common causes: re-reading files already read, running verify before committing, not using the worktree.
> **Never promote Memory before confirming git diff is non-empty.** Promoting with an empty diff leaves the graph out of sync with reality if the PR fails.

- **Always commit before running `flow verify`** — steps without UI/API keywords in their description use a git commit check as the fallback. They will stay `planned` forever until there is a commit on the branch vs master in `internal/`. This is the most common cause of a step being stuck at `planned` despite the code being done.
- **A step showing `planned` with all context components `✓` means the description fell into the git commit bucket** — don't add more graph components; just commit and re-verify.
- **Never promote without `flow verify --key <key>` exiting 0** — visual inspection is not sufficient
- **One git branch per scenario** — never implement two scenarios on the same branch
- **`ServiceMethod` objects are now fully supported on branches.**
- **`task sqlc`** is required after editing `.sql` files — always use `task sqlc`, never raw `sqlc generate`. This runs both `sqlc generate` and the NullTime patch (`sqlc:fix`)
- **Swagger:** use `swag init -g cmd/api/main.go -o docs/swagger --parseDependency --parseInternal` directly (the Taskfile `--oas 3` flag may not be supported)
- **`templ` and `sqlc` binaries** are in `/root/go/bin` — add to PATH if needed: `export PATH="$PATH:/root/go/bin"`
- **Pre-existing LSP errors** may exist on the branch (e.g. `ui.VersionMiddleware`, `GalleryDB`) — these are unrelated to scenario work
- **Always run `flow verify --list` at the start of a session** to see which scenarios have branches
- **Validate component file paths before implementing** — before writing code, check that `file` properties in the graph point to real paths (`ls <path>` or `glob`). Graph plans are written before implementation; the actual path may differ slightly. Fix the graph property with `flow graph update` if wrong rather than creating files at wrong paths.

---

## Description Metadata (backward compatibility)

Note: `ServiceMethod` objects created before #127 was fixed were represented as `SourceFile` objects with a `signature` property — these still verify correctly since the checker logic is identical.

For objects created before issue #121 was fixed, implementation metadata was encoded
as `key: value` pairs inside `properties.description`:

    "SoftDeleteEmployee query. file: internal/db/queries/employees.sql | name: SoftDeleteEmployee"

The script still reads this format, so old scenarios continue to work without migration.

**For new objects:** `file`, `name`, `signature`, `domain` are first-class properties
and should be stored directly, not encoded in description.

---

## Audit output triage

`flow audit --key <key>` output is split into two tiers:

1. **Needs review** (`rule-*` warnings) — these are high-signal. Act on them before implementing.
   - `rule-api-auth-guard` — auth guard must be first in the handler
   - `rule-db-org-scoped` — SQL queries must include org_id filter
   - `rule-service-input-structs` — mutating service methods need typed input structs
   - etc.

2. **Pattern suggestions** (`pat-*` warnings, dimmed) — these are low-signal "verify if applicable" hints.
   Skip any that don't apply to this scenario (e.g. `pat-api-paginated-list` on archive/mutation endpoints).

The outcome is `warn` whenever any warnings exist — this is expected for almost every scenario.
`warn` passes the `--init-branch` gate. Only `fail` (violations) blocks it.

---

## Reverse relationship lookup

To find what contexts or scenarios require a given component (incoming edges):

```bash
# What contexts require this UIComponent?
flow graph rels --reverse <component-entity-id>

# What contexts require this APIEndpoint?
flow graph rels --reverse <endpoint-entity-id>
```

This is useful when you want to know "where is this component used?" without doing a `flow graph list` + filter.

---

## Abort / Rollback

If a scenario implementation needs to be abandoned:

```bash
# 1. Remove the worktree (returns to main checkout automatically)
flow worktree remove <key>

# 2. Delete the git branch
git branch -D scenario/<key>
```

The plan/next-gen branch is never mutated during execution, so no cleanup is needed there.
To retry, simply run `flow verify --key K --init-branch` followed by `flow worktree add <key>`.

---

## Session Close Protocol

Before closing ANY session (implementation or planning), you MUST:

1. **Run final verify:**
   ```bash
   flow verify --key <key>
   ```

2. **Report the result** in your closing summary:
   - Exit code (0 = all implemented, non-zero = items remaining)
   - Any remaining `✗ planned` or `✗ not_existing` items
   - Any `step_order` mismatches or warnings

3. **Do NOT close the session** without this step. Even if you believe the work is done, the verify run is required to confirm the graph state matches the codebase.

If `flow verify` is not available (e.g., no execute branch yet), run `flow verify --key <key>` anyway — it will show the planning completeness tree, which is still a useful final state snapshot.

**Closing summary template:**
```
Session complete. Final flow verify:
  Exit code: 0
  Steps: N implemented, 0 remaining
  No mismatches or warnings
```
