## Context

Diane utilizes the Emergent platform as its backend for handling AI orchestration. Currently, most agent execution might happen locally or through other means, but Emergent supports running custom agents natively on its infrastructure. We need to integrate this functionality directly into Diane so users can create, configure, deploy, and monitor these backend-executed custom agents. 

This integration requires new API clients for the Emergent backend, as well as UI modifications to handle the configuration and monitoring lifecycles.

## Goals / Non-Goals

**Goals:**
- Provide a seamless mechanism to deploy custom agents to the Emergent backend.
- Build a robust API client integration for managing the lifecycle of Emergent custom agents.
- Create a user interface for monitoring active Emergent agents, their logs, and their status using standard Diane layout patterns (e.g., `MasterDetailView`, `SummaryCard`).
- Ensure MCP servers can be correctly passed or referenced when running agents on the Emergent backend.

**Non-Goals:**
- Redesigning the entire Emergent backend orchestration logic; we are just acting as a client.
- Completely deprecating local agent execution, if any.
- Creating a visual node-based agent builder (out of scope for now, configuration is enough).

## Decisions

- **API Client Layer:** We will add a new `EmergentAgentService` to handle all REST/GraphQL API interactions specifically related to custom agents. This keeps the logic isolated from standard orchestration services.
- **Monitoring UI:** We will use the existing `MasterDetailView` component to build the monitoring dashboard. The master column will list active agents, and the detail view will show specific metrics (using `SummaryCard`) and a live log feed.
- **State Management:** Use SwiftUI's `ObservableObject` and state containers to manage the real-time polling or WebSocket connections required for monitoring agent status.

## Risks / Trade-offs

- **Risk:** High latency or connection dropping while monitoring an agent.
  - *Mitigation:* Implement robust retry logic and indicate "Offline" or "Reconnecting" states in the UI clearly using standard `Spacing.large` for loading/error states.
- **Risk:** Tool execution conflicts between local MCP servers and backend agents.
  - *Mitigation:* Ensure the configuration flow explicitly defines tool routing rules before an agent is deployed to the Emergent backend.
