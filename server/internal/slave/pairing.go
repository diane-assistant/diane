package slave

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/diane-assistant/diane/internal/db"
)

// PairingService manages slave pairing requests
type PairingService struct {
	db              *db.DB
	ca              *CertificateAuthority
	pendingRequests map[string]*PairingRequestInfo
	mu              sync.RWMutex
	notifyChannel   chan *PairingNotification
}

// PairingRequestInfo holds in-memory state for a pairing request
type PairingRequestInfo struct {
	HostID      string
	PairingCode string
	CSR         []byte
	Platform    string
	RequestedAt time.Time
	ExpiresAt   time.Time
}

// PairingNotification represents a notification about a pairing request
type PairingNotification struct {
	HostID      string
	PairingCode string
	EventType   string // "new", "approved", "denied", "expired"
}

// NewPairingService creates a new pairing service
func NewPairingService(database *db.DB, ca *CertificateAuthority) *PairingService {
	ps := &PairingService{
		db:              database,
		ca:              ca,
		pendingRequests: make(map[string]*PairingRequestInfo),
		notifyChannel:   make(chan *PairingNotification, 10),
	}

	// Start cleanup goroutine
	go ps.cleanupExpiredRequests()

	return ps
}

// GeneratePairingCode generates a 6-digit pairing code
func (ps *PairingService) GeneratePairingCode() (string, error) {
	// Generate random 6-digit code (100000-999999)
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "", fmt.Errorf("failed to generate pairing code: %w", err)
	}

	code := int(n.Int64()) + 100000

	// Format as "123-456"
	return fmt.Sprintf("%03d-%03d", code/1000, code%1000), nil
}

// CreatePairingRequest creates a new pairing request
func (ps *PairingService) CreatePairingRequest(hostID string, csrPEM []byte, platform string) (string, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Check if host already has a pending request
	for _, req := range ps.pendingRequests {
		if req.HostID == hostID && time.Now().Before(req.ExpiresAt) {
			return "", fmt.Errorf("pairing request already pending for host %s", hostID)
		}
	}

	// Generate pairing code
	pairingCode, err := ps.GeneratePairingCode()
	if err != nil {
		return "", err
	}

	// Create request info
	now := time.Now()
	expiresAt := now.Add(10 * time.Minute)

	req := &PairingRequestInfo{
		HostID:      hostID,
		PairingCode: pairingCode,
		CSR:         csrPEM,
		Platform:    platform,
		RequestedAt: now,
		ExpiresAt:   expiresAt,
	}

	// Store in memory
	ps.pendingRequests[pairingCode] = req

	// Persist to database
	if err := ps.db.CreatePairingRequest(hostID, pairingCode, string(csrPEM), platform, expiresAt); err != nil {
		delete(ps.pendingRequests, pairingCode)
		return "", fmt.Errorf("failed to persist pairing request: %w", err)
	}

	// Send notification
	ps.notify(&PairingNotification{
		HostID:      hostID,
		PairingCode: pairingCode,
		EventType:   "new",
	})

	return pairingCode, nil
}

// ApprovePairingRequest approves a pairing request and issues certificate
func (ps *PairingService) ApprovePairingRequest(hostID, pairingCode string) (certPEM, caCertPEM []byte, err error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Find request
	req, ok := ps.pendingRequests[pairingCode]
	if !ok {
		return nil, nil, fmt.Errorf("pairing request not found")
	}

	if req.HostID != hostID {
		return nil, nil, fmt.Errorf("host ID mismatch")
	}

	if time.Now().After(req.ExpiresAt) {
		delete(ps.pendingRequests, pairingCode)
		return nil, nil, fmt.Errorf("pairing request expired")
	}

	// Sign the CSR
	certPEM, serialNumber, err := ps.ca.SignCSR(req.CSR, hostID, 365)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to sign CSR: %w", err)
	}

	// Get CA certificate
	caCertPEM, err = ps.ca.GetCACertPEM()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get CA cert: %w", err)
	}

	// Create slave server record in database
	now := time.Now()
	expiresAt := now.AddDate(0, 0, 365)
	_, err = ps.db.CreateSlaveServer(hostID, serialNumber, req.Platform, now, expiresAt)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create slave server record: %w", err)
	}

	// Update request status and store certificate
	if err := ps.db.UpdatePairingRequestApproved(hostID, pairingCode, string(certPEM)); err != nil {
		// Log error but don't fail
		fmt.Printf("Warning: failed to update pairing request status: %v\n", err)
	}

	// Remove from pending
	delete(ps.pendingRequests, pairingCode)

	// Send notification
	ps.notify(&PairingNotification{
		HostID:      hostID,
		PairingCode: pairingCode,
		EventType:   "approved",
	})

	return certPEM, caCertPEM, nil
}

// GetPairingStatus retrieves the status of a pairing request by code
func (ps *PairingService) GetPairingStatus(pairingCode string) (string, string, error) {
	// First check in-memory pending requests
	ps.mu.RLock()
	_, ok := ps.pendingRequests[pairingCode]
	ps.mu.RUnlock()

	if ok {
		return "pending", "", nil
	}

	// Check database for approved/denied requests
	dbReq, err := ps.db.GetPairingRequest(pairingCode)
	if err != nil {
		return "", "", fmt.Errorf("failed to get pairing request: %w", err)
	}
	if dbReq == nil {
		return "not_found", "", nil
	}

	return dbReq.Status, dbReq.Certificate, nil
}

// GetCACertPEM returns the CA certificate PEM
func (ps *PairingService) GetCACertPEM() ([]byte, error) {
	return ps.ca.GetCACertPEM()
}

// DenyPairingRequest denies a pairing request
func (ps *PairingService) DenyPairingRequest(hostID, pairingCode string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	req, ok := ps.pendingRequests[pairingCode]
	if !ok {
		return fmt.Errorf("pairing request not found")
	}

	if req.HostID != hostID {
		return fmt.Errorf("host ID mismatch")
	}

	// Update request status
	if err := ps.db.UpdatePairingRequestStatus(hostID, pairingCode, "denied"); err != nil {
		return fmt.Errorf("failed to update pairing request status: %w", err)
	}

	// Remove from pending
	delete(ps.pendingRequests, pairingCode)

	// Send notification
	ps.notify(&PairingNotification{
		HostID:      hostID,
		PairingCode: pairingCode,
		EventType:   "denied",
	})

	return nil
}

// GetPendingRequests returns all pending pairing requests
func (ps *PairingService) GetPendingRequests() []*PairingRequestInfo {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	var requests []*PairingRequestInfo
	for _, req := range ps.pendingRequests {
		if time.Now().Before(req.ExpiresAt) {
			requests = append(requests, req)
		}
	}

	return requests
}

// GetNotificationChannel returns the channel for pairing notifications
func (ps *PairingService) GetNotificationChannel() <-chan *PairingNotification {
	return ps.notifyChannel
}

// notify sends a notification to the channel (non-blocking)
func (ps *PairingService) notify(notification *PairingNotification) {
	select {
	case ps.notifyChannel <- notification:
	default:
		// Channel full, skip notification
	}
}

// cleanupExpiredRequests periodically removes expired pairing requests
func (ps *PairingService) cleanupExpiredRequests() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		ps.mu.Lock()
		now := time.Now()
		for code, req := range ps.pendingRequests {
			if now.After(req.ExpiresAt) {
				delete(ps.pendingRequests, code)

				// Notify about expiration
				ps.notify(&PairingNotification{
					HostID:      req.HostID,
					PairingCode: code,
					EventType:   "expired",
				})
			}
		}
		ps.mu.Unlock()

		// Cleanup database
		if err := ps.db.CleanupExpiredPairingRequests(); err != nil {
			fmt.Printf("Warning: failed to cleanup expired pairing requests: %v\n", err)
		}
	}
}
