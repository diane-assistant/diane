package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/diane-assistant/diane/internal/db"
	"github.com/diane-assistant/diane/internal/slave"
)

// HostsAPI handles host/node information endpoints
type HostsAPI struct {
	db           *db.DB
	slaveManager *slave.Manager
}

// NewHostsAPI creates a new HostsAPI
func NewHostsAPI(database *db.DB, slaveManager *slave.Manager) *HostsAPI {
	slog.Info("NewHostsAPI called", "database_nil", database == nil, "slaveManager_nil", slaveManager == nil)
	return &HostsAPI{db: database, slaveManager: slaveManager}
}

// HostInfo represents a host/node in the system
type HostInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"` // "master" or "slave"
	Platform string `json:"platform,omitempty"`
	Online   bool   `json:"online"`
}

// RegisterRoutes registers host API routes on the given mux
func (api *HostsAPI) RegisterRoutes(mux *http.ServeMux) {
	slog.Info("HostsAPI.RegisterRoutes called, registering /hosts")
	mux.HandleFunc("/hosts", api.handleHosts)
	slog.Info("HostsAPI: /hosts route registered successfully")
}

// handleHosts handles GET /hosts
func (api *HostsAPI) handleHosts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	var hosts []HostInfo

	// Always include master node
	hosts = append(hosts, HostInfo{
		ID:     "master",
		Name:   "Master",
		Type:   "master",
		Online: true, // Master is always online (we're running on it)
	})

	// Add slave nodes from database
	if api.db != nil {
		slaves, err := api.db.ListSlaveServers()
		if err != nil {
			// Log error but don't fail the request
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		for _, slave := range slaves {
			// Check if slave is online via manager's registry
			online := false
			if api.slaveManager != nil && api.slaveManager.GetRegistry() != nil {
				online = api.slaveManager.GetRegistry().IsConnected(slave.ID)
			}

			hosts = append(hosts, HostInfo{
				ID:       slave.ID,
				Name:     slave.HostID,
				Type:     "slave",
				Platform: slave.Platform,
				Online:   online,
			})
		}
	}

	json.NewEncoder(w).Encode(hosts)
}
