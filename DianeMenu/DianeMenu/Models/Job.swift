import Foundation

/// Represents a scheduled job
struct Job: Codable, Identifiable {
    let id: Int64
    let name: String
    let command: String
    let schedule: String
    let enabled: Bool
    let actionType: String?
    let agentName: String?
    let createdAt: Date
    let updatedAt: Date
    
    enum CodingKeys: String, CodingKey {
        case id
        case name
        case command
        case schedule
        case enabled
        case actionType = "action_type"
        case agentName = "agent_name"
        case createdAt = "created_at"
        case updatedAt = "updated_at"
    }
    
    /// Human-readable description of the cron schedule
    var scheduleDescription: String {
        CronParser.describe(schedule)
    }
    
    /// Whether this is an agent action job
    var isAgentAction: Bool {
        actionType == "agent"
    }
}

/// Represents a job execution log entry
struct JobExecution: Codable, Identifiable {
    let id: Int64
    let jobID: Int64
    let jobName: String?
    let startedAt: Date
    let endedAt: Date?
    let exitCode: Int?
    let stdout: String?
    let stderr: String?
    let error: String?
    
    enum CodingKeys: String, CodingKey {
        case id
        case jobID = "job_id"
        case jobName = "job_name"
        case startedAt = "started_at"
        case endedAt = "ended_at"
        case exitCode = "exit_code"
        case stdout
        case stderr
        case error
    }
    
    /// Whether the execution was successful
    var isSuccess: Bool {
        exitCode == 0 && error == nil
    }
    
    /// Duration of the execution
    var duration: TimeInterval? {
        guard let endedAt = endedAt else { return nil }
        return endedAt.timeIntervalSince(startedAt)
    }
    
    /// Formatted duration string
    var durationString: String {
        guard let duration = duration else { return "running..." }
        if duration < 1 {
            return String(format: "%.0fms", duration * 1000)
        } else if duration < 60 {
            return String(format: "%.1fs", duration)
        } else if duration < 3600 {
            let minutes = Int(duration / 60)
            let seconds = Int(duration.truncatingRemainder(dividingBy: 60))
            return "\(minutes)m \(seconds)s"
        } else {
            let hours = Int(duration / 3600)
            let minutes = Int((duration.truncatingRemainder(dividingBy: 3600)) / 60)
            return "\(hours)h \(minutes)m"
        }
    }
}

/// Simple cron expression parser for human-readable descriptions
enum CronParser {
    static func describe(_ expression: String) -> String {
        let parts = expression.split(separator: " ").map(String.init)
        guard parts.count >= 5 else { return expression }
        
        let minute = parts[0]
        let hour = parts[1]
        let dayOfMonth = parts[2]
        let month = parts[3]
        let dayOfWeek = parts[4]
        
        // Common patterns
        if minute == "*" && hour == "*" && dayOfMonth == "*" && month == "*" && dayOfWeek == "*" {
            return "Every minute"
        }
        
        if minute.starts(with: "*/") && hour == "*" && dayOfMonth == "*" && month == "*" && dayOfWeek == "*" {
            let interval = String(minute.dropFirst(2))
            return "Every \(interval) minutes"
        }
        
        if hour.starts(with: "*/") && dayOfMonth == "*" && month == "*" && dayOfWeek == "*" {
            let interval = String(hour.dropFirst(2))
            return "Every \(interval) hours"
        }
        
        if minute != "*" && hour != "*" && dayOfMonth == "*" && month == "*" && dayOfWeek == "*" {
            return "Daily at \(formatTime(hour: hour, minute: minute))"
        }
        
        if minute != "*" && hour != "*" && dayOfMonth == "*" && month == "*" && dayOfWeek != "*" {
            let days = describeDaysOfWeek(dayOfWeek)
            return "\(days) at \(formatTime(hour: hour, minute: minute))"
        }
        
        if minute != "*" && hour != "*" && dayOfMonth != "*" && month == "*" && dayOfWeek == "*" {
            return "Monthly on day \(dayOfMonth) at \(formatTime(hour: hour, minute: minute))"
        }
        
        if minute == "0" && hour == "*" && dayOfMonth == "*" && month == "*" && dayOfWeek == "*" {
            return "Every hour"
        }
        
        return expression
    }
    
    private static func formatTime(hour: String, minute: String) -> String {
        let h = Int(hour) ?? 0
        let m = Int(minute) ?? 0
        let period = h >= 12 ? "PM" : "AM"
        let displayHour = h == 0 ? 12 : (h > 12 ? h - 12 : h)
        return String(format: "%d:%02d %@", displayHour, m, period)
    }
    
    private static func describeDaysOfWeek(_ expr: String) -> String {
        let dayNames = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"]
        
        if expr.contains(",") {
            let days = expr.split(separator: ",").compactMap { Int($0) }.map { dayNames[$0 % 7] }
            return days.joined(separator: ", ")
        }
        
        if expr.contains("-") {
            let range = expr.split(separator: "-").compactMap { Int($0) }
            if range.count == 2 {
                return "\(dayNames[range[0] % 7])-\(dayNames[range[1] % 7])"
            }
        }
        
        if let day = Int(expr) {
            return dayNames[day % 7]
        }
        
        // Handle text day names
        let textDays: [String: String] = [
            "0": "Sunday", "1": "Monday", "2": "Tuesday", "3": "Wednesday",
            "4": "Thursday", "5": "Friday", "6": "Saturday", "7": "Sunday"
        ]
        
        return textDays[expr] ?? expr
    }
}
