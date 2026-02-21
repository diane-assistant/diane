# Multi-Agent Orchestration: Real-Life Development Task Walkthrough

*Design Date: February 14, 2026*

## The Scenario

**User says to Diane**: "Add a dark mode toggle to the Diane companion macOS app settings page"

This triggers a development task involving **5 specialized agents** working together through orchestrated loops, sharing state via the Emergent knowledge graph, and "discussing" solutions before implementing them.

---

## The Agents Involved

```
┌──────────────────────────────────────────────────────────────────┐
│                    ORCHESTRATOR AGENT                            │
│  Role: Breaks down task, coordinates agents, manages lifecycle  │
│  Runs: Always (Diane core goroutine)                            │
│  LLM: Lightweight model for coordination decisions              │
└──────────────┬───────────────────────────────────────────────────┘
               │ spawns & coordinates
    ┌──────────┼──────────┬──────────────┬──────────────┐
    ▼          ▼          ▼              ▼              ▼
┌────────┐ ┌────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐
│RESEARCH│ │DESIGNER│ │DEVELOPER │ │  TESTER  │ │ REVIEWER │
│ AGENT  │ │ AGENT  │ │  AGENT   │ │  AGENT   │ │  AGENT   │
│        │ │        │ │          │ │          │ │          │
│Claude  │ │Claude  │ │OpenCode  │ │OpenCode  │ │Gemini    │
│Sonnet  │ │Sonnet  │ │(Claude)  │ │(Claude)  │ │CLI       │
└────────┘ └────────┘ └──────────┘ └──────────┘ └──────────┘
```

**Key insight**: Each agent is a real ACP agent (OpenCode, Claude Code, Gemini CLI) that Diane already knows how to spawn and communicate with via the ACP protocol. The orchestrator is a new component inside Diane that coordinates them.

---

## How It Actually Works

### The Core Loop Architecture

There are **three types of loops** operating at different levels:

```
LEVEL 1: ORCHESTRATOR LOOP (outer)
 │  Manages the overall task lifecycle
 │  Decides which phase to run next
 │  Checks if task is complete
 │
 ├─► LEVEL 2: PHASE LOOP (middle)
 │    Runs one phase (research, design, implement, test, review)
 │    Manages agents within that phase
 │    Decides if phase is complete or needs retry
 │
 └──► LEVEL 3: AGENT LOOP (inner)
       Individual agent working on a subtask
       Agent uses tools, writes code, produces artifacts
       Reports back when done or stuck
```

---

## Step-by-Step Walkthrough

### PHASE 0: Task Decomposition (Orchestrator, ~2 seconds)

The Orchestrator receives the user's request and breaks it down. It doesn't use a coding agent for this — it's a lightweight LLM call inside Diane itself.

```
ORCHESTRATOR INTERNAL STATE:
┌─────────────────────────────────────────────────────────┐
│ Task: "Add dark mode toggle to macOS app settings"      │
│                                                         │
│ Decomposition:                                          │
│   Phase 1: RESEARCH   → What exists? What patterns?     │
│   Phase 2: DESIGN     → UI/UX design, component plan    │
│   Phase 3: IMPLEMENT  → Write the code                  │
│   Phase 4: TEST       → Verify it works                 │
│   Phase 5: REVIEW     → Code quality check              │
│                                                         │
│ Dependencies:                                           │
│   RESEARCH ──► DESIGN ──► IMPLEMENT ──► TEST ──► REVIEW │
│                              ▲                          │
│                              │                          │
│                    (retry if TEST fails)                 │
└─────────────────────────────────────────────────────────┘
```

**Emergent State Created:**
```
Orchestrator creates entities in Emergent:

  DevTask("dark-mode-toggle")
    ├── Phase("research")     status: pending
    ├── Phase("design")       status: pending
    ├── Phase("implement")    status: pending
    ├── Phase("test")         status: pending
    └── Phase("review")       status: pending
```

---

### PHASE 1: Research (~30 seconds)

**Orchestrator dispatches Research Agent via ACP:**

```go
// Inside Diane's orchestrator — this is real Go code using existing ACP client
func (o *Orchestrator) runResearchPhase(task *DevTask) (*PhaseResult, error) {
    // 1. Build context from Emergent graph
    //    Query existing app structure, SwiftUI patterns used, etc.
    appContext := o.emergent.Query(`
        MATCH (c:Context {name: "diane-macos-app"})
        RETURN c.properties
    `)

    // 2. Dispatch to Research Agent via ACP
    prompt := fmt.Sprintf(`
        Research task: %s

        Current app context:
        %s

        Investigate:
        1. Current settings page structure in the Diane macOS app (Diane/ directory)
        2. SwiftUI dark mode patterns and best practices
        3. macOS system appearance API (@Environment(\.colorScheme))
        4. UserDefaults patterns for persisting appearance preference
        5. Any existing color/theme definitions in the codebase

        Return a structured research report with:
        - Current codebase state (relevant files, existing patterns)
        - Recommended approach (with rationale)
        - Key SwiftUI APIs to use
        - Files that need to be modified
        - Potential risks or complications
    `, task.Description, appContext)

    // ACP call — this uses Diane's existing acp.Client
    run, err := o.acpClient.RunSync("claude-code", acp.RunCreateRequest{
        AgentName: "claude-code",
        Input:     []acp.Message{acp.NewUserMessage(prompt)},
        Mode:      acp.RunModeSync,
    })

    // 3. Parse research results
    research := o.parseResearchOutput(run.GetTextOutput())

    // 4. Store in Emergent
    o.emergent.CreateEntity("ResearchReport", map[string]interface{}{
        "task_id":     task.ID,
        "findings":    research.Findings,
        "approach":    research.RecommendedApproach,
        "files":       research.RelevantFiles,
        "apis":        research.KeyAPIs,
        "risks":       research.Risks,
    })

    // 5. Update phase status
    o.emergent.UpdateEntity(task.Phases["research"].ID, map[string]interface{}{
        "status": "completed",
    })

    return &PhaseResult{
        Phase:     "research",
        Output:    research,
        Status:    "completed",
        Duration:  time.Since(start),
    }, nil
}
```

**Research Agent output** (stored in Emergent):
```json
{
  "findings": {
    "settings_file": "Diane/Views/SettingsView.swift",
    "existing_sections": ["General", "Agents", "MCP Servers"],
    "color_scheme": "Uses system default, no manual override",
    "app_storage_pattern": "@AppStorage used for preferences"
  },
  "recommended_approach": "Add AppearanceSection to SettingsView with @AppStorage toggle",
  "relevant_files": [
    "Diane/Views/SettingsView.swift",
    "Diane/DianApp.swift",
    "Diane/Assets.xcassets"
  ],
  "key_apis": [
    "@AppStorage(\"appearance\") — persist preference",
    "@Environment(\\.colorScheme) — read current scheme",
    ".preferredColorScheme(_:) — apply override",
    "NSApplication.shared.appearance — programmatic macOS override"
  ],
  "risks": [
    "Need to apply .preferredColorScheme at root view level",
    "Some system colors may not adapt without explicit dark variants"
  ]
}
```

---

### PHASE 2: Design (~45 seconds)

Now the Orchestrator starts the Design Agent, feeding it the research results.

**The "Discussion" Pattern**: Before coding, the Designer and Researcher have an asynchronous exchange:

```
DISCUSSION LOOP (2 rounds max):
┌─────────────────────────────────────────────────────────────┐
│                                                             │
│  Round 1: Designer proposes UI layout                       │
│    ┌──────────┐          ┌──────────┐                       │
│    │ Designer │──proposal──►│ Research │                     │
│    │  Agent   │◄──feedback──│  Agent   │                     │
│    └──────────┘          └──────────┘                       │
│    Designer: "3-option segmented control: Light/Dark/Auto"  │
│    Research:  "Agree, but use Picker with .segmented style  │
│               instead of custom segmented control —         │
│               it's the native SwiftUI pattern"              │
│                                                             │
│  Round 2: Designer refines based on feedback                │
│    Designer: "Updated. Using Picker(.segmented).            │
│              Added preview of color changes.                │
│              Proposed color palette for custom accents."     │
│    Research:  "LGTM. No further concerns."                  │
│                                                             │
│  Consensus reached → proceed to implementation              │
└─────────────────────────────────────────────────────────────┘
```

**How this discussion works in code:**

```go
func (o *Orchestrator) runDesignPhaseWithDiscussion(task *DevTask) (*PhaseResult, error) {
    // 1. Load research results from Emergent
    research := o.emergent.GetRelated(task.ID, "HAS_RESEARCH")

    // 2. Get initial design proposal from Designer
    designProposal := o.runAgent("claude-code", fmt.Sprintf(`
        Based on this research: %s

        Create a UI/UX design for adding dark mode toggle to the settings page.
        Include:
        - Component hierarchy
        - State management approach
        - User interaction flow
        - Specific SwiftUI code structure (not full implementation)
    `, research))

    // 3. Store proposal in Emergent
    proposalEntity := o.emergent.CreateEntity("DesignProposal", map[string]interface{}{
        "content":  designProposal,
        "status":   "proposed",
        "round":    1,
    })

    // 4. DISCUSSION LOOP — max 3 rounds
    for round := 1; round <= 3; round++ {
        // Get feedback from Research Agent
        feedback := o.runAgent("claude-code", fmt.Sprintf(`
            You are reviewing this design proposal for technical feasibility:

            PROPOSAL: %s
            RESEARCH CONTEXT: %s

            Evaluate:
            1. Does this align with the codebase patterns found in research?
            2. Are the SwiftUI APIs used correctly?
            3. Any technical concerns or better alternatives?

            Respond with either:
            - "APPROVED" + brief confirmation
            - "SUGGEST_CHANGES" + specific actionable feedback
        `, designProposal, research))

        // 5. Store feedback in Emergent
        o.emergent.CreateEntity("DesignFeedback", map[string]interface{}{
            "round":     round,
            "feedback":  feedback,
            "from":      "research-agent",
        })
        o.emergent.CreateRelationship(proposalEntity.ID, feedbackEntity.ID, "HAS_FEEDBACK")

        // 6. Check if approved
        if strings.Contains(feedback, "APPROVED") {
            o.emergent.UpdateEntity(proposalEntity.ID, map[string]interface{}{
                "status": "approved",
            })
            break
        }

        // 7. Refine design based on feedback
        designProposal = o.runAgent("claude-code", fmt.Sprintf(`
            Refine your design based on this feedback:
            ORIGINAL PROPOSAL: %s
            FEEDBACK: %s
            Create an updated proposal addressing the concerns.
        `, designProposal, feedback))

        // Update in Emergent
        o.emergent.UpdateEntity(proposalEntity.ID, map[string]interface{}{
            "content": designProposal,
            "round":   round + 1,
        })
    }

    return &PhaseResult{Phase: "design", Output: designProposal, Status: "completed"}, nil
}
```

**Design stored in Emergent:**
```
DevTask("dark-mode-toggle")
  ├── ResearchReport(...)                    ✅ completed
  ├── DesignProposal(...)                    ✅ approved (round 2)
  │     ├── DesignFeedback(round=1, "SUGGEST_CHANGES: use Picker")
  │     └── DesignFeedback(round=2, "APPROVED")
  ├── Phase("research")     status: completed
  ├── Phase("design")       status: completed
  ├── Phase("implement")    status: pending    ◄── next
  ├── Phase("test")         status: pending
  └── Phase("review")       status: pending
```

---

### PHASE 3: Implementation (~3-5 minutes)

This is where a real coding agent (OpenCode) does the work. The Orchestrator gives it everything it needs from Emergent.

```go
func (o *Orchestrator) runImplementPhase(task *DevTask) (*PhaseResult, error) {
    // 1. Gather ALL context from Emergent
    research := o.emergent.GetRelated(task.ID, "HAS_RESEARCH")
    design := o.emergent.GetRelated(task.ID, "HAS_DESIGN")

    // 2. Build comprehensive implementation prompt
    prompt := fmt.Sprintf(`
        TASK: %s

        RESEARCH FINDINGS:
        %s

        APPROVED DESIGN:
        %s

        IMPLEMENTATION INSTRUCTIONS:
        1. Modify SettingsView.swift to add an Appearance section with a
           Picker using .segmented style offering Light / Dark / System options
        2. Use @AppStorage("appearanceMode") to persist the choice
        3. Apply .preferredColorScheme() at the root view in DianeApp.swift
        4. Test that the toggle works by building the project

        FILES TO MODIFY:
        - Diane/Views/SettingsView.swift
        - Diane/DianeApp.swift

        After implementation:
        - Run 'xcodebuild -scheme Diane -configuration Debug build' to verify it compiles
        - Report which files were changed and what was done
    `, task.Description, research, design)

    // 3. Run the coding agent via ACP
    //    OpenCode gets the full workspace context
    run, err := o.acpClient.CreateRun(acp.RunCreateRequest{
        AgentName: "opencode",
        Input:     []acp.Message{acp.NewUserMessage(prompt)},
        Mode:      acp.RunModeAsync,  // async because it takes minutes
    })

    // 4. AGENT EXECUTION LOOP — poll for completion
    for {
        status, err := o.acpClient.GetRun(run.RunID)
        if err != nil {
            return nil, err
        }

        switch status.Status {
        case acp.RunStatusCompleted:
            // Parse output, extract changed files
            result := o.parseImplementationResult(status.GetTextOutput())

            // Store artifacts in Emergent
            for _, file := range result.ChangedFiles {
                o.emergent.CreateEntity("Artifact", map[string]interface{}{
                    "file_path":   file.Path,
                    "change_type": file.ChangeType, // "modified", "created"
                    "summary":     file.Summary,
                })
            }

            o.emergent.UpdateEntity(task.Phases["implement"].ID, map[string]interface{}{
                "status": "completed",
                "build_result": result.BuildResult,
            })

            return &PhaseResult{Phase: "implement", Output: result, Status: "completed"}, nil

        case acp.RunStatusFailed:
            // Store failure, may retry
            return nil, fmt.Errorf("implementation failed: %s", status.Error.Message)

        case acp.RunStatusAwaiting:
            // Agent needs input — escalate to user or auto-resolve
            o.handleAwaitingInput(status)

        default:
            // Still running, update progress in Emergent
            o.emergent.UpdateEntity(task.Phases["implement"].ID, map[string]interface{}{
                "status": "in_progress",
                "agent_status": string(status.Status),
            })
            time.Sleep(5 * time.Second)
        }
    }
}
```

---

### PHASE 4: Testing (~1-2 minutes)

The Test Agent runs independently from the Developer Agent. It gets the list of changed files from Emergent and verifies the implementation.

```go
func (o *Orchestrator) runTestPhase(task *DevTask) (*PhaseResult, error) {
    // 1. Get artifacts (changed files) from Emergent
    artifacts := o.emergent.GetRelated(task.ID, "HAS_ARTIFACT")
    design := o.emergent.GetRelated(task.ID, "HAS_DESIGN")

    // 2. Build test prompt — different agent, fresh perspective
    prompt := fmt.Sprintf(`
        TESTING TASK: Verify the dark mode toggle implementation

        EXPECTED BEHAVIOR (from design):
        %s

        FILES THAT WERE CHANGED:
        %s

        VERIFICATION STEPS:
        1. Read each changed file and verify the implementation matches the design
        2. Check that @AppStorage("appearanceMode") is used correctly
        3. Check that .preferredColorScheme() is applied at root view level
        4. Verify there are no hardcoded colors that won't adapt to dark mode
        5. Run: xcodebuild -scheme Diane -configuration Debug build
        6. Check for any SwiftUI preview issues
        7. Look for edge cases: what happens on first launch? what if stored
           value is invalid?

        Report:
        - PASS or FAIL for each verification step
        - If FAIL: specific issue description and suggested fix
        - Overall verdict: PASS, FAIL, or NEEDS_CHANGES
    `, design, formatArtifacts(artifacts))

    run, err := o.acpClient.RunSync("opencode", acp.RunCreateRequest{
        AgentName: "opencode",
        Input:     []acp.Message{acp.NewUserMessage(prompt)},
        Mode:      acp.RunModeSync,
    })

    testResult := o.parseTestResult(run.GetTextOutput())

    // Store test results in Emergent
    o.emergent.CreateEntity("TestResult", map[string]interface{}{
        "task_id":       task.ID,
        "verdict":       testResult.Verdict,    // "PASS", "FAIL", "NEEDS_CHANGES"
        "steps":         testResult.Steps,
        "issues":        testResult.Issues,
        "build_success": testResult.BuildSuccess,
    })

    // 3. RETRY LOOP — if tests fail, send back to implementation
    if testResult.Verdict == "FAIL" || testResult.Verdict == "NEEDS_CHANGES" {
        return &PhaseResult{
            Phase:  "test",
            Output: testResult,
            Status: "failed",
            RetryWith: &RetryInstruction{
                GoToPhase: "implement",
                Context:   testResult.Issues,
                Message:   "Fix these issues found during testing",
            },
        }, nil
    }

    return &PhaseResult{Phase: "test", Output: testResult, Status: "completed"}, nil
}
```

---

### PHASE 5: Review (~1 minute)

A **different coding agent** (Gemini CLI) reviews the code for quality, providing a fresh perspective.

```go
func (o *Orchestrator) runReviewPhase(task *DevTask) (*PhaseResult, error) {
    artifacts := o.emergent.GetRelated(task.ID, "HAS_ARTIFACT")
    testResults := o.emergent.GetRelated(task.ID, "HAS_TEST_RESULT")

    prompt := fmt.Sprintf(`
        CODE REVIEW: Dark mode toggle implementation

        Review the following changed files for:
        1. Code quality and SwiftUI best practices
        2. Consistency with existing codebase patterns
        3. Accessibility considerations
        4. Performance implications
        5. Edge cases and error handling

        Changed files: %s
        Test results: %s

        Provide:
        - Overall assessment (APPROVE, REQUEST_CHANGES, or COMMENT)
        - Specific inline comments with file:line references
        - Suggestions for improvement (optional vs required)
    `, formatArtifacts(artifacts), testResults)

    // Use a DIFFERENT agent for review — fresh perspective
    run, err := o.acpClient.RunSync("gemini", acp.RunCreateRequest{
        AgentName: "gemini",
        Input:     []acp.Message{acp.NewUserMessage(prompt)},
        Mode:      acp.RunModeSync,
    })

    review := o.parseReviewResult(run.GetTextOutput())

    o.emergent.CreateEntity("CodeReview", map[string]interface{}{
        "verdict":     review.Verdict,
        "comments":    review.Comments,
        "suggestions": review.Suggestions,
    })

    if review.Verdict == "REQUEST_CHANGES" {
        // Filter for required changes only
        requiredChanges := filterRequired(review.Comments)
        return &PhaseResult{
            Phase:  "review",
            Status: "failed",
            RetryWith: &RetryInstruction{
                GoToPhase: "implement",
                Context:   requiredChanges,
                Message:   "Address these code review comments",
            },
        }, nil
    }

    return &PhaseResult{Phase: "review", Output: review, Status: "completed"}, nil
}
```

---

## The Orchestrator Main Loop

This is the **top-level loop** that ties everything together:

```go
func (o *Orchestrator) ExecuteDevTask(task *DevTask) error {
    maxRetries := 3
    retryCount := 0

    // Phase execution order
    phases := []Phase{
        {Name: "research",  Runner: o.runResearchPhase},
        {Name: "design",    Runner: o.runDesignPhaseWithDiscussion},
        {Name: "implement", Runner: o.runImplementPhase},
        {Name: "test",      Runner: o.runTestPhase},
        {Name: "review",    Runner: o.runReviewPhase},
    }

    currentPhase := 0

    // ═══════════════════════════════════════════════════════
    // MAIN ORCHESTRATOR LOOP
    // ═══════════════════════════════════════════════════════
    for currentPhase < len(phases) {
        phase := phases[currentPhase]

        // Update state in Emergent
        o.emergent.UpdateEntity(task.ID, map[string]interface{}{
            "current_phase": phase.Name,
            "status":        "in_progress",
        })

        // Notify user of progress
        o.notify(fmt.Sprintf("Phase %d/%d: %s started", currentPhase+1, len(phases), phase.Name))

        // ═══════════════════════════════════════════════════
        // RUN PHASE
        // ═══════════════════════════════════════════════════
        result, err := phase.Runner(task)

        if err != nil {
            // Hard error — agent crashed, network issue, etc.
            o.emergent.CreateEntity("Error", map[string]interface{}{
                "phase":   phase.Name,
                "error":   err.Error(),
                "retry":   retryCount,
            })

            if retryCount < maxRetries {
                retryCount++
                o.notify(fmt.Sprintf("Phase %s failed, retrying (%d/%d): %s",
                    phase.Name, retryCount, maxRetries, err.Error()))
                continue // retry same phase
            }

            // Max retries exceeded — escalate to user
            o.escalateToUser(task, phase.Name, err)
            return err
        }

        // ═══════════════════════════════════════════════════
        // HANDLE PHASE RESULT
        // ═══════════════════════════════════════════════════
        if result.Status == "completed" {
            // Phase succeeded — move forward
            retryCount = 0
            currentPhase++
            o.notify(fmt.Sprintf("Phase %s completed successfully", phase.Name))

        } else if result.RetryWith != nil {
            // Phase failed with retry instructions (e.g., test failure)
            retryCount++

            if retryCount > maxRetries {
                o.escalateToUser(task, phase.Name, fmt.Errorf("max retries exceeded"))
                return fmt.Errorf("max retries exceeded for phase %s", phase.Name)
            }

            // ═════════════════════════════════════════════
            // RETRY LOOP — go back to earlier phase
            // ═════════════════════════════════════════════
            //
            // This is the critical feedback loop:
            //   TEST fails → go back to IMPLEMENT with failure context
            //   REVIEW requests changes → go back to IMPLEMENT
            //
            targetPhase := result.RetryWith.GoToPhase
            for i, p := range phases {
                if p.Name == targetPhase {
                    currentPhase = i

                    // Inject failure context into next run
                    // The implement agent sees what went wrong
                    task.RetryContext = &RetryContext{
                        FailedPhase:  phase.Name,
                        Issues:       result.RetryWith.Context,
                        AttemptNumber: retryCount,
                        Message:      result.RetryWith.Message,
                    }

                    o.notify(fmt.Sprintf(
                        "Phase %s needs changes → returning to %s (attempt %d/%d)",
                        phase.Name, targetPhase, retryCount, maxRetries))
                    break
                }
            }
        }
    }

    // ═══════════════════════════════════════════════════════
    // TASK COMPLETE
    // ═══════════════════════════════════════════════════════
    o.emergent.UpdateEntity(task.ID, map[string]interface{}{
        "status":       "completed",
        "completed_at": time.Now().Format(time.RFC3339),
    })

    o.notify(fmt.Sprintf("Task '%s' completed successfully!", task.Description))
    return nil
}
```

---

## The Complete Flow Diagram

```
USER: "Add dark mode toggle to macOS app settings"
 │
 ▼
╔═══════════════════════════════════════════════════════════════╗
║                    ORCHESTRATOR LOOP                         ║
║                                                              ║
║  ┌─────────────────────────────────────────────────────────┐ ║
║  │ Phase 1: RESEARCH                                       │ ║
║  │                                                         │ ║
║  │  Orchestrator ──ACP──► Claude Code (research mode)      │ ║
║  │       │                      │                          │ ║
║  │       │              reads codebase, finds patterns     │ ║
║  │       │                      │                          │ ║
║  │       ◄──────── research report ────┘                   │ ║
║  │       │                                                 │ ║
║  │       └──► Emergent: store ResearchReport               │ ║
║  │                                                         │ ║
║  │  Status: ✅ completed                                    │ ║
║  └─────────────────────────────────────────────────────────┘ ║
║                          │                                   ║
║                          ▼                                   ║
║  ┌─────────────────────────────────────────────────────────┐ ║
║  │ Phase 2: DESIGN (with Discussion Loop)                  │ ║
║  │                                                         │ ║
║  │  ┌────────────── DISCUSSION LOOP ──────────────────┐    │ ║
║  │  │                                                 │    │ ║
║  │  │  Round 1:                                       │    │ ║
║  │  │    Designer ──proposal──► Researcher            │    │ ║
║  │  │    Designer ◄──feedback── Researcher            │    │ ║
║  │  │    Verdict: SUGGEST_CHANGES                     │    │ ║
║  │  │                                                 │    │ ║
║  │  │  Round 2:                                       │    │ ║
║  │  │    Designer ──refined──► Researcher             │    │ ║
║  │  │    Designer ◄──"APPROVED"── Researcher          │    │ ║
║  │  │    Verdict: APPROVED ✅                          │    │ ║
║  │  │                                                 │    │ ║
║  │  └─────────────────────────────────────────────────┘    │ ║
║  │                                                         │ ║
║  │  └──► Emergent: store DesignProposal + Feedback         │ ║
║  │                                                         │ ║
║  │  Status: ✅ completed                                    │ ║
║  └─────────────────────────────────────────────────────────┘ ║
║                          │                                   ║
║                          ▼                                   ║
║  ┌─────────────────────────────────────────────────────────┐ ║
║  │ Phase 3: IMPLEMENT                                      │ ║
║  │                                                         │ ║
║  │  Orchestrator ──ACP──► OpenCode (implementation)        │ ║
║  │       │                      │                          │ ║
║  │       │   ┌── AGENT EXECUTION LOOP ──┐                  │ ║
║  │       │   │  poll every 5s           │                  │ ║
║  │       │   │  status: in_progress     │                  │ ║
║  │       │   │  status: in_progress     │                  │ ║
║  │       │   │  status: completed ✅     │                  │ ║
║  │       │   └──────────────────────────┘                  │ ║
║  │       │                      │                          │ ║
║  │       ◄───── changed files list ────┘                   │ ║
║  │       │                                                 │ ║
║  │       └──► Emergent: store Artifacts                    │ ║
║  │                                                         │ ║
║  │  Status: ✅ completed                                    │ ║
║  └─────────────────────────────────────────────────────────┘ ║
║                          │                                   ║
║                          ▼                                   ║
║  ┌─────────────────────────────────────────────────────────┐ ║
║  │ Phase 4: TEST                              attempt 1    │ ║
║  │                                                         │ ║
║  │  Orchestrator ──ACP──► OpenCode (test mode)             │ ║
║  │       │                      │                          │ ║
║  │       │              reads changed files                │ ║
║  │       │              runs xcodebuild                    │ ║
║  │       │              checks implementation vs design    │ ║
║  │       │                      │                          │ ║
║  │       ◄──── VERDICT: NEEDS_CHANGES ─┘                   │ ║
║  │       │     "Missing .preferredColorScheme on            │ ║
║  │       │      ContentView — only applied to              │ ║
║  │       │      SettingsView, not the whole app"           │ ║
║  │       │                                                 │ ║
║  │  ══════════════════════════════════════                  │ ║
║  │  RETRY: go back to Phase 3 with context                 │ ║
║  │  ══════════════════════════════════════                  │ ║
║  └───────────────────────┬─────────────────────────────────┘ ║
║                          │                                   ║
║              ┌───────────┘  (retry loop)                     ║
║              ▼                                               ║
║  ┌─────────────────────────────────────────────────────────┐ ║
║  │ Phase 3: IMPLEMENT (retry attempt 2)                    │ ║
║  │                                                         │ ║
║  │  Context injected:                                      │ ║
║  │    "Fix: .preferredColorScheme must be applied at       │ ║
║  │     root view level in DianeApp.swift, not just         │ ║
║  │     SettingsView"                                       │ ║
║  │                                                         │ ║
║  │  OpenCode fixes the specific issue                      │ ║
║  │                                                         │ ║
║  │  Status: ✅ completed                                    │ ║
║  └─────────────────────────────────────────────────────────┘ ║
║                          │                                   ║
║                          ▼                                   ║
║  ┌─────────────────────────────────────────────────────────┐ ║
║  │ Phase 4: TEST (attempt 2)                               │ ║
║  │                                                         │ ║
║  │  All checks pass                                        │ ║
║  │  Build succeeds                                         │ ║
║  │  VERDICT: PASS ✅                                        │ ║
║  └─────────────────────────────────────────────────────────┘ ║
║                          │                                   ║
║                          ▼                                   ║
║  ┌─────────────────────────────────────────────────────────┐ ║
║  │ Phase 5: REVIEW                                         │ ║
║  │                                                         │ ║
║  │  Gemini CLI reviews code quality                        │ ║
║  │  VERDICT: APPROVE ✅                                     │ ║
║  │  Comments: "Good use of @AppStorage. Consider adding    │ ║
║  │   .animation(.easeInOut) for smooth transitions."       │ ║
║  └─────────────────────────────────────────────────────────┘ ║
║                                                              ║
╚══════════════════════════════════════════════════════════════╝
 │
 ▼
TASK COMPLETE — User notified via Discord/notification
```

---

## Final Emergent Graph State

After the task completes, all knowledge is persisted in the Emergent graph:

```
DevTask("dark-mode-toggle")  status: completed
 │
 ├── PHASE ──► Phase("research")     status: completed
 │               └── PRODUCED ──► ResearchReport(findings, approach, files, APIs)
 │
 ├── PHASE ──► Phase("design")       status: completed
 │               └── PRODUCED ──► DesignProposal(segmented picker, @AppStorage)
 │                                   ├── HAS_FEEDBACK ──► Feedback(round=1, SUGGEST_CHANGES)
 │                                   └── HAS_FEEDBACK ──► Feedback(round=2, APPROVED)
 │
 ├── PHASE ──► Phase("implement")    status: completed
 │               ├── PRODUCED ──► Artifact("SettingsView.swift", modified)
 │               └── PRODUCED ──► Artifact("DianeApp.swift", modified)
 │
 ├── PHASE ──► Phase("test")         status: completed
 │               ├── PRODUCED ──► TestResult(attempt=1, NEEDS_CHANGES)
 │               └── PRODUCED ──► TestResult(attempt=2, PASS)
 │
 └── PHASE ──► Phase("review")       status: completed
                 └── PRODUCED ──► CodeReview(APPROVE, suggestions=[animation])
```

This graph is **queryable later**. Next time someone asks "why did we implement dark mode this way?", the full decision history — including the designer/researcher discussion, the test failure and fix, and the review suggestions — is all there.

---

## Triggering Modes

This same orchestration loop can be triggered in multiple ways:

### 1. User Request (Interactive)
```
User ──MCP──► Diane ──► Orchestrator.ExecuteDevTask(...)
```

### 2. Scheduled (Cron Job)
```yaml
# In Diane's job scheduler
- name: "weekly-dependency-audit"
  schedule: "0 9 * * MON"
  action_type: "agent"
  agent_task:
    type: "dev_task"
    description: "Audit all npm/go dependencies for security vulnerabilities"
    phases: ["research", "implement", "test"]  # no design/review needed
```

### 3. Event-Driven (Webhook / File Watch)
```go
// GitHub webhook → Diane → Orchestrator
func (o *Orchestrator) OnGitHubIssueCreated(issue GitHubIssue) {
    if issue.HasLabel("auto-fix") {
        task := &DevTask{
            Description: issue.Title,
            Context:     issue.Body,
            Source:      "github-issue",
            SourceRef:   issue.URL,
        }
        go o.ExecuteDevTask(task) // async execution
    }
}
```

### 4. Agent-Initiated (Proactive)
```go
// Research Agent finds a problem during routine monitoring
func (o *Orchestrator) OnAgentAlert(alert AgentAlert) {
    if alert.Severity == "high" && alert.Type == "security_vulnerability" {
        task := &DevTask{
            Description: fmt.Sprintf("Fix security vulnerability: %s", alert.Summary),
            Context:     alert.Details,
            Source:      "agent-alert",
            Priority:    "critical",
        }
        // Requires user approval for critical changes
        o.requestApproval(task, func() {
            go o.ExecuteDevTask(task)
        })
    }
}
```

---

## Resource & Cost Management

Each agent run costs tokens. The orchestrator tracks this:

```go
type CostTracker struct {
    Budget     float64            // max cost for this task
    Spent      float64            // cost so far
    ByPhase    map[string]float64 // cost per phase
    ByAgent    map[string]float64 // cost per agent
}

// Before each agent call
func (o *Orchestrator) beforeAgentCall(agentName string, estimatedTokens int) error {
    estimatedCost := o.estimateCost(agentName, estimatedTokens)
    if o.costs.Spent + estimatedCost > o.costs.Budget {
        return fmt.Errorf("budget exceeded: spent $%.2f of $%.2f budget",
            o.costs.Spent, o.costs.Budget)
    }
    return nil
}
```

---

## Parallel Agent Execution

Some phases can run agents in parallel:

```go
// Example: Research and Design can partially overlap
// Research produces initial findings → Design starts
// Research continues with deeper analysis → feeds into Design refinement

func (o *Orchestrator) runParallelPhases(task *DevTask) {
    var wg sync.WaitGroup

    // Run research and initial design concurrently
    wg.Add(2)

    go func() {
        defer wg.Done()
        o.runResearchPhase(task)
    }()

    go func() {
        defer wg.Done()
        // Wait for initial research findings before starting design
        <-task.Phases["research"].InitialFindings
        o.runDesignPhase(task)
    }()

    wg.Wait()
}
```

---

## Summary: The Three Loops

| Loop | Level | What it does | When it repeats |
|------|-------|-------------|-----------------|
| **Orchestrator Loop** | Outer | Walks through phases, handles retries | Retries when test/review fails → goes back to implement |
| **Discussion Loop** | Middle | Agents exchange proposals and feedback | Repeats until consensus (max 3 rounds) |
| **Agent Execution Loop** | Inner | Polls async ACP agent for completion | Every 5s until agent finishes |

The key insight: **Diane doesn't implement the agents** — it **orchestrates real coding agents** (OpenCode, Claude Code, Gemini) that already exist in its ACP gallery. The orchestrator is a new Go component that manages the coordination, stores state in Emergent, and handles the retry/feedback loops.
