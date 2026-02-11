# SpecMCP Implementation Plan

**Created**: 2026-02-11  
**Updated**: 2026-02-11  
**Status**: Ready for Implementation

## Overview

This document outlines the step-by-step implementation plan for SpecMCP, organized into phases with task dependencies.

---

## Phase 1: Foundation (Template Pack)

Create the Emergent template pack that defines all entity and relationship types.

### Tasks

| ID | Task | Complexity | Dependencies | Status |
|----|------|------------|--------------|--------|
| P1-T1 | Research Emergent template pack format and API | 3 | - | pending |
| P1-T2 | Create Actor entity type definition | 2 | P1-T1 | pending |
| P1-T3 | Create CodingAgent entity type definition | 3 | P1-T1 | pending |
| P1-T4 | Create Pattern entity type definition | 3 | P1-T1 | pending |
| P1-T5 | Create Constitution entity type definition | 3 | P1-T1 | pending |
| P1-T6 | Create Change management entity types (Change, Proposal, Spec, Requirement, Design) | 5 | P1-T1 | pending |
| P1-T7 | Create Scenario entity types (Scenario, ScenarioStep) | 4 | P1-T1 | pending |
| P1-T8 | Create TestCase entity type definition | 3 | P1-T1 | pending |
| P1-T9 | Create APIContract entity type definition | 3 | P1-T1 | pending |
| P1-T10 | Create Task entity type definition | 4 | P1-T1 | pending |
| P1-T11 | Create Structural entity types (Context, UIComponent, Action) | 5 | P1-T1 | pending |
| P1-T12 | Create GraphSync entity type definition | 2 | P1-T1 | pending |
| P1-T13 | Define all relationship types (including ownership) | 6 | P1-T2 through P1-T12 | pending |
| P1-T14 | Register template pack with Emergent | 3 | P1-T13 | pending |
| P1-T15 | Test template pack with manual entity creation | 3 | P1-T14 | pending |

**Phase 1 Total**: 52 points

### Deliverables

- `specmcp-template-pack.yaml` - Complete template pack definition
- Template pack registered in Emergent
- Verified with test entities

---

## Phase 2: MCP Server Scaffold

Set up the Go project structure and basic MCP protocol implementation.

### Tasks

| ID | Task | Complexity | Dependencies | Status |
|----|------|------------|--------------|--------|
| P2-T1 | Initialize Go module and project structure | 2 | - | pending |
| P2-T2 | Set up configuration loading (YAML, env vars) | 3 | P2-T1 | pending |
| P2-T3 | Implement Emergent API client (REST) | 5 | P2-T1 | pending |
| P2-T4 | Implement MCP stdio transport | 4 | P2-T1 | pending |
| P2-T5 | Implement MCP tool registration | 4 | P2-T4 | pending |
| P2-T6 | Add logging and error handling | 3 | P2-T2 | pending |
| P2-T7 | Create Makefile with build targets | 2 | P2-T1 | pending |
| P2-T8 | Add basic integration test setup | 3 | P2-T3, P2-T5 | pending |

**Phase 2 Total**: 26 points

### Deliverables

- `/server/specmcp/` - Go project directory
- Working MCP server that responds to `tools/list`
- Emergent API client with authentication

---

## Phase 3: Core Query Tools

Implement read-only query tools for exploring the graph.

### Tasks

| ID | Task | Complexity | Dependencies | Status |
|----|------|------------|--------------|--------|
| P3-T1 | Implement `spec_get_context` tool | 4 | P2-T3, P2-T5 | pending |
| P3-T2 | Implement `spec_get_component` tool | 4 | P2-T3, P2-T5 | pending |
| P3-T3 | Implement `spec_get_action` tool | 3 | P2-T3, P2-T5 | pending |
| P3-T4 | Implement `spec_get_scenario` tool | 4 | P2-T3, P2-T5 | pending |
| P3-T5 | Implement `spec_get_patterns` tool | 3 | P2-T3, P2-T5 | pending |
| P3-T6 | Implement `spec_impact_analysis` tool | 6 | P3-T1 through P3-T5 | pending |
| P3-T7 | Add query result formatting | 3 | P3-T1 | pending |
| P3-T8 | Write tests for query tools | 4 | P3-T1 through P3-T6 | pending |

**Phase 3 Total**: 31 points

### Deliverables

- Working query tools for all entity types
- Impact analysis capability
- Test coverage for query tools

---

## Phase 4: Workflow Tools

Implement tools for creating and managing changes.

### Tasks

| ID | Task | Complexity | Dependencies | Status |
|----|------|------------|--------------|--------|
| P4-T1 | Implement `spec_new` tool (create change + proposal) | 5 | P2-T3, P2-T5 | pending |
| P4-T2 | Implement `spec_artifact` tool (add requirements, scenarios, etc.) | 6 | P4-T1 | pending |
| P4-T3 | Implement `spec_archive` tool | 3 | P4-T1 | pending |
| P4-T4 | Implement scenario step creation logic | 4 | P4-T2 | pending |
| P4-T5 | Implement automatic relationship creation | 5 | P4-T2 | pending |
| P4-T6 | Add validation for workflow transitions | 4 | P4-T1 through P4-T3 | pending |
| P4-T7 | Write tests for workflow tools | 4 | P4-T1 through P4-T5 | pending |

**Phase 4 Total**: 31 points

### Deliverables

- Complete change management workflow
- Validation for state transitions
- Test coverage for workflow tools

---

## Phase 5: Task Management Tools

Implement task generation, assignment, and tracking.

### Tasks

| ID | Task | Complexity | Dependencies | Status |
|----|------|------------|--------------|--------|
| P5-T1 | Implement `spec_generate_tasks` tool | 7 | P4-T2 | pending |
| P5-T2 | Implement dependency detection algorithm | 6 | P5-T1 | pending |
| P5-T3 | Implement `spec_get_available_tasks` tool | 4 | P5-T1 | pending |
| P5-T4 | Implement `spec_get_parallel_capacity` tool | 3 | P5-T3 | pending |
| P5-T5 | Implement `spec_assign_task` tool | 4 | P5-T3 | pending |
| P5-T6 | Implement `spec_complete_task` tool | 4 | P5-T5 | pending |
| P5-T7 | Implement `spec_get_scenario_progress` tool | 4 | P5-T1 | pending |
| P5-T8 | Implement `spec_get_critical_path` tool | 5 | P5-T2 | pending |
| P5-T9 | Implement velocity tracking | 4 | P5-T6 | pending |
| P5-T10 | Write tests for task management | 5 | P5-T1 through P5-T9 | pending |

**Phase 5 Total**: 46 points

### Deliverables

- Automatic task generation from scenarios
- Parallel execution support
- Progress tracking and velocity metrics
- Test coverage for task management

---

## Phase 6: Sync Tools

Implement codebase synchronization.

### Tasks

| ID | Task | Complexity | Dependencies | Status |
|----|------|------------|--------------|--------|
| P6-T1 | Implement `spec_sync_status` tool | 3 | P2-T3 | pending |
| P6-T2 | Implement git commit tracking | 4 | P6-T1 | pending |
| P6-T3 | Implement context extraction (screen detection) | 6 | P6-T2 | pending |
| P6-T4 | Implement component extraction | 5 | P6-T2 | pending |
| P6-T5 | Implement action extraction | 5 | P6-T2 | pending |
| P6-T6 | Implement `spec_sync` tool (full sync) | 6 | P6-T3 through P6-T5 | pending |
| P6-T7 | Implement incremental sync (delta detection) | 7 | P6-T6 | pending |
| P6-T8 | Write tests for sync tools | 5 | P6-T1 through P6-T7 | pending |

**Phase 6 Total**: 41 points

### Deliverables

- Codebase analysis and extraction
- Git-aware synchronization
- Incremental update detection
- Test coverage for sync tools

---

## Phase 7: Pattern Tools

Implement pattern detection and management.

### Tasks

| ID | Task | Complexity | Dependencies | Status |
|----|------|------------|--------------|--------|
| P7-T1 | Create core pattern library (seed data) | 4 | P1-T11 | pending |
| P7-T2 | Implement `spec_suggest_patterns` tool | 6 | P7-T1, P3-T5 | pending |
| P7-T3 | Implement `spec_apply_pattern` tool | 3 | P3-T5 | pending |
| P7-T4 | Implement pattern confirmation workflow | 4 | P7-T2 | pending |
| P7-T5 | Write tests for pattern tools | 4 | P7-T1 through P7-T4 | pending |

**Phase 7 Total**: 21 points

### Deliverables

- Core pattern library with common patterns
- AI-assisted pattern suggestions
- Pattern application and tracking
- Test coverage for pattern tools

---

## Phase 8: Integration & Polish

Final integration, documentation, and release preparation.

### Tasks

| ID | Task | Complexity | Dependencies | Status |
|----|------|------------|--------------|--------|
| P8-T1 | End-to-end integration testing | 5 | All previous phases | pending |
| P8-T2 | Performance optimization | 4 | P8-T1 | pending |
| P8-T3 | Create MCP configuration examples | 2 | P8-T1 | pending |
| P8-T4 | Write user documentation | 4 | P8-T1 | pending |
| P8-T5 | Create demo/tutorial scenario | 3 | P8-T4 | pending |
| P8-T6 | Release build and distribution | 3 | P8-T1 through P8-T5 | pending |

**Phase 8 Total**: 21 points

### Deliverables

- Fully tested MCP server
- User documentation
- Demo/tutorial
- Release artifacts

---

## Summary

| Phase | Description | Points | Dependencies |
|-------|-------------|--------|--------------|
| P1 | Foundation (Template Pack) | 52 | - |
| P2 | MCP Server Scaffold | 26 | P1 |
| P3 | Core Query Tools | 31 | P2 |
| P4 | Workflow Tools | 31 | P2 |
| P5 | Task Management Tools | 46 | P4 |
| P6 | Sync Tools | 41 | P2 |
| P7 | Pattern Tools | 21 | P1, P3 |
| P8 | Integration & Polish | 21 | All |

**Total**: 269 points

### Dependency Graph

```
P1 (Template Pack)
├──────────────────────────────► P7 (Pattern Tools)
│                                      │
▼                                      ▼
P2 (MCP Scaffold)                      │
├──────────────────────────────────────┘
├──► P3 (Query Tools) ─────────────────┐
│         │                            │
├──► P4 (Workflow Tools)               │
│         │                            │
│         ▼                            │
│    P5 (Task Management)              │
│         │                            │
├──► P6 (Sync Tools)                   │
│         │                            │
▼         ▼                            ▼
└──────► P8 (Integration) ◄────────────┘
```

### Parallel Execution Opportunities

**After P1 completes:**
- P2 and P7-T1 can run in parallel

**After P2 completes:**
- P3, P4, P6 can run in parallel

**After P3 and P4 complete:**
- P5 and remaining P7 can run in parallel

---

## Getting Started

### Prerequisites

1. Emergent running locally at `localhost:3002`
2. Go 1.21+ installed
3. Access to Emergent API key

### First Steps

1. Start with P1-T1: Research Emergent template pack format
2. Study existing template packs in Emergent codebase
3. Create entity type definitions one at a time
4. Test each entity type before moving to next

### Development Environment

```bash
# Clone and setup
cd /Users/mcj/code/emgt/diane
mkdir -p server/specmcp

# Emergent API access
export EMERGENT_URL="http://localhost:3002"
export EMERGENT_API_KEY="your-api-key"
export EMERGENT_PROJECT_ID="b697f070-f13a-423b-8678-04978fd39e21"
```

---

## Progress Tracking

Use this document to track progress. Update task status as work progresses:

- `pending` - Not started
- `in_progress` - Currently being worked on
- `completed` - Done
- `blocked` - Waiting on dependency

When completing a task, note the actual complexity and any learnings for future estimation.
