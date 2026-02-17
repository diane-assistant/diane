import SwiftUI
import UserNotifications

/// App delegate to handle window activation, lifecycle events, and notification actions
class AppDelegate: NSObject, NSApplicationDelegate, UNUserNotificationCenterDelegate {
    // Reference to status monitor for handling notification actions
    var statusMonitor: StatusMonitor?
    
    func applicationDidFinishLaunching(_ notification: Notification) {
        // Set up notification delegate
        UNUserNotificationCenter.current().delegate = self
        
        // Register notification categories with actions
        registerNotificationCategories()
        
        // Request notification permissions for pairing requests
        UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .sound, .badge]) { granted, error in
            if let error = error {
                print("Error requesting notification permissions: \(error)")
            } else if granted {
                print("Notification permissions granted")
            }
        }
        
        // Ensure the main window activates on launch
        NSApp.activate(ignoringOtherApps: true)
    }
    
    /// Register notification categories with action buttons
    private func registerNotificationCategories() {
        let approveAction = UNNotificationAction(
            identifier: "APPROVE_PAIRING",
            title: "Accept",
            options: [.authenticationRequired]
        )
        
        let denyAction = UNNotificationAction(
            identifier: "DENY_PAIRING",
            title: "Deny",
            options: [.destructive, .authenticationRequired]
        )
        
        let pairingCategory = UNNotificationCategory(
            identifier: "PAIRING_REQUEST",
            actions: [approveAction, denyAction],
            intentIdentifiers: [],
            options: [.customDismissAction]
        )
        
        UNUserNotificationCenter.current().setNotificationCategories([pairingCategory])
    }
    
    // MARK: - UNUserNotificationCenterDelegate
    
    /// Handle notification actions (Accept/Deny buttons)
    func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        didReceive response: UNNotificationResponse,
        withCompletionHandler completionHandler: @escaping () -> Void
    ) {
        let userInfo = response.notification.request.content.userInfo
        let hostname = userInfo["hostname"] as? String ?? ""
        let pairingCode = userInfo["pairing_code"] as? String ?? ""
        
        switch response.actionIdentifier {
        case "APPROVE_PAIRING":
            Task { @MainActor in
                await statusMonitor?.approvePairing(hostname: hostname, pairingCode: pairingCode)
            }
        case "DENY_PAIRING":
            Task { @MainActor in
                await statusMonitor?.denyPairing(hostname: hostname, pairingCode: pairingCode)
            }
        case UNNotificationDefaultActionIdentifier:
            // User tapped the notification itself - open the settings to show pending requests
            DispatchQueue.main.async {
                NSApp.activate(ignoringOtherApps: true)
                // Open settings window
                if #available(macOS 14.0, *) {
                    NSApp.sendAction(Selector(("showSettingsWindow:")), to: nil, from: nil)
                } else {
                    NSApp.sendAction(Selector(("showPreferencesWindow:")), to: nil, from: nil)
                }
            }
        default:
            break
        }
        
        completionHandler()
    }
    
    /// Show notifications even when app is in foreground
    func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        willPresent notification: UNNotification,
        withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void
    ) {
        // Show banner and play sound even when app is in foreground
        completionHandler([.banner, .sound, .badge])
    }
    
    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows flag: Bool) -> Bool {
        // When clicking dock icon, show the main window
        if !flag {
            // No visible windows, open the main window
            MainWindowView.openMainWindow()
        }
        return true
    }
    
    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        // Keep the app running (menu bar stays visible) when main window is closed
        // This is essential for menu bar apps
        return false
    }
}

@main
struct DianeApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) var appDelegate
    @StateObject private var statusMonitor = StatusMonitor()
    @StateObject private var updateChecker = UpdateChecker()
    @State private var serverConfig = ServerConfiguration()
    @State private var hasStarted = false
    @Environment(\.openWindow) private var openWindow
    
    private var iconName: String {
        switch statusMonitor.connectionState {
        case .unknown, .disconnected:
            return "recordingtape.circle"
        case .connected:
            return "recordingtape.circle.fill"
        case .error:
            return "exclamationmark.circle.fill"
        }
    }
    
    var body: some Scene {
        // Primary desktop window
        Window("Diane", id: "main") {
            Group {
                if serverConfig.isConfigured {
                    MainWindowView()
                        .environmentObject(statusMonitor)
                        .environmentObject(updateChecker)
                        .environment(serverConfig)
                        .frame(minWidth: 800, minHeight: 600)
                        .task {
                            await startServicesIfNeeded()
                        }
                        .onReceive(NotificationCenter.default.publisher(for: NSNotification.Name("OpenMainWindow"))) { _ in
                            NSApp.activate(ignoringOtherApps: true)
                        }
                } else {
                    MacServerSetupView(config: serverConfig) {
                        configureAndStart()
                    }
                }
            }
        }
        .defaultSize(width: 1000, height: 700)
        .commands {
            // Replace Cmd+Q to close window instead of quitting
            CommandGroup(replacing: .appTermination) {
                Button("Close Window") {
                    // Close the main window but keep app running
                    if let window = NSApp.windows.first(where: { $0.title == "Diane" }) {
                        window.close()
                    }
                }
                .keyboardShortcut("q", modifiers: .command)
                
                Divider()
                
                Button("Quit Diane") {
                    exit(0)
                }
                .keyboardShortcut("q", modifiers: [.command, .option])
            }
        }
        
        // Menu bar as secondary quick-access
        MenuBarExtra {
            MenuBarView()
                .environmentObject(statusMonitor)
                .environmentObject(updateChecker)
                .task {
                    await startServicesIfNeeded()
                }
        } label: {
            Image(systemName: iconName)
                .task {
                    // This task runs when the label appears (at app launch, not menu open)
                    await startServicesIfNeeded()
                }
        }
        .menuBarExtraStyle(.window)
        
        // Settings window (opened via SettingsLink or Cmd+,)
        Settings {
            SettingsView()
                .environmentObject(statusMonitor)
                .environmentObject(updateChecker)
                .environment(serverConfig)
                .frame(minWidth: 450, minHeight: 300)
        }
    }
    
    /// Called when the user completes the setup flow
    private func configureAndStart() {
        statusMonitor.configure(from: serverConfig)
        Task {
            await startServicesIfNeeded()
        }
    }
    
    @MainActor
    private func startServicesIfNeeded() async {
        // Don't start until configured
        guard serverConfig.isConfigured else { return }
        guard !hasStarted else { return }
        hasStarted = true
        
        // Wire up the app delegate with the status monitor for notification actions
        appDelegate.statusMonitor = statusMonitor
        
        // Configure the status monitor from server config if not already done
        statusMonitor.configure(from: serverConfig)
        
        // Wire up the status monitor reference for pausing during updates
        updateChecker.statusMonitor = statusMonitor
        
        try? await Task.sleep(nanoseconds: 100_000_000) // 100ms
        await statusMonitor.start()
        await updateChecker.start()
    }
}

/// Menu bar icon that changes based on connection state
/// Uses SF Symbols for reliable rendering in menu bar
struct MenuBarIcon: View {
    let connectionState: ConnectionState
    let updateAvailable: Bool
    
    var body: some View {
        HStack(spacing: 2) {
            Image(systemName: iconName)
                .symbolRenderingMode(.hierarchical)
                .foregroundStyle(iconColor)
            
            // Show update badge
            if updateAvailable {
                Image(systemName: "arrow.up.circle.fill")
                    .font(.system(size: 9))
                    .foregroundStyle(.orange)
            }
        }
    }
    
    private var iconName: String {
        switch connectionState {
        case .unknown:
            return "recordingtape.circle"
        case .connected:
            return "recordingtape.circle.fill"
        case .disconnected:
            return "recordingtape.circle"
        case .error:
            return "exclamationmark.circle.fill"
        }
    }
    
    private var iconColor: Color {
        switch connectionState {
        case .unknown:
            return .secondary
        case .connected:
            return .primary
        case .disconnected:
            return .secondary
        case .error:
            return .orange
        }
    }
}

/// Fallback icon using SF Symbols if custom icon isn't available
struct MenuBarIconFallback: View {
    let connectionState: ConnectionState
    
    var body: some View {
        Image(systemName: iconName)
            .symbolRenderingMode(.palette)
            .foregroundStyle(iconColor, .primary)
    }
    
    private var iconName: String {
        switch connectionState {
        case .unknown:
            return "waveform.circle"
        case .connected:
            return "waveform.circle.fill"
        case .disconnected:
            return "waveform.circle"
        case .error:
            return "exclamationmark.circle.fill"
        }
    }
    
    private var iconColor: Color {
        switch connectionState {
        case .unknown:
            return .secondary
        case .connected:
            return .green
        case .disconnected:
            return .secondary
        case .error:
            return .orange
        }
    }
}
