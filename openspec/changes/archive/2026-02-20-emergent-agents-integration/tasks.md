## 1. Emergent API Client Layer

- [x] 1.1 Create `EmergentAgentService` to handle core API communication with the Emergent backend
- [x] 1.2 Implement the POST endpoint request to configure and save a custom agent
- [x] 1.3 Implement the POST endpoint request to deploy/execute the custom agent on Emergent
- [x] 1.4 Create polling or WebSocket mechanisms to fetch real-time agent status, logs, and metrics

## 2. Agent Configuration UI

- [x] 2.1 Build the SwiftUI form for Custom Agent configuration (name, persona, tools)
- [x] 2.2 Wire the configuration form to `EmergentAgentService` to handle "Save" action
- [x] 2.3 Implement the "Deploy" action and handle "Deploying" / "Running" / "Error" states
- [x] 2.4 Use standard Spacing.large for loading and error states during deployment

## 3. Agent Monitoring UI

- [x] 3.1 Create `EmergentAgentMonitoringViewModel` to manage the real-time state of active agents
- [x] 3.2 Implement the `MasterDetailView` dashboard displaying all active/completed custom agents
- [x] 3.3 Ensure the master column uses `MasterListHeader` for the agent list
- [x] 3.4 Build the agent detail view using `SummaryCard` for displaying key execution metrics
- [x] 3.5 Implement the live log feed section using `DetailSection` component
- [x] 3.6 Handle offline/disconnect states in the detail view showing a clear message using Spacing.large

## 4. MCP Context Wiring

- [x] 4.1 Update MCP server context loading to pass selected tool capabilities into the agent configuration payload
