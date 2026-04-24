## MODIFIED Requirements

### Requirement: Trigger and cancel actions in the monitoring view

The agent monitoring detail view SHALL expose "Run Now" and "Cancel" actions directly from the live agent view, without requiring navigation to a separate management view. "Run Now" SHALL call `POST /api/admin/agents/{id}/trigger`. "Cancel" SHALL be available only for agents with a run in `running` or `paused` status, calling `POST /api/admin/agents/{id}/runs/{runId}/cancel` for the most recent active run.

#### Scenario: Run Now available in monitoring header
- **WHEN** the user is viewing a monitored agent's detail
- **THEN** a "Run Now" button MUST appear in the detail header
- **WHEN** clicked, the system MUST call `POST /api/admin/agents/{id}/trigger` and display the resulting runId as feedback

#### Scenario: Cancel available for active runs
- **WHEN** the monitored agent has a run with status `running` or `paused`
- **THEN** a "Cancel Run" button MUST appear in the detail header or run section
- **WHEN** clicked, the system MUST call `POST /api/admin/agents/{id}/runs/{runId}/cancel`
- **THEN** the run status MUST update to `cancelled` in the live view

#### Scenario: Cancel not available when no active run
- **WHEN** the agent has no run with status `running` or `paused`
- **THEN** the "Cancel Run" button MUST NOT be shown

### Requirement: Run history tab in the monitoring detail view

The agent monitoring detail view SHALL add a "Runs" tab showing the recent run history (via `GET /api/admin/agents/{id}/runs`), complementing the existing live log view.

#### Scenario: Switching to the Runs tab
- **WHEN** the user clicks the "Runs" tab in the agent detail pane
- **THEN** the system MUST fetch and display up to 10 recent runs
- **THEN** each run row MUST show: status badge, startedAt (relative time), durationMs (formatted), stepCount, and errorMessage if status is `error`

#### Scenario: Run history auto-refreshes when agent is active
- **WHEN** the agent has a run in `running` status
- **THEN** the runs list MUST refresh periodically (matching the existing log refresh interval)

#### Scenario: Navigating from run history to agent management
- **WHEN** the user clicks on a run row
- **THEN** the system MUST navigate to or highlight that run in the Agent Management view for full detail
