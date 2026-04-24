import Foundation
import SwiftUI

@MainActor
@Observable
final class EmergentMCPRegistryViewModel {
    var servers: [EmergentMCPServerDTO] = []
    var selectedServerId: String? {
        didSet {
            if let id = selectedServerId {
                Task { await loadServerDetail(id: id) }
            } else {
                selectedServerDetail = nil
            }
            isInEditMode = false
        }
    }
    var selectedServerDetail: EmergentMCPServerDetailDTO?
    
    var isLoading = false
    var error: String?
    
    // Edit Mode State
    var isInEditMode = false
    var editSnapshot: EmergentMCPServerDetailDTO?
    var editError: String?
    
    var editName = ""
    var editCommand = ""
    var editArgs: [String] = []
    var editEnv: [String: String] = [:]
    var editUrl = ""
    var editHeaders: [String: String] = [:]
    var editEnabled = false
    
    var hasChanges: Bool {
        guard let snapshot = editSnapshot else { return false }
        return snapshot.name != editName ||
               (snapshot.command ?? "") != editCommand ||
               (snapshot.args ?? []) != editArgs ||
               (snapshot.env ?? [:]) != editEnv ||
               (snapshot.url ?? "") != editUrl ||
               (snapshot.headers ?? [:]) != editHeaders ||
               snapshot.enabled != editEnabled
    }
    
    func loadServers() async {
        isLoading = true
        error = nil
        do {
            servers = try await EmergentAdminClient.shared.getMCPServers()
        } catch {
            self.error = error.localizedDescription
        }
        isLoading = false
    }
    
    func loadServerDetail(id: String) async {
        do {
            selectedServerDetail = try await EmergentAdminClient.shared.getMCPServer(id: id)
        } catch {
            self.error = error.localizedDescription
        }
    }
    
    func enterEditMode() {
        guard let detail = selectedServerDetail else { return }
        editSnapshot = detail
        editName = detail.name
        editCommand = detail.command ?? ""
        editArgs = detail.args ?? []
        editEnv = detail.env ?? [:]
        editUrl = detail.url ?? ""
        editHeaders = detail.headers ?? [:]
        editEnabled = detail.enabled
        isInEditMode = true
        editError = nil
    }
    
    func discardInlineEdit() {
        isInEditMode = false
        editError = nil
    }
    
    func saveInlineEdit() async {
        guard let detail = selectedServerDetail, hasChanges else { return }
        editError = nil
        
        let update = EmergentUpdateMCPServerDTO(
            name: editName,
            command: detail.type == .stdio ? (editCommand.isEmpty ? nil : editCommand) : nil,
            args: detail.type == .stdio ? editArgs : nil,
            env: editEnv,
            url: (detail.type == .sse || detail.type == .http) ? (editUrl.isEmpty ? nil : editUrl) : nil,
            headers: (detail.type == .sse || detail.type == .http) ? editHeaders : nil,
            enabled: editEnabled
        )
        
        do {
            let updated = try await EmergentAdminClient.shared.updateMCPServer(id: detail.id, dto: update)
            if let idx = servers.firstIndex(where: { $0.id == updated.id }) {
                servers[idx] = updated
            }
            await loadServerDetail(id: detail.id)
            isInEditMode = false
        } catch {
            editError = error.localizedDescription
        }
    }
    
    func createServer(dto: EmergentCreateMCPServerDTO) async throws {
        _ = try await EmergentAdminClient.shared.createMCPServer(dto: dto)
        await loadServers()
    }
    
    func deleteServer(id: String) async -> Bool {
        do {
            try await EmergentAdminClient.shared.deleteMCPServer(id: id)
            if selectedServerId == id {
                selectedServerId = nil
            }
            await loadServers()
            return true
        } catch {
            self.error = error.localizedDescription
            return false
        }
    }
}
