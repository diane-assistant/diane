import SwiftUI

/// List of scheduled jobs.
struct JobsListView: View {
    @Environment(\.dianeClient) private var client

    @State private var jobs: [Job] = []
    @State private var searchText = ""
    @State private var isLoading = true

    private var filteredJobs: [Job] {
        if searchText.isEmpty { return jobs }
        return jobs.filter { $0.name.localizedCaseInsensitiveContains(searchText) }
    }

    var body: some View {
        Group {
            if isLoading && jobs.isEmpty {
                ProgressView("Loading jobs...")
            } else if jobs.isEmpty {
                EmptyStateView(
                    icon: "clock",
                    title: "No Jobs",
                    description: "No scheduled jobs are configured"
                )
            } else {
                List(filteredJobs) { job in
                    NavigationLink {
                        JobDetailView(job: job)
                    } label: {
                        JobRow(job: job)
                    }
                }
                .searchable(text: $searchText, prompt: "Search jobs")
            }
        }
        .navigationTitle("Jobs")
        .refreshable { await refresh() }
        .task { await refresh() }
    }

    private func refresh() async {
        guard let client else { return }
        jobs = (try? await client.getJobs()) ?? []
        isLoading = false
    }
}

// MARK: - Job Row

private struct JobRow: View {
    let job: Job

    var body: some View {
        HStack {
            VStack(alignment: .leading, spacing: 2) {
                Text(job.name)
                    .font(.body)
                Text(job.scheduleDescription)
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            Spacer()
            if job.enabled {
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

// MARK: - Job Detail

struct JobDetailView: View {
    let job: Job

    @Environment(\.dianeClient) private var client
    @State private var executions: [JobExecution] = []
    @State private var isLoadingLogs = true

    var body: some View {
        List {
            DetailSection(title: "Configuration") {
                InfoRow(label: "Name", value: job.name)
                InfoRow(label: "Schedule", value: job.schedule)
                InfoRow(label: "Description", value: job.scheduleDescription)
                InfoRow(label: "Enabled", value: job.enabled ? "Yes" : "No")
                if let actionType = job.actionType {
                    InfoRow(label: "Action Type", value: actionType)
                }
                if let agentName = job.agentName {
                    InfoRow(label: "Agent", value: agentName)
                }
            }

            DetailSection(title: "Command") {
                Text(job.command)
                    .font(.system(.caption, design: .monospaced))
                    .textSelection(.enabled)
            }

            DetailSection(title: "Timestamps") {
                InfoRow(label: "Created", value: job.createdAt.formatted())
                InfoRow(label: "Updated", value: job.updatedAt.formatted())
            }

            // Recent executions
            DetailSection(title: "Recent Executions") {
                if isLoadingLogs {
                    ProgressView()
                } else if executions.isEmpty {
                    Text("No recent executions")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                } else {
                    ForEach(executions) { exec in
                        VStack(alignment: .leading, spacing: 4) {
                            HStack {
                                Image(systemName: exec.isSuccess ? "checkmark.circle.fill" : "xmark.circle.fill")
                                    .foregroundStyle(exec.isSuccess ? .green : .red)
                                    .font(.caption)
                                Text(exec.startedAt.formatted())
                                    .font(.caption)
                                Spacer()
                                Text(exec.durationString)
                                    .font(.caption2)
                                    .foregroundStyle(.secondary)
                            }
                            if let error = exec.error, !error.isEmpty {
                                Text(error)
                                    .font(.caption2)
                                    .foregroundStyle(.red)
                                    .lineLimit(2)
                            }
                        }
                    }
                }
            }
        }
        .navigationTitle(job.name)
        .task {
            guard let client else { return }
            executions = (try? await client.getJobLogs(jobName: job.name, limit: 10)) ?? []
            isLoadingLogs = false
        }
    }
}
