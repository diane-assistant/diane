# Capability: emergent-agent-monitoring

### Requirement: View Active Custom Agents
The system SHALL display a list of all custom agents currently running or recently completed on the Emergent backend.

#### Scenario: User navigates to Agent Monitoring view
- **WHEN** the user opens the Agent Monitoring dashboard
- **THEN** the system MUST present a MasterDetailView where the master column lists the agents
- **THEN** the master column MUST use MasterListHeader

### Requirement: View Agent Live Details
The system SHALL display real-time status, metric summaries, and execution logs for a selected Emergent custom agent.

#### Scenario: User selects a running agent
- **WHEN** the user selects an agent from the master list
- **THEN** the system MUST display the agent's detailed status in the detail view
- **THEN** the system MUST display key metrics using the SummaryCard component
- **THEN** the system MUST display a live-updating log feed in a DetailSection

#### Scenario: Agent goes offline or disconnects
- **WHEN** the connection to monitor the agent status fails
- **THEN** the system MUST display an error or offline state using Spacing.large (12)
