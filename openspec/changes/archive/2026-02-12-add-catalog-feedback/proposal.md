## Why

The ComponentCatalog app currently lets you browse and preview every view and component, but there's no way to capture feedback while reviewing. When you spot something that needs changing, you have to context-switch to GitHub, manually describe which component and state you were looking at, and hope the description is unambiguous. Adding a feedback-to-issue flow directly in the catalog eliminates that friction — write a comment, hit a button, and a structured GitHub issue is created with the component selector, preset state, and your comment already filled in.

## What Changes

- Add a "Feedback" section to the controls panel in `CatalogContentView` with a text editor for writing comments and a "Create Issue" button
- Generate a structured selector string from the current `CatalogItem` and active preset (e.g., `AgentsView [Loaded]`)
- Shell out to `gh issue create` to create a GitHub issue with a structured body containing the selector, component metadata, and user comment
- Show success/error feedback inline after issue creation
- Allow configuring the target GitHub repo (default to current repo)

## Capabilities

### New Capabilities
- `catalog-feedback`: Comment text field, selector generation, and `gh issue create` integration in the ComponentCatalog controls panel

### Modified Capabilities

## Impact

- `Diane/ComponentCatalog/CatalogContentView.swift` — new feedback section in controls panel
- New file(s) for the feedback/issue creation logic (keeping it out of the view)
- ComponentCatalog target only — no changes to the Diane target
- Runtime dependency on `gh` CLI being installed and authenticated
