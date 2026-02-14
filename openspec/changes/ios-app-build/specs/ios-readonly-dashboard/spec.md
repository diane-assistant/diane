## ADDED Requirements

### Requirement: Status overview dashboard
The app SHALL display a summary dashboard showing the overall Diane server status.

#### Scenario: Dashboard shows connection and server state
- **WHEN** the user views the Status tab/section
- **THEN** the dashboard displays: connection state, server version, number of running MCP servers, number of configured providers, and number of configured agents

#### Scenario: Dashboard shows MCP server health
- **WHEN** MCP servers are configured
- **THEN** the dashboard shows a summary card with count of running, stopped, and errored servers

### Requirement: MCP Servers list view
The app SHALL display a read-only list of all configured MCP servers with their status.

#### Scenario: User views MCP servers list
- **WHEN** the user navigates to the MCP Servers section
- **THEN** a list is displayed showing each server's name, type (stdio/sse), and current status (running/stopped/error)

#### Scenario: User views MCP server detail
- **WHEN** the user taps an MCP server in the list
- **THEN** a detail view shows: server name, type, command/URL, arguments, environment variables, status, and the list of tools it provides

#### Scenario: Empty MCP servers list
- **WHEN** no MCP servers are configured
- **THEN** an empty state view is shown with a message indicating no servers are configured

### Requirement: Tools browser
The app SHALL display a read-only list of all available tools across MCP servers.

#### Scenario: User views tools list
- **WHEN** the user navigates to the Tools section (within MCP Servers or as a sub-view)
- **THEN** a list is displayed showing each tool's name, description, and which server provides it

#### Scenario: User views tool detail
- **WHEN** the user taps a tool in the list
- **THEN** a detail view shows the tool's name, description, and input schema

### Requirement: Providers list view
The app SHALL display a read-only list of all configured AI providers.

#### Scenario: User views providers list
- **WHEN** the user navigates to the Providers section
- **THEN** a list is displayed showing each provider's name, type, and authentication status

#### Scenario: User views provider detail
- **WHEN** the user taps a provider in the list
- **THEN** a detail view shows: provider name, type, base URL, authentication type, configured models, and usage statistics if available

### Requirement: Agents list view
The app SHALL display a read-only list of all configured agents.

#### Scenario: User views agents list
- **WHEN** the user navigates to the Agents section
- **THEN** a list is displayed showing each agent's name and description

#### Scenario: User views agent detail
- **WHEN** the user taps an agent in the list
- **THEN** a detail view shows: agent name, description, system prompt, assigned MCP servers, assigned provider, and recent run history

### Requirement: Contexts list view
The app SHALL display a read-only list of all configured contexts.

#### Scenario: User views contexts list
- **WHEN** the user navigates to the Contexts section
- **THEN** a list is displayed showing each context's name and connected servers

#### Scenario: User views context detail
- **WHEN** the user taps a context in the list
- **THEN** a detail view shows: context name, connected MCP servers, available tools, and connection status

### Requirement: Scheduled jobs list view
The app SHALL display a read-only list of all scheduled jobs.

#### Scenario: User views jobs list
- **WHEN** the user navigates to the Jobs section
- **THEN** a list is displayed showing each job's name, cron schedule, enabled status, and last run time

#### Scenario: User views job detail
- **WHEN** the user taps a job in the list
- **THEN** a detail view shows: job name, cron expression, human-readable schedule, enabled state, associated agent, and recent execution history

### Requirement: Usage statistics view
The app SHALL display usage statistics for API calls and token consumption.

#### Scenario: User views usage summary
- **WHEN** the user navigates to the Usage section
- **THEN** a summary is displayed showing total API calls, total tokens used, and cost breakdown by provider for the selected time range

#### Scenario: User filters usage by time range
- **WHEN** the user selects a different time range (today, 7 days, 30 days)
- **THEN** the usage statistics update to reflect the selected period

### Requirement: Settings view
The app SHALL display a settings screen with server connection configuration and app preferences.

#### Scenario: User views settings
- **WHEN** the user navigates to the Settings section
- **THEN** the settings screen shows: server address configuration, connection status, and app version

### Requirement: Pull-to-refresh on all list views
The app SHALL support pull-to-refresh on all list views to manually trigger a data refresh.

#### Scenario: User pulls to refresh
- **WHEN** the user pulls down on any list view
- **THEN** the app fetches fresh data from the server
- **AND** the list updates with the latest information

### Requirement: All views are read-only
The app SHALL NOT provide any controls to modify server configuration, create, edit, or delete resources in this initial version.

#### Scenario: No edit controls visible
- **WHEN** the user views any detail screen
- **THEN** no edit buttons, delete buttons, toggle switches, or text input fields for modifying data are displayed
