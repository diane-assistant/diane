// Package store defines storage interfaces for Diane entities, enabling
// migration from SQLite to Emergent. Each entity type gets its own
// interface, SQLite adapter, Emergent implementation, and dual-write wrapper.
//
// TODO(emergent-migration): Future interfaces to create in this package:
//   - JobStore       (see internal/db/jobs.go for method signatures)
//   - ExecutionStore (see internal/db/executions.go)
//   - AgentStore     (see internal/db/agents.go — includes AgentLog)
//
// Each follows the same pattern as ProviderStore:
//  1. {entity}.go          — interface definition
//  2. {entity}_sqlite.go   — thin wrapper around *db.DB
//  3. {entity}_emergent.go — Emergent graph API implementation
//  4. {entity}_dual.go     — dual-write wrapper
package store

import (
	"github.com/diane-assistant/diane/internal/db"
)

// ProviderStore defines the data access interface for providers.
// All implementations (SQLite, Emergent, dual-write) satisfy this interface.
type ProviderStore interface {
	CreateProvider(p *db.Provider) (int64, error)
	GetProvider(id int64) (*db.Provider, error)
	GetProviderByName(name string) (*db.Provider, error)
	ListProviders() ([]*db.Provider, error)
	ListProvidersByType(ptype db.ProviderType) ([]*db.Provider, error)
	ListEnabledProvidersByType(ptype db.ProviderType) ([]*db.Provider, error)
	GetDefaultProvider(ptype db.ProviderType) (*db.Provider, error)
	UpdateProvider(p *db.Provider) error
	DeleteProvider(id int64) error
	SetDefaultProvider(id int64) error
	EnableProvider(id int64) error
	DisableProvider(id int64) error
}
