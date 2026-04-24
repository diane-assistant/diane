import Foundation

/// Status values for an agent question.
enum AgentQuestionStatus: String, Codable {
    case pending
    case answered
    case expired
    case cancelled
}

/// A predefined answer option for an agent question.
struct AgentQuestionOption: Codable, Identifiable {
    var id: String { value }
    let label: String
    let value: String
    let description: String
}

/// A question asked by an emergent agent during a run that requires user input.
struct AgentQuestion: Codable, Identifiable {
    let id: String
    let runId: String
    let agentId: String
    let question: String
    let options: [AgentQuestionOption]
    let status: AgentQuestionStatus
    let createdAt: Date
    let respondedAt: Date?

    /// Whether the question has predefined options the user can choose from.
    var hasOptions: Bool { !options.isEmpty }
}
