package store

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	sdk "github.com/emergent-company/emergent/apps/server-go/pkg/sdk"
	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"
	"github.com/google/uuid"

	"github.com/diane-assistant/diane/internal/db"
)

// EmergentSlaveStore implements SlaveStore against the Emergent knowledge-base graph API.
//
// Mapping:
//
//	SlaveServer:
//	  - SQLite table row -> graph object type "slave_server"
//	  - HostID (unique)  -> properties.host_id + label "host_id:{hostID}"
//	  - CertSerial       -> properties.cert_serial
//	  - Platform         -> properties.platform
//	  - Version          -> properties.version
//	  - IssuedAt         -> properties.issued_at (RFC3339Nano)
//	  - ExpiresAt        -> properties.expires_at (RFC3339Nano)
//	  - LastSeen         -> properties.last_seen (RFC3339Nano, nullable)
//	  - Enabled          -> properties.enabled (bool)
//	  - CreatedAt        -> object.CreatedAt (built-in)
//	  - UpdatedAt        -> properties.updated_at (RFC3339Nano)
//
//	RevokedCredential:
//	  - SQLite table row -> graph object type "revoked_credential"
//	  - CertSerial (pk)  -> properties.cert_serial + label "cert_serial:{serial}"
//	  - HostID           -> properties.host_id
//	  - RevokedAt        -> properties.revoked_at (RFC3339Nano)
//	  - Reason           -> properties.reason
//	  - CreatedAt        -> object.CreatedAt
//
//	PairingRequest:
//	  - SQLite table row -> graph object type "pairing_request"
//	  - PairingCode (pk) -> properties.pairing_code + label "pairing_code:{code}"
//	  - HostID           -> properties.host_id + label "host_id:{hostID}"
//	  - CSR              -> properties.csr
//	  - Platform         -> properties.platform (added to schema)
//	  - RequestedAt      -> properties.requested_at (RFC3339Nano)
//	  - ExpiresAt        -> properties.expires_at (RFC3339Nano)
//	  - Status           -> properties.status ("pending", "approved", "rejected")
//	  - Certificate      -> properties.certificate (only if approved)
//	  - CreatedAt        -> object.CreatedAt
type EmergentSlaveStore struct {
	client *sdk.Client
}

const (
	slaveServerType    = "slave_server"
	revokedCredType    = "revoked_credential"
	pairingRequestType = "pairing_request"
)

// NewEmergentSlaveStore creates a new Emergent-backed SlaveStore.
func NewEmergentSlaveStore(client *sdk.Client) *EmergentSlaveStore {
	return &EmergentSlaveStore{client: client}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func hostIDLabel(hostID string) string     { return fmt.Sprintf("host_id:%s", hostID) }
func certSerialLabel(serial string) string { return fmt.Sprintf("cert_serial:%s", serial) }
func pairingCodeLabel(code string) string  { return fmt.Sprintf("pairing_code:%s", code) }

// slaveServerToProperties converts a db.SlaveServer to Emergent properties.
func slaveServerToProperties(s *db.SlaveServer) map[string]any {
	props := map[string]any{
		"host_id":     s.HostID,
		"cert_serial": s.CertSerial,
		"platform":    s.Platform,
		"version":     s.Version,
		"issued_at":   s.IssuedAt.UTC().Format(time.RFC3339Nano),
		"expires_at":  s.ExpiresAt.UTC().Format(time.RFC3339Nano),
		"enabled":     s.Enabled,
		"updated_at":  time.Now().UTC().Format(time.RFC3339Nano),
	}
	if s.LastSeen != nil {
		props["last_seen"] = s.LastSeen.UTC().Format(time.RFC3339Nano)
	}
	return props
}

// slaveServerFromObject converts an Emergent GraphObject to a db.SlaveServer.
func slaveServerFromObject(obj *graph.GraphObject) (*db.SlaveServer, error) {
	s := &db.SlaveServer{
		ID:        obj.ID,
		CreatedAt: obj.CreatedAt,
	}

	if v, ok := obj.Properties["host_id"].(string); ok {
		s.HostID = v
	}
	if v, ok := obj.Properties["cert_serial"].(string); ok {
		s.CertSerial = v
	}
	if v, ok := obj.Properties["platform"].(string); ok {
		s.Platform = v
	}
	if v, ok := obj.Properties["version"].(string); ok {
		s.Version = v
	}
	if v, ok := obj.Properties["enabled"].(bool); ok {
		s.Enabled = v
	}

	// Parse timestamps
	if v, ok := obj.Properties["issued_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			s.IssuedAt = t
		}
	}
	if v, ok := obj.Properties["expires_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			s.ExpiresAt = t
		}
	}
	if v, ok := obj.Properties["last_seen"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			s.LastSeen = &t
		}
	}
	if v, ok := obj.Properties["updated_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			s.UpdatedAt = t
		}
	}
	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = s.CreatedAt
	}

	return s, nil
}

// revokedCredentialToProperties converts a db.RevokedCredential to properties.
func revokedCredentialToProperties(r *db.RevokedCredential) map[string]any {
	return map[string]any{
		"cert_serial": r.CertSerial,
		"host_id":     r.HostID,
		"revoked_at":  r.RevokedAt.UTC().Format(time.RFC3339Nano),
		"reason":      r.Reason,
	}
}

// revokedCredentialFromObject converts an Emergent GraphObject to a db.RevokedCredential.
func revokedCredentialFromObject(obj *graph.GraphObject) (*db.RevokedCredential, error) {
	r := &db.RevokedCredential{
		ID: obj.ID,
	}

	if v, ok := obj.Properties["cert_serial"].(string); ok {
		r.CertSerial = v
	}
	if v, ok := obj.Properties["host_id"].(string); ok {
		r.HostID = v
	}
	if v, ok := obj.Properties["reason"].(string); ok {
		r.Reason = v
	}
	if v, ok := obj.Properties["revoked_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			r.RevokedAt = t
		}
	}

	return r, nil
}

// pairingRequestToProperties converts a db.PairingRequest to properties.
func pairingRequestToProperties(p *db.PairingRequest) map[string]any {
	props := map[string]any{
		"pairing_code": p.PairingCode,
		"host_id":      p.HostID,
		"csr":          p.CSR,
		"requested_at": p.RequestedAt.UTC().Format(time.RFC3339Nano),
		"expires_at":   p.ExpiresAt.UTC().Format(time.RFC3339Nano),
		"status":       p.Status,
	}
	if p.Certificate != "" {
		props["certificate"] = p.Certificate
	}
	return props
}

// pairingRequestFromObject converts an Emergent GraphObject to a db.PairingRequest.
func pairingRequestFromObject(obj *graph.GraphObject) (*db.PairingRequest, error) {
	p := &db.PairingRequest{
		ID: obj.ID,
	}

	if v, ok := obj.Properties["pairing_code"].(string); ok {
		p.PairingCode = v
	}
	if v, ok := obj.Properties["host_id"].(string); ok {
		p.HostID = v
	}
	if v, ok := obj.Properties["csr"].(string); ok {
		p.CSR = v
	}
	if v, ok := obj.Properties["status"].(string); ok {
		p.Status = v
	}
	if v, ok := obj.Properties["certificate"].(string); ok {
		p.Certificate = v
	}
	if v, ok := obj.Properties["requested_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			p.RequestedAt = t
		}
	}
	if v, ok := obj.Properties["expires_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			p.ExpiresAt = t
		}
	}

	return p, nil
}

// ---------------------------------------------------------------------------
// SlaveServer operations
// ---------------------------------------------------------------------------

func (s *EmergentSlaveStore) CreateSlaveServer(ctx context.Context, hostID, certSerial, platform string, issuedAt, expiresAt time.Time) (*db.SlaveServer, error) {
	id := uuid.New().String()

	server := &db.SlaveServer{
		ID:         id,
		HostID:     hostID,
		CertSerial: certSerial,
		Platform:   platform,
		IssuedAt:   issuedAt,
		ExpiresAt:  expiresAt,
		Enabled:    true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	props := slaveServerToProperties(server)
	labels := []string{hostIDLabel(hostID)}

	status := "active"
	obj, err := s.client.Graph.CreateObject(ctx, &graph.CreateObjectRequest{
		Type:       slaveServerType,
		Status:     &status,
		Properties: props,
		Labels:     labels,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent create slave server: %w", err)
	}

	slog.Info("emergent: created slave server", "host_id", hostID, "object_id", obj.ID)
	return s.GetSlaveServerByHostID(ctx, hostID)
}

func (s *EmergentSlaveStore) GetSlaveServerByHostID(ctx context.Context, hostID string) (*db.SlaveServer, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  slaveServerType,
		Label: hostIDLabel(hostID),
		Limit: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent lookup slave by host_id %q: %w", hostID, err)
	}
	if len(resp.Items) == 0 {
		return nil, nil
	}

	return slaveServerFromObject(resp.Items[0])
}

func (s *EmergentSlaveStore) ListSlaveServers(ctx context.Context) ([]*db.SlaveServer, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  slaveServerType,
		Limit: 1000,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent list slave servers: %w", err)
	}

	servers := make([]*db.SlaveServer, 0, len(resp.Items))
	for _, obj := range resp.Items {
		srv, err := slaveServerFromObject(obj)
		if err != nil {
			slog.Warn("skipping malformed slave server", "object_id", obj.ID, "error", err)
			continue
		}
		servers = append(servers, srv)
	}

	// Sort by host_id (match SQLite ORDER BY)
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].HostID < servers[j].HostID
	})

	return servers, nil
}

func (s *EmergentSlaveStore) UpdateSlaveStatus(ctx context.Context, hostID string, enabled bool) error {
	obj, err := s.lookupSlaveByHostID(ctx, hostID)
	if err != nil {
		return err
	}
	if obj == nil {
		return fmt.Errorf("slave server not found: %s", hostID)
	}

	_, err = s.client.Graph.UpdateObject(ctx, obj.ID, &graph.UpdateObjectRequest{
		Properties: map[string]any{
			"enabled":    enabled,
			"updated_at": time.Now().UTC().Format(time.RFC3339Nano),
		},
	})
	if err != nil {
		return fmt.Errorf("emergent update slave status: %w", err)
	}

	slog.Info("emergent: updated slave status", "host_id", hostID, "enabled", enabled)
	return nil
}

func (s *EmergentSlaveStore) UpdateSlaveLastSeen(ctx context.Context, hostID string) error {
	obj, err := s.lookupSlaveByHostID(ctx, hostID)
	if err != nil {
		return err
	}
	if obj == nil {
		return fmt.Errorf("slave server not found: %s", hostID)
	}

	now := time.Now()
	_, err = s.client.Graph.UpdateObject(ctx, obj.ID, &graph.UpdateObjectRequest{
		Properties: map[string]any{
			"last_seen":  now.UTC().Format(time.RFC3339Nano),
			"updated_at": now.UTC().Format(time.RFC3339Nano),
		},
	})
	if err != nil {
		return fmt.Errorf("emergent update last seen: %w", err)
	}

	return nil
}

func (s *EmergentSlaveStore) UpdateSlaveVersion(ctx context.Context, hostID, version string) error {
	obj, err := s.lookupSlaveByHostID(ctx, hostID)
	if err != nil {
		return err
	}
	if obj == nil {
		return fmt.Errorf("slave server not found: %s", hostID)
	}

	_, err = s.client.Graph.UpdateObject(ctx, obj.ID, &graph.UpdateObjectRequest{
		Properties: map[string]any{
			"version":    version,
			"updated_at": time.Now().UTC().Format(time.RFC3339Nano),
		},
	})
	if err != nil {
		return fmt.Errorf("emergent update slave version: %w", err)
	}

	slog.Info("emergent: updated slave version", "host_id", hostID, "version", version)
	return nil
}

func (s *EmergentSlaveStore) UpdateSlaveServerCredentials(ctx context.Context, hostID, certSerial, platform string, issuedAt, expiresAt time.Time) error {
	obj, err := s.lookupSlaveByHostID(ctx, hostID)
	if err != nil {
		return err
	}
	if obj == nil {
		return fmt.Errorf("slave server not found: %s", hostID)
	}

	_, err = s.client.Graph.UpdateObject(ctx, obj.ID, &graph.UpdateObjectRequest{
		Properties: map[string]any{
			"cert_serial": certSerial,
			"platform":    platform,
			"issued_at":   issuedAt.UTC().Format(time.RFC3339Nano),
			"expires_at":  expiresAt.UTC().Format(time.RFC3339Nano),
			"updated_at":  time.Now().UTC().Format(time.RFC3339Nano),
		},
	})
	if err != nil {
		return fmt.Errorf("emergent update slave credentials: %w", err)
	}

	slog.Info("emergent: updated slave credentials", "host_id", hostID, "cert_serial", certSerial)
	return nil
}

func (s *EmergentSlaveStore) DeleteSlaveServer(ctx context.Context, hostID string) error {
	obj, err := s.lookupSlaveByHostID(ctx, hostID)
	if err != nil {
		return err
	}
	if obj == nil {
		return nil // Already deleted
	}

	err = s.client.Graph.DeleteObject(ctx, obj.ID)
	if err != nil {
		return fmt.Errorf("emergent delete slave server: %w", err)
	}

	slog.Info("emergent: deleted slave server", "host_id", hostID, "object_id", obj.ID)
	return nil
}

// ---------------------------------------------------------------------------
// RevokedCredential operations
// ---------------------------------------------------------------------------

func (s *EmergentSlaveStore) RevokeSlaveCredential(ctx context.Context, hostID, certSerial, reason string) error {
	id := uuid.New().String()

	cred := &db.RevokedCredential{
		ID:         id,
		HostID:     hostID,
		CertSerial: certSerial,
		RevokedAt:  time.Now(),
		Reason:     reason,
	}

	props := revokedCredentialToProperties(cred)
	labels := []string{certSerialLabel(certSerial)}

	status := "active"
	_, err := s.client.Graph.CreateObject(ctx, &graph.CreateObjectRequest{
		Type:       revokedCredType,
		Status:     &status,
		Properties: props,
		Labels:     labels,
	})
	if err != nil {
		return fmt.Errorf("emergent revoke credential: %w", err)
	}

	slog.Info("emergent: revoked credential", "host_id", hostID, "cert_serial", certSerial, "reason", reason)
	return nil
}

func (s *EmergentSlaveStore) IsCredentialRevoked(ctx context.Context, certSerial string) (bool, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  revokedCredType,
		Label: certSerialLabel(certSerial),
		Limit: 1,
	})
	if err != nil {
		return false, fmt.Errorf("emergent check revocation: %w", err)
	}

	return len(resp.Items) > 0, nil
}

func (s *EmergentSlaveStore) ListRevokedCredentials(ctx context.Context) ([]*db.RevokedCredential, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  revokedCredType,
		Limit: 1000,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent list revoked credentials: %w", err)
	}

	creds := make([]*db.RevokedCredential, 0, len(resp.Items))
	for _, obj := range resp.Items {
		cred, err := revokedCredentialFromObject(obj)
		if err != nil {
			slog.Warn("skipping malformed revoked credential", "object_id", obj.ID, "error", err)
			continue
		}
		creds = append(creds, cred)
	}

	// Sort by revoked_at DESC (match SQLite ORDER BY)
	sort.Slice(creds, func(i, j int) bool {
		return creds[i].RevokedAt.After(creds[j].RevokedAt)
	})

	return creds, nil
}

// ---------------------------------------------------------------------------
// PairingRequest operations
// ---------------------------------------------------------------------------

func (s *EmergentSlaveStore) CreatePairingRequest(ctx context.Context, hostID, pairingCode, csr, platform string, expiresAt time.Time) error {
	id := uuid.New().String()

	req := &db.PairingRequest{
		ID:          id,
		HostID:      hostID,
		PairingCode: pairingCode,
		CSR:         csr,
		RequestedAt: time.Now(),
		ExpiresAt:   expiresAt,
		Status:      "pending",
	}

	props := pairingRequestToProperties(req)
	props["platform"] = platform // Add platform field
	labels := []string{pairingCodeLabel(pairingCode), hostIDLabel(hostID)}

	status := "active"
	_, err := s.client.Graph.CreateObject(ctx, &graph.CreateObjectRequest{
		Type:       pairingRequestType,
		Status:     &status,
		Properties: props,
		Labels:     labels,
	})
	if err != nil {
		return fmt.Errorf("emergent create pairing request: %w", err)
	}

	slog.Info("emergent: created pairing request", "host_id", hostID, "pairing_code", pairingCode)
	return nil
}

func (s *EmergentSlaveStore) GetPairingRequest(ctx context.Context, pairingCode string) (*db.PairingRequest, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  pairingRequestType,
		Label: pairingCodeLabel(pairingCode),
		Limit: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent lookup pairing request %q: %w", pairingCode, err)
	}
	if len(resp.Items) == 0 {
		return nil, nil
	}

	return pairingRequestFromObject(resp.Items[0])
}

func (s *EmergentSlaveStore) ListPendingPairingRequests(ctx context.Context) ([]*db.PairingRequest, error) {
	now := time.Now()
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type: pairingRequestType,
		PropertyFilters: []graph.PropertyFilter{
			{Path: "status", Op: "eq", Value: "pending"},
		},
		Limit: 1000,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent list pending pairing requests: %w", err)
	}

	requests := make([]*db.PairingRequest, 0)
	for _, obj := range resp.Items {
		req, err := pairingRequestFromObject(obj)
		if err != nil {
			slog.Warn("skipping malformed pairing request", "object_id", obj.ID, "error", err)
			continue
		}
		// Filter out expired requests
		if req.ExpiresAt.After(now) {
			requests = append(requests, req)
		}
	}

	// Sort by requested_at DESC (match SQLite ORDER BY)
	sort.Slice(requests, func(i, j int) bool {
		return requests[i].RequestedAt.After(requests[j].RequestedAt)
	})

	return requests, nil
}

func (s *EmergentSlaveStore) UpdatePairingRequestStatus(ctx context.Context, hostID, pairingCode, status string) error {
	obj, err := s.lookupPairingRequestByCode(ctx, pairingCode)
	if err != nil {
		return err
	}
	if obj == nil {
		return fmt.Errorf("pairing request not found: %s", pairingCode)
	}

	_, err = s.client.Graph.UpdateObject(ctx, obj.ID, &graph.UpdateObjectRequest{
		Properties: map[string]any{
			"status": status,
		},
	})
	if err != nil {
		return fmt.Errorf("emergent update pairing request status: %w", err)
	}

	slog.Info("emergent: updated pairing request status", "pairing_code", pairingCode, "status", status)
	return nil
}

func (s *EmergentSlaveStore) UpdatePairingRequestApproved(ctx context.Context, hostID, pairingCode, certPEM string) error {
	obj, err := s.lookupPairingRequestByCode(ctx, pairingCode)
	if err != nil {
		return err
	}
	if obj == nil {
		return fmt.Errorf("pairing request not found: %s", pairingCode)
	}

	_, err = s.client.Graph.UpdateObject(ctx, obj.ID, &graph.UpdateObjectRequest{
		Properties: map[string]any{
			"status":      "approved",
			"certificate": certPEM,
		},
	})
	if err != nil {
		return fmt.Errorf("emergent approve pairing request: %w", err)
	}

	slog.Info("emergent: approved pairing request", "pairing_code", pairingCode)
	return nil
}

func (s *EmergentSlaveStore) CleanupExpiredPairingRequests(ctx context.Context) error {
	now := time.Now()
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type: pairingRequestType,
		PropertyFilters: []graph.PropertyFilter{
			{Path: "status", Op: "eq", Value: "pending"},
		},
		Limit: 1000,
	})
	if err != nil {
		return fmt.Errorf("emergent list pairing requests for cleanup: %w", err)
	}

	deleted := 0
	for _, obj := range resp.Items {
		req, err := pairingRequestFromObject(obj)
		if err != nil {
			continue
		}
		if req.ExpiresAt.Before(now) && req.Status == "pending" {
			if err := s.client.Graph.DeleteObject(ctx, obj.ID); err != nil {
				slog.Warn("failed to delete expired pairing request", "object_id", obj.ID, "error", err)
				continue
			}
			deleted++
		}
	}

	if deleted > 0 {
		slog.Info("emergent: cleaned up expired pairing requests", "count", deleted)
	}
	return nil
}

func (s *EmergentSlaveStore) DeletePendingPairingRequestsForHost(ctx context.Context, hostID string) error {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  pairingRequestType,
		Label: hostIDLabel(hostID),
		PropertyFilters: []graph.PropertyFilter{
			{Path: "status", Op: "eq", Value: "pending"},
		},
		Limit: 1000,
	})
	if err != nil {
		return fmt.Errorf("emergent list pairing requests for host: %w", err)
	}

	deleted := 0
	for _, obj := range resp.Items {
		if err := s.client.Graph.DeleteObject(ctx, obj.ID); err != nil {
			slog.Warn("failed to delete pairing request", "object_id", obj.ID, "error", err)
			continue
		}
		deleted++
	}

	if deleted > 0 {
		slog.Info("emergent: deleted pending pairing requests for host", "host_id", hostID, "count", deleted)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Internal lookup helpers
// ---------------------------------------------------------------------------

func (s *EmergentSlaveStore) lookupSlaveByHostID(ctx context.Context, hostID string) (*graph.GraphObject, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  slaveServerType,
		Label: hostIDLabel(hostID),
		Limit: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent lookup slave by host_id: %w", err)
	}
	if len(resp.Items) == 0 {
		return nil, nil
	}
	return resp.Items[0], nil
}

func (s *EmergentSlaveStore) lookupPairingRequestByCode(ctx context.Context, pairingCode string) (*graph.GraphObject, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  pairingRequestType,
		Label: pairingCodeLabel(pairingCode),
		Limit: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent lookup pairing request: %w", err)
	}
	if len(resp.Items) == 0 {
		return nil, nil
	}
	return resp.Items[0], nil
}
