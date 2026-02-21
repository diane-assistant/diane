## Why

The current custom agent form enforces a requirement for a URL endpoint, which restricts the possibilities for creating and running local or "emergent" agents. By removing this requirement and exposing rich workspace configurations (like base images, repositories, and setup commands), we empower users to create and orchestrate fully dynamic, sandboxed emergent agents directly from Diane.

## What Changes

- Modify the Custom Agent creation form to make the "URL" field optional.
- Introduce a "Workspace Configuration" section in the UI (Base Image, Repository, Setup Commands, Provider) for emergent agents.
- Update underlying data models to support these advanced workspace parameters.
- Support logic to handle execution and connection of emergent agents using the Emergent Engine's workspace API.

## Capabilities

### New Capabilities
- `emergent-agents`: Support for instantiating and running agents dynamically with configurable workspaces (images, repos, sandbox providers) without relying on an external, hardcoded URL endpoint.

### Modified Capabilities
- `custom-agents`: Modifying the requirements for custom agents to support optional URL configurations and new workspace settings.

## Impact

- **UI**: The Custom Agent form views will need updates to allow blank/optional URL inputs and new fields for Workspace Configuration.
- **Agent Models**: Core data models for agents will need a new `WorkspaceConfig` struct to accommodate emergent settings.
- **Agent Execution**: Systems must gracefully route URL-less agents to the Emergent Engine, passing along the rich workspace configuration.
