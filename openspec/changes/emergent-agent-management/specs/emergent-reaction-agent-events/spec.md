## ADDED Requirements

### Requirement: View pending events for a reaction agent

The system SHALL display unprocessed graph objects (pending events) for a reaction agent via `GET /api/admin/agents/{id}/pending-events`. The view SHALL only be visible for agents with `triggerType == "reaction"`. It SHALL show a list of pending objects with their type, key, id, and timestamps, plus the reaction config (objectTypes, events, concurrencyStrategy, ignoreAgentTriggered, ignoreSelfTriggered) and the total count.

#### Scenario: Opening pending events for a reaction agent
- **WHEN** the user selects a reaction agent and navigates to the "Pending Events" tab
- **THEN** the system MUST call `GET /api/admin/agents/{id}/pending-events` (limit 100)
- **THEN** the system MUST display the totalCount and a list of pending objects

#### Scenario: Pending object row display
- **WHEN** pending events are loaded
- **THEN** each row MUST show the object type (as a badge), key, id, and updatedAt timestamp

#### Scenario: Reaction config summary
- **WHEN** the pending events response is loaded
- **THEN** a "Reaction Config" DetailSection MUST show objectTypes, events (created/updated/deleted), concurrencyStrategy, ignoreAgentTriggered, and ignoreSelfTriggered as InfoRow items

#### Scenario: No pending events
- **WHEN** the API returns an empty objects array
- **THEN** the system MUST display an empty state: "No pending events"

#### Scenario: Pending events tab hidden for non-reaction agents
- **WHEN** the selected agent has triggerType other than `reaction`
- **THEN** the "Pending Events" tab MUST NOT be displayed

### Requirement: Batch trigger a reaction agent for selected objects

The system SHALL allow the user to select pending event objects and batch-trigger the reaction agent via `POST /api/admin/agents/{id}/batch-trigger` with up to 100 objectIds. The response SHALL report how many runs were queued vs. skipped.

#### Scenario: Selecting objects for batch trigger
- **WHEN** the user selects one or more pending event objects using checkboxes
- **THEN** a "Trigger Selected" action button MUST become enabled

#### Scenario: Executing batch trigger
- **WHEN** the user clicks "Trigger Selected" with at least one object selected
- **THEN** the system MUST call `POST /api/admin/agents/{id}/batch-trigger` with the selected objectIds
- **THEN** on success, the system MUST display a summary: "{queued} queued, {skipped} skipped"

#### Scenario: Batch trigger respects maximum of 100 objects
- **WHEN** more than 100 objects are selected
- **THEN** the system MUST show a warning and limit the batch to 100 objectIds

#### Scenario: Batch trigger failure
- **WHEN** the batch trigger API call fails
- **THEN** the system MUST display an inline error message and leave the selected objects unchanged
