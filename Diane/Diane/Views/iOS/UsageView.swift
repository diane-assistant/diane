import SwiftUI

/// Usage summary view showing token and cost metrics.
struct UsageView: View {
    @Environment(\.dianeClient) private var client

    @State private var summary: UsageSummaryResponse?
    @State private var isLoading = true
    @State private var selectedRange: UsageRange = .week

    enum UsageRange: String, CaseIterable {
        case day = "24h"
        case week = "7d"
        case month = "30d"
        case all = "All"

        var from: Date? {
            switch self {
            case .day: Calendar.current.date(byAdding: .day, value: -1, to: Date())
            case .week: Calendar.current.date(byAdding: .day, value: -7, to: Date())
            case .month: Calendar.current.date(byAdding: .day, value: -30, to: Date())
            case .all: nil
            }
        }
    }

    var body: some View {
        Group {
            if isLoading && summary == nil {
                ProgressView("Loading usage...")
            } else if let summary = summary {
                List {
                    // Time range picker
                    Section {
                        Picker("Time Range", selection: $selectedRange) {
                            ForEach(UsageRange.allCases, id: \.self) { range in
                                Text(range.rawValue).tag(range)
                            }
                        }
                        .pickerStyle(.segmented)
                        .listRowBackground(Color.clear)
                        .listRowInsets(EdgeInsets())
                        .padding(.horizontal)
                    }

                    // Total cost
                    Section {
                        HStack {
                            Text("Total Cost")
                                .font(.headline)
                            Spacer()
                            Text(summary.formattedTotalCost)
                                .font(.title2.bold())
                        }
                    }

                    // Per-model breakdown
                    if !summary.summary.isEmpty {
                        Section("By Model") {
                            ForEach(summary.summary) { record in
                                UsageSummaryRow(record: record)
                            }
                        }
                    }
                }
            } else {
                EmptyStateView(
                    icon: "chart.bar",
                    title: "No Usage Data",
                    description: "No usage data available for the selected period"
                )
            }
        }
        .navigationTitle("Usage")
        .refreshable { await refresh() }
        .task { await refresh() }
        .onChange(of: selectedRange) { _, _ in
            Task { await refresh() }
        }
    }

    private func refresh() async {
        guard let client else { return }
        summary = try? await client.getUsageSummary(from: selectedRange.from, to: nil)
        isLoading = false
    }
}

// MARK: - Usage Summary Row

private struct UsageSummaryRow: View {
    let record: UsageSummaryRecord

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack {
                Text(record.model)
                    .font(.body)
                Spacer()
                Text(record.formattedCost)
                    .font(.body.bold())
            }

            HStack(spacing: 12) {
                Label(record.providerName, systemImage: "cloud")
                Label("\(record.totalRequests) req", systemImage: "arrow.up.arrow.down")
                Label(record.formattedTokens, systemImage: "text.word.spacing")
            }
            .font(.caption)
            .foregroundStyle(.secondary)

            // Token breakdown
            HStack(spacing: 16) {
                HStack(spacing: 4) {
                    Text("In:")
                        .foregroundStyle(.secondary)
                    Text(formatTokens(record.totalInput))
                }
                HStack(spacing: 4) {
                    Text("Out:")
                        .foregroundStyle(.secondary)
                    Text(formatTokens(record.totalOutput))
                }
                if record.totalCached > 0 {
                    HStack(spacing: 4) {
                        Text("Cached:")
                            .foregroundStyle(.secondary)
                        Text(formatTokens(record.totalCached))
                    }
                }
            }
            .font(.caption2)
        }
        .padding(.vertical, 2)
    }

    private func formatTokens(_ count: Int) -> String {
        if count >= 1_000_000 {
            return String(format: "%.1fM", Double(count) / 1_000_000.0)
        } else if count >= 1_000 {
            return String(format: "%.1fK", Double(count) / 1_000.0)
        }
        return "\(count)"
    }
}
