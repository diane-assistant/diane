## 1. ViewModel: Edit Mode and Dirty-State Tracking

- [ ] 1.1 Add `isInEditMode: Bool` property to `MCPRegistryViewModel` (default `false`)
- [ ] 1.2 Add `editSnapshot` stored property (capture `MCPServer` values when entering edit mode) to `MCPRegistryViewModel`
- [ ] 1.3 Add computed `hasChanges: Bool` that compares `editName`, `editCommand`, `editArgs`, `editEnv`, `editURL`, `editHeaders`, `editOAuth` against `editSnapshot` values
- [ ] 1.4 Add `enterEditMode(for server: MCPServer)` method that sets `isInEditMode = true`, calls `populateEditFields(from:)`, and captures `editSnapshot`
- [ ] 1.5 Add `discardInlineEdit()` method that restores edit fields from `editSnapshot` and sets `isInEditMode = false`
- [ ] 1.6 Add `saveInlineEdit()` async method that calls existing `updateServer()` logic inline and sets `isInEditMode = false` on success (keeps edit mode on failure)

## 2. ViewModel: Context Toggle

- [ ] 2.1 Add `toggleContextForServer(_ server: MCPServer, contextName: String, add: Bool)` async method to `MCPRegistryViewModel`
- [ ] 2.2 Implement optimistic local update in `contextServers` dictionary before the API call
- [ ] 2.3 Call `client.addServerToContext` or `client.removeServerFromContext` based on `add` parameter
- [ ] 2.4 Implement rollback on API failure: revert `contextServers` to previous state and set `error` message
- [ ] 2.5 Add `isServerInContext(_ server: MCPServer, contextName: String) -> Bool` helper method

## 3. ViewModel: Test Connection

- [ ] 3.1 Add `isTesting: Bool` and `lastTestServerName: String?` properties to `MCPRegistryViewModel`
- [ ] 3.2 Add `testConnection(for server: MCPServer)` async method that sets `isTesting = true`, calls `loadData()` to refresh, then sets `isTesting = false` and records `lastTestServerName`
- [ ] 3.3 Clear `lastTestServerName` when `selectedServer` changes (add logic in the view or via a `didSet`-equivalent)

## 4. View: Detail Header with Edit/Save/Discard Buttons

- [ ] 4.1 In `MCPRegistryView`, replace the `serverDetailView(_:)` header section: add an "Edit" button (pencil icon) for non-builtin servers that calls `viewModel.enterEditMode(for:)`
- [ ] 4.2 When `viewModel.isInEditMode` is `true`, replace the "Edit" button with "Save" and "Discard" buttons in the header
- [ ] 4.3 Disable the "Save" button when `!viewModel.hasChanges` or `!viewModel.canSaveEdit`
- [ ] 4.4 Wire "Save" to `viewModel.saveInlineEdit()` and "Discard" to `viewModel.discardInlineEdit()`

## 5. View: Inline Editable Configuration Fields

- [ ] 5.1 In `serverDetailView(_:)`, conditionally render read-only (`InfoRow`, static text) vs editable controls based on `viewModel.isInEditMode`
- [ ] 5.2 For read mode: keep existing `InfoRow` and static text display (current behavior)
- [ ] 5.3 For STDIO edit mode: render `TextField` for name, `TextField` for command, `StringArrayEditor` for `$viewModel.editArgs`, `KeyValueEditor` for `$viewModel.editEnv`
- [ ] 5.4 For SSE/HTTP edit mode: render `TextField` for name, `TextField` for URL, `KeyValueEditor` for `$viewModel.editHeaders`, `OAuthConfigEditor` for `$viewModel.editOAuth`
- [ ] 5.5 Display server type as a read-only label (not a picker) in edit mode
- [ ] 5.6 Show inline error message from `viewModel.editError` when save fails

## 6. View: Context Assignment Checkboxes

- [ ] 6.1 Replace the read-only "Available in Contexts" section with a "Contexts" `DetailSection` for non-builtin servers
- [ ] 6.2 Render one `Toggle` row per context from `viewModel.contexts`, using `viewModel.isServerInContext()` for the toggle state
- [ ] 6.3 Wire each toggle's onChange to call `viewModel.toggleContextForServer(_:contextName:add:)`
- [ ] 6.4 Keep the existing read-only badge display for built-in servers

## 7. View: Test Connection Button

- [ ] 7.1 Add a "Test Connection" button in the Runtime Status `DetailSection` for non-builtin servers
- [ ] 7.2 Show a `ProgressView` spinner on the button while `viewModel.isTesting` is `true`, and disable the button
- [ ] 7.3 After test completes, display the runtime status (connected/disconnected, tool counts, error) from `statusMonitor` — the existing Runtime Status section handles this already

## 8. View: Remove Edit Sheet for Editing

- [ ] 8.1 Remove the `.sheet(item: $viewModel.editingServer)` binding from `MCPRegistryView.body`
- [ ] 8.2 Remove the `editServerSheet(for:)` method from `MCPRegistryView`
- [ ] 8.3 Remove the "Edit" menu item from the ellipsis `Menu` in the detail header (keep Duplicate and Delete)
- [ ] 8.4 Verify the create flow (`showCreateServer` sheet with `MCPServerFormSheet`) still works unchanged

## 9. Testing

- [ ] 9.1 Add unit tests for `hasChanges` computed property: verify it returns `false` when fields match snapshot, `true` when any field differs
- [ ] 9.2 Add unit tests for `enterEditMode` / `discardInlineEdit` / `saveInlineEdit` state transitions
- [ ] 9.3 Add unit tests for `toggleContextForServer` with success and failure (rollback) cases
- [ ] 9.4 Add unit tests for `testConnection` state management (`isTesting` flag, `lastTestServerName`)
- [ ] 9.5 Update existing snapshot tests if they reference the old detail view layout
