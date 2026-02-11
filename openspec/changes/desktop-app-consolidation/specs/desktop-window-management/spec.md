## ADDED Requirements

### Requirement: Main desktop window exists
The application SHALL provide a primary desktop window that serves as the main interface for all functionality.

#### Scenario: Application launches with main window visible
- **WHEN** user launches the application for the first time
- **THEN** the main desktop window appears at the center of the screen with default dimensions

#### Scenario: Window persists size and position
- **WHEN** user resizes or moves the main window and then quits the application
- **THEN** the window reopens at the same size and position on next launch

### Requirement: Standard window controls are functional
The main window SHALL support standard macOS window controls including minimize, maximize/zoom, and close.

#### Scenario: User minimizes window
- **WHEN** user clicks the minimize button or presses Cmd+M
- **THEN** the window minimizes to the dock

#### Scenario: User closes window
- **WHEN** user clicks the close button or presses Cmd+W
- **THEN** the window closes but the application remains running (accessible from dock or menu bar)

#### Scenario: User maximizes window
- **WHEN** user clicks the maximize/zoom button
- **THEN** the window resizes to fill the available screen space

### Requirement: Dock icon is present and functional
The application SHALL appear in the macOS dock with an appropriate icon.

#### Scenario: Application shows in dock
- **WHEN** the application is running
- **THEN** the application icon is visible in the dock

#### Scenario: Clicking dock icon activates window
- **WHEN** user clicks the dock icon
- **THEN** the main window becomes active and comes to the front

#### Scenario: Right-click dock icon shows menu
- **WHEN** user right-clicks the dock icon
- **THEN** the system shows a context menu with standard application actions

### Requirement: Command-Tab integration works
The application SHALL appear in the macOS Command-Tab application switcher.

#### Scenario: Application appears in switcher
- **WHEN** user presses Command-Tab while the application is running
- **THEN** the application icon appears in the application switcher

#### Scenario: Switching to application via Command-Tab
- **WHEN** user selects the application in Command-Tab switcher
- **THEN** the application activates and the main window comes to the front

### Requirement: Window has reasonable default and minimum sizes
The main window SHALL have sensible default dimensions and enforce minimum size constraints.

#### Scenario: Default window size is appropriate
- **WHEN** application launches with no saved window state
- **THEN** the window opens with dimensions of 1000x700 pixels

#### Scenario: Window cannot be made too small
- **WHEN** user attempts to resize the window below minimum dimensions
- **THEN** the window stops resizing at 800x600 pixels minimum

### Requirement: Window title reflects application state
The main window title SHALL display "Diane" as the application name.

#### Scenario: Window shows application name
- **WHEN** the main window is visible
- **THEN** the window title bar displays "Diane"
