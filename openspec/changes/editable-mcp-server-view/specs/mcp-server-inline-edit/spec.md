## ADDED Requirements

### Requirement: Detail view supports edit mode for non-builtin servers

The MCP server detail view SHALL provide an "Edit" button in the header that toggles the view between read mode and edit mode. The edit button SHALL NOT appear for built-in servers. In read mode, the view SHALL display all configuration fields as static text (current behavior). In edit mode, the view SHALL replace static fields with editable controls appropriate to the field type.

#### Scenario: Entering edit mode
- **WHEN** a non-builtin server is selected and the user clicks the "Edit" button in the detail header
- **THEN** the detail view enters edit mode, replacing read-only fields with editable controls, and showing "Save" and "Discard" buttons in the header

#### Scenario: Edit button hidden for built-in servers
- **WHEN** a built-in server is selected
- **THEN** the detail header SHALL NOT display an "Edit" button, and all fields SHALL remain read-only

#### Scenario: Exiting edit mode via Discard
- **WHEN** the user clicks the "Discard" button while in edit mode
- **THEN** all edited fields SHALL revert to their values at the time edit mode was entered, and the view SHALL return to read mode

### Requirement: Editable configuration fields match server type

In edit mode, the detail view SHALL display editable controls for all configuration fields relevant to the server's type. For STDIO servers: name (TextField), command (TextField), arguments (StringArrayEditor), and environment variables (KeyValueEditor). For SSE/HTTP servers: name (TextField), URL (TextField), headers (KeyValueEditor), and OAuth configuration (OAuthConfigEditor). The server type itself SHALL NOT be editable after creation.

#### Scenario: STDIO server edit fields
- **WHEN** a STDIO server is in edit mode
- **THEN** the view SHALL display editable TextField for name and command, a StringArrayEditor for arguments, and a KeyValueEditor for environment variables

#### Scenario: SSE server edit fields
- **WHEN** an SSE or HTTP server is in edit mode
- **THEN** the view SHALL display editable TextField for name and URL, a KeyValueEditor for headers, and an OAuthConfigEditor for OAuth configuration

#### Scenario: Server type is not editable
- **WHEN** any server is in edit mode
- **THEN** the server type SHALL be displayed as a read-only label, not as an editable picker

### Requirement: Save persists configuration changes

The "Save" button SHALL be enabled only when at least one configuration field differs from the values when edit mode was entered (dirty-state detection via snapshot comparison). Clicking "Save" SHALL call the update API to persist changes and exit edit mode on success.

#### Scenario: Save enabled when fields changed
- **WHEN** the user modifies at least one configuration field while in edit mode
- **THEN** the "Save" button SHALL become enabled

#### Scenario: Save disabled when no changes
- **WHEN** the user enters edit mode but makes no changes, or reverts all changes to their original values
- **THEN** the "Save" button SHALL remain disabled

#### Scenario: Successful save
- **WHEN** the user clicks "Save" and the API call succeeds
- **THEN** the server's configuration SHALL be updated in the local list, the detail view SHALL exit edit mode, and the updated values SHALL be reflected in read mode

#### Scenario: Save failure
- **WHEN** the user clicks "Save" and the API call fails
- **THEN** the view SHALL remain in edit mode, the edited values SHALL be preserved, and an error message SHALL be displayed inline

### Requirement: Form validation prevents invalid saves

The "Save" button SHALL be disabled when required fields are empty. For STDIO servers, name and command are required. For SSE/HTTP servers, name and URL are required. This SHALL use the same validation logic as the create form (`isServerFormValid`).

#### Scenario: Missing required STDIO fields
- **WHEN** a STDIO server is in edit mode and either the name or command field is empty
- **THEN** the "Save" button SHALL be disabled

#### Scenario: Missing required HTTP fields
- **WHEN** an SSE/HTTP server is in edit mode and either the name or URL field is empty
- **THEN** the "Save" button SHALL be disabled

### Requirement: Context assignment via checkboxes with immediate apply

The detail view SHALL display a "Contexts" section showing all available contexts as toggle rows. Each toggle SHALL indicate whether the current server is assigned to that context. Toggling a context SHALL immediately call the add/remove API without requiring a separate "Save" action.

#### Scenario: Viewing context assignment
- **WHEN** a non-builtin server is selected (in either read or edit mode)
- **THEN** the "Contexts" section SHALL display one toggle row per available context, with each toggle ON if the server is assigned to that context

#### Scenario: Adding server to a context
- **WHEN** the user toggles a context checkbox from OFF to ON
- **THEN** the system SHALL immediately call the add-server-to-context API, and the toggle SHALL reflect the new state

#### Scenario: Removing server from a context
- **WHEN** the user toggles a context checkbox from ON to OFF
- **THEN** the system SHALL immediately call the remove-server-from-context API, and the toggle SHALL reflect the new state

#### Scenario: Context toggle failure with rollback
- **WHEN** a context toggle API call fails
- **THEN** the toggle SHALL revert to its previous state (optimistic update rollback) and an inline error message SHALL be displayed

#### Scenario: Built-in servers show read-only context badges
- **WHEN** a built-in server is selected
- **THEN** the "Contexts" section SHALL display context names as read-only badges (current behavior), not editable checkboxes

### Requirement: Edit sheet removed for editing, retained for creation

The modal edit sheet (`MCPServerFormSheet`) SHALL no longer be triggered for editing existing servers. The "Edit" action in the ellipsis menu SHALL be removed. The sheet SHALL remain available for the "Add Server" (create) flow.

#### Scenario: Ellipsis menu has no Edit option
- **WHEN** the user opens the ellipsis menu for a non-builtin server
- **THEN** the menu SHALL contain "Duplicate" and "Delete" actions but SHALL NOT contain an "Edit" action

#### Scenario: Create flow unchanged
- **WHEN** the user clicks "Add Server" in the header
- **THEN** the `MCPServerFormSheet` SHALL appear as a modal sheet for creating a new server, with all existing create-flow behavior intact
