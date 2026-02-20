## Context

The MCP server detail view in `MCPRegistryView.swift` (lines 303-557) is read-only. Server configuration lives in `serverDetailView(_:)` using `InfoRow` and static text. Editing requires opening a modal `MCPServerFormSheet` (lines 648-787), and context assignment requires switching to the Contexts tab (Cmd+2). The ViewModel already has full edit state (`editName`, `editCommand`, etc.) and an `updateServer()` method that calls `client.updateMCPServerConfig()`. Context APIs (`addServerToContext`, `removeServerFromContext`) exist in `DianeClient` but aren't exposed from `MCPRegistryViewModel`.

## Goals / Non-Goals

**Goals:**
- Convert the detail pane from read-only to inline-editable for non-builtin servers
- Add context assignment checkboxes directly in the detail view
- Wire `OAuthConfigEditor` into the detail view for SSE/HTTP servers
- Add a "Test Connection" button following the `testAgent`/`testProvider` pattern
- Provide Save/Discard actions with dirty-state tracking
- Keep the `MCPServerFormSheet` for the create flow only

**Non-Goals:**
- Changing the master list (left pane) layout or behavior
- Implementing the daemon-side `POST /mcp-servers/{id}/test` endpoint (backend work, out of scope)
- Multi-node deployment editing (the "Deployed on Nodes" section stays read-only)
- Modifying the iOS `MCPServerDetailView` (read-only HTTP client, separate concern)
- Per-tool enable/disable within contexts (stay in the Contexts tab for that granularity)

## Decisions

### 1. Inline editing via edit-mode toggle, not always-editable fields

**Choice**: The detail view starts in read mode. An "Edit" button in the header enters edit mode, which swaps `InfoRow`/static text for `TextField`/`StringArrayEditor`/`KeyValueEditor`. Save or Discard exits edit mode.

**Why not always-editable**: Always-editable fields feel unstable — accidental edits risk data loss and there's no visual signal of what changed. An explicit edit mode gives the user a clear "I'm making changes" state, matches macOS conventions (Xcode inspectors, System Settings), and makes dirty-state tracking simpler (compare snapshot-on-enter vs current).

**Why not keep the modal sheet**: The sheet duplicates the detail view's information, forces context-switching, and can't show runtime status alongside the fields being edited. Inline editing keeps everything visible.

### 2. Dirty-state tracking via snapshot comparison

**Choice**: When entering edit mode, capture a snapshot of the server's current values. The "Save" button is enabled only when edit fields differ from the snapshot. "Discard" restores the snapshot.

**Implementation**: Reuse the existing `editName`, `editCommand`, `editArgs`, `editEnv`, `editURL`, `editHeaders`, `editOAuth` properties in `MCPRegistryViewModel`. Add a new `editSnapshot` stored property (a lightweight struct or the `MCPServer` itself) captured via `populateEditFields(from:)`. Add a computed `hasChanges: Bool` that compares current edit fields against the snapshot. Context checkbox changes are tracked separately since they hit different API endpoints.

**Why over onChange-based tracking**: Snapshot comparison is pure and testable. No need to intercept every field mutation.

### 3. Context assignment via immediate-apply checkboxes (no Save required)

**Choice**: Context checkboxes apply immediately on toggle — they don't wait for "Save". Each toggle calls `addServerToContext` or `removeServerFromContext` directly.

**Why immediate**: Context assignment is a relationship change, not a config change. It maps to a separate API endpoint and doesn't interact with the server's own configuration. Making it Save-dependent would mean bundling unrelated operations into one transaction, complicating error handling. The Contexts tab already uses immediate toggles for enable/disable — this keeps behavior consistent.

**UI**: A `DetailSection` titled "Contexts" showing a `VStack` of `Toggle` rows, one per context. Each shows the context name and a checkbox-style toggle. The toggle is bound to whether `contextServers[contextName]` contains the current server.

### 4. Test Connection using existing runtime status refresh

**Choice**: Since no `POST /mcp-servers/{id}/test` endpoint exists yet, the "Test Connection" button triggers a full `loadData()` refresh and then checks the `statusMonitor` for the server's `connected` status. The result is displayed inline as a success/failure banner that auto-dismisses.

**Why not block on a missing endpoint**: The daemon already tracks connection status via `getMCPServers()` (which feeds `StatusMonitor`). A refresh gives equivalent feedback. When the dedicated test endpoint is added later, swapping in `client.testMCPServer(id:)` is a one-line change in the ViewModel.

**Future**: When `POST /mcp-servers-config/{id}/test` is added, create `MCPServerTestResult` model and `testMCPServer(id:)` on `DianeClientProtocol`, following the `testAgent`/`testProvider` pattern exactly (POST, decode result, store in `testResults` dictionary).

### 5. Built-in servers remain non-editable

**Choice**: If `server.isBuiltin`, the detail view stays fully read-only — no edit button, no context checkboxes. Built-in servers are managed by Diane and cannot be reconfigured.

**Why**: Built-in servers have no user-configurable fields. Showing disabled form fields adds clutter. The existing "Managed by Diane" label is sufficient.

### 6. Remove the Edit sheet trigger, keep the sheet component

**Choice**: Remove the `.sheet(item: $viewModel.editingServer)` binding and the "Edit" menu item that triggers it. Keep `MCPServerFormSheet` as a component — it's still used for the Create flow.

**Why not delete the component**: `MCPServerFormSheet` is used by `createServerSheet`. Only the edit-mode usage is being replaced.

### 7. File change scope

| File | Change |
|------|--------|
| `MCPRegistryView.swift` | Replace `serverDetailView(_:)` with editable version. Remove edit sheet binding and `editServerSheet(for:)`. Add edit-mode state (`isEditMode`) to the view. |
| `MCPRegistryViewModel.swift` | Add `isInEditMode`, `hasChanges`, `editSnapshot`. Add `toggleContextForServer(_:contextName:)`, `saveInlineEdit()`, `discardInlineEdit()`, `testConnection()`. Add `testResult` and `isTesting` state. |
| `DianeClientProtocol.swift` | _(future)_ Add `testMCPServer(id:)` when endpoint exists. No changes needed now. |

## Risks / Trade-offs

- **[Risk] Context toggle failures leave inconsistent state** → Mitigation: Use optimistic local update with rollback on error, same pattern as `toggleServer()`. Show inline error text on failure.
- **[Risk] Save fails mid-flight after some context toggles already applied** → Mitigation: Context toggles are independent (immediate-apply), so config Save failure doesn't affect them. Only server config changes need rollback.
- **[Risk] Test Connection gives stale results without a dedicated endpoint** → Mitigation: Acceptable for now — `StatusMonitor` polls regularly. The UI can show "Last checked: X seconds ago" to set expectations. Future endpoint will replace this.
- **[Trade-off] Edit mode adds UI complexity vs. always-editable simplicity** → Acceptable: Edit mode is the standard macOS pattern and prevents accidental modifications. The implementation reuses existing ViewModel edit state.
