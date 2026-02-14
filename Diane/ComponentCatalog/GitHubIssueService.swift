import Foundation

// MARK: - IssueCreationResult

enum IssueCreationResult: Equatable {
    case success(url: String)
    case error(message: String)
}

// MARK: - GitHubIssueService

@MainActor
final class GitHubIssueService {

    // MARK: - Selector Generation

    /// Generates a human-readable selector from the current catalog state.
    /// e.g. "AgentsView [Loaded]" or "InfoRow" (no suffix when no preset).
    static func selector(for item: CatalogItem, preset: CatalogPreset?) -> String {
        if let preset {
            return "\(item.rawValue) [\(preset.name)]"
        }
        return item.rawValue
    }

    // MARK: - Issue Title

    /// Generates a title: `[Catalog] {selector}: {first line truncated to 60 chars}`
    static func issueTitle(selector: String, comment: String) -> String {
        let firstLine = comment
            .trimmingCharacters(in: .whitespacesAndNewlines)
            .components(separatedBy: .newlines)
            .first ?? ""
        let truncated = firstLine.count > 60
            ? String(firstLine.prefix(60)) + "..."
            : firstLine
        return "[Catalog] \(selector): \(truncated)"
    }

    // MARK: - Issue Body

    /// Generates a structured markdown issue body.
    static func issueBody(
        selector: String,
        component: String,
        category: String,
        state: String,
        comment: String
    ) -> String {
        """
        ## Catalog Feedback: \(selector)

        **Component:** \(component)
        **Category:** \(category)
        **State:** \(state)

        ---

        \(comment)
        """
    }

    // MARK: - Create Issue

    /// Shells out to `gh issue create` and returns the result.
    /// - Parameters:
    ///   - item: The catalog item being reviewed.
    ///   - preset: The currently active preset (nil if none).
    ///   - comment: The user's feedback text.
    ///   - repo: Optional `owner/repo` override. When nil, `gh` uses the repo from the working directory.
    /// - Returns: An `IssueCreationResult` with either the issue URL or an error message.
    static func createIssue(
        item: CatalogItem,
        preset: CatalogPreset?,
        comment: String,
        repo: String? = nil
    ) async -> IssueCreationResult {
        let sel = selector(for: item, preset: preset)
        let title = issueTitle(selector: sel, comment: comment)
        let state = preset?.name ?? "Default"
        let body = issueBody(
            selector: sel,
            component: item.rawValue,
            category: item.category.rawValue,
            state: state,
            comment: comment
        )

        // Build arguments
        var args = [
            "issue", "create",
            "--title", title,
            "--body", body,
            "--label", "catalog-feedback"
        ]
        if let repo, !repo.isEmpty {
            args += ["-R", repo]
        }

        // Find gh
        let ghPath = "/usr/local/bin/gh"
        let brewGhPath = "/opt/homebrew/bin/gh"
        let resolvedGhPath: String
        if FileManager.default.fileExists(atPath: brewGhPath) {
            resolvedGhPath = brewGhPath
        } else if FileManager.default.fileExists(atPath: ghPath) {
            resolvedGhPath = ghPath
        } else {
            // Try PATH lookup via /usr/bin/env
            resolvedGhPath = "/usr/bin/env"
            args = ["gh"] + args
        }

        let finalArgs = args
        return await withCheckedContinuation { continuation in
            DispatchQueue.global(qos: .userInitiated).async {
                let process = Process()
                process.executableURL = URL(fileURLWithPath: resolvedGhPath)
                process.arguments = finalArgs

                // Set working directory to project root for gh repo detection.
                // Walk up from the app bundle to find the directory containing Diane/.
                if let bundleURL = Bundle.main.bundleURL as URL? {
                    var dir = bundleURL.deletingLastPathComponent()
                    for _ in 0..<10 {
                        let candidate = dir.appendingPathComponent("Diane")
                        if FileManager.default.fileExists(atPath: candidate.path) {
                            process.currentDirectoryURL = dir
                            break
                        }
                        let parent = dir.deletingLastPathComponent()
                        if parent == dir { break }
                        dir = parent
                    }
                }

                let stdoutPipe = Pipe()
                let stderrPipe = Pipe()
                process.standardOutput = stdoutPipe
                process.standardError = stderrPipe

                do {
                    try process.run()
                    process.waitUntilExit()
                } catch {
                    continuation.resume(returning: .error(message: "Failed to launch gh: \(error.localizedDescription)"))
                    return
                }

                let stdoutData = stdoutPipe.fileHandleForReading.readDataToEndOfFile()
                let stderrData = stderrPipe.fileHandleForReading.readDataToEndOfFile()
                let stdout = String(data: stdoutData, encoding: .utf8)?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
                let stderr = String(data: stderrData, encoding: .utf8)?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""

                if process.terminationStatus == 0 {
                    // gh prints the issue URL to stdout on success
                    continuation.resume(returning: .success(url: stdout))
                } else {
                    let message = stderr.isEmpty ? "gh exited with status \(process.terminationStatus)" : stderr
                    continuation.resume(returning: .error(message: message))
                }
            }
        }
    }
}
