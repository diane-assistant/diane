## Why

The MCP server detail view in the macOS app is entirely read-only. Users see server configuration, context membership, and status but must navigate to a modal sheet (Edit) or a separate tab (Contexts, Cmd+2) to make any changes. This breaks the direct-manipulation principle — users should be able to inspect and modify a server's configuration in-place. Adding inline editing, context assignment with checkboxes, and a test-connection button will reduce navigation friction and make MCP server management feel like a first-class experience.

## What Changes

- **Replace the read-only detail view with an inline-editable detail view** — configuration fields (name, command/URL, args, env, headers) become editable text fields, `StringArrayEditor`s, and `KeyValueEditor`s directly in the detail pane instead of requiring a modal sheet.
- **Add context assignment via checkboxes** — the "Available in Contexts" section shows all contexts with checkboxes so users can assign/unassign a server to contexts without leaving the detail view.
- **Add a Save button** — a persistent save action in the detail header commits any configuration or context changes.
- **Add a Test Connection button** — lets users trigger a connectivity check for the selected MCP server and see the result inline, similar to the existing `testProvider` / `testAgent` patterns.
- **Wire `OAuthConfigEditor` into the detail view** — for SSE/HTTP servers, expose OAuth configuration editing using the existing `OAuthConfigEditor` component that is currently unused.
- **Remove the separate Edit sheet** — since editing is now inline, the modal `MCPServerFormSheet` is only needed for the Create flow.

## Capabilities

### New Capabilities
- `mcp-server-inline-edit`: Inline-editable MCP server detail view with save, covering all configuration fields, context assignment checkboxes, and OAuth editing.
- `mcp-server-test-connection`: Test-connection button and result display for MCP servers, including any necessary API endpoint.

### Modified Capabilities
_(none — no existing spec-level requirements are changing)_

## Impact

- **Views**: `MCPRegistryView.swift` — major rework of `serverDetailView(_:)` (lines 303-557) from read-only to editable. `MCPServerFormSheet` scope reduced to create-only.
- **ViewModels**: `MCPRegistryViewModel.swift` — new methods for inline save, context toggle, and test-connection. New `@Published` editing state properties.
- **API / Client**: `DianeClient.swift` — new `testMCPServer(id:)` method (requires a corresponding daemon endpoint). Context assignment methods already exist (`addServerToContext`, `removeServerFromContext`).
- **Components**: `OAuthConfigEditor` wired into the detail view for SSE/HTTP servers.
- **Backend**: A new `POST /mcp-servers/{id}/test` (or similar) endpoint is needed on the daemon side for test-connection. This is out of scope for the macOS app change but must be coordinated.
- **Tests**: Snapshot tests for `MCPServersView` will need updating. New unit tests for inline editing state management and context toggle logic in the ViewModel.
