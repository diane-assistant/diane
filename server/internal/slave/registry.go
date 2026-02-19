package slave

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/diane-assistant/diane/internal/slavetypes"
	"github.com/diane-assistant/diane/internal/store"
	"github.com/gorilla/websocket"
)

// ConnectionStatus represents the status of a slave connection
type ConnectionStatus string

const (
	StatusConnected    ConnectionStatus = "connected"
	StatusDisconnected ConnectionStatus = "disconnected"
	StatusReconnecting ConnectionStatus = "reconnecting"
)

// SlaveConnection represents a connected slave server
type SlaveConnection struct {
	HostID        string
	Conn          *websocket.Conn
	Tools         []map[string]interface{}
	Status        ConnectionStatus
	LastHeartbeat time.Time
	ConnectedAt   time.Time
	CertSerial    string
	mu            sync.RWMutex
}

// Registry manages connected slave servers
type Registry struct {
	connections   map[string]*SlaveConnection // hostID -> connection
	mu            sync.RWMutex
	db            store.SlaveStore
	ca            *CertificateAuthority
	notifyChannel chan *RegistryNotification
}

// RegistryNotification represents a registry event
type RegistryNotification struct {
	HostID    string
	EventType string // "connected", "disconnected", "tools_updated"
	Tools     []map[string]interface{}
}

// NewRegistry creates a new slave registry
func NewRegistry(slaveStore store.SlaveStore, ca *CertificateAuthority) *Registry {
	r := &Registry{
		connections:   make(map[string]*SlaveConnection),
		db:            slaveStore,
		ca:            ca,
		notifyChannel: make(chan *RegistryNotification, 10),
	}

	// Start heartbeat monitoring
	go r.monitorHeartbeats()

	return r
}

// Register adds a new slave connection to the registry
func (r *Registry) Register(hostID string, conn *websocket.Conn, certSerial string, tools []map[string]interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if host already connected
	if existing, ok := r.connections[hostID]; ok {
		// Close old connection
		existing.Conn.Close()
	}

	// Create new connection
	slaveConn := &SlaveConnection{
		HostID:        hostID,
		Conn:          conn,
		Tools:         tools,
		Status:        StatusConnected,
		LastHeartbeat: time.Now(),
		ConnectedAt:   time.Now(),
		CertSerial:    certSerial,
	}

	r.connections[hostID] = slaveConn

	// Update database
	if err := r.db.UpdateSlaveLastSeen(context.Background(), hostID); err != nil {
		fmt.Printf("Warning: failed to update last seen for %s: %v\n", hostID, err)
	}

	// Send notification
	r.notify(&RegistryNotification{
		HostID:    hostID,
		EventType: "connected",
		Tools:     tools,
	})

	return nil
}

// Unregister removes a slave connection from the registry
func (r *Registry) Unregister(hostID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if conn, ok := r.connections[hostID]; ok {
		conn.Status = StatusDisconnected
		conn.Conn.Close()
		delete(r.connections, hostID)

		// Send notification
		r.notify(&RegistryNotification{
			HostID:    hostID,
			EventType: "disconnected",
		})
	}
}

// GetConnection retrieves a slave connection by host ID
func (r *Registry) GetConnection(hostID string) (*SlaveConnection, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	conn, ok := r.connections[hostID]
	return conn, ok
}

// GetConnectedSlaves returns all connected slaves
func (r *Registry) GetConnectedSlaves() []*SlaveConnection {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var slaves []*SlaveConnection
	for _, conn := range r.connections {
		if conn.Status == StatusConnected {
			slaves = append(slaves, conn)
		}
	}

	return slaves
}

// GetAllSlaves returns all slaves (connected and disconnected from DB)
func (r *Registry) GetAllSlaves() ([]*SlaveInfo, error) {
	// Get from database
	dbSlaves, err := r.db.ListSlaveServers(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to list slaves from DB: %w", err)
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var slaves []*SlaveInfo
	for _, dbSlave := range dbSlaves {
		info := &SlaveInfo{
			HostID:     dbSlave.HostID,
			CertSerial: dbSlave.CertSerial,
			Platform:   dbSlave.Platform,
			Version:    dbSlave.Version,
			IssuedAt:   dbSlave.IssuedAt,
			ExpiresAt:  dbSlave.ExpiresAt,
			Enabled:    dbSlave.Enabled,
			Status:     StatusDisconnected,
		}

		// Check if connected
		if conn, ok := r.connections[dbSlave.HostID]; ok {
			info.Status = conn.Status
			info.LastHeartbeat = &conn.LastHeartbeat
			info.ConnectedAt = &conn.ConnectedAt
			info.ToolCount = len(conn.Tools)
			info.Tools = conn.Tools
		} else if dbSlave.LastSeen != nil {
			info.LastHeartbeat = dbSlave.LastSeen
		}

		slaves = append(slaves, info)
	}

	return slaves, nil
}

// UpdateTools updates the tools for a connected slave
func (r *Registry) UpdateTools(hostID string, tools []map[string]interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	conn, ok := r.connections[hostID]
	if !ok {
		return fmt.Errorf("slave %s not connected", hostID)
	}

	conn.mu.Lock()
	conn.Tools = tools
	conn.mu.Unlock()

	// Send notification
	r.notify(&RegistryNotification{
		HostID:    hostID,
		EventType: "tools_updated",
		Tools:     tools,
	})

	return nil
}

// UpdateHeartbeat updates the last heartbeat time for a slave
func (r *Registry) UpdateHeartbeat(hostID string) error {
	r.mu.RLock()
	conn, ok := r.connections[hostID]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("slave %s not connected", hostID)
	}

	conn.mu.Lock()
	conn.LastHeartbeat = time.Now()
	conn.mu.Unlock()

	// Update database
	return r.db.UpdateSlaveLastSeen(context.Background(), hostID)
}

// IsConnected checks if a slave is currently connected
func (r *Registry) IsConnected(hostID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	conn, ok := r.connections[hostID]
	return ok && conn.Status == StatusConnected
}

// GetNotificationChannel returns the channel for registry notifications
func (r *Registry) GetNotificationChannel() <-chan *RegistryNotification {
	return r.notifyChannel
}

// notify sends a notification to the channel (non-blocking)
func (r *Registry) notify(notification *RegistryNotification) {
	select {
	case r.notifyChannel <- notification:
	default:
		// Channel full, skip notification
	}
}

// monitorHeartbeats checks for stale connections and marks them as disconnected
func (r *Registry) monitorHeartbeats() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		r.mu.Lock()
		now := time.Now()
		timeout := 2 * time.Minute // Consider slave dead after 2 minutes without heartbeat

		for hostID, conn := range r.connections {
			conn.mu.RLock()
			lastHeartbeat := conn.LastHeartbeat
			conn.mu.RUnlock()

			if now.Sub(lastHeartbeat) > timeout {
				fmt.Printf("Slave %s heartbeat timeout, marking as disconnected\n", hostID)
				conn.Status = StatusDisconnected
				conn.Conn.Close()
				delete(r.connections, hostID)

				// Send notification
				r.notify(&RegistryNotification{
					HostID:    hostID,
					EventType: "disconnected",
				})
			}
		}
		r.mu.Unlock()
	}
}

// Connect registers a slave connection (alias for Register)
func (r *Registry) Connect(hostname string, tools []map[string]interface{}, conn *websocket.Conn) error {
	return r.Register(hostname, conn, "", tools)
}

// Disconnect removes a slave connection (alias for Unregister)
func (r *Registry) Disconnect(hostname string) {
	r.Unregister(hostname)
}

// Heartbeat updates the heartbeat timestamp for a slave
func (r *Registry) Heartbeat(hostname string) error {
	return r.UpdateHeartbeat(hostname)
}

// HandleResponse processes a response message from a slave
// This is used by the WebSocket client to handle tool call responses
func (r *Registry) HandleResponse(hostname string, msg slavetypes.Message) {
	// Response handling is delegated to the WSClient waiting for responses
	// The registry just forwards the notification
	r.notify(&RegistryNotification{
		HostID:    hostname,
		EventType: "response",
	})
}

// SlaveInfo contains information about a slave server
type SlaveInfo struct {
	HostID        string
	CertSerial    string
	Platform      string
	Version       string
	IssuedAt      time.Time
	ExpiresAt     time.Time
	LastHeartbeat *time.Time
	ConnectedAt   *time.Time
	Enabled       bool
	Status        ConnectionStatus
	ToolCount     int
	Tools         []map[string]interface{}
}

// UpdateConnection updates a connection's tools
func (sc *SlaveConnection) UpdateTools(tools []map[string]interface{}) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.Tools = tools
}

// GetTools returns a copy of the tools
func (sc *SlaveConnection) GetTools() []map[string]interface{} {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	tools := make([]map[string]interface{}, len(sc.Tools))
	copy(tools, sc.Tools)
	return tools
}
