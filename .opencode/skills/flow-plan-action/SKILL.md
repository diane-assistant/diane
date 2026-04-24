---
name: flow-plan-action
description: >-
  Plan and wire Action objects in the Memory graph.
  Use whenever the user says "add an action to a step", "fix no action assigned
  warning", "wire has_action", "create an action for step N", "what triggers the
  transition", "what type should the action be", "add a click action", "what is
  an Action?", or "what's the difference between Action and Context?". Covers
  Action creation, has_action wiring, type values (click/submit/navigate/system),
  label field, and the rule that only non-last steps need an Action.
metadata:
  author: emergent
  version: "1.0"
---

> **CRITICAL: Never use the `memory` MCP tool directly.** All graph reads and writes MUST go
> through the `flow` CLI (`flow graph`, `flow query`, `flow audit`, `flow journal`, etc.).
> The `memory` tool is not available in this workflow. If you cannot find a `flow` command
> for something, use `flow --help` or `flow graph --help` — do not fall back to `memory`.

# Action Planning & Wiring

In the Memory graph, an **Action** describes the user gesture or system event that triggers the transition from one `ScenarioStep` to the next.

## Canonical Model
```
ScenarioStep
  ├── occurs_in  → Context    (WHERE it happens)
  └── has_action → Action     (WHAT triggers the next step)
```

## Action Rules
- **Required on all steps except the last** in a scenario.
- `flow verify` shows `⚠ no action assigned` for non-last steps missing an Action.
- The last step has no Action (no next step to transition to).
- **The action on step N describes what the user does to leave step N and arrive at step N+1.**
  - Step 1 action → what triggers the move to step 2
  - Step 2 action → what triggers the move to step 3
  - Last step → no action
- **Always plan and wire actions in step-sequence order** (step 1 first, then step 2, etc.) to avoid wiring the wrong action to the wrong step.

## Properties
| Property | Required | Description |
|---|---|---|
| `description` | yes | Full sentence describing the user gesture: "Admin clicks the Delete button in the row actions menu" |
| `type` | yes | One of: `click`, `submit`, `navigate`, `system` |
| `label` | no | The visible UI label of the button/link, e.g. `"Delete"` |
| `element` | no | The specific HTML element the user interacts with, e.g. `"Bulk Reset button in toolbar"`, `"checkbox per slot row"`, `"Confirm button in modal"` |

### Type Values
- `click`: User clicks a button or link.
- `submit`: User submits a form.
- `navigate`: User navigates to a different page/route.
- `system`: System-triggered (e.g., redirect after save).

### `element` field guidance

Use `element` to name the **specific control** the user touches. This bridges the gap between the step narrative and the UIComponent's required elements list — the implementer can match the action's `element` to the corresponding item in the UIComponent `description`.

Examples:
| Scenario | `label` | `element` |
|---|---|---|
| User checks a row | "Select Slots" | `"checkbox per slot row"` |
| User clicks toolbar button | "Bulk Reset" | `"Bulk Reset button in toolbar"` |
| User submits a modal form | "Create" | `"Confirm button in modal"` |
| User clicks a nav tab | "API Keys" | `"API Keys link in settings sidebar"` |

**Rule:** Always set `element` for `click` and `submit` actions. It is optional for `navigate` and `system`.

## Key Commands

**Create an Action** (prints entity_id):
```bash
ACT_ID=$(flow graph create --type Action --key act-employee-clicks-delete \
  --properties '{"description": "Admin clicks the Delete button in the row actions menu", "type": "click", "label": "Delete", "element": "Delete button in row actions menu"}')
echo $ACT_ID
```

**Wire Step → Action (has_action):**
```bash
flow graph relate --type has_action --from <step-entity-id> --to $ACT_ID
```

**List existing Actions:**
```bash
flow graph list --type Action
```

## Naming Convention
Use `act-<subject>-<verb>-<object>`:
- `act-employee-clicks-delete`
- `act-admin-submits-form`
- `act-system-redirects-to-detail`

## Workflow: Fixing `⚠ no action assigned`
1. Run `flow verify --key <scenario>`.
2. For each non-last step with `⚠`:
   - Identify action type from step description.
   - **Check for an existing Action** that describes the same gesture before creating:
     ```bash
     flow graph list --type Action
     ```
     If a matching Action exists, **reuse it** — wire `has_action` from the step to the existing object. Do not create a duplicate.
   - If no match, create a new Action object with key, description, type, label.
   - Wire to step using `has_action` relationship.
3. Re-run verify to confirm warnings are resolved.
