import SwiftUI

// MARK: - Design Tokens
//
// Centralized layout constants for the DianeMenu app.
// All components and views should use these tokens instead of magic numbers.
//
// Based on an 8pt grid system aligned with Apple HIG for macOS.
//
// ## Principles
//
// 1. Components are responsive by default -- they take available space
//    unless they are a specific fixed-size design element (badge, icon button).
//
// 2. Components own their INNER padding. Parents own OUTER spacing.
//    A card adds padding inside its border. The parent VStack/HStack controls
//    the gap between cards.
//
// 3. Modifier order: .padding() -> .background() -> .cornerRadius() -> .overlay()
//    Padding goes first so the background includes the padded area.
//
// 4. Always specify explicit spacing on stacks. Never rely on default (nil)
//    spacing -- it varies by child view type and is unpredictable.

// MARK: - Spacing

/// Inter-element spacing values for stacks and layout gaps.
enum Spacing {
    /// 2pt -- Label + subtitle pairs within a single row.
    static let xxSmall: CGFloat = 2

    /// 4pt -- Tightly grouped elements: icon + text in small badges, related indicators.
    static let xSmall: CGFloat = 4

    /// 6pt -- Compact content groups: items within a detail section, search field internals.
    static let small: CGFloat = 6

    /// 8pt -- Standard component-internal spacing: content within cards, editors, small groups.
    static let medium: CGFloat = 8

    /// 12pt -- Section-internal spacing: items within a loading/error/empty state, form groups.
    static let large: CGFloat = 12

    /// 16pt -- Form fields, content groups within sheets and panels.
    static let xLarge: CGFloat = 16

    /// 20pt -- Top-level scroll content sections, major content areas.
    static let xxLarge: CGFloat = 20
}

// MARK: - Padding

/// Padding values for containers and content areas.
enum Padding {
    /// 6pt -- Compact row items: editor rows (StringArrayEditor, KeyValueEditor items).
    static let rowItem: CGFloat = 6

    /// 8pt -- Small insets: search fields, code blocks, inline controls.
    static let small: CGFloat = 8

    /// 12pt -- Section content: inside DetailSection, grouped content areas.
    static let section: CGFloat = 12

    /// 16pt -- Standard container padding: cards, panels, sheet content, column margins.
    /// This is the default when a component sits inside a column or container.
    static let standard: CGFloat = 16

    /// 20pt -- Generous content areas: detail view scroll content, spacious layouts.
    static let large: CGFloat = 20
}

// MARK: - Corner Radius

/// Corner radius values for backgrounds and borders.
enum CornerRadius {
    /// 4pt -- Small elements: badges, inline tags, editor row items.
    static let small: CGFloat = 4

    /// 6pt -- Medium elements: code blocks, search fields, menu items.
    static let medium: CGFloat = 6

    /// 8pt -- Standard containers: cards, sections, sheets, panels.
    static let standard: CGFloat = 8
}

// MARK: - Layout

/// Standard layout dimensions for common patterns.
enum Layout {
    /// Standard icon button/action column width (for alignment across rows).
    static let iconColumnWidth: CGFloat = 20

    /// Standard label column width for InfoRow and similar label-value pairs.
    static let labelColumnWidth: CGFloat = 80

    /// Standard key column max width for KeyValueEditor and similar.
    static let keyColumnMaxWidth: CGFloat = 120

    /// Fixed width for the menu bar popover.
    static let menuBarWidth: CGFloat = 280
}

// MARK: - List Rows

/// Standard padding for list rows across master-detail views.
enum ListRow {
    /// Horizontal padding for list rows.
    static let horizontalPadding: CGFloat = 16

    /// Vertical padding for list rows.
    static let verticalPadding: CGFloat = 10

    /// Spacing between elements within a row HStack.
    static let contentSpacing: CGFloat = 12

    /// Leading padding for row dividers (indented from the row edge).
    static let dividerLeadingPadding: CGFloat = 16
}

// MARK: - Badges

/// Standard badge styling constants.
enum Badge {
    static let horizontalPadding: CGFloat = 6
    static let verticalPadding: CGFloat = 2
    static let cornerRadius: CGFloat = 4
    static let backgroundOpacity: Double = 0.15
}

// MARK: - View Modifier: Badge Style

/// Applies consistent badge styling. Usage:
///
///     Text("Running")
///         .badgeStyle(color: .green)
///
struct BadgeStyle: ViewModifier {
    let color: Color

    func body(content: Content) -> some View {
        content
            .font(.caption2)
            .foregroundStyle(color)
            .padding(.horizontal, Badge.horizontalPadding)
            .padding(.vertical, Badge.verticalPadding)
            .background(color.opacity(Badge.backgroundOpacity))
            .cornerRadius(Badge.cornerRadius)
    }
}

extension View {
    func badgeStyle(color: Color) -> some View {
        modifier(BadgeStyle(color: color))
    }
}
