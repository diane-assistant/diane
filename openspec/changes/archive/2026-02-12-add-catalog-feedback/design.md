## Context

The ComponentCatalog is a standalone macOS app target that renders every DianeMenu view and component with mock data, preset states, and interactive layout controls. It already has a three-column layout: sidebar (item list), center (preview canvas), and trailing (controls panel with preset selector and layout sliders).

The goal is to add a feedback flow so that while reviewing components in the catalog, the user can write a comment and create a GitHub issue directly — with the component selector and state context automatically included. This eliminates the context-switch of manually filing issues.

The `gh` CLI is the integration path. It's already installed and authenticated on the developer's machine. The ComponentCatalog app will shell out to `gh issue create` via `Process`.

## Goals / Non-Goals

**Goals:**
- Add a feedback text field and "Create Issue" button to the controls panel
- Auto-generate a selector string from the current CatalogItem + preset
- Create structured GitHub issues via `gh issue create` with component context
- Provide inline success/error feedback

**Non-Goals:**
- Screenshot capture (deferred to a future change)
- Sub-component element selection (feedback is at the CatalogItem level)
- GitHub OAuth or token management (relies on `gh` CLI being pre-authed)
- Editing or listing existing issues from within the catalog

## Decisions

### 1. Shell out to `gh` via `Process` (not URLSession + GitHub API)

The user already has `gh` installed and authenticated. Using `Process` to run `gh issue create` avoids managing tokens, OAuth flows, or API versioning. The catalog is a developer tool, not a user-facing product — relying on `gh` is appropriate.

**Alternative considered**: Direct GitHub REST API via URLSession. Rejected because it requires token storage/management and adds complexity for no practical benefit in this context.

### 2. Separate `GitHubIssueService` class (not inline in the view)

The `gh` shell-out logic, selector formatting, and issue body templating will live in a dedicated `GitHubIssueService` class in the ComponentCatalog directory. This keeps CatalogContentView focused on layout and state, and makes the service testable independently.

**Alternative considered**: Inline the `Process` call in the view's button action. Rejected because it mixes concerns and makes the logic harder to test or reuse.

### 3. Per-item feedback text stored in a dictionary

Feedback text will be stored as `@State private var feedbackText: [CatalogItem: String]` in CatalogContentView, mirroring the existing `selectedPresets` dictionary pattern. This preserves draft comments when switching between items.

**Alternative considered**: Single `@State var feedbackText: String` that resets on item switch. Rejected because losing in-progress feedback when browsing is frustrating.

### 4. Issue body as structured markdown

The issue body will use a consistent markdown template:

```markdown
## Catalog Feedback: {selector}

**Component:** {CatalogItem.rawValue}
**Category:** {category}
**State:** {preset name or "Default"}

---

{user comment}
```

This gives both humans and LLMs clear, parseable context. The title will be `[Catalog] {selector}: {first line of comment truncated to 60 chars}`.

### 5. Working directory for `gh` set to the project root

`Process.currentDirectoryURL` will be set to the project's root directory so `gh` can auto-detect the repository. The project path can be derived at build time or hardcoded as a reasonable default. An optional repo override field allows pointing at a different repository.

### 6. Label issues with `catalog-feedback`

All issues created from the catalog will include `--label catalog-feedback`. If the label doesn't exist in the repo, `gh` will still create the issue (just without the label) — no pre-configuration required.

## Risks / Trade-offs

- **`gh` not installed or not authed** — The service will check for `gh` availability at first use and surface a clear error. No silent failure.
- **`Process` and sandboxing** — The ComponentCatalog target does NOT use App Sandbox (it's a developer tool with ad-hoc signing). `Process` calls will work without entitlement issues.
- **Working directory detection** — If the app is launched from Finder rather than Xcode, the working directory won't be the project root. Mitigation: use `Bundle.main.bundleURL` to walk up to the project root, or allow the user to set the repo explicitly via `-R owner/repo`.
- **Rate limiting** — GitHub API rate limits apply to `gh` too, but for a manual feedback workflow this is not a practical concern.
