package db

// TODO(emergent-migration): JobExecution entity must be migrated to Emergent.
// This is tightly coupled with Jobs — migrate together.
// Steps:
//   1. Create ExecutionStore interface in internal/store/execution.go
//   2. Graph object type: "job_execution"
//   3. Relationship: job -> has_execution -> job_execution
//   4. Properties: started_at, ended_at, exit_code, stdout, stderr, error
//   5. Labels: legacy_id:{id}
//   6. DeleteOldExecutions maps to listing by temporal filter + bulk delete
// Tracking: docs/EMERGENT_MIGRATION_PLAN.md

import (
	"fmt"
)

// CreateJobExecution creates a new job execution log entry
// DEPRECATED: Stub for refactoring. Implement ExecutionStore interface instead.
func (db *DB) CreateJobExecution(jobID int64) (int64, error) {
	return 0, fmt.Errorf("CreateJobExecution: not implemented - refactor to ExecutionStore")
}

// UpdateJobExecution updates a job execution with results
// DEPRECATED: Stub for refactoring. Implement ExecutionStore interface instead.
func (db *DB) UpdateJobExecution(id int64, exitCode int, stdout, stderr string, execErr error) error {
	return fmt.Errorf("UpdateJobExecution: not implemented - refactor to ExecutionStore")
}

// GetJobExecution retrieves a single job execution by ID
// DEPRECATED: Stub for refactoring. Implement ExecutionStore interface instead.
func (db *DB) GetJobExecution(id int64) (*JobExecution, error) {
	return nil, fmt.Errorf("GetJobExecution: not implemented - refactor to ExecutionStore")
}

// ListJobExecutions retrieves job executions with optional filters
// DEPRECATED: Stub for refactoring. Implement ExecutionStore interface instead.
func (db *DB) ListJobExecutions(jobID *int64, limit, offset int) ([]*JobExecution, error) {
	return nil, fmt.Errorf("ListJobExecutions: not implemented - refactor to ExecutionStore")
}

// DeleteOldExecutions deletes job executions older than the specified duration
// DEPRECATED: Stub for refactoring. Implement ExecutionStore interface instead.
func (db *DB) DeleteOldExecutions(retentionDays int) (int64, error) {
	return 0, fmt.Errorf("DeleteOldExecutions: not implemented - refactor to ExecutionStore")
}
