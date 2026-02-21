## Why

Diane relies on the "Emergent" platform as its backend. The Emergent backend provides native functionality to run custom agents directly on their infrastructure. We want to leverage this capability to allow Diane users to configure, trigger, and monitor custom agents running seamlessly on the Emergent backend, expanding our orchestration capabilities without heavy local compute.

## What Changes

- Integrate with the Emergent backend APIs for custom agent execution.
- Add configuration flows in the Diane UI to define and deploy custom agents to the Emergent backend.
- Build monitoring components to observe the status, logs, and results of agents running on Emergent.
- Update the MCP integration to allow these backend agents to interact with localized or shared tools if supported by Emergent.

## Capabilities

### New Capabilities
- `emergent-custom-agents`: Covers the API integration, configuration, and execution lifecycle for running custom agents on the Emergent backend.
- `emergent-agent-monitoring`: UI components and data fetching logic to monitor the status and output of backend-executed agents.

### Modified Capabilities

## Impact

- **API Layer**: New endpoints/services to communicate with the Emergent backend's custom agent APIs.
- **UI**: New views for agent deployment and monitoring, utilizing the Diane design tokens and standard components (e.g., `DetailSection`, `SummaryCard`).
