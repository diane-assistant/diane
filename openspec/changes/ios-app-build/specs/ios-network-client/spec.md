## ADDED Requirements

### Requirement: HTTP API client communicates with remote Diane server
The iOS app SHALL include an HTTP client that communicates with a Diane server over the network using plain HTTP REST requests.

#### Scenario: Client sends GET request to server
- **WHEN** the app needs to fetch data (e.g., server status, MCP list)
- **THEN** the client sends an HTTP GET request to the configured server address
- **AND** parses the JSON response using the shared `Codable` model types

#### Scenario: Client handles server unreachable
- **WHEN** the client sends a request and the server does not respond within 10 seconds
- **THEN** the client reports a connection error
- **AND** the UI displays a "Server unreachable" state

#### Scenario: Client handles malformed response
- **WHEN** the server returns a response that cannot be decoded
- **THEN** the client reports a decoding error with the response status code
- **AND** the UI displays an error state with a retry option

### Requirement: Server address is configurable
The app SHALL allow the user to configure the Diane server address (host and port).

#### Scenario: User enters server address on first launch
- **WHEN** the app launches with no saved server address
- **THEN** the app presents a server configuration screen
- **AND** the user can enter a hostname or IP address and port number

#### Scenario: User enters Tailscale hostname
- **WHEN** the user enters a Tailscale machine name (e.g., `my-mac`) or full Tailscale domain (e.g., `my-mac.tail12345.ts.net`)
- **THEN** the client resolves the address and connects over the Tailscale VPN network

#### Scenario: Server address is persisted
- **WHEN** the user saves a server address
- **THEN** the address is stored in UserDefaults
- **AND** subsequent app launches use the saved address without prompting

#### Scenario: User changes server address
- **WHEN** the user navigates to Settings and edits the server address
- **THEN** the client disconnects from the current server and reconnects to the new address

### Requirement: Connection status is visible
The app SHALL display the current connection state to the user at all times.

#### Scenario: App is connected
- **WHEN** the client successfully receives a response from the server
- **THEN** the connection status indicator shows "Connected" with a green indicator

#### Scenario: App is disconnected
- **WHEN** the client cannot reach the server
- **THEN** the connection status indicator shows "Disconnected" with a red indicator

#### Scenario: App is connecting
- **WHEN** the client is attempting to reach the server (initial connection or reconnect)
- **THEN** the connection status indicator shows "Connecting..." with a loading indicator

### Requirement: Client polls for status updates
The app SHALL periodically poll the server for updated status information.

#### Scenario: Automatic polling when connected
- **WHEN** the app is connected to a server and in the foreground
- **THEN** the client polls for status updates every 5 seconds

#### Scenario: Polling stops in background
- **WHEN** the app moves to the background
- **THEN** the client stops polling to conserve battery and network resources

#### Scenario: Polling resumes on foreground
- **WHEN** the app returns to the foreground
- **THEN** the client immediately polls for fresh data and resumes the periodic poll cycle

### Requirement: Client conforms to DianeClientProtocol
The iOS network client SHALL implement the existing `DianeClientProtocol` interface to enable code reuse with ViewModels.

#### Scenario: ViewModels use protocol abstraction
- **WHEN** a ViewModel needs to fetch data
- **THEN** it calls methods on `DianeClientProtocol`
- **AND** the iOS network client handles the HTTP transport transparently
