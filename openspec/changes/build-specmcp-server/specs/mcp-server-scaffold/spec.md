## ADDED Requirements

### Requirement: Go project structure
The system SHALL be implemented as a Go module at `server/specmcp/` with a clean package structure: `cmd/specmcp/` for the binary entrypoint, `internal/` for private packages, and `pkg/` for any public interfaces.

#### Scenario: Project initializes successfully
- **WHEN** `go build ./cmd/specmcp/` is run in the project directory
- **THEN** a `specmcp` binary is produced without errors

### Requirement: MCP stdio transport
The system SHALL implement the Model Context Protocol using stdio transport (stdin/stdout), allowing AI coding assistants to launch the server as a subprocess and communicate via JSON-RPC 2.0 messages.

#### Scenario: MCP initialization handshake
- **WHEN** an MCP client sends an `initialize` request with its capabilities
- **THEN** the server responds with its name ("specmcp"), version, and list of supported tools

#### Scenario: Tool listing
- **WHEN** an MCP client sends a `tools/list` request
- **THEN** the server responds with all registered tools, each with a name, description, and JSON Schema for input parameters

### Requirement: Emergent SDK client integration
The system SHALL use the Emergent Go SDK (`github.com/emergent-company/emergent/apps/server-go/pkg/sdk@v0.7.0`) for all graph database operations. The client SHALL be initialized with server URL, API key, and project ID from configuration.

#### Scenario: SDK client connects to Emergent
- **WHEN** the specmcp server starts with valid Emergent configuration
- **THEN** the SDK client is initialized and can perform graph operations (e.g., ListObjects)

#### Scenario: SDK client handles connection failure
- **WHEN** the specmcp server starts but Emergent is unreachable
- **THEN** the server logs an error and responds to MCP tool calls with descriptive error messages rather than crashing

### Requirement: Configuration management
The system SHALL load configuration from environment variables (`EMERGENT_URL`, `EMERGENT_API_KEY`, `EMERGENT_PROJECT_ID`) with optional YAML config file override. The configuration SHALL include Emergent connection settings and MCP server metadata.

#### Scenario: Environment variable configuration
- **WHEN** the server starts with `EMERGENT_URL`, `EMERGENT_API_KEY`, and `EMERGENT_PROJECT_ID` environment variables set
- **THEN** the server connects to the specified Emergent instance using those credentials

#### Scenario: Missing required configuration
- **WHEN** the server starts without `EMERGENT_API_KEY` set
- **THEN** the server exits with a clear error message indicating which configuration is missing

### Requirement: Structured logging
The system SHALL produce structured log output (JSON format) to stderr, keeping stdout reserved for MCP protocol communication. Log levels SHALL be configurable.

#### Scenario: Logs go to stderr
- **WHEN** the server processes an MCP request
- **THEN** any log output appears on stderr while the MCP response appears on stdout

### Requirement: Tool registration framework
The system SHALL provide an internal framework for registering MCP tools with their schemas, handlers, and descriptions. Adding a new tool SHALL require implementing a handler function and registering it — no protocol-level code changes.

#### Scenario: Register a new tool
- **WHEN** a developer creates a new tool handler and registers it
- **THEN** the tool appears in `tools/list` responses and can be invoked via `tools/call`
