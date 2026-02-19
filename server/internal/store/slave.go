package store

import (
	"context"
	"time"

	"github.com/diane-assistant/diane/internal/db"
)

// SlaveStore defines the interface for slave server storage operations
type SlaveStore interface {
	// SlaveServer operations
	CreateSlaveServer(ctx context.Context, hostID, certSerial, platform string, issuedAt, expiresAt time.Time) (*db.SlaveServer, error)
	GetSlaveServerByHostID(ctx context.Context, hostID string) (*db.SlaveServer, error)
	ListSlaveServers(ctx context.Context) ([]*db.SlaveServer, error)
	UpdateSlaveStatus(ctx context.Context, hostID string, enabled bool) error
	UpdateSlaveLastSeen(ctx context.Context, hostID string) error
	UpdateSlaveVersion(ctx context.Context, hostID, version string) error
	UpdateSlaveServerCredentials(ctx context.Context, hostID, certSerial, platform string, issuedAt, expiresAt time.Time) error
	DeleteSlaveServer(ctx context.Context, hostID string) error

	// RevokedCredential operations
	RevokeSlaveCredential(ctx context.Context, hostID, certSerial, reason string) error
	IsCredentialRevoked(ctx context.Context, certSerial string) (bool, error)
	ListRevokedCredentials(ctx context.Context) ([]*db.RevokedCredential, error)

	// PairingRequest operations
	CreatePairingRequest(ctx context.Context, hostID, pairingCode, csr, platform string, expiresAt time.Time) error
	GetPairingRequest(ctx context.Context, pairingCode string) (*db.PairingRequest, error)
	ListPendingPairingRequests(ctx context.Context) ([]*db.PairingRequest, error)
	UpdatePairingRequestStatus(ctx context.Context, hostID, pairingCode, status string) error
	UpdatePairingRequestApproved(ctx context.Context, hostID, pairingCode, certPEM string) error
	CleanupExpiredPairingRequests(ctx context.Context) error
	DeletePendingPairingRequestsForHost(ctx context.Context, hostID string) error
}
