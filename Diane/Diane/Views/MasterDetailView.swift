import SwiftUI

/// Reusable master-detail split view component
/// Used by AgentsView, ContextsView, and ProvidersView for consistent layout
///
/// Provides a standardized 1/3-2/3 split with:
/// - Master (left): List column with min 250px, ideal 330px (~1/3), max 400px
/// - Detail (right): Detail view with min 400px, ideal 620px (~2/3)
struct MasterDetailView<Master: View, Detail: View>: View {
    let master: Master
    let detail: Detail
    
    init(@ViewBuilder master: () -> Master, @ViewBuilder detail: () -> Detail) {
        self.master = master()
        self.detail = detail()
    }
    
    var body: some View {
        HSplitView {
            master
                .frame(minWidth: 250, idealWidth: 330, maxWidth: 400)
            
            detail
                .frame(minWidth: 400, idealWidth: 620)
        }
    }
}

/// Standard list section header for master column
/// Provides consistent styling across all master-detail views
struct MasterListHeader: View {
    var icon: String? = nil
    let title: String
    var count: Int? = nil
    var trailingIcon: String? = nil
    var trailingTooltip: String? = nil
    var action: (() -> Void)? = nil
    
    var body: some View {
        HStack(spacing: 8) {
            if let icon {
                Image(systemName: icon)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .frame(width: 20, alignment: .center)
            }
            Text(title)
                .font(.subheadline.weight(.semibold))
            if let count {
                Text("\(count)")
                    .font(.caption2.weight(.medium).monospacedDigit())
                    .foregroundStyle(.secondary)
                    .padding(.horizontal, 6)
                    .padding(.vertical, 2)
                    .background(Color(nsColor: .separatorColor).opacity(0.3))
                    .cornerRadius(4)
            }
            Spacer()
            
            if let trailingIcon {
                if let action {
                    Button(action: action) {
                        Image(systemName: trailingIcon)
                            .foregroundStyle(.secondary)
                    }
                    .buttonStyle(.plain)
                    .help(trailingTooltip ?? "")
                } else {
                    Image(systemName: trailingIcon)
                        .foregroundStyle(.secondary)
                        .help(trailingTooltip ?? "")
                }
            }
        }
        .padding(.horizontal)
        .padding(.vertical, 8)
        .background(Color(nsColor: .windowBackgroundColor))
    }
}

#Preview("Master-Detail Layout") {
    MasterDetailView {
        VStack {
            MasterListHeader(icon: "list.bullet", title: "Items")
            List {
                Text("Item 1")
                Text("Item 2")
                Text("Item 3")
            }
        }
    } detail: {
        VStack {
            Text("Detail View")
                .font(.title)
            Spacer()
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(Color.gray.opacity(0.1))
    }
    .frame(width: 1000, height: 700)
}
