# Capability: Shared Components Extraction

## Purpose

Ensure general-purpose UI components are extracted from domain-specific view files into standalone files under `Components/`, accessible to both the main Diane target and the ComponentCatalog target.

## Requirements

### Requirement: Reusable components extracted into shared files
General-purpose UI components currently defined inside domain-specific view files SHALL be extracted into separate files under a `Components/` directory, accessible to both the main Diane target and the ComponentCatalog target.

#### Scenario: DetailSection extracted
- **WHEN** the project is built
- **THEN** `DetailSection` is defined in `Components/DetailSection.swift` instead of inside `MCPServersView.swift`

#### Scenario: InfoRow extracted
- **WHEN** the project is built
- **THEN** `InfoRow` is defined in `Components/InfoRow.swift` instead of inside `MCPServersView.swift`

#### Scenario: StringArrayEditor extracted
- **WHEN** the project is built
- **THEN** `StringArrayEditor` is defined in `Components/StringArrayEditor.swift` instead of inside `MCPServersView.swift`

#### Scenario: KeyValueEditor extracted
- **WHEN** the project is built
- **THEN** `KeyValueEditor` is defined in `Components/KeyValueEditor.swift` instead of inside `MCPServersView.swift`

#### Scenario: SummaryCard extracted
- **WHEN** the project is built
- **THEN** `SummaryCard` is defined in `Components/SummaryCard.swift` instead of inside `UsageView.swift`

#### Scenario: OAuthConfigEditor extracted
- **WHEN** the project is built
- **THEN** `OAuthConfigEditor` is defined in `Components/OAuthConfigEditor.swift` instead of inside `MCPServersView.swift`

### Requirement: Extraction preserves existing behavior
Extracting components into separate files SHALL NOT change their API, appearance, or behavior. All existing call sites SHALL compile without modification.

#### Scenario: MCPServersView compiles unchanged
- **WHEN** DetailSection, InfoRow, StringArrayEditor, KeyValueEditor, and OAuthConfigEditor are moved out of MCPServersView.swift
- **THEN** MCPServersView.swift compiles without changes because the extracted types remain in the same module

#### Scenario: UsageView compiles unchanged
- **WHEN** SummaryCard is moved out of UsageView.swift
- **THEN** UsageView.swift compiles without changes because SummaryCard remains in the same module

#### Scenario: Visual output unchanged
- **WHEN** the main Diane app is built and run after extraction
- **THEN** all views render identically to before the extraction

### Requirement: Both targets can access shared components
The extracted component files SHALL be included in the build sources of both the Diane main target and the ComponentCatalog target.

#### Scenario: Main target includes components
- **WHEN** the Diane target is built
- **THEN** all extracted component files are compiled as part of the target

#### Scenario: Catalog target includes components
- **WHEN** the ComponentCatalog target is built
- **THEN** all extracted component files are compiled as part of the target

### Requirement: MasterDetailView and MasterListHeader remain in place
MasterDetailView and MasterListHeader, which are already in their own file (`MasterDetailView.swift`), SHALL NOT be moved — they are already properly extracted.

#### Scenario: No redundant extraction
- **WHEN** the shared components extraction is complete
- **THEN** `MasterDetailView.swift` remains at its current location and is not duplicated
