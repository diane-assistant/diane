## ADDED Requirements

### Requirement: List workspace images (built-in and custom)

The system SHALL display all workspace images available for the current project via `GET /api/admin/workspace-images`. The list SHALL show both built-in images and custom (user-registered) images. Each row SHALL show: name, provider, type (built-in or custom), status, and docker_ref if present.

#### Scenario: Navigating to workspace images
- **WHEN** the user opens the "Workspace Images" section
- **THEN** the system MUST fetch and display all images in a list
- **THEN** each row MUST show name, provider badge, type badge (built-in/custom), and status

#### Scenario: Image status display
- **WHEN** an image has a non-empty status (e.g. "pulling", "ready", "error")
- **THEN** the status MUST be shown as a badge on the image row

#### Scenario: Error message for failed image
- **WHEN** an image has a non-empty error_msg
- **THEN** the error_msg MUST be visible in the detail view (not the row)

#### Scenario: Empty image list
- **WHEN** no images are registered
- **THEN** the list MUST show an empty state with an "Add Image" affordance

### Requirement: View workspace image detail

The system SHALL display full detail for a selected workspace image. Fields shown SHALL include: name, provider, type, status, docker_ref, project_id, created_at, updated_at, and error_msg (if present).

#### Scenario: Selecting a workspace image
- **WHEN** the user selects an image from the list
- **THEN** the detail pane MUST show all `WorkspaceImageDTO` fields in labeled InfoRow components
- **THEN** if error_msg is non-empty, it MUST be shown in a highlighted error section

### Requirement: Register a custom workspace image

The system SHALL allow registering a new custom workspace image via `POST /api/admin/workspace-images`. The registration form SHALL capture: name (required), docker_ref (required for docker-based images), and provider (optional — defaults based on docker_ref presence). Submitting triggers a background docker pull on the backend.

#### Scenario: Opening the register form
- **WHEN** the user clicks "Add Image" in the workspace images header
- **THEN** a sheet MUST open with fields for name, docker_ref, and optional provider picker

#### Scenario: Successful registration with background pull
- **WHEN** the user submits valid data and `POST /api/admin/workspace-images` returns 201
- **THEN** the new image MUST appear in the list with a "pulling" or "pending" status badge
- **THEN** the sheet MUST close

#### Scenario: Validation
- **WHEN** name is empty or docker_ref is empty
- **THEN** the submit button MUST be disabled

#### Scenario: Registration conflict
- **WHEN** the API returns 409 Conflict (duplicate name)
- **THEN** the form MUST show an inline error: "An image with this name already exists"

#### Scenario: Registration failure (other errors)
- **WHEN** the API returns a 400 or 500 error
- **THEN** the form MUST show an inline error message and remain open

### Requirement: Delete a custom workspace image

The system SHALL allow deleting a custom workspace image via `DELETE /api/admin/workspace-images/{id}`. Built-in images SHALL NOT have a delete action. Deletion is permanent and requires a confirmation prompt.

#### Scenario: Delete action only for custom images
- **WHEN** a built-in image is selected
- **THEN** no delete action SHALL be available in the detail pane or context menu

#### Scenario: Deleting a custom image
- **WHEN** the user selects "Delete" on a custom image
- **THEN** a confirmation dialog MUST appear with the image name
- **WHEN** confirmed, the system MUST call `DELETE /api/admin/workspace-images/{id}` (expects 204)
- **THEN** the image MUST be removed from the list

#### Scenario: Deletion failure
- **WHEN** the API returns an error (e.g. 400 if attempting to delete built-in)
- **THEN** the image MUST remain in the list and an error message MUST be displayed
