import SwiftUI

struct KeyValueEditor: View {
    @Binding var items: [String: String]
    let title: String
    let keyPlaceholder: String
    let valuePlaceholder: String
    
    @State private var newKey = ""
    @State private var newValue = ""
    
    var sortedKeys: [String] {
        items.keys.sorted()
    }
    
    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.medium) {
            Text(title)
                .font(.caption)
                .foregroundStyle(.secondary)
                .padding(.horizontal, Padding.rowItem)
            
            if !items.isEmpty {
                VStack(spacing: Spacing.xSmall) {
                    ForEach(sortedKeys, id: \.self) { key in
                        HStack(spacing: Spacing.medium) {
                            Text(key)
                                .font(.system(.caption, design: .monospaced))
                                .foregroundStyle(.secondary)
                                .frame(maxWidth: Layout.keyColumnMaxWidth, alignment: .leading)
                            
                            Text("=")
                                .foregroundStyle(.tertiary)
                                .frame(width: 12, alignment: .center)
                            
                            Text(items[key] ?? "")
                                .font(.system(.caption, design: .monospaced))
                                .lineLimit(1)
                            
                            Spacer()
                            
                            Button {
                                items.removeValue(forKey: key)
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
                TextField(keyPlaceholder, text: $newKey)
                    .textFieldStyle(.roundedBorder)
                    .frame(maxWidth: Layout.keyColumnMaxWidth)
                
                Text("=")
                    .foregroundStyle(.tertiary)
                    .frame(width: 12, alignment: .center)
                
                TextField(valuePlaceholder, text: $newValue)
                    .textFieldStyle(.roundedBorder)
                
                Button {
                    if !newKey.isEmpty {
                        items[newKey] = newValue.isEmpty ? "" : newValue
                        newKey = ""
                        newValue = ""
                    }
                } label: {
                    Image(systemName: "plus.circle.fill")
                }
                .buttonStyle(.plain)
                .disabled(newKey.isEmpty)
                .frame(width: Layout.iconColumnWidth, alignment: .center)
            }
            .padding(.horizontal, Padding.rowItem)
            .onSubmit {
                if !newKey.isEmpty {
                    items[newKey] = newValue.isEmpty ? "" : newValue
                    newKey = ""
                    newValue = ""
                }
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}
