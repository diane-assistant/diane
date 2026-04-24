import Foundation
import SwiftUI
import os.log

@MainActor
@Observable
final class EmergentWorkspaceImagesViewModel {
    var images: [EmergentWorkspaceImageDTO] = []
    var selectedImage: EmergentWorkspaceImageDTO?
    
    var isLoading = false
    var error: String?
    
    // Register Image form state
    var newImageName = ""
    var newImageDockerRef = ""
    var newImageProvider = ""
    var isRegistering = false
    var registerError: String?
    
    func loadImages() async {
        isLoading = true
        error = nil
        do {
            images = try await EmergentAdminClient.shared.getWorkspaceImages()
        } catch {
            self.error = error.localizedDescription
            Logger(subsystem: "com.diane.mac", category: "EmergentWorkspaceImagesViewModel")
                .error("Failed to load workspace images: \(error.localizedDescription)")
        }
        isLoading = false
    }
    
    func selectImage(_ image: EmergentWorkspaceImageDTO?) {
        selectedImage = image
    }
    
    func registerImage() async -> Bool {
        isRegistering = true
        registerError = nil
        
        let req = EmergentCreateWorkspaceImageRequest(
            name: newImageName,
            dockerRef: newImageDockerRef,
            provider: newImageProvider.isEmpty ? nil : newImageProvider
        )
        
        do {
            let _ = try await EmergentAdminClient.shared.createWorkspaceImage(dto: req)
            await loadImages()
            
            // reset form
            newImageName = ""
            newImageDockerRef = ""
            newImageProvider = ""
            isRegistering = false
            return true
        } catch {
            registerError = error.localizedDescription
            isRegistering = false
            return false
        }
    }
    
    func deleteImage(id: String) async -> Bool {
        do {
            try await EmergentAdminClient.shared.deleteWorkspaceImage(id: id)
            if selectedImage?.id == id {
                selectedImage = nil
            }
            await loadImages()
            return true
        } catch {
            self.error = error.localizedDescription
            return false
        }
    }
}
