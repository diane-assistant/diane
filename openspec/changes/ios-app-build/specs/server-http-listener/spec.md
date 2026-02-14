## ADDED Requirements

### Requirement: Go server exposes HTTP listener
The Diane Go server SHALL expose an HTTP listener that serves the same API endpoints currently available via Unix socket.

#### Scenario: Server starts HTTP listener on configured port
- **WHEN** the Diane server starts with HTTP listener enabled
- **THEN** the server listens on the configured port (default 8080) on all network interfaces
- **AND** logs the HTTP listener address at startup

#### Scenario: HTTP listener is opt-in
- **WHEN** the Diane server starts without explicit HTTP configuration
- **THEN** the HTTP listener is NOT started (Unix socket only, preserving current behavior)

#### Scenario: HTTP listener configured via flag or config
- **WHEN** the user sets `--http-addr :8080` flag or `http_addr: ":8080"` in config
- **THEN** the server starts the HTTP listener on the specified address

### Requirement: HTTP endpoints mirror Unix socket API
The HTTP listener SHALL serve the same request/response format as the Unix socket API.

#### Scenario: GET endpoints return same JSON
- **WHEN** a client sends a GET request to an HTTP endpoint (e.g., `/api/status`)
- **THEN** the response body is identical JSON to what the Unix socket returns for the same endpoint

#### Scenario: All read endpoints are available
- **WHEN** the HTTP listener is running
- **THEN** the following read endpoints are available: status, MCP servers list, MCP server detail, tools list, providers list, provider detail, agents list, agent detail, contexts list, context detail, jobs list, job detail, usage stats

### Requirement: HTTP listener handles errors gracefully
The HTTP listener SHALL return appropriate HTTP status codes for error conditions.

#### Scenario: Unknown endpoint returns 404
- **WHEN** a client requests an endpoint that does not exist
- **THEN** the server returns HTTP 404 with a JSON error body

#### Scenario: Internal error returns 500
- **WHEN** an internal error occurs while processing a request
- **THEN** the server returns HTTP 500 with a JSON error body describing the failure

#### Scenario: Malformed request returns 400
- **WHEN** a client sends a request with invalid parameters
- **THEN** the server returns HTTP 400 with a JSON error body

### Requirement: HTTP listener does not require authentication initially
The HTTP listener SHALL operate without authentication for the initial version, relying on network-level security (Tailscale VPN).

#### Scenario: Unauthenticated request succeeds
- **WHEN** a client sends a request to any endpoint without authentication headers
- **THEN** the server processes the request normally

### Requirement: HTTP listener only serves read operations initially
The HTTP listener SHALL only expose read (GET) endpoints in the initial version. Write operations remain Unix socket only.

#### Scenario: POST/PUT/DELETE requests are rejected
- **WHEN** a client sends a POST, PUT, or DELETE request to the HTTP listener
- **THEN** the server returns HTTP 405 Method Not Allowed
- **AND** the response includes an `Allow: GET` header

### Requirement: HTTP and Unix socket listeners coexist
The HTTP listener SHALL run alongside the existing Unix socket listener without interference.

#### Scenario: Both listeners serve requests concurrently
- **WHEN** both HTTP and Unix socket listeners are active
- **THEN** requests on either transport are handled independently
- **AND** both return consistent data from the same underlying state

#### Scenario: Unix socket continues to accept write operations
- **WHEN** the HTTP listener is running
- **THEN** the Unix socket listener continues to accept all operations (read and write) as before
