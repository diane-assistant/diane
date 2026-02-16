import SwiftUI

struct UsageView: View {
    @EnvironmentObject var statusMonitor: StatusMonitor
    @State private var viewModel: UsageViewModel
    @State private var clientInitialized = false

    init(viewModel: UsageViewModel = UsageViewModel()) {
        _viewModel = State(initialValue: viewModel)
    }

    var body: some View {
        VStack(spacing: 0) {
            headerView
            
            Divider()
            
            if viewModel.isLoading {
                loadingView
            } else if let error = viewModel.error {
                errorView(error)
            } else {
                ScrollView {
                    VStack(alignment: .leading, spacing: 20) {
                        summaryCardsView
                        
                        if let summary = viewModel.usageSummary, !summary.summary.isEmpty {
                            usageBreakdownView(summary)
                        }
                        
                        if let recent = viewModel.recentUsage, !recent.records.isEmpty {
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
            // Initialize with the correct client from StatusMonitor if available
            if !clientInitialized, let configuredClient = statusMonitor.configuredClient {
                viewModel = UsageViewModel(client: configuredClient)
                clientInitialized = true
            }
            await viewModel.loadData()
        }
        .onChange(of: viewModel.selectedTimeRange) { _, _ in
            Task { await viewModel.loadData() }
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
            
            TimeRangePicker(selection: $viewModel.selectedTimeRange)
                .frame(width: 300)
            
            Button {
                Task { await viewModel.loadData() }
            } label: {
                Image(systemName: "arrow.clockwise")
            }
            .disabled(viewModel.isLoading)
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
                Task { await viewModel.loadData() }
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
    
    // MARK: - Summary Cards
    
    private var summaryCardsView: some View {
        HStack(spacing: 16) {
            SummaryCard(
                title: "Total Cost",
                value: viewModel.usageSummary?.formattedTotalCost ?? "$0.00",
                icon: "dollarsign.circle",
                color: .green
            )
            
            SummaryCard(
                title: "Total Requests",
                value: "\(viewModel.totalRequests)",
                icon: "arrow.up.arrow.down",
                color: .blue
            )
            
            SummaryCard(
                title: "Total Tokens",
                value: UsageViewModel.formatTokens(viewModel.totalTokens),
                icon: "text.word.spacing",
                color: .purple
            )
            
            SummaryCard(
                title: "Providers",
                value: "\(viewModel.providerCount)",
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
                        Text(UsageViewModel.formatTokens(record.totalTokens))
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
