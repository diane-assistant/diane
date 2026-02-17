package slave

import (
	"fmt"
	"log/slog"

	"github.com/diane-assistant/diane/internal/db"
	"github.com/diane-assistant/diane/internal/mcpproxy"
)

// Manager coordinates slave connections with the MCP proxy
type Manager struct {
	registry *Registry
	proxy    *mcpproxy.Proxy
	server   *Server
	db       *db.DB
	pairing  *PairingService
}

// NewManager creates a new slave manager
func NewManager(database *db.DB, proxy *mcpproxy.Proxy, ca *CertificateAuthority) (*Manager, error) {
	registry := NewRegistry(database, ca)
	pairing := NewPairingService(database, ca)

	m := &Manager{
		registry: registry,
		proxy:    proxy,
		db:       database,
		pairing:  pairing,
	}

	// Start monitoring registry notifications
	go m.monitorRegistry()

	return m, nil
}

// StartServer starts the WebSocket server for slave connections
func (m *Manager) StartServer(addr string, ca *CertificateAuthority) error {
	server, err := Start(addr, ca, m.registry, m.db, m.pairing)
	if err != nil {
		return fmt.Errorf("failed to start slave server: %w", err)
	}

	m.server = server
	slog.Info("Slave server initialized", "addr", addr)
	return nil
}

// monitorRegistry watches for slave connection events and updates the proxy
func (m *Manager) monitorRegistry() {
	notifyChan := m.registry.GetNotificationChannel()

	for notification := range notifyChan {
		switch notification.EventType {
		case "connected":
			m.handleSlaveConnected(notification)
		case "disconnected":
			m.handleSlaveDisconnected(notification)
		case "tools_updated":
			m.handleToolsUpdated(notification)
		}
	}
}

// handleSlaveConnected registers a slave with the MCP proxy
func (m *Manager) handleSlaveConnected(notification *RegistryNotification) {
	slog.Info("Slave connected, registering with proxy",
		"hostname", notification.HostID,
		"tools", len(notification.Tools))

	// Create a virtual client for the slave
	// The slave's tools are already registered in the registry
	// We create a proxy client that represents the slave
	conn, ok := m.registry.GetConnection(notification.HostID)
	if !ok {
		slog.Error("Failed to get slave connection", "hostname", notification.HostID)
		return
	}

	// Create a SlaveProxyClient that wraps the slave connection
	client := NewSlaveProxyClient(notification.HostID, conn, m.server)

	// Register with MCP proxy
	if err := m.proxy.RegisterSlaveClient(notification.HostID, client); err != nil {
		slog.Error("Failed to register slave with proxy",
			"hostname", notification.HostID,
			"error", err)
		return
	}

	slog.Info("Slave registered with MCP proxy", "hostname", notification.HostID)
}

// handleSlaveDisconnected removes a slave from the MCP proxy
func (m *Manager) handleSlaveDisconnected(notification *RegistryNotification) {
	slog.Info("Slave disconnected, unregistering from proxy",
		"hostname", notification.HostID)

	if err := m.proxy.UnregisterSlaveClient(notification.HostID); err != nil {
		slog.Warn("Failed to unregister slave from proxy",
			"hostname", notification.HostID,
			"error", err)
	}
}

// handleToolsUpdated invalidates tool cache for the slave
func (m *Manager) handleToolsUpdated(notification *RegistryNotification) {
	slog.Info("Slave tools updated",
		"hostname", notification.HostID,
		"tools", len(notification.Tools))

	// Get the client and invalidate its cache
	client, ok := m.proxy.GetClient(notification.HostID)
	if ok {
		client.InvalidateToolCache()
	}
}

// GetRegistry returns the slave registry
func (m *Manager) GetRegistry() *Registry {
	return m.registry
}

// GetServer returns the WebSocket server
func (m *Manager) GetServer() *Server {
	return m.server
}

// GetPairingService returns the pairing service
func (m *Manager) GetPairingService() *PairingService {
	return m.pairing
}

// RevokeCredential revokes a slave's credentials
func (m *Manager) RevokeCredential(hostname, reason string) error {
	// Get slave info to get cert serial
	slave, err := m.db.GetSlaveServerByHostID(hostname)
	if err != nil {
		return fmt.Errorf("failed to find slave %s: %w", hostname, err)
	}

	// Add to revoked credentials
	if err := m.db.RevokeSlaveCredential(hostname, slave.CertSerial, reason); err != nil {
		return fmt.Errorf("failed to revoke credential: %w", err)
	}

	// Disconnect if connected
	m.registry.Disconnect(hostname)

	// Update slave status in DB
	if err := m.db.UpdateSlaveStatus(hostname, false); err != nil {
		slog.Warn("Failed to disable slave", "hostname", hostname, "error", err)
	}

	return nil
}

// ListRevokedCredentials retrieves all revoked credentials
func (m *Manager) ListRevokedCredentials() ([]*db.RevokedCredential, error) {
	return m.db.ListRevokedCredentials()
}

// RestartSlave sends a restart command to a connected slave
func (m *Manager) RestartSlave(hostname string) error {
	if m.server == nil {
		return fmt.Errorf("slave server not initialized")
	}

	return m.server.SendRestartCommand(hostname)
}

// UpgradeSlave sends an upgrade command to a specific slave
func (m *Manager) UpgradeSlave(hostname string) error {
	if m.server == nil {
		return fmt.Errorf("slave server not initialized")
	}

	return m.server.SendUpgradeCommand(hostname)
}

// Stop stops the slave manager and server
func (m *Manager) Stop() error {
	if m.server != nil {
		return m.server.Stop()
	}
	return nil
}
