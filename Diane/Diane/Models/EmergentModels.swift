import Foundation

// MARK: - API Response Wrapper
struct EmergentAPIResponse<T: Codable>: Codable {
    let success: Bool
    let data: T?
    let message: String?
    let error: String?
}

// MARK: - Agents
enum EmergentAgentTriggerType: String, Codable, CaseIterable {
    case schedule, manual, reaction, webhook
}

enum EmergentAgentExecutionMode: String, Codable, CaseIterable {
    case suggest, execute, hybrid
}

struct EmergentAgentCapabilities: Codable, Equatable {
    var canCreateObjects: Bool?
    var canUpdateObjects: Bool?
    var canDeleteObjects: Bool?
    var canCreateRelationships: Bool?
    var allowedObjectTypes: [String]?
}

enum EmergentConcurrencyStrategy: String, Codable, CaseIterable {
    case skip, parallel
}

enum EmergentReactionEventType: String, Codable, CaseIterable {
    case created, updated, deleted
}

struct EmergentReactionConfig: Codable, Equatable {
    var events: [EmergentReactionEventType]?
    var objectTypes: [String]?
    var concurrencyStrategy: EmergentConcurrencyStrategy?
    var ignoreAgentTriggered: Bool?
    var ignoreSelfTriggered: Bool?
}

struct EmergentAgentDTO: Codable, Identifiable, Equatable {
    let id: String
    let projectId: String
    var name: String
    var description: String?
    var prompt: String?
    var triggerType: EmergentAgentTriggerType?
    var executionMode: EmergentAgentExecutionMode?
    var strategyType: String?
    var cronSchedule: String?
    var enabled: Bool
    var capabilities: EmergentAgentCapabilities?
    var reactionConfig: EmergentReactionConfig?
    let lastRunAt: String?
    let lastRunStatus: String?
    let createdAt: String?
    let updatedAt: String?
}

struct EmergentAgentCreateDTO: Codable {
    var name: String
    var description: String?
    var prompt: String?
    var triggerType: EmergentAgentTriggerType?
    var executionMode: EmergentAgentExecutionMode?
    var strategyType: String?
    var cronSchedule: String?
    var enabled: Bool
    var capabilities: EmergentAgentCapabilities?
    var reactionConfig: EmergentReactionConfig?
    var projectId: String?
}

struct EmergentAgentUpdateDTO: Codable {
    var name: String?
    var description: String?
    var prompt: String?
    var triggerType: EmergentAgentTriggerType?
    var executionMode: EmergentAgentExecutionMode?
    var strategyType: String?
    var cronSchedule: String?
    var enabled: Bool?
    var capabilities: EmergentAgentCapabilities?
    var reactionConfig: EmergentReactionConfig?
}

// MARK: - Agent Runs
enum EmergentAgentRunStatus: String, Codable {
    case running, success, skipped, error, paused, cancelled
}

enum EmergentSessionStatus: String, Codable {
    case provisioning, active, completed, error
}

struct EmergentAgentRunDTO: Codable, Identifiable, Equatable {
    let id: String
    let agentId: String
    let status: EmergentAgentRunStatus
    let sessionStatus: EmergentSessionStatus?
    let startedAt: String?
    let completedAt: String?
    let durationMs: Int?
    let stepCount: Int?
    let errorMessage: String?
    let parentRunId: String?
    let maxSteps: Int?
}

// MARK: - Pending Events & Batch Trigger
struct EmergentPendingEventObjectDTO: Codable, Identifiable, Equatable, Hashable {
    let id: String
    let type: String
    let key: String
    let createdAt: String?
    let updatedAt: String?
    let version: Int?
}

struct EmergentPendingEventsResponseDTO: Codable {
    let totalCount: Int
    let objects: [EmergentPendingEventObjectDTO]?
    let reactionConfig: EmergentReactionConfig?
}

struct EmergentBatchTriggerRequestDTO: Codable {
    let objectIds: [String]
}

struct EmergentBatchTriggerResponseDTO: Codable {
    let queued: Int
    let skipped: Int
}

struct EmergentTriggerResponseDTO: Codable {
    let success: Bool
    let message: String?
    let runId: String?
    let error: String?
}

// MARK: - MCP Servers
enum EmergentMCPServerType: String, Codable, CaseIterable {
    case builtin, stdio, sse, http
}

struct EmergentMCPServerDTO: Codable, Identifiable, Equatable {
    let id: String
    let projectId: String
    var name: String
    let type: EmergentMCPServerType
    var command: String?
    var args: [String]?
    var env: [String: String]?
    var url: String?
    var headers: [String: String]?
    var enabled: Bool
    let toolCount: Int?
    let createdAt: String?
    let updatedAt: String?
}

struct EmergentMCPServerToolDTO: Codable, Identifiable, Equatable {
    let id: String
    let serverId: String
    let toolName: String
    let description: String?
    let enabled: Bool
}

struct EmergentMCPServerDetailDTO: Codable, Identifiable, Equatable {
    let id: String
    let projectId: String
    var name: String
    let type: EmergentMCPServerType
    var command: String?
    var args: [String]?
    var env: [String: String]?
    var url: String?
    var headers: [String: String]?
    var enabled: Bool
    let toolCount: Int?
    let tools: [EmergentMCPServerToolDTO]?
    let createdAt: String?
    let updatedAt: String?
}

struct EmergentCreateMCPServerDTO: Codable {
    var name: String
    var type: EmergentMCPServerType
    var command: String?
    var args: [String]?
    var env: [String: String]?
    var url: String?
    var headers: [String: String]?
    var enabled: Bool
}

struct EmergentUpdateMCPServerDTO: Codable {
    var name: String?
    var command: String?
    var args: [String]?
    var env: [String: String]?
    var url: String?
    var headers: [String: String]?
    var enabled: Bool?
}

// MARK: - Workspace Images
struct EmergentWorkspaceImageDTO: Codable, Identifiable, Equatable, Hashable {
    let id: String
    let projectId: String?
    let name: String
    let provider: String?
    let type: String?
    let status: String?
    let dockerRef: String?
    let errorMsg: String?
    let createdAt: String?
    let updatedAt: String?
}

struct EmergentCreateWorkspaceImageRequest: Codable {
    var name: String
    var dockerRef: String
    var provider: String?
}

// MARK: - Agent Workspace Config
enum EmergentRepoSourceType: String, Codable, CaseIterable {
    case none, taskContext = "task_context", fixed
}

struct EmergentRepoSourceConfig: Codable, Equatable {
    var type: EmergentRepoSourceType
    var url: String?
    var branch: String?
}

struct EmergentResourceLimits: Codable, Equatable {
    var cpu: String?
    var memory: String?
    var disk: String?
}

struct EmergentAgentWorkspaceConfig: Codable, Equatable {
    var enabled: Bool
    var baseImage: String?
    var provider: String?
    var repoSource: EmergentRepoSourceConfig?
    var resourceLimits: EmergentResourceLimits?
    var setupCommands: [String]?
    var tools: [String]?
    var checkoutOnStart: Bool?
}
