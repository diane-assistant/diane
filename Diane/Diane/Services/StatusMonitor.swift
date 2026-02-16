import Foundation
import Combine
import UserNotifications
import os.log

private let logger = Logger(subsystem: "com.diane.Diane", category: "StatusMonitor")

/// Observable object that monitors Diane status.
/// Supports both local (Unix socket) and remote (HTTP) connections.
@MainActor
class StatusMonitor: ObservableObject {
    @Published var status: DianeStatus = .empty
    @Published var connectionState: ConnectionState = .unknown
    @Published var isLoading: Bool = false
    @Published var lastError: String?
    @Published var isPaused: Bool = false  // Pause during updates
    @Published var isRemoteMode: Bool = false
    @Published var serverDisplayName: String = "Unknown"
    
    // Slave management
    @Published var slaves: [SlaveInfo] = []
    @Published var pendingPairingRequests: [PairingRequest] = []
    
    private var client: DianeClientProtocol?
    
    /// The configured client instance for use by ViewModels
    /// This allows ViewModels to use the same remote/local client as StatusMonitor
    var configuredClient: DianeClientProtocol? {
        return client
    }
    private nonisolated(unsafe) var pollTimer: Timer?
    private var pollInterval: TimeInterval = 5.0
    private var hasStarted = false
    
    init() {
        logger.info("StatusMonitor initialized")
        FileLogger.shared.info("StatusMonitor initialized", category: "StatusMonitor")
    }
    
    deinit {
        pollTimer?.invalidate()
    }
    
    /// Configure for local mode (Unix socket)
    func configureLocal() {
        logger.info("StatusMonitor configuring for local mode")
        FileLogger.shared.info("StatusMonitor configuring for local mode", category: "StatusMonitor")
        client = DianeClient()
        isRemoteMode = false
        serverDisplayName = "Local"
    }
    
    /// Configure for remote mode (HTTP)
    func configureRemote(baseURL: URL, apiKey: String?) {
        logger.info("StatusMonitor configuring for remote mode: \(baseURL.absoluteString)")
        FileLogger.shared.info("StatusMonitor configuring for remote mode: \(baseURL.absoluteString)", category: "StatusMonitor")
        let effectiveKey = (apiKey?.isEmpty ?? true) ? nil : apiKey
        client = DianeHTTPClient(baseURL: baseURL, apiKey: effectiveKey)
        isRemoteMode = true
        serverDisplayName = baseURL.host ?? "Remote"
    }
    
    /// Configure from a ServerConfiguration object
    func configure(from config: ServerConfiguration) {
        stopPolling()
        hasStarted = false
        
        guard config.isConfigured, let mode = config.connectionMode else {
            logger.info("StatusMonitor: config not yet configured, skipping")
            client = nil
            connectionState = .disconnected
            status = .empty
            return
        }
        
        switch mode {
        case .local:
            configureLocal()
        case .remote:
            guard let url = config.baseURL else {
                logger.error("StatusMonitor: remote mode but no base URL")
                client = nil
                connectionState = .error("No server URL configured")
                return
            }
            configureRemote(baseURL: url, apiKey: config.apiKey)
        }
    }
    
    /// Call this after the app has fully initialized
    func start() async {
        guard !hasStarted else { 
            logger.info("StatusMonitor already started, skipping")
            FileLogger.shared.info("StatusMonitor already started, skipping", category: "StatusMonitor")
            return 
        }
        
        guard client != nil else {
            logger.info("StatusMonitor: no client configured, cannot start")
            return
        }
        
        hasStarted = true
        logger.info("StatusMonitor starting...")
        FileLogger.shared.info("StatusMonitor starting...", category: "StatusMonitor")
        
        await startPolling()
        
        // Auto-start Diane if enabled and not already running (local mode only)
        if !isRemoteMode {
            await autoStartIfNeeded()
        }
    }
    
    /// Auto-start Diane if the setting is enabled (local mode only)
    private func autoStartIfNeeded() async {
        guard !isRemoteMode else { return }
        
        // Small delay to let the initial status check complete
        try? await Task.sleep(nanoseconds: 500_000_000)
        
        // Check if auto-start is enabled (default true for first launch)
        let autoStartEnabled = UserDefaults.standard.object(forKey: "autoStartDiane") == nil || 
                               UserDefaults.standard.bool(forKey: "autoStartDiane")
        
        logger.info("Auto-start enabled: \(autoStartEnabled), isRunning: \(self.isRunning)")
        
        guard autoStartEnabled else {
            return
        }
        
        // Only start if not already running
        if !isRunning {
            logger.info("Auto-starting Diane...")
            await startDiane()
        }
    }
    
    /// Start polling for status updates
    func startPolling() async {
        logger.info("Starting polling...")
        
        // Initial fetch
        await refresh()
        
        // Setup timer for periodic updates
        pollTimer = Timer.scheduledTimer(withTimeInterval: pollInterval, repeats: true) { [weak self] _ in
            Task { @MainActor [weak self] in
                await self?.refresh()
            }
        }
        logger.info("Polling timer started with interval: \(self.pollInterval)s")
    }
    
    /// Stop polling
    func stopPolling() {
        pollTimer?.invalidate()
        pollTimer = nil
        logger.info("Polling stopped")
    }
    
    /// Refresh status
    func refresh() async {
        guard let client else {
            connectionState = .disconnected
            return
        }
        
        // Skip refresh if paused (e.g., during update)
        guard !isPaused else {
            logger.info("Refresh skipped - monitor is paused")
            return
        }
        
        if isRemoteMode {
            // Remote mode: just try to get status via HTTP
            do {
                let newStatus = try await client.getStatus()
                status = newStatus
                connectionState = .connected
                lastError = nil
                logger.info("Remote status refresh successful: connected, \(newStatus.totalTools) tools")
                FileLogger.shared.info("Remote status refresh successful", category: "StatusMonitor")
            } catch {
                logger.error("Remote status refresh failed: \(error.localizedDescription)")
                FileLogger.shared.error("Remote status refresh failed: \(error.localizedDescription)", category: "StatusMonitor")
                connectionState = .error("Connection failed")
                status = .empty
                lastError = error.localizedDescription
            }
        } else {
            // Local mode: check socket and process
            let socketExists = client.socketExists
            let processRunning = client.isProcessRunning()
            logger.info("Refresh: socketExists=\(socketExists), processRunning=\(processRunning)")
            FileLogger.shared.info("Refresh: socketExists=\(socketExists), processRunning=\(processRunning)", category: "StatusMonitor")
            
            // Quick check if socket exists
            guard socketExists || processRunning else {
                logger.info("No socket and no process, setting disconnected")
                FileLogger.shared.info("No socket and no process, setting disconnected", category: "StatusMonitor")
                connectionState = .disconnected
                status = .empty
                return
            }
            
            do {
                let newStatus = try await client.getStatus()
                status = newStatus
                connectionState = .connected
                lastError = nil
                logger.info("Status refresh successful: connected, \(newStatus.totalTools) tools")
                FileLogger.shared.info("Status refresh successful: connected, \(newStatus.totalTools) tools", category: "StatusMonitor")
            } catch {
                logger.error("Status refresh failed: \(error.localizedDescription)")
                FileLogger.shared.error("Status refresh failed: \(error.localizedDescription)", category: "StatusMonitor")
                // Fallback: check if process is running via PID
                if client.isProcessRunning() {
                    logger.warning("Process running but API failed, setting error state")
                    FileLogger.shared.warning("Process running but API failed, setting error state", category: "StatusMonitor")
                    connectionState = .error("API unavailable")
                } else {
                    logger.info("Process not running, setting disconnected")
                    FileLogger.shared.info("Process not running, setting disconnected", category: "StatusMonitor")
                    connectionState = .disconnected
                    status = .empty
                }
                lastError = error.localizedDescription
            }
        }
        
        // Refresh slaves and pairing requests (only if connected)
        if connectionState == .connected {
            await refreshSlaves()
            await refreshPairingRequests()
        }
    }
    
    /// Refresh list of connected slaves
    private func refreshSlaves() async {
        guard let client else { return }
        
        do {
            let newSlaves = try await client.getSlaves()
            slaves = newSlaves
        } catch {
            logger.error("Failed to refresh slaves: \(error.localizedDescription)")
            // Don't clear slaves on error - keep showing last known state
        }
    }
    
    /// Refresh list of pending pairing requests
    private func refreshPairingRequests() async {
        guard let client else { return }
        
        do {
            let newRequests = try await client.getPendingPairingRequests()
            
            // Check if there are new requests (for notifications)
            let previousCodes = Set(pendingPairingRequests.map { $0.pairingCode })
            let newCodes = Set(newRequests.map { $0.pairingCode })
            let addedCodes = newCodes.subtracting(previousCodes)
            
            pendingPairingRequests = newRequests
            
            // Show notification for new pairing requests
            if !addedCodes.isEmpty {
                for request in newRequests where addedCodes.contains(request.pairingCode) {
                    showPairingNotification(for: request)
                }
            }
        } catch {
            logger.error("Failed to refresh pairing requests: \(error.localizedDescription)")
            // Don't clear requests on error - keep showing last known state
        }
    }
    
    /// Show a macOS notification for a new pairing request
    private func showPairingNotification(for request: PairingRequest) {
        let content = UNMutableNotificationContent()
        content.title = "New Pairing Request"
        content.body = "\(request.hostname) (\(request.platformDisplay)) wants to pair\nCode: \(request.pairingCode)"
        content.sound = .default
        content.categoryIdentifier = "PAIRING_REQUEST"
        
        let notificationRequest = UNNotificationRequest(
            identifier: "pairing-\(request.pairingCode)",
            content: content,
            trigger: nil // Deliver immediately
        )
        
        UNUserNotificationCenter.current().add(notificationRequest) { error in
            if let error = error {
                logger.error("Failed to show pairing notification: \(error.localizedDescription)")
            }
        }
    }
    
    /// Approve a pairing request
    func approvePairing(hostname: String, pairingCode: String) async {
        guard let client else { return }
        
        do {
            try await client.approvePairingRequest(hostname: hostname, pairingCode: pairingCode)
            
            // Refresh lists immediately
            await refreshSlaves()
            await refreshPairingRequests()
            
            logger.info("Approved pairing for \(hostname)")
        } catch {
            logger.error("Failed to approve pairing: \(error.localizedDescription)")
            lastError = "Failed to approve pairing: \(error.localizedDescription)"
        }
    }
    
    /// Deny a pairing request
    func denyPairing(hostname: String, pairingCode: String) async {
        guard let client else { return }
        
        do {
            try await client.denyPairingRequest(hostname: hostname, pairingCode: pairingCode)
            
            // Refresh requests immediately
            await refreshPairingRequests()
            
            logger.info("Denied pairing for \(hostname)")
        } catch {
            logger.error("Failed to deny pairing: \(error.localizedDescription)")
            lastError = "Failed to deny pairing: \(error.localizedDescription)"
        }
    }
    
    /// Reload MCP configuration
    func reloadConfig() async {
        guard let client else { return }
        isLoading = true
        defer { isLoading = false }
        
        do {
            try await client.reloadConfig()
            // Wait for reload to complete - server may be briefly unavailable
            try? await Task.sleep(nanoseconds: 1_000_000_000)
            // Retry refresh a few times since the server might still be initializing
            await refreshWithRetry(maxAttempts: 3, delayMs: 1000)
        } catch {
            lastError = error.localizedDescription
        }
    }
    
    /// Refresh with retry logic for operations that may cause temporary unavailability
    private func refreshWithRetry(maxAttempts: Int, delayMs: UInt64) async {
        guard let client else { return }
        for attempt in 1...maxAttempts {
            do {
                let newStatus = try await client.getStatus()
                status = newStatus
                connectionState = .connected
                lastError = nil
                logger.info("Refresh successful on attempt \(attempt)")
                return
            } catch {
                logger.info("Refresh attempt \(attempt)/\(maxAttempts) failed: \(error.localizedDescription)")
                if attempt < maxAttempts {
                    try? await Task.sleep(nanoseconds: delayMs * 1_000_000)
                }
            }
        }
        // All attempts failed, do a normal refresh to set appropriate state
        await refresh()
    }
    
    /// Restart an MCP server
    func restartMCPServer(name: String) async {
        guard let client else { return }
        isLoading = true
        defer { isLoading = false }
        
        do {
            try await client.restartMCPServer(name: name)
            // Wait for server to restart
            try? await Task.sleep(nanoseconds: 1_000_000_000)
            // Retry refresh since the server might still be initializing
            await refreshWithRetry(maxAttempts: 3, delayMs: 1000)
        } catch {
            lastError = error.localizedDescription
        }
    }
    
    /// Start Diane (local mode only)
    func startDiane() async {
        guard let client, !isRemoteMode else { return }
        isLoading = true
        defer { isLoading = false }
        
        do {
            try client.startDiane()
            // Wait for startup
            try? await Task.sleep(nanoseconds: 2_000_000_000)
            await refresh()
        } catch {
            lastError = error.localizedDescription
        }
    }
    
    /// Stop Diane (local mode only)
    func stopDiane() async {
        guard let client, !isRemoteMode else { return }
        isLoading = true
        defer { isLoading = false }
        
        do {
            try client.stopDiane()
            // Wait for shutdown
            try? await Task.sleep(nanoseconds: 1_000_000_000)
            await refresh()
        } catch {
            lastError = error.localizedDescription
        }
    }
    
    /// Restart Diane (local mode only)
    func restartDiane() async {
        guard let client, !isRemoteMode else { return }
        isLoading = true
        defer { isLoading = false }
        
        do {
            try await client.restartDiane()
            // Wait for startup
            try? await Task.sleep(nanoseconds: 2_000_000_000)
            await refresh()
        } catch {
            lastError = error.localizedDescription
        }
    }
    
    /// Check if Diane is running
    var isRunning: Bool {
        guard let client else { return false }
        if case .connected = connectionState {
            return true
        }
        if !isRemoteMode {
            return client.isProcessRunning()
        }
        return false
    }
    
    /// Start OAuth authentication for an MCP server
    func startAuth(serverName: String) async -> DeviceCodeInfo? {
        guard let client else { return nil }
        isLoading = true
        defer { isLoading = false }
        
        do {
            let deviceInfo = try await client.startAuth(serverName: serverName)
            return deviceInfo
        } catch {
            lastError = error.localizedDescription
            logger.error("Failed to start auth for \(serverName): \(error.localizedDescription)")
            return nil
        }
    }
    
    /// Poll for OAuth token and refresh status when complete
    func pollAuthAndRefresh(serverName: String, deviceCode: String, interval: Int) async -> Bool {
        guard let client else { return false }
        do {
            try await client.pollAuth(serverName: serverName, deviceCode: deviceCode, interval: interval)
            // Auth successful, refresh to show updated status
            await refreshWithRetry(maxAttempts: 3, delayMs: 1000)
            return true
        } catch {
            lastError = error.localizedDescription
            logger.error("Auth polling failed for \(serverName): \(error.localizedDescription)")
            return false
        }
    }
}
