package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/diane-assistant/diane/internal/slave"
)

// SlaveInfo represents information about a slave server for API responses
type SlaveInfo struct {
	Hostname    string `json:"hostname"`
	Status      string `json:"status"`
	ToolCount   int    `json:"tool_count"`
	LastSeen    string `json:"last_seen,omitempty"`
	ConnectedAt string `json:"connected_at,omitempty"`
	CertSerial  string `json:"cert_serial"`
	IssuedAt    string `json:"issued_at"`
	ExpiresAt   string `json:"expires_at"`
	Enabled     bool   `json:"enabled"`
}

// PairingRequest represents a pairing request for API responses
type PairingRequest struct {
	Hostname    string `json:"hostname"`
	PairingCode string `json:"pairing_code"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	ExpiresAt   string `json:"expires_at"`
}

// PairRequestBody is the request body for initiating pairing
type PairRequestBody struct {
	Hostname string `json:"hostname"`
	CSR      string `json:"csr"`
}

// ApproveRequestBody is the request body for approving a pairing request
type ApproveRequestBody struct {
	Hostname    string `json:"hostname"`
	PairingCode string `json:"pairing_code"`
}

// RevokeRequestBody is the request body for revoking slave credentials
type RevokeRequestBody struct {
	Hostname string `json:"hostname"`
	Reason   string `json:"reason,omitempty"`
}

// handleSlaves handles GET /api/slaves - list all slaves
func (s *Server) handleSlaves(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.slaveManager == nil {
		http.Error(w, "Slave manager not initialized", http.StatusServiceUnavailable)
		return
	}

	slaves, err := s.slaveManager.GetRegistry().GetAllSlaves()
	if err != nil {
		slog.Error("Failed to list slaves", "error", err)
		http.Error(w, "Failed to list slaves", http.StatusInternalServerError)
		return
	}

	response := make([]SlaveInfo, 0, len(slaves))
	for _, slave := range slaves {
		info := SlaveInfo{
			Hostname:   slave.HostID,
			Status:     string(slave.Status),
			ToolCount:  slave.ToolCount,
			CertSerial: slave.CertSerial,
			IssuedAt:   slave.IssuedAt.Format(time.RFC3339),
			ExpiresAt:  slave.ExpiresAt.Format(time.RFC3339),
			Enabled:    slave.Enabled,
		}

		if slave.LastHeartbeat != nil {
			info.LastSeen = slave.LastHeartbeat.Format(time.RFC3339)
		}
		if slave.ConnectedAt != nil {
			info.ConnectedAt = slave.ConnectedAt.Format(time.RFC3339)
		}

		response = append(response, info)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSlavePending handles GET /api/slaves/pending - list pending pairing requests
func (s *Server) handlePendingSlaves(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.slaveManager == nil {
		http.Error(w, "Slave manager not initialized", http.StatusServiceUnavailable)
		return
	}

	pendingReqs := s.slaveManager.GetPairingService().GetPendingRequests()

	response := make([]PairingRequest, 0, len(pendingReqs))
	for _, req := range pendingReqs {
		response = append(response, PairingRequest{
			Hostname:    req.HostID,
			PairingCode: req.PairingCode,
			Status:      "pending",
			CreatedAt:   req.RequestedAt.Format(time.RFC3339),
			ExpiresAt:   req.ExpiresAt.Format(time.RFC3339),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSlavePair handles POST /api/slaves/pair - initiate pairing
func (s *Server) handlePairSlave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PairRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Hostname == "" {
		http.Error(w, "hostname is required", http.StatusBadRequest)
		return
	}

	if req.CSR == "" {
		http.Error(w, "csr is required", http.StatusBadRequest)
		return
	}

	if s.slaveManager == nil {
		http.Error(w, "Slave manager not initialized", http.StatusServiceUnavailable)
		return
	}

	// Create pairing request
	code, err := s.slaveManager.GetPairingService().CreatePairingRequest(req.Hostname, []byte(req.CSR))
	if err != nil {
		slog.Error("Failed to create pairing request", "hostname", req.Hostname, "error", err)
		http.Error(w, fmt.Sprintf("Failed to create pairing request: %v", err), http.StatusBadRequest)
		return
	}

	slog.Info("Pairing initiated", "hostname", req.Hostname, "code", code)

	response := map[string]interface{}{
		"success":      true,
		"message":      "Pairing initiated",
		"pairing_code": code,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSlaveApprove handles POST /api/slaves/approve - approve pairing request
func (s *Server) handleApproveSlave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ApproveRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Hostname == "" {
		http.Error(w, "hostname is required", http.StatusBadRequest)
		return
	}

	if req.PairingCode == "" {
		http.Error(w, "pairing_code is required", http.StatusBadRequest)
		return
	}

	if s.slaveManager == nil {
		http.Error(w, "Slave manager not initialized", http.StatusServiceUnavailable)
		return
	}

	// Approve the pairing request
	certPEM, caCertPEM, err := s.slaveManager.GetPairingService().ApprovePairingRequest(req.Hostname, req.PairingCode)
	if err != nil {
		slog.Error("Failed to approve pairing request", "hostname", req.Hostname, "error", err)
		http.Error(w, fmt.Sprintf("Failed to approve pairing: %v", err), http.StatusBadRequest)
		return
	}

	slog.Info("Pairing approved", "hostname", req.Hostname)

	response := map[string]interface{}{
		"success":     true,
		"message":     "Pairing approved successfully",
		"certificate": string(certPEM),
		"ca_cert":     string(caCertPEM),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSlaveDeny handles POST /api/slaves/deny - deny pairing request
func (s *Server) handleDenySlave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ApproveRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Hostname == "" {
		http.Error(w, "hostname is required", http.StatusBadRequest)
		return
	}

	if req.PairingCode == "" {
		http.Error(w, "pairing_code is required", http.StatusBadRequest)
		return
	}

	if s.slaveManager == nil {
		http.Error(w, "Slave manager not initialized", http.StatusServiceUnavailable)
		return
	}

	// Deny the pairing request
	if err := s.slaveManager.GetPairingService().DenyPairingRequest(req.Hostname, req.PairingCode); err != nil {
		slog.Error("Failed to deny pairing request", "hostname", req.Hostname, "error", err)
		http.Error(w, fmt.Sprintf("Failed to deny pairing: %v", err), http.StatusBadRequest)
		return
	}

	slog.Info("Pairing denied", "hostname", req.Hostname)

	response := map[string]interface{}{
		"success": true,
		"message": "Pairing denied successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSlaveRevoke handles POST /api/slaves/revoke - revoke slave credentials
func (s *Server) handleRevokeSlave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RevokeRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Hostname == "" {
		http.Error(w, "hostname is required", http.StatusBadRequest)
		return
	}

	// TODO: Implement credential revocation
	slog.Info("Credential revocation requested", "hostname", req.Hostname, "reason", req.Reason)

	response := map[string]interface{}{
		"success": false,
		"message": "Credential revocation not yet fully implemented",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSlaveRevoked handles GET /api/slaves/revoked - list revoked credentials
func (s *Server) handleRevokedSlaves(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.slaveManager == nil {
		http.Error(w, "Slave manager not initialized", http.StatusServiceUnavailable)
		return
	}

	revoked, err := s.slaveManager.ListRevokedCredentials()
	if err != nil {
		slog.Error("Failed to list revoked credentials", "error", err)
		http.Error(w, "Failed to list revoked credentials", http.StatusInternalServerError)
		return
	}

	response := make([]map[string]interface{}, 0, len(revoked))
	for _, r := range revoked {
		response = append(response, map[string]interface{}{
			"hostname":    r.HostID,
			"cert_serial": r.CertSerial,
			"revoked_at":  r.RevokedAt.Format(time.RFC3339),
			"reason":      r.Reason,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSlaveAction handles actions on specific slaves (DELETE, etc.)
func (s *Server) handleSlaveAction(w http.ResponseWriter, r *http.Request) {
	// Extract hostname from path: /api/slaves/{hostname}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/slaves/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Hostname required", http.StatusBadRequest)
		return
	}

	hostname := parts[0]

	if s.slaveManager == nil {
		http.Error(w, "Slave manager not initialized", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodDelete:
		// Revoke and delete a slave
		slog.Info("Slave deletion requested", "hostname", hostname)

		if err := s.slaveManager.RevokeCredential(hostname, "Deleted by user"); err != nil {
			slog.Error("Failed to revoke slave credentials", "hostname", hostname, "error", err)
			http.Error(w, "Failed to delete slave", http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{
			"success": true,
			"message": "Slave deleted and credentials revoked",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)

	case http.MethodGet:
		// Get detailed info about a specific slave
		slog.Info("Slave info requested", "hostname", hostname)

		slaves, err := s.slaveManager.GetRegistry().GetAllSlaves()
		if err != nil {
			slog.Error("Failed to list slaves", "error", err)
			http.Error(w, "Failed to retrieve slave info", http.StatusInternalServerError)
			return
		}

		var slaveInfo *slave.SlaveInfo
		for _, s := range slaves {
			if s.HostID == hostname {
				slaveInfo = s
				break
			}
		}

		if slaveInfo == nil {
			http.Error(w, "Slave not found", http.StatusNotFound)
			return
		}

		info := SlaveInfo{
			Hostname:   slaveInfo.HostID,
			Status:     string(slaveInfo.Status),
			ToolCount:  slaveInfo.ToolCount,
			CertSerial: slaveInfo.CertSerial,
			IssuedAt:   slaveInfo.IssuedAt.Format(time.RFC3339),
			ExpiresAt:  slaveInfo.ExpiresAt.Format(time.RFC3339),
			Enabled:    slaveInfo.Enabled,
		}

		if slaveInfo.LastHeartbeat != nil {
			info.LastSeen = slaveInfo.LastHeartbeat.Format(time.RFC3339)
		}
		if slaveInfo.ConnectedAt != nil {
			info.ConnectedAt = slaveInfo.ConnectedAt.Format(time.RFC3339)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSlaveHealth handles GET /api/slaves/health - health check for slave connectivity
func (s *Server) handleSlaveHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	healthy := s.slaveManager != nil
	message := "Slave management active"
	if !healthy {
		message = "Slave management not initialized"
	}

	response := map[string]interface{}{
		"healthy": healthy,
		"message": message,
	}

	if healthy {
		// Add count of connected slaves
		slaves := s.slaveManager.GetRegistry().GetConnectedSlaves()
		response["connected_count"] = len(slaves)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// RegisterSlaveRoutes registers all slave management routes
func RegisterSlaveRoutes(mux *http.ServeMux, server *Server) {
	mux.HandleFunc("/slaves", server.handleSlaves)
	mux.HandleFunc("/slaves/pending", server.handlePendingSlaves)
	mux.HandleFunc("/slaves/pair", server.handlePairSlave)
	mux.HandleFunc("/slaves/approve", server.handleApproveSlave)
	mux.HandleFunc("/slaves/deny", server.handleDenySlave)
	mux.HandleFunc("/slaves/revoke", server.handleRevokeSlave)
	mux.HandleFunc("/slaves/revoked", server.handleRevokedSlaves)
	mux.HandleFunc("/slaves/health", server.handleSlaveHealth)
	mux.HandleFunc("/slaves/", server.handleSlaveAction)
}

// Ensure slave package is imported
var _ = slave.NewRegistry
