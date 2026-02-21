## 1. Data Model Updates

- [x] 1.1 Update `Diane/Diane/Models/Agent.swift` to make `url` optional (verify).
- [x] 1.2 Create a `WorkspaceConfig` Swift struct matching the Emergent API (baseImage, repoUrl, repoBranch, setupCommands, provider).
- [x] 1.3 Add an optional `workspaceConfig` property to the `AgentConfig` model.
- [x] 1.4 Update Go backend (`server/internal/store/acp_agent.go`, `agent_emergent.go`) to parse and store the new workspace configuration fields.

## 2. UI Updates (Custom Agent Form)

- [x] 2.1 Update `AgentsViewModel.swift` to handle optional URL and hold state for new workspace fields (image, repo URL, setup commands).
- [x] 2.2 In `AgentsViews.swift`, mark the "URL" field as optional.
- [x] 2.3 In `AgentsViews.swift`, add a new `DetailSection` for "Workspace" containing text fields for Image and Repo, and a `StringArrayEditor` for Setup Commands.
- [x] 2.4 Update the agent detail view to display the workspace configuration if present.

## 3. Execution Logic Updates

- [x] 3.1 Locate the agent execution routing in `server/internal/cli/agent.go`.
- [x] 3.2 Branch execution: If URL is missing, route to the emergent engine execution path.
- [x] 3.3 Ensure the workspace configuration payload is correctly passed to the Emergent API when deploying or triggering the agent.
