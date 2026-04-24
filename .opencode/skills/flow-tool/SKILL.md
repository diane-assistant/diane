# flow-tool

Reference guide for the `flow` CLI used in agent sessions.

---

## Graph management

### Create a single object
```bash
flow graph create \
  --branch $BRANCH \
  --type <Type> \
  --key <key> \
  --status <status> \
  --properties '{"name": "...", "file": "..."}'
```

### Create multiple objects and relationships in one call (preferred)
```bash
flow graph create-batch --branch $BRANCH --file plan.json
# or from stdin:
echo '[...]' | flow graph create-batch --branch $BRANCH
```

Batch format — JSON array of `create` and `relate` ops:
```json
[
  {"op": "create", "type": "SQLQuery", "key": "sq-list-cases", "status": "not_existing",
   "props": {"name": "ListCases", "file": "internal/db/queries/cases.sql"}},
  {"op": "relate", "type": "uses", "from": "svc-case-list", "to": "sq-list-cases"}
]
```

Keys created earlier in the same batch are resolved automatically — no separate lookup needed.

### Relate two objects
```bash
flow graph relate --branch $BRANCH --from <key-or-id> --to <key-or-id> --type <rel-type>
```

### Get / list / update
```bash
flow graph get   --branch $BRANCH --key <key>
flow graph list  --branch $BRANCH --type <Type>
flow graph update --branch $BRANCH --key <key> --properties '{"name": "..."}'
```

### View dependency tree
```bash
flow graph tree --branch $BRANCH --key <key>
flow graph tree --branch $BRANCH --type Scenario
```

---

## Scenario verification

```bash
flow verify --key <scenario-key>          # verify one scenario
flow verify --list                        # list all scenarios with status
flow verify --list --status planned       # filter by status
flow verify --list --tier implementation  # filter by tier
```

---

## Spec lookup

```bash
flow spec --key ep-cases-create
flow spec --path /api/v1/cases --method POST
```

---

## Key-prefix conventions

| Type | Key prefix | Example |
|---|---|---|
| Scenario | `s-` | `s-case-create` |
| ScenarioStep | `step-` | `step-case-create-1` |
| Context | `ctx-` | `ctx-case-form` |
| Action | `act-` | `act-case-submit` |
| APIEndpoint | `ep-` | `ep-cases-create` |
| ServiceMethod | `svc-` | `svc-case-create` |
| SQLQuery | `sq-` | `sq-insert-case` |
| SourceFile | `sf-` | `sf-cases-migration` |
| UIComponent | `ui-` | `ui-case-form` |

---

## Object property reference

### Scenario
| Property | Required | Notes |
|---|---|---|
| `description` | yes | What the scenario does |

### ScenarioStep
| Property | Required | Notes |
|---|---|---|
| `description` | yes | What happens in this step |
| `order` | yes | Integer step number |

### Context
| Property | Required | Notes |
|---|---|---|
| `description` | yes | Screen or context description |

### Action
| Property | Required | Notes |
|---|---|---|
| `type` | yes | `click`, `submit`, `navigate`, or `system` |
| `label` | no | Human-readable label |

### APIEndpoint
| Property | Required | Notes |
|---|---|---|
| `method` | yes | HTTP verb (GET, POST, etc.) |
| `path` | yes | URL path |
| `handler` | yes | Go handler function name |
| `file` | yes | Path to handler file |

### ServiceMethod
| Property | Required | Notes |
|---|---|---|
| `name` | yes | Go method name |
| `file` | yes | Path to service file |

### SQLQuery
| Property | Required | Notes |
|---|---|---|
| `name` | yes | SQLC query name |
| `file` | yes | Path to .sql file |

### SourceFile
| Property | Required | Notes |
|---|---|---|
| `name` | yes | File name |
| `path` | yes | Full path |
| `signature` | no | Brief description of contents |

### UIComponent
| Property | Required | Notes |
|---|---|---|
| `description` | yes | What the component renders |
| `file` | yes | Path to .templ file |
| `new_file` | no | `"true"` if this is a new file |

---

## Common relationship types

| Relationship | From → To | Meaning |
|---|---|---|
| `occurs_in` | ScenarioStep → Context | Step happens in this context |
| `has_action` | ScenarioStep → Action | Step is triggered by this action |
| `requires` | Context → APIEndpoint / UIComponent | Context needs this component |
| `calls` | APIEndpoint → ServiceMethod | Endpoint calls this service method |
| `uses` | ServiceMethod → SQLQuery | Service uses this query |
| `uses` | SQLQuery → SourceFile | Query lives in this file |

---

## Journal

```bash
flow journal note "Free-text note about what was done"
flow journal note --branch <branch-id> "Note scoped to a branch"
flow journal list
```
