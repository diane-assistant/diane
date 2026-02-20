import Foundation
import os.log

private let logger = Logger(subsystem: "com.diane.Diane", category: "Usage")

/// Drives the Usage tab — loads summary and recent usage data, manages
/// time‐range selection and loading/error state.
@MainActor @Observable
final class UsageViewModel {

    // MARK: - Published State

    var usageSummary: UsageSummaryResponse?
    var recentUsage: UsageResponse?
    var isLoading = true
    var error: String?
    var selectedTimeRange: TimeRange = .month

    // MARK: - Dependencies

    private let client: DianeClientProtocol

    init(client: DianeClientProtocol = DianeClient.shared) {
        self.client = client
    }

    // MARK: - Time Range

    enum TimeRange: String, CaseIterable {
        case day = "24 Hours"
        case week = "7 Days"
        case month = "30 Days"
        case year = "1 Year"

        var from: Date {
            let now = Date()
            switch self {
            case .day:   return Calendar.current.date(byAdding: .day, value: -1, to: now) ?? now
            case .week:  return Calendar.current.date(byAdding: .day, value: -7, to: now) ?? now
            case .month: return Calendar.current.date(byAdding: .month, value: -1, to: now) ?? now
            case .year:  return Calendar.current.date(byAdding: .year, value: -1, to: now) ?? now
            }
        }
    }

    // MARK: - Async Operations

    func loadData() async {
        isLoading = true
        error = nil
        FileLogger.shared.info("Loading usage data for time range '\(selectedTimeRange.rawValue)'...", category: "Usage")

        do {
            let loadedSummary = try await client.getUsageSummary(from: selectedTimeRange.from, to: nil)
            let loadedRecent = try await client.getUsage(from: selectedTimeRange.from, to: nil, limit: 50, service: nil, providerID: nil)

            usageSummary = loadedSummary
            recentUsage = loadedRecent
            FileLogger.shared.info("Loaded usage data: \(loadedSummary.summary.count) summary records, \(loadedRecent.records.count) recent entries", category: "Usage")
        } catch {
            self.error = error.localizedDescription
            FileLogger.shared.error("Failed to load usage data: \(error.localizedDescription)", category: "Usage")
        }

        isLoading = false
    }

    // MARK: - Pure / Static Helpers

    /// Format a token count into a human-readable short string (e.g. "1.2K", "3.4M").
    static func formatTokens(_ count: Int) -> String {
        if count >= 1_000_000 {
            return String(format: "%.1fM", Double(count) / 1_000_000.0)
        } else if count >= 1_000 {
            return String(format: "%.1fK", Double(count) / 1_000.0)
        }
        return "\(count)"
    }

    // MARK: - Computed Summaries

    /// Total number of requests across all summary records.
    var totalRequests: Int {
        usageSummary?.summary.reduce(0) { $0 + $1.totalRequests } ?? 0
    }

    /// Total tokens across all summary records.
    var totalTokens: Int {
        usageSummary?.summary.reduce(0) { $0 + $1.totalTokens } ?? 0
    }

    /// Number of distinct providers.
    var providerCount: Int {
        Set(usageSummary?.summary.map { $0.providerID } ?? []).count
    }
}
