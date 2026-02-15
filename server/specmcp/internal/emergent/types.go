package emergent

import "time"

// Entity type constants match the template pack definitions.
const (
	TypeActor        = "Actor"
	TypeCodingAgent  = "CodingAgent"
	TypePattern      = "Pattern"
	TypeConstitution = "Constitution"
	TypeChange       = "Change"
	TypeProposal     = "Proposal"
	TypeSpec         = "Spec"
	TypeRequirement  = "Requirement"
	TypeScenario     = "Scenario"
	TypeScenarioStep = "ScenarioStep"
	TypeDesign       = "Design"
	TypeTask         = "Task"
	TypeTestCase     = "TestCase"
	TypeAPIContract  = "APIContract"
	TypeContext      = "Context"
	TypeUIComponent  = "UIComponent"
	TypeAction       = "Action"
	TypeGraphSync    = "GraphSync"
)

// Relationship type constants.
const (
	RelInheritsFrom       = "inherits_from"
	RelUsesPattern        = "uses_pattern"
	RelExtendsPattern     = "extends_pattern"
	RelHasProposal        = "has_proposal"
	RelHasSpec            = "has_spec"
	RelHasDesign          = "has_design"
	RelHasTask            = "has_task"
	RelHasRequirement     = "has_requirement"
	RelHasScenario        = "has_scenario"
	RelExecutedBy         = "executed_by"
	RelHasStep            = "has_step"
	RelVariantOf          = "variant_of"
	RelOccursIn           = "occurs_in"
	RelPerforms           = "performs"
	RelComposedOf         = "composed_of"
	RelUsesComponent      = "uses_component"
	RelNestedIn           = "nested_in"
	RelAvailableIn        = "available_in"
	RelNavigatesTo        = "navigates_to"
	RelHasSubtask         = "has_subtask"
	RelBlocks             = "blocks"
	RelBlockedBy          = "blocked_by"
	RelImplements         = "implements"
	RelAssignedTo         = "assigned_to"
	RelGovernedBy         = "governed_by"
	RelRequiresPattern    = "requires_pattern"
	RelForbidsPattern     = "forbids_pattern"
	RelTestedBy           = "tested_by"
	RelTests              = "tests"
	RelHasContract        = "has_contract"
	RelImplementsContract = "implements_contract"
	RelOwnedBy            = "owned_by"
)

// Status constants for Change and Task entities.
const (
	StatusActive     = "active"
	StatusArchived   = "archived"
	StatusPending    = "pending"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusBlocked    = "blocked"
)

// Change represents a feature, bug fix, or refactoring effort.
type Change struct {
	ID         string   `json:"id,omitempty"`
	Name       string   `json:"name"`
	Status     string   `json:"status"`
	BaseCommit string   `json:"base_commit,omitempty"`
	Tags       []string `json:"tags,omitempty"`
}

// Proposal represents the intent of a change.
type Proposal struct {
	ID     string   `json:"id,omitempty"`
	Intent string   `json:"intent"`
	Scope  string   `json:"scope,omitempty"`
	Impact string   `json:"impact,omitempty"`
	Tags   []string `json:"tags,omitempty"`
}

// Spec represents a domain-specific specification container.
type Spec struct {
	ID        string   `json:"id,omitempty"`
	Name      string   `json:"name"`
	Domain    string   `json:"domain,omitempty"`
	Purpose   string   `json:"purpose,omitempty"`
	DeltaType string   `json:"delta_type,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

// Requirement represents a specific behavior the system must have.
type Requirement struct {
	ID          string   `json:"id,omitempty"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Strength    string   `json:"strength,omitempty"`
	DeltaType   string   `json:"delta_type,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// Scenario represents a concrete example of a requirement.
type Scenario struct {
	ID      string   `json:"id,omitempty"`
	Name    string   `json:"name"`
	Title   string   `json:"title,omitempty"`
	Given   string   `json:"given,omitempty"`
	When    string   `json:"when,omitempty"`
	Then    string   `json:"then,omitempty"`
	AndAlso []string `json:"and_also,omitempty"`
	Tags    []string `json:"tags,omitempty"`
}

// ScenarioStep represents a step in a complex scenario.
type ScenarioStep struct {
	ID          string   `json:"id,omitempty"`
	Sequence    int      `json:"sequence"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
}

// Design represents the technical approach for a change.
type Design struct {
	ID          string   `json:"id,omitempty"`
	Approach    string   `json:"approach,omitempty"`
	Decisions   string   `json:"decisions,omitempty"`
	DataFlow    string   `json:"data_flow,omitempty"`
	FileChanges []string `json:"file_changes,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// Task represents an implementation task.
type Task struct {
	ID                 string     `json:"id,omitempty"`
	Number             string     `json:"number"`
	Description        string     `json:"description"`
	TaskType           string     `json:"task_type,omitempty"`
	Status             string     `json:"status"`
	ComplexityPoints   int        `json:"complexity_points,omitempty"`
	StartedAt          *time.Time `json:"started_at,omitempty"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
	ActualHours        float64    `json:"actual_hours,omitempty"`
	Artifacts          []string   `json:"artifacts,omitempty"`
	VerificationMethod string     `json:"verification_method,omitempty"`
	VerificationNotes  string     `json:"verification_notes,omitempty"`
	Tags               []string   `json:"tags,omitempty"`
}

// Actor represents a user, role, or persona.
type Actor struct {
	ID          string   `json:"id,omitempty"`
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// CodingAgent represents a developer or AI agent.
type CodingAgent struct {
	ID                  string   `json:"id,omitempty"`
	Name                string   `json:"name"`
	DisplayName         string   `json:"display_name,omitempty"`
	Type                string   `json:"type"`
	Active              bool     `json:"active"`
	Skills              []string `json:"skills,omitempty"`
	Specialization      string   `json:"specialization,omitempty"`
	Instructions        string   `json:"instructions,omitempty"`
	VelocityPointsPerHr float64  `json:"velocity_points_per_hour,omitempty"`
	Tags                []string `json:"tags,omitempty"`
}

// Pattern represents a reusable implementation pattern.
type Pattern struct {
	ID            string   `json:"id,omitempty"`
	Name          string   `json:"name"`
	DisplayName   string   `json:"display_name,omitempty"`
	Type          string   `json:"type"`
	Scope         string   `json:"scope,omitempty"`
	Description   string   `json:"description,omitempty"`
	ExampleCode   string   `json:"example_code,omitempty"`
	UsageGuidance string   `json:"usage_guidance,omitempty"`
	Tags          []string `json:"tags,omitempty"`
}

// Constitution represents project-wide principles.
type Constitution struct {
	ID                   string   `json:"id,omitempty"`
	Name                 string   `json:"name"`
	Version              string   `json:"version"`
	Principles           string   `json:"principles,omitempty"`
	Guardrails           []string `json:"guardrails,omitempty"`
	TestingRequirements  string   `json:"testing_requirements,omitempty"`
	SecurityRequirements string   `json:"security_requirements,omitempty"`
	PatternsRequired     []string `json:"patterns_required,omitempty"`
	PatternsForbidden    []string `json:"patterns_forbidden,omitempty"`
	Tags                 []string `json:"tags,omitempty"`
}

// TestCase links scenarios to executable tests.
type TestCase struct {
	ID              string     `json:"id,omitempty"`
	Name            string     `json:"name"`
	TestFile        string     `json:"test_file,omitempty"`
	TestFunction    string     `json:"test_function,omitempty"`
	TestFramework   string     `json:"test_framework,omitempty"`
	Status          string     `json:"status,omitempty"`
	LastRunAt       *time.Time `json:"last_run_at,omitempty"`
	CoveragePercent float64    `json:"coverage_percent,omitempty"`
	Tags            []string   `json:"tags,omitempty"`
}

// APIContract represents a machine-readable API definition.
type APIContract struct {
	ID               string     `json:"id,omitempty"`
	Name             string     `json:"name"`
	Format           string     `json:"format"`
	Version          string     `json:"version,omitempty"`
	FilePath         string     `json:"file_path,omitempty"`
	BaseURL          string     `json:"base_url,omitempty"`
	Description      string     `json:"description,omitempty"`
	AutoGenerated    bool       `json:"auto_generated,omitempty"`
	LastValidatedAt  *time.Time `json:"last_validated_at,omitempty"`
	ValidationStatus string     `json:"validation_status,omitempty"`
	Tags             []string   `json:"tags,omitempty"`
}

// Context represents a screen, modal, or interaction surface.
type Context struct {
	ID          string   `json:"id,omitempty"`
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name,omitempty"`
	Type        string   `json:"type,omitempty"`
	Scope       string   `json:"scope,omitempty"`
	Platform    []string `json:"platform,omitempty"`
	Description string   `json:"description,omitempty"`
	FilePath    string   `json:"file_path,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// UIComponent represents a reusable UI component.
type UIComponent struct {
	ID          string   `json:"id,omitempty"`
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name,omitempty"`
	Type        string   `json:"type,omitempty"`
	FilePath    string   `json:"file_path,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// Action represents a user action or system operation.
type Action struct {
	ID           string   `json:"id,omitempty"`
	Name         string   `json:"name"`
	DisplayLabel string   `json:"display_label,omitempty"`
	Type         string   `json:"type,omitempty"`
	Description  string   `json:"description,omitempty"`
	HandlerPath  string   `json:"handler_path,omitempty"`
	Tags         []string `json:"tags,omitempty"`
}

// GraphSync tracks synchronization state.
type GraphSync struct {
	ID               string     `json:"id,omitempty"`
	LastSyncedCommit string     `json:"last_synced_commit,omitempty"`
	LastSyncedAt     *time.Time `json:"last_synced_at,omitempty"`
	Status           string     `json:"status"`
	Tags             []string   `json:"tags,omitempty"`
}
