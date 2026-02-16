// Package tasks implements the SpecMCP task management tools:
// spec_generate_tasks, spec_get_available_tasks, spec_assign_task,
// spec_complete_task, spec_get_critical_path.
package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/diane-assistant/diane/server/specmcp/internal/emergent"
	"github.com/diane-assistant/diane/server/specmcp/internal/mcp"
	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"
)

// --- spec_generate_tasks ---

type generateTasksParams struct {
	ChangeID string           `json:"change_id"`
	Tasks    []taskDefinition `json:"tasks"`
}

type taskDefinition struct {
	Number             string   `json:"number"`
	Description        string   `json:"description"`
	TaskType           string   `json:"task_type,omitempty"`
	ComplexityPoints   int      `json:"complexity_points,omitempty"`
	VerificationMethod string   `json:"verification_method,omitempty"`
	Implements         string   `json:"implements,omitempty"`
	Blocks             []string `json:"blocks,omitempty"`
	ParentTaskNumber   string   `json:"parent_task_number,omitempty"`
	Tags               []string `json:"tags,omitempty"`
}

type GenerateTasks struct {
	client *emergent.Client
}

func NewGenerateTasks(client *emergent.Client) *GenerateTasks {
	return &GenerateTasks{client: client}
}

func (t *GenerateTasks) Name() string { return "spec_generate_tasks" }
func (t *GenerateTasks) Description() string {
	return "Generate tasks for a change from a task list. Creates Task entities with dependencies (blocks/blocked_by), subtask relationships, and implements links. Tasks are created in order; blocking references use task numbers."
}
func (t *GenerateTasks) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "change_id": {
      "type": "string",
      "description": "ID of the change to generate tasks for"
    },
    "tasks": {
      "type": "array",
      "description": "Array of task definitions",
      "items": {
        "type": "object",
        "properties": {
          "number": {"type": "string", "description": "Task number (e.g. '1.1', '2.3')"},
          "description": {"type": "string", "description": "What the task does"},
          "task_type": {"type": "string", "enum": ["implementation", "testing", "documentation", "refactoring", "investigation"]},
          "complexity_points": {"type": "integer", "description": "1-10 complexity estimate"},
          "verification_method": {"type": "string", "description": "How to verify completion"},
          "implements": {"type": "string", "description": "ID of the entity this task implements"},
          "blocks": {"type": "array", "items": {"type": "string"}, "description": "Task numbers this task blocks"},
          "parent_task_number": {"type": "string", "description": "Parent task number for subtask relationship"},
          "tags": {"type": "array", "items": {"type": "string"}}
        },
        "required": ["number", "description"]
      }
    }
  },
  "required": ["change_id", "tasks"]
}`)
}

func (t *GenerateTasks) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p generateTasksParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}
	if p.ChangeID == "" {
		return mcp.ErrorResult("change_id is required"), nil
	}
	if len(p.Tasks) == 0 {
		return mcp.ErrorResult("at least one task is required"), nil
	}

	// Verify change exists and has a design
	_, err := t.client.GetChange(ctx, p.ChangeID)
	if err != nil {
		return mcp.ErrorResult(fmt.Sprintf("change not found: %v", err)), nil
	}

	// Create all tasks first to build a number→ID map
	numberToID := make(map[string]string)
	created := make([]map[string]any, 0, len(p.Tasks))

	for _, td := range p.Tasks {
		// Check for context cancellation between task creations
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("task generation cancelled: %w", ctx.Err())
		default:
		}

		task, err := t.client.CreateTask(ctx, p.ChangeID, &emergent.Task{
			Number:             td.Number,
			Description:        td.Description,
			TaskType:           td.TaskType,
			Status:             emergent.StatusPending,
			ComplexityPoints:   td.ComplexityPoints,
			VerificationMethod: td.VerificationMethod,
			Tags:               td.Tags,
		})
		if err != nil {
			return nil, fmt.Errorf("creating task %s: %w", td.Number, err)
		}
		numberToID[td.Number] = task.ID
		created = append(created, map[string]any{
			"id":                task.ID,
			"number":            td.Number,
			"description":       td.Description,
			"complexity_points": td.ComplexityPoints,
		})
	}

	// Now create relationships using the number→ID map
	relCount := 0
	for _, td := range p.Tasks {
		// Check for context cancellation between relationship batches
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("task relationship creation cancelled: %w", ctx.Err())
		default:
		}

		taskID := numberToID[td.Number]

		// Create blocks relationships
		for _, blocksNum := range td.Blocks {
			blockedID, ok := numberToID[blocksNum]
			if !ok {
				continue // silently skip unknown task numbers
			}
			if _, err := t.client.CreateRelationship(ctx, emergent.RelBlocks, taskID, blockedID, nil); err != nil {
				return nil, fmt.Errorf("creating blocks %s→%s: %w", td.Number, blocksNum, err)
			}
			relCount++
		}

		// Create subtask relationship
		if td.ParentTaskNumber != "" {
			parentID, ok := numberToID[td.ParentTaskNumber]
			if ok {
				if _, err := t.client.CreateRelationship(ctx, emergent.RelHasSubtask, parentID, taskID, nil); err != nil {
					return nil, fmt.Errorf("creating subtask %s→%s: %w", td.ParentTaskNumber, td.Number, err)
				}
				relCount++
			}
		}

		// Create implements relationship
		if td.Implements != "" {
			if _, err := t.client.CreateRelationship(ctx, emergent.RelImplements, taskID, td.Implements, nil); err != nil {
				return nil, fmt.Errorf("creating implements for %s: %w", td.Number, err)
			}
			relCount++
		}
	}

	// Calculate total complexity
	totalPoints := 0
	for _, td := range p.Tasks {
		totalPoints += td.ComplexityPoints
	}

	return mcp.JSONResult(map[string]any{
		"tasks":              created,
		"task_count":         len(created),
		"relationship_count": relCount,
		"total_complexity":   totalPoints,
		"message":            fmt.Sprintf("Generated %d tasks with %d relationships", len(created), relCount),
	})
}

// --- spec_get_available_tasks ---

type getAvailableTasksParams struct {
	ChangeID string `json:"change_id"`
}

type GetAvailableTasks struct {
	client *emergent.Client
}

func NewGetAvailableTasks(client *emergent.Client) *GetAvailableTasks {
	return &GetAvailableTasks{client: client}
}

func (t *GetAvailableTasks) Name() string { return "spec_get_available_tasks" }
func (t *GetAvailableTasks) Description() string {
	return "Get tasks that are available to work on: pending, not blocked by incomplete tasks, and not assigned."
}
func (t *GetAvailableTasks) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "change_id": {
      "type": "string",
      "description": "ID of the change to get available tasks for"
    }
  },
  "required": ["change_id"]
}`)
}

func (t *GetAvailableTasks) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p getAvailableTasksParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}
	if p.ChangeID == "" {
		return mcp.ErrorResult("change_id is required"), nil
	}

	tasks, err := t.client.ListTasks(ctx, p.ChangeID)
	if err != nil {
		return nil, fmt.Errorf("listing tasks: %w", err)
	}

	// Build task ID→status map
	taskStatus := make(map[string]string)
	for _, task := range tasks {
		taskStatus[task.ID] = task.Status
	}

	// Batch-fetch all blocks and assigned_to relationships in one ExpandGraph call
	// from the change, depth 2 (change → tasks → blocks/assigned_to targets)
	blockedByMap := make(map[string][]string) // taskID → []blockerIDs
	assignedSet := make(map[string]bool)      // taskIDs that have assignments

	resp, err := t.client.ExpandGraph(ctx, &graph.GraphExpandRequest{
		RootIDs:           []string{p.ChangeID},
		Direction:         "outgoing",
		MaxDepth:          2,
		MaxNodes:          500,
		MaxEdges:          1000,
		RelationshipTypes: []string{emergent.RelHasTask, emergent.RelBlocks, emergent.RelAssignedTo},
	})
	if err == nil {
		// Normalize edge IDs so SrcID/DstID reference the node's current .ID,
		// which matches the task.ID from ListTasks. Fixes canonical vs version ID mismatch.
		nodeIdx := emergent.NewNodeIndex(resp.Nodes)
		emergent.CanonicalizeEdgeIDs(resp.Edges, nodeIdx)

		for _, edge := range resp.Edges {
			switch edge.Type {
			case emergent.RelBlocks:
				// edge.SrcID blocks edge.DstID
				if _, ok := taskStatus[edge.DstID]; ok {
					blockedByMap[edge.DstID] = append(blockedByMap[edge.DstID], edge.SrcID)
				}
			case emergent.RelAssignedTo:
				if _, ok := taskStatus[edge.SrcID]; ok {
					assignedSet[edge.SrcID] = true
				}
			}
		}
	} else {
		// Fallback: individual checks (original behavior)
		for _, task := range tasks {
			if task.Status != emergent.StatusPending {
				continue
			}
			blocked, err := t.isBlocked(ctx, task.ID, taskStatus)
			if err != nil {
				return nil, fmt.Errorf("checking if task %s is blocked: %w", task.Number, err)
			}
			if blocked {
				blockedByMap[task.ID] = []string{"unknown"}
			}
			assigned, err := t.isAssigned(ctx, task.ID)
			if err != nil {
				return nil, fmt.Errorf("checking assignment for %s: %w", task.Number, err)
			}
			if assigned {
				assignedSet[task.ID] = true
			}
		}
	}

	available := make([]map[string]any, 0)
	for _, task := range tasks {
		if task.Status != emergent.StatusPending {
			continue
		}

		// Check if blocked by any incomplete task
		if blockers, ok := blockedByMap[task.ID]; ok {
			isBlocked := false
			for _, blockerID := range blockers {
				status, ok := taskStatus[blockerID]
				if !ok || status != emergent.StatusCompleted {
					isBlocked = true
					break
				}
			}
			if isBlocked {
				continue
			}
		}

		// Check if already assigned
		if assignedSet[task.ID] {
			continue
		}

		available = append(available, map[string]any{
			"id":                task.ID,
			"number":            task.Number,
			"description":       task.Description,
			"task_type":         task.TaskType,
			"complexity_points": task.ComplexityPoints,
			"tags":              task.Tags,
		})
	}

	// Sort by task number
	sort.Slice(available, func(i, j int) bool {
		return available[i]["number"].(string) < available[j]["number"].(string)
	})

	return mcp.JSONResult(map[string]any{
		"available_tasks": available,
		"count":           len(available),
	})
}

// isBlocked checks if a task is blocked by any incomplete task via blocked_by relationships.
func (t *GetAvailableTasks) isBlocked(ctx context.Context, taskID string, taskStatus map[string]string) (bool, error) {
	// Look for incoming "blocks" relationships (i.e., things that block this task)
	rels, err := t.client.ListRelationships(ctx, &graph.ListRelationshipsOptions{
		Type:  emergent.RelBlocks,
		DstID: taskID,
		Limit: 50,
	})
	if err != nil {
		return false, err
	}

	for _, rel := range rels {
		status, ok := taskStatus[rel.SrcID]
		if !ok {
			// Unknown task — conservatively treat as blocking
			return true, nil
		}
		if status != emergent.StatusCompleted {
			return true, nil
		}
	}
	return false, nil
}

// isAssigned checks if a task has an assigned_to relationship.
func (t *GetAvailableTasks) isAssigned(ctx context.Context, taskID string) (bool, error) {
	rels, err := t.client.ListRelationships(ctx, &graph.ListRelationshipsOptions{
		Type:  emergent.RelAssignedTo,
		SrcID: taskID,
		Limit: 1,
	})
	if err != nil {
		return false, err
	}
	return len(rels) > 0, nil
}

// --- spec_assign_task ---

type assignTaskParams struct {
	TaskID  string `json:"task_id"`
	AgentID string `json:"agent_id"`
}

type AssignTask struct {
	client *emergent.Client
}

func NewAssignTask(client *emergent.Client) *AssignTask {
	return &AssignTask{client: client}
}

func (t *AssignTask) Name() string { return "spec_assign_task" }
func (t *AssignTask) Description() string {
	return "Assign a task to a CodingAgent. Updates task status to in_progress and creates assigned_to relationship."
}
func (t *AssignTask) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "task_id": {"type": "string", "description": "ID of the task to assign"},
    "agent_id": {"type": "string", "description": "ID of the CodingAgent to assign to"}
  },
  "required": ["task_id", "agent_id"]
}`)
}

func (t *AssignTask) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p assignTaskParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}
	if p.TaskID == "" || p.AgentID == "" {
		return mcp.ErrorResult("task_id and agent_id are required"), nil
	}

	// Get task to verify it exists and is assignable
	task, err := t.client.GetTask(ctx, p.TaskID)
	if err != nil {
		return mcp.ErrorResult(fmt.Sprintf("task not found: %v", err)), nil
	}
	if task.Status != emergent.StatusPending {
		return mcp.ErrorResult(fmt.Sprintf("task %s is %s, can only assign pending tasks", task.Number, task.Status)), nil
	}

	// Verify agent exists
	_, err = t.client.GetObject(ctx, p.AgentID)
	if err != nil {
		return mcp.ErrorResult(fmt.Sprintf("agent not found: %v", err)), nil
	}

	// Create assigned_to relationship
	if _, err := t.client.CreateRelationship(ctx, emergent.RelAssignedTo, p.TaskID, p.AgentID, nil); err != nil {
		return nil, fmt.Errorf("creating assignment: %w", err)
	}

	// Update task status to in_progress with started_at
	now := time.Now()
	task, err = t.client.UpdateTaskStatus(ctx, p.TaskID, emergent.StatusInProgress, map[string]any{
		"started_at": now.Format(time.RFC3339),
	})
	if err != nil {
		return nil, fmt.Errorf("updating task status: %w", err)
	}

	return mcp.JSONResult(map[string]any{
		"task_id":    p.TaskID,
		"agent_id":   p.AgentID,
		"number":     task.Number,
		"status":     task.Status,
		"started_at": now.Format(time.RFC3339),
		"message":    fmt.Sprintf("Assigned task %s to agent", task.Number),
	})
}

// --- spec_complete_task ---

type completeTaskParams struct {
	TaskID            string   `json:"task_id"`
	Artifacts         []string `json:"artifacts,omitempty"`
	VerificationNotes string   `json:"verification_notes,omitempty"`
}

type CompleteTask struct {
	client *emergent.Client
}

func NewCompleteTask(client *emergent.Client) *CompleteTask {
	return &CompleteTask{client: client}
}

func (t *CompleteTask) Name() string { return "spec_complete_task" }
func (t *CompleteTask) Description() string {
	return "Mark a task as completed. Records artifacts and verification notes. Checks if any blocked tasks become available."
}
func (t *CompleteTask) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "task_id": {"type": "string", "description": "ID of the task to complete"},
    "artifacts": {
      "type": "array",
      "items": {"type": "string"},
      "description": "List of file paths or artifact references produced"
    },
    "verification_notes": {
      "type": "string",
      "description": "Notes on how the task was verified"
    }
  },
  "required": ["task_id"]
}`)
}

func (t *CompleteTask) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p completeTaskParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}
	if p.TaskID == "" {
		return mcp.ErrorResult("task_id is required"), nil
	}

	// Get task
	task, err := t.client.GetTask(ctx, p.TaskID)
	if err != nil {
		return mcp.ErrorResult(fmt.Sprintf("task not found: %v", err)), nil
	}
	if task.Status == emergent.StatusCompleted {
		return mcp.ErrorResult(fmt.Sprintf("task %s is already completed", task.Number)), nil
	}

	// Update to completed
	now := time.Now()
	props := map[string]any{
		"completed_at": now.Format(time.RFC3339),
	}
	if p.Artifacts != nil {
		props["artifacts"] = p.Artifacts
	}
	if p.VerificationNotes != "" {
		props["verification_notes"] = p.VerificationNotes
	}

	// Calculate actual hours if started_at exists
	if task.StartedAt != nil {
		hours := now.Sub(*task.StartedAt).Hours()
		props["actual_hours"] = hours
	}

	task, err = t.client.UpdateTaskStatus(ctx, p.TaskID, emergent.StatusCompleted, props)
	if err != nil {
		return nil, fmt.Errorf("completing task: %w", err)
	}

	// Find tasks that this task was blocking and check if they're now unblocked
	// Use ExpandGraph to batch-fetch blocked tasks and all their blockers in one call
	unblocked := make([]map[string]any, 0)
	expandResp, err := t.client.ExpandGraph(ctx, &graph.GraphExpandRequest{
		RootIDs:           []string{p.TaskID},
		Direction:         "both",
		MaxDepth:          2,
		MaxNodes:          200,
		MaxEdges:          500,
		RelationshipTypes: []string{emergent.RelBlocks},
	})
	if err == nil {
		// Normalize edge IDs to match node primary IDs. Fixes canonical vs version ID mismatch.
		nodeIdx := emergent.NewNodeIndex(expandResp.Nodes)
		emergent.CanonicalizeEdgeIDs(expandResp.Edges, nodeIdx)

		// Build a map of all nodes from the expand response (using normalized IDs)
		nodeMap := make(map[string]*graph.ExpandNode)
		for _, node := range expandResp.Nodes {
			nodeMap[node.ID] = node
		}

		// Build an ID set for the completed task so edge matching works
		// regardless of which ID variant the edge stores.
		taskIDs := emergent.NewIDSet(p.TaskID, "")
		// Also add canonical ID if available from the expand response
		if node, ok := nodeIdx[p.TaskID]; ok {
			taskIDs = emergent.NewIDSet(node.ID, node.CanonicalID)
		}

		// Find tasks this task directly blocks (outgoing blocks edges from this task)
		blockedTaskIDs := make(map[string]bool)
		for _, edge := range expandResp.Edges {
			if taskIDs[edge.SrcID] && edge.Type == emergent.RelBlocks {
				blockedTaskIDs[edge.DstID] = true
			}
		}

		// For each blocked task, check if all OTHER blockers are completed
		for blockedID := range blockedTaskIDs {
			stillBlocked := false
			for _, edge := range expandResp.Edges {
				if edge.DstID == blockedID && edge.Type == emergent.RelBlocks && !taskIDs[edge.SrcID] {
					// Another blocker — check its status from the expand nodes
					if node, ok := nodeMap[edge.SrcID]; ok {
						status, _ := node.Properties["status"].(string)
						if status != emergent.StatusCompleted {
							stillBlocked = true
							break
						}
					} else {
						// Unknown node — conservatively treat as blocked
						stillBlocked = true
						break
					}
				}
			}

			if !stillBlocked {
				if node, ok := nodeMap[blockedID]; ok {
					entry := map[string]any{
						"id": blockedID,
					}
					if node.Properties != nil {
						if num, ok := node.Properties["number"].(string); ok {
							entry["number"] = num
						}
						if desc, ok := node.Properties["description"].(string); ok {
							entry["description"] = desc
						}
					}
					unblocked = append(unblocked, entry)
				}
			}
		}
	}

	return mcp.JSONResult(map[string]any{
		"task_id":      p.TaskID,
		"number":       task.Number,
		"status":       emergent.StatusCompleted,
		"completed_at": now.Format(time.RFC3339),
		"unblocked":    unblocked,
		"message":      fmt.Sprintf("Completed task %s. %d task(s) unblocked.", task.Number, len(unblocked)),
	})
}

// --- spec_get_critical_path ---

type getCriticalPathParams struct {
	ChangeID string `json:"change_id"`
}

type GetCriticalPath struct {
	client *emergent.Client
}

func NewGetCriticalPath(client *emergent.Client) *GetCriticalPath {
	return &GetCriticalPath{client: client}
}

func (t *GetCriticalPath) Name() string { return "spec_get_critical_path" }
func (t *GetCriticalPath) Description() string {
	return "Calculate the critical path through a change's tasks: the longest dependency chain that determines minimum completion time."
}
func (t *GetCriticalPath) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "change_id": {
      "type": "string",
      "description": "ID of the change to analyze"
    }
  },
  "required": ["change_id"]
}`)
}

func (t *GetCriticalPath) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p getCriticalPathParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}
	if p.ChangeID == "" {
		return mcp.ErrorResult("change_id is required"), nil
	}

	// Get all tasks for the change
	tasks, err := t.client.ListTasks(ctx, p.ChangeID)
	if err != nil {
		return nil, fmt.Errorf("listing tasks: %w", err)
	}
	if len(tasks) == 0 {
		return mcp.JSONResult(map[string]any{
			"critical_path":     []any{},
			"total_complexity":  0,
			"completed_points":  0,
			"remaining_points":  0,
			"parallel_capacity": 0,
			"message":           "No tasks found",
		})
	}

	// Build adjacency graph: task blocks → task
	taskByID := make(map[string]*emergent.Task)
	for _, task := range tasks {
		taskByID[task.ID] = task
	}

	// Get all blocks relationships among these tasks using a single ExpandGraph call
	blocksGraph := make(map[string][]string) // blocker → []blocked
	blockedBy := make(map[string][]string)   // blocked → []blockers
	expandResp, err := t.client.ExpandGraph(ctx, &graph.GraphExpandRequest{
		RootIDs:           []string{p.ChangeID},
		Direction:         "outgoing",
		MaxDepth:          2,
		MaxNodes:          500,
		MaxEdges:          1000,
		RelationshipTypes: []string{emergent.RelHasTask, emergent.RelBlocks},
	})
	if err == nil {
		// Normalize edge IDs to match node/task primary IDs. Fixes canonical vs version ID mismatch.
		nodeIdx := emergent.NewNodeIndex(expandResp.Nodes)
		emergent.CanonicalizeEdgeIDs(expandResp.Edges, nodeIdx)

		for _, edge := range expandResp.Edges {
			if edge.Type == emergent.RelBlocks {
				if _, ok := taskByID[edge.SrcID]; ok {
					if _, ok := taskByID[edge.DstID]; ok {
						blocksGraph[edge.SrcID] = append(blocksGraph[edge.SrcID], edge.DstID)
						blockedBy[edge.DstID] = append(blockedBy[edge.DstID], edge.SrcID)
					}
				}
			}
		}
	} else {
		// Fallback: individual relationship lookups
		for _, task := range tasks {
			rels, err := t.client.ListRelationships(ctx, &graph.ListRelationshipsOptions{
				Type:  emergent.RelBlocks,
				SrcID: task.ID,
				Limit: 50,
			})
			if err != nil {
				continue
			}
			for _, rel := range rels {
				if _, ok := taskByID[rel.DstID]; ok {
					blocksGraph[task.ID] = append(blocksGraph[task.ID], rel.DstID)
					blockedBy[rel.DstID] = append(blockedBy[rel.DstID], task.ID)
				}
			}
		}
	}

	// Find critical path using longest path in DAG
	// Use memoized DFS to find longest path from each node
	memo := make(map[string]int) // taskID → longest path complexity from this node
	var longestFrom func(id string) int
	longestFrom = func(id string) int {
		if v, ok := memo[id]; ok {
			return v
		}
		task := taskByID[id]
		best := task.ComplexityPoints
		for _, dep := range blocksGraph[id] {
			candidate := task.ComplexityPoints + longestFrom(dep)
			if candidate > best {
				best = candidate
			}
		}
		memo[id] = best
		return best
	}

	// Find the task that starts the longest path
	var maxLen int
	var startID string
	for _, task := range tasks {
		pathLen := longestFrom(task.ID)
		if pathLen > maxLen {
			maxLen = pathLen
			startID = task.ID
		}
	}

	// Reconstruct the critical path
	criticalPath := make([]map[string]any, 0)
	current := startID
	for current != "" {
		task := taskByID[current]
		criticalPath = append(criticalPath, map[string]any{
			"id":                task.ID,
			"number":            task.Number,
			"description":       task.Description,
			"complexity_points": task.ComplexityPoints,
			"status":            task.Status,
		})

		// Follow the edge to the longest next node
		var nextID string
		var nextLen int
		for _, dep := range blocksGraph[current] {
			depLen := longestFrom(dep)
			if depLen > nextLen {
				nextLen = depLen
				nextID = dep
			}
		}
		current = nextID
	}

	// Calculate progress stats
	var totalPoints, completedPoints int
	var pendingCount, inProgressCount, completedCount, blockedCount int
	for _, task := range tasks {
		totalPoints += task.ComplexityPoints
		switch task.Status {
		case emergent.StatusCompleted:
			completedPoints += task.ComplexityPoints
			completedCount++
		case emergent.StatusInProgress:
			inProgressCount++
		case emergent.StatusBlocked:
			blockedCount++
		default:
			pendingCount++
		}
	}

	// Calculate parallel capacity: tasks with no incomplete blockers and not yet started
	parallelCapacity := 0
	for _, task := range tasks {
		if task.Status != emergent.StatusPending {
			continue
		}
		blockers := blockedBy[task.ID]
		allComplete := true
		for _, bID := range blockers {
			if b, ok := taskByID[bID]; ok && b.Status != emergent.StatusCompleted {
				allComplete = false
				break
			}
		}
		if allComplete {
			parallelCapacity++
		}
	}

	return mcp.JSONResult(map[string]any{
		"critical_path":      criticalPath,
		"critical_path_cost": maxLen,
		"progress": map[string]any{
			"total_tasks":      len(tasks),
			"total_points":     totalPoints,
			"completed_points": completedPoints,
			"remaining_points": totalPoints - completedPoints,
			"percent_complete": safePercent(completedPoints, totalPoints),
			"by_status": map[string]int{
				"pending":     pendingCount,
				"in_progress": inProgressCount,
				"completed":   completedCount,
				"blocked":     blockedCount,
			},
		},
		"parallel_capacity": parallelCapacity,
	})
}

func safePercent(num, denom int) float64 {
	if denom == 0 {
		return 0
	}
	return float64(num) / float64(denom) * 100
}
