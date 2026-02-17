import SwiftUI
import AppKit

/// Main window view with sidebar navigation and detail view
/// 
/// This consolidates all functionality (MCP Servers, Scheduler, Agents, Contexts, Providers, Usage, Settings)
/// into a single unified desktop application interface.
///
/// ## Navigation Structure
/// - **Sidebar**: List of 7 sections with icons and labels
/// - **Detail View**: Dynamically switches based on selected section
/// - **Persistence**: Selected section is saved and restored across app launches
///
/// ## Keyboard Shortcuts
/// - Cmd+1: MCP Servers
/// - Cmd+2: Scheduler
/// - Cmd+3: Agents
/// - Cmd+4: Contexts
/// - Cmd+5: Providers
/// - Cmd+6: Usage
/// - Cmd+,: Settings
///
/// ## Features
/// - Status indicator showing connection state in toolbar
/// - Start/Restart controls in toolbar
/// - Minimum window size: 800x600
/// - Default window size: 1000x700
/// - Automatic window position/size persistence
struct MainWindowView: View {
    @EnvironmentObject var statusMonitor: StatusMonitor
    @EnvironmentObject var updateChecker: UpdateChecker
    
    /// Navigation section enum defining all available sections in the sidebar
    enum Section: String, CaseIterable, Identifiable {
        case mcpRegistry = "MCP Registry"
        case mcpServers = "MCP Servers"
        case scheduler = "Scheduler"
        case agents = "Agents"
        case contexts = "Contexts"
        case providers = "Providers"
        case usage = "Usage"
        case settings = "Settings"
        
        var id: String { rawValue }
        
        /// SF Symbol icon for each section
        var icon: String {
            switch self {
            case .mcpRegistry:
                return "book.closed"
            case .mcpServers:
                return "server.rack"
            case .scheduler:
                return "calendar.badge.clock"
            case .agents:
                return "person.3.fill"
            case .contexts:
                return "square.stack.3d.up"
            case .providers:
                return "cpu"
            case .usage:
                return "chart.bar.doc.horizontal"
            case .settings:
                return "gear"
            }
        }
    }
    
    /// Selected section (persisted across launches)
    @SceneStorage("selectedSection") private var selectedSection: Section = .mcpServers
    
    var body: some View {
        NavigationSplitView {
            // Sidebar with navigation sections
            List(Section.allCases, selection: $selectedSection) { section in
                NavigationLink(value: section) {
                    Label(section.rawValue, systemImage: section.icon)
                }
                .keyboardShortcut(shortcutKey(for: section), modifiers: .command)
            }
            .navigationTitle("Diane")
            .listStyle(.sidebar)
            .toolbar {
                ToolbarItem(placement: .status) {
                    statusIndicator
                }
            }
        } detail: {
            // Detail view based on selected section
            detailView(for: selectedSection)
                .toolbar {
                    ToolbarItem(placement: .automatic) {
                        toolbarControls
                    }
                }
        }
    }
    
    /// Status indicator showing connection state
    private var statusIndicator: some View {
        HStack(spacing: 6) {
            Circle()
                .fill(statusColor)
                .frame(width: 8, height: 8)
            
            Text(statusMonitor.connectionState.description)
                .font(.caption)
                .foregroundStyle(.secondary)
        }
    }
    
    /// Toolbar controls for the detail view
    private var toolbarControls: some View {
        HStack(spacing: 8) {
            // Connection status indicator (always visible)
            serverStatusView
            
            // Local server controls (start/restart)
            if !statusMonitor.isRemoteMode {
                if statusMonitor.isLoading {
                    ProgressView()
                        .scaleEffect(0.6)
                } else if case .connected = statusMonitor.connectionState {
                    Button {
                        Task { await statusMonitor.restartDiane() }
                    } label: {
                        Label("Restart", systemImage: "arrow.clockwise")
                    }
                    .help("Restart Diane")
                } else {
                    Button {
                        Task { await statusMonitor.startDiane() }
                    } label: {
                        Label("Start", systemImage: "play.fill")
                    }
                    .help("Start Diane")
                }
            }
        }
    }
    
    /// Server status indicator with server list menu
    @State private var showServerList = false
    
    private var serverStatusView: some View {
        HStack(spacing: 6) {
            Circle()
                .fill(statusColor)
                .frame(width: 8, height: 8)
            
            Text(statusMonitor.serverDisplayName)
                .font(.caption)
                .foregroundStyle(.primary)
        }
        .padding(.horizontal, 6)
        .padding(.vertical, 2)
        .contentShape(Rectangle())
        .onTapGesture {
            showServerList.toggle()
        }
        .popover(isPresented: $showServerList) {
            ServerListPopover(
                statusMonitor: statusMonitor,
                onClose: { showServerList = false }
            )
        }
        .help("Connected to \(statusMonitor.serverDisplayName)")
    }
    
    /// Returns the color for the current connection state
    private var statusColor: Color {
        switch statusMonitor.connectionState {
        case .unknown:
            return .gray
        case .connected:
            return .green
        case .disconnected:
            return .red
        case .error:
            return .red
        }
    }
    
    /// Returns the detail view for the given section
    @ViewBuilder
    private func detailView(for section: Section?) -> some View {
        switch section {
        case .mcpRegistry:
            MCPRegistryView()
        case .mcpServers:
            MCPServersView()
        case .scheduler:
            SchedulerView()
        case .agents:
            AgentsView()
        case .contexts:
            ContextsView()
        case .providers:
            ProvidersView()
        case .usage:
            UsageView()
        case .settings:
            SettingsView()
        case nil:
            Text("Select a section")
                .foregroundStyle(.secondary)
        }
    }
    
    /// Returns keyboard shortcut key for each section
    private func shortcutKey(for section: Section) -> KeyEquivalent {
        switch section {
        case .mcpRegistry: return "0"
        case .mcpServers: return "1"
        case .scheduler: return "2"
        case .agents: return "3"
        case .contexts: return "4"
        case .providers: return "5"
        case .usage: return "6"
        case .settings: return ","
        }
    }
    
    /// Opens or brings the main window to the front
    /// This is a helper function for menu bar integration and external calls
    static func openMainWindow() {
        // Activate the app (brings to front)
        NSApp.activate(ignoringOtherApps: true)
        
        // Find any existing window and show it
        for window in NSApp.windows {
            // Look for the main content window (not menu bar popover)
            if window.title == "Diane" || window.identifier?.rawValue.contains("main") == true {
                window.makeKeyAndOrderFront(nil)
                window.orderFrontRegardless()
                return
            }
        }
        
        // If we get here, no window was found
        // This can happen if the window was fully closed
        // The only way to reopen it in SwiftUI is to use the app's openWindow environment
        // Since we can't access that from a static method, we need to use a notification
        NotificationCenter.default.post(name: NSNotification.Name("OpenMainWindow"), object: nil)
    }
}

#Preview {
    MainWindowView()
        .environmentObject(StatusMonitor())
        .environmentObject(UpdateChecker())
}
