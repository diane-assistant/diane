## Context

SpecMCP is a new Go MCP server that provides spec-driven development workflow to AI coding assistants via the Model Context Protocol. It uses Emergent's knowledge graph as its backend for storing entities (specs, requirements, scenarios, tasks, components, etc.) and their relationships. The server communicates over stdio (JSON-RPC 2.0) and delegates all persistence to the Emergent API at localhost:3002 via the published Go SDK.

The existing codebase has no Go MCP server infrastructure — this is greenfield. The Emergent Go SDK (`sdk@v0.7.0`) provides REST client wrappers for graph operations, template packs, branches, search, and type registry. The SDK is well-structured with sub-clients for each domain.

Key constraints:
- Emergent must be running locally (or accessible via network) for any graph operations
- MCP protocol requires stdout for communication; all logging must go to stderr
- Template packs are JSON structures stored in the database, defining object types (with property schemas) and relationship types (with source/target constraints)

SDK v0.7.0 capabilities (all previously-identified gaps are now resolved):
- **Template Pack CRUD**: `TemplatePacks.CreatePack`, `GetPack`, `DeletePack` — full lifecycle
- **Type Schema Registration**: `TypeRegistry.CreateType`, `UpdateType`, `DeleteType` — register custom types per-project
- **Property-level Filtering**: `PropertyFilter` on `ListObjects` with 9 operators (eq, neq, gt, gte, lt, lte, contains, exists, in)
- **Bulk Operations**: `Graph.BulkCreateObjects` and `Graph.BulkCreateRelationships` (max 100 items, partial-success)
- **Inverse Relationships**: `GraphRelationship.InverseRelationship` field — server auto-creates inverse when template pack declares `inverseType`
- **Thread-safe SetContext**: `sync.RWMutex` on all sub-clients
- **Custom HTTP client**: `Config.HTTPClient` field for timeout/transport customization

## Goals / Non-Goals

**Goals:**
- Implement a fully functional MCP server exposing ~20 tools for spec-driven development
- Define a complete Emergent template pack (17 entity types, ~30 relationship types)
- Support the full OpenSpec-inspired workflow: Change → Proposal → Specs → Design → Tasks → Implementation
- Enable parallel task execution via dependency-aware task management
- Support codebase synchronization (entity extraction from source files)
- Provide impact analysis and pattern management capabilities

**Non-Goals:**
- Multi-project support (single project per server instance for v1)
- Branch-aware workflows (no Emergent branch operations in v1)
- Real-time CI/CD integration (manual task completion only)
- Web UI or dashboard (MCP tools only)
- Generating code from specs (spec-anchored, not spec-as-source)
- SSE or HTTP transport (stdio only for v1)

## Decisions

### 1. Project Structure

**Decision**: Use a flat internal package layout within `server/specmcp/`.

```
server/specmcp/
├── cmd/specmcp/main.go          # Entrypoint
├── internal/
│   ├── config/                   # Configuration loading
│   ├── emergent/                 # Emergent SDK wrapper with domain helpers
│   ├── mcp/                     # MCP protocol (transport, handler, registry)
│   └── tools/                   # Tool implementations
│       ├── query/               # spec_get_context, spec_get_component, etc.
│       ├── workflow/            # spec_new, spec_artifact, spec_archive
│       ├── tasks/               # spec_generate_tasks, spec_assign_task, etc.
│       ├── sync/                # spec_sync, spec_sync_status
│       └── patterns/            # spec_get_patterns, spec_suggest_patterns, etc.
├── templates/                    # Template pack JSON definition
├── go.mod
├── go.sum
└── Makefile
```

**Rationale**: The `internal/` package prevents external imports while keeping the code organized by domain. Tool implementations are grouped by capability area matching the spec structure. This mirrors the Emergent SDK's own sub-client pattern.

**Alternative considered**: Single `pkg/` directory with all tools flat — rejected because 20+ tool files in one directory hinders navigation and the tools naturally group by domain.

### 2. MCP Protocol Implementation

**Decision**: Implement the MCP protocol directly using JSON-RPC 2.0 over stdin/stdout, without a third-party MCP library.

**Rationale**: The MCP protocol surface we need is small — `initialize`, `tools/list`, `tools/call`, and `notifications/initialized`. A direct implementation gives us full control, no dependency risk, and is straightforward in Go with `encoding/json` and `bufio.Scanner` for line-delimited JSON.

**Alternative considered**: Using an MCP Go SDK (e.g., `mark3labs/mcp-go`) — rejected because adding a dependency for a simple protocol wrapper is unnecessary, and we'd need to understand the library's abstraction anyway.

### 3. Emergent Client Wrapper

**Decision**: Create a thin domain-specific wrapper around the Emergent SDK that exposes typed operations (e.g., `CreateChange(name)`, `GetContext(name)`, `GetAvailableTasks()`) rather than having tools call raw SDK methods.

**Rationale**: The SDK deals in generic `GraphObject` and `GraphRelationship` types. A wrapper provides:
- Type-safe entity creation (properties validated at compile time via Go structs)
- Reusable query patterns (expand graph, traverse relationships)
- Single place to handle entity-to-struct mapping
- Easier testing (mock the wrapper, not the SDK)

**Alternative considered**: Tools calling SDK directly — rejected because property name strings and map[string]any would be scattered across all tool implementations.

### 4. Template Pack Creation

**Decision**: Define the template pack as a JSON file in `templates/specmcp-pack.json` and provide a `make seed` command that creates it via `TemplatePacks.CreatePack()` from the SDK, then assigns it to the project with `TemplatePacks.AssignPack()`.

**Rationale**: Template packs are the schema definition layer — they must exist before any entities can be created. The SDK v0.7.0 provides full template pack CRUD via `CreatePack`, `GetPack`, and `DeletePack`. A seed script reads the JSON definition and calls the SDK directly — no raw HTTP needed.

**Alternative considered**: Embedding pack creation in the server startup — rejected because template pack creation is a one-time setup operation, not a per-start concern.

### 5. Tool Registration Pattern

**Decision**: Use a registry pattern where each tool implements a `Tool` interface:

```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() json.RawMessage
    Execute(ctx context.Context, params json.RawMessage) (any, error)
}
```

Tools self-register at init time. The MCP handler iterates the registry for `tools/list` and dispatches `tools/call` by name.

**Rationale**: Adding new tools requires only implementing the interface and calling `Register()` — no changes to protocol code. This is the standard pattern for extensible command systems.

### 6. Task Generation Strategy

**Decision**: Task generation uses a two-pass approach:
1. **Entity pass**: Create one task per Requirement, Scenario, referenced Context, UIComponent, and Action
2. **Dependency pass**: Walk relationships to create `blocks` edges (components block contexts, contexts block actions, actions block scenarios)

**Rationale**: The graph already encodes the relationships between entities. Task generation is essentially a graph transformation — mapping spec relationships to task dependencies. The two-pass approach ensures all tasks exist before dependencies are wired.

**Alternative considered**: LLM-based task generation — rejected for v1 because deterministic graph traversal is more predictable and testable. LLM-assisted task refinement can be added later.

### 7. Sync / Entity Extraction

**Decision**: Implement extraction as a pluggable analyzer system. Each analyzer handles a file type/framework:

```go
type Analyzer interface {
    CanAnalyze(filePath string) bool
    Analyze(filePath string, content []byte) ([]Entity, []Relationship, error)
}
```

Start with a React/TypeScript analyzer. Add more analyzers as needed.

**Rationale**: Different projects use different frameworks. A pluggable system means we can add Go, Python, Swift analyzers without changing the sync infrastructure. The React analyzer covers the immediate use case.

**Alternative considered**: AST-based analysis using Go's parser — only works for Go files. A regex/heuristic approach over file content is more portable and sufficient for v1.

### 8. Configuration Hierarchy

**Decision**: Configuration loads in order: defaults → YAML file (`specmcp.yaml`) → environment variables. Environment variables take precedence.

```
EMERGENT_URL          → emergent.url
EMERGENT_API_KEY      → emergent.api_key
EMERGENT_PROJECT_ID   → emergent.project_id
SPECMCP_LOG_LEVEL     → log.level
```

**Rationale**: Standard 12-factor app configuration. Environment variables allow easy override in different environments without file changes. YAML provides a readable default configuration.

## Risks / Trade-offs

- **[Emergent availability]** The server is useless if Emergent is down. → Mitigation: Clear error messages on connection failure; consider health check at startup with graceful degradation.

- **[Sync accuracy]** Heuristic-based code analysis will miss entities or create false positives. → Mitigation: Pattern confirmation workflow requires human approval. Start with high-confidence extraction rules and iterate.

- **[Large graph performance]** Impact analysis and traversal on large graphs could be slow. → Mitigation: Use Emergent's ExpandGraph with depth limits. Add caching for frequently-queried subgraphs if needed.

- **[Single framework support]** Only React/TypeScript extraction in v1. → Mitigation: Analyzer plugin system makes adding frameworks straightforward. Prioritize based on user needs.

- **[No branch isolation]** All changes operate on the same Emergent graph without branch isolation. → Mitigation: Change entities with "active"/"archived" status provide logical isolation. Emergent branches can be added in v2.

- **[Bulk operation limits]** BulkCreateObjects and BulkCreateRelationships have a 100-item cap per request. → Mitigation: Batch into chunks of 100 during sync and seed operations. Partial-success semantics mean individual failures don't roll back the batch.

## Open Questions

1. ~~**Template pack creation API**: What is the exact Emergent API endpoint for creating a new template pack?~~ → **Resolved**: SDK v0.7.0 provides `TemplatePacks.CreatePack()` via `POST /api/template-packs`.
2. **MCP notifications**: Should the server emit progress notifications during long-running operations (e.g., full sync)? MCP supports notifications but not all clients handle them.
3. **Entity versioning**: Should we use Emergent's built-in versioning (CanonicalID/SupersedesID) for tracking spec changes over time, or is the Change/archive model sufficient?
