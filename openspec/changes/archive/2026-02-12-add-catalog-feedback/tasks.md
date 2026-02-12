## 1. GitHubIssueService

- [x] 1.1 Create `GitHubIssueService.swift` in `DianeMenu/ComponentCatalog/` with a `createIssue(selector:component:category:state:comment:repo:)` async method that shells out to `gh issue create` via `Process`
- [x] 1.2 Implement selector string generation: `"{CatalogItem.rawValue} [{preset.name}]"` (omit bracket suffix when no preset)
- [x] 1.3 Implement structured issue body markdown template with Component, Category, State, and Comment sections
- [x] 1.4 Implement issue title generation: `"[Catalog] {selector}: {first line truncated to 60 chars}"`
- [x] 1.5 Include `--label catalog-feedback` in the `gh` invocation
- [x] 1.6 Support optional `-R owner/repo` override via a `repo` parameter
- [x] 1.7 Set `Process.currentDirectoryURL` to the project root directory (derived from bundle path or a sensible default)
- [x] 1.8 Return a result type with the issue URL on success or error message on failure
- [x] 1.9 Add `GitHubIssueService.swift` to the ComponentCatalog target in pbxproj (PBXFileReference + PBXBuildFile)

## 2. Feedback UI in Controls Panel

- [x] 2.1 Add `@State private var feedbackText: [CatalogItem: String]` to CatalogContentView for per-item draft persistence
- [x] 2.2 Add `@State private var isCreatingIssue: Bool` and `@State private var issueResult: IssueCreationResult?` for submission state
- [x] 2.3 Create a `feedbackSection(for:)` ViewBuilder method with a "Feedback" GroupBox containing a TextEditor and "Create Issue" button
- [x] 2.4 Add the feedback section to the controls panel below the existing sections (canvas size / layout controls)
- [x] 2.5 Disable the "Create Issue" button when feedback text is empty or whitespace-only, or when `isCreatingIssue` is true
- [x] 2.6 Show a ProgressView or spinner on the button while `isCreatingIssue` is true

## 3. Issue Creation Wiring

- [x] 3.1 Wire the "Create Issue" button action to call `GitHubIssueService.createIssue()` with the current item, preset, and feedback text
- [x] 3.2 On success: clear the feedback text for that item, set `issueResult` to success with the issue URL
- [x] 3.3 On failure: set `issueResult` to error with the error message
- [x] 3.4 Display success feedback inline (issue URL as a clickable link) that auto-dismisses after a few seconds
- [x] 3.5 Display error feedback inline in red text

## 4. Repository Configuration

- [x] 4.1 Add an optional repo override text field in the feedback section (collapsed by default, expandable via a disclosure or small "Settings" button)
- [x] 4.2 Store the repo override in `@AppStorage` so it persists across launches
- [x] 4.3 Pass the repo override to `GitHubIssueService` when non-empty

## 5. Build Verification

- [x] 5.1 Build ComponentCatalog target — verify clean compilation
- [x] 5.2 Build DianeMenu target — verify no regressions
- [x] 5.3 Run full test suite — verify all tests pass
