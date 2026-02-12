## ADDED Requirements

### Requirement: Layout parameter controls in trailing panel
The trailing panel SHALL expose interactive controls for key layout parameters that update the rendered preview in real time without recompilation.

#### Scenario: Controls appear for selected component
- **WHEN** a view or component is selected in the catalog
- **THEN** the trailing panel displays relevant layout controls below the state selector

#### Scenario: Real-time preview update
- **WHEN** the user adjusts any control (slider, stepper, picker)
- **THEN** the preview updates immediately without any build or compile step

### Requirement: Spacing and padding controls
The controls panel SHALL include sliders for spacing and padding values that affect the rendered component.

#### Scenario: Global spacing control
- **WHEN** the user adjusts a "Spacing" slider
- **THEN** the VStack/HStack spacing within the rendered component changes to the slider's value

#### Scenario: Padding control
- **WHEN** the user adjusts a "Padding" slider
- **THEN** the outer padding of the rendered component changes to the slider's value

#### Scenario: Value display
- **WHEN** a slider is at a given position
- **THEN** the current numeric value (in points) is displayed next to the slider

### Requirement: Font size controls
The controls panel SHALL include a stepper or slider for adjusting font sizes used in the rendered component.

#### Scenario: Font size adjustment
- **WHEN** the user adjusts the font size control
- **THEN** text elements in the rendered component update to the new size

#### Scenario: Font size range
- **WHEN** the font size control is displayed
- **THEN** it allows values from 8 to 32 points with 1-point increments

### Requirement: Color controls
The controls panel SHALL include color pickers for key color values (accent color, background, text color).

#### Scenario: Accent color change
- **WHEN** the user selects a new accent color
- **THEN** interactive elements (buttons, toggles, selections) in the preview update to the new color

#### Scenario: Background color change
- **WHEN** the user selects a new background color
- **THEN** the component's background changes to the selected color

### Requirement: Corner radius control
The controls panel SHALL include a slider for adjusting corner radii on bordered elements.

#### Scenario: Corner radius adjustment
- **WHEN** the user adjusts the corner radius slider
- **THEN** rounded elements in the rendered component update their corner radius

### Requirement: Preview size controls
The trailing panel SHALL include width and height fields for setting the preview canvas dimensions.

#### Scenario: Width adjustment
- **WHEN** the user changes the width value
- **THEN** the preview canvas width updates and the component re-renders at the new width

#### Scenario: Height adjustment
- **WHEN** the user changes the height value
- **THEN** the preview canvas height updates and the component re-renders at the new height

#### Scenario: Size presets
- **WHEN** the user clicks a size preset button (e.g., "Compact", "Default", "Wide")
- **THEN** the width and height fields update to the preset values and the preview re-renders

### Requirement: Controls use environment-based layout tokens
Layout parameter controls SHALL apply values via SwiftUI environment or a shared observable object, so components read them reactively without prop drilling.

#### Scenario: Environment-based propagation
- **WHEN** a layout parameter is changed
- **THEN** it propagates through the SwiftUI environment to all child views of the preview

#### Scenario: No source code modification required
- **WHEN** the user adjusts controls
- **THEN** no files are written or modified — all changes are ephemeral within the running catalog session
