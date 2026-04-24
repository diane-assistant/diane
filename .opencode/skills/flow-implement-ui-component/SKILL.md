---
name: flow-implement-ui-component
description: >-
  Implement a UIComponent during scenario execution — write the Templ file, UI
  handler, or route registration that a planned UIComponent maps to. Use
  whenever the user says "implement the UI for X", "write the Templ file for
  Y", "add the UI component", "build the frontend for this step", "write the
  page for Z", "implement ui-component", or "what file do I create for this
  UIComponent?". Covers reading the component graph, new_file vs
  modify_existing, Templ conventions, HTMX wiring, and running task build:ui.
metadata:
  author: emergent
  version: "1.0"
---

> **CRITICAL: Never use the `memory` MCP tool directly.** All graph reads and writes MUST go
> through the `flow` CLI (`flow graph`, `flow query`, `flow audit`, `flow journal`, etc.).
> The `memory` tool is not available in this workflow. If you cannot find a `flow` command
> for something, use `flow --help` or `flow graph --help` — do not fall back to `memory`.

# Implement UIComponent

This skill covers writing the actual code for a **UIComponent** (Templ file, page, or UI handler route) during scenario implementation. It is the implementation counterpart to `plan-ui-component`.

## Mapping to Code
A UIComponent maps to:
- **Templ templates**: `internal/ui/pages/<domain>/` or `internal/ui/components/`
- **UI handlers**: `internal/ui/handler/<domain>/handler.go`
- **Route registration**: `internal/server/server.go`

## Implementation Steps
1. **Read from Graph**: Check `file`, `name`, and `change_type` (`new_file` or `modify_existing`).
2. **If `new_file`**: Create the Templ file, then run `task templ` to generate Go code.
3. **If `modify_existing`**: Edit the existing file to add the new element.
4. **Wire HTMX**: Use `hx-get`, `hx-post`, etc., to call the target **APIEndpoint**.
5. **Build UI**: Run `task build:ui` after any Templ changes to update CSS/Go code.

## Key Conventions
- **Location**: Pages in `internal/ui/pages/<domain>/`, shared in `internal/ui/components/`.
- **Partials**: Use `IsHTMXRequest(r)` in handlers to detect HTMX vs full-page load.
- **Styling**: Use DaisyUI 5 and Tailwind CSS v4 classes.
- **HTMX**: Follow HTMX v4 + Templ patterns (see `htmx-ui` skill).

## Related Skills
- `plan-ui-component`: How to add a UIComponent to the graph.
- `htmx-ui`: HTMX v4 + Templ + DaisyUI conventions.
- `daisyui`: DaisyUI 5 component library.
- `implement-api-endpoint`: The API this UI calls.
