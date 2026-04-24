## MODIFIED Requirements

### Requirement: Agent workspace configuration form section

The custom agent configuration form SHALL include a "Workspace" section for configuring the agent's execution environment, using `AgentWorkspaceConfig`. This section SHALL be fetched via `GET /api/admin/agent-definitions/{id}/workspace-config` when the agent definition is loaded and persisted via `PUT /api/admin/agent-definitions/{id}/workspace-config` on save.

The `AgentWorkspaceConfig` fields exposed in the form SHALL be:
- `enabled` (Toggle) — enable/disable the workspace for this agent
- `base_image` (TextField) — base Docker image reference; populated from available workspace images
- `provider` (Picker: `""` auto, `firecracker`, `gvisor`, `e2b`) — explicit sandbox provider; empty means auto
- `repo_source.type` (Picker: `none`, `task_context`, `fixed`) — repository source strategy
- `repo_source.url` (TextField, shown only when type is `fixed`) — git repository URL
- `repo_source.branch` (TextField, shown only when type is `fixed` or `task_context`) — default branch
- `resource_limits.cpu` (TextField, e.g. `"2"`) — CPU cores
- `resource_limits.memory` (TextField, e.g. `"4G"`) — memory limit
- `resource_limits.disk` (TextField, e.g. `"10G"`) — disk limit
- `setup_commands` (StringArrayEditor) — shell commands run after workspace startup
- `tools` (StringArrayEditor) — tool names available to the agent in this workspace
- `checkout_on_start` (Toggle) — whether to checkout the repository on workspace start

#### Scenario: Loading existing workspace configuration
- **WHEN** the user opens the custom agent configuration form for an existing agent definition
- **THEN** the system MUST fetch the workspace config via `GET /api/admin/agent-definitions/{id}/workspace-config`
- **THEN** all workspace fields MUST be pre-populated with the returned values

#### Scenario: Workspace section collapsed by default
- **WHEN** the form is opened and `enabled` is false
- **THEN** the Workspace section MUST appear collapsed (showing only the `enabled` toggle)
- **WHEN** the user enables the workspace toggle
- **THEN** the full workspace configuration fields MUST expand and become editable

#### Scenario: Provider picker shows auto option
- **WHEN** the provider field is empty string `""`
- **THEN** the Picker MUST display "Auto (default)" as the selected option
- **WHEN** the user selects a specific provider (firecracker/gvisor/e2b)
- **THEN** the picker MUST display the selected provider name

#### Scenario: Repo source fields are conditional
- **WHEN** `repo_source.type` is `none`
- **THEN** url and branch fields MUST NOT be shown
- **WHEN** `repo_source.type` is `task_context`
- **THEN** only the branch TextField MUST be shown (url is derived from task context)
- **WHEN** `repo_source.type` is `fixed`
- **THEN** both url (required) and branch TextFields MUST be shown

#### Scenario: Base image picker integrates with workspace images
- **WHEN** the user focuses the base_image TextField
- **THEN** the system MUST show a picker or autocomplete populated from `GET /api/admin/workspace-images`, showing names of images with status "ready"

#### Scenario: Saving workspace configuration
- **WHEN** the user clicks Save on the custom agent form
- **THEN** the system MUST call `PUT /api/admin/agent-definitions/{id}/workspace-config` with the complete `AgentWorkspaceConfig` struct
- **THEN** on success, the workspace config section MUST reflect the saved values

#### Scenario: Save failure for workspace config
- **WHEN** `PUT /api/admin/agent-definitions/{id}/workspace-config` returns an error
- **THEN** the form MUST remain open and display an inline error in the Workspace section
- **THEN** the other agent fields (name, prompt, etc.) MUST NOT be affected

#### Scenario: Workspace config not required when disabled
- **WHEN** `enabled` is false
- **THEN** the system MUST NOT call the workspace config endpoint on save
- **THEN** the PUT body MUST include `{ "enabled": false }` to explicitly disable the workspace
