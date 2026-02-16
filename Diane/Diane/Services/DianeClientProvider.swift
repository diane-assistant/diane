import Foundation
import SwiftUI

/// Protocol for providing the current configured Diane client
protocol DianeClientProviding {
    var configuredClient: DianeClientProtocol? { get }
}

/// Extension to make StatusMonitor conform to DianeClientProviding
extension StatusMonitor: DianeClientProviding {
    // Already implemented in StatusMonitor.swift
}

/// Environment key for DianeClientProvider
private struct DianeClientProviderKey: EnvironmentKey {
    static let defaultValue: DianeClientProviding? = nil
}

extension EnvironmentValues {
    var dianeClientProvider: DianeClientProviding? {
        get { self[DianeClientProviderKey.self] }
        set { self[DianeClientProviderKey.self] = newValue }
    }
}