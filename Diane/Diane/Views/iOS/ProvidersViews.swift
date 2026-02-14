import SwiftUI

/// List of configured providers.
struct ProvidersListView: View {
    @Environment(\.dianeClient) private var client

    @State private var providers: [Provider] = []
    @State private var searchText = ""
    @State private var isLoading = true

    private var filteredProviders: [Provider] {
        if searchText.isEmpty { return providers }
        return providers.filter { $0.name.localizedCaseInsensitiveContains(searchText) }
    }

    var body: some View {
        Group {
            if isLoading && providers.isEmpty {
                ProgressView("Loading providers...")
            } else if providers.isEmpty {
                EmptyStateView(
                    icon: "cloud",
                    title: "No Providers",
                    description: "No providers are configured"
                )
            } else {
                List(filteredProviders) { provider in
                    NavigationLink {
                        ProviderDetailView(provider: provider)
                    } label: {
                        ProviderRow(provider: provider)
                    }
                }
                .searchable(text: $searchText, prompt: "Search providers")
            }
        }
        .navigationTitle("Providers")
        .refreshable { await refresh() }
        .task { await refresh() }
    }

    private func refresh() async {
        guard let client else { return }
        providers = (try? await client.getProviders(type: nil)) ?? []
        isLoading = false
    }
}

// MARK: - Provider Row

private struct ProviderRow: View {
    let provider: Provider

    var body: some View {
        HStack {
            Image(systemName: provider.type.icon)
                .foregroundStyle(.secondary)
                .frame(width: 24)

            VStack(alignment: .leading, spacing: 2) {
                Text(provider.name)
                    .font(.body)
                HStack(spacing: 6) {
                    Text(provider.serviceName)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    Text(provider.type.displayName)
                        .font(.caption2)
                        .padding(.horizontal, 6)
                        .padding(.vertical, 1)
                        .background(Color.secondary.opacity(0.1))
                        .clipShape(Capsule())
                }
            }

            Spacer()

            if provider.isDefault {
                Image(systemName: "star.fill")
                    .font(.caption)
                    .foregroundStyle(.yellow)
            }

            if provider.enabled {
                Image(systemName: "circle.fill")
                    .font(.caption2)
                    .foregroundStyle(.green)
            } else {
                Image(systemName: "circle.slash")
                    .font(.caption2)
                    .foregroundStyle(.secondary)
            }
        }
    }
}

// MARK: - Provider Detail

struct ProviderDetailView: View {
    let provider: Provider

    var body: some View {
        List {
            DetailSection(title: "Overview") {
                InfoRow(label: "Name", value: provider.name)
                InfoRow(label: "Service", value: provider.serviceName)
                InfoRow(label: "Type", value: provider.type.displayName)
                InfoRow(label: "Enabled", value: provider.enabled ? "Yes" : "No")
                InfoRow(label: "Default", value: provider.isDefault ? "Yes" : "No")
                InfoRow(label: "Auth Type", value: provider.authType.displayName)
            }

            if !provider.config.isEmpty {
                DetailSection(title: "Configuration") {
                    ForEach(provider.config.sorted(by: { $0.key < $1.key }), id: \.key) { key, value in
                        InfoRow(label: key, value: describeAnyCodable(value))
                    }
                }
            }

            DetailSection(title: "Timestamps") {
                InfoRow(label: "Created", value: provider.createdAt.formatted())
                InfoRow(label: "Updated", value: provider.updatedAt.formatted())
            }
        }
        .navigationTitle(provider.name)
    }

    private func describeAnyCodable(_ value: AnyCodable) -> String {
        if let str = value.value as? String { return str }
        if let num = value.value as? Int { return "\(num)" }
        if let num = value.value as? Double { return String(format: "%.2f", num) }
        if let bool = value.value as? Bool { return bool ? "true" : "false" }
        return "\(value.value)"
    }
}
