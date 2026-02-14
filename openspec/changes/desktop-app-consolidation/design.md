## Context

Diane is currently a SwiftUI menu bar application that uses `MenuBarExtra` to provide a dropdown interface. The app uses `WindowManager` to open separate auxiliary windows for different features (Tools Browser, Scheduler, Agents, Contexts, Providers, Usage). This approach creates a fragmented UX where each feature opens in its own disconnected window.

The app structure:
- **DianeApp**: Main app entry using `MenuBarExtra` with `.window` style
- **MenuBarView**: Primary dropdown menu showing status and launching auxiliary windows
- **WindowManager**: Singleton managing NSWindow instances for each feature area
- **Multiple Views**: ProvidersView, AgentsView, ContextsView, etc. - each designed as standalone windows

Current constraints:
- SwiftUI app with macOS 13+ target
- Menu bar icon updates based on connection state
- StatusMonitor and UpdateChecker as environment objects
- Diane backend runs as separate Go process (MCP server)

## Goals / Non-Goals

**Goals:**
- Create a standard desktop application with dock icon and window that consolidates all functionality
- Implement unified navigation system (sidebar or tabs) to browse Tools, Scheduler, Agents, Contexts, Providers, Usage, Settings
- Maintain menu bar icon as optional quick-access that opens the main window
- Preserve existing View components with minimal changes
- Ensure proper macOS application behavior (dock presence, Cmd+Tab, window management)
- Keep backward compatibility with existing SwiftUI views and services

**Non-Goals:**
- Multi-window MDI interface (we want a single unified window)
- Redesigning individual feature views (Tools, Scheduler, etc.) - they stay as-is
- Cross-platform support beyond macOS
- Changing the Diane backend or MCP protocol

## Decisions

### 1. Application Architecture: Hybrid Menu Bar + Desktop Window

**Decision**: Transform from `MenuBarExtra`-only to dual-mode app with both menu bar and main desktop window.

**Rationale**: 
- `MenuBarExtra` becomes auxiliary quick-access launcher
- Main window uses standard `Window` scene for proper dock integration
- Allows gradual migration - menu bar stays functional

**Implementation**:
```swift
@main
struct DianeApp: App {
    var body: some Scene {
        // Primary desktop window
        Window("Diane", id: "main") {
            MainWindowView()
                .environmentObject(statusMonitor)
                .environmentObject(updateChecker)
        }
        .defaultSize(width: 1000, height: 700)
        .commands { /* custom menu bar commands */ }
        
        // Menu bar as secondary quick-access
        MenuBarExtra { /* simplified menu */ } label: { /* icon */ }
            .menuBarExtraStyle(.window)
    }
}
```

**Alternatives considered**:
- Remove menu bar entirely → Rejected: users expect menu bar for status monitoring
- Keep separate windows → Rejected: defeats consolidation purpose
- Use WindowGroup → Rejected: allows multiple instances, we want single window

### 2. Navigation Pattern: Sidebar + Detail View

**Decision**: Use NavigationSplitView with persistent sidebar containing all sections.

**Rationale**:
- Native macOS pattern (Finder, Mail, Settings)
- Sidebar can collapse for more workspace
- Natural fit for 6+ distinct sections
- SwiftUI NavigationSplitView provides automatic state management

**Implementation**:
```swift
struct MainWindowView: View {
    @State private var selectedSection: Section? = .tools
    
    enum Section: String, CaseIterable, Identifiable {
        case tools = "Tools"
        case scheduler = "Scheduler"
        case agents = "Agents"
        case contexts = "Contexts"
        case providers = "Providers"
        case usage = "Usage"
        case settings = "Settings"
        
        var id: String { rawValue }
        var icon: String { /* SF Symbol names */ }
    }
    
    var body: some View {
        NavigationSplitView {
            // Sidebar with sections
            List(Section.allCases, selection: $selectedSection) { section in
                NavigationLink(value: section) {
                    Label(section.rawValue, systemImage: section.icon)
                }
            }
        } detail: {
            // Detail view switches based on selection
            detailView(for: selectedSection)
        }
    }
}
```

**Alternatives considered**:
- TabView → Rejected: less scalable, can't show status in sidebar
- Custom split view → Rejected: reinventing wheel, lose native macOS behavior

### 3. Window Lifecycle: Single Main Window with WindowManager Deprecation

**Decision**: Replace WindowManager pattern with single main window + navigation. Keep WindowManager temporarily for menu bar popout links.

**Rationale**:
- Single main window means no window management complexity
- Existing views (ToolsBrowserView, etc.) can be embedded directly in detail view
- Menu bar can still open the main window and navigate to specific section

**Migration path**:
1. Create MainWindowView with NavigationSplitView
2. Embed existing views (ToolsBrowserView, SchedulerView, etc.) in detail pane
3. Update MenuBarView buttons to open main window + set navigation state
4. Eventually deprecate WindowManager once menu bar simplified

**Alternatives considered**:
- Keep WindowManager → Rejected: maintains fragmentation
- Immediate removal of menu bar → Rejected: too disruptive

### 4. Activation Pattern: LSUIElement Toggle

**Decision**: Use `LSUIElement = NO` in Info.plist to make app appear in dock and Cmd+Tab by default.

**Rationale**:
- Desktop app should behave like normal macOS app
- Dock presence signals it's a "real" application
- Cmd+Tab integration essential for desktop workflow
- Menu bar icon provides additional access point

**Implementation**:
- Info.plist: `LSUIElement` = `NO` (or omit entirely - default is NO)
- App automatically gets dock icon, Cmd+Tab presence
- Menu bar icon coexists with dock icon

**Alternatives considered**:
- LSUIElement = YES with manual dock badge → Rejected: complex, non-standard
- User preference toggle → Deferred: can add later if requested

### 5. Menu Bar Behavior: Open Main Window on Click

**Decision**: Menu bar icon click opens main window and brings it to front. Simplified dropdown shows only critical actions (Start/Stop, Quit).

**Rationale**:
- Reduces cognitive overhead - one place for all features
- Menu bar becomes status indicator + launcher
- Aligns with desktop app mental model

**Implementation**:
```swift
MenuBarExtra {
    VStack {
        Text("Status: \(statusMonitor.connectionState)")
        Divider()
        Button("Open Diane") { openMainWindow() }
        Button("Restart") { statusMonitor.restart() }
        Button("Quit") { NSApp.terminate(nil) }
    }
} label: { /* icon */ }
```

**Alternatives considered**:
- Full feature dropdown → Rejected: duplicates main window
- Remove menu bar → Deferred: keep for quick status access

## Risks / Trade-offs

**Risk: Breaking change for users expecting menu bar-only interface**
→ **Mitigation**: Menu bar remains functional as launcher. Announce change with migration guide. Consider adding first-run tutorial.

**Risk: Window state management complexity (position, size, last viewed section)**
→ **Mitigation**: SwiftUI's `.defaultSize()` and `NavigationSplitView` handle most automatically. Use `SceneStorage` for persisting selected section across launches.

**Risk: Performance with all views loaded in single window**
→ **Mitigation**: Detail view uses lazy loading - views only initialized when selected. Existing StatusMonitor polling unchanged.

**Risk: Sidebar taking up space on smaller screens**
→ **Mitigation**: NavigationSplitView allows sidebar collapse. Set reasonable minimum window size. Consider responsive breakpoints.

**Trade-off: Lose independent window positioning for each feature**
→ **Accept**: Users can no longer position Tools, Agents, etc. in separate spaces. Benefit of unified navigation outweighs this loss.

**Trade-off: Increased memory footprint (single process with all views)**
→ **Accept**: SwiftUI's lazy loading minimizes impact. Modern Macs have sufficient RAM. Benefit of consolidation worth marginal memory increase.

## Migration Plan

**Phase 1: Create Main Window (Week 1)**
1. Add `Window("Diane")` scene to DianeApp
2. Create MainWindowView with NavigationSplitView structure
3. Create Section enum and basic sidebar
4. Test window appearance, dock icon, Cmd+Tab

**Phase 2: Embed Existing Views (Week 1-2)**
1. Create detail view router switching on Section
2. Embed ToolsBrowserView in Tools section (remove window chrome)
3. Embed SchedulerView, AgentsView, ContextsView, ProvidersView, UsageView similarly
4. Port SettingsView to detail pane
5. Test each view works within main window

**Phase 3: Update Menu Bar Integration (Week 2)**
1. Add openMainWindow() helper using NSApp.keyWindow
2. Update MenuBarView button actions to call openMainWindow() + navigation
3. Simplify MenuBarView to status + quick actions only
4. Test menu bar → main window flow

**Phase 4: Polish & Testing (Week 2-3)**
1. Add SceneStorage for persisting selected section
2. Test window state preservation across quit/launch
3. Verify StatusMonitor and UpdateChecker work in main window
4. Add keyboard shortcuts (Cmd+1-7 for sections)
5. Update app icon if needed
6. Test on macOS 13, 14, 15

**Rollback strategy**: 
- Keep WindowManager code intact initially
- Feature flag to toggle between old (WindowManager) and new (MainWindow) modes
- If critical issues, revert to MenuBarExtra-only by hiding Window scene

## Open Questions

1. **Should we add a preference to hide dock icon?** 
   - Deferred: Ship without, gauge user feedback
   
2. **Should status indicator live in sidebar or toolbar?**
   - Propose: Toolbar above sidebar for visibility
   
3. **Do we need window state restoration for multi-window scenarios?**
   - No: We're explicitly moving to single window
   
4. **Should menu bar dropdown still show all MCP servers?**
   - Propose: Yes, for quick status check without opening main window
