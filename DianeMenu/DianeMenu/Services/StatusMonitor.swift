import Foundation
import Combine
import os.log

private let logger = Logger(subsystem: "com.diane.DianeMenu", category: "StatusMonitor")

/// Observable object that monitors Diane status
@MainActor
class StatusMonitor: ObservableObject {
    @Published var status: DianeStatus = .empty
    @Published var connectionState: ConnectionState = .unknown
    @Published var isLoading: Bool = false
    @Published var lastError: String?
    
    private let client = DianeClient()
    private var pollTimer: Timer?
    private var pollInterval: TimeInterval = 5.0
    private var hasStarted = false
    
    init() {
        logger.info("StatusMonitor initialized")
    }
    
    deinit {
        let timer = pollTimer
        DispatchQueue.main.async {
            timer?.invalidate()
        }
    }
    
    /// Call this after the app has fully initialized
    func start() async {
        guard !hasStarted else { 
            logger.info("StatusMonitor already started, skipping")
            return 
        }
        hasStarted = true
        logger.info("StatusMonitor starting...")
        
        await startPolling()
        
        // Auto-start Diane if enabled and not already running
        await autoStartIfNeeded()
    }
    
    /// Auto-start Diane if the setting is enabled
    private func autoStartIfNeeded() async {
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
        let socketExists = client.socketExists
        let processRunning = client.isProcessRunning()
        logger.info("Refresh: socketExists=\(socketExists), processRunning=\(processRunning)")
        
        // Quick check if socket exists
        guard socketExists || processRunning else {
            logger.info("No socket and no process, setting disconnected")
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
        } catch {
            logger.error("Status refresh failed: \(error.localizedDescription)")
            // Fallback: check if process is running via PID
            if client.isProcessRunning() {
                logger.warning("Process running but API failed, setting error state")
                connectionState = .error("API unavailable")
            } else {
                logger.info("Process not running, setting disconnected")
                connectionState = .disconnected
                status = .empty
            }
            lastError = error.localizedDescription
        }
    }
    
    /// Reload MCP configuration
    func reloadConfig() async {
        isLoading = true
        defer { isLoading = false }
        
        do {
            try await client.reloadConfig()
            await refresh()
        } catch {
            lastError = error.localizedDescription
        }
    }
    
    /// Restart an MCP server
    func restartMCPServer(name: String) async {
        isLoading = true
        defer { isLoading = false }
        
        do {
            try await client.restartMCPServer(name: name)
            // Wait a bit for server to restart
            try? await Task.sleep(nanoseconds: 500_000_000)
            await refresh()
        } catch {
            lastError = error.localizedDescription
        }
    }
    
    /// Start Diane
    func startDiane() async {
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
    
    /// Stop Diane
    func stopDiane() async {
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
    
    /// Restart Diane
    func restartDiane() async {
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
        if case .connected = connectionState {
            return true
        }
        return client.isProcessRunning()
    }
}
