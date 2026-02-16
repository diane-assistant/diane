# Distributed MCP Proxy - Implementation Progress

## ✅ Completed Components

### 1. Database Schema (server/internal/db/db.go)
**Status**: ✅ Complete

Added three new tables to the migration:

```sql
-- Slave servers for distributed MCP
CREATE TABLE IF NOT EXISTS slave_servers (
    id TEXT PRIMARY KEY,
    host_id TEXT UNIQUE NOT NULL,
    cert_serial TEXT NOT NULL,
    issued_at DATETIME NOT NULL,
    expires_at DATETIME NOT NULL,
    last_seen DATETIME,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Revoked slave credentials
CREATE TABLE IF NOT EXISTS revoked_slave_credentials (
    id TEXT PRIMARY KEY,
    host_id TEXT NOT NULL,
    cert_serial TEXT NOT NULL,
    revoked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    reason TEXT
);

-- Pending pairing requests
CREATE TABLE IF NOT EXISTS pairing_requests (
    id TEXT PRIMARY KEY,
    host_id TEXT NOT NULL,
    pairing_code TEXT NOT NULL,
    csr TEXT NOT NULL,
    requested_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
);
```

### 2. Database Operations (server/internal/db/slave_servers.go)
**Status**: ✅ Complete

Implemented full CRUD operations:

**Slave Server Operations**:
- `CreateSlaveServer(hostID, certSerial, issuedAt, expiresAt)` - Create new slave
- `GetSlaveServerByHostID(hostID)` - Retrieve slave by ID
- `ListSlaveServers()` - Get all slaves
- `UpdateSlaveLastSeen(hostID)` - Update heartbeat timestamp
- `DeleteSlaveServer(hostID)` - Remove slave

**Credential Revocation**:
- `RevokeSlaveCredential(hostID, certSerial, reason)` - Revoke a slave's cert
- `IsCredentialRevoked(certSerial)` - Check if cert is revoked
- `ListRevokedCredentials()` - View all revoked certs

**Pairing Management**:
- `CreatePairingRequest(hostID, pairingCode, csr, expiresAt)` - Store pairing request
- `GetPairingRequest(hostID, pairingCode)` - Retrieve pairing request
- `ListPendingPairingRequests()` - Get all pending requests
- `UpdatePairingRequestStatus(hostID, pairingCode, status)` - Update status
- `CleanupExpiredPairingRequests()` - Remove expired requests

### 3. Certificate Authority (server/internal/slave/ca.go)
**Status**: ✅ Complete

Full-featured CA for managing slave certificates:

**Features**:
- Self-signed CA generation (10-year validity)
- Persists CA cert and key to `~/.diane/slave-ca-cert.pem` and `~/.diane/slave-ca-key.pem`
- Loads existing CA on startup if present
- CSR signing with configurable validity period
- Client certificate verification
- Expiration checking

**Key Functions**:
- `NewCertificateAuthority(dataDir)` - Create or load CA
- `SignCSR(csrPEM, hostID, validDays)` - Sign a slave's CSR
- `GetCACertPEM()` - Get CA cert for slaves
- `VerifyClientCert(certPEM)` - Verify slave certificates

**Security**:
- 4096-bit RSA keys
- Proper key usage flags (DigitalSignature, KeyEncipherment)
- Extended key usage for client authentication
- Certificate chain verification

### 4. Pairing Service (server/internal/slave/pairing.go)
**Status**: ✅ Complete

Manages the entire pairing workflow:

**Features**:
- 6-digit pairing code generation (format: "123-456")
- 10-minute expiration for pairing codes
- In-memory + database persistence
- Notification channel for UI/CLI updates
- Automatic cleanup of expired requests

**Key Functions**:
- `GeneratePairingCode()` - Generate random 6-digit code
- `CreatePairingRequest(hostID, csrPEM)` - Initiate pairing
- `ApprovePairingRequest(hostID, pairingCode)` - Sign certificate and approve
- `DenyPairingRequest(hostID, pairingCode)` - Reject pairing
- `GetPendingRequests()` - List all pending requests
- `GetNotificationChannel()` - Subscribe to pairing events

**Notifications**:
- `new` - New pairing request created
- `approved` - Pairing approved, cert issued
- `denied` - Pairing denied by admin
- `expired` - Pairing code expired

---

## 🚧 Remaining Components

### 5. Slave Registry (server/internal/slave/registry.go)
**Status**: ⏳ Pending

**Purpose**: Manage connected slave servers in memory

**Required Features**:
- Track active WebSocket connections
- Store slave metadata (host_id, tools, status)
- Handle slave connection/disconnection events
- Heartbeat monitoring
- Tool registration from slaves

**Suggested Structure**:
```go
type Registry struct {
    connections map[string]*SlaveConnection // hostID -> connection
    mu          sync.RWMutex
    db          *db.DB
}

type SlaveConnection struct {
    HostID        string
    Conn          *websocket.Conn
    Tools         []map[string]interface{}
    Status        ConnectionStatus
    LastHeartbeat time.Time
    Client        *mcpproxy.WSClient
}
```

### 6. WebSocket Slave Server (server/internal/slave/server.go)
**Status**: ⏳ Pending

**Purpose**: Accept WebSocket connections from slave Diane instances

**Required Features**:
- WebSocket endpoint: `wss://master:8765/slave/connect`
- mTLS authentication using CA
- Handle slave registration messages
- Route tool calls to slaves
- Send heartbeat pings
- Detect disconnections

**Integration Points**:
- Use `CertificateAuthority` to verify client certs
- Use `Registry` to track connections
- Check `db.IsCredentialRevoked()` before accepting connections

### 7. WebSocket MCP Client (server/internal/mcpproxy/ws_client.go)
**Status**: ⏳ Pending

**Purpose**: Client that connects master to slaves via WebSocket

**Required Features**:
- Implement `mcpproxy.Client` interface
- WebSocket connection management
- MCP protocol over WebSocket (tools/list, tools/call)
- Automatic reconnection with exponential backoff
- Handle connection errors

**Suggested Interface Implementation**:
```go
type WSClient struct {
    url        string
    conn       *websocket.Conn
    mu         sync.Mutex
    connected  bool
    notifyChan chan string
}

func (c *WSClient) ListTools() ([]map[string]interface{}, error)
func (c *WSClient) CallTool(toolName string, arguments map[string]interface{}) (json.RawMessage, error)
func (c *WSClient) IsConnected() bool
func (c *WSClient) Close() error
```

### 8. Update MCP Proxy (server/internal/mcpproxy/proxy.go)
**Status**: ⏳ Pending

**Required Changes**:
1. Add "remote" transport type to `ServerConfig`
2. Instantiate `WSClient` for remote slaves
3. Maintain existing prefix logic (already works with `hostname_toolname` pattern!)

**Note**: The existing proxy code already implements the `hostname_` prefix pattern we need! Lines 179-186 and 197-217 handle this perfectly.

### 9. Update Tool Listing (server/mcp/server.go)
**Status**: ⏳ Pending

**Required Changes**:
- Query `Registry` for connected slaves
- Aggregate slave tools with `hostname_` prefix
- Existing proxy code will handle this if slaves are added as clients

**Example**:
```go
// In listTools():
// 1. Get tools from proxy (includes external MCP servers)
proxyTools := proxy.ListAllTools()

// 2. Get tools from slave registry
for _, slave := range slaveRegistry.GetConnectedSlaves() {
    for _, tool := range slave.Tools {
        tool["name"] = slave.HostID + "_" + tool["name"]
        proxyTools = append(proxyTools, tool)
    }
}
```

### 10. CLI Commands (server/cmd/slave.go)
**Status**: ⏳ Pending

**Required Commands**:

**Master Commands**:
```bash
diane slave pending               # List pending pairing requests
diane slave approve <host> <code> # Approve pairing
diane slave deny <host> <code>    # Deny pairing
diane slave list                  # List all slaves (connected/disconnected)
diane slave revoke <host>         # Revoke slave credentials
diane slave revoke --all          # Revoke all slaves
diane slave revoked               # List revoked credentials
```

**Slave Commands**:
```bash
diane slave pair --master=<url> --host-id=<name>  # Initiate pairing
diane slave start                                  # Start slave server
diane slave stop                                   # Stop slave server
```

### 11. Revocation Logic (server/internal/slave/revocation.go)
**Status**: ⏳ Pending

**Required Features**:
- Check CRL on every connection attempt
- Force disconnect revoked slaves
- Add reason to revocation log
- Cleanup revoked credentials (optional)

**Integration Points**:
- Called by WebSocket server before accepting connection
- Called by CLI `slave revoke` command
- Used by Registry to disconnect active slaves

### 12. Pairing API Endpoints (server/internal/api/slave.go)
**Status**: ⏳ Pending

**Required Endpoints**:
```
POST /api/slave/pair              # Slave initiates pairing
POST /api/slave/approve           # Admin approves pairing
POST /api/slave/deny              # Admin denies pairing
GET  /api/slave/pending           # List pending requests
GET  /api/slave/health            # Health status of all slaves
POST /api/slave/revoke            # Revoke slave credentials
GET  /api/slave/revoked           # List revoked credentials
```

---

## 📊 Implementation Status

| Component | Status | Lines of Code | Priority |
|-----------|--------|---------------|----------|
| Database Schema | ✅ Complete | ~50 | High |
| Database Operations | ✅ Complete | ~300 | High |
| Certificate Authority | ✅ Complete | ~260 | High |
| Pairing Service | ✅ Complete | ~250 | High |
| **Slave Registry** | ⏳ Pending | ~200 | **High** |
| **WebSocket Server** | ⏳ Pending | ~400 | **High** |
| **WebSocket Client** | ⏳ Pending | ~300 | **High** |
| **MCP Proxy Updates** | ⏳ Pending | ~50 | **High** |
| **Tool Listing Updates** | ⏳ Pending | ~30 | **High** |
| **CLI Commands** | ⏳ Pending | ~500 | High |
| **Revocation Logic** | ⏳ Pending | ~100 | Medium |
| **API Endpoints** | ⏳ Pending | ~400 | Medium |

**Total Progress**: ~1,110 / ~2,840 LOC (**39% complete**)

---

## 🔧 Next Steps

### Immediate Priority (Core Functionality):

1. **Slave Registry** - Required for tracking connections
2. **WebSocket Server** - Required for accepting slave connections
3. **WebSocket Client** - Required for communicating with slaves
4. **MCP Proxy Integration** - Wire up registry to proxy
5. **Tool Listing** - Expose slave tools to agents

### Secondary Priority (User Interface):

6. **CLI Commands** - User management interface
7. **API Endpoints** - HTTP API for pairing/management
8. **Revocation Logic** - Security enforcement

### Testing Priority:

9. **Unit Tests** - Test individual components
10. **Integration Tests** - Test full pairing flow
11. **E2E Tests** - Test agent calling slave tools

---

## 🎯 Testing Scenarios

### Scenario 1: Basic Pairing and Connection

**Context**: Admin wants to add a remote workstation as a slave

**Steps**:
1. On workstation: `diane slave pair --master=https://main:8765 --host-id=workstation`
2. Workstation generates CSR, creates pairing request, shows code "123-456"
3. On master: `diane slave pending` shows workstation with code
4. On master: `diane slave approve workstation 123-456`
5. Master signs CSR, returns certificate to workstation
6. Workstation connects via WebSocket with mTLS
7. Workstation registers its tools (e.g., file_registry_search)
8. On master: `diane slave list` shows workstation as "connected"

**Expected Result**: Workstation is connected and ready for tool calls

### Scenario 2: Agent Calls Tool on Remote Slave

**Context**: AI agent needs to search files on the workstation

**Steps**:
1. Agent queries master: `tools/list`
2. Master returns tools including `workstation_file_registry_search`
3. Agent calls `workstation_file_registry_search` with query="proposal"
4. Master's mcpproxy parses prefix: hostname="workstation", tool="file_registry_search"
5. Master routes call to workstation's WebSocket connection
6. Workstation executes local file_registry_search
7. Workstation returns results to master
8. Master forwards results to agent

**Expected Result**: Agent receives files from workstation

### Scenario 3: Slave Disconnection and Reconnection

**Context**: Workstation loses network connection

**Steps**:
1. Workstation's network drops
2. Master's WebSocket server detects disconnection
3. Registry marks workstation as "disconnected"
4. Agent attempts to call `workstation_file_registry_search`
5. Master returns error: "Host workstation is not available"
6. Workstation's network recovers
7. Workstation reconnects with saved credentials (mTLS)
8. Master verifies certificate, accepts connection
9. Registry marks workstation as "connected"
10. Agent can now call tools on workstation again

**Expected Result**: Automatic reconnection without re-pairing

### Scenario 4: Credential Revocation

**Context**: Admin needs to revoke compromised slave credentials

**Steps**:
1. Admin runs: `diane slave revoke workstation --reason="Compromised"`
2. Master adds certificate to revocation list
3. Master immediately closes workstation's WebSocket connection
4. Workstation attempts to reconnect
5. Master checks CRL, finds certificate revoked
6. Master rejects connection with "credentials revoked"
7. Workstation shows error: "Credentials revoked. Please re-pair with master"
8. Admin runs: `diane slave revoked` to see revocation log

**Expected Result**: Workstation cannot reconnect until re-paired

### Scenario 5: Pairing Code Expiration

**Context**: User generates pairing code but doesn't approve in time

**Steps**:
1. Workstation generates pairing request, code "456-789"
2. Code expires after 10 minutes
3. Admin attempts: `diane slave approve workstation 456-789`
4. Master returns error: "Pairing request expired"
5. Workstation must generate new pairing request

**Expected Result**: Expired codes cannot be used

---

## 📝 Implementation Notes

### Import Path Issue
**Fixed**: Changed import from `github.com/mcncl/diane/server/internal/db` to `github.com/diane-assistant/diane/internal/db`

### Dependencies Required
- `github.com/gorilla/websocket` - For WebSocket communication
- `github.com/golang-jwt/jwt/v5` - Already in go.mod ✅
- `github.com/google/uuid` - Already in go.mod ✅
- Standard library crypto packages ✅

### Configuration Storage
All slave-related data is stored in:
- Database: `~/.diane/cron.db` (SQLite)
- CA Certificate: `~/.diane/slave-ca-cert.pem`
- CA Private Key: `~/.diane/slave-ca-key.pem` (permissions: 0600)
- Slave credentials (on slave): `~/.diane/slave-credentials.json`

### Backward Compatibility
- No configuration required for single-instance Diane
- Schema changes are additive (new tables only)
- Existing MCP proxy code already supports the `hostname_` prefix pattern
- No changes to existing API contracts

---

## 🚀 Ready to Continue

The foundation is complete! The next critical components are:

1. **Slave Registry** - ~200 LOC, straightforward
2. **WebSocket Server** - ~400 LOC, most complex component
3. **WebSocket Client** - ~300 LOC, implements existing interface

With these three components, the core distributed MCP functionality will be operational.

**Estimated completion time for remaining work**: 6-8 hours of focused development

---

**Document Version**: 1.0  
**Last Updated**: 2026-02-16  
**Implementation Progress**: 39% Complete
