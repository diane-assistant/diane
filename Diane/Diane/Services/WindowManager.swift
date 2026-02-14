import SwiftUI
import AppKit

/// Manages auxiliary windows for the menu bar app
/// NOTE: This is deprecated in favor of the unified main window (MainWindowView).
/// Use MainWindowView.openMainWindow() instead to open the main application window.
@MainActor
class WindowManager {
    static let shared = WindowManager()
    
    private var toolsBrowserWindow: NSWindow?
    private var schedulerWindow: NSWindow?
    private var agentsWindow: NSWindow?
    private var contextsWindow: NSWindow?
    private var providersWindow: NSWindow?
    private var usageWindow: NSWindow?
    
    private init() {}
    
    /// Open the Tools Browser window
    /// - Warning: Deprecated. Use MainWindowView.openMainWindow() instead to open the main window with all features.
    @available(*, deprecated, message: "Use MainWindowView.openMainWindow() instead")
    func openToolsBrowser() {
        // Open the unified main window instead
        MainWindowView.openMainWindow()
    }
    
    /// Close the Tools Browser window
    @available(*, deprecated, message: "No longer needed with unified main window")
    func closeToolsBrowser() {
        toolsBrowserWindow?.close()
    }
    
    /// Open the Scheduler window
    /// - Warning: Deprecated. Use MainWindowView.openMainWindow() instead to open the main window with all features.
    @available(*, deprecated, message: "Use MainWindowView.openMainWindow() instead")
    func openScheduler() {
        // Open the unified main window instead
        MainWindowView.openMainWindow()
    }
    
    /// Close the Scheduler window
    @available(*, deprecated, message: "No longer needed with unified main window")
    func closeScheduler() {
        schedulerWindow?.close()
    }
    
    /// Open the Agents window
    /// - Warning: Deprecated. Use MainWindowView.openMainWindow() instead to open the main window with all features.
    @available(*, deprecated, message: "Use MainWindowView.openMainWindow() instead")
    func openAgents() {
        // Open the unified main window instead
        MainWindowView.openMainWindow()
    }
    
    /// Close the Agents window
    @available(*, deprecated, message: "No longer needed with unified main window")
    func closeAgents() {
        agentsWindow?.close()
    }
    
    /// Open the Contexts window
    /// - Warning: Deprecated. Use MainWindowView.openMainWindow() instead to open the main window with all features.
    @available(*, deprecated, message: "Use MainWindowView.openMainWindow() instead")
    func openContexts() {
        // Open the unified main window instead
        MainWindowView.openMainWindow()
    }
    
    /// Close the Contexts window
    @available(*, deprecated, message: "No longer needed with unified main window")
    func closeContexts() {
        contextsWindow?.close()
    }
    
    /// Open the Providers window
    /// - Warning: Deprecated. Use MainWindowView.openMainWindow() instead to open the main window with all features.
    @available(*, deprecated, message: "Use MainWindowView.openMainWindow() instead")
    func openProviders() {
        // Open the unified main window instead
        MainWindowView.openMainWindow()
    }
    
    /// Close the Providers window
    @available(*, deprecated, message: "No longer needed with unified main window")
    func closeProviders() {
        providersWindow?.close()
    }
    
    /// Open the Usage window
    /// - Warning: Deprecated. Use MainWindowView.openMainWindow() instead to open the main window with all features.
    @available(*, deprecated, message: "Use MainWindowView.openMainWindow() instead")
    func openUsage() {
        // Open the unified main window instead
        MainWindowView.openMainWindow()
    }
    
    /// Close the Usage window
    @available(*, deprecated, message: "No longer needed with unified main window")
    func closeUsage() {
        usageWindow?.close()
    }
}
