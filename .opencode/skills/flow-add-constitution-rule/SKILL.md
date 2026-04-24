---
name: flow-add-constitution-rule
description: >-
  Add a new Rule to the legalplant-api planning constitution and wire it to
  constitution-v1. Use whenever the user says "add a rule", "add a
  constitution rule", "add a constraint", "we should always do X", "every
  Y must have Z", "add this to the constitution", "make this a rule",
  "add a naming rule for X", "define a rule that all X must Y",
  "add a rule to check that Z", "flow-add-constitution-rule", or
  "run flow-add-constitution-rule". Covers Rule object creation, key naming
  (rule-<category>-<slug>), allowed categories, the applies_to field,
  auto_check regex for naming rules, wiring to constitution-v1, and
  running flow rules to confirm.
metadata:
  author: emergent
  version: "1.0"
---

# Add Constitution Rule

A **Rule** is a named, non-negotiable constraint in the planning graph.
Rules are collected in `constitution-v1` and can be checked against graph
objects with `flow check <Type>`.

---

## Rule object schema

| Property | Required | Description |
|---|---|---|
| `name` | yes | Short human label, e.g. `"UIComponent key prefix"` |
| `statement` | yes | The constraint in plain English — what must be true |
| `category` | yes | `ui` \| `api` \| `service` \| `db` \| `naming` \| `scenario` |
| `rationale` | no | Why this rule exists |
| `applies_to` | no | Comma-separated object type(s), e.g. `"UIComponent"` or `"UIComponent,Scenario"` |
| `auto_check` | no | A Go-compatible regex applied to the object **key** for automatic checking. Only set this when the check can be expressed as a key pattern. |

**Key naming:** `rule-<category>-<slug>`
- `rule-naming-ui-component-key`
- `rule-ui-empty-state`
- `rule-scenario-acceptance-criteria`
- `rule-api-pagination`

---

## Categories and when to use them

| Category | Use for |
|---|---|
| `naming` | Key prefix / slug conventions for any object type |
| `ui` | UIComponent quality, Templ conventions, HTMX patterns |
| `api` | APIEndpoint requirements, Swagger, pagination |
| `service` | ServiceMethod conventions, error wrapping |
| `db` | SQLQuery and SourceFile conventions |
| `scenario` | Scenario quality (acceptance criteria, step quality) |

---

## Workflow

### 1. Clarify the rule

Ask (or infer from context):
- What object type(s) does this apply to? (`applies_to`)
- Which category? (see table above)
- Can the check be expressed as a key regex? If yes, set `auto_check`.

### 2. Check existing rules first

```bash
flow rules
# or filtered:
flow rules --category naming
```

Avoid creating duplicates. If a similar rule exists, update it instead:
```bash
flow graph update --key rule-naming-ui-component-key \
  --properties '{"statement": "updated statement"}'
```

### 3. Create the Rule object

```bash
flow graph create --type Rule \
  --key rule-<category>-<slug> \
  --properties '{
    "name": "...",
    "statement": "...",
    "category": "<category>",
    "rationale": "...",
    "applies_to": "<Type>",
    "auto_check": "^<regex>$"
  }'
```

Capture the `entity_id` from the output — you need it to wire to the constitution.

### 4. Wire to constitution-v1

```bash
# Get the constitution entity ID
CONST_ID=$(flow graph get --key constitution-v1 | python3 -c "import sys,json; print(json.load(sys.stdin)['entity_id'])")

# Wire the rule
RULE_ID=<entity_id from step 3>
flow graph relate --type includes --from "$CONST_ID" --to "$RULE_ID"
```

### 5. Confirm

```bash
flow rules
# or filtered to the rule's category:
flow rules --category <category>
```

The new rule should appear in the table.

### 6. Run a check (optional but recommended)

If `applies_to` is set and objects of that type exist, run:
```bash
flow check <Type> --category <category>
```

This shows whether the new rule auto-passes, auto-fails, or is flagged for review.

---

## Auto-check patterns (naming rules)

Use these confirmed-working regex patterns as a reference when writing `auto_check`:

| Object type | Pattern | Example key |
|---|---|---|
| UIComponent | `^ui-[a-z][a-z0-9-]+$` | `ui-cases-detail-modal` |
| APIEndpoint | `^ep-[a-z][a-z0-9-]+$` | `ep-cases-create` |
| ServiceMethod | `^svc-[a-z][a-z0-9-]+$` | `svc-cases-get` |
| SQLQuery | `^sq-[a-z][a-z0-9-]+$` | `sq-get-case` |
| Pattern | `^pat-[a-z][a-z0-9-]+$` | `pat-ui-htmx-swap` |
| Rule | `^rule-[a-z][a-z0-9-]+$` | `rule-naming-api-key` |

---

## Example: add a rule that all Scenarios must have `user_value` set

```bash
flow graph create --type Rule \
  --key rule-scenario-user-value \
  --properties '{
    "name": "Scenarios must have user_value",
    "statement": "Every Scenario must have the user_value property set — a plain-English sentence describing the real outcome the user gains. This must be set before the planning step is considered complete.",
    "category": "scenario",
    "rationale": "Scenarios without user_value are planned in a vacuum. User value keeps implementation focused on outcomes.",
    "applies_to": "Scenario"
  }'
```

Then wire it (substitute the actual entity_id):
```bash
CONST_ID=$(flow graph get --key constitution-v1 | python3 -c "import sys,json; print(json.load(sys.stdin)['entity_id'])")
flow graph relate --type includes --from "$CONST_ID" --to <new-rule-entity-id>
```
