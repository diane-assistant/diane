package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SlaveServer represents a slave server configuration in the database
type SlaveServer struct {
	ID         string
	HostID     string
	CertSerial string
	IssuedAt   time.Time
	ExpiresAt  time.Time
	LastSeen   *time.Time
	Enabled    bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// RevokedCredential represents a revoked slave credential
type RevokedCredential struct {
	ID         string
	HostID     string
	CertSerial string
	RevokedAt  time.Time
	Reason     string
}

// PairingRequest represents a pending pairing request
type PairingRequest struct {
	ID          string
	HostID      string
	PairingCode string
	CSR         string
	RequestedAt time.Time
	ExpiresAt   time.Time
	Status      string
	Certificate string // Only present if approved
}

// CreateSlaveServer creates a new slave server configuration
func (db *DB) CreateSlaveServer(hostID, certSerial string, issuedAt, expiresAt time.Time) (*SlaveServer, error) {
	id := uuid.New().String()
	now := time.Now()

	_, err := db.conn.Exec(`
		INSERT INTO slave_servers (id, host_id, cert_serial, issued_at, expires_at, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 1, ?, ?)
	`, id, hostID, certSerial, issuedAt, expiresAt, now, now)

	if err != nil {
		return nil, fmt.Errorf("failed to create slave server: %w", err)
	}

	return db.GetSlaveServerByHostID(hostID)
}

// GetSlaveServerByHostID retrieves a slave server by host ID
func (db *DB) GetSlaveServerByHostID(hostID string) (*SlaveServer, error) {
	var s SlaveServer
	var lastSeen sql.NullTime

	err := db.conn.QueryRow(`
		SELECT id, host_id, cert_serial, issued_at, expires_at, last_seen, enabled, created_at, updated_at
		FROM slave_servers
		WHERE host_id = ?
	`, hostID).Scan(&s.ID, &s.HostID, &s.CertSerial, &s.IssuedAt, &s.ExpiresAt, &lastSeen, &s.Enabled, &s.CreatedAt, &s.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get slave server: %w", err)
	}

	if lastSeen.Valid {
		s.LastSeen = &lastSeen.Time
	}

	return &s, nil
}

// ListSlaveServers retrieves all slave servers
func (db *DB) ListSlaveServers() ([]*SlaveServer, error) {
	rows, err := db.conn.Query(`
		SELECT id, host_id, cert_serial, issued_at, expires_at, last_seen, enabled, created_at, updated_at
		FROM slave_servers
		ORDER BY host_id
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list slave servers: %w", err)
	}
	defer rows.Close()

	var servers []*SlaveServer
	for rows.Next() {
		var s SlaveServer
		var lastSeen sql.NullTime

		err := rows.Scan(&s.ID, &s.HostID, &s.CertSerial, &s.IssuedAt, &s.ExpiresAt, &lastSeen, &s.Enabled, &s.CreatedAt, &s.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan slave server: %w", err)
		}

		if lastSeen.Valid {
			s.LastSeen = &lastSeen.Time
		}

		servers = append(servers, &s)
	}

	return servers, nil
}

// UpdateSlaveStatus updates the enabled status of a slave
func (db *DB) UpdateSlaveStatus(hostID string, enabled bool) error {
	_, err := db.conn.Exec(`
		UPDATE slave_servers
		SET enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE host_id = ?
	`, enabled, hostID)
	if err != nil {
		return fmt.Errorf("failed to update slave status: %w", err)
	}
	return nil
}

// UpdateSlaveLastSeen updates the last seen timestamp for a slave
func (db *DB) UpdateSlaveLastSeen(hostID string) error {
	_, err := db.conn.Exec(`
		UPDATE slave_servers
		SET last_seen = ?, updated_at = ?
		WHERE host_id = ?
	`, time.Now(), time.Now(), hostID)

	return err
}

// DeleteSlaveServer deletes a slave server configuration
func (db *DB) DeleteSlaveServer(hostID string) error {
	_, err := db.conn.Exec(`
		DELETE FROM slave_servers
		WHERE host_id = ?
	`, hostID)

	return err
}

// RevokeSlaveCredential revokes a slave's credentials
func (db *DB) RevokeSlaveCredential(hostID, certSerial, reason string) error {
	id := uuid.New().String()

	_, err := db.conn.Exec(`
		INSERT INTO revoked_slave_credentials (id, host_id, cert_serial, revoked_at, reason)
		VALUES (?, ?, ?, ?, ?)
	`, id, hostID, certSerial, time.Now(), reason)

	return err
}

// IsCredentialRevoked checks if a certificate is revoked
func (db *DB) IsCredentialRevoked(certSerial string) (bool, error) {
	var count int
	err := db.conn.QueryRow(`
		SELECT COUNT(*) FROM revoked_slave_credentials
		WHERE cert_serial = ?
	`, certSerial).Scan(&count)

	if err != nil {
		return false, fmt.Errorf("failed to check revocation status: %w", err)
	}

	return count > 0, nil
}

// ListRevokedCredentials retrieves all revoked credentials
func (db *DB) ListRevokedCredentials() ([]*RevokedCredential, error) {
	rows, err := db.conn.Query(`
		SELECT id, host_id, cert_serial, revoked_at, reason
		FROM revoked_slave_credentials
		ORDER BY revoked_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list revoked credentials: %w", err)
	}
	defer rows.Close()

	var revoked []*RevokedCredential
	for rows.Next() {
		var r RevokedCredential
		var reason sql.NullString

		err := rows.Scan(&r.ID, &r.HostID, &r.CertSerial, &r.RevokedAt, &reason)
		if err != nil {
			return nil, fmt.Errorf("failed to scan revoked credential: %w", err)
		}

		if reason.Valid {
			r.Reason = reason.String
		}

		revoked = append(revoked, &r)
	}

	return revoked, nil
}

// CreatePairingRequest creates a new pairing request
func (db *DB) CreatePairingRequest(hostID, pairingCode, csr string, expiresAt time.Time) error {
	id := uuid.New().String()

	_, err := db.conn.Exec(`
		INSERT INTO pairing_requests (id, host_id, pairing_code, csr, requested_at, expires_at, status)
		VALUES (?, ?, ?, ?, ?, ?, 'pending')
	`, id, hostID, pairingCode, csr, time.Now(), expiresAt)

	return err
}

// GetPairingRequest retrieves a pairing request by code
func (db *DB) GetPairingRequest(pairingCode string) (*PairingRequest, error) {
	var pr PairingRequest
	var cert sql.NullString

	// Note: We don't filter by status so we can retrieve approved requests
	err := db.conn.QueryRow(`
		SELECT id, host_id, pairing_code, csr, requested_at, expires_at, status, certificate
		FROM pairing_requests
		WHERE pairing_code = ?
	`, pairingCode).Scan(&pr.ID, &pr.HostID, &pr.PairingCode, &pr.CSR, &pr.RequestedAt, &pr.ExpiresAt, &pr.Status, &cert)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get pairing request: %w", err)
	}

	if cert.Valid {
		pr.Certificate = cert.String
	}

	return &pr, nil
}

// ListPendingPairingRequests retrieves all pending pairing requests
func (db *DB) ListPendingPairingRequests() ([]*PairingRequest, error) {
	rows, err := db.conn.Query(`
		SELECT id, host_id, pairing_code, csr, requested_at, expires_at, status
		FROM pairing_requests
		WHERE status = 'pending' AND expires_at > ?
		ORDER BY requested_at DESC
	`, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to list pending pairing requests: %w", err)
	}
	defer rows.Close()

	var requests []*PairingRequest
	for rows.Next() {
		var pr PairingRequest

		err := rows.Scan(&pr.ID, &pr.HostID, &pr.PairingCode, &pr.CSR, &pr.RequestedAt, &pr.ExpiresAt, &pr.Status)
		if err != nil {
			return nil, fmt.Errorf("failed to scan pairing request: %w", err)
		}

		requests = append(requests, &pr)
	}

	return requests, nil
}

// UpdatePairingRequestStatus updates the status of a pairing request
func (db *DB) UpdatePairingRequestStatus(hostID, pairingCode, status string) error {
	_, err := db.conn.Exec(`
		UPDATE pairing_requests
		SET status = ?
		WHERE host_id = ? AND pairing_code = ?
	`, status, hostID, pairingCode)

	return err
}

// UpdatePairingRequestApproved marks a request as approved and stores the certificate
func (db *DB) UpdatePairingRequestApproved(hostID, pairingCode, certPEM string) error {
	_, err := db.conn.Exec(`
		UPDATE pairing_requests
		SET status = 'approved', certificate = ?
		WHERE host_id = ? AND pairing_code = ?
	`, certPEM, hostID, pairingCode)

	return err
}

// CleanupExpiredPairingRequests deletes expired pairing requests
func (db *DB) CleanupExpiredPairingRequests() error {
	_, err := db.conn.Exec(`
		DELETE FROM pairing_requests
		WHERE expires_at < ? AND status = 'pending'
	`, time.Now())

	return err
}
