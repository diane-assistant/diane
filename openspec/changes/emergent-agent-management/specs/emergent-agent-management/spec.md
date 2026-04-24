## ADDED Requirements

### Requirement: List all agents for a project

The system SHALL display all agents for the current project in a master list, fetched via `GET /api/admin/agents` with `X-Project-ID` header. Each row SHALL show the agent name, trigger type (`schedule`, `manual`, `reaction`, `webhook`), execution mode (`suggest`, `execute`, `hybrid`), enabled state, and last run status.

#### Scenario: Navigating to agent management
- **WHEN** the user navigates to the Agent Management section
- **THEN** the system MUST display a MasterDetailView with a list of all agents in the master column
- **THEN** each agent row MUST show name, triggerType badge, executionMode badge, and enabled/disabled state

#### Scenario: Empty project has no agents
- **WHEN** the API returns an empty agent list
- **THEN** the master column MUST display an empty state message

#### Scenario: API error loading agents
- **WHEN** `GET /api/admin/agents` fails
- **THEN** the system MUST display an error state using Spacing.large (12) with a retry option

### Requirement: View agent detail

The system SHALL display the full detail of a selected agent in the detail pane. Fields shown SHALL include: name, description, prompt, triggerType, executionMode, strategyType, cronSchedule (for schedule agents), enabled state, capabilities (canCreateObjects, canUpdateObjects, canDeleteObjects, canCreateRelationships, allowedObjectTypes), and timestamps (createdAt, updatedAt, lastRunAt, lastRunStatus).

#### Scenario: Selecting an agent
- **WHEN** the user selects an agent from the master list
- **THEN** the detail pane MUST display all agent fields in labeled InfoRow components within DetailSection groups

#### Scenario: Capabilities section
- **WHEN** the agent has a capabilities object
- **THEN** the detail pane MUST display a "Capabilities" DetailSection showing boolean toggles and allowed object types as read-only badges

### Requirement: Update agent configuration

The system SHALL allow editing an agent's configuration via `PATCH /api/admin/agents/{id}`. Editable fields SHALL include: name, description, prompt, triggerType, executionMode, cronSchedule (for schedule trigger), and enabled state. The system SHALL use snapshot-based dirty tracking and show Save/Discard actions in edit mode.

#### Scenario: Entering edit mode
- **WHEN** the user clicks the "Edit" button in the agent detail header
- **THEN** all editable fields SHALL switch to editable controls (TextField for text, Picker for enums, Toggle for booleans)
- **THEN** Save and Discard buttons SHALL appear in the header

#### Scenario: cronSchedule only shown for schedule trigger
- **WHEN** the agent's triggerType is `schedule`
- **THEN** the edit form MUST display a cronSchedule TextField
- **WHEN** triggerType is not `schedule`
- **THEN** cronSchedule SHALL NOT be shown

#### Scenario: Saving changes
- **WHEN** the user modifies fields and clicks Save
- **THEN** the system MUST call `PATCH /api/admin/agents/{id}` with only the changed fields
- **THEN** on success, the agent list and detail view MUST update with the new values and exit edit mode

#### Scenario: Save failure
- **WHEN** `PATCH /api/admin/agents/{id}` returns an error
- **THEN** the view MUST remain in edit mode and display an inline error message

### Requirement: Delete an agent

The system SHALL allow deleting an agent via `DELETE /api/admin/agents/{id}`. A confirmation prompt SHALL be shown before deletion. On success, the agent SHALL be removed from the master list.

#### Scenario: User deletes an agent
- **WHEN** the user selects "Delete" from the agent's context menu or detail header
- **THEN** the system MUST show a confirmation dialog with the agent name
- **WHEN** the user confirms
- **THEN** the system MUST call `DELETE /api/admin/agents/{id}` and remove the agent from the list

#### Scenario: Deletion failure
- **WHEN** `DELETE /api/admin/agents/{id}` returns an error
- **THEN** the agent MUST remain in the list and an error message MUST be displayed

### Requirement: Manually trigger an agent run

The system SHALL allow manually triggering an agent execution via `POST /api/admin/agents/{id}/trigger`. After triggering, the system SHALL display the resulting run ID and navigate to the run history.

#### Scenario: Triggering a manual run
- **WHEN** the user clicks "Run Now" in the agent detail header or context menu
- **THEN** the system MUST call `POST /api/admin/agents/{id}/trigger`
- **THEN** on success, the system MUST display the new runId and refresh the run history

#### Scenario: Trigger failure
- **WHEN** the trigger API call fails
- **THEN** the system MUST display an inline error with the error message

### Requirement: View agent run history

The system SHALL display recent runs for an agent via `GET /api/admin/agents/{id}/runs`. Each run SHALL show: status badge (running/success/skipped/error/paused/cancelled), start time, duration, step count, and error message if applicable.

#### Scenario: Viewing run history tab
- **WHEN** the user selects the "Runs" tab in the agent detail pane
- **THEN** the system MUST fetch and display the last 10 runs (default limit)
- **THEN** each run row MUST show status badge, startedAt, durationMs (formatted), and stepCount

#### Scenario: Running run shows session status
- **WHEN** a run has status `running`
- **THEN** the row MUST also show the sessionStatus (provisioning/active) as a secondary badge

#### Scenario: Error run shows error message
- **WHEN** a run has status `error`
- **THEN** the run row or detail expansion MUST show the errorMessage field

#### Scenario: Multi-agent run shows parent link
- **WHEN** a run has a non-empty `parentRunId`
- **THEN** the run detail MUST indicate it is a child of a parent run

### Requirement: Cancel a running agent run

The system SHALL allow cancelling an in-progress run via `POST /api/admin/agents/{id}/runs/{runId}/cancel`. The cancel action SHALL only be available for runs with status `running` or `paused`.

#### Scenario: Cancelling a run
- **WHEN** the user clicks "Cancel" on a running or paused run
- **THEN** the system MUST call `POST /api/admin/agents/{id}/runs/{runId}/cancel`
- **THEN** on success, the run's status MUST update to `cancelled` in the list

#### Scenario: Cancel not available for terminal runs
- **WHEN** a run has status `success`, `error`, `skipped`, or `cancelled`
- **THEN** no cancel action SHALL be shown for that run
