package db

// TODO(emergent-migration): Jobs entity must be migrated to Emergent.
// Steps:
//   1. Create JobStore interface in internal/store/job.go (matching all methods below)
//   2. Create EmergentJobStore in internal/store/job_emergent.go
//      - Graph object type: "job"
//      - Properties: name, command, schedule, enabled, action_type, agent_name
//      - Labels: legacy_id:{id}, name:{name}
//   3. Wire JobStore into MCP tools (mcp/tools/jobs/) and API layer
//   4. Migrate job_executions similarly (related entity, 1:N relationship via graph edges)
// Tracking: docs/EMERGENT_MIGRATION_PLAN.md

import (
	"fmt"
)

// CreateJob creates a new job in the database
// DEPRECATED: Stub for refactoring. Implement JobStore interface instead.
func (db *DB) CreateJob(name, command, schedule string) (*Job, error) {
	return nil, fmt.Errorf("CreateJob: not implemented - refactor to JobStore")
}

// CreateJobWithAction creates a new job with an action type
// DEPRECATED: Stub for refactoring. Implement JobStore interface instead.
func (db *DB) CreateJobWithAction(name, command, schedule, actionType string, agentName *string) (*Job, error) {
	return nil, fmt.Errorf("CreateJobWithAction: not implemented - refactor to JobStore")
}

// GetJob retrieves a job by ID
// DEPRECATED: Stub for refactoring. Implement JobStore interface instead.
func (db *DB) GetJob(id int64) (*Job, error) {
	return nil, fmt.Errorf("GetJob: not implemented - refactor to JobStore")
}

// GetJobByName retrieves a job by name
// DEPRECATED: Stub for refactoring. Implement JobStore interface instead.
func (db *DB) GetJobByName(name string) (*Job, error) {
	return nil, fmt.Errorf("GetJobByName: not implemented - refactor to JobStore")
}

// ListJobs retrieves all jobs, optionally filtered by enabled status
// DEPRECATED: Stub for refactoring. Implement JobStore interface instead.
func (db *DB) ListJobs(enabledOnly bool) ([]*Job, error) {
	return nil, fmt.Errorf("ListJobs: not implemented - refactor to JobStore")
}

// UpdateJob updates a job's properties
// DEPRECATED: Stub for refactoring. Implement JobStore interface instead.
func (db *DB) UpdateJob(id int64, command, schedule *string, enabled *bool) error {
	return fmt.Errorf("UpdateJob: not implemented - refactor to JobStore")
}

// UpdateJobFull updates a job's properties including action type
// DEPRECATED: Stub for refactoring. Implement JobStore interface instead.
func (db *DB) UpdateJobFull(id int64, command, schedule *string, enabled *bool, actionType *string, agentName *string) error {
	return fmt.Errorf("UpdateJobFull: not implemented - refactor to JobStore")
}

// DeleteJob deletes a job by ID
// DEPRECATED: Stub for refactoring. Implement JobStore interface instead.
func (db *DB) DeleteJob(id int64) error {
	return fmt.Errorf("DeleteJob: not implemented - refactor to JobStore")
}
