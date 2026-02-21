import Foundation

/// Represents a workspace configuration for emergent agents
struct WorkspaceConfig: Codable, Equatable {
    let baseImage: String?
    let repoUrl: String?
    let repoBranch: String?
    let provider: String?
    let setupCommands: [String]?
    
    enum CodingKeys: String, CodingKey {
        case baseImage = "base_image"
        case repoUrl = "repo_url"
        case repoBranch = "repo_branch"
        case provider
        case setupCommands = "setup_commands"
    }
}
