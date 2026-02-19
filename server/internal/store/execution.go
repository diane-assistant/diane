package store

import (
	"context"

	"github.com/diane-assistant/diane/internal/db"
)

// ExecutionStore defines the interface for job execution log storage operations.
type ExecutionStore interface {
	// CreateJobExecution creates a new execution entry for a job. Returns the execution ID.
	CreateJobExecution(ctx context.Context, jobID int64) (int64, error)

	// UpdateJobExecution updates an execution with its results.
	UpdateJobExecution(ctx context.Context, id int64, exitCode int, stdout, stderr string, execErr error) error

	// GetJobExecution retrieves a single execution by its legacy ID.
	GetJobExecution(ctx context.Context, id int64) (*db.JobExecution, error)

	// ListJobExecutions returns executions, optionally filtered by job ID.
	ListJobExecutions(ctx context.Context, jobID *int64, limit, offset int) ([]*db.JobExecution, error)

	// DeleteOldExecutions removes executions older than retentionDays. Returns the count deleted.
	DeleteOldExecutions(ctx context.Context, retentionDays int) (int64, error)
}
