## Why

AI coding assistants today lack a structured way to understand project specifications, track implementation progress, and coordinate parallel work. OpenSpec provides a file-based spec-driven workflow, but its flat-file approach cannot express the rich relationships between entities (requirements, scenarios, components, tasks), perform impact analysis, or support concurrent agent execution. SpecMCP bridges this gap by exposing a graph-backed spec-driven development workflow through the Model Context Protocol, enabling any MCP-compatible AI assistant to work with structured specifications stored in Emergent's knowledge graph.

## What Changes

- **New Go MCP server** (`server/specmcp/`) — stdio-based MCP server that AI coding assistants connect to
- **Emergent template pack** — Defines 17 entity types and ~30 relationship types for spec-driven development
- **Workflow tools** — Create changes with proposal/specs/design/tasks artifacts following the OpenSpec model
- **Query tools** — Explore contexts, components, actions, scenarios, and run impact analysis
- **Task management tools** — Generate tasks from specs, track dependencies, assign to agents, calculate parallel capacity and critical paths
- **Sync tools** — Analyze codebases, extract entities from source code, track git-aware synchronization
- **Pattern tools** — Detect, suggest, and apply reusable implementation patterns
- **Constitution support** — Project-wide non-negotiable rules that govern all changes
- **API contract support** — Machine-readable API definitions (OpenAPI, AsyncAPI, etc.) linked to specs

## Capabilities

### New Capabilities
- `template-pack`: Emergent template pack with all entity and relationship type definitions for the spec-driven domain model
- `mcp-server-scaffold`: Go project structure, MCP stdio transport, Emergent SDK client, configuration, and tool registration
- `query-tools`: Read-only tools for exploring the knowledge graph — get contexts, components, actions, scenarios, patterns, and run impact analysis
- `workflow-tools`: Change lifecycle management — create changes with proposals, add specs/requirements/scenarios, add designs, archive completed changes
- `task-management`: Task generation from specs, dependency tracking, parallel execution, agent assignment, progress calculation, velocity tracking, and critical path analysis
- `sync-tools`: Codebase analysis, entity extraction from source code, git commit tracking, incremental delta detection
- `pattern-tools`: Core pattern library, AI-assisted pattern suggestions, pattern application and confirmation workflow
- `constitution-and-contracts`: Constitution enforcement (project-wide rules) and API contract management (machine-readable definitions linked to specs)

### Modified Capabilities

## Impact

- **New binary**: `specmcp` Go binary added to `server/specmcp/`
- **Emergent dependency**: Requires Emergent running at localhost:3002 with the SpecMCP template pack installed
- **Go SDK dependency**: `github.com/emergent-company/emergent/apps/server-go/pkg/sdk@v0.7.0`
- **MCP ecosystem**: Any MCP-compatible client (Claude Desktop, OpenCode, Cursor, etc.) can connect via stdio
- **No existing code modified**: This is an entirely new subsystem within the Diane project
