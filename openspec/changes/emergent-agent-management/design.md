## Context

The Emergent layer in Diane is currently a stub: `EmergentAgentService.swift` (63 lines, fully mocked), `AgentMonitoringView.swift` + `EmergentAgentMonitoringViewModel.swift` (local data only), and `AgentConfigurationView.swift` (no real API calls). The real Emergent API is at `http://mcj-emergent:3002` and uses bearer token auth + optional `X-Project-ID` header for project-scoped endpoints.

There is no `EmergentAdminClient` — it must be created. The `DianeHTTPClient` already has the bearer-auth pattern to follow (`Authorization: Bearer <apiKey>`, set per-request for non-empty keys). Sidebar navigation uses `MainWindowView.swift`'s `Section` enum + `detailView(for:)` switch — adding a new top-level section requires 4 localised edits.

## Goals / Non-Goals

**Goals:**
- Create `EmergentAdminClient.swift` with all Emergent admin API methods
- Add `EmergentAgentDTO`, `EmergentAgentRunDTO`, `EmergentMCPServerDTO`, `WorkspaceImageDTO`, `AgentWorkspaceConfig` Swift models
- Add `EmergentAgentManagementView` (agent CRUD + runs + cancel) with its ViewModel
- Extend `AgentMonitoringView` with Run Now, Cancel, and Runs tab
- Add `EmergentMCPServerRegistryView` (full CRUD) with its ViewModel
- Add `EmergentWorkspaceImagesView` (list/register/delete) with its ViewModel
- Extend `AgentConfigurationView` with the full `AgentWorkspaceConfig` form section
- Wire everything into the sidebar under a new **"Emergent"** section (or sub-sections)

**Non-Goals:**
- Modifying the iOS app (Emergent admin is macOS-only)
- Real-time streaming of agent logs (covered by existing monitoring)
- `install-default-agents` UI (admin-only CLI operation)
- Per-tool enable/disable within the MCP registry (out of scope)

## Decisions

### 1. New top-level sidebar section: "Emergent"

**Choice**: Add a single `case emergent = "Emergent"` to `MainWindowView.Section`, routing to a new `EmergentDashboardView` that internally uses a secondary tab/section picker to navigate between: Agents, MCP Servers, Workspace Images.

**Why not separate sidebar entries for each sub-feature**: The three new areas (agents, MCP servers, workspace images) are all Emergent admin concepts and should be visually grouped. A single top-level "Emergent" entry with internal sub-navigation avoids cluttering the sidebar, which already has 7 items. The existing `AgentMonitoringView` (accessed via sidebar "Agents" section) is separate from the new management view — monitoring stays where it is.

**Implementation**: `EmergentDashboardView` contains a `Picker`/`segmented` control at the top or a secondary `List` sidebar, routing between `EmergentAgentManagementView`, `EmergentMCPServerRegistryView`, `EmergentWorkspaceImagesView`.

**Sidebar edits** (all in `MainWindowView.swift`):
- Add `case emergent = "Emergent"` with icon `"sparkles"` to the `Section` enum
- Add `case .emergent: "sparkles"` to the `icon` computed property
- Add `case "emergent": selectedSection = .emergent` to the `onReceive` handler
- Add `case .emergent: EmergentDashboardView()` to `detailView(for:)`

### 2. New `EmergentAdminClient` — standalone HTTP client, not extending DianeClient

**Choice**: Create `EmergentAdminClient.swift` as a standalone `@MainActor final class` (not extending `DianeClientProtocol`) with a configurable `baseURL` and `bearerToken`. The base URL defaults to `http://mcj-emergent:3002` but is configurable via `SettingsView`. The client sends `Authorization: Bearer <token>` on every request, and `X-Project-ID` as a header parameter when a `projectId` is provided.

**Why not add to `DianeClientProtocol`**: The Emergent API is a completely separate backend with different auth, URL, and domain. Mixing it into `DianeClientProtocol` would force both `DianeClient` (Unix socket) and `DianeHTTPClient` (HTTP to Diane daemon) to implement Emergent methods, which makes no sense architecturally. A separate client keeps concerns clean.

**Pattern** (mirrors `DianeHTTPClient`):
```swift
@MainActor final class EmergentAdminClient {
    var baseURL: URL
    var bearerToken: String?
    var projectId: String?

    private func request<T: Decodable>(_ path: String, method: String = "GET", body: Data? = nil) async throws -> T
    // Sets Authorization: Bearer <token>, X-Project-ID: <projectId> per-request
}
```

**Settings**: Add `emergentBaseURL` (String, default `"http://mcj-emergent:3002"`), `emergentBearerToken` (String), `emergentProjectId` (String) to `@AppStorage` in `SettingsView`. A shared `EmergentAdminClient.shared` singleton is configured from `AppStorage` values.

### 3. New Swift models for Emergent API DTOs

**Choice**: Create `EmergentModels.swift` in `Models/` with all new Codable structs matching the API DTOs. Use `snake_case` decoding with `.convertFromSnakeCase` strategy. Keep names distinct from existing types to avoid collision.

**Key new types**:

| Swift Type | Emergent DTO | Notes |
|---|---|---|
| `EmergentAgentDTO` | `AgentDTO` | Distinct from existing `AgentConfig`; includes `id, name, description, prompt, triggerType, executionMode, strategyType, cronSchedule, enabled, capabilities, reactionConfig, lastRunAt, lastRunStatus, createdAt, updatedAt, projectId` |
| `EmergentAgentRunDTO` | `AgentRunDTO` | `id, agentId, status, sessionStatus, startedAt, completedAt, durationMs, stepCount, errorMessage, parentRunId, maxSteps` |
| `EmergentAgentCapabilities` | `AgentCapabilities` | `canCreateObjects, canUpdateObjects, canDeleteObjects, canCreateRelationships, allowedObjectTypes` |
| `EmergentReactionConfig` | `ReactionConfig` | `events, objectTypes, concurrencyStrategy, ignoreAgentTriggered, ignoreSelfTriggered` |
| `EmergentPendingEventDTO` | `PendingEventObjectDTO` | `id, type, key, createdAt, updatedAt, version` |
| `EmergentMCPServerDTO` | `MCPServerDTO` (Emergent) | Different from existing `MCPServer` (Diane); prefix avoids collision: `id, name, type, command, args, env, url, headers, enabled, toolCount, projectId, createdAt, updatedAt` |
| `EmergentMCPServerDetailDTO` | `MCPServerDetailDTO` | Extends `EmergentMCPServerDTO` + `tools: [EmergentMCPToolDTO]` |
| `EmergentMCPToolDTO` | `MCPServerToolDTO` | `id, serverId, toolName, description, enabled, inputSchema` |
| `EmergentWorkspaceImageDTO` | `WorkspaceImageDTO` | `id, name, provider, type, status, dockerRef, projectId, errorMsg, createdAt, updatedAt` |
| `EmergentAgentWorkspaceConfig` | `AgentWorkspaceConfig` | `enabled, baseImage, provider, repoSource (type/url/branch), resourceLimits (cpu/memory/disk), setupCommands, tools, checkoutOnStart` |

**Existing `WorkspaceConfig`** in `AgentWorkspace.swift` is the Diane-daemon concept and stays unchanged. The new `EmergentAgentWorkspaceConfig` is the Emergent-side concept.

### 4. Agent Management view: tabbed detail pane

**Choice**: `EmergentAgentManagementView` uses `MasterDetailView`. The master column lists all agents (via `EmergentAgentManagementViewModel`). The detail pane has two tabs: "Details" (config + edit mode) and "Runs" (run history with cancel).

**Detail tab sections**:
- "General" `DetailSection`: name, description, triggerType badge, executionMode badge, strategyType, enabled toggle — read/edit mode
- "Schedule" `DetailSection` (only when `triggerType == .schedule`): cronSchedule TextField
- "Prompt" `DetailSection`: multi-line TextEditor for prompt field
- "Capabilities" `DetailSection`: boolean read-only InfoRows (canCreate/Update/Delete/Create), allowedObjectTypes badges
- "Reaction Config" `DetailSection` (only when `triggerType == .reaction`): events, objectTypes, concurrencyStrategy, ignore flags
- "Workspace Config" `DetailSection`: shows `AgentWorkspaceConfig` fields (see § 7)
- For `reaction` agents: third tab "Pending Events"

**Edit mode**: Follows `editable-mcp-server-view` pattern — snapshot-on-enter, Save/Discard in header toolbar, `hasChanges` computed from snapshot comparison. `isEditMode: Bool` on ViewModel.

**Run history tab**: Calls `GET /api/admin/agents/{id}/runs?limit=10`. Each row shows status badge (colour-coded), relative time, formatted duration, stepCount. Running runs show "Cancel" button. Auto-refreshes every 5s when any run is in `running` status.

**Header toolbar buttons**:
- "Run Now" → `POST /api/admin/agents/{id}/trigger` → shows transient banner with runId
- "Edit" / "Save" / "Discard" (edit mode toggle)
- "Delete" (destructive, in toolbar or context menu, requires confirmation alert)

### 5. Pending Events as a third tab (reaction agents only)

**Choice**: The "Pending Events" tab appears in `EmergentAgentManagementView`'s detail pane only when `agent.triggerType == .reaction`. It uses a `List` with multi-select (macOS `List` selection binding with a `Set<String>` of ids). A "Trigger Selected ({n})" button in the tab header becomes enabled when selection is non-empty, disabled/warning when selection > 100.

**ViewModel method**: `batchTrigger(objectIds: [String])` → `POST /api/admin/agents/{id}/batch-trigger` → shows summary: "X queued, Y skipped" as an inline banner.

### 6. Emergent MCP Server Registry — separate ViewModel, reuse MCPServerFormSheet pattern

**Choice**: `EmergentMCPServerRegistryView` uses `MasterDetailView`. `EmergentMCPRegistryViewModel` is a new `@MainActor @Observable final class` (not extending `MCPRegistryViewModel` which handles the Diane-daemon MCP servers). The create sheet reuses the `MCPServerFormSheet` visual pattern — a new `EmergentMCPServerFormSheet` — since the fields differ slightly (no "placement" concept, no "contexts" checkboxes). The detail pane uses inline edit mode matching the `editable-mcp-server-view` pattern exactly (same `isEditMode` / `hasChanges` / snapshot mechanism).

**Type-conditional field rendering** in both create form and edit mode:
- `stdio`: command (TextField, required), args (`StringArrayEditor`), env (`KeyValueEditor`)
- `sse` / `http`: url (TextField, required), headers (`KeyValueEditor`)
- All types: name (TextField, required), enabled (Toggle), env (`KeyValueEditor`)

**Tools section**: fetched via `GET /api/admin/mcp-servers/{id}` (detail endpoint). Displayed as a read-only list of tool names with description and enabled badge. Not editable in this change.

### 7. Workspace Configuration form section in `AgentConfigurationView`

**Choice**: Add a collapsible "Workspace" `DetailSection` at the bottom of the existing `AgentConfigurationView`. The section is collapsed when `workspaceConfig.enabled == false` (showing only the enabled Toggle). When enabled, it expands to show all `EmergentAgentWorkspaceConfig` fields.

**Field controls**:
| Field | Control |
|---|---|
| `enabled` | Toggle (always visible, collapses/expands rest) |
| `baseImage` | TextField with a `.popover` or `Picker` backed by images from `GET /api/admin/workspace-images` (status == ready), loaded lazily when the user focuses the field |
| `provider` | Picker: "Auto", "Firecracker", "gVisor", "E2B" (maps to `""`, `"firecracker"`, `"gvisor"`, `"e2b"`) |
| `repoSource.type` | Picker: "None", "Task Context", "Fixed" |
| `repoSource.url` | TextField (shown only when type == `.fixed`) |
| `repoSource.branch` | TextField (shown when type == `.fixed` or `.taskContext`) |
| `resourceLimits.cpu` | TextField (e.g. "2") |
| `resourceLimits.memory` | TextField (e.g. "4G") |
| `resourceLimits.disk` | TextField (e.g. "10G") |
| `setupCommands` | `StringArrayEditor` |
| `tools` | `StringArrayEditor` |
| `checkoutOnStart` | Toggle |

**Save behavior**: On "Deploy", `AgentConfigurationView` first calls existing agent save, then if `workspaceConfig.enabled == true`: `PUT /api/admin/agent-definitions/{id}/workspace-config`. If enabled == false: sends `{ "enabled": false }` to explicitly disable. The two API calls are sequential; workspace config failure shows an error in the Workspace section without rolling back the agent save.

**Fetch on load**: When `AgentConfigurationView` appears for an existing agent definition (has an `id`), `EmergentAdminClient.shared.getWorkspaceConfig(definitionId:)` is called and the result populates the workspace form fields.

### 8. Workspace Images view — flat list, no detail pane split needed

**Choice**: `EmergentWorkspaceImagesView` uses a plain `List` (not `MasterDetailView`) since the detail content is compact enough to show in-line via expansion or a lightweight sheet. Built-in images are non-deletable (delete button hidden). Custom images show a "Delete" button with a confirmation alert.

**Register sheet**: A simple form sheet with name TextField, docker_ref TextField, provider Picker (optional). Shows "Pulling image…" status badge on the newly registered image row after creation.

**`EmergentWorkspaceImagesViewModel`**: `@MainActor @Observable final class`. Methods: `loadImages()`, `registerImage(name:dockerRef:provider:)`, `deleteImage(id:)`.

### 9. ViewModels summary

| ViewModel | Owns | New / Extended |
|---|---|---|
| `EmergentAdminClient` | All HTTP calls to Emergent API | **New** |
| `EmergentAgentManagementViewModel` | Agent list, selected agent, edit state, run history, pending events | **New** |
| `EmergentMCPRegistryViewModel` | Emergent MCP server list, selected server, edit state | **New** |
| `EmergentWorkspaceImagesViewModel` | Workspace image list, register/delete | **New** |
| `EmergentAgentMonitoringViewModel` | Existing monitoring — add `triggerRun()`, `cancelRun(runId:)`, `runs: [EmergentAgentRunDTO]` | **Extended** |
| `AgentsViewModel` | Existing ACP agents — no changes | Unchanged |

### 10. File change scope

| File | Change |
|---|---|
| `Models/EmergentModels.swift` | **New** — all Emergent DTO types |
| `Services/EmergentAdminClient.swift` | **New** — replaces/extends stub `EmergentAgentService`; all Emergent admin HTTP methods |
| `Views/Emergent/EmergentDashboardView.swift` | **New** — top-level container with sub-section picker |
| `Views/Emergent/EmergentAgentManagementView.swift` | **New** — agent list + detail + tabs (config, runs, pending events) |
| `Views/Emergent/EmergentMCPServerRegistryView.swift` | **New** — MCP server list + detail + create sheet |
| `Views/Emergent/EmergentWorkspaceImagesView.swift` | **New** — workspace images list + register sheet |
| `ViewModels/EmergentAgentManagementViewModel.swift` | **New** |
| `ViewModels/EmergentMCPRegistryViewModel.swift` | **New** |
| `ViewModels/EmergentWorkspaceImagesViewModel.swift` | **New** |
| `Views/Emergent/AgentConfigurationView.swift` | **Modified** — add collapsible Workspace section |
| `Views/Emergent/AgentMonitoringView.swift` | **Modified** — add Run Now, Cancel, Runs tab |
| `ViewModels/EmergentAgentMonitoringViewModel.swift` | **Modified** — add trigger/cancel/runs state |
| `Views/MainWindowView.swift` | **Modified** — add `case emergent` to `Section` enum + routing |
| `DianeApp.swift` | **Modified** — add keyboard shortcut for "Emergent" section |
| `Views/SettingsView.swift` | **Modified** — add Emergent connection settings (baseURL, token, projectId) |

## Risks / Trade-offs

- **[Risk] `EmergentAgentService` is referenced by `AgentConfigurationView` via `@EnvironmentObject`** → Mitigation: Keep `EmergentAgentService` in place for now; wire `EmergentAdminClient.shared` as a singleton alongside it. Migrate call sites incrementally within this change.
- **[Risk] No existing project ID management in Diane** → Mitigation: Store `emergentProjectId` in `@AppStorage`. The settings view prompts the user to enter it. Most endpoints work without it; it is only required for `GET /api/admin/agents` (list) and MCP server listing.
- **[Risk] MCP server name collision with Diane-daemon `MCPServer`** → Mitigation: All new Emergent MCP types are prefixed `EmergentMCP*`. No shared model reuse.
- **[Risk] Workspace image lazy loading in base_image picker could add latency** → Mitigation: Load images once when `EmergentWorkspaceImagesViewModel` initialises; share the instance via environment. The agent config form consumes the same cached list.
- **[Trade-off] Two tabs vs flat detail for agent management** → Tabs add complexity but are necessary because the detail content (config fields + run history + pending events) is too dense for a single scroll view. macOS `TabView` with `.tabViewStyle(.automatic)` is the right fit.
