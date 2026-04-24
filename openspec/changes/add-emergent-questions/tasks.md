## 1. Go service layer (`server/internal/emergent/questions.go`)

- [x] 1.1 Add `ProjectID` field to `Config` struct in `config.go` and populate it from the config file if present.
- [x] 1.2 Create `server/internal/emergent/questions.go` with `AgentQuestion`, `AgentQuestionOption`, and `QuestionsService` types.
- [x] 1.3 Implement `NewQuestionsService(cfg *Config) (*QuestionsService, error)` — calls `GET /api/auth/me` to resolve `project_id` if not already set in config, then stores base URL, API key, project ID, and a `*http.Client`.
- [x] 1.4 Implement `ListQuestions(ctx, status string) ([]AgentQuestion, error)` calling `GET /api/projects/{projectId}/agent-questions?status={status}`.
- [x] 1.5 Implement `RespondToQuestion(ctx, questionID, response string) error` calling `POST /api/projects/{projectId}/agent-questions/{questionId}/respond` with `{"response": "..."}` body; map HTTP 404 and 409 to typed errors.

## 2. Daemon API server (`server/internal/api/api.go`)

- [x] 2.1 Add `questionsService *emergent.QuestionsService` field to the `Server` struct.
- [x] 2.2 In `NewServer()`, attempt to instantiate `QuestionsService` (log a warning and leave it nil if Emergent is not configured — do not fail startup).
- [x] 2.3 Register `mux.HandleFunc("/questions", s.handleQuestions)` and `mux.HandleFunc("/questions/", s.handleQuestionAction)` alongside existing routes.
- [x] 2.4 Implement `handleQuestions` — `GET` only; reads `?status=` param (default `pending`); returns 503 if `questionsService` is nil; otherwise returns JSON array.
- [x] 2.5 Implement `handleQuestionAction` — routes `POST /questions/{id}/respond`; parses body; forwards 404/409 from upstream with appropriate messages; returns 202 on success.

## 3. Daemon API client (`server/internal/api/client.go`)

- [x] 3.1 Add `ListQuestions(status string) ([]emergent.AgentQuestion, error)` to `Client` — `GET http://unix/questions?status={status}`, decodes JSON response.
- [x] 3.2 Add `RespondToQuestion(id, response string) error` to `Client` — `POST http://unix/questions/{id}/respond` with JSON body; returns error on non-202 response.

## 4. CLI commands (`server/internal/cli/questions.go`)

- [x] 4.1 Create `server/internal/cli/questions.go` with `newQuestionsCmd(client *api.Client) *cobra.Command`.
- [x] 4.2 Implement `questions list` subcommand — calls `client.ListQuestions("pending")`; renders a lipgloss table (question ID truncated to 8 chars, agent ID, question text, options count); prints "No pending questions." when list is empty.
- [x] 4.3 Implement `questions answer <id>` subcommand — looks up the question by ID from the list; if options present, prints numbered list and reads a selection from stdin; if no options, reads free-text from stdin; `--response` flag skips interactive prompt.
- [x] 4.4 Register `newQuestionsCmd` in the root cobra command in `server/cmd/acp-server/main.go` (or wherever the CLI root is assembled).

## 5. Swift model (`Diane/Diane/Models/AgentQuestion.swift`)

- [x] 5.1 Create `Diane/Diane/Models/AgentQuestion.swift` with `AgentQuestion` and `AgentQuestionOption` structs (`Codable`, `Identifiable`), matching the fields from the Emergent API (`id`, `runId`, `agentId`, `question`, `options`, `status`, `createdAt`).
- [x] 5.2 Add `AgentQuestionStatus` enum (`pending`, `answered`, `expired`, `cancelled`).

## 6. Swift data layer

- [x] 6.1 Add `fetchPendingQuestions() async throws -> [AgentQuestion]` to `DianeClient` (or a new `QuestionsService.swift`) — `GET /questions?status=pending` over the Unix socket transport.
- [x] 6.2 Add `respondToQuestion(id: String, response: String) async throws` — `POST /questions/{id}/respond` with `{"response": "..."}` body; throws on non-202.

## 7. SwiftUI ViewModel (`Diane/Diane/ViewModels/QuestionsViewModel.swift`)

- [x] 7.1 Create `QuestionsViewModel.swift` as a `@MainActor ObservableObject` with `@Published` properties: `questions: [AgentQuestion]`, `pendingCount: Int`, `isLoading: Bool`, `error: String?`.
- [x] 7.2 Implement `refresh() async` — calls `fetchPendingQuestions`, updates `questions` and `pendingCount`, clears error on success, sets error string on failure.
- [x] 7.3 Implement `respond(to id: String, response: String) async throws` — calls `respondToQuestion`, then calls `refresh()` on success.
- [x] 7.4 Start a 10-second polling timer on `init()` that calls `refresh()` in a detached task; cancel on `deinit`.

## 8. SwiftUI View (`Diane/Diane/Views/Emergent/QuestionsView.swift`)

- [x] 8.1 Create `QuestionsView.swift` using `MasterDetailView` — master list shows one row per question (question text truncated to 2 lines, agent ID as secondary label), sorted by `createdAt` ascending.
- [x] 8.2 Add empty state to master list: centred "No pending questions" with `Spacing.large` padding, shown when `viewModel.questions` is empty and not loading.
- [x] 8.3 Implement detail view — shows full question text in a `DetailSection`; if `question.options` is non-empty, renders a selectable list (radio-style) for options; otherwise renders a `TextEditor` for free-text input.
- [x] 8.4 Add a "Submit" button in the detail view — disabled until an option is selected or the text field is non-empty; calls `viewModel.respond(to:response:)` on tap.
- [x] 8.5 Show inline error label below Submit when `viewModel.error` is non-nil and the failed question is selected; preserve the user's entered response across retries.

## 9. Sidebar integration

- [x] 9.1 Add a "Questions" navigation item to the macOS sidebar (or toolbar) wired to `QuestionsView`.
- [x] 9.2 Apply `.badge(viewModel.pendingCount)` to the navigation item; hide badge (or show 0) when `pendingCount == 0`.
- [x] 9.3 Inject `QuestionsViewModel` as a shared `@StateObject` at the app or scene level so the badge count stays live while navigating other views.
