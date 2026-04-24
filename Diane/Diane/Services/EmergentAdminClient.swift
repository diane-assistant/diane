import Foundation
import SwiftUI
import os.log

private let logger = Logger(subsystem: "com.diane.mac", category: "EmergentAdminClient")

enum EmergentAdminClientError: LocalizedError {
    case invalidURL(path: String)
    case serverError(statusCode: Int, message: String)
    case decodingFailed(underlying: Error)
    case connectionFailed(underlying: Error)

    var errorDescription: String? {
        switch self {
        case .invalidURL(let path):
            return "Invalid URL: \(path)"
        case .serverError(let statusCode, let message):
            return "Server Error (\(statusCode)): \(message)"
        case .decodingFailed(let underlying):
            return "Failed to decode response: \(underlying.localizedDescription)"
        case .connectionFailed(let underlying):
            return "Connection failed: \(underlying.localizedDescription)"
        }
    }
}

@MainActor
final class EmergentAdminClient: ObservableObject {
    static let shared = EmergentAdminClient()

    @AppStorage("emergentBaseURL") var baseURLString: String = "http://localhost:5300"
    @AppStorage("emergentAPIKey") var bearerToken: String = ""
    @AppStorage("emergentProjectID") var projectId: String = ""

    private let session: URLSession

    private var baseURL: URL? {
        URL(string: baseURLString)
    }

    private let decoder: JSONDecoder
    private let encoder: JSONEncoder

    private init() {
        let config = URLSessionConfiguration.default
        config.timeoutIntervalForRequest = 30
        self.session = URLSession(configuration: config)

        self.decoder = JSONDecoder()
        self.decoder.keyDecodingStrategy = .convertFromSnakeCase

        self.encoder = JSONEncoder()
        self.encoder.keyEncodingStrategy = .convertToSnakeCase
    }

    private func request<T: Decodable>(_ path: String, method: String = "GET", body: Data? = nil) async throws -> T {
        guard let baseURL = self.baseURL else {
            throw EmergentAdminClientError.invalidURL(path: path)
        }
        guard let url = URL(string: path, relativeTo: baseURL) else {
            throw EmergentAdminClientError.invalidURL(path: path)
        }

        var request = URLRequest(url: url)
        request.httpMethod = method
        request.setValue("application/json", forHTTPHeaderField: "Accept")

        if let body = body {
            request.httpBody = body
            request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        }

        if !bearerToken.isEmpty {
            request.setValue("Bearer \(bearerToken)", forHTTPHeaderField: "Authorization")
        }

        if !projectId.isEmpty {
            request.setValue(projectId, forHTTPHeaderField: "X-Project-ID")
        }

        logger.debug("[\(method)] \(url.absoluteString)")

        let (data, response): (Data, URLResponse)
        do {
            (data, response) = try await session.data(for: request)
        } catch {
            throw EmergentAdminClientError.connectionFailed(underlying: error)
        }

        guard let httpResponse = response as? HTTPURLResponse else {
            throw EmergentAdminClientError.serverError(statusCode: 0, message: "Invalid response type")
        }

        if !(200...299).contains(httpResponse.statusCode) {
            let message = String(data: data, encoding: .utf8) ?? "Unknown error"
            // try to parse emergent error structure
            if let errorResp = try? decoder.decode(EmergentAPIResponse<String>.self, from: data), let err = errorResp.error {
                throw EmergentAdminClientError.serverError(statusCode: httpResponse.statusCode, message: err)
            }
            throw EmergentAdminClientError.serverError(statusCode: httpResponse.statusCode, message: message)
        }

        do {
            if T.self == EmptyResponse.self {
                return EmptyResponse() as! T
            }
            return try decoder.decode(T.self, from: data)
        } catch {
            logger.error("Decoding error: \(error.localizedDescription)\nData: \(String(data: data, encoding: .utf8) ?? "")")
            throw EmergentAdminClientError.decodingFailed(underlying: error)
        }
    }

    struct EmptyResponse: Codable {}

    // MARK: - Agents CRUD
    func getAgents() async throws -> [EmergentAgentDTO] {
        let resp: EmergentAPIResponse<[EmergentAgentDTO]> = try await request("/api/admin/agents")
        return resp.data ?? []
    }

    func getAgent(id: String) async throws -> EmergentAgentDTO {
        let resp: EmergentAPIResponse<EmergentAgentDTO> = try await request("/api/admin/agents/\(id)")
        guard let data = resp.data else { throw EmergentAdminClientError.serverError(statusCode: 404, message: "Agent not found") }
        return data
    }

    func createAgent(dto: EmergentAgentCreateDTO) async throws -> EmergentAgentDTO {
        let body = try encoder.encode(dto)
        let resp: EmergentAPIResponse<EmergentAgentDTO> = try await request("/api/admin/agents", method: "POST", body: body)
        guard let data = resp.data else { throw EmergentAdminClientError.serverError(statusCode: 500, message: "No data in response") }
        return data
    }

    func updateAgent(id: String, dto: EmergentAgentUpdateDTO) async throws -> EmergentAgentDTO {
        let body = try encoder.encode(dto)
        let resp: EmergentAPIResponse<EmergentAgentDTO> = try await request("/api/admin/agents/\(id)", method: "PATCH", body: body)
        guard let data = resp.data else { throw EmergentAdminClientError.serverError(statusCode: 500, message: "No data in response") }
        return data
    }

    func deleteAgent(id: String) async throws {
        let _: EmergentAPIResponse<EmptyResponse> = try await request("/api/admin/agents/\(id)", method: "DELETE")
    }

    // MARK: - Agent Runs
    func triggerRun(agentId: String) async throws -> EmergentTriggerResponseDTO {
        return try await request("/api/admin/agents/\(agentId)/trigger", method: "POST")
    }

    func getRuns(agentId: String, limit: Int = 10) async throws -> [EmergentAgentRunDTO] {
        let resp: EmergentAPIResponse<[EmergentAgentRunDTO]> = try await request("/api/admin/agents/\(agentId)/runs?limit=\(limit)")
        return resp.data ?? []
    }

    func cancelRun(agentId: String, runId: String) async throws {
        let _: EmergentAPIResponse<[String: String]> = try await request("/api/admin/agents/\(agentId)/runs/\(runId)/cancel", method: "POST")
    }

    func getPendingEvents(agentId: String) async throws -> EmergentPendingEventsResponseDTO {
        let resp: EmergentAPIResponse<EmergentPendingEventsResponseDTO> = try await request("/api/admin/agents/\(agentId)/pending-events")
        guard let data = resp.data else { throw EmergentAdminClientError.serverError(statusCode: 500, message: "No data in response") }
        return data
    }

    func batchTrigger(agentId: String, objectIds: [String]) async throws -> EmergentBatchTriggerResponseDTO {
        let req = EmergentBatchTriggerRequestDTO(objectIds: objectIds)
        let body = try encoder.encode(req)
        let resp: EmergentAPIResponse<EmergentBatchTriggerResponseDTO> = try await request("/api/admin/agents/\(agentId)/batch-trigger", method: "POST", body: body)
        guard let data = resp.data else { throw EmergentAdminClientError.serverError(statusCode: 500, message: "No data in response") }
        return data
    }

    // MARK: - MCP Servers
    func getMCPServers() async throws -> [EmergentMCPServerDTO] {
        let resp: EmergentAPIResponse<[EmergentMCPServerDTO]> = try await request("/api/admin/mcp-servers")
        return resp.data ?? []
    }

    func getMCPServer(id: String) async throws -> EmergentMCPServerDetailDTO {
        let resp: EmergentAPIResponse<EmergentMCPServerDetailDTO> = try await request("/api/admin/mcp-servers/\(id)")
        guard let data = resp.data else { throw EmergentAdminClientError.serverError(statusCode: 404, message: "Server not found") }
        return data
    }

    func createMCPServer(dto: EmergentCreateMCPServerDTO) async throws -> EmergentMCPServerDTO {
        let body = try encoder.encode(dto)
        let resp: EmergentAPIResponse<EmergentMCPServerDTO> = try await request("/api/admin/mcp-servers", method: "POST", body: body)
        guard let data = resp.data else { throw EmergentAdminClientError.serverError(statusCode: 500, message: "No data in response") }
        return data
    }

    func updateMCPServer(id: String, dto: EmergentUpdateMCPServerDTO) async throws -> EmergentMCPServerDTO {
        let body = try encoder.encode(dto)
        let resp: EmergentAPIResponse<EmergentMCPServerDTO> = try await request("/api/admin/mcp-servers/\(id)", method: "PATCH", body: body)
        guard let data = resp.data else { throw EmergentAdminClientError.serverError(statusCode: 500, message: "No data in response") }
        return data
    }

    func deleteMCPServer(id: String) async throws {
        // MCP server delete returns 204 No Content
        let _: EmptyResponse = try await request("/api/admin/mcp-servers/\(id)", method: "DELETE")
    }

    // MARK: - Workspace Images
    func getWorkspaceImages() async throws -> [EmergentWorkspaceImageDTO] {
        // Note: the openapi schema for workspace images indicates it returns a ListResponse which is: { "data": [...] }
        struct ListResponse: Codable { let data: [EmergentWorkspaceImageDTO] }
        let resp: ListResponse = try await request("/api/admin/workspace-images")
        return resp.data
    }

    func createWorkspaceImage(dto: EmergentCreateWorkspaceImageRequest) async throws -> EmergentWorkspaceImageDTO {
        let body = try encoder.encode(dto)
        let resp: EmergentAPIResponse<EmergentWorkspaceImageDTO> = try await request("/api/admin/workspace-images", method: "POST", body: body)
        guard let data = resp.data else { throw EmergentAdminClientError.serverError(statusCode: 500, message: "No data in response") }
        return data
    }

    func deleteWorkspaceImage(id: String) async throws {
        let _: EmptyResponse = try await request("/api/admin/workspace-images/\(id)", method: "DELETE")
    }

    // MARK: - Workspace Config
    func getWorkspaceConfig(definitionId: String) async throws -> EmergentAgentWorkspaceConfig {
        let resp: EmergentAPIResponse<EmergentAgentWorkspaceConfig> = try await request("/api/admin/agent-definitions/\(definitionId)/workspace-config")
        guard let data = resp.data else { throw EmergentAdminClientError.serverError(statusCode: 404, message: "Workspace config not found") }
        return data
    }

    func updateWorkspaceConfig(definitionId: String, config: EmergentAgentWorkspaceConfig) async throws -> EmergentAgentWorkspaceConfig {
        let body = try encoder.encode(config)
        let resp: EmergentAPIResponse<EmergentAgentWorkspaceConfig> = try await request("/api/admin/agent-definitions/\(definitionId)/workspace-config", method: "PUT", body: body)
        guard let data = resp.data else { throw EmergentAdminClientError.serverError(statusCode: 500, message: "No data in response") }
        return data
    }
}
