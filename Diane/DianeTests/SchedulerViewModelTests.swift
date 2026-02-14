import XCTest
@testable import Diane

/// Tests for `SchedulerViewModel` — pure functions, async data operations,
/// and state management.
@MainActor
final class SchedulerViewModelTests: XCTestCase {

    // MARK: - Helpers

    private func makeViewModel(
        jobs: [Job] = [],
        executions: [JobExecution] = []
    ) -> (SchedulerViewModel, MockDianeClient) {
        let mock = MockDianeClient()
        mock.jobs = jobs
        mock.jobExecutions = executions
        let vm = SchedulerViewModel(client: mock)
        return (vm, mock)
    }

    // =========================================================================
    // MARK: - Pure Function Tests
    // =========================================================================

    // MARK: filteredJobs

    func testFilteredJobs_emptySearchReturnsAll() {
        let jobs = TestFixtures.makeJobList()
        let result = SchedulerViewModel.filteredJobs(jobs, searchText: "")
        XCTAssertEqual(result.count, jobs.count)
    }

    func testFilteredJobs_filtersByName() {
        let jobs = TestFixtures.makeJobList()
        let result = SchedulerViewModel.filteredJobs(jobs, searchText: "backup")
        XCTAssertEqual(result.count, 1)
        XCTAssertEqual(result.first?.name, "backup-db")
    }

    func testFilteredJobs_filtersByCommand() {
        let jobs = TestFixtures.makeJobList()
        let result = SchedulerViewModel.filteredJobs(jobs, searchText: "cleanup")
        XCTAssertEqual(result.count, 1)
        XCTAssertEqual(result.first?.name, "cleanup-logs")
    }

    func testFilteredJobs_caseInsensitive() {
        let jobs = TestFixtures.makeJobList()
        let result = SchedulerViewModel.filteredJobs(jobs, searchText: "BACKUP")
        XCTAssertEqual(result.count, 1)
    }

    func testFilteredJobs_noMatch() {
        let jobs = TestFixtures.makeJobList()
        let result = SchedulerViewModel.filteredJobs(jobs, searchText: "nonexistent")
        XCTAssertTrue(result.isEmpty)
    }

    func testFilteredJobs_emptyList() {
        let result = SchedulerViewModel.filteredJobs([], searchText: "test")
        XCTAssertTrue(result.isEmpty)
    }

    // MARK: filteredExecutions

    func testFilteredExecutions_nilJobReturnsAll() {
        let execs = TestFixtures.makeExecutionList()
        let result = SchedulerViewModel.filteredExecutions(execs, forJob: nil)
        XCTAssertEqual(result.count, execs.count)
    }

    func testFilteredExecutions_filtersByJobName() {
        let execs = TestFixtures.makeExecutionList()
        let result = SchedulerViewModel.filteredExecutions(execs, forJob: "backup-db")
        XCTAssertEqual(result.count, 2)
        XCTAssertTrue(result.allSatisfy { $0.jobName == "backup-db" })
    }

    func testFilteredExecutions_noMatch() {
        let execs = TestFixtures.makeExecutionList()
        let result = SchedulerViewModel.filteredExecutions(execs, forJob: "nonexistent")
        XCTAssertTrue(result.isEmpty)
    }

    func testFilteredExecutions_emptyList() {
        let result = SchedulerViewModel.filteredExecutions([], forJob: "backup-db")
        XCTAssertTrue(result.isEmpty)
    }

    // =========================================================================
    // MARK: - Async Operation Tests
    // =========================================================================

    // MARK: loadData

    func testLoadData_populatesJobsAndExecutions() async {
        let jobs = TestFixtures.makeJobList()
        let execs = TestFixtures.makeExecutionList()
        let (vm, _) = makeViewModel(jobs: jobs, executions: execs)

        await vm.loadData()

        XCTAssertEqual(vm.jobs.count, jobs.count)
        XCTAssertEqual(vm.executions.count, execs.count)
        XCTAssertFalse(vm.isLoading)
        XCTAssertNil(vm.error)
    }

    func testLoadData_setsErrorOnFailure() async {
        let (vm, mock) = makeViewModel()
        mock.errorToThrow = MockError.networkFailure

        await vm.loadData()

        XCTAssertNotNil(vm.error)
        XCTAssertTrue(vm.jobs.isEmpty)
        XCTAssertTrue(vm.executions.isEmpty)
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

        XCTAssertEqual(mock.callCount(for: "getJobs"), 1)
        XCTAssertEqual(mock.callCount(for: "getJobLogs"), 1)
    }

    // MARK: loadLogs

    func testLoadLogs_updatesExecutions() async {
        let execs = TestFixtures.makeExecutionList()
        let (vm, mock) = makeViewModel(executions: execs)

        await vm.loadLogs(forJob: nil)

        XCTAssertEqual(vm.executions.count, execs.count)
        XCTAssertEqual(mock.callCount(for: "getJobLogs"), 1)
    }

    func testLoadLogs_filtersForSpecificJob() async {
        let execs = TestFixtures.makeExecutionList()
        let (vm, _) = makeViewModel(executions: execs)

        await vm.loadLogs(forJob: "backup-db")

        // MockDianeClient filters by jobName
        XCTAssertEqual(vm.executions.count, 2)
        XCTAssertTrue(vm.executions.allSatisfy { $0.jobName == "backup-db" })
    }

    func testLoadLogs_silentlyFailsOnError() async {
        let (vm, mock) = makeViewModel()
        mock.errorToThrow = MockError.networkFailure

        // Should not crash or set error
        await vm.loadLogs(forJob: nil)

        // Error is silently swallowed (original behavior)
        XCTAssertNil(vm.error)
    }

    // MARK: toggleJob

    func testToggleJob_callsClientAndRefreshes() async {
        let jobs = TestFixtures.makeJobList()
        let (vm, mock) = makeViewModel(jobs: jobs)
        await vm.loadData()

        let job = jobs[0] // backup-db, enabled = true
        await vm.toggleJob(job, enabled: false)

        XCTAssertEqual(mock.callCount(for: "toggleJob"), 1)
        // getJobs called twice: once in loadData, once after toggle
        XCTAssertEqual(mock.callCount(for: "getJobs"), 2)
        // The mock updates the job in place
        XCTAssertFalse(vm.jobs.first(where: { $0.name == "backup-db" })?.enabled ?? true)
    }

    func testToggleJob_silentlyFailsOnError() async {
        let jobs = [TestFixtures.makeJob()]
        let (vm, mock) = makeViewModel(jobs: jobs)
        await vm.loadData()

        mock.errorToThrow = MockError.networkFailure
        await vm.toggleJob(jobs[0], enabled: false)

        // Should not crash or set error on ViewModel
        XCTAssertNil(vm.error)
    }

    // MARK: selectJob

    func testSelectJob_setsSelectedAndLogsFilter() {
        let (vm, _) = makeViewModel()
        let job = TestFixtures.makeJob(name: "my-job")

        vm.selectJob(job)

        XCTAssertEqual(vm.selectedJob?.name, "my-job")
        XCTAssertEqual(vm.showLogsForJob, "my-job")
    }

    // MARK: showAllLogs

    func testShowAllLogs_clearsSelectionAndFilter() {
        let (vm, _) = makeViewModel()
        let job = TestFixtures.makeJob()
        vm.selectJob(job)

        vm.showAllLogs()

        XCTAssertNil(vm.selectedJob)
        XCTAssertNil(vm.showLogsForJob)
    }

    // =========================================================================
    // MARK: - Computed Properties (instance)
    // =========================================================================

    func testFilteredJobsComputedProperty() async {
        let jobs = TestFixtures.makeJobList()
        let (vm, _) = makeViewModel(jobs: jobs)
        await vm.loadData()

        vm.searchText = "backup"
        XCTAssertEqual(vm.filteredJobs.count, 1)

        vm.searchText = ""
        XCTAssertEqual(vm.filteredJobs.count, jobs.count)
    }

    func testFilteredExecutionsComputedProperty() async {
        let execs = TestFixtures.makeExecutionList()
        let (vm, _) = makeViewModel(executions: execs)
        await vm.loadData()

        vm.showLogsForJob = "backup-db"
        XCTAssertEqual(vm.filteredExecutions.count, 2)

        vm.showLogsForJob = nil
        XCTAssertEqual(vm.filteredExecutions.count, execs.count)
    }
}
