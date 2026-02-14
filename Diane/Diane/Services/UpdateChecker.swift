import Foundation
import AppKit
import os.log

private let logger = Logger(subsystem: "com.diane.Diane", category: "UpdateChecker")

/// Checks for updates from GitHub releases and handles auto-update
@MainActor
class UpdateChecker: ObservableObject {
    @Published var updateAvailable: Bool = false
    @Published var latestVersion: String?
    @Published var releaseURL: URL?
    @Published var releaseNotes: String?
    @Published var isChecking: Bool = false
    @Published var isUpdating: Bool = false
    @Published var updateProgress: Double = 0
    @Published var updateStatus: String = ""
    @Published var updateError: String?
    
    private let repoOwner = "diane-assistant"
    private let repoName = "diane"
    private nonisolated(unsafe) var checkTimer: Timer?
    private let checkInterval: TimeInterval = 180 // Check every 3 minutes
    private var hasStarted = false
    private var downloadAssetURL: URL?
    
    // Reference to status monitor to pause during updates
    weak var statusMonitor: StatusMonitor?
    
    // Minimum version that has the Unix socket API required by Diane
    // Versions before this don't have the API and will break Diane
    private let minimumCompatibleVersion = "1.3.0"
    
    init() {
        // Don't start any async work here - wait for start() to be called
    }
    
    deinit {
        checkTimer?.invalidate()
    }
    
    /// Call this after the app has fully initialized
    func start() async {
        guard !hasStarted else { return }
        hasStarted = true
        
        // Check on launch
        await checkForUpdates()
        
        // Setup periodic check
        checkTimer = Timer.scheduledTimer(withTimeInterval: checkInterval, repeats: true) { [weak self] _ in
            Task { @MainActor [weak self] in
                await self?.checkForUpdates()
            }
        }
    }
    
    /// Check GitHub for the latest release
    func checkForUpdates() async {
        FileLogger.shared.info("checkForUpdates starting", category: "UpdateChecker")
        isChecking = true
        defer { isChecking = false }
        
        guard let url = URL(string: "https://api.github.com/repos/\(repoOwner)/\(repoName)/releases/latest") else {
            return
        }
        
        do {
            var request = URLRequest(url: url)
            request.setValue("application/vnd.github.v3+json", forHTTPHeaderField: "Accept")
            request.timeoutInterval = 10
            
            let (data, response) = try await URLSession.shared.data(for: request)
            
            guard let httpResponse = response as? HTTPURLResponse,
                  httpResponse.statusCode == 200 else {
                return
            }
            
            let release = try JSONDecoder().decode(GitHubRelease.self, from: data)
            
            latestVersion = release.tagName
            releaseURL = URL(string: release.htmlUrl)
            releaseNotes = release.body
            
            // Find the darwin-arm64 or darwin-amd64 binary asset
            downloadAssetURL = findDarwinAsset(in: release.assets)
            
            // Compare with current version from the running server
            // We get this from the status API rather than running --version
            // because the binary doesn't have a --version flag
            let currentVersion = await getCurrentVersionFromAPI()
            
            // Only offer update if:
            // 1. Latest version is newer than current
            // 2. Latest version meets minimum compatibility (has socket API)
            let isNewer = isNewerVersion(release.tagName, than: currentVersion)
            let isCompatible = !isNewerVersion(minimumCompatibleVersion, than: release.tagName)
            updateAvailable = isNewer && isCompatible
            
            logger.info("Update check: latest=\(release.tagName), current=\(currentVersion), isNewer=\(isNewer), isCompatible=\(isCompatible), updateAvailable=\(self.updateAvailable)")
            FileLogger.shared.info("Update check: latest=\(release.tagName), current=\(currentVersion), isNewer=\(isNewer), isCompatible=\(isCompatible), updateAvailable=\(self.updateAvailable)", category: "UpdateChecker")
            
        } catch {
            // Silently fail - don't bother user with update check errors
            print("Update check failed: \(error)")
        }
    }
    
    /// Find the appropriate darwin binary asset for this machine
    private func findDarwinAsset(in assets: [GitHubAsset]?) -> URL? {
        guard let assets = assets else { return nil }
        
        // Determine architecture
        #if arch(arm64)
        let archPattern = "darwin-arm64"
        let fallbackPattern = "darwin"
        #else
        let archPattern = "darwin-amd64"
        let fallbackPattern = "darwin"
        #endif
        
        // Try to find exact match first
        if let asset = assets.first(where: { $0.name.contains(archPattern) }) {
            return URL(string: asset.browserDownloadUrl)
        }
        
        // Fallback to any darwin asset
        if let asset = assets.first(where: { $0.name.contains(fallbackPattern) }) {
            return URL(string: asset.browserDownloadUrl)
        }
        
        return nil
    }
    
    /// Perform the auto-update
    func performUpdate() async {
        logger.info("performUpdate called")
        
        guard let assetURL = downloadAssetURL else {
            logger.error("No download URL available")
            updateError = "No download URL available"
            return
        }
        
        logger.info("Starting update from: \(assetURL.absoluteString)")
        
        // Pause status monitor during update to prevent error flashing
        statusMonitor?.isPaused = true
        
        isUpdating = true
        updateError = nil
        updateProgress = 0
        updateStatus = "Preparing update..."
        
        let homeDir = FileManager.default.homeDirectoryForCurrentUser
        let binaryPath = homeDir.appendingPathComponent(".diane/bin/diane").path
        let backupPath = homeDir.appendingPathComponent(".diane/bin/diane.backup").path
        
        do {
            // Step 1: Download the new binary
            updateStatus = "Downloading update..."
            updateProgress = 0.1
            
            let (tempURL, _) = try await downloadFile(from: assetURL)
            
            updateProgress = 0.4
            updateStatus = "Extracting update..."
            
            // Small delay to show extraction status
            try await Task.sleep(nanoseconds: 200_000_000)
            
            updateProgress = 0.5
            updateStatus = "Installing update..."
            
            // Step 2: Stop Diane
            updateProgress = 0.6
            updateStatus = "Stopping Diane..."
            
            await stopDiane()
            
            // Wait a moment for process to fully stop
            try await Task.sleep(nanoseconds: 500_000_000)
            
            // Step 3: Backup current binary
            updateProgress = 0.7
            updateStatus = "Backing up current version..."
            
            let fm = FileManager.default
            
            // Remove old backup if exists
            try? fm.removeItem(atPath: backupPath)
            
            // Backup current binary
            if fm.fileExists(atPath: binaryPath) {
                try fm.moveItem(atPath: binaryPath, toPath: backupPath)
            }
            
            // Step 4: Move new binary into place
            updateProgress = 0.8
            updateStatus = "Installing new version..."
            
            try fm.moveItem(atPath: tempURL.path, toPath: binaryPath)
            
            // Make it executable
            try fm.setAttributes([.posixPermissions: 0o755], ofItemAtPath: binaryPath)
            
            // Step 5: Start Diane
            updateProgress = 0.9
            updateStatus = "Starting Diane..."
            
            try startDiane()
            
            // Wait for it to start
            try await Task.sleep(nanoseconds: 3_000_000_000)
            
            updateProgress = 1.0
            updateStatus = "Update complete!"
            updateAvailable = false
            
            // Clean up backup after successful start
            try? fm.removeItem(atPath: backupPath)
            
            // Resume status monitor and refresh
            statusMonitor?.isPaused = false
            await statusMonitor?.refresh()
            
        } catch {
            logger.error("Update failed: \(error.localizedDescription)")
            updateError = error.localizedDescription
            updateStatus = "Update failed"
            
            // Try to restore from backup
            let fm = FileManager.default
            if fm.fileExists(atPath: backupPath) && !fm.fileExists(atPath: binaryPath) {
                try? fm.moveItem(atPath: backupPath, toPath: binaryPath)
                try? startDiane()
            }
            
            // Resume status monitor even on failure
            statusMonitor?.isPaused = false
        }
        
        // Keep showing status for a moment, then reset
        try? await Task.sleep(nanoseconds: 2_000_000_000)
        isUpdating = false
    }
    
    /// Download a file, extract if tar.gz, and return the local URL of the binary
    private func downloadFile(from url: URL) async throws -> (URL, URLResponse) {
        let (localURL, response) = try await URLSession.shared.download(from: url)
        
        let homeDir = FileManager.default.homeDirectoryForCurrentUser
        let binDir = homeDir.appendingPathComponent(".diane/bin")
        let downloadPath = binDir.appendingPathComponent("diane.download.tar.gz")
        let extractDir = binDir.appendingPathComponent("diane.extract")
        let finalPath = binDir.appendingPathComponent("diane.new")
        
        let fm = FileManager.default
        
        // Clean up any previous download artifacts
        try? fm.removeItem(at: downloadPath)
        try? fm.removeItem(at: extractDir)
        try? fm.removeItem(at: finalPath)
        
        // Move downloaded file to our location
        try fm.moveItem(at: localURL, to: downloadPath)
        
        // Check if this is a tar.gz file (check by filename or content)
        let urlString = url.absoluteString.lowercased()
        if urlString.hasSuffix(".tar.gz") || urlString.hasSuffix(".tgz") {
            // Extract the tar.gz file
            try fm.createDirectory(at: extractDir, withIntermediateDirectories: true)
            
            let process = Process()
            process.executableURL = URL(fileURLWithPath: "/usr/bin/tar")
            process.arguments = ["-xzf", downloadPath.path, "-C", extractDir.path]
            process.standardOutput = FileHandle.nullDevice
            process.standardError = FileHandle.nullDevice
            
            try process.run()
            process.waitUntilExit()
            
            guard process.terminationStatus == 0 else {
                throw UpdateError.extractionFailed("tar extraction failed with status \(process.terminationStatus)")
            }
            
            // Find the diane binary in the extracted files
            let extractedBinary = try findDianeBinary(in: extractDir)
            
            // Move the binary to final location
            try fm.moveItem(at: extractedBinary, to: finalPath)
            
            // Clean up
            try? fm.removeItem(at: downloadPath)
            try? fm.removeItem(at: extractDir)
            
            return (finalPath, response)
        } else {
            // Not a tar.gz, assume it's the raw binary
            try fm.moveItem(at: downloadPath, to: finalPath)
            return (finalPath, response)
        }
    }
    
    /// Find the diane binary in an extracted directory
    private func findDianeBinary(in directory: URL) throws -> URL {
        let fm = FileManager.default
        
        // Check for "diane" directly in the directory
        let directPath = directory.appendingPathComponent("diane")
        if fm.fileExists(atPath: directPath.path) {
            return directPath
        }
        
        // Search one level deep (in case it's in a subdirectory)
        if let contents = try? fm.contentsOfDirectory(at: directory, includingPropertiesForKeys: nil) {
            for item in contents {
                var isDir: ObjCBool = false
                if fm.fileExists(atPath: item.path, isDirectory: &isDir) && isDir.boolValue {
                    let nestedPath = item.appendingPathComponent("diane")
                    if fm.fileExists(atPath: nestedPath.path) {
                        return nestedPath
                    }
                }
                // Check if the item itself is the diane binary
                if item.lastPathComponent == "diane" {
                    return item
                }
            }
        }
        
        throw UpdateError.binaryNotFound("Could not find diane binary in extracted archive")
    }
    
    /// Update-specific errors
    enum UpdateError: LocalizedError {
        case extractionFailed(String)
        case binaryNotFound(String)
        
        var errorDescription: String? {
            switch self {
            case .extractionFailed(let msg): return msg
            case .binaryNotFound(let msg): return msg
            }
        }
    }
    
    /// Stop Diane server
    private func stopDiane() async {
        let homeDir = FileManager.default.homeDirectoryForCurrentUser
        let pidPath = homeDir.appendingPathComponent(".diane/mcp.pid").path
        
        guard let content = try? String(contentsOfFile: pidPath, encoding: .utf8),
              let pid = Int(content.trimmingCharacters(in: .whitespacesAndNewlines)) else {
            return
        }
        
        kill(Int32(pid), SIGTERM)
        
        // Wait for process to exit
        for _ in 0..<20 {
            if kill(Int32(pid), 0) != 0 {
                break
            }
            try? await Task.sleep(nanoseconds: 100_000_000)
        }
    }
    
    /// Start Diane server
    private func startDiane() throws {
        let homeDir = FileManager.default.homeDirectoryForCurrentUser
        let binaryPath = homeDir.appendingPathComponent(".diane/bin/diane").path
        
        let process = Process()
        process.executableURL = URL(fileURLWithPath: binaryPath)
        process.standardInput = FileHandle.nullDevice
        process.standardOutput = FileHandle.nullDevice
        process.standardError = FileHandle.nullDevice
        
        try process.run()
    }
    
    /// Get current Diane version from the running server's API
    private func getCurrentVersionFromAPI() async -> String {
        let homeDir = FileManager.default.homeDirectoryForCurrentUser
        let socketPath = homeDir.appendingPathComponent(".diane/diane.sock").path
        
        return await withCheckedContinuation { continuation in
            DispatchQueue.global(qos: .utility).async {
                let process = Process()
                process.executableURL = URL(fileURLWithPath: "/usr/bin/curl")
                process.arguments = [
                    "--unix-socket", socketPath,
                    "-s", // silent
                    "-f", // fail on HTTP errors
                    "--max-time", "3",
                    "--connect-timeout", "2",
                    "http://localhost/status"
                ]
                
                let pipe = Pipe()
                process.standardOutput = pipe
                process.standardError = FileHandle.nullDevice
                
                do {
                    try process.run()
                    process.waitUntilExit()
                    
                    guard process.terminationStatus == 0 else {
                        logger.warning("getCurrentVersionFromAPI: curl failed with status \(process.terminationStatus)")
                        FileLogger.shared.warning("getCurrentVersionFromAPI: curl failed with status \(process.terminationStatus)", category: "UpdateChecker")
                        continuation.resume(returning: "unknown")
                        return
                    }
                    
                    let data = pipe.fileHandleForReading.readDataToEndOfFile()
                    
                    // Parse JSON to extract version
                    if let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
                       let version = json["version"] as? String {
                        logger.info("getCurrentVersionFromAPI: got version \(version)")
                        FileLogger.shared.info("getCurrentVersionFromAPI: got version \(version)", category: "UpdateChecker")
                        continuation.resume(returning: version)
                    } else {
                        let rawData = String(data: data, encoding: .utf8) ?? "<non-utf8>"
                        logger.warning("getCurrentVersionFromAPI: failed to parse JSON")
                        FileLogger.shared.warning("getCurrentVersionFromAPI: failed to parse JSON, raw=\(rawData.prefix(200))", category: "UpdateChecker")
                        continuation.resume(returning: "unknown")
                    }
                } catch {
                    logger.error("getCurrentVersionFromAPI: error \(error.localizedDescription)")
                    FileLogger.shared.error("getCurrentVersionFromAPI: error \(error.localizedDescription)", category: "UpdateChecker")
                    continuation.resume(returning: "unknown")
                }
            }
        }
    }
    
    /// Compare semantic versions
    private func isNewerVersion(_ latest: String, than current: String) -> Bool {
        let latestClean = cleanVersion(latest)
        let currentClean = cleanVersion(current)
        
        let latestParts = latestClean.split(separator: ".").compactMap { Int($0) }
        let currentParts = currentClean.split(separator: ".").compactMap { Int($0) }
        
        // Pad arrays to same length
        let maxLen = max(latestParts.count, currentParts.count)
        let latestPadded = latestParts + Array(repeating: 0, count: maxLen - latestParts.count)
        let currentPadded = currentParts + Array(repeating: 0, count: maxLen - currentParts.count)
        
        logger.debug("isNewerVersion: latest='\(latest)' -> '\(latestClean)' -> \(latestPadded), current='\(current)' -> '\(currentClean)' -> \(currentPadded)")
        FileLogger.shared.debug("isNewerVersion: latest='\(latest)' -> '\(latestClean)' -> \(latestPadded), current='\(current)' -> '\(currentClean)' -> \(currentPadded)", category: "UpdateChecker")
        
        for i in 0..<maxLen {
            if latestPadded[i] > currentPadded[i] {
                return true
            } else if latestPadded[i] < currentPadded[i] {
                return false
            }
        }
        
        return false
    }
    
    /// Clean version string (remove 'v' prefix and any suffix like '-dirty')
    private func cleanVersion(_ version: String) -> String {
        var clean = version
        
        // Remove 'v' prefix
        if clean.hasPrefix("v") {
            clean = String(clean.dropFirst())
        }
        
        // Remove anything after the version number (e.g., "-1-gb2d7e13-dirty")
        if let dashIndex = clean.firstIndex(of: "-") {
            clean = String(clean[..<dashIndex])
        }
        
        return clean
    }
    
    /// Open the release page in browser (fallback)
    func openReleasePage() {
        if let url = releaseURL {
            NSWorkspace.shared.open(url)
        }
    }
}

// MARK: - GitHub API Models

struct GitHubRelease: Codable {
    let tagName: String
    let htmlUrl: String
    let body: String?
    let publishedAt: String?
    let assets: [GitHubAsset]?
    
    enum CodingKeys: String, CodingKey {
        case tagName = "tag_name"
        case htmlUrl = "html_url"
        case body
        case publishedAt = "published_at"
        case assets
    }
}

struct GitHubAsset: Codable {
    let name: String
    let browserDownloadUrl: String
    let size: Int
    
    enum CodingKeys: String, CodingKey {
        case name
        case browserDownloadUrl = "browser_download_url"
        case size
    }
}
