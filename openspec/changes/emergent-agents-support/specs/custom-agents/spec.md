## MODIFIED Requirements

### Requirement: Custom Agent URL Field Validation
The custom agent form SHALL NOT enforce the URL field as required.

#### Scenario: Saving a custom agent without a URL
- **WHEN** a user fills out the custom agent form and leaves the URL field blank
- **THEN** the form allows submission
- **AND** the system successfully saves the agent

#### Scenario: Displaying URL optionality
- **WHEN** a user opens the custom agent creation form
- **THEN** the URL field label indicates that it is optional
- **AND** no validation error appears when it is empty
