# Development Environment Topology

## Overview

The Diane project operates in a distributed multi-node development environment connected via a private network (Tailscale/ZeroTier). This document describes the architecture, node roles, connectivity details, and operational procedures.

## Network Architecture

**Network Type:** Private mesh network (Tailscale/ZeroTier)  
**IP Range:** `100.x.x.x`  
**Topology:** Master-slave architecture with 1 master and 2 slave nodes

## Node Inventory

### Master Node: `mcj-emergent`

- **Hostname:** `mcj-emergent`
- **IP Address:** `100.71.82.7`
- **User:** `root@100.71.82.7`
- **Platform:** Arch Linux (6.18.9-arch1-2)
- **Role:** Primary development and service orchestration node
- **Status:** ✅ Operational
- **Diane Version:** `dev`
- **Location:** `/root` (git repo root)

**Services Running:**
- Docker containers:
  - `emergent-server` — Main application server (currently unhealthy)
  - `emergent-db` — Database service (healthy)
  - `emergent-kreuzberg` — Kreuzberg service (healthy)
  - `emergent-minio` — Object storage (healthy)
- Diane server process (PID: 1153477) running as `diane serve`

**Notes:**
- Arch Linux system with pacman package manager
- Diane built from source (version: `dev`)
- Docker available at `/usr/bin/docker`

### Slave Node 1: `mcj-mini-2` (Mac.banglab)

- **Hostname:** `Mac.banglab` (Tailscale name: `mcj-mini-2`)
- **IP Address:** `100.123.170.53`
- **User:** `mcj@100.123.170.53`
- **Platform:** macOS
- **Role:** Always-on worker node
- **Status:** ✅ Reachable and operational (uptime: 17+ days)
- **Load:** 3.36, 3.48, 3.61

**Services Running:**
- Diane process (1 process running)
- SSH server (port 22)

**Notes:**
- Always-on Mac machine
- Reliable for production workloads
- Good for macOS-specific testing and builds
- Diane binary not in default PATH (needs PATH configuration)

### Slave Node 2: `tool` (Laptop)

- **Hostname:** `tool` (Tailscale name)
- **IP Address:** `100.75.227.125`
- **User:** `mcj@100.75.227.125`
- **Platform:** macOS
- **Role:** Development/testing node
- **Status:** ⚠️ Intermittently available (currently offline, last seen 6m ago)

**Notes:**
- Laptop device, frequently unavailable
- Not suitable for critical workloads
- Useful for ad-hoc testing when online

## Connectivity Verification

### SSH Access Test

**Check Master:**
```bash
ssh root@100.71.82.7 hostname
# Expected: mcj-emergent
```

**Check Slave Node 1:**
```bash
ssh mcj@100.123.170.53 hostname
# Expected: Mac.banglab
```

**Check Slave Node 2:**
```bash
ssh mcj@100.75.227.125 hostname
# May timeout if laptop is offline
```

### Verify Diane is Running

**On Master:**
```bash
ssh root@100.71.82.7 "diane version && pgrep -af diane"
```

**On Slave Nodes:**
```bash
ssh mcj@100.123.170.53 "diane version && pgrep -af diane"
ssh mcj@100.75.227.125 "diane version && pgrep -af diane"
```

## Upgrade Procedures

### Upgrade All Nodes

A convenience script is provided to upgrade Diane on all reachable nodes:

```bash
# Upgrade to latest version on all nodes
./scripts/upgrade-all-nodes.sh

# Force upgrade even if already at latest version
./scripts/upgrade-all-nodes.sh --force
```

The script will:
1. Attempt to upgrade the master node locally
2. SSH into each slave node and trigger upgrade
3. Skip unreachable nodes automatically
4. Provide a summary of successes/failures

### Manual Upgrade (Single Node)

**On Master:**
```bash
ssh root@100.71.82.7 "diane upgrade"
# Or with force flag:
ssh root@100.71.82.7 "diane upgrade --force"
```

**On Slave Nodes:**
```bash
ssh mcj@100.123.170.53 "diane upgrade"
ssh mcj@100.75.227.125 "diane upgrade"
```

### Build and Deploy from Source

**Build on Master:**
```bash
ssh root@100.71.82.7
cd /root
make build          # Build diane binary
make build-acp      # Build acp-server binary
make install        # Install to ~/.diane/bin/
```

**Deploy to Slave Nodes:**
```bash
# Copy binary to slave node 1
scp root@100.71.82.7:/root/dist/diane mcj@100.123.170.53:~/.diane/bin/diane

# Copy binary to slave node 2
scp root@100.71.82.7:/root/dist/diane mcj@100.75.227.125:~/.diane/bin/diane

# Restart services on slaves (if using systemd)
ssh mcj@100.123.170.53 "systemctl --user restart diane"
ssh mcj@100.75.227.125 "systemctl --user restart diane"
```

## Troubleshooting

### Node Unreachable

If a slave node is unreachable:

1. **Check Tailscale/ZeroTier status:**
   ```bash
   tailscale status  # or zerotier-cli listnetworks
   ```

2. **Verify the node is powered on** (especially for laptop/slave2)

3. **Check firewall rules:**
   ```bash
   # On the unreachable node
   sudo ufw status
   ```

4. **Test basic connectivity:**
   ```bash
   ping 100.123.170.53
   ping 100.75.227.125
   ```

### Diane Process Not Running

**Check process status:**
```bash
pgrep -af diane
```

**Check systemd service (Linux):**
```bash
systemctl status diane
journalctl -u diane -n 50
```

**Manually restart:**
```bash
# Kill existing process
pkill diane

# Start in foreground for debugging
diane

# Or restart service
systemctl restart diane  # Linux
launchctl restart diane  # macOS
```

### Docker Container Issues (Master Node)

**List all containers:**
```bash
docker ps -a
```

**View logs:**
```bash
docker logs <container_name>
# Or use Dozzle web UI
```

**Restart specific service:**
```bash
docker restart <container_name>
```

**Restart all containers:**
```bash
docker restart $(docker ps -q)
```

## Network Topology Diagram

```
┌─────────────────────────────────────────────────┐
│  Private Network (Tailscale)                    │
│  100.x.x.x range                                │
└─────────────────────────────────────────────────┘
                      │
        ┌─────────────┼─────────────┐
        │             │             │
        ▼             ▼             ▼
   ┌──────────┐  ┌──────────┐  ┌──────────┐
   │ Master   │  │ Slave 1  │  │ Slave 2  │
   │mcj-emergent││mcj-mini-2│  │  tool    │
   │100.71... │  │100.123.. │  │100.75..  │
   │  (root)  │  │  (mcj)   │  │  (mcj)   │
   │ ✅       │  │ ✅       │  │ ⚠️       │
   │ Always   │  │ Always   │  │Sometimes │
   └──────────┘  └──────────┘  └──────────┘
       │
   Emergent
   Services
   + Diane
```

## Development Workflow

### Typical Development Cycle

1. **Develop on Master:**
   ```bash
   cd /root/diane
   # Make code changes
   make build
   make test
   ```

2. **Deploy to Test Nodes:**
   ```bash
   # Install locally first
   make install
   
   # Then copy to slaves for testing
   scp dist/diane mcj@100.123.170.53:~/.diane/bin/
   ```

3. **Verify Deployment:**
   ```bash
   ./scripts/upgrade-all-nodes.sh --force
   ```

4. **Create Release:**
   ```bash
   make release  # Builds for all platforms
   ```

### Working with Distributed Services

The master node runs the core infrastructure, while slave nodes can:
- Run Diane instances for distributed MCP serving
- Execute platform-specific tests (macOS vs Linux)
- Provide redundancy and load distribution

## Node Configuration Files

**Diane Config Locations:**
- Master: `/root/.diane/`
- Slave 1: `/Users/mcj/.diane/`
- Slave 2: `/Users/mcj/.diane/` or `/home/mcj/.diane/`

**Important Files:**
- `~/.diane/bin/diane` — Binary
- `~/.diane/bin/acp-server` — ACP server binary
- `~/.diane/secrets/` — Credentials and config
- `~/.diane/logs/` — Log files

## Security Notes

- All nodes are connected via encrypted Tailscale mesh network
- SSH keys should be configured for passwordless access between nodes
- Credentials stored in `~/.diane/secrets/` should never be committed to git
- Master node has elevated privileges (`root` user) — be careful with destructive operations
- `emergent-server` container currently showing unhealthy status — needs investigation

## Maintenance Schedule

**Regular Tasks:**
- **Weekly:** Check that all nodes are reachable and Diane is running
- **Monthly:** Update Diane to latest version via `upgrade-all-nodes.sh`
- **Quarterly:** Review Docker container inventory on master node
- **As-needed:** Restart containers/services when logs show errors

## Contact and Support

For issues with the development environment:
1. Check this document first
2. Review logs on affected nodes
3. Test network connectivity
4. Consult the main README.md for Diane-specific help

---

**Last Updated:** 2026-02-20  
**Maintained By:** Development Team
