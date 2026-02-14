import Foundation

/// Simple file logger that writes to ~/.diane/dianemenu.log
final class FileLogger: Sendable {
    static let shared = FileLogger()
    
    private let logFileURL: URL
    private let dateFormatter: DateFormatter
    private let queue = DispatchQueue(label: "com.diane.FileLogger", qos: .utility)
    
    private init() {
        let homeDir = FileManager.default.homeDirectoryForCurrentUser
        logFileURL = homeDir.appendingPathComponent(".diane/dianemenu.log")
        
        dateFormatter = DateFormatter()
        dateFormatter.dateFormat = "yyyy-MM-dd HH:mm:ss.SSS"
        
        // Create .diane directory if needed
        let dianeDir = homeDir.appendingPathComponent(".diane")
        try? FileManager.default.createDirectory(at: dianeDir, withIntermediateDirectories: true)
        
        // Log startup
        log("FileLogger initialized, logging to: \(logFileURL.path)")
    }
    
    func log(_ message: String, level: String = "INFO", category: String = "General", file: String = #file, line: Int = #line) {
        let timestamp = dateFormatter.string(from: Date())
        let fileName = (file as NSString).lastPathComponent
        let logLine = "[\(timestamp)] [\(level)] [\(category)] \(fileName):\(line) - \(message)\n"
        
        queue.async { [weak self] in
            guard let self = self else { return }
            
            if let handle = try? FileHandle(forWritingTo: self.logFileURL) {
                handle.seekToEndOfFile()
                if let data = logLine.data(using: .utf8) {
                    handle.write(data)
                }
                try? handle.close()
            } else {
                // File doesn't exist, create it
                try? logLine.write(to: self.logFileURL, atomically: true, encoding: .utf8)
            }
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
}

// Convenience global function
func fileLog(_ message: String, level: String = "INFO", category: String = "General", file: String = #file, line: Int = #line) {
    FileLogger.shared.log(message, level: level, category: category, file: file, line: line)
}
