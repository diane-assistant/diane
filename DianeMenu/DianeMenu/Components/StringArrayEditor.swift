import SwiftUI

struct StringArrayEditor: View {
    @Binding var items: [String]
    let title: String
    let placeholder: String
    
    @State private var newItem = ""
    
    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.medium) {
            Text(title)
                .font(.caption)
                .foregroundStyle(.secondary)
                .padding(.horizontal, Padding.rowItem)
            
            if !items.isEmpty {
                VStack(spacing: Spacing.xSmall) {
                    ForEach(items.indices, id: \.self) { index in
                        HStack(spacing: Spacing.medium) {
                            Text("[\(index)]")
                                .font(.system(.caption, design: .monospaced))
                                .foregroundStyle(.tertiary)
                                .frame(width: 30)
                            
                            Text(items[index])
                                .font(.system(.caption, design: .monospaced))
                            
                            Spacer()
                            
                            Button {
                                items.remove(at: index)
                            } label: {
                                Image(systemName: "minus.circle.fill")
                                    .foregroundStyle(.red)
                            }
                            .buttonStyle(.plain)
                            .frame(width: Layout.iconColumnWidth, alignment: .center)
                        }
                        .padding(Padding.rowItem)
                        .background(Color(nsColor: .controlBackgroundColor))
                        .cornerRadius(CornerRadius.small)
                    }
                }
            }
            
            HStack(spacing: Spacing.medium) {
                Spacer()
                    .frame(width: 30)
                
                TextField(placeholder, text: $newItem)
                    .textFieldStyle(.roundedBorder)
                
                Button {
                    if !newItem.isEmpty {
                        items.append(newItem)
                        newItem = ""
                    }
                } label: {
                    Image(systemName: "plus.circle.fill")
                }
                .buttonStyle(.plain)
                .disabled(newItem.isEmpty)
                .frame(width: Layout.iconColumnWidth, alignment: .center)
            }
            .padding(.horizontal, Padding.rowItem)
            .onSubmit {
                if !newItem.isEmpty {
                    items.append(newItem)
                    newItem = ""
                }
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}
