import XCTest
@testable import Diane

/// Tests for `UsageViewModel` — pure functions, async data operations,
/// computed properties, and state management.
@MainActor
final class UsageViewModelTests: XCTestCase {

    // MARK: - Helpers

    private func makeViewModel(
        summaryResponse: UsageSummaryResponse? = nil,
        usageResponse: UsageResponse? = nil
    ) -> (UsageViewModel, MockDianeClient) {
        let mock = MockDianeClient()
        mock.usageSummaryResponse = summaryResponse
        mock.usageResponse = usageResponse
        let vm = UsageViewModel(client: mock)
        return (vm, mock)
    }

    // =========================================================================
    // MARK: - Pure Function Tests
    // =========================================================================

    // MARK: formatTokens

    func testFormatTokens_smallNumber() {
        XCTAssertEqual(UsageViewModel.formatTokens(0), "0")
        XCTAssertEqual(UsageViewModel.formatTokens(1), "1")
        XCTAssertEqual(UsageViewModel.formatTokens(999), "999")
    }

    func testFormatTokens_thousands() {
        XCTAssertEqual(UsageViewModel.formatTokens(1000), "1.0K")
        XCTAssertEqual(UsageViewModel.formatTokens(1500), "1.5K")
        XCTAssertEqual(UsageViewModel.formatTokens(999_999), "1000.0K")
    }

    func testFormatTokens_millions() {
        XCTAssertEqual(UsageViewModel.formatTokens(1_000_000), "1.0M")
        XCTAssertEqual(UsageViewModel.formatTokens(2_500_000), "2.5M")
        XCTAssertEqual(UsageViewModel.formatTokens(10_000_000), "10.0M")
    }

    // =========================================================================
    // MARK: - Async Operation Tests
    // =========================================================================

    // MARK: loadData

    func testLoadData_populatesSummaryAndRecent() async {
        let summary = TestFixtures.makeUsageSummaryResponse()
        let usage = TestFixtures.makeUsageResponse()
        let (vm, _) = makeViewModel(summaryResponse: summary, usageResponse: usage)

        await vm.loadData()

        XCTAssertNotNil(vm.usageSummary)
        XCTAssertNotNil(vm.recentUsage)
        XCTAssertEqual(vm.usageSummary?.summary.count, 3)
        XCTAssertEqual(vm.recentUsage?.records.count, 3)
        XCTAssertFalse(vm.isLoading)
        XCTAssertNil(vm.error)
    }

    func testLoadData_setsErrorOnFailure() async {
        let (vm, mock) = makeViewModel()
        mock.errorToThrow = MockError.networkFailure

        await vm.loadData()

        XCTAssertNotNil(vm.error)
        XCTAssertNil(vm.usageSummary)
        XCTAssertNil(vm.recentUsage)
        XCTAssertFalse(vm.isLoading)
    }

    func testLoadData_setsIsLoadingFalseAfterCompletion() async {
        let (vm, _) = makeViewModel()
        XCTAssertTrue(vm.isLoading) // initial value

        await vm.loadData()

        XCTAssertFalse(vm.isLoading)
    }

    func testLoadData_callsClientMethods() async {
        let (vm, mock) = makeViewModel()

        await vm.loadData()

        XCTAssertEqual(mock.callCount(for: "getUsageSummary"), 1)
        XCTAssertEqual(mock.callCount(for: "getUsage"), 1)
    }

    func testLoadData_clearsExistingErrorOnRetry() async {
        let (vm, mock) = makeViewModel()
        mock.errorToThrow = MockError.networkFailure
        await vm.loadData()
        XCTAssertNotNil(vm.error)

        // Clear error and retry
        mock.errorToThrow = nil
        await vm.loadData()
        XCTAssertNil(vm.error)
    }

    func testLoadData_emptyResponseSetsNilSummaryData() async {
        // No summaryResponse or usageResponse configured => mock returns empty defaults
        let (vm, _) = makeViewModel()

        await vm.loadData()

        XCTAssertNotNil(vm.usageSummary)
        XCTAssertNotNil(vm.recentUsage)
        XCTAssertEqual(vm.usageSummary?.summary.count, 0)
        XCTAssertEqual(vm.recentUsage?.records.count, 0)
    }

    // =========================================================================
    // MARK: - Computed Property Tests
    // =========================================================================

    func testTotalRequests_withData() async {
        let summary = TestFixtures.makeUsageSummaryResponse()
        let (vm, _) = makeViewModel(summaryResponse: summary)
        await vm.loadData()

        // 10 + 25 + 5 = 40
        XCTAssertEqual(vm.totalRequests, 40)
    }

    func testTotalRequests_withoutData() {
        let (vm, _) = makeViewModel()
        XCTAssertEqual(vm.totalRequests, 0)
    }

    func testTotalTokens_withData() async {
        let summary = TestFixtures.makeUsageSummaryResponse()
        let (vm, _) = makeViewModel(summaryResponse: summary)
        await vm.loadData()

        // Record 1: 5000+2000=7000, Record 2: 12000+4000=16000, Record 3: 3000+1500=4500
        // Total: 27500
        XCTAssertEqual(vm.totalTokens, 27500)
    }

    func testTotalTokens_withoutData() {
        let (vm, _) = makeViewModel()
        XCTAssertEqual(vm.totalTokens, 0)
    }

    func testProviderCount_withData() async {
        let summary = TestFixtures.makeUsageSummaryResponse()
        let (vm, _) = makeViewModel(summaryResponse: summary)
        await vm.loadData()

        // 2 providers: openai (id 1) and anthropic (id 2)
        XCTAssertEqual(vm.providerCount, 2)
    }

    func testProviderCount_withoutData() {
        let (vm, _) = makeViewModel()
        XCTAssertEqual(vm.providerCount, 0)
    }

    // =========================================================================
    // MARK: - Time Range Tests
    // =========================================================================

    func testTimeRange_allCases() {
        let cases = UsageViewModel.TimeRange.allCases
        XCTAssertEqual(cases.count, 4)
        XCTAssertEqual(cases.map(\.rawValue), ["24 Hours", "7 Days", "30 Days", "1 Year"])
    }

    func testTimeRange_fromDates() {
        let now = Date()

        // Day range should be roughly 24h ago
        let dayRange = UsageViewModel.TimeRange.day.from
        let dayDiff = now.timeIntervalSince(dayRange)
        XCTAssertEqual(dayDiff, 86400, accuracy: 2.0)

        // Week range should be roughly 7 days ago
        let weekRange = UsageViewModel.TimeRange.week.from
        let weekDiff = now.timeIntervalSince(weekRange)
        XCTAssertEqual(weekDiff, 86400 * 7, accuracy: 2.0)
    }

    func testSelectedTimeRange_defaultIsMonth() {
        let (vm, _) = makeViewModel()
        XCTAssertEqual(vm.selectedTimeRange, .month)
    }

    func testSelectedTimeRange_canBeChanged() {
        let (vm, _) = makeViewModel()
        vm.selectedTimeRange = .week
        XCTAssertEqual(vm.selectedTimeRange, .week)
    }
}
