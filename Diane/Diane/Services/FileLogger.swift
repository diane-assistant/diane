import Foundation

/// Log level for filtering
enum LogLevel: Int, Comparable, Sendable {
    case debug = 0
    case info = 1
    case warning = 2
    case error = 3
    
    static func < (lhs: LogLevel, rhs: LogLevel) -> Bool {
        lhs.rawValue < rhs.rawValue
    }
    
    var label: String {
        switch self {
        case .debug: return "DEBUG"
        case .info: return "INFO"
        case .warning: return "WARN"
        case .error: return "ERROR"
        }
    }
}

/// File logger with log rotation and level filtering.
///
/// Writes to `~/.diane/dianemenu.log` with automatic rotation:
/// - Rotates when file exceeds `maxFileSize` (default 10MB)
/// - Keeps up to `maxRotatedFiles` (default 3) rotated copies
/// - Rotated files named `dianemenu.1.log`, `dianemenu.2.log`, etc.
/// - On init, truncates any existing log over `maxFileSize` (cleans up legacy bloat)
final class FileLogger: Sendable {
    static let shared = FileLogger()
    
    private let logFileURL: URL
    private let dateFormatter: DateFormatter
    private let queue = DispatchQueue(label: "com.diane.FileLogger", qos: .utility)
    
    /// Maximum log file size before rotation (10MB)
    private let maxFileSize: UInt64 = 10 * 1024 * 1024
    
    /// Number of rotated log files to keep
    private let maxRotatedFiles = 3
    
    /// Minimum log level to write (DEBUG = everything)
    nonisolated(unsafe) static var minimumLevel: LogLevel = .debug
    
    private init() {
        let homeDir = FileManager.default.homeDirectoryForCurrentUser
        logFileURL = homeDir.appendingPathComponent(".diane/dianemenu.log")
        
        dateFormatter = DateFormatter()
        dateFormatter.dateFormat = "yyyy-MM-dd HH:mm:ss.SSS"
        
        // Create .diane directory if needed
        let dianeDir = homeDir.appendingPathComponent(".diane")
        try? FileManager.default.createDirectory(at: dianeDir, withIntermediateDirectories: true)
        
        // On startup: if existing log is oversized, rotate it immediately
        // This handles the legacy 179MB+ log file from before rotation existed
        rotateIfNeeded()
        
        // Log startup (after rotation so this goes into a fresh file if rotated)
        writeLogLine("[INFO] [General] FileLogger.swift:0 - FileLogger initialized with rotation (max \(maxFileSize / 1024 / 1024)MB, keep \(maxRotatedFiles))\n")
    }
    
    func log(_ message: String, level: String = "INFO", category: String = "General", file: String = #file, line: Int = #line) {
        // Parse level string for filtering
        let logLevel: LogLevel
        switch level {
        case "DEBUG": logLevel = .debug
        case "WARN": logLevel = .warning
        case "ERROR": logLevel = .error
        default: logLevel = .info
        }
        
        guard logLevel >= Self.minimumLevel else { return }
        
        let timestamp = dateFormatter.string(from: Date())
        let fileName = (file as NSString).lastPathComponent
        let logLine = "[\(timestamp)] [\(level)] [\(category)] \(fileName):\(line) - \(message)\n"
        
        queue.async { [weak self] in
            guard let self = self else { return }
            self.writeLogLine(logLine)
            self.rotateIfNeeded()
        }
    }
    
    func debug(_ message: String, category: String = "General", file: String = #file, line: Int = #line) {
        log(message, level: "DEBUG", category: category, file: file, line: line)
    }
    
    func info(_ message: String, category: String = "General", file: String = #file, line: Int = #line) {
        log(message, level: "INFO", category: category, file: file, line: line)
    }
    
    func warning(_ message: String, category: String = "General", file: String = #file, line: Int = #line) {
        log(message, level: "WARN", category: category, file: file, line: line)
    }
    
    func error(_ message: String, category: String = "General", file: String = #file, line: Int = #line) {
        log(message, level: "ERROR", category: category, file: file, line: line)
    }
    
    // MARK: - Private
    
    /// Write a single log line to the file. Must be called on `queue`.
    private func writeLogLine(_ logLine: String) {
        if let handle = try? FileHandle(forWritingTo: logFileURL) {
            handle.seekToEndOfFile()
            if let data = logLine.data(using: .utf8) {
                handle.write(data)
            }
            try? handle.close()
        } else {
            // File doesn't exist, create it
            try? logLine.write(to: logFileURL, atomically: true, encoding: .utf8)
        }
    }
    
    /// Check file size and rotate if over the limit. Must be called on `queue`.
    private func rotateIfNeeded() {
        guard let attrs = try? FileManager.default.attributesOfItem(atPath: logFileURL.path),
              let fileSize = attrs[.size] as? UInt64,
              fileSize > maxFileSize else {
            return
        }
        
        let fm = FileManager.default
        let dir = logFileURL.deletingLastPathComponent()
        let baseName = logFileURL.deletingPathExtension().lastPathComponent // "dianemenu"
        let ext = logFileURL.pathExtension // "log"
        
        // Delete the oldest rotated file if it exists
        let oldestPath = dir.appendingPathComponent("\(baseName).\(maxRotatedFiles).\(ext)").path
        try? fm.removeItem(atPath: oldestPath)
        
        // Shift existing rotated files: 2→3, 1→2
        for i in stride(from: maxRotatedFiles - 1, through: 1, by: -1) {
            let src = dir.appendingPathComponent("\(baseName).\(i).\(ext)").path
            let dst = dir.appendingPathComponent("\(baseName).\(i + 1).\(ext)").path
            if fm.fileExists(atPath: src) {
                try? fm.moveItem(atPath: src, toPath: dst)
            }
        }
        
        // Move current log to .1
        let rotatedPath = dir.appendingPathComponent("\(baseName).1.\(ext)").path
        try? fm.moveItem(atPath: logFileURL.path, toPath: rotatedPath)
        
        // Create fresh empty log file with rotation notice
        let notice = "--- Log rotated at \(dateFormatter.string(from: Date())) (previous file was \(fileSize / 1024 / 1024)MB) ---\n"
        try? notice.write(to: logFileURL, atomically: true, encoding: .utf8)
    }
}

// Convenience global function
func fileLog(_ message: String, level: String = "INFO", category: String = "General", file: String = #file, line: Int = #line) {
    FileLogger.shared.log(message, level: level, category: category, file: file, line: line)
}
