import Foundation
import Observation
import os.log

private let logger = Logger(subsystem: "com.diane.Diane", category: "Scheduler")

/// ViewModel for SchedulerView that owns all scheduler state and business logic.
///
/// Accepts `DianeClientProtocol` via its initializer so tests can inject a mock.
@MainActor
@Observable
final class SchedulerViewModel {

    // MARK: - Dependencies

    @ObservationIgnored
    let client: DianeClientProtocol

    // MARK: - State

    var jobs: [Job] = []
    var executions: [JobExecution] = []
    var isLoading = true
    var error: String?
    var searchText = ""
    var selectedJob: Job?
    var showLogsForJob: String?

    // MARK: - Init

    init(client: DianeClientProtocol = DianeClient.shared) {
        self.client = client
    }

    // MARK: - Computed Properties

    var filteredJobs: [Job] {
        Self.filteredJobs(jobs, searchText: searchText)
    }

    var filteredExecutions: [JobExecution] {
        Self.filteredExecutions(executions, forJob: showLogsForJob)
    }

    // MARK: - Data Operations

    func loadData() async {
        isLoading = true
        error = nil
        FileLogger.shared.info("Loading scheduler data...", category: "Scheduler")

        do {
            let loadedJobs = try await client.getJobs()
            let loadedLogs = try await client.getJobLogs(jobName: nil, limit: 100)

            jobs = loadedJobs
            executions = loadedLogs
            FileLogger.shared.info("Loaded \(loadedJobs.count) jobs and \(loadedLogs.count) executions", category: "Scheduler")
        } catch {
            self.error = error.localizedDescription
            FileLogger.shared.error("Failed to load scheduler data: \(error.localizedDescription)", category: "Scheduler")
        }

        isLoading = false
    }

    func loadLogs(forJob jobName: String?) async {
        FileLogger.shared.info("Loading logs for job: \(jobName ?? "all")", category: "Scheduler")
        do {
            executions = try await client.getJobLogs(jobName: jobName, limit: 100)
        } catch {
            // Silently fail for log refresh (matches original behavior)
            FileLogger.shared.error("Failed to load logs for job '\(jobName ?? "all")': \(error.localizedDescription)", category: "Scheduler")
        }
    }

    func toggleJob(_ job: Job, enabled: Bool) async {
        FileLogger.shared.info("Toggling job '\(job.name)' enabled=\(enabled)", category: "Scheduler")
        do {
            try await client.toggleJob(name: job.name, enabled: enabled)
            // Refresh job list
            jobs = try await client.getJobs()
            FileLogger.shared.info("Toggled job '\(job.name)' successfully", category: "Scheduler")
        } catch {
            // Silently fail (matches original behavior)
            FileLogger.shared.error("Failed to toggle job '\(job.name)': \(error.localizedDescription)", category: "Scheduler")
        }
    }

    func selectJob(_ job: Job) {
        selectedJob = job
        showLogsForJob = job.name
    }

    func showAllLogs() {
        showLogsForJob = nil
        selectedJob = nil
    }

    // MARK: - Static Pure Functions

    /// Filter jobs by search text. Returns all jobs when search text is empty.
    static func filteredJobs(_ jobs: [Job], searchText: String) -> [Job] {
        guard !searchText.isEmpty else { return jobs }
        return jobs.filter {
            $0.name.localizedCaseInsensitiveContains(searchText) ||
            $0.command.localizedCaseInsensitiveContains(searchText)
        }
    }

    /// Filter executions by job name. Returns all executions when jobName is nil.
    static func filteredExecutions(_ executions: [JobExecution], forJob jobName: String?) -> [JobExecution] {
        guard let jobName = jobName else { return executions }
        return executions.filter { $0.jobName == jobName }
    }
}
