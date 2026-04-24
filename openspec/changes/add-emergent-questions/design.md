## Context

Emergent agents can pause mid-run and ask the user a question before proceeding. These questions are surfaced via three Emergent REST endpoints (`GET /api/projects/{projectId}/agent-questions`, `GET /api/projects/{projectId}/agent-runs/{runId}/questions`, and `POST /api/projects/{projectId}/agent-questions/{questionId}/respond`). Currently Diane has no awareness of these questions, so paused agent runs silently stall. This design adds a thin integration layer—service, daemon proxy, CLI commands, and SwiftUI panel—that lets users see and answer pending questions from both the terminal and the macOS app.

## Goals / Non-Goals

**Goals:**
- Expose a typed Go service that calls the Emergent questions REST endpoints directly (not via the graph SDK, which doesn't cover these endpoints).
- Proxy those calls through the daemon's Unix-socket API so CLI and GUI use a single access point.
- Add `diane-ctl questions list` and `diane-ctl questions answer` CLI commands.
- Add a SwiftUI questions panel with a list view and inline answer form.

**Non-Goals:**
- Real-time push / WebSocket notification of new questions (polling is sufficient for v1).
- Answering questions on behalf of a specific run (project-level listing covers all pending questions).
- Modifying the Emergent backend or the SDK graph client.

## Decisions

### 1. Project ID resolution via `/api/auth/me`

The questions endpoints require a `projectId` path parameter, but the existing `Config` struct only stores `baseURL` and `apiKey`. The Emergent REST API exposes `GET /api/auth/me` which returns `project_id` (among other fields) given a valid API key. The questions service will call this endpoint once at startup and cache the result in `Config`, reusing the same `net/http` client used for all question calls.

*Rationale:* Avoids requiring the user to manually configure a project ID and keeps the config file minimal. The `project_id` field already exists in `configFile` as a passthrough for users who want to hard-code it; we will respect that value if present and fall back to `/api/auth/me` otherwise.

### 2. Dedicated `questions.go` file in `server/internal/emergent/`

Rather than extending the SDK client (which uses the graph API and has no questions support), we add `server/internal/emergent/questions.go` with a plain `net/http`-based `QuestionsService` struct. It holds a `*http.Client`, the base URL, the API key, and the resolved project ID.

```
type AgentQuestion struct {
    ID         string                `json:"id"`
    RunID      string                `json:"runId"`
    AgentID    string                `json:"agentId"`
    Question   string                `json:"question"`
    Options    []AgentQuestionOption `json:"options"`
    Status     string                `json:"status"`
    CreatedAt  time.Time             `json:"createdAt"`
}

type AgentQuestionOption struct {
    Label       string `json:"label"`
    Value       string `json:"value"`
    Description string `json:"description"`
}

type QuestionsService struct { ... }

func NewQuestionsService(cfg *Config) (*QuestionsService, error)
func (s *QuestionsService) ListQuestions(ctx context.Context, status string) ([]AgentQuestion, error)
func (s *QuestionsService) RespondToQuestion(ctx context.Context, questionID, response string) error
```

*Rationale:* Keeps questions logic self-contained and testable without mocking the graph SDK. The SDK client singleton (`GetClient()`) remains unchanged.

### 3. Daemon proxy: two new routes registered alongside existing handlers

Two routes are registered in `Start()` in `api.go` using the existing `mux.HandleFunc` pattern:

- `GET /questions` — accepts optional `?status=` query param (defaults to `pending`), returns JSON array of `AgentQuestion`.
- `POST /questions/{id}/respond` — accepts `{"response": "..."}` body, returns 202 on success, forwards 404/409 from upstream.

A `QuestionsService` is instantiated at server startup (alongside the existing store builders) and stored on the `Server` struct. If Emergent is not configured, both handlers return 503.

*Rationale:* Consistent with how other Emergent-backed features (agents, providers) are plumbed through the daemon. The CLI and GUI both talk to the daemon over the Unix socket, avoiding direct Emergent API calls from Swift.

### 4. Daemon API client additions

Two methods are added to `server/internal/api/client.go`:

```go
func (c *Client) ListQuestions(status string) ([]emergent.AgentQuestion, error)
func (c *Client) RespondToQuestion(id, response string) error
```

*Rationale:* Follows the exact pattern of all other client methods (e.g., `ListAgents`, `RunAgent`). The CLI and any future consumers use this typed client rather than raw HTTP.

### 5. CLI: `questions` command group in `server/internal/cli/`

A new file `server/internal/cli/questions.go` adds a `newQuestionsCmd` function returning a cobra parent command with two subcommands:

- `list` — calls `client.ListQuestions("pending")`, renders a table using `lipgloss` (matching the style in `style.go`): truncated question ID (8 chars), agent ID, question text, options summary.
- `answer <id>` — fetches the specific question (by filtering the list), then either:
  - Displays numbered options and reads a number from stdin if options are present.
  - Displays the question and reads free-text from stdin if no options.
  - Short-circuits to the `--response` flag value if provided (non-interactive mode).

*Rationale:* Follows the `agent.go` / `jobs.go` pattern exactly. `bufio.NewReader(os.Stdin)` for interactive input, no additional dependencies.

### 6. SwiftUI: new `QuestionsViewModel` + `QuestionsView`

**Data layer** (`DianeClient` or new `QuestionsService.swift`):
- `GET /questions?status=pending` polled every 10 seconds (matching the polling interval used in `EmergentAgentMCPContext.swift`).
- `POST /questions/{id}/respond` for submission.
- Both calls go to the daemon over the existing `DianeClient` Unix-socket transport.

**`QuestionsViewModel.swift`**:
```swift
@MainActor class QuestionsViewModel: ObservableObject {
    @Published var questions: [AgentQuestion] = []
    @Published var pendingCount: Int = 0
    @Published var isLoading: Bool = false
    @Published var error: String? = nil

    func refresh() async
    func respond(to id: String, response: String) async throws
}
```

**`QuestionsView.swift`** (new file in `Diane/Diane/Views/Emergent/`):
- Uses `MasterDetailView` pattern consistent with `AgentMonitoringView.swift`.
- Master list: one row per question showing question text (truncated to 2 lines) and agent ID, sorted by `createdAt` ascending.
- Detail view: full question text, then either a `Picker`/radio-style list for option questions or a `TextEditor` for free-text. Submit button disabled until valid input. Inline error label below Submit on failure.
- Empty state: centred "No pending questions" text with `Spacing.large` padding.

**Badge**: A `.badge(viewModel.pendingCount)` modifier on the sidebar navigation item for the Questions section (or a toolbar item if the panel is not in the sidebar).

*Rationale:* Reuses existing `MasterDetailView`, `DetailSection`, `SummaryCard`, and design tokens already in the codebase, keeping visual consistency with the agent monitoring and MCP server views.

## Risks / Trade-offs

- **Project ID bootstrap latency**: The `/api/auth/me` call adds one extra HTTP round-trip at daemon startup. Mitigated by caching the result in memory; errors here produce a clear 503 to the client.
- **Polling vs. push for the GUI**: 10-second polling means a user could wait up to 10 seconds before seeing a new question. Acceptable for v1; a future enhancement could use SSE if the Emergent API exposes it.
- **Emergent not configured**: Both daemon endpoints and CLI commands return early with a clear "Emergent not configured" message, identical to the pattern used for agents and providers.
