## ADDED Requirements

### Requirement: Generate tasks from specs
The system SHALL provide a `spec_generate_tasks` tool that analyzes a Change's Specs, Requirements, and Scenarios to produce a hierarchical task tree. Tasks SHALL be typed (spec, context, component, action, pattern) and ordered bottom-up by dependency: components before contexts, contexts before actions, actions before scenarios.

#### Scenario: Generate task tree for a change
- **WHEN** `spec_generate_tasks` is called for a Change with 2 Specs containing 5 Requirements and 8 Scenarios
- **THEN** a task tree is produced with Tasks linked to the Change via `has_task`, each Task has an `implements` relationship to the entity it covers, and dependencies are expressed via `blocks` relationships

#### Scenario: Tasks include complexity estimates
- **WHEN** tasks are generated
- **THEN** each Task has a `complexity_points` value between 1-10 based on the estimated complexity of the implementation

### Requirement: Dependency detection
The system SHALL automatically detect dependencies between tasks based on the entity relationships in the graph. A Task implementing a UIComponent that is used by a Context SHALL block the Task implementing that Context.

#### Scenario: Component blocks context task
- **WHEN** Task A implements UIComponent "user-list" and Task B implements Context "user-management-screen" which uses "user-list"
- **THEN** Task A blocks Task B (a `blocks` relationship exists from A to B)

#### Scenario: No circular dependencies
- **WHEN** tasks are generated for any valid spec structure
- **THEN** the dependency graph is a DAG — no circular blocking chains exist

### Requirement: Get available tasks
The system SHALL provide a `spec_get_available_tasks` tool that returns all tasks matching the criteria: status is "pending", no unresolved `blocked_by` relationships, and not currently `assigned_to` any CodingAgent.

#### Scenario: Identify available tasks
- **WHEN** `spec_get_available_tasks` is called and there are 10 tasks total, 3 completed, 2 in-progress, 2 blocked, and 3 pending-unblocked-unassigned
- **THEN** the response lists exactly 3 available tasks with their complexity points and required skills

### Requirement: Get parallel capacity
The system SHALL provide a `spec_get_parallel_capacity` tool that returns the number of available tasks and the list of those tasks, representing how many agents could work simultaneously.

#### Scenario: Report parallel capacity
- **WHEN** `spec_get_parallel_capacity` is called
- **THEN** the response includes the count of available tasks and the task list, sorted by complexity descending

### Requirement: Assign task to agent
The system SHALL provide a `spec_assign_task` tool that creates an `assigned_to` relationship between a Task and a CodingAgent, updates the Task status to "in_progress", and records `started_at` timestamp. The tool SHALL validate that the agent's skills match the task's required skills before assignment.

#### Scenario: Assign available task
- **WHEN** `spec_assign_task` is called with task "T5" and agent "alice"
- **THEN** an `assigned_to` relationship is created, task status becomes "in_progress", and `started_at` is set to current time

#### Scenario: Reject assignment of blocked task
- **WHEN** `spec_assign_task` is called for a task that has unresolved `blocked_by` relationships
- **THEN** the tool returns an error listing which tasks are blocking it

### Requirement: Agent skill matching
The `spec_get_available_tasks` tool SHALL include suitable agents for each task by matching the task's required skills against CodingAgent skills. When multiple agents match, they SHALL be ranked by current workload (points in progress) ascending, preferring agents with less current work.

#### Scenario: List suitable agents for available tasks
- **WHEN** `spec_get_available_tasks` is called and Task T5 requires skills ["react", "typescript"]
- **THEN** the response includes a `suitable_agents` list for T5 containing all CodingAgents whose `skills` include both "react" and "typescript", sorted by current workload

#### Scenario: No suitable agents
- **WHEN** `spec_get_available_tasks` is called and Task T8 requires skills ["rust"] but no active CodingAgent has "rust" in their skills
- **THEN** Task T8's `suitable_agents` list is empty

### Requirement: Complete task and unlock dependents
The system SHALL provide a `spec_complete_task` tool that marks a Task as "completed", records `completed_at` timestamp, calculates `actual_hours`, and returns the list of tasks that are newly unblocked as a result.

#### Scenario: Complete task unlocks dependent
- **WHEN** `spec_complete_task` is called for Task A which blocks Task B, and Task B has no other blockers
- **THEN** Task A status becomes "completed", `completed_at` is recorded, and Task B appears in the "newly_available" list

#### Scenario: Complete task with artifacts
- **WHEN** `spec_complete_task` is called with artifacts `["src/components/UserList.tsx", "src/components/UserList.test.tsx"]`
- **THEN** the Task's `artifacts` property is updated with the provided file paths

### Requirement: Scenario progress calculation
The system SHALL provide a `spec_get_scenario_progress` tool that calculates completion percentage for a scenario by summing the complexity points of completed tasks vs. total task points.

#### Scenario: Calculate progress
- **WHEN** `spec_get_scenario_progress` is called for a scenario with 25 total points and 9 completed points
- **THEN** the response shows 9/25 points (36% complete) with estimated remaining hours based on agent velocity

### Requirement: Critical path analysis
The system SHALL provide a `spec_get_critical_path` tool that finds the longest dependency chain (by total complexity points) for a given scenario or change, identifying bottleneck tasks.

#### Scenario: Identify critical path
- **WHEN** `spec_get_critical_path` is called for a change with multiple parallel dependency chains
- **THEN** the response lists the longest chain of tasks (by summed complexity points) and the total points on the critical path

### Requirement: Velocity tracking
The system SHALL track CodingAgent velocity by recording completed task complexity points and actual hours. The velocity (points/hour) SHALL be stored on the CodingAgent entity and updated after each task completion.

#### Scenario: Update agent velocity
- **WHEN** agent "alice" completes a 4-point task in 3.2 hours, having previously completed a 6-point task in 4.8 hours
- **THEN** alice's `velocity_points_per_hour` is updated to the rolling average: (4+6)/(3.2+4.8) = 1.25 points/hour

#### Scenario: Predict task duration
- **WHEN** a task with 6 complexity points is being estimated for agent "alice" with velocity 1.25 points/hour
- **THEN** the predicted duration is 4.8 hours
