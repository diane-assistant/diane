import SwiftUI
import AppKit

/// Manages auxiliary windows for the menu bar app
class WindowManager {
    static let shared = WindowManager()
    
    private var toolsBrowserWindow: NSWindow?
    private var schedulerWindow: NSWindow?
    private var agentsWindow: NSWindow?
    
    private init() {}
    
    /// Open the Tools Browser window
    func openToolsBrowser() {
        // If window exists and is visible, just bring it to front
        if let window = toolsBrowserWindow, window.isVisible {
            window.makeKeyAndOrderFront(nil)
            NSApp.activate(ignoringOtherApps: true)
            return
        }
        
        // Create new window
        let contentView = ToolsBrowserView()
        
        let window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 600, height: 500),
            styleMask: [.titled, .closable, .resizable, .miniaturizable],
            backing: .buffered,
            defer: false
        )
        
        window.title = "Tools Browser"
        window.contentView = NSHostingView(rootView: contentView)
        window.center()
        window.setFrameAutosaveName("ToolsBrowser")
        window.isReleasedWhenClosed = false
        window.minSize = NSSize(width: 400, height: 300)
        
        toolsBrowserWindow = window
        
        window.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }
    
    /// Close the Tools Browser window
    func closeToolsBrowser() {
        toolsBrowserWindow?.close()
    }
    
    /// Open the Scheduler window
    func openScheduler() {
        // If window exists and is visible, just bring it to front
        if let window = schedulerWindow, window.isVisible {
            window.makeKeyAndOrderFront(nil)
            NSApp.activate(ignoringOtherApps: true)
            return
        }
        
        // Create new window
        let contentView = SchedulerView()
        
        let window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 900, height: 600),
            styleMask: [.titled, .closable, .resizable, .miniaturizable],
            backing: .buffered,
            defer: false
        )
        
        window.title = "Scheduler"
        window.contentView = NSHostingView(rootView: contentView)
        window.center()
        window.setFrameAutosaveName("Scheduler")
        window.isReleasedWhenClosed = false
        window.minSize = NSSize(width: 600, height: 400)
        
        schedulerWindow = window
        
        window.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }
    
    /// Close the Scheduler window
    func closeScheduler() {
        schedulerWindow?.close()
    }
    
    /// Open the Agents window
    func openAgents() {
        // If window exists and is visible, just bring it to front
        if let window = agentsWindow, window.isVisible {
            window.makeKeyAndOrderFront(nil)
            NSApp.activate(ignoringOtherApps: true)
            return
        }
        
        // Create new window
        let contentView = AgentsView()
        
        let window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 950, height: 650),
            styleMask: [.titled, .closable, .resizable, .miniaturizable],
            backing: .buffered,
            defer: false
        )
        
        window.title = "ACP Agents"
        window.contentView = NSHostingView(rootView: contentView)
        window.center()
        window.setFrameAutosaveName("Agents")
        window.isReleasedWhenClosed = false
        window.minSize = NSSize(width: 750, height: 450)
        
        agentsWindow = window
        
        window.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }
    
    /// Close the Agents window
    func closeAgents() {
        agentsWindow?.close()
    }
}
