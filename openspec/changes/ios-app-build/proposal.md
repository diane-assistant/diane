## Why

Diane currently runs exclusively as a macOS desktop application, communicating with a local Go daemon via Unix socket. Users who manage MCP servers, AI providers, agents, and scheduled jobs have no way to monitor or review their configuration from an iPhone or iPad. An iOS companion app would give users on-the-go visibility into their Diane setup — checking server status, reviewing agent configurations, and monitoring usage — without needing to be at their Mac.

The initial iOS version will be read-only, providing a monitoring and inspection interface that connects to a Diane server over the network (HTTP/HTTPS). This reduces scope and risk while validating the architecture for a future read-write version.

## What Changes

- Create a new iOS app target (iPhone + iPad) in the Diane Xcode project
- Implement a network-based API client that communicates with the Diane server over HTTP/HTTPS instead of Unix socket
- Expose an HTTP listener on the Diane Go server to serve the same API currently available via Unix socket
- Adapt the existing SwiftUI views for iOS, replacing macOS-specific APIs (AppKit, HSplitView, NSPasteboard, MenuBarExtra) with iOS equivalents (UIKit where needed, NavigationStack, UIPasteboard, tab-based navigation)
- Provide a read-only view of all configuration areas: MCP servers, providers, agents, contexts, scheduled jobs, usage, and settings
- Support adaptive layouts for both iPhone (compact) and iPad (regular) size classes

## Capabilities

### New Capabilities

- `ios-network-client`: HTTP/HTTPS API client for communicating with a remote Diane server, including server address configuration, connection status, and authentication
- `ios-adaptive-navigation`: Tab-based navigation for iPhone and sidebar navigation for iPad, adapting the macOS NavigationSplitView pattern to iOS idioms
- `ios-readonly-dashboard`: Read-only views of all Diane configuration areas (MCP servers, providers, agents, contexts, jobs, usage, settings) optimized for mobile display
- `server-http-listener`: HTTP listener endpoint on the Diane Go server that exposes the existing Unix socket API over the network for remote client access

### Modified Capabilities

- `design-tokens`: Extend DesignTokens to replace `Color(nsColor:)` references with cross-platform color definitions that work on both macOS and iOS

## Impact

- **Diane Go server**: Needs a new HTTP listener to expose the API over the network (currently Unix socket only)
- **Xcode project**: New iOS app target alongside existing macOS target; shared code where possible
- **Shared models**: All `Codable` model types (DianeStatus, MCPServer, Provider, Job, Agent, Context) are already platform-independent and can be shared
- **Components**: Some components (InfoRow, EmptyStateView, TimeRangePicker, OAuthConfigEditor) are portable; others (MasterDetailView, SummaryCard, DetailSection, KeyValueEditor) need macOS-specific color references replaced
- **ViewModels**: Can be largely reused with a protocol-based client abstraction (DianeClientProtocol already exists)
- **Security**: Exposing the API over HTTP introduces authentication and network security requirements that don't exist with the local Unix socket
- **Users**: No impact on existing macOS users; iOS is an additive capability
