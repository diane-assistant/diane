import Foundation
import os.log
import SwiftUI

private let logger = Logger(subsystem: "com.diane.ios", category: "IOSStatusMonitor")

/// Connection state for the iOS app
enum IOSConnectionState: Equatable {
    case connecting
    case connected
    case disconnected
    case error(String)
}

/// iOS-specific status monitor that polls the Diane server over HTTP.
/// Manages connection state and responds to app lifecycle changes.
@MainActor
@Observable
class IOSStatusMonitor {
    let client: DianeHTTPClient

    var connectionState: IOSConnectionState = .disconnected
    var status: DianeStatus?

    private var pollingTask: Task<Void, Never>?
    private var isPolling = false

    /// Polling interval in seconds
    private let pollInterval: TimeInterval = 5

    init(client: DianeHTTPClient) {
        self.client = client
    }

    /// Start polling the server for status updates
    func startPolling() {
        guard !isPolling else { return }
        isPolling = true
        connectionState = .connecting

        logger.info("Starting status polling")

        pollingTask = Task { [weak self] in
            guard let self else { return }

            // Immediate first poll
            await self.poll()

            // Then poll on interval
            while !Task.isCancelled && self.isPolling {
                try? await Task.sleep(nanoseconds: UInt64(self.pollInterval * 1_000_000_000))
                guard !Task.isCancelled && self.isPolling else { break }
                await self.poll()
            }
        }
    }

    /// Stop polling
    func stopPolling() {
        logger.info("Stopping status polling")
        isPolling = false
        pollingTask?.cancel()
        pollingTask = nil
    }

    /// Handle scene phase changes — start/stop polling based on app state
    func handleScenePhase(_ phase: ScenePhase) {
        switch phase {
        case .active:
            logger.info("App became active, starting polling")
            startPolling()
        case .inactive, .background:
            logger.info("App going to background, stopping polling")
            stopPolling()
        @unknown default:
            break
        }
    }

    /// Perform a single poll
    func poll() async {
        do {
            let newStatus = try await client.getStatus()
            self.status = newStatus
            self.connectionState = .connected
        } catch {
            logger.error("Poll failed: \(error.localizedDescription)")

            if case .connected = connectionState {
                // Was connected, now lost connection
                self.connectionState = .disconnected
            } else if case .connecting = connectionState {
                // Still trying to connect
                self.connectionState = .error(error.localizedDescription)
            }
            // If already disconnected/error, keep that state
        }
    }
}
