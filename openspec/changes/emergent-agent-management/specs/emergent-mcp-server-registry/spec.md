## ADDED Requirements

### Requirement: List registered MCP servers for a project

The system SHALL display all MCP servers registered with the Emergent backend for the current project via `GET /api/admin/mcp-servers`. Each row SHALL show: server name, type badge (stdio/sse/http), enabled state, and tool count.

#### Scenario: Navigating to MCP server registry
- **WHEN** the user opens the "Emergent MCP Servers" section
- **THEN** the system MUST fetch and display all registered servers using a MasterDetailView
- **THEN** each row MUST display server name, type badge, enabled state, and toolCount

#### Scenario: Empty registry
- **WHEN** no MCP servers are registered
- **THEN** the master list MUST display an empty state with an "Add Server" affordance

#### Scenario: Load failure
- **WHEN** `GET /api/admin/mcp-servers` fails
- **THEN** the system MUST display an error state with a retry option

### Requirement: View MCP server detail with tool list

The system SHALL display the full configuration and registered tools for a selected MCP server via `GET /api/admin/mcp-servers/{id}`. The detail view SHALL show all configuration fields and the tool list (MCPServerDetailDTO includes a `tools` array).

#### Scenario: Selecting a server
- **WHEN** the user selects an MCP server from the list
- **THEN** the detail pane MUST show: name, type, url (for sse/http), command (for stdio), args (for stdio), env (key-value), headers (key-value), enabled state, toolCount, createdAt, updatedAt

#### Scenario: Tool list section
- **WHEN** the detail is loaded with tools present
- **THEN** a "Tools" DetailSection MUST show each tool with toolName, description, and enabled badge

### Requirement: Register a new MCP server

The system SHALL allow registering a new MCP server via `POST /api/admin/mcp-servers` using a sheet form. The form SHALL capture all fields from `CreateMCPServerDTO`: name, type, command (stdio only), args (stdio only), env (all types), url (sse/http only), headers (sse/http only), enabled toggle.

#### Scenario: Opening the create form
- **WHEN** the user clicks "Add Server" in the header
- **THEN** a sheet MUST open with a form to configure the new server

#### Scenario: Type-conditional fields
- **WHEN** the user selects type `stdio`
- **THEN** the form MUST show command (TextField, required) and args (StringArrayEditor)
- **WHEN** the user selects type `sse` or `http`
- **THEN** the form MUST show url (TextField, required) and headers (KeyValueEditor)
- **THEN** command and args MUST NOT be shown for non-stdio types

#### Scenario: Successful registration
- **WHEN** the user submits valid data and `POST /api/admin/mcp-servers` succeeds
- **THEN** the new server MUST appear in the master list and the sheet MUST close

#### Scenario: Validation prevents submission
- **WHEN** the name is empty, or a required type-specific field (command for stdio, url for sse/http) is empty
- **THEN** the submit button MUST be disabled

#### Scenario: Registration failure
- **WHEN** the API returns an error (e.g. duplicate name, invalid config)
- **THEN** an inline error MUST be shown in the form sheet and the sheet MUST remain open

### Requirement: Update an MCP server configuration

The system SHALL allow updating a registered MCP server via `PATCH /api/admin/mcp-servers/{id}`. Editable fields match `UpdateMCPServerDTO`: name, command (stdio), args (stdio), env, url (sse/http), headers (sse/http), enabled. The detail view SHALL use inline edit mode (edit/save/discard) with snapshot-based dirty tracking.

#### Scenario: Entering edit mode
- **WHEN** the user clicks "Edit" in the server detail header
- **THEN** all configuration fields become editable controls
- **THEN** Save and Discard buttons appear

#### Scenario: Saving changes
- **WHEN** the user modifies fields and clicks Save
- **THEN** the system MUST call `PATCH /api/admin/mcp-servers/{id}` with the updated values
- **THEN** on success the detail view MUST exit edit mode and show the updated values

#### Scenario: Save failure
- **WHEN** `PATCH` returns an error
- **THEN** the detail view MUST remain in edit mode and show an inline error

### Requirement: Delete a registered MCP server

The system SHALL allow deleting a registered MCP server via `DELETE /api/admin/mcp-servers/{id}`. A confirmation prompt SHALL be shown. On success, the server and all its tools are removed from the list.

#### Scenario: Deleting a server
- **WHEN** the user selects "Delete" from the server context menu
- **THEN** a confirmation dialog MUST appear with the server name
- **WHEN** confirmed, the system MUST call `DELETE /api/admin/mcp-servers/{id}` (expects 204)
- **THEN** the server MUST be removed from the master list

#### Scenario: Deletion failure
- **WHEN** the API returns an error
- **THEN** the server MUST remain in the list and an error message MUST be shown
