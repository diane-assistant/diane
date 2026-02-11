## ADDED Requirements

### Requirement: Application launches properly
The application SHALL launch successfully and display the main window on startup.

#### Scenario: First launch shows welcome state
- **WHEN** user launches the application for the first time
- **THEN** the main window appears centered with default settings

#### Scenario: Subsequent launches restore state
- **WHEN** user launches the application after previous use
- **THEN** the window appears with saved size, position, and last viewed section

### Requirement: Application remains running when window is closed
The application SHALL continue running in the background when the main window is closed.

#### Scenario: Closing window does not quit application
- **WHEN** user closes the main window using the close button or Cmd+W
- **THEN** the application remains running and accessible from the dock and menu bar

#### Scenario: Application is still visible in dock after window close
- **WHEN** the main window is closed
- **THEN** the dock icon remains visible indicating the application is still running

### Requirement: Application activates when focused
The application SHALL bring the main window to the front when activated.

#### Scenario: Clicking dock icon activates window
- **WHEN** user clicks the dock icon while the main window is hidden or minimized
- **THEN** the main window becomes visible and comes to the front

#### Scenario: Clicking dock icon when window is already visible
- **WHEN** user clicks the dock icon while the main window is already visible
- **THEN** the window remains in front and gains focus

#### Scenario: Command-Tab activation shows window
- **WHEN** user switches to the application using Command-Tab
- **THEN** the main window becomes visible and comes to the front

### Requirement: Application quits cleanly
The application SHALL save state and terminate properly when quit.

#### Scenario: Cmd+Q quits the application
- **WHEN** user presses Command+Q
- **THEN** the application saves window state and terminates completely

#### Scenario: Quit from dock menu quits application
- **WHEN** user selects "Quit" from the dock icon context menu
- **THEN** the application saves window state and terminates completely

#### Scenario: State is saved on quit
- **WHEN** the application quits
- **THEN** window size, position, sidebar state, and current section are saved

### Requirement: Application integrates with system menu bar
The application SHALL provide a standard macOS menu bar with appropriate menu items.

#### Scenario: Application menu is present
- **WHEN** the application is active
- **THEN** the system menu bar shows "Diane" as the application menu with standard items (About, Preferences, Quit)

#### Scenario: File menu provides window controls
- **WHEN** the application is active
- **THEN** the File menu includes standard items like "Close Window" (Cmd+W)

#### Scenario: Window menu provides window management
- **WHEN** the application is active
- **THEN** the Window menu includes items like "Minimize" (Cmd+M) and "Zoom"

#### Scenario: Help menu is available
- **WHEN** the application is active
- **THEN** the Help menu provides access to documentation and support

### Requirement: Application handles multiple activation attempts
The application SHALL handle being launched when already running.

#### Scenario: Launching when already running activates window
- **WHEN** user attempts to launch the application while it is already running
- **THEN** the existing instance activates and the main window comes to the front

#### Scenario: No duplicate instances are created
- **WHEN** user attempts to launch the application multiple times
- **THEN** only one instance of the application runs

### Requirement: Application respects system sleep and wake
The application SHALL handle system sleep and wake events appropriately.

#### Scenario: Application continues after system wake
- **WHEN** the system wakes from sleep
- **THEN** the application remains running and functional

#### Scenario: Window state preserved after system wake
- **WHEN** the system wakes from sleep
- **THEN** the main window maintains its previous state (open/closed, position, size)

### Requirement: Application handles minimize to dock
The application SHALL support minimizing the window to the dock.

#### Scenario: Window minimizes on Cmd+M
- **WHEN** user presses Command+M
- **THEN** the main window minimizes to the dock

#### Scenario: Clicking minimized window in dock restores it
- **WHEN** user clicks the minimized window icon in the dock
- **THEN** the window restores to its previous size and position

### Requirement: Application supports Force Quit
The application SHALL terminate when force quit by the system.

#### Scenario: Force Quit terminates application
- **WHEN** user force quits the application via Activity Monitor or Cmd+Option+Esc
- **THEN** the application terminates immediately without saving state
