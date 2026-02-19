package store

import (
	"context"

	"github.com/diane-assistant/diane/internal/db"
)

// JobStore defines the interface for job storage operations.
type JobStore interface {
	// CreateJob creates a new job with a shell action type.
	CreateJob(ctx context.Context, name, command, schedule string) (*db.Job, error)

	// CreateJobWithAction creates a new job with an explicit action type.
	CreateJobWithAction(ctx context.Context, name, command, schedule, actionType string, agentName *string) (*db.Job, error)

	// GetJob retrieves a job by its legacy ID.
	GetJob(ctx context.Context, id int64) (*db.Job, error)

	// GetJobByName retrieves a job by its unique name.
	GetJobByName(ctx context.Context, name string) (*db.Job, error)

	// ListJobs returns all jobs, optionally filtering to enabled-only.
	ListJobs(ctx context.Context, enabledOnly bool) ([]*db.Job, error)

	// UpdateJob applies partial updates to a job (command, schedule, enabled).
	UpdateJob(ctx context.Context, id int64, command, schedule *string, enabled *bool) error

	// UpdateJobFull applies partial updates including action type and agent name.
	UpdateJobFull(ctx context.Context, id int64, command, schedule *string, enabled *bool, actionType *string, agentName *string) error

	// DeleteJob removes a job by its legacy ID.
	DeleteJob(ctx context.Context, id int64) error
}
