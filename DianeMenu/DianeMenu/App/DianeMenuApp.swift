import SwiftUI

@main
struct DianeMenuApp: App {
    @StateObject private var statusMonitor = StatusMonitor()
    @StateObject private var updateChecker = UpdateChecker()
    @State private var hasStarted = false
    
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
        MenuBarExtra {
            MenuBarView()
                .environmentObject(statusMonitor)
                .environmentObject(updateChecker)
                .task {
                    // Start services once, with a small delay to let UI settle
                    guard !hasStarted else { return }
                    hasStarted = true
                    
                    // Wire up the status monitor reference for pausing during updates
                    updateChecker.statusMonitor = statusMonitor
                    
                    try? await Task.sleep(nanoseconds: 100_000_000) // 100ms
                    await statusMonitor.start()
                    await updateChecker.start()
                }
        } label: {
            Image(systemName: iconName)
        }
        .menuBarExtraStyle(.window)
        
        Settings {
            SettingsView()
                .environmentObject(statusMonitor)
        }
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
