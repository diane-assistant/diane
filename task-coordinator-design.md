# Task Coordinator Design: Who Orchestrates the Agents?

*Design Date: February 14, 2026*

## The Question

Given a SpecMCP change with a task DAG (tasks with explicit `blocks` dependencies), **who coordinates which tasks get dispatched to which agents, when, and in parallel?** And what does it mean to store sessions in Emergent?

---

## What Already Exists

SpecMCP already has the primitives:

| Tool | What it does |
|------|-------------|
| `spec_generate_tasks` | Creates Task entities with `blocks` relationships in Emergent |
| `spec_get_available_tasks` | Returns tasks that are: pending + unblocked + unassigned |
| `spec_assign_task` | Creates `assigned_to` relationship, sets status to `in_progress` |
| `spec_complete_task` | Marks done, records artifacts, **returns newly-unblocked tasks** |
| `spec_get_critical_path` | Longest dependency chain + `parallel_capacity` count |

The `CodingAgent` entity already exists with `name`, `type`, `skills`, `specialization`, `velocity_points_per_hour`. The `assigned_to` relationship connects Task to CodingAgent.

The ACP client already has `RunSync`, `RunAsync`, `WaitForCompletion`, `CreateRun` with `SessionID` support.

**The gap**: Nothing connects these. No component calls `spec_get_available_tasks`, picks agents, dispatches via ACP, monitors completion, and calls `spec_complete_task` to unlock the next wave.

---

## Approach A: Code Coordinator (Go Loop)

A `TaskDispatcher` goroutine that mechanically drives the task DAG.

### How It Works

```go
type TaskDispatcher struct {
    specClient    *emergent.Client   // SpecMCP operations
    acpClient     *acp.Client        // Agent dispatch
    changeID      string
    maxParallel   int                // Max concurrent agents
    agentSelector AgentSelector      // Strategy for picking agents
}

func (d *TaskDispatcher) Run(ctx context.Context) error {
    for {
        // 1. Get available tasks (pending + unblocked + unassigned)
        available := d.specClient.GetAvailableTasks(ctx, d.changeID)
        if len(available) == 0 {
            if d.allTasksCompleted(ctx) {
                return nil // Done
            }
            // Tasks exist but all blocked or in-progress — wait
            time.Sleep(5 * time.Second)
            continue
        }

        // 2. How many slots do we have?
        inProgress := d.countInProgress(ctx)
        slots := d.maxParallel - inProgress
        if slots <= 0 {
            time.Sleep(5 * time.Second)
            continue
        }

        // 3. Pick tasks up to available slots
        batch := available[:min(slots, len(available))]

        // 4. Dispatch each in parallel
        for _, task := range batch {
            agent := d.agentSelector.Pick(task)
            d.specClient.AssignTask(ctx, task.ID, agent.ID)

            go d.executeTask(ctx, task, agent)
        }
    }
}

func (d *TaskDispatcher) executeTask(ctx context.Context, task Task, agent CodingAgent) {
    // Build prompt with full context from Emergent
    prompt := d.buildPrompt(ctx, task)

    // Create ACP session for this task
    session := d.createSession(ctx, task, agent)

    // Dispatch via ACP
    run, _ := d.acpClient.CreateRun(acp.RunCreateRequest{
        AgentName: agent.Name,
        SessionID: session.ID,
        Input:     []acp.Message{acp.NewUserMessage(prompt)},
        Mode:      acp.RunModeAsync,
    })

    // Store run reference in session
    d.updateSession(ctx, session.ID, map[string]any{
        "acp_run_id": run.RunID,
        "status":     "running",
    })

    // Poll for completion
    result, _ := d.acpClient.WaitForCompletion(run.RunID, 5*time.Second, 30*time.Minute)

    // Record completion
    d.specClient.CompleteTask(ctx, task.ID, result.Artifacts, result.VerificationNotes)

    // Update session
    d.updateSession(ctx, session.ID, map[string]any{
        "status":       "completed",
        "output":       result.GetTextOutput(),
        "completed_at": time.Now(),
    })

    // spec_complete_task already returns newly-unblocked tasks
    // The main loop will pick them up on next iteration
}
```

### What the Code Coordinator Decides

| Decision | How |
|----------|-----|
| **Which tasks to run** | `spec_get_available_tasks` — pure graph query, no judgment |
| **How many in parallel** | `maxParallel` config (e.g., 3) |
| **Which agent per task** | `AgentSelector` strategy — could be simple (round-robin, by specialization tag match) or configurable |
| **What context to include** | `buildPrompt` — deterministic template that pulls design, specs, related artifacts from Emergent |
| **When to retry** | On `RunStatusFailed` — re-queue task as pending, increment retry count |
| **When to stop** | All tasks completed or max retries exceeded |

### What the Code Coordinator Does NOT Decide

- Whether the task breakdown is correct
- Whether the agent's output is good enough
- Whether to modify the plan mid-execution
- Whether two tasks actually conflict even though they don't have a `blocks` relationship

### Constraints

```
Constraint 1: DETERMINISTIC DISPATCH
  Available tasks are dispatched in order. No judgment about
  "this task would benefit from running after that other one
  even though there's no formal dependency."

Constraint 2: STATIC AGENT SELECTION
  Agent selection is based on metadata (tags, specialization)
  not on understanding the task content. "This SwiftUI task
  should go to the agent that just finished the other SwiftUI
  task and has context" — the code coordinator can't reason
  about this unless you explicitly encode it.

Constraint 3: NO MID-FLIGHT ADAPTATION
  If task 3.1 reveals that the design was wrong and task 3.2
  should be modified, the code coordinator has no mechanism
  to pause 3.2 and revise the plan. It just executes the DAG
  as given.

Constraint 4: FIXED CONTEXT WINDOW
  The prompt template is static. It pulls the same categories
  of context for every task. Can't reason about "this particular
  task needs the test output from task 2.3 but not the design
  decisions from 1.1."
```

### Cost

Zero LLM tokens for coordination. All tokens go to the actual work.

---

## Approach B: LLM Coordinator

An LLM agent that has SpecMCP tools + ACP dispatch tools. It reads the task graph, reasons about what to do, and dispatches subagents.

### How It Works

The coordinator is itself an ACP agent (or a direct LLM session) with a system prompt like:

```
You are a task coordinator for the SpecMCP system. You have access to:

- spec_get_available_tasks(change_id) — get tasks ready to work on
- spec_get_critical_path(change_id) — understand the dependency structure
- spec_assign_task(task_id, agent_id) — assign a task to an agent
- spec_complete_task(task_id, artifacts, notes) — mark task done
- dispatch_agent(agent_name, task_id, prompt) — spawn an ACP agent
- get_agent_status(run_id) — check if an agent finished
- get_session(session_id) — read a previous session's output

Your job:
1. Check what tasks are available
2. Decide which agents should handle which tasks
3. Dispatch them (in parallel when appropriate)
4. Monitor completion
5. When tasks complete, check what's newly available
6. Repeat until all tasks are done

You may also:
- Modify the plan if you discover issues
- Skip tasks that become unnecessary
- Re-assign tasks if an agent fails
- Inject context from completed tasks into new ones
```

### What the LLM Coordinator Decides

Everything the code coordinator does, PLUS:

| Decision | How |
|----------|-----|
| **Context selection** | Reads completed task outputs and selects relevant context per task |
| **Agent affinity** | "Agent X just finished the data model — give it the related API task too" |
| **Plan adaptation** | "Task 3.1 output shows the design assumed wrong DB schema. I need to update task 3.2's prompt to account for the actual schema." |
| **Conflict detection** | "Tasks 2.1 and 2.3 both touch the same file. I'll run 2.1 first even though they're technically independent." |
| **Quality gating** | Reads agent output before marking complete — "This implementation is missing error handling, I'll re-dispatch to the same agent with feedback." |

### Constraints

```
Constraint 1: TOKEN COST
  Every coordination decision costs tokens. For a 20-task
  change, the coordinator might make 40+ LLM calls just for
  coordination (not counting the actual work). With a smart
  model this could be $5-15 in coordination overhead alone.

Constraint 2: LATENCY
  Each coordination decision takes 2-10 seconds for the LLM
  to reason. For parallel dispatch of 4 tasks, that's 4
  sequential decisions before any work starts.

Constraint 3: UNPREDICTABILITY
  The LLM might make different decisions on the same input.
  "Why did it assign the testing task to the implementation
  agent?" Hard to debug, hard to reproduce.

Constraint 4: CONTEXT WINDOW
  As tasks complete and the coordinator accumulates context,
  it may hit token limits. Needs compaction strategy.

Constraint 5: FAILURE MODES
  If the coordinator LLM hallucinates a task ID or makes a
  bad tool call, the whole pipeline stalls. The code coordinator
  can't hallucinate.
```

### Cost

Significant. Roughly 10-30% additional token cost on top of the actual work, depending on task count and how much reasoning is needed per dispatch.

---

## Approach C: The Hybrid

Code handles the mechanical loop. LLM handles the judgment calls.

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    TASK DISPATCHER (Go)                      │
│                                                             │
│  Loop:                                                      │
│    1. spec_get_available_tasks()                             │
│    2. For each available task:                               │
│       a. LLM call: "Given this task + context,              │
│          which agent? what context to include?"              │
│       b. spec_assign_task()                                  │
│       c. dispatch via ACP (async)                            │
│    3. Poll running tasks for completion                      │
│    4. On completion:                                         │
│       a. LLM call: "Is this output acceptable?              │
│          Mark complete or retry with feedback?"              │
│       b. spec_complete_task() or re-queue                    │
│    5. Repeat                                                │
│                                                             │
│  The loop itself is Go code — deterministic, testable.       │
│  The DECISIONS within the loop are LLM calls — flexible.     │
└─────────────────────────────────────────────────────────────┘
```

### What's Code vs What's LLM

| Aspect | Code (Go) | LLM |
|--------|-----------|-----|
| Available task query | x | |
| Parallel slot management | x | |
| ACP dispatch mechanics | x | |
| Poll/wait for completion | x | |
| Session lifecycle | x | |
| Emergent entity updates | x | |
| Which agent for this task | | x |
| What context to include in prompt | | x |
| Is this output good enough | | x |
| Should we retry or escalate | | x |
| Should we modify the plan | | x |

### LLM Calls Per Task

| When | LLM Call | Cost |
|------|----------|------|
| Before dispatch | "Pick agent + build prompt" | ~1K tokens in, ~500 tokens out |
| After completion | "Evaluate output quality" | ~2K tokens in, ~200 tokens out |
| On failure (rare) | "Diagnose + decide retry strategy" | ~1K tokens in, ~500 tokens out |

For a 15-task change: ~15 dispatch calls + ~15 evaluation calls = ~30 lightweight LLM calls. At ~$0.01-0.05 each, total coordination cost is $0.30-1.50. Compared to the actual agent work (likely $5-50), this is negligible.

---

## Sessions in Emergent

### Why

Right now sessions are ephemeral — they exist only while an ACP run is active. Once the agent finishes, the session is gone. This means:

- No record of what an agent tried, what it saw, what it produced
- No ability to resume a failed task from where it left off
- No cross-task learning ("last time we assigned a SwiftUI task to agent X, it took 45 minutes and failed twice")
- No audit trail for debugging ("why did the tests break after task 3.2?")

### Entity Model

```
New types to add to types.go:

TypeSession     = "Session"
TypeSessionMsg  = "SessionMessage"

New relationships:

RelHasSession      = "has_session"       // Task → Session
RelConductedBy     = "conducted_by"      // Session → CodingAgent
RelHasMessage      = "has_message"       // Session → SessionMessage
RelFollowsUp       = "follows_up"        // Session → Session (retry chain)
RelInformedBy      = "informed_by"       // Session → Session (context dependency)
```

```go
// Session represents an agent work session for a task.
type Session struct {
    ID           string     `json:"id,omitempty"`
    Status       string     `json:"status"`         // created, running, completed, failed, cancelled
    ACPRunID     string     `json:"acp_run_id"`     // Reference to ACP run
    ACPAgentName string     `json:"acp_agent_name"` // Which ACP agent
    Prompt       string     `json:"prompt"`         // The prompt sent
    Output       string     `json:"output"`         // Final output text
    TokensIn     int        `json:"tokens_in,omitempty"`
    TokensOut    int        `json:"tokens_out,omitempty"`
    CostUSD      float64    `json:"cost_usd,omitempty"`
    StartedAt    *time.Time `json:"started_at,omitempty"`
    CompletedAt  *time.Time `json:"completed_at,omitempty"`
    DurationSec  float64    `json:"duration_sec,omitempty"`
    RetryOf      string     `json:"retry_of,omitempty"`     // Previous session ID if retry
    RetryReason  string     `json:"retry_reason,omitempty"` // Why we retried
    Tags         []string   `json:"tags,omitempty"`
}

// SessionMessage represents a single message in a session.
// For long sessions, stores the conversation turn-by-turn.
type SessionMessage struct {
    ID        string     `json:"id,omitempty"`
    Role      string     `json:"role"`      // user, assistant, tool
    Content   string     `json:"content"`
    Sequence  int        `json:"sequence"`
    Timestamp *time.Time `json:"timestamp,omitempty"`
    Tags      []string   `json:"tags,omitempty"`
}
```

### Graph Shape

```
Change("add-dark-mode")
  │
  ├── has_task ──► Task("1.1 Create AppearanceMode enum")
  │                  │
  │                  ├── assigned_to ──► CodingAgent("opencode")
  │                  │
  │                  ├── has_session ──► Session(status: "failed", retry_reason: "build error")
  │                  │                    │
  │                  │                    ├── conducted_by ──► CodingAgent("opencode")
  │                  │                    └── has_message ──► [SessionMessage, SessionMessage, ...]
  │                  │
  │                  └── has_session ──► Session(status: "completed")  ◄── follows_up ── (previous session)
  │                                       │
  │                                       ├── conducted_by ──► CodingAgent("opencode")
  │                                       └── has_message ──► [SessionMessage, SessionMessage, ...]
  │
  ├── has_task ──► Task("1.2 Add toggle to SettingsView")
  │                  │
  │                  └── has_session ──► Session(status: "completed")
  │                                       │
  │                                       ├── conducted_by ──► CodingAgent("claude-code")
  │                                       └── informed_by ──► Session (from task 1.1)
  │
  ...
```

### What This Enables

**1. Resumable sessions**
If task 1.1 fails and we retry, the new session has `follows_up` pointing to the failed one. The coordinator can include the failure output in the retry prompt: "Previous attempt failed with: [error]. Fix this specific issue."

**2. Context threading**
When task 1.2 depends on 1.1, the coordinator can follow `blocks` → find 1.1's completed session → read its output → inject relevant parts into 1.2's prompt. The `informed_by` relationship explicitly tracks which sessions informed which.

**3. Agent performance tracking**
Query: "For CodingAgent X, what's the average session duration, retry rate, and cost?" — all answerable from the graph.

```
Graph query: CodingAgent("opencode") ←conducted_by── Session[*]
  → avg(duration_sec), count(where status="failed") / count(*), sum(cost_usd)
```

**4. Audit trail**
"Why did task 3.2 produce a broken implementation?" → Follow has_session → read SessionMessages → see exactly what prompt was sent and what the agent did.

**5. Cross-change learning**
"Last time we had a SwiftUI task, which agent was fastest?" — query across all changes.

---

## How the Full System Fits Together

```
USER REQUEST
    │
    ▼
┌──────────────────────────────────────────────────────────┐
│  SPEC WORKFLOW (existing)                                 │
│                                                          │
│  spec_new → spec_artifact (proposal, specs, design)       │
│         → spec_generate_tasks (creates DAG in Emergent)   │
│                                                          │
│  At this point the task DAG exists with blocks            │
│  relationships, complexity estimates, and tags.           │
└──────────────────────────────┬───────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────┐
│  TASK DISPATCHER (new component)                          │
│                                                          │
│  Go loop:                                                │
│    ┌──────────────────────────────────────────────────┐   │
│    │  1. spec_get_available_tasks(change_id)          │   │
│    │     → [Task 1.1, Task 1.2, Task 2.1]            │   │
│    │                                                  │   │
│    │  2. For each (up to maxParallel):                │   │
│    │     a. LLM: pick agent + build prompt            │   │
│    │     b. Create Session in Emergent                │   │
│    │     c. spec_assign_task(task, agent)              │   │
│    │     d. acp.CreateRun(agent, prompt, async)        │   │
│    │                                                  │   │
│    │  3. Poll running sessions:                       │   │
│    │     for each running session:                    │   │
│    │       status = acp.GetRun(run_id)                │   │
│    │       if completed:                              │   │
│    │         LLM: evaluate output quality             │   │
│    │         if good:                                 │   │
│    │           spec_complete_task(task_id)             │   │
│    │           → returns newly_unblocked tasks        │   │
│    │         else:                                    │   │
│    │           create retry Session (follows_up)      │   │
│    │           re-dispatch                            │   │
│    │       if failed:                                 │   │
│    │         record failure in Session                │   │
│    │         retry or escalate                        │   │
│    │                                                  │   │
│    │  4. Sleep(poll_interval)                         │   │
│    │  5. Loop until all tasks completed or max retries│   │
│    └──────────────────────────────────────────────────┘   │
│                                                          │
└──────────────────────────────────────────────────────────┘
                               │
            ┌──────────────────┼──────────────────┐
            ▼                  ▼                  ▼
     ┌─────────────┐   ┌─────────────┐   ┌─────────────┐
     │  OpenCode   │   │ Claude Code │   │ Gemini CLI  │
     │  (ACP)      │   │   (ACP)     │   │   (ACP)     │
     │             │   │             │   │             │
     │ Task 1.1    │   │ Task 1.2    │   │ Task 2.1    │
     │ Session A   │   │ Session B   │   │ Session C   │
     └──────┬──────┘   └──────┬──────┘   └──────┬──────┘
            │                 │                  │
            └─────────────────┼──────────────────┘
                              │
                              ▼
                    ┌──────────────────┐
                    │    EMERGENT      │
                    │                  │
                    │  Change          │
                    │  ├── Tasks       │
                    │  ├── Sessions    │
                    │  ├── Agents      │
                    │  └── Messages    │
                    └──────────────────┘
```

### Concrete Example: Task 1.1 Completes, Unlocking 2.1 and 2.3

```
TIME 0:00 — Initial state
  Available: [1.1, 1.2, 1.3]    (no blockers)
  Blocked:   [2.1, 2.3, 3.1]    (blocked by 1.x tasks)

  Dispatcher creates 3 sessions, assigns to 3 agents, dispatches

TIME 0:45 — Task 1.2 completes first
  spec_complete_task("1.2")
    → unblocked: [2.1]           (2.1 was only blocked by 1.2)
  
  Dispatcher sees 2.1 is now available
  LLM call: "Task 2.1 needs the data model from 1.2's output"
    → reads Session B's output from Emergent
    → includes relevant parts in 2.1's prompt
  Creates Session D, dispatches to agent

TIME 1:30 — Tasks 1.1 and 1.3 complete
  spec_complete_task("1.1")
    → unblocked: [2.3]           (2.3 was blocked by 1.1 + 1.2, both now done)
  spec_complete_task("1.3")
    → unblocked: []              (nothing was only blocked by 1.3)

  Dispatcher sees 2.3 is now available
  Creates Session E, dispatches

TIME 3:00 — Task 2.1 fails
  Agent output: "Build error: missing import"
  LLM evaluation: "Fixable error, retry with context"
  
  Creates Session F (follows_up → Session D)
  Prompt includes: "Previous attempt failed with: 'missing import for AppearanceMode'.
                    The enum was defined in task 1.1. See: [output from Session A]"
  Re-dispatches

TIME 3:30 — Task 2.1 retry succeeds
  spec_complete_task("2.1")
    → unblocked: [3.1]

  And so on...
```

---

## Comparison Table

| Dimension | Code Coordinator | LLM Coordinator | Hybrid |
|-----------|-----------------|-----------------|--------|
| **Token cost for coordination** | $0 | $5-15 per change | $0.30-1.50 per change |
| **Latency per dispatch** | <10ms | 2-10s | 2-5s (one LLM call) |
| **Agent selection quality** | Tag matching (simple) | Contextual reasoning | Contextual reasoning |
| **Context building quality** | Template-based (rigid) | Selective (intelligent) | Selective (intelligent) |
| **Plan adaptation** | None | Full | On-failure only |
| **Output quality gating** | None (trust agent) | Full review | Quick evaluation |
| **Debuggability** | Excellent (deterministic) | Poor (non-deterministic) | Good (code loop, LLM decisions logged) |
| **Failure modes** | Predictable | Can hallucinate | LLM failures isolated to decisions |
| **Testability** | Unit testable | Hard to test | Code loop testable, LLM calls mockable |
| **Complexity** | Low (~200 LOC) | High (~500 LOC + prompts) | Medium (~350 LOC + 2 prompts) |

---

## Constraints Summary

Regardless of approach, these constraints apply:

### Hard Constraints (from SpecMCP design)

1. **Task DAG is authoritative** — `blocks` relationships determine execution order. The coordinator cannot violate these.
2. **One agent per task** — `assigned_to` is singular. No splitting a task across agents.
3. **Status state machine** — pending → in_progress → completed/failed. No skipping states.
4. **Task completion unlocks** — `spec_complete_task` is the only way to unblock downstream tasks.

### Soft Constraints (design decisions)

5. **Max parallelism** — How many agents run simultaneously? Resource and cost limit.
6. **Retry budget** — How many retries per task before escalating to user?
7. **Session depth** — How many messages in a session before compacting?
8. **Cross-task context** — How much output from task A goes into task B's prompt?

### Open Questions

9. **File conflict detection** — Two parallel tasks editing the same file? The DAG doesn't encode this. Options: (a) trust the task generator to add `blocks` for conflicting files, (b) coordinator detects at dispatch time by checking `design.file_changes`, (c) let git handle merge conflicts post-hoc.

10. **Agent pool management** — Are agents persistent (always running, take tasks from a queue) or ephemeral (spawned per task)? ACP supports both patterns.

11. **User visibility** — How does the user see progress? Options: (a) notifications per task completion, (b) live dashboard showing the DAG with color-coded status, (c) periodic summary.

12. **Scope** — Does the dispatcher handle one change at a time, or multiple changes in parallel? If multiple, what about cross-change conflicts?

---

## Recommendation

**Start with the Hybrid.** Here's why:

1. The Go loop gives you a solid, debuggable, testable foundation. You can run it without any LLM coordinator at all (just use tag-based agent selection) and it works.

2. Add LLM calls as upgrades, not dependencies:
   - **v1**: Code loop + tag-based agent selection + template prompts. Zero LLM coordination cost.
   - **v2**: Add LLM agent selection. "Given task X with tags [swift, ui] and agents [opencode, claude-code], which agent?" — one cheap call.
   - **v3**: Add LLM output evaluation. "Is this output complete?" — one cheap call after each task.
   - **v4**: Add LLM context threading. "What from session A is relevant for task B?" — one call per dependent task.
   - **v5**: Add LLM plan adaptation. "Task 3.1 revealed X, should we modify 3.2?" — only on failure/surprise.

3. Sessions in Emergent are the foundation that makes all of this work. Without persistent sessions, the LLM calls have no memory to work with. With them, even the pure code coordinator can do "retry with failure context from previous session."

The progression from v1 to v5 can happen incrementally. Each version works standalone. Each LLM call is optional and has a code fallback.
