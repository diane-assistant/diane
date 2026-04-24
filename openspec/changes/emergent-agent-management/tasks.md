## 1. Core API and Models

- [x] 1.1 Create `EmergentModels.swift` and define Codable DTOs (`EmergentAgentDTO`, `EmergentAgentRunDTO`, `EmergentMCPServerDTO`, `EmergentWorkspaceImageDTO`, `EmergentAgentWorkspaceConfig` etc.) matching the OpenAPI spec using `snake_case` decoding.
- [x] 1.2 Create `EmergentAdminClient.swift` class.
- [x] 1.3 Add `baseURL`, `bearerToken`, and `projectId` properties to `EmergentAdminClient`, driven by `@AppStorage` defaults.
- [x] 1.4 Implement internal generic `request<T>` method in `EmergentAdminClient` that applies `Authorization` and `X-Project-ID` headers.
- [x] 1.5 Implement Agent CRUD methods (`getAgents`, `createAgent`, `getAgent`, `updateAgent`, `deleteAgent`).
- [x] 1.6 Implement Agent Run methods (`triggerRun`, `getRuns`, `cancelRun`, `getPendingEvents`, `batchTrigger`).
- [x] 1.7 Implement MCP Server methods (`getMCPServers`, `getMCPServer`, `createMCPServer`, `updateMCPServer`, `deleteMCPServer`).
- [x] 1.8 Implement Workspace Image methods (`getWorkspaceImages`, `createWorkspaceImage`, `deleteWorkspaceImage`).
- [x] 1.9 Implement Workspace Config methods (`getWorkspaceConfig`, `updateWorkspaceConfig`).
- [x] 1.10 In `SettingsView.swift`, add fields for "Emergent Base URL", "Emergent API Key", and "Emergent Project ID" and bind them to `@AppStorage`.

## 2. Navigation and Top-level Routing

- [x] 2.1 Update `MainWindowView.Section` enum to include `case emergent = "Emergent"` with the `sparkles` icon.
- [x] 2.2 Add navigation routing in `MainWindowView`'s `onReceive` and `detailView(for:)` switch statements.
- [x] 2.3 Create `EmergentDashboardView.swift` to serve as the top-level section container.
- [x] 2.4 Add a top picker or segmented control in `EmergentDashboardView` to switch between "Agents", "MCP Servers", and "Images".
- [x] 2.5 In `DianeApp.swift`, add a keyboard shortcut (e.g. `Cmd+7`) to post the `"NavigateToSection"` notification for `emergent`.

## 3. Emergent MCP Server Registry

- [x] 3.1 Create `EmergentMCPRegistryViewModel.swift` (`@Observable`) to manage `servers: [EmergentMCPServerDTO]`, selected server, loading states, and inline edit state (`isInEditMode`, `hasChanges`, `editSnapshot`).
- [x] 3.2 Create `EmergentMCPServerRegistryView.swift` using `MasterDetailView`.
- [x] 3.3 Implement the master list showing server name, type badge, and tool count.
- [x] 3.4 Implement the detail pane showing configuration fields (`InfoRow`) and a read-only list of associated tools.
- [x] 3.5 Implement inline edit mode in the detail pane with Save/Discard actions, swapping static text for `TextField`, `StringArrayEditor`, and `KeyValueEditor` based on the server type.
- [x] 3.6 Create `EmergentMCPServerFormSheet.swift` for the "Add Server" flow, handling type-conditional validation (e.g. `command` required for stdio).
- [x] 3.7 Add Delete action with confirmation dialog.

## 4. Emergent Workspace Images

- [x] 4.1 Create `EmergentWorkspaceImagesViewModel.swift` (`@Observable`) to manage `images: [EmergentWorkspaceImageDTO]`.
- [x] 4.2 Create `EmergentWorkspaceImagesView.swift` using a standard `List` (no master-detail split needed).
- [x] 4.3 Show list rows with image name, provider badge, type (built-in/custom), and status badge (e.g. "pulling", "ready").
- [x] 4.4 Create a register sheet for new custom images with `name`, `docker_ref`, and `provider` inputs.
- [x] 4.5 Add Delete action for custom images (hide for built-in) with confirmation dialog.

## 5. Emergent Agent Management

- [x] 5.1 Create `EmergentAgentManagementViewModel.swift` (`@Observable`) to manage `agents: [EmergentAgentDTO]`, selected agent, inline edit state, `runs`, and `pendingEvents`.
- [x] 5.2 Create `EmergentAgentManagementView.swift` using `MasterDetailView`.
- [x] 5.3 Implement the master list showing agent name, trigger type badge, execution mode badge, and status.
- [x] 5.4 Implement the "Details" tab in the detail pane, using `DetailSection` and `InfoRow` to show config, schedule, prompt, and capabilities.
- [x] 5.5 Implement inline edit mode for the Details tab (edit name, description, prompt, trigger type, schedule).
- [x] 5.6 Implement the "Runs" tab to fetch and display the agent's recent run history (`getRuns`).
- [x] 5.7 In the Runs tab, add a "Cancel" button for runs with status `running` or `paused`.
- [x] 5.8 Add a "Run Now" button to the agent detail header.
- [x] 5.9 Add a Delete action to the context menu/header with confirmation dialog.

## 6. Reaction Agent Pending Events

- [x] 6.1 In `EmergentAgentManagementView`, add a third "Pending Events" tab that only appears when `agent.triggerType == .reaction`.
- [x] 6.2 Implement a multi-selection `List` showing the object type, key, id, and timestamp of pending events.
- [x] 6.3 Show a "Reaction Config" summary block above the list.
- [x] 6.4 Add a "Trigger Selected" button that calls `batchTrigger` with the selected IDs.
- [x] 6.5 Display the batch trigger result summary (queued vs skipped) as an inline banner.

## 7. Extensions to Existing Agent Views

- [x] 7.1 In `AgentMonitoringView.swift` (existing), add a "Run Now" button to the detail header that triggers the agent.
- [x] 7.2 In `AgentMonitoringView.swift`, add a "Cancel Run" button for active runs.
- [x] 7.3 In `AgentMonitoringView.swift`, add a "Runs" tab showing the history of runs (sharing logic/ViewModel with the management view or duplicating the component).
- [x] 7.4 In `AgentConfigurationView.swift` (existing), fetch `AgentWorkspaceConfig` on load via `EmergentAdminClient`.
- [x] 7.5 Add a collapsible "Workspace Configuration" `DetailSection` bound to the workspace config fields.
- [x] 7.6 Add `TextField`, `StringArrayEditor`, and `Picker` components for `baseImage`, `provider`, `repoSource`, `resourceLimits`, and `setupCommands`.
- [x] 7.7 Make the `baseImage` field suggest or pick from the loaded `WorkspaceImages` list.
- [x] 7.8 On save, conditionally call `PUT /api/admin/agent-definitions/{id}/workspace-config` based on the enabled toggle.
