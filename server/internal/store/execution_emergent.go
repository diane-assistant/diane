package store

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	sdk "github.com/emergent-company/emergent/apps/server-go/pkg/sdk"
	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"

	"github.com/diane-assistant/diane/internal/db"
)

// EmergentExecutionStore implements ExecutionStore against the Emergent graph API.
//
// Mapping:
//
//	JobExecution:
//	  - Graph object type: "job_execution"
//	  - ID            -> properties.legacy_id + label "legacy_id:{id}"
//	  - JobID         -> properties.job_id + label "job_id:{id}"
//	  - StartedAt     -> properties.started_at (RFC3339Nano)
//	  - EndedAt       -> properties.ended_at (RFC3339Nano, nullable)
//	  - ExitCode      -> properties.exit_code (nullable)
//	  - Stdout        -> properties.stdout
//	  - Stderr        -> properties.stderr
//	  - Error         -> properties.error (nullable)
type EmergentExecutionStore struct {
	client *sdk.Client
}

const (
	jobExecutionType = "job_execution"
)

// NewEmergentExecutionStore creates a new Emergent-backed ExecutionStore.
func NewEmergentExecutionStore(client *sdk.Client) *EmergentExecutionStore {
	return &EmergentExecutionStore{client: client}
}

// ---------------------------------------------------------------------------
// Label helpers
// ---------------------------------------------------------------------------

func execLegacyIDLabel(id int64) string { return fmt.Sprintf("legacy_id:%d", id) }
func execJobIDLabel(jobID int64) string { return fmt.Sprintf("job_id:%d", jobID) }

// ---------------------------------------------------------------------------
// Conversion helpers
// ---------------------------------------------------------------------------

func executionToProperties(e *db.JobExecution) map[string]any {
	props := map[string]any{
		"legacy_id":  e.ID,
		"job_id":     e.JobID,
		"started_at": e.StartedAt.Format(time.RFC3339Nano),
		"stdout":     e.Stdout,
		"stderr":     e.Stderr,
	}
	if e.EndedAt != nil {
		props["ended_at"] = e.EndedAt.Format(time.RFC3339Nano)
	}
	if e.ExitCode != nil {
		props["exit_code"] = *e.ExitCode
	}
	if e.Error != nil {
		props["error"] = *e.Error
	}
	return props
}

func executionFromObject(obj *graph.GraphObject) (*db.JobExecution, error) {
	e := &db.JobExecution{}

	if v, ok := obj.Properties["legacy_id"]; ok {
		e.ID = toInt64(v)
	}
	if v, ok := obj.Properties["job_id"]; ok {
		e.JobID = toInt64(v)
	}
	if v, ok := obj.Properties["started_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			e.StartedAt = t
		}
	}
	if e.StartedAt.IsZero() {
		e.StartedAt = obj.CreatedAt
	}
	if v, ok := obj.Properties["ended_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			e.EndedAt = &t
		}
	}
	if v, ok := obj.Properties["exit_code"]; ok && v != nil {
		code := int(toInt64(v))
		e.ExitCode = &code
	}
	if v, ok := obj.Properties["stdout"].(string); ok {
		e.Stdout = v
	}
	if v, ok := obj.Properties["stderr"].(string); ok {
		e.Stderr = v
	}
	if v, ok := obj.Properties["error"].(string); ok {
		e.Error = &v
	}

	return e, nil
}

// nextExecLegacyID allocates the next legacy ID for executions.
func (s *EmergentExecutionStore) nextExecLegacyID(ctx context.Context) (int64, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  jobExecutionType,
		Limit: 1000,
	})
	if err != nil {
		return 0, fmt.Errorf("emergent list for next execution ID: %w", err)
	}

	var maxID int64
	for _, obj := range resp.Items {
		if id := toInt64(obj.Properties["legacy_id"]); id > maxID {
			maxID = id
		}
	}
	return maxID + 1, nil
}

// ---------------------------------------------------------------------------
// ExecutionStore implementation
// ---------------------------------------------------------------------------

func (s *EmergentExecutionStore) CreateJobExecution(ctx context.Context, jobID int64) (int64, error) {
	id, err := s.nextExecLegacyID(ctx)
	if err != nil {
		return 0, err
	}

	now := time.Now().UTC()
	e := &db.JobExecution{
		ID:        id,
		JobID:     jobID,
		StartedAt: now,
	}

	props := executionToProperties(e)
	labels := []string{
		execLegacyIDLabel(id),
		execJobIDLabel(jobID),
	}
	status := "active"

	obj, err := s.client.Graph.CreateObject(ctx, &graph.CreateObjectRequest{
		Type:       jobExecutionType,
		Status:     &status,
		Properties: props,
		Labels:     labels,
	})
	if err != nil {
		return 0, fmt.Errorf("emergent create job execution: %w", err)
	}

	slog.Info("emergent: created job execution", "legacy_id", id, "job_id", jobID, "object_id", obj.ID)
	return id, nil
}

func (s *EmergentExecutionStore) UpdateJobExecution(ctx context.Context, id int64, exitCode int, stdout, stderr string, execErr error) error {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  jobExecutionType,
		Label: execLegacyIDLabel(id),
		Limit: 1,
	})
	if err != nil {
		return fmt.Errorf("emergent lookup execution for update: %w", err)
	}
	if len(resp.Items) == 0 {
		return fmt.Errorf("execution not found: id=%d", id)
	}

	obj := resp.Items[0]
	now := time.Now().UTC()

	props := map[string]any{
		"ended_at":  now.Format(time.RFC3339Nano),
		"exit_code": exitCode,
		"stdout":    stdout,
		"stderr":    stderr,
	}
	if execErr != nil {
		props["error"] = execErr.Error()
	}

	_, err = s.client.Graph.UpdateObject(ctx, obj.ID, &graph.UpdateObjectRequest{
		Properties: props,
	})
	if err != nil {
		return fmt.Errorf("emergent update job execution: %w", err)
	}

	slog.Info("emergent: updated job execution", "legacy_id", id)
	return nil
}

func (s *EmergentExecutionStore) GetJobExecution(ctx context.Context, id int64) (*db.JobExecution, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  jobExecutionType,
		Label: execLegacyIDLabel(id),
		Limit: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent lookup execution by id %d: %w", id, err)
	}
	if len(resp.Items) == 0 {
		return nil, fmt.Errorf("execution not found: id=%d", id)
	}
	return executionFromObject(resp.Items[0])
}

func (s *EmergentExecutionStore) ListJobExecutions(ctx context.Context, jobID *int64, limit, offset int) ([]*db.JobExecution, error) {
	opts := &graph.ListObjectsOptions{
		Type:  jobExecutionType,
		Limit: 1000,
	}
	if jobID != nil {
		opts.Label = execJobIDLabel(*jobID)
	}

	resp, err := s.client.Graph.ListObjects(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("emergent list job executions: %w", err)
	}

	executions := make([]*db.JobExecution, 0, len(resp.Items))
	for _, obj := range resp.Items {
		e, err := executionFromObject(obj)
		if err != nil {
			slog.Warn("skipping malformed execution", "object_id", obj.ID, "error", err)
			continue
		}
		executions = append(executions, e)
	}

	// Sort by started_at descending (newest first)
	sort.Slice(executions, func(i, k int) bool {
		return executions[i].StartedAt.After(executions[k].StartedAt)
	})

	// Apply offset and limit
	if offset >= len(executions) {
		return []*db.JobExecution{}, nil
	}
	executions = executions[offset:]
	if limit > 0 && limit < len(executions) {
		executions = executions[:limit]
	}

	return executions, nil
}

func (s *EmergentExecutionStore) DeleteOldExecutions(ctx context.Context, retentionDays int) (int64, error) {
	cutoff := time.Now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour)

	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  jobExecutionType,
		Limit: 1000,
	})
	if err != nil {
		return 0, fmt.Errorf("emergent list executions for cleanup: %w", err)
	}

	var deleted int64
	for _, obj := range resp.Items {
		var startedAt time.Time
		if v, ok := obj.Properties["started_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
				startedAt = t
			}
		}
		if startedAt.IsZero() {
			startedAt = obj.CreatedAt
		}

		if startedAt.Before(cutoff) {
			if err := s.client.Graph.DeleteObject(ctx, obj.ID); err != nil {
				slog.Warn("failed to delete old execution", "object_id", obj.ID, "error", err)
				continue
			}
			deleted++
		}
	}

	slog.Info("emergent: deleted old executions", "count", deleted, "retention_days", retentionDays)
	return deleted, nil
}
