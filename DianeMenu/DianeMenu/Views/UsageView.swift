import SwiftUI

struct UsageView: View {
    @State private var usageSummary: UsageSummaryResponse?
    @State private var recentUsage: UsageResponse?
    @State private var isLoading = true
    @State private var error: String?
    @State private var selectedTimeRange: TimeRange = .month
    
    private let client = DianeClient()
    
    enum TimeRange: String, CaseIterable {
        case day = "24 Hours"
        case week = "7 Days"
        case month = "30 Days"
        case year = "1 Year"
        
        var from: Date {
            let now = Date()
            switch self {
            case .day: return Calendar.current.date(byAdding: .day, value: -1, to: now) ?? now
            case .week: return Calendar.current.date(byAdding: .day, value: -7, to: now) ?? now
            case .month: return Calendar.current.date(byAdding: .month, value: -1, to: now) ?? now
            case .year: return Calendar.current.date(byAdding: .year, value: -1, to: now) ?? now
            }
        }
    }
    
    var body: some View {
        VStack(spacing: 0) {
            headerView
            
            Divider()
            
            if isLoading {
                loadingView
            } else if let error = error {
                errorView(error)
            } else {
                ScrollView {
                    VStack(alignment: .leading, spacing: 20) {
                        summaryCardsView
                        
                        if let summary = usageSummary, !summary.summary.isEmpty {
                            usageBreakdownView(summary)
                        }
                        
                        if let recent = recentUsage, !recent.records.isEmpty {
                            recentActivityView(recent)
                        }
                    }
                    .padding()
                }
            }
        }
        .frame(minWidth: 600, idealWidth: 700, maxWidth: .infinity,
               minHeight: 400, idealHeight: 500, maxHeight: .infinity)
        .task {
            await loadData()
        }
        .onChange(of: selectedTimeRange) { _, _ in
            Task { await loadData() }
        }
    }
    
    // MARK: - Header
    
    private var headerView: some View {
        HStack(spacing: 12) {
            Image(systemName: "chart.bar.doc.horizontal")
                .foregroundStyle(.secondary)
            
            Text("Usage")
                .font(.headline)
            
            Spacer()
            
            Picker("Time Range", selection: $selectedTimeRange) {
                ForEach(TimeRange.allCases, id: \.self) { range in
                    Text(range.rawValue).tag(range)
                }
            }
            .pickerStyle(.segmented)
            .frame(width: 300)
            
            Button {
                Task { await loadData() }
            } label: {
                Image(systemName: "arrow.clockwise")
            }
            .disabled(isLoading)
        }
        .padding()
    }
    
    // MARK: - Loading View
    
    private var loadingView: some View {
        VStack(spacing: 12) {
            ProgressView()
            Text("Loading usage data...")
                .font(.caption)
                .foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
    
    // MARK: - Error View
    
    private func errorView(_ message: String) -> some View {
        VStack(spacing: 12) {
            Image(systemName: "exclamationmark.triangle.fill")
                .font(.largeTitle)
                .foregroundStyle(.orange)
            Text("Failed to load usage data")
                .font(.headline)
            Text(message)
                .font(.caption)
                .foregroundStyle(.secondary)
            Button("Retry") {
                Task { await loadData() }
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
    
    // MARK: - Summary Cards
    
    private var summaryCardsView: some View {
        HStack(spacing: 16) {
            SummaryCard(
                title: "Total Cost",
                value: usageSummary?.formattedTotalCost ?? "$0.00",
                icon: "dollarsign.circle",
                color: .green
            )
            
            SummaryCard(
                title: "Total Requests",
                value: "\(usageSummary?.summary.reduce(0) { $0 + $1.totalRequests } ?? 0)",
                icon: "arrow.up.arrow.down",
                color: .blue
            )
            
            SummaryCard(
                title: "Total Tokens",
                value: formatTokens(usageSummary?.summary.reduce(0) { $0 + $1.totalTokens } ?? 0),
                icon: "text.word.spacing",
                color: .purple
            )
            
            SummaryCard(
                title: "Providers",
                value: "\(Set(usageSummary?.summary.map { $0.providerID } ?? []).count)",
                icon: "cpu",
                color: .orange
            )
        }
    }
    
    // MARK: - Usage Breakdown
    
    private func usageBreakdownView(_ summary: UsageSummaryResponse) -> some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Usage by Model")
                .font(.headline)
            
            VStack(spacing: 8) {
                ForEach(summary.summary.sorted { $0.totalCost > $1.totalCost }) { record in
                    UsageRowView(record: record, totalCost: summary.totalCost)
                }
            }
            .padding()
            .background(Color(nsColor: .controlBackgroundColor))
            .cornerRadius(8)
        }
    }
    
    // MARK: - Recent Activity
    
    private func recentActivityView(_ recent: UsageResponse) -> some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                Text("Recent Activity")
                    .font(.headline)
                Spacer()
                Text("\(recent.records.count) records")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            
            VStack(spacing: 0) {
                // Header row
                HStack {
                    Text("Time")
                        .frame(width: 100, alignment: .leading)
                    Text("Provider")
                        .frame(width: 100, alignment: .leading)
                    Text("Model")
                        .frame(minWidth: 120, alignment: .leading)
                    Spacer()
                    Text("Tokens")
                        .frame(width: 80, alignment: .trailing)
                    Text("Cost")
                        .frame(width: 70, alignment: .trailing)
                }
                .font(.caption.weight(.semibold))
                .foregroundStyle(.secondary)
                .padding(.horizontal, 12)
                .padding(.vertical, 8)
                
                Divider()
                
                ForEach(recent.records.prefix(20)) { record in
                    HStack {
                        Text(record.createdAt, style: .time)
                            .frame(width: 100, alignment: .leading)
                        Text(record.providerName)
                            .frame(width: 100, alignment: .leading)
                            .lineLimit(1)
                        Text(record.model)
                            .frame(minWidth: 120, alignment: .leading)
                            .lineLimit(1)
                        Spacer()
                        Text(formatTokens(record.totalTokens))
                            .frame(width: 80, alignment: .trailing)
                        Text(record.formattedCost)
                            .frame(width: 70, alignment: .trailing)
                            .foregroundStyle(record.cost > 0 ? .primary : .secondary)
                    }
                    .font(.caption)
                    .padding(.horizontal, 12)
                    .padding(.vertical, 6)
                    
                    if record.id != recent.records.prefix(20).last?.id {
                        Divider()
                    }
                }
            }
            .background(Color(nsColor: .controlBackgroundColor))
            .cornerRadius(8)
        }
    }
    
    // MARK: - Helpers
    
    private func formatTokens(_ count: Int) -> String {
        if count >= 1_000_000 {
            return String(format: "%.1fM", Double(count) / 1_000_000.0)
        } else if count >= 1_000 {
            return String(format: "%.1fK", Double(count) / 1_000.0)
        }
        return "\(count)"
    }
    
    private func loadData() async {
        isLoading = true
        error = nil
        
        do {
            async let summaryTask = client.getUsageSummary(from: selectedTimeRange.from)
            async let recentTask = client.getUsage(from: selectedTimeRange.from, limit: 50)
            
            let (loadedSummary, loadedRecent) = try await (summaryTask, recentTask)
            usageSummary = loadedSummary
            recentUsage = loadedRecent
        } catch {
            self.error = error.localizedDescription
        }
        
        isLoading = false
    }
}

// MARK: - Summary Card

struct SummaryCard: View {
    let title: String
    let value: String
    let icon: String
    let color: Color
    
    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Image(systemName: icon)
                    .foregroundStyle(color)
                Text(title)
                    .foregroundStyle(.secondary)
            }
            .font(.caption)
            
            Text(value)
                .font(.title2.weight(.semibold))
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding()
        .background(Color(nsColor: .controlBackgroundColor))
        .cornerRadius(8)
    }
}

// MARK: - Usage Row View

struct UsageRowView: View {
    let record: UsageSummaryRecord
    let totalCost: Double
    
    private var percentage: Double {
        guard totalCost > 0 else { return 0 }
        return record.totalCost / totalCost
    }
    
    var body: some View {
        VStack(spacing: 6) {
            HStack {
                VStack(alignment: .leading, spacing: 2) {
                    Text(record.model)
                        .font(.subheadline.weight(.medium))
                    HStack(spacing: 8) {
                        Text(record.providerName)
                        Text("\(record.totalRequests) requests")
                        Text(record.formattedTokens + " tokens")
                    }
                    .font(.caption)
                    .foregroundStyle(.secondary)
                }
                
                Spacer()
                
                Text(record.formattedCost)
                    .font(.subheadline.weight(.semibold))
                    .foregroundStyle(record.totalCost > 0 ? .primary : .secondary)
            }
            
            // Cost bar
            GeometryReader { geometry in
                ZStack(alignment: .leading) {
                    Rectangle()
                        .fill(Color.secondary.opacity(0.1))
                        .frame(height: 4)
                    
                    Rectangle()
                        .fill(Color.blue)
                        .frame(width: geometry.size.width * percentage, height: 4)
                }
                .cornerRadius(2)
            }
            .frame(height: 4)
        }
    }
}

#Preview {
    UsageView()
}
