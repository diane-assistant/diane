# Distributed MCP Proxy System - Specification

**Change ID**: `distributed-mcp-proxy`  
**Status**: Design Phase  
**Priority**: High  
**Complexity**: High

## Overview

This specification defines a distributed architecture for Diane where **slave servers** on different machines can connect to a **master Diane instance**, enabling AI agents to execute MCP tools on specific remote hosts while maintaining a simple, secure pairing flow.

---

## Key Design Principles

Based on industry best practices from Bluetooth/IoT pairing, Kubernetes node registration, and zero-configuration networking:

1. **Simple Pairing Flow**: Inspired by Bluetooth numeric comparison - slave initiates connection, master confirms
2. **Automatic Certificate Exchange**: Following Kubernetes TLS bootstrapping pattern
3. **Zero Manual Configuration**: Like mDNS/Bonjour - minimize user setup steps
4. **Consistent Naming**: Use existing Diane convention `hostname_toolname`
5. **Key Revocation**: Enable master to invalidate slave credentials

---

## Tool Naming Convention (UPDATED)

### Decision: Use Existing Underscore Pattern

**Format**: `{hostname}_{tool_name}`

**Examples**:
- `workstation_file_registry_search`
- `server_cloudflare_dns_add`
- `laptop_reminders_add`

**Rationale**:
- ✅ Consistent with existing external MCP server naming (e.g., `filesystem_read_file`)
- ✅ No changes needed to existing prefix parsing logic (server/internal/mcpproxy/proxy.go:197-206)
- ✅ Familiar to existing Diane users
- ✅ Simple underscore-based parsing

**Implementation**: Reuse existing `serverName + "_" + toolName` pattern in mcpproxy/proxy.go

---

## Simplified Pairing Flow

### Design Inspiration
- **Bluetooth Numeric Comparison**: User confirms matching code on both devices
- **Kubernetes TLS Bootstrap**: Automated certificate exchange after initial token
- **Home IoT Pairing**: Simple "press button to pair" UX

### Pairing Process

```
┌─────────────┐                           ┌─────────────┐
│   SLAVE     │                           │   MASTER    │
│ (workstation)│                           │  (main)     │
└─────────────┘                           └─────────────┘
      │                                           │
      │ 1. diane slave pair --master=https://... │
      │──────────────────────────────────────────>│
      │        [Generate pairing code: 123-456]   │
      │                                           │
      │                                           │ 2. Show pairing request
      │                                           │    in CLI/UI:
      │                                           │    ┌──────────────────┐
      │                                           │    │ Pairing Request  │
      │                                           │    │ Code: 123-456    │
      │                                           │    │ From: workstation│
      │                                           │    │ [Confirm] [Deny] │
      │                                           │    └──────────────────┘
      │                                           │
      │     3. Admin confirms pairing             │
      │<──────────────────────────────────────────│
      │        [Token + Certificate issued]       │
      │                                           │
      │ 4. Save credentials locally               │
      │    Connect with WebSocket                 │
      │──────────────────────────────────────────>│
      │                                           │
      │ 5. Register tools                         │
      │──────────────────────────────────────────>│
      │                                           │
      │<──────────────────────────────────────────│
      │        [Connected & ready]                │
```

### User Experience

**On Slave Machine**:
```bash
$ diane slave pair --master=https://master.example.com:8765 --host-id=workstation

Connecting to master...
Pairing code: 123-456

Waiting for confirmation from master...
(Admin must approve this pairing request on the master)

✓ Pairing successful!
✓ Credentials saved to ~/.diane/slave-credentials.json
✓ Connecting to master...
✓ Connected! Registered 12 tools.

To start the slave server:
  diane slave start
```

**On Master Machine** (CLI):
```bash
$ diane slave pending

Pending pairing requests:
1. Host: workstation
   Code: 123-456
   Requested: 2 minutes ago

$ diane slave approve workstation 123-456

✓ Approved pairing for 'workstation'
✓ Certificate issued (expires in 365 days)
✓ Slave connected and registered 12 tools
```

**On Master Machine** (UI):
- Desktop notification: "New slave pairing request from 'workstation'"
- Confirmation dialog with pairing code
- Click "Approve" → slave connects automatically

---

## Security Model

### Pairing Code Generation
```go
// 6-digit numeric code, similar to Bluetooth PIN
// Easy to read over phone/screen share
code := generatePairingCode() // e.g., "123-456"
```

### Certificate-Based Authentication (After Pairing)

Following **Kubernetes TLS Bootstrapping** pattern:

1. **Pairing Request**: Slave generates ephemeral key pair, sends CSR + pairing code
2. **Master Approval**: Admin confirms pairing code
3. **Certificate Issuance**: Master signs CSR, returns:
   - Client certificate (for authentication)
   - JWT refresh token (for credential rotation)
   - CA certificate (for verifying master)
4. **Persistent Connection**: Slave saves credentials, connects with mTLS

### Certificate Details
```go
type SlaveCredentials struct {
    HostID           string    `json:"host_id"`
    ClientCert       string    `json:"client_cert"`       // PEM-encoded
    ClientKey        string    `json:"client_key"`        // PEM-encoded
    CACert           string    `json:"ca_cert"`           // For verifying master
    RefreshToken     string    `json:"refresh_token"`     // JWT for rotation
    IssuedAt         time.Time `json:"issued_at"`
    ExpiresAt        time.Time `json:"expires_at"`
}
```

### JWT Refresh Token (for Rotation)
```json
{
  "host_id": "workstation",
  "permissions": ["tools:register", "tools:execute"],
  "exp": 1735689600,
  "iat": 1704067200,
  "iss": "diane-master-abc123"
}
```

---

## Key Invalidation Feature

### Use Cases
- Slave machine is compromised
- Machine decommissioned
- Need to revoke access temporarily
- Certificate expired and renewal failed

### CLI Commands

```bash
# List all slaves and their credential status
$ diane slave list
┌────────────┬───────────┬─────────────────┬────────────────┐
│ Host ID    │ Status    │ Last Heartbeat  │ Cert Expires   │
├────────────┼───────────┼─────────────────┼────────────────┤
│ workstation│ connected │ 2s ago          │ 2027-02-16     │
│ server     │ connected │ 5s ago          │ 2027-02-16     │
│ laptop     │ offline   │ 2h ago          │ 2026-12-01     │
└────────────┴───────────┴─────────────────┴────────────────┘

# Revoke credentials for a slave
$ diane slave revoke workstation

⚠  This will immediately disconnect 'workstation' and invalidate its credentials.
   The slave will need to re-pair to reconnect.

Are you sure? [y/N]: y

✓ Revoked credentials for 'workstation'
✓ Slave disconnected
✓ Certificate added to revocation list

# Revoke all slaves (emergency)
$ diane slave revoke --all

⚠  This will disconnect ALL slaves immediately!
Are you sure? [y/N]: y

✓ Revoked 3 slave credentials
✓ All slaves disconnected

# View revocation list
$ diane slave revoked
┌────────────┬─────────────────┬────────────────────┐
│ Host ID    │ Revoked At      │ Reason             │
├────────────┼─────────────────┼────────────────────┤
│ old-laptop │ 2026-02-10      │ Machine retired    │
│ test-vm    │ 2026-02-01      │ Testing completed  │
└────────────┴─────────────────┴────────────────────┘
```

### Implementation

**Certificate Revocation List (CRL)**:
```go
type RevokedCredential struct {
    HostID        string    `json:"host_id"`
    CertSerial    string    `json:"cert_serial"`
    RevokedAt     time.Time `json:"revoked_at"`
    Reason        string    `json:"reason"`
}

// Stored in database: revoked_slave_credentials table
// Checked on every WebSocket connection attempt
```

**Revocation Process**:
1. Admin runs `diane slave revoke <host-id>`
2. Master adds certificate to CRL in database
3. Master immediately closes WebSocket connection to slave
4. Slave's next connection attempt is rejected
5. Slave shows error: "Credentials revoked. Please re-pair with master."

**Re-pairing After Revocation**:
- Slave must go through full pairing flow again
- Old certificate cannot be reused
- New pairing code generated

---

## Architecture Components

### 1. Slave Manager Service
**Location**: `server/internal/slave/manager.go`

**Responsibilities**:
- Accept WebSocket connections from slaves
- Manage pairing requests and approval workflow
- Authenticate slave connections using mTLS
- Maintain registry of connected slaves
- Monitor health via heartbeats
- Handle certificate revocation

### 2. WebSocket Client (Master → Slave)
**Location**: `server/internal/mcpproxy/ws_client.go`

**Responsibilities**:
- Establish WebSocket connection to slave
- Send MCP protocol messages (tools/list, tools/call)
- Handle connection failures and retry logic
- Maintain persistent connection with heartbeat

### 3. Pairing Service
**Location**: `server/internal/slave/pairing.go`

**Responsibilities**:
- Generate pairing codes
- Manage pending pairing requests
- Issue certificates after approval
- Send notifications to admin (UI/CLI)

### 4. Certificate Authority
**Location**: `server/internal/slave/ca.go`

**Responsibilities**:
- Generate master CA certificate (once on first run)
- Sign slave CSRs after approval
- Maintain certificate revocation list
- Handle certificate expiration and renewal

---

## Data Models

### SlaveServerConfig (Database)
```go
type SlaveServerConfig struct {
    ID           string    `json:"id" db:"id"`
    HostID       string    `json:"host_id" db:"host_id"`           // Unique identifier
    CertSerial   string    `json:"cert_serial" db:"cert_serial"`   // Certificate serial number
    IssuedAt     time.Time `json:"issued_at" db:"issued_at"`
    ExpiresAt    time.Time `json:"expires_at" db:"expires_at"`
    LastSeen     time.Time `json:"last_seen" db:"last_seen"`       // Last heartbeat
    Enabled      bool      `json:"enabled" db:"enabled"`
    CreatedAt    time.Time `json:"created_at" db:"created_at"`
    UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}
```

### PairingRequest (In-Memory)
```go
type PairingRequest struct {
    HostID      string    `json:"host_id"`
    PairingCode string    `json:"pairing_code"`    // 6-digit code
    CSR         []byte    `json:"csr"`             // Certificate signing request
    RequestedAt time.Time `json:"requested_at"`
    ExpiresAt   time.Time `json:"expires_at"`      // Pairing code expires after 10 minutes
}
```

### SlaveConnection (Runtime State)
```go
type SlaveConnection struct {
    HostID        string              `json:"host_id"`
    Connection    *websocket.Conn     `json:"-"`
    Tools         []map[string]interface{} `json:"tools"`
    Status        ConnectionStatus    `json:"status"`       // connected, disconnected, reconnecting
    LastHeartbeat time.Time          `json:"last_heartbeat"`
    ToolCount     int                `json:"tool_count"`
    Client        *mcpproxy.WSClient `json:"-"`
}
```

### RevokedCredential (Database)
```go
type RevokedCredential struct {
    ID         string    `json:"id" db:"id"`
    HostID     string    `json:"host_id" db:"host_id"`
    CertSerial string    `json:"cert_serial" db:"cert_serial"`
    RevokedAt  time.Time `json:"revoked_at" db:"revoked_at"`
    Reason     string    `json:"reason" db:"reason"`
}
```

---

## API Contracts

### 1. Pairing Request
**Endpoint**: `POST /api/slave/pair`

**Request**:
```json
{
  "host_id": "workstation",
  "csr": "-----BEGIN CERTIFICATE REQUEST-----\n...",
  "pairing_code": "123-456"
}
```

**Response** (pending approval):
```json
{
  "status": "pending",
  "message": "Pairing request submitted. Waiting for master approval.",
  "expires_at": "2026-02-16T10:40:00Z"
}
```

**Response** (approved):
```json
{
  "status": "approved",
  "credentials": {
    "host_id": "workstation",
    "client_cert": "-----BEGIN CERTIFICATE-----\n...",
    "client_key": "-----BEGIN PRIVATE KEY-----\n...",
    "ca_cert": "-----BEGIN CERTIFICATE-----\n...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
    "expires_at": "2027-02-16T10:30:00Z"
  },
  "ws_url": "wss://master.example.com:8765/slave/connect"
}
```

### 2. Slave Connection (WebSocket)
**Endpoint**: `wss://master:8765/slave/connect`

**Authentication**: mTLS (client certificate)

**Initial Handshake**:
```json
{
  "jsonrpc": "2.0",
  "method": "slave/register",
  "params": {
    "host_id": "workstation",
    "tools": [
      {
        "name": "file_registry_search",
        "description": "Search for files in registry",
        "inputSchema": {...}
      }
    ]
  },
  "id": 1
}
```

### 3. Pairing Approval (Master API)
**Endpoint**: `POST /api/slave/approve`

**Request**:
```json
{
  "host_id": "workstation",
  "pairing_code": "123-456"
}
```

**Response**:
```json
{
  "success": true,
  "message": "Slave 'workstation' approved and connected",
  "certificate_expires": "2027-02-16T10:30:00Z"
}
```

### 4. Credential Revocation
**Endpoint**: `POST /api/slave/revoke`

**Request**:
```json
{
  "host_id": "workstation",
  "reason": "Machine compromised"
}
```

**Response**:
```json
{
  "success": true,
  "message": "Credentials revoked for 'workstation'",
  "disconnected": true
}
```

### 5. Health Check
**Endpoint**: `GET /api/slave/health`

**Response**:
```json
{
  "total_slaves": 3,
  "connected": 2,
  "disconnected": 1,
  "slaves": [
    {
      "host_id": "workstation",
      "status": "connected",
      "last_heartbeat": "2s ago",
      "tool_count": 12,
      "cert_expires": "2027-02-16"
    },
    {
      "host_id": "server",
      "status": "connected",
      "last_heartbeat": "5s ago",
      "tool_count": 8,
      "cert_expires": "2027-02-16"
    },
    {
      "host_id": "laptop",
      "status": "disconnected",
      "last_heartbeat": "2h ago",
      "tool_count": 0,
      "cert_expires": "2026-12-01"
    }
  ]
}
```

---

## Database Schema Changes

### New Tables

```sql
-- Slave server configurations
CREATE TABLE slave_servers (
    id TEXT PRIMARY KEY,
    host_id TEXT UNIQUE NOT NULL,
    cert_serial TEXT NOT NULL,
    issued_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    last_seen TIMESTAMP,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Revoked slave credentials
CREATE TABLE revoked_slave_credentials (
    id TEXT PRIMARY KEY,
    host_id TEXT NOT NULL,
    cert_serial TEXT NOT NULL,
    revoked_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    reason TEXT
);

-- Pending pairing requests (in-memory, but optional persistence)
CREATE TABLE pairing_requests (
    id TEXT PRIMARY KEY,
    host_id TEXT NOT NULL,
    pairing_code TEXT NOT NULL,
    csr TEXT NOT NULL,
    requested_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    status TEXT DEFAULT 'pending'  -- pending, approved, denied, expired
);
```

---

## Implementation Phases

### Phase 1: Core Pairing Flow (High Priority)
1. Implement pairing request endpoint
2. Create pairing code generation
3. Build approval workflow (CLI + UI)
4. Generate and issue certificates
5. Store slave configuration in database

### Phase 2: Connection & Registry (High Priority)
1. WebSocket server for slave connections
2. mTLS authentication
3. Slave registry (in-memory state)
4. Heartbeat mechanism
5. Tool registration from slaves

### Phase 3: Tool Routing (High Priority)
1. Extend mcpproxy with WebSocket client
2. Update tool listing with `hostname_` prefix
3. Parse prefix and route to correct slave
4. Handle offline slave errors

### Phase 4: Revocation & Management (Medium Priority)
1. Implement revocation API
2. Certificate revocation list
3. `diane slave revoke` CLI command
4. Disconnect logic on revocation
5. Re-pairing flow

### Phase 5: Polish & Monitoring (Low Priority)
1. Desktop notifications for pairing requests
2. Health monitoring dashboard
3. Certificate renewal automation
4. Metrics and logging
5. Documentation

---

## Success Criteria

✅ User can pair slave with master using single command  
✅ Admin confirms pairing with simple approval (code matching)  
✅ Certificate exchange happens automatically behind the scenes  
✅ Slave tools appear with `hostname_` prefix  
✅ Agent can call tools on specific slaves  
✅ Admin can revoke slave credentials instantly  
✅ System handles offline slaves gracefully  
✅ Backward compatible with single-instance Diane  

---

## Security Considerations

1. **Pairing Code**: 6-digit numeric, expires after 10 minutes, one-time use
2. **Certificate Lifetime**: 365 days default, renewable via refresh token
3. **Transport Security**: WSS (WebSocket Secure) with TLS 1.3
4. **Mutual Authentication**: mTLS ensures both master and slave verify each other
5. **Revocation**: Immediate disconnection, certificate CRL check on every connection
6. **Credential Storage**: Stored in `~/.diane/slave-credentials.json` with 0600 permissions

---

## References

- [MCP Specification](https://modelcontextprotocol.io/)
- [Kubernetes TLS Bootstrapping](https://kubernetes.io/docs/reference/access-authn-authz/kubelet-tls-bootstrapping/)
- [Bluetooth Secure Simple Pairing](https://en.wikipedia.org/wiki/Bluetooth#Pairing_mechanisms)
- [Zero-Configuration Networking](https://en.wikipedia.org/wiki/Zero-configuration_networking)
- Diane MCP Proxy: `server/internal/mcpproxy/proxy.go`

---

**Document Version**: 1.0  
**Last Updated**: 2026-02-16  
**Status**: Ready for Implementation
