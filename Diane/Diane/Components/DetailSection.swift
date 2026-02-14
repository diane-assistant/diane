import SwiftUI

struct DetailSection<Content: View>: View {
    let title: String
    @ViewBuilder let content: Content
    
    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.medium) {
            Text(title)
                .font(.headline)
                .foregroundStyle(.secondary)
            
            VStack(alignment: .leading, spacing: Spacing.small) {
                content
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(Padding.section)
            .background(Colors.controlBackground)
            .cornerRadius(CornerRadius.standard)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}
