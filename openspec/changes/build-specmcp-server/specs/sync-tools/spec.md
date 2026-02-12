## ADDED Requirements

### Requirement: Sync status check
The system SHALL provide a `spec_sync_status` tool that compares the graph's last synced git commit against the current HEAD to determine if the graph is up to date.

#### Scenario: Graph is in sync
- **WHEN** `spec_sync_status` is called and the GraphSync entity's `last_synced_commit` matches the current git HEAD
- **THEN** the response shows status "synced" with the commit hash and sync timestamp

#### Scenario: Graph is stale
- **WHEN** `spec_sync_status` is called and the GraphSync entity's `last_synced_commit` is behind git HEAD
- **THEN** the response shows status "stale" with the number of commits behind and suggests running `spec_sync`

### Requirement: Full codebase sync
The system SHALL provide a `spec_sync` tool that analyzes the codebase and updates the graph with extracted entities and relationships. The sync SHALL:
1. Detect Contexts (screens, modals, panels) from source files
2. Detect UIComponents (reusable components) from source files
3. Detect Actions (handlers, API calls) from source files
4. Create or update corresponding graph entities
5. Create relationships between extracted entities
6. Update the GraphSync entity with the current commit

#### Scenario: Initial sync of codebase
- **WHEN** `spec_sync` is called on a codebase with no previous sync
- **THEN** all detectable entities are created in the graph, relationships are established, and GraphSync records the current git commit

#### Scenario: Sync creates entity relationships
- **WHEN** `spec_sync` detects a Context that imports and uses a UIComponent
- **THEN** a `uses_component` relationship is created between the Context and UIComponent entities

### Requirement: Context extraction
The system SHALL extract Context entities from source files by detecting screen/page/modal/panel components based on file naming conventions, directory structure, and component patterns (e.g., files in `pages/`, `screens/`, or components with route definitions).

#### Scenario: Extract screen from pages directory
- **WHEN** `spec_sync` analyzes a file at `src/pages/UserManagement.tsx` that exports a React component
- **THEN** a Context entity is created with name "user-management", type "screen", and file_path pointing to the source file

#### Scenario: Extract modal from component patterns
- **WHEN** `spec_sync` analyzes a file that renders a Modal/Dialog wrapper component
- **THEN** a Context entity is created with type "modal"

### Requirement: Component extraction
The system SHALL extract UIComponent entities from source files by detecting reusable component definitions. Component composition SHALL be detected from import relationships.

#### Scenario: Extract component with composition
- **WHEN** `spec_sync` analyzes a component "UserList" that imports and renders "UserCard" and "Pagination"
- **THEN** a UIComponent entity is created for "UserList" with `composed_of` relationships to "UserCard" and "Pagination"

### Requirement: Action extraction
The system SHALL extract Action entities from source files by detecting event handlers, API calls, navigation functions, and mutation triggers.

#### Scenario: Extract mutation action
- **WHEN** `spec_sync` analyzes a handler function `handleUpdateRole` that makes a PUT API call
- **THEN** an Action entity is created with name "update-role", type "mutation", and handler_path pointing to the function location

### Requirement: Git commit tracking
The system SHALL track which git commit the graph was last synced with, enabling detection of changes since last sync.

#### Scenario: Track sync commit
- **WHEN** `spec_sync` completes successfully at commit `abc123`
- **THEN** the GraphSync entity's `last_synced_commit` is set to `abc123` and `last_synced_at` is set to current timestamp

### Requirement: Pattern auto-detection during sync
During sync, the system SHALL analyze extracted entities for common implementation patterns and create suggested `uses_pattern` relationships. Suggested patterns MUST be marked as unconfirmed and require explicit user confirmation before being finalized (see pattern-tools confirmation workflow).

#### Scenario: Detect pattern during sync
- **WHEN** `spec_sync` extracts a Context that contains a list view with filters and pagination
- **THEN** the system suggests the "master-detail" layout pattern for that Context, marked as unconfirmed

### Requirement: Auto-tagging during sync
During sync, the system SHALL automatically apply tags to extracted entities based on file path conventions and component characteristics. Tags SHALL follow the namespaced convention: `domain:<value>`, `platform:<value>`, `lifecycle:<value>`.

#### Scenario: Tag entity by file path
- **WHEN** `spec_sync` extracts a Context from `src/pages/auth/LoginPage.tsx`
- **THEN** the Context is tagged with `domain:auth` and `platform:web`

#### Scenario: Tag entity by framework
- **WHEN** `spec_sync` extracts a UIComponent from a React Native project file
- **THEN** the component is tagged with `platform:mobile`

### Requirement: Incremental sync
The system SHALL support incremental sync that only processes files changed since the last synced commit, using `git diff` to detect modified files.

#### Scenario: Incremental sync processes only changed files
- **WHEN** `spec_sync` is called and the graph was last synced at commit A, current HEAD is commit C with 5 files changed
- **THEN** only the 5 changed files are analyzed and their entities updated, rather than re-analyzing the entire codebase

#### Scenario: Incremental sync detects deleted entities
- **WHEN** a file that previously contained a UIComponent is deleted between syncs
- **THEN** the incremental sync marks the corresponding UIComponent entity as removed or updates its status
