package store

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"time"

	sdk "github.com/emergent-company/emergent/apps/server-go/pkg/sdk"
	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"

	"github.com/diane-assistant/diane/internal/db"
)

// EmergentJobStore implements JobStore against the Emergent graph API.
//
// Mapping:
//
//	Job:
//	  - Graph object type: "job"
//	  - ID (auto-increment) -> properties.legacy_id + label "legacy_id:{id}"
//	  - Name (unique)       -> properties.name + label "name:{name}"
//	  - Command             -> properties.command
//	  - Schedule            -> properties.schedule
//	  - Enabled             -> properties.enabled (bool) + label "enabled:{true|false}"
//	  - ActionType          -> properties.action_type
//	  - AgentName           -> properties.agent_name (nullable)
//	  - CreatedAt           -> properties.created_at (RFC3339Nano)
//	  - UpdatedAt           -> properties.updated_at (RFC3339Nano)
type EmergentJobStore struct {
	client *sdk.Client
}

const (
	jobType = "job"
)

// NewEmergentJobStore creates a new Emergent-backed JobStore.
func NewEmergentJobStore(client *sdk.Client) *EmergentJobStore {
	return &EmergentJobStore{client: client}
}

// ---------------------------------------------------------------------------
// Label helpers
// ---------------------------------------------------------------------------

func jobLegacyIDLabel(id int64) string    { return fmt.Sprintf("legacy_id:%d", id) }
func jobNameLabel(name string) string     { return fmt.Sprintf("name:%s", name) }
func jobEnabledLabel(enabled bool) string { return fmt.Sprintf("enabled:%t", enabled) }

// jobLabels builds the full label set for a job.
func jobLabels(j *db.Job) []string {
	labels := []string{
		jobLegacyIDLabel(j.ID),
		jobNameLabel(j.Name),
		jobEnabledLabel(j.Enabled),
	}
	return labels
}

// ---------------------------------------------------------------------------
// Conversion helpers
// ---------------------------------------------------------------------------

func jobToProperties(j *db.Job) map[string]any {
	now := time.Now().UTC()
	props := map[string]any{
		"name":        j.Name,
		"command":     j.Command,
		"schedule":    j.Schedule,
		"enabled":     j.Enabled,
		"action_type": j.ActionType,
		"updated_at":  now.Format(time.RFC3339Nano),
	}
	if j.ID != 0 {
		props["legacy_id"] = j.ID
	}
	if !j.CreatedAt.IsZero() {
		props["created_at"] = j.CreatedAt.Format(time.RFC3339Nano)
	} else {
		props["created_at"] = now.Format(time.RFC3339Nano)
	}
	if j.AgentName != nil {
		props["agent_name"] = *j.AgentName
	}
	return props
}

func jobFromObject(obj *graph.GraphObject) (*db.Job, error) {
	j := &db.Job{}

	// legacy_id
	if v, ok := obj.Properties["legacy_id"]; ok {
		j.ID = toInt64(v)
	}
	if v, ok := obj.Properties["name"].(string); ok {
		j.Name = v
	}
	if v, ok := obj.Properties["command"].(string); ok {
		j.Command = v
	}
	if v, ok := obj.Properties["schedule"].(string); ok {
		j.Schedule = v
	}
	if v, ok := obj.Properties["enabled"].(bool); ok {
		j.Enabled = v
	}
	if v, ok := obj.Properties["action_type"].(string); ok {
		j.ActionType = v
	}
	if j.ActionType == "" {
		j.ActionType = "shell"
	}
	if v, ok := obj.Properties["agent_name"].(string); ok {
		j.AgentName = &v
	}
	if v, ok := obj.Properties["created_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			j.CreatedAt = t
		}
	}
	if j.CreatedAt.IsZero() {
		j.CreatedAt = obj.CreatedAt
	}
	if v, ok := obj.Properties["updated_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			j.UpdatedAt = t
		}
	}
	if j.UpdatedAt.IsZero() {
		j.UpdatedAt = j.CreatedAt
	}
	return j, nil
}

// toInt64 converts a JSON number (float64, json.Number, int64) to int64.
func toInt64(v interface{}) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case json.Number:
		id, _ := n.Int64()
		return id
	case int64:
		return n
	}
	return 0
}

// nextJobLegacyID allocates the next legacy ID by scanning existing job objects.
func (s *EmergentJobStore) nextJobLegacyID(ctx context.Context) (int64, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  jobType,
		Limit: 1000,
	})
	if err != nil {
		return 0, fmt.Errorf("emergent list for next job ID: %w", err)
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
// JobStore implementation
// ---------------------------------------------------------------------------

func (s *EmergentJobStore) CreateJob(ctx context.Context, name, command, schedule string) (*db.Job, error) {
	return s.CreateJobWithAction(ctx, name, command, schedule, "shell", nil)
}

func (s *EmergentJobStore) CreateJobWithAction(ctx context.Context, name, command, schedule, actionType string, agentName *string) (*db.Job, error) {
	id, err := s.nextJobLegacyID(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	j := &db.Job{
		ID:         id,
		Name:       name,
		Command:    command,
		Schedule:   schedule,
		Enabled:    true,
		ActionType: actionType,
		AgentName:  agentName,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	props := jobToProperties(j)
	labels := jobLabels(j)
	status := "active"

	obj, err := s.client.Graph.CreateObject(ctx, &graph.CreateObjectRequest{
		Type:       jobType,
		Status:     &status,
		Properties: props,
		Labels:     labels,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent create job: %w", err)
	}

	slog.Info("emergent: created job", "name", name, "legacy_id", id, "object_id", obj.ID)
	return j, nil
}

func (s *EmergentJobStore) GetJob(ctx context.Context, id int64) (*db.Job, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  jobType,
		Label: jobLegacyIDLabel(id),
		Limit: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent lookup job by id %d: %w", id, err)
	}
	if len(resp.Items) == 0 {
		return nil, fmt.Errorf("job not found: id=%d", id)
	}
	return jobFromObject(resp.Items[0])
}

func (s *EmergentJobStore) GetJobByName(ctx context.Context, name string) (*db.Job, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  jobType,
		Label: jobNameLabel(name),
		Limit: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent lookup job by name %q: %w", name, err)
	}
	if len(resp.Items) == 0 {
		return nil, fmt.Errorf("job not found: name=%s", name)
	}
	return jobFromObject(resp.Items[0])
}

func (s *EmergentJobStore) ListJobs(ctx context.Context, enabledOnly bool) ([]*db.Job, error) {
	opts := &graph.ListObjectsOptions{
		Type:  jobType,
		Limit: 1000,
	}
	if enabledOnly {
		opts.Label = jobEnabledLabel(true)
	}

	resp, err := s.client.Graph.ListObjects(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("emergent list jobs: %w", err)
	}

	jobs := make([]*db.Job, 0, len(resp.Items))
	for _, obj := range resp.Items {
		j, err := jobFromObject(obj)
		if err != nil {
			slog.Warn("skipping malformed job", "object_id", obj.ID, "error", err)
			continue
		}
		jobs = append(jobs, j)
	}

	sort.Slice(jobs, func(i, k int) bool { return jobs[i].Name < jobs[k].Name })
	return jobs, nil
}

func (s *EmergentJobStore) UpdateJob(ctx context.Context, id int64, command, schedule *string, enabled *bool) error {
	return s.UpdateJobFull(ctx, id, command, schedule, enabled, nil, nil)
}

func (s *EmergentJobStore) UpdateJobFull(ctx context.Context, id int64, command, schedule *string, enabled *bool, actionType *string, agentName *string) error {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  jobType,
		Label: jobLegacyIDLabel(id),
		Limit: 1,
	})
	if err != nil {
		return fmt.Errorf("emergent lookup job for update: %w", err)
	}
	if len(resp.Items) == 0 {
		return fmt.Errorf("job not found: id=%d", id)
	}

	obj := resp.Items[0]

	// Parse current job from object
	j, err := jobFromObject(obj)
	if err != nil {
		return err
	}

	// Apply updates
	if command != nil {
		j.Command = *command
	}
	if schedule != nil {
		j.Schedule = *schedule
	}
	if enabled != nil {
		j.Enabled = *enabled
	}
	if actionType != nil {
		j.ActionType = *actionType
	}
	if agentName != nil {
		j.AgentName = agentName
	}
	j.UpdatedAt = time.Now().UTC()

	props := jobToProperties(j)
	labels := jobLabels(j)

	_, err = s.client.Graph.UpdateObject(ctx, obj.ID, &graph.UpdateObjectRequest{
		Properties:    props,
		Labels:        labels,
		ReplaceLabels: true,
	})
	if err != nil {
		return fmt.Errorf("emergent update job: %w", err)
	}

	slog.Info("emergent: updated job", "name", j.Name, "legacy_id", id)
	return nil
}

func (s *EmergentJobStore) DeleteJob(ctx context.Context, id int64) error {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  jobType,
		Label: jobLegacyIDLabel(id),
		Limit: 1,
	})
	if err != nil {
		return fmt.Errorf("emergent lookup job for delete: %w", err)
	}
	if len(resp.Items) == 0 {
		return nil // Already deleted
	}

	err = s.client.Graph.DeleteObject(ctx, resp.Items[0].ID)
	if err != nil {
		return fmt.Errorf("emergent delete job: %w", err)
	}

	slog.Info("emergent: deleted job", "legacy_id", id, "object_id", resp.Items[0].ID)
	return nil
}
