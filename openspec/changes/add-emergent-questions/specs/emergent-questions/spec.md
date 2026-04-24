## ADDED Requirements

### Requirement: Questions service layer for the Emergent backend

The system SHALL provide a dedicated HTTP client service in `server/internal/emergent/` that communicates directly with the Emergent REST API to fetch and answer agent questions. The service SHALL use the existing `Config` (base URL + API key) and expose typed Go functions for the three question endpoints: list project questions, list run questions, and respond to a question.

#### Scenario: Listing pending questions for a project
- **WHEN** the questions service is called to list questions with status `pending`
- **THEN** it calls `GET /api/projects/{projectId}/agent-questions?status=pending` on the Emergent backend
- **AND** returns a slice of `AgentQuestion` values with fields: `id`, `runId`, `agentId`, `question`, `options` (label/value/description), `status`, and `createdAt`

#### Scenario: Responding to a question
- **WHEN** the questions service is called with a question ID and a response string
- **THEN** it calls `POST /api/projects/{projectId}/agent-questions/{questionId}/respond` with body `{"response": "<text>"}`
- **AND** returns successfully on HTTP 202, indicating the agent run has been resumed
- **AND** returns an error if the question is already answered (HTTP 409) or not found (HTTP 404)

#### Scenario: Emergent backend unreachable
- **WHEN** the Emergent backend is not reachable
- **THEN** the service returns an error that callers can surface to the user

---

### Requirement: Daemon API endpoints for questions

The daemon HTTP server (`server/internal/api/api.go`) SHALL expose two endpoints that proxy to the questions service layer, making questions accessible to both the CLI and the GUI via the local Unix socket.

#### Scenario: Listing pending questions via daemon API
- **WHEN** a client sends `GET /questions?status=pending` to the daemon
- **THEN** the daemon calls the questions service and returns the list as JSON
- **AND** if Emergent is not configured, the daemon returns HTTP 503 with a descriptive error

#### Scenario: Answering a question via daemon API
- **WHEN** a client sends `POST /questions/{id}/respond` with body `{"response": "<text>"}` to the daemon
- **THEN** the daemon calls the questions service and returns HTTP 202 on success
- **AND** forwards 404 and 409 errors from the backend with appropriate messages

---

### Requirement: CLI commands for listing and answering questions

The CLI (`diane-ctl`) SHALL provide two subcommands under a `questions` group to allow users to interact with pending agent questions from the terminal.

#### Scenario: Listing pending questions
- **WHEN** the user runs `diane-ctl questions list`
- **THEN** the CLI prints a formatted table of pending questions showing: question ID (truncated), agent name, the question text, and any options (label: value)
- **AND** if there are no pending questions, it prints a message indicating none are pending

#### Scenario: Answering a question with options
- **WHEN** the user runs `diane-ctl questions answer <id>` and the question has predefined options
- **THEN** the CLI displays the question text and numbered options, and prompts the user to select one
- **AND** submits the selected option's value as the response

#### Scenario: Answering a free-text question
- **WHEN** the user runs `diane-ctl questions answer <id>` and the question has no predefined options
- **THEN** the CLI displays the question text and prompts the user to type a free-text response
- **AND** submits the entered text as the response

#### Scenario: Non-interactive answer via flag
- **WHEN** the user runs `diane-ctl questions answer <id> --response "my answer"`
- **THEN** the CLI submits the provided response directly without interactive prompting

---

### Requirement: GUI questions panel displaying pending questions

The macOS SwiftUI app SHALL include a questions panel that surfaces pending Emergent agent questions so the user can see and respond to them without leaving the app.

#### Scenario: Pending questions badge visible
- **WHEN** there is at least one pending question
- **THEN** the app SHALL display a badge or indicator (e.g., on the sidebar or toolbar) showing the count of pending questions

#### Scenario: Questions panel shows list of pending questions
- **WHEN** the user opens the questions panel
- **THEN** the panel SHALL display each pending question as a row showing the question text and the associated agent name
- **AND** questions SHALL be sorted by creation time, oldest first

#### Scenario: No pending questions state
- **WHEN** the questions panel is open and there are no pending questions
- **THEN** the panel SHALL display an empty state message (e.g., "No pending questions")

---

### Requirement: GUI inline answer form

The macOS SwiftUI app SHALL allow the user to answer a pending question directly from the questions panel, without navigating away.

#### Scenario: Answering a question with predefined options
- **WHEN** the user selects a pending question that has predefined options
- **THEN** the detail view SHALL display the question text and the options as selectable controls (e.g., radio buttons or a picker)
- **AND** a "Submit" button SHALL become enabled when an option is selected

#### Scenario: Answering a free-text question
- **WHEN** the user selects a pending question that has no predefined options
- **THEN** the detail view SHALL display the question text and a text field for free-text input
- **AND** a "Submit" button SHALL become enabled when the text field is non-empty

#### Scenario: Successful submission
- **WHEN** the user clicks "Submit" and the API call succeeds
- **THEN** the question SHALL be removed from the pending list
- **AND** the panel SHALL update to reflect the new pending question count

#### Scenario: Submission failure
- **WHEN** the user clicks "Submit" and the API call fails
- **THEN** an inline error message SHALL be displayed below the question
- **AND** the user's entered response SHALL be preserved so they can retry
