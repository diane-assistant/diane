## MODIFIED Requirements

### Requirement: Color tokens are cross-platform
The DesignTokens color definitions SHALL work on both macOS and iOS without conditional compilation where possible.

#### Scenario: Colors resolve on macOS
- **WHEN** a view references a DesignTokens color on macOS
- **THEN** the color resolves to the appropriate macOS system color

#### Scenario: Colors resolve on iOS
- **WHEN** a view references a DesignTokens color on iOS
- **THEN** the color resolves to the appropriate iOS system color

#### Scenario: Cross-platform color API
- **WHEN** a component uses a DesignTokens color
- **THEN** it uses a SwiftUI `Color` value (not `Color(nsColor:)` or `Color(uiColor:)`)
- **AND** platform-specific resolution is handled inside DesignTokens via `#if os(macOS)` / `#if os(iOS)` guards

### Requirement: Spacing and layout tokens are platform-independent
The spacing, padding, corner radius, and layout constants SHALL remain unchanged and work identically on both platforms.

#### Scenario: Spacing values are the same on iOS
- **WHEN** a view uses `Spacing.medium` or `Padding.standard` on iOS
- **THEN** the values are identical to their macOS counterparts (8pt and 16pt respectively)

### Requirement: Components using NSColor are updated
All reusable components that reference `Color(nsColor:)` SHALL be updated to use cross-platform DesignTokens color properties.

#### Scenario: SummaryCard uses cross-platform colors
- **WHEN** SummaryCard is rendered on iOS
- **THEN** it uses DesignTokens color properties instead of `Color(nsColor: .controlBackgroundColor)`

#### Scenario: DetailSection uses cross-platform colors
- **WHEN** DetailSection is rendered on iOS
- **THEN** it uses DesignTokens color properties instead of `Color(nsColor:)` references

#### Scenario: KeyValueEditor uses cross-platform colors
- **WHEN** KeyValueEditor is rendered on iOS
- **THEN** it uses DesignTokens color properties instead of `Color(nsColor:)` references

#### Scenario: StringArrayEditor uses cross-platform colors
- **WHEN** StringArrayEditor is rendered on iOS
- **THEN** it uses DesignTokens color properties instead of `Color(nsColor:)` references
