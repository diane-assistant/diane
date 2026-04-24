import Foundation
import Combine

/// Manages the state for the questions panel.
/// Polls the daemon every 10 seconds for pending questions and provides
/// a method for submitting answers.
@MainActor
class QuestionsViewModel: ObservableObject {
    @Published var questions: [AgentQuestion] = []
    @Published var pendingCount: Int = 0
    @Published var isLoading: Bool = false
    @Published var error: String?

    private let client: DianeClient
    private var pollingTask: Task<Void, Never>?

    init(client: DianeClient = .shared) {
        self.client = client
        startPolling()
    }

    deinit {
        pollingTask?.cancel()
    }

    /// Fetch pending questions immediately.
    func refresh() async {
        isLoading = true
        error = nil
        do {
            let fetched = try await client.fetchPendingQuestions()
            questions = fetched.sorted { $0.createdAt < $1.createdAt }
            pendingCount = questions.count
        } catch {
            self.error = error.localizedDescription
        }
        isLoading = false
    }

    /// Submit an answer for a question. Refreshes the list on success.
    func respond(to id: String, response: String) async throws {
        try await client.respondToQuestion(id: id, response: response)
        await refresh()
    }

    // MARK: - Private

    private func startPolling() {
        pollingTask?.cancel()
        pollingTask = Task { [weak self] in
            while !Task.isCancelled {
                await self?.refresh()
                try? await Task.sleep(nanoseconds: 10_000_000_000) // 10 seconds
            }
        }
    }
}
