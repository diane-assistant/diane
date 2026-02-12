import SwiftUI

struct SummaryCard: View {
    let title: String
    let value: String
    let icon: String
    let color: Color
    
    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.medium) {
            HStack(spacing: Spacing.xSmall) {
                Image(systemName: icon)
                    .foregroundStyle(color)
                Text(title)
                    .foregroundStyle(.secondary)
                    .lineLimit(1)
                    .fixedSize(horizontal: true, vertical: false)
            }
            .font(.caption)
            
            Text(value)
                .font(.title2.weight(.semibold))
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(Padding.standard)
        .background(Color(nsColor: .controlBackgroundColor))
        .cornerRadius(CornerRadius.standard)
    }
}
