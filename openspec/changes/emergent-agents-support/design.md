## Context

Currently, the custom agent creation form enforces a hardcoded URL endpoint. The Emergent API supports dynamic "emergent" agents that run in sandboxed workspaces (e.g., Firecracker, e2b) defined by base images, repositories, and setup commands, rather than rigid HTTP endpoints. Diane needs to expose these capabilities.

## Goals / Non-Goals

**Goals:**
- Make the URL field optional in the Custom Agent UI.
- Expose Emergent Workspace settings (Base Image, Repo Source, Setup Commands, Provider) in the UI.
- Update the `Agent` data models to include an optional `WorkspaceConfig` object.
- Route URL-less agents to the emergent execution path.

**Non-Goals:**
- Full implementation of the Emergent Engine backend itself (Diane only acts as the client configuring it).

## Decisions

1. **Model Update (`AgentConfig` & `AgentWorkspaceConfig`)**:
   - Change `url` property to optional.
   - Introduce a nested `WorkspaceConfig` struct in Swift containing `baseImage`, `repoUrl`, `repoBranch`, `setupCommands`, and `provider`.
   - *Rationale*: Aligns Diane's client model with the `AgentWorkspaceConfig` schema from the Emergent OpenAPI spec.

2. **UI Updates (Custom Agent Form)**:
   - Add a toggle or automatically detect emergent mode when URL is omitted.
   - Add a new `DetailSection(title: "Workspace Configuration")` containing fields for Image, Repo URL, and a StringArrayEditor for Setup Commands.
   - *Rationale*: Provides a native, integrated experience for configuring advanced agent sandboxes without leaving Diane.

3. **Execution & Storage Routing**:
   - When saving or triggering an agent without a URL, package the `WorkspaceConfig` and send it to the Go backend, which proxies it to the Emergent API (`/api/admin/workspace-images` or agent trigger endpoints).

## Risks / Trade-offs

- **UI Complexity**: Adding many fields might clutter the simple custom agent form.
  - *Mitigation*: Hide the Workspace Configuration section behind an "Advanced" toggle or only show it when "Emergent Agent" is selected as the type.
