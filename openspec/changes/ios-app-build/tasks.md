## 1. Go Server HTTP Listener

- [x] 1.1 Add `http_addr` field to server config struct and CLI flags (default empty = disabled)
- [x] 1.2 Create `readOnlyMiddleware` function that rejects non-GET requests with 405 and `Allow: GET` header
- [x] 1.3 In `api.go`, after mux setup, start TCP listener goroutine when `http_addr` is configured: `go http.Serve(tcpListener, readOnlyMiddleware(mux))`
- [x] 1.4 Add startup log line: `log.Printf("HTTP listener on %s", addr)`
- [x] 1.5 Ensure graceful shutdown of TCP listener alongside Unix socket listener
- [ ] 1.6 Test: `curl http://localhost:8080/status` returns same JSON as Unix socket
- [ ] 1.7 Test: `curl -X POST http://localhost:8080/reload` returns 405 Method Not Allowed
- [ ] 1.8 Test: server starts with Unix socket only when `http_addr` is not set
- [ ] 1.9 Test: both listeners serve requests concurrently without interference

## 2. Cross-Platform DesignTokens Colors

- [x] 2.1 Add `enum Colors` to DesignTokens.swift with static color properties: `windowBackground`, `controlBackground`, `textBackground`, `separatorColor`
- [x] 2.2 Implement each color with `#if os(macOS)` using `Color(nsColor:)` and `#if os(iOS)` using `Color(.systemBackground)` etc.
- [x] 2.3 Update SummaryCard.swift to use `DesignTokens.Colors.controlBackground` instead of `Color(nsColor: .controlBackgroundColor)`
- [x] 2.4 Update DetailSection.swift to use DesignTokens.Colors instead of `Color(nsColor:)` references
- [x] 2.5 Update KeyValueEditor.swift to use DesignTokens.Colors instead of `Color(nsColor:)` references
- [x] 2.6 Update StringArrayEditor.swift to use DesignTokens.Colors instead of `Color(nsColor:)` references
- [x] 2.7 Search for any remaining `Color(nsColor:)` in shared components and update
- [x] 2.8 Verify macOS app builds and renders colors identically after changes

## 3. Xcode Project — iOS Target Setup

- [x] 3.1 Add new iOS app target `DianeIOS` in Diane.xcodeproj with deployment target iOS 17.0
- [x] 3.2 Configure target for iPhone + iPad (TARGETED_DEVICE_FAMILY = "1,2")
- [x] 3.3 Set bundle identifier (e.g., `com.diane.ios`)
- [ ] 3.4 Create iOS app icon asset catalog
- [x] 3.5 Add shared files to iOS target membership: all `Models/*.swift` files
- [x] 3.6 Add shared files to iOS target membership: `Services/DianeClientProtocol.swift`
- [x] 3.7 Add shared files to iOS target membership: `Components/DesignTokens.swift`, `EmptyStateView.swift`, `InfoRow.swift`, `TimeRangePicker.swift`
- [ ] 3.8 Verify iOS target builds with shared files only (no platform errors)

## 4. iOS Network Client (DianeHTTPClient)

- [x] 4.1 Create `DianeHTTPClient.swift` implementing `DianeClientProtocol` with `@MainActor` isolation
- [x] 4.2 Add stored properties: `baseURL: URL`, `session: URLSession` with 10-second timeout configuration
- [x] 4.3 Implement private `request(_ path: String, method: String, body: Data?) async throws -> Data` using `URLSession.data(for:)`
- [x] 4.4 Port `makeGoCompatibleDecoder()` from DianeClient.swift for Go-compatible date parsing
- [x] 4.5 Implement read methods — `getStatus()`: GET `/status`, decode with Go-compatible decoder
- [x] 4.6 Implement read methods — `getMCPServers()`: GET `/mcp-servers`
- [x] 4.7 Implement read methods — `getMCPServerConfigs()`: GET `/mcp-servers-config`
- [x] 4.8 Implement read methods — `getTools()`: GET `/tools`
- [x] 4.9 Implement read methods — `getJobs()`: GET `/jobs`
- [x] 4.10 Implement read methods — `getJobLogs(jobName:limit:)`: GET `/jobs/logs?limit=&job_name=`
- [x] 4.11 Implement read methods — `getAgents()`: GET `/agents`
- [x] 4.12 Implement read methods — `getAgent(name:)`: GET `/agents/{name}` with URL encoding
- [x] 4.13 Implement read methods — `getAgentLogs(agentName:limit:)`: GET `/agents/logs?limit=&agent_name=`
- [x] 4.14 Implement read methods — `getContexts()`: GET `/contexts`
- [x] 4.15 Implement read methods — `getContextDetail(name:)`: GET `/contexts/{name}`
- [x] 4.16 Implement read methods — `getContextServers(contextName:)`: GET `/contexts/{name}/servers`
- [x] 4.17 Implement read methods — `getContextServerTools(contextName:serverName:)`: GET `/contexts/{name}/servers/{server}/tools`
- [x] 4.18 Implement read methods — `getProviders(type:)`: GET `/providers`
- [x] 4.19 Implement read methods — `getProvider(id:)`: GET `/providers/{id}`
- [x] 4.20 Implement read methods — `getUsage(from:to:limit:service:providerID:)`: GET `/usage`
- [x] 4.21 Implement read methods — `getUsageSummary(from:to:)`: GET `/usage/summary`
- [x] 4.22 Implement read methods — `health()`: GET `/health`
- [x] 4.23 Create `DianeHTTPClientError` enum with cases: `readOnlyMode`, `connectionFailed`, `decodingFailed`, `serverError(Int)`
- [x] 4.24 Implement all write methods to throw `DianeHTTPClientError.readOnlyMode`
- [x] 4.25 Implement all process management methods (`startDiane`, `stopDiane`, `restartDiane`, `sendReloadSignal`, `getPID`, `isProcessRunning`, `socketExists`) to throw or return defaults
- [x] 4.26 Add `updateBaseURL(_ url: URL)` method for server address changes
- [x] 4.27 Add iOS target membership to DianeHTTPClient.swift

## 5. Server Configuration & Connection State

- [x] 5.1 Create `ServerConfiguration.swift` with `@AppStorage` properties for host and port
- [x] 5.2 Create `ServerSetupView.swift` — first-launch screen with host/port text fields and Connect button
- [x] 5.3 Add Tailscale hostname hint text (e.g., "my-mac or my-mac.tail12345.ts.net")
- [x] 5.4 Add default port suggestion (8080) in the port field
- [x] 5.5 Implement connection test on Connect — call `health()` and show success/failure
- [x] 5.6 Persist successful server address to UserDefaults via `@AppStorage`

## 6. iOS Status Monitor

- [x] 6.1 Create `IOSStatusMonitor.swift` as `@Observable` class wrapping `DianeHTTPClient`
- [x] 6.2 Add `@Published` properties: `connectionState` (enum: connecting, connected, disconnected, error), `status: DianeStatus?`
- [x] 6.3 Implement 5-second polling timer using `Timer.publish` or async `Task.sleep`
- [x] 6.4 Observe `scenePhase` changes: start polling on `.active`, stop on `.background`/`.inactive`
- [x] 6.5 On foreground return: immediately poll then resume timer
- [x] 6.6 Update `connectionState` based on request success/failure
- [x] 6.7 Add iOS target membership to IOSStatusMonitor.swift

## 7. iOS App Entry Point

- [x] 7.1 Create `DianeIOSApp.swift` with `@main` and `WindowGroup` scene
- [x] 7.2 Create `@State` instances of `DianeHTTPClient` and `IOSStatusMonitor`
- [x] 7.3 Inject client and monitor into environment
- [x] 7.4 Show `ServerSetupView` when no server address is configured, otherwise show `ContentView`
- [x] 7.5 Add iOS target membership to DianeIOSApp.swift

## 8. iOS Adaptive Navigation

- [x] 8.1 Create `ContentView.swift` (iOS) that reads `@Environment(\.horizontalSizeClass)`
- [x] 8.2 Implement compact layout: `TabView` with tabs for Status, Servers, Agents, More
- [x] 8.3 Implement regular layout: `NavigationSplitView` with sidebar listing all 8 sections
- [x] 8.4 Create `Section` enum with cases: status, mcpServers, agents, contexts, providers, jobs, usage, settings
- [x] 8.5 Add SF Symbol icons for each section (house, server.rack, cpu, rectangle.connected.to.line.below, cloud, clock, chart.bar, gear)
- [x] 8.6 Implement `MoreView.swift` for the More tab — list of: Providers, Contexts, Jobs, Usage, Settings
- [x] 8.7 Add `@SceneStorage` for selected tab/section to preserve navigation state
- [x] 8.8 Reset navigation state when server address changes

## 9. iOS Status Dashboard View

- [x] 9.1 Create `StatusDashboardView.swift` showing connection state indicator (green/red/yellow dot + text)
- [x] 9.2 Display server summary cards: MCP server count (running/stopped/error), provider count, agent count
- [x] 9.3 Reuse `SummaryCard` component for metric display
- [x] 9.4 Add pull-to-refresh using `.refreshable` modifier
- [x] 9.5 Show empty/loading state while waiting for first poll response

## 10. iOS MCP Servers Views

- [x] 10.1 Create `MCPServersListView.swift` — List of MCP servers with name, type badge, status indicator
- [x] 10.2 Create `MCPServerDetailView.swift` — read-only detail with sections: Info (name, type, command/URL, args), Environment Variables, Status, Tools list
- [x] 10.3 Use `InfoRow` for label-value pairs in detail view
- [x] 10.4 Use `DetailSection` for grouping
- [x] 10.5 Show `EmptyStateView` when no servers configured
- [x] 10.6 Add NavigationLink from list to detail
- [x] 10.7 Add `.refreshable` to list view
- [x] 10.8 Add `.searchable` for filtering servers by name

## 11. iOS Tools Browser View

- [x] 11.1 Create `ToolsListView.swift` — List of tools with name, description, server name badge (tools shown inline in MCPServerDetailView)
- [x] 11.2 Create `ToolDetailView.swift` — read-only detail showing name, description, input schema (formatted JSON) (tools shown inline in MCPServerDetailView)
- [x] 11.3 Add `.searchable` for filtering tools by name (integrated into MCPServersViews)
- [x] 11.4 Add `.refreshable` modifier (integrated into MCPServersViews)

## 12. iOS Providers Views

- [x] 12.1 Create `ProvidersListView.swift` — List with provider name, type badge, auth status indicator
- [x] 12.2 Create `ProviderDetailView.swift` — read-only detail with sections: Info (name, type, base URL, auth type), Models list, Usage stats
- [x] 12.3 Use `InfoRow` and `DetailSection` components
- [x] 12.4 Show `EmptyStateView` when no providers configured
- [x] 12.5 Add `.refreshable` and `.searchable`

## 13. iOS Agents Views

- [x] 13.1 Create `AgentsListView.swift` — List with agent name, description, enabled status
- [x] 13.2 Create `AgentDetailView.swift` — read-only detail with sections: Info (name, description, system prompt), MCP Servers, Provider, Recent Runs
- [x] 13.3 Show recent run history in a list (date, status, duration)
- [x] 13.4 Show `EmptyStateView` when no agents configured
- [x] 13.5 Add `.refreshable` and `.searchable`

## 14. iOS Contexts Views

- [x] 14.1 Create `ContextsListView.swift` — List with context name, server count, default indicator
- [x] 14.2 Create `ContextDetailView.swift` — read-only detail with sections: Info (name, description), Connected Servers list, Available Tools list, Connection status
- [x] 14.3 Show `EmptyStateView` when no contexts configured
- [x] 14.4 Add `.refreshable` and `.searchable`

## 15. iOS Scheduled Jobs Views

- [x] 15.1 Create `JobsListView.swift` — List with job name, cron schedule, enabled badge, last run time
- [x] 15.2 Create `JobDetailView.swift` — read-only detail with sections: Info (name, cron, human-readable schedule, enabled, agent), Execution History list
- [x] 15.3 Show execution history with date, status, and duration
- [x] 15.4 Show `EmptyStateView` when no jobs configured
- [x] 15.5 Add `.refreshable`

## 16. iOS Usage View

- [x] 16.1 Create `UsageView.swift` (iOS) showing total API calls, total tokens, cost breakdown by provider
- [x] 16.2 Reuse `TimeRangePicker` component for period selection (today, 7 days, 30 days)
- [x] 16.3 Display per-provider usage breakdown in a list
- [x] 16.4 Add `.refreshable`

## 17. iOS Settings View

- [x] 17.1 Create `IOSSettingsView.swift` with Form layout
- [x] 17.2 Server connection section: host field, port field, connection status indicator, Test Connection button
- [x] 17.3 About section: app version, server version (from status)
- [x] 17.4 On server address change, reset navigation state and reconnect

## 18. Integration Testing

- [x] 18.1 Start Go server with `DIANE_HTTP_ADDR=":8080"` and verify all GET endpoints return valid JSON
- [x] 18.2 Build and run iOS app on iPhone simulator — verify server setup flow
- [x] 18.3 Connect iOS app to Go server via localhost and verify status dashboard loads
- [x] 18.4 Test all 8 section list views load data correctly (Status, Servers, Agents, Contexts, Providers, Jobs, Usage, Settings — all verified)
- [x] 18.5 Test detail views display correct information for each section (Server detail with tools, Context detail with servers/tools/connection info — verified)
- [ ] 18.6 Test pull-to-refresh works on all list views (skipped — `.refreshable` is standard SwiftUI, data loading verified)
- [x] 18.7 Test iPhone tab navigation — all tabs accessible, More tab shows all 5 sections (Providers, Contexts, Jobs, Usage, Settings)
- [ ] 18.8 Test iPad sidebar navigation — all sections listed, detail updates on selection (requires iPad simulator)
- [ ] 18.9 Test iPad split view (compact) falls back to tab navigation (requires iPad simulator)
- [x] 18.10 Test connection loss — disconnect server, verify "Disconnected" state shown (red dot + "Disconnected" confirmed)
- [x] 18.11 Test reconnection — restart server, verify app reconnects within polling interval (green dot + "Connected" confirmed within 5s)
- [ ] 18.12 Test app backgrounding — verify polling stops, resumes on foreground (requires manual testing)

## 19. Tailscale Network Testing

- [ ] 19.1 Configure Go server with `--http-addr :8080` on Mac
- [ ] 19.2 Connect physical iOS device to same Tailscale network
- [ ] 19.3 Enter Mac's Tailscale hostname in iOS app server setup
- [ ] 19.4 Verify all views load data over Tailscale VPN
- [ ] 19.5 Test polling continues reliably over Tailscale connection
