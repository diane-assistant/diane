import SwiftUI

struct InfoRow: View {
    let label: String
    let value: String
    
    var body: some View {
        HStack(spacing: Spacing.medium) {
            Text(label)
                .font(.caption)
                .foregroundStyle(.secondary)
                .frame(width: Layout.labelColumnWidth, alignment: .leading)
            Text(value)
                .font(.caption)
                .textSelection(.enabled)
            Spacer()
        }
    }
}
