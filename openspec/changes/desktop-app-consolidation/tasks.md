## 1. Project Setup and Structure

- [x] 1.1 Create MainWindowView.swift file in Diane/Diane/Views/
- [x] 1.2 Create Section enum within MainWindowView for navigation sections
- [x] 1.3 Add SF Symbol icon mapping for each section (tools, scheduler, agents, contexts, providers, usage, settings)
- [x] 1.4 Verify Info.plist does not have LSUIElement set to YES (should be NO or omitted for dock icon)

## 2. Main Window Scene

- [x] 2.1 Add Window scene to DianeApp.swift with id "main" and title "Diane"
- [x] 2.2 Set window default size to 1000x700 using .defaultSize() modifier
- [x] 2.3 Add MainWindowView as the window's content with environment objects (statusMonitor, updateChecker)
- [x] 2.4 Configure window to persist size and position using .setFrameAutosaveName or SceneStorage
- [ ] 2.5 Test that window appears on launch and shows in dock

## 3. Navigation Split View Structure

- [x] 3.1 Implement NavigationSplitView in MainWindowView with sidebar and detail sections
- [x] 3.2 Add @State property for selectedSection (default to .tools)
- [x] 3.3 Create sidebar List with Section.allCases displaying NavigationLinks
- [x] 3.4 Add Label with icon and text for each navigation item in sidebar
- [ ] 3.5 Verify sidebar shows all 7 sections (Tools, Scheduler, Agents, Contexts, Providers, Usage, Settings)

## 4. Detail View Router

- [x] 4.1 Create detailView(for:) function that switches based on selectedSection
- [x] 4.2 Embed ToolsBrowserView in Tools section case (remove window-specific styling)
- [x] 4.3 Embed SchedulerView in Scheduler section case
- [x] 4.4 Embed AgentsView in Agents section case
- [x] 4.5 Embed ContextsView in Contexts section case
- [x] 4.6 Embed ProvidersView in Providers section case
- [x] 4.7 Embed UsageView in Usage section case
- [x] 4.8 Port SettingsView to Settings section (remove Settings scene, embed in detail view)
- [ ] 4.9 Test each section loads correctly when selected from sidebar

## 5. State Persistence

- [x] 5.1 Add @SceneStorage for selectedSection to persist last viewed section across launches
- [x] 5.2 Verify default section is Tools on first launch
- [ ] 5.3 Test that selected section is restored when relaunching app
- [ ] 5.4 Add @SceneStorage or AppStorage for sidebar collapse state (if implementing collapsible sidebar)

## 6. Keyboard Shortcuts

- [x] 6.1 Add .keyboardShortcut("1", modifiers: .command) to Tools navigation
- [x] 6.2 Add .keyboardShortcut("2", modifiers: .command) to Scheduler navigation
- [x] 6.3 Add .keyboardShortcut("3", modifiers: .command) to Agents navigation
- [x] 6.4 Add .keyboardShortcut("4", modifiers: .command) to Contexts navigation
- [x] 6.5 Add .keyboardShortcut("5", modifiers: .command) to Providers navigation
- [x] 6.6 Add .keyboardShortcut("6", modifiers: .command) to Usage navigation
- [x] 6.7 Add .keyboardShortcut(",", modifiers: .command) to Settings navigation
- [ ] 6.8 Test all keyboard shortcuts navigate to correct sections

## 7. Window Lifecycle Management

- [x] 7.1 Implement NSApplicationDelegateAdaptor to handle window activation when app is clicked in dock
- [x] 7.2 Ensure window closes with Cmd+W but app continues running
- [ ] 7.3 Verify clicking dock icon when window is closed reopens the window
- [ ] 7.4 Verify clicking dock icon when window is minimized restores the window
- [ ] 7.5 Test Cmd+Tab activation brings window to front
- [ ] 7.6 Verify Cmd+Q quits the app completely
- [ ] 7.7 Test Cmd+M minimizes window to dock

## 8. Menu Bar Integration

- [x] 8.1 Add openMainWindow() helper function in DianeApp or shared utility
- [x] 8.2 Update MenuBarView to simplify dropdown (keep status, MCP servers, critical controls)
- [x] 8.3 Replace "Open Tools Browser", "Open Scheduler", etc. buttons with single "Open Diane" button
- [x] 8.4 Make "Open Diane" button call openMainWindow() to activate main window
- [ ] 8.5 Update menu bar icon click behavior to open main window (if not using dropdown style)
- [ ] 8.6 Keep Start/Stop/Restart/Quit buttons in menu bar dropdown
- [ ] 8.7 Keep MCP servers status list in dropdown for quick viewing
- [ ] 8.8 Test menu bar "Open Diane" button activates and brings window to front

## 9. Window Sizing and Constraints

- [x] 9.1 Set minimum window size to 800x600 using .frame(minWidth:minHeight:) or window delegate
- [ ] 9.2 Test window cannot be resized smaller than minimum dimensions
- [ ] 9.3 Verify window can be maximized/zoomed properly
- [ ] 9.4 Test window resize handles smoothly with NavigationSplitView

## 10. Environment Objects and Services

- [ ] 10.1 Verify StatusMonitor is passed to MainWindowView via .environmentObject()
- [ ] 10.2 Verify UpdateChecker is passed to MainWindowView via .environmentObject()
- [ ] 10.3 Ensure all embedded views (Tools, Scheduler, etc.) can access StatusMonitor
- [ ] 10.4 Test that status updates in main window when backend connection changes
- [ ] 10.5 Test update notifications appear correctly in main window

## 11. Polish and UX

- [x] 11.1 Add status indicator to main window toolbar or sidebar (connection state, version, uptime)
- [x] 11.2 Style NavigationSplitView sidebar with appropriate colors and spacing
- [x] 11.3 Ensure selected section has clear visual highlight in sidebar
- [ ] 11.4 Add smooth transitions when switching between sections
- [ ] 11.5 Verify all views fit properly in detail pane without awkward scrolling or clipping
- [x] 11.6 Test dark mode appearance for main window and all sections

## 12. WindowManager Deprecation (Gradual)

- [x] 12.1 Mark WindowManager methods (openToolsBrowser, openScheduler, etc.) as deprecated with warnings
- [x] 12.2 Update WindowManager methods to open main window and navigate to section instead of separate window
- [x] 12.3 Test that old window opening code paths now activate main window
- [ ] 12.4 Remove WindowManager once all references updated and tested (deferred to future if needed)

## 13. Testing and Validation

- [ ] 13.1 Test on macOS 13 (Ventura) - minimum supported version
- [ ] 13.2 Test on macOS 14 (Sonoma)
- [ ] 13.3 Test on macOS 15 (Sequoia)
- [ ] 13.4 Verify app appears in dock on all OS versions
- [ ] 13.5 Verify Cmd+Tab switcher shows app icon on all OS versions
- [ ] 13.6 Test all navigation sections load without errors
- [ ] 13.7 Test all keyboard shortcuts work correctly
- [ ] 13.8 Test window state persistence across quit/relaunch cycles
- [ ] 13.9 Test menu bar integration (open window, status display)
- [ ] 13.10 Test force quit handling (Cmd+Option+Esc)
- [ ] 13.11 Test system sleep/wake with app running
- [ ] 13.12 Perform smoke test of each feature area (Tools, Scheduler, Agents, Contexts, Providers, Usage, Settings)

## 14. Documentation and Cleanup

- [ ] 14.1 Update README.md to mention desktop app mode with dock icon
- [ ] 14.2 Add comments to MainWindowView explaining navigation structure
- [ ] 14.3 Document keyboard shortcuts in Help menu or documentation
- [ ] 14.4 Remove or comment out old WindowManager references if fully replaced
- [ ] 14.5 Update app screenshots/assets if needed for desktop mode
