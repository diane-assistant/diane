## Why

The current Diane application runs as a menu bar utility, but Diane's expanding functionality (MCPs, providers, configuration, monitoring) would be better served by a full-featured desktop application. A dedicated desktop app with standard window management would provide better discoverability, organization, and user experience as a professional tool that can be docked and managed like other native applications.

## What Changes

- Create a full desktop application with a standard window and dock icon, replacing the menu bar-only approach
- Consolidate all management interfaces (MCPs, providers, configuration, monitoring) into a unified desktop interface
- Implement native window management (minimize, maximize, close) with proper dock integration
- Provide organized navigation between different functional areas within a single application window
- Maintain the existing menu bar icon as an optional quick-access entry point that opens the main window
- Ensure the app behaves like a standard macOS application (appears in dock, Command+Tab switching, etc.)

## Capabilities

### New Capabilities

- `desktop-window-management`: Native desktop window creation, management, and dock integration for the main application window
- `unified-ui-navigation`: Centralized navigation system to browse and access all functionality (MCPs, providers, settings) within the desktop app
- `app-lifecycle-management`: Application lifecycle handling for desktop mode (launch, activate, minimize, quit, dock interaction)

### Modified Capabilities

- `menu-bar-integration`: Change from primary interface to optional quick-access launcher that opens the main desktop window

## Impact

- **Diane code**: Significant refactor from menu bar app to full desktop application
- **UI architecture**: New main window structure with navigation and content areas
- **User experience**: Changed interaction model from dropdown menu to navigable desktop application
- **Build process**: May need updated app bundle configuration for proper dock integration
- **Existing users**: **BREAKING** - Changes the fundamental UX from menu bar only to desktop app (though menu bar access can remain)
