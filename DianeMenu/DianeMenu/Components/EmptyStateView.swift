import SwiftUI

/// Reusable empty state placeholder shown when a list or content area has no items.
///
/// Supports several configurations:
/// - Icon + title + description + action button (most common)
/// - Icon + title + description (informational)
/// - Icon + title only (compact, e.g. log panels)
/// - Icon + title + description + custom content (e.g. code examples)
///
/// Usage:
///
///     // With action button
///     EmptyStateView(
///         icon: "server.rack",
///         title: "No MCP servers configured",
///         description: "Add an MCP server to extend Diane's capabilities",
///         actionLabel: "Add Server",
///         action: { viewModel.showCreateServer = true }
///     )
///
///     // With custom content
///     EmptyStateView(
///         icon: "person.3",
///         title: "No agents configured",
///         description: "Use the gallery to add agents:"
///     ) {
///         Text("./scripts/acp-gallery.sh install gemini")
///             .font(.system(.caption, design: .monospaced))
///             .padding(Padding.small)
///             .background(Color(nsColor: .textBackgroundColor))
///             .cornerRadius(CornerRadius.medium)
///     }
struct EmptyStateView<Content: View>: View {
    let icon: String
    let title: String
    var description: String? = nil
    var actionLabel: String? = nil
    var actionIcon: String = "plus"
    var action: (() -> Void)? = nil
    let content: Content?

    /// Creates an empty state with an optional action button (no custom content).
    init(
        icon: String,
        title: String,
        description: String? = nil,
        actionLabel: String? = nil,
        actionIcon: String = "plus",
        action: (() -> Void)? = nil
    ) where Content == EmptyView {
        self.icon = icon
        self.title = title
        self.description = description
        self.actionLabel = actionLabel
        self.actionIcon = actionIcon
        self.action = action
        self.content = nil
    }

    /// Creates an empty state with custom trailing content (e.g. code examples).
    init(
        icon: String,
        title: String,
        description: String? = nil,
        @ViewBuilder content: () -> Content
    ) {
        self.icon = icon
        self.title = title
        self.description = description
        self.actionLabel = nil
        self.actionIcon = "plus"
        self.action = nil
        self.content = content()
    }

    var body: some View {
        VStack(spacing: Spacing.large) {
            Image(systemName: icon)
                .font(.system(size: 32))
                .foregroundStyle(.secondary)

            VStack(spacing: Spacing.xSmall) {
                Text(title)
                    .font(.headline)

                if let description {
                    Text(description)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .multilineTextAlignment(.center)
                        .frame(maxWidth: 300)
                }
            }

            if let content {
                content
            }

            if let actionLabel, let action {
                Button(action: action) {
                    Label(actionLabel, systemImage: actionIcon)
                }
                .buttonStyle(.borderedProminent)
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}
