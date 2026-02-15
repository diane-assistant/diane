package workflow

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/diane-assistant/diane/server/specmcp/internal/emergent"
	"github.com/diane-assistant/diane/server/specmcp/internal/guards"
	"github.com/diane-assistant/diane/server/specmcp/internal/mcp"
	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"
)

// specArtifactParams defines the input for spec_artifact.
type specArtifactParams struct {
	ChangeID     string         `json:"change_id"`
	ArtifactType string         `json:"artifact_type"`
	Content      map[string]any `json:"content"`
}

// SpecArtifact adds artifacts to a change.
type SpecArtifact struct {
	client *emergent.Client
	runner *guards.Runner
}

// NewSpecArtifact creates a SpecArtifact tool.
func NewSpecArtifact(client *emergent.Client) *SpecArtifact {
	return &SpecArtifact{
		client: client,
		runner: guards.NewRunner(),
	}
}

func (t *SpecArtifact) Name() string { return "spec_artifact" }

func (t *SpecArtifact) Description() string {
	return "Add an artifact to an existing change. Supports: spec (with requirements and scenarios), design, task, actor, pattern, test_case, api_contract, context, ui_component, action, data_model, service, scenario_step. Enforces workflow ordering guards: Proposal → Spec → Design → Tasks."
}

func (t *SpecArtifact) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "change_id": {
      "type": "string",
      "description": "ID of the change to add the artifact to"
    },
    "artifact_type": {
      "type": "string",
      "description": "Type of artifact to add",
      "enum": ["proposal", "spec", "design", "task", "actor", "coding_agent", "pattern", "test_case", "api_contract", "context", "ui_component", "action", "data_model", "service", "requirement", "scenario", "scenario_step", "constitution"]
    },
    "content": {
      "type": "object",
      "description": "Artifact-specific content. Fields depend on artifact_type. For spec: name, domain, purpose, requirements (array), scenarios (array). For design: approach, decisions, file_changes. For task: number, description, task_type, complexity_points."
    }
  },
  "required": ["change_id", "artifact_type", "content"]
}`)
}

func (t *SpecArtifact) Execute(ctx context.Context, params json.RawMessage) (*mcp.ToolsCallResult, error) {
	var p specArtifactParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcp.ErrorResult(fmt.Sprintf("invalid parameters: %v", err)), nil
	}

	if p.ChangeID == "" {
		return mcp.ErrorResult("change_id is required"), nil
	}
	if p.ArtifactType == "" {
		return mcp.ErrorResult("artifact_type is required"), nil
	}
	if p.Content == nil {
		return mcp.ErrorResult("content is required"), nil
	}

	// Verify change exists and is active
	change, err := t.client.GetChange(ctx, p.ChangeID)
	if err != nil {
		return mcp.ErrorResult(fmt.Sprintf("change not found: %v", err)), nil
	}
	if change.Status == emergent.StatusArchived {
		return mcp.ErrorResult("cannot add artifacts to an archived change"), nil
	}

	// Run workflow ordering guards
	gctx := &guards.GuardContext{
		ChangeID:     p.ChangeID,
		ArtifactType: p.ArtifactType,
	}
	if err := guards.PopulateChangeState(ctx, t.client, gctx); err != nil {
		return nil, fmt.Errorf("populating change state for guards: %w", err)
	}

	outcome := t.runner.Run(ctx, gctx, guards.ArtifactGuards())
	if outcome.Blocked {
		return mcp.ErrorResult(outcome.FormatBlockMessage()), nil
	}

	// Dispatch to type-specific handler
	switch p.ArtifactType {
	case "proposal":
		return t.addProposal(ctx, p.ChangeID, p.Content)
	case "spec":
		return t.addSpec(ctx, p.ChangeID, p.Content)
	case "design":
		return t.addDesign(ctx, p.ChangeID, p.Content)
	case "task":
		return t.addTask(ctx, p.ChangeID, p.Content)
	case "requirement":
		return t.addRequirement(ctx, p.Content)
	case "scenario":
		return t.addScenario(ctx, p.Content)
	case "scenario_step":
		return t.addScenarioStep(ctx, p.Content)
	case "actor":
		return t.addGenericEntity(ctx, emergent.TypeActor, p.Content, nil, "")
	case "coding_agent":
		return t.addGenericEntity(ctx, emergent.TypeCodingAgent, p.Content, nil, "")
	case "pattern":
		return t.addGenericEntity(ctx, emergent.TypePattern, p.Content, nil, "")
	case "test_case":
		return t.addTestCase(ctx, p.Content)
	case "api_contract":
		return t.addGenericEntity(ctx, emergent.TypeAPIContract, p.Content, nil, "")
	case "context":
		return t.addGenericEntity(ctx, emergent.TypeContext, p.Content, nil, "")
	case "ui_component":
		return t.addGenericEntity(ctx, emergent.TypeUIComponent, p.Content, nil, "")
	case "action":
		return t.addGenericEntity(ctx, emergent.TypeAction, p.Content, nil, "")
	case "data_model":
		return t.addGenericEntity(ctx, emergent.TypeDataModel, p.Content, nil, "")
	case "service":
		return t.addGenericEntity(ctx, emergent.TypeService, p.Content, nil, "")
	case "constitution":
		return t.addConstitution(ctx, p.ChangeID, p.Content)
	default:
		return mcp.ErrorResult(fmt.Sprintf("unsupported artifact type: %s", p.ArtifactType)), nil
	}
}

// addProposal creates a Proposal for the change (1:1, rejects duplicates).
func (t *SpecArtifact) addProposal(ctx context.Context, changeID string, content map[string]any) (*mcp.ToolsCallResult, error) {
	// Check for existing proposal
	has, err := t.hasRelType(ctx, changeID, emergent.RelHasProposal)
	if err != nil {
		return nil, err
	}
	if has {
		return mcp.ErrorResult("Change already has a Proposal (1:1 relationship)"), nil
	}

	proposal, err := t.client.CreateProposal(ctx, changeID, &emergent.Proposal{
		Intent: getString(content, "intent"),
		Scope:  getString(content, "scope"),
		Impact: getString(content, "impact"),
		Tags:   getStringSlice(content, "tags"),
	})
	if err != nil {
		return nil, fmt.Errorf("creating proposal: %w", err)
	}

	return jsonResult(map[string]any{
		"type":    "proposal",
		"id":      proposal.ID,
		"message": "Created proposal",
	})
}

// addSpec creates a Spec with optional Requirements and Scenarios.
func (t *SpecArtifact) addSpec(ctx context.Context, changeID string, content map[string]any) (*mcp.ToolsCallResult, error) {
	spec, err := t.client.CreateSpec(ctx, changeID, &emergent.Spec{
		Name:      getString(content, "name"),
		Domain:    getString(content, "domain"),
		Purpose:   getString(content, "purpose"),
		DeltaType: getString(content, "delta_type"),
		Tags:      getStringSlice(content, "tags"),
	})
	if err != nil {
		return nil, fmt.Errorf("creating spec: %w", err)
	}

	result := map[string]any{
		"type":    "spec",
		"id":      spec.ID,
		"name":    spec.Name,
		"message": fmt.Sprintf("Created spec %q", spec.Name),
	}

	// Create nested requirements if provided
	if reqs, ok := content["requirements"].([]any); ok {
		reqResults := make([]map[string]any, 0, len(reqs))
		for _, r := range reqs {
			reqMap, ok := r.(map[string]any)
			if !ok {
				continue
			}
			req, err := t.client.CreateRequirement(ctx, spec.ID, &emergent.Requirement{
				Name:        getString(reqMap, "name"),
				Description: getString(reqMap, "description"),
				Strength:    getString(reqMap, "strength"),
				DeltaType:   getString(reqMap, "delta_type"),
				Tags:        getStringSlice(reqMap, "tags"),
			})
			if err != nil {
				return nil, fmt.Errorf("creating requirement: %w", err)
			}

			reqResult := map[string]any{
				"id":   req.ID,
				"name": req.Name,
			}

			// Create nested scenarios for this requirement
			if scenarios, ok := reqMap["scenarios"].([]any); ok {
				scenResults := make([]map[string]any, 0, len(scenarios))
				for _, s := range scenarios {
					scenMap, ok := s.(map[string]any)
					if !ok {
						continue
					}
					scen, err := t.client.CreateScenario(ctx, req.ID, &emergent.Scenario{
						Name:  getString(scenMap, "name"),
						Title: getString(scenMap, "title"),
						Given: getString(scenMap, "given"),
						When:  getString(scenMap, "when"),
						Then:  getString(scenMap, "then"),
						Tags:  getStringSlice(scenMap, "tags"),
					})
					if err != nil {
						return nil, fmt.Errorf("creating scenario: %w", err)
					}
					scenResults = append(scenResults, map[string]any{
						"id":   scen.ID,
						"name": scen.Name,
					})
				}
				reqResult["scenarios"] = scenResults
			}

			reqResults = append(reqResults, reqResult)
		}
		result["requirements"] = reqResults
	}

	return jsonResult(result)
}

// addDesign creates a Design for the change (1:1, rejects duplicates).
func (t *SpecArtifact) addDesign(ctx context.Context, changeID string, content map[string]any) (*mcp.ToolsCallResult, error) {
	has, err := t.hasRelType(ctx, changeID, emergent.RelHasDesign)
	if err != nil {
		return nil, err
	}
	if has {
		return mcp.ErrorResult("Change already has a Design (1:1 relationship)"), nil
	}

	design, err := t.client.CreateDesign(ctx, changeID, &emergent.Design{
		Approach:    getString(content, "approach"),
		Decisions:   getString(content, "decisions"),
		DataFlow:    getString(content, "data_flow"),
		FileChanges: getStringSlice(content, "file_changes"),
		Tags:        getStringSlice(content, "tags"),
	})
	if err != nil {
		return nil, fmt.Errorf("creating design: %w", err)
	}

	return jsonResult(map[string]any{
		"type":    "design",
		"id":      design.ID,
		"message": "Created design",
	})
}

// addTask creates a Task for the change.
func (t *SpecArtifact) addTask(ctx context.Context, changeID string, content map[string]any) (*mcp.ToolsCallResult, error) {
	task, err := t.client.CreateTask(ctx, changeID, &emergent.Task{
		Number:             getString(content, "number"),
		Description:        getString(content, "description"),
		TaskType:           getString(content, "task_type"),
		Status:             emergent.StatusPending,
		ComplexityPoints:   getInt(content, "complexity_points"),
		VerificationMethod: getString(content, "verification_method"),
		Tags:               getStringSlice(content, "tags"),
	})
	if err != nil {
		return nil, fmt.Errorf("creating task: %w", err)
	}

	result := map[string]any{
		"type":        "task",
		"id":          task.ID,
		"number":      task.Number,
		"description": task.Description,
		"status":      task.Status,
		"message":     fmt.Sprintf("Created task %s", task.Number),
	}

	// Create blocking relationships if specified
	if blocks, ok := content["blocks"].([]any); ok {
		for _, b := range blocks {
			if blockedID, ok := b.(string); ok {
				if _, err := t.client.CreateRelationship(ctx, emergent.RelBlocks, task.ID, blockedID, nil); err != nil {
					return nil, fmt.Errorf("creating blocks relationship: %w", err)
				}
			}
		}
	}

	// Create implements relationship if specified
	if implID := getString(content, "implements"); implID != "" {
		if _, err := t.client.CreateRelationship(ctx, emergent.RelImplements, task.ID, implID, nil); err != nil {
			return nil, fmt.Errorf("creating implements relationship: %w", err)
		}
	}

	// Create subtask relationship if specified
	if parentID := getString(content, "parent_task_id"); parentID != "" {
		if _, err := t.client.CreateRelationship(ctx, emergent.RelHasSubtask, parentID, task.ID, nil); err != nil {
			return nil, fmt.Errorf("creating subtask relationship: %w", err)
		}
	}

	return jsonResult(result)
}

// addRequirement creates a Requirement linked to a Spec (spec_id in content).
func (t *SpecArtifact) addRequirement(ctx context.Context, content map[string]any) (*mcp.ToolsCallResult, error) {
	specID := getString(content, "spec_id")
	if specID == "" {
		return mcp.ErrorResult("content.spec_id is required for requirement artifacts"), nil
	}

	req, err := t.client.CreateRequirement(ctx, specID, &emergent.Requirement{
		Name:        getString(content, "name"),
		Description: getString(content, "description"),
		Strength:    getString(content, "strength"),
		DeltaType:   getString(content, "delta_type"),
		Tags:        getStringSlice(content, "tags"),
	})
	if err != nil {
		return nil, fmt.Errorf("creating requirement: %w", err)
	}

	return jsonResult(map[string]any{
		"type":    "requirement",
		"id":      req.ID,
		"name":    req.Name,
		"message": fmt.Sprintf("Created requirement %q", req.Name),
	})
}

// addScenario creates a Scenario linked to a Requirement (requirement_id in content).
func (t *SpecArtifact) addScenario(ctx context.Context, content map[string]any) (*mcp.ToolsCallResult, error) {
	reqID := getString(content, "requirement_id")
	if reqID == "" {
		return mcp.ErrorResult("content.requirement_id is required for scenario artifacts"), nil
	}

	scen, err := t.client.CreateScenario(ctx, reqID, &emergent.Scenario{
		Name:  getString(content, "name"),
		Title: getString(content, "title"),
		Given: getString(content, "given"),
		When:  getString(content, "when"),
		Then:  getString(content, "then"),
		Tags:  getStringSlice(content, "tags"),
	})
	if err != nil {
		return nil, fmt.Errorf("creating scenario: %w", err)
	}

	return jsonResult(map[string]any{
		"type":    "scenario",
		"id":      scen.ID,
		"name":    scen.Name,
		"message": fmt.Sprintf("Created scenario %q", scen.Name),
	})
}

// addScenarioStep creates a ScenarioStep linked to a Scenario.
func (t *SpecArtifact) addScenarioStep(ctx context.Context, content map[string]any) (*mcp.ToolsCallResult, error) {
	scenarioID := getString(content, "scenario_id")
	if scenarioID == "" {
		return mcp.ErrorResult("content.scenario_id is required for scenario_step artifacts"), nil
	}

	props := map[string]any{
		"sequence":    getInt(content, "sequence"),
		"description": getString(content, "description"),
	}
	labels := getStringSlice(content, "tags")

	obj, err := t.client.CreateObject(ctx, emergent.TypeScenarioStep, nil, props, labels)
	if err != nil {
		return nil, fmt.Errorf("creating scenario step: %w", err)
	}

	// Link to scenario
	if _, err := t.client.CreateRelationship(ctx, emergent.RelHasStep, scenarioID, obj.ID, nil); err != nil {
		return nil, fmt.Errorf("linking step to scenario: %w", err)
	}

	// Auto-create occurs_in relationship if context_id is provided
	if contextID := getString(content, "context_id"); contextID != "" {
		if _, err := t.client.CreateRelationship(ctx, emergent.RelOccursIn, obj.ID, contextID, nil); err != nil {
			return nil, fmt.Errorf("creating occurs_in relationship: %w", err)
		}
	}

	// Auto-create performs relationship if action_id is provided
	if actionID := getString(content, "action_id"); actionID != "" {
		if _, err := t.client.CreateRelationship(ctx, emergent.RelPerforms, obj.ID, actionID, nil); err != nil {
			return nil, fmt.Errorf("creating performs relationship: %w", err)
		}
	}

	return jsonResult(map[string]any{
		"type":    "scenario_step",
		"id":      obj.ID,
		"message": "Created scenario step",
	})
}

// addTestCase creates a TestCase and optionally links it to a Scenario.
func (t *SpecArtifact) addTestCase(ctx context.Context, content map[string]any) (*mcp.ToolsCallResult, error) {
	props := map[string]any{
		"name":           getString(content, "name"),
		"test_file":      getString(content, "test_file"),
		"test_function":  getString(content, "test_function"),
		"test_framework": getString(content, "test_framework"),
		"status":         getString(content, "status"),
	}
	key := getString(content, "name")

	obj, err := t.client.CreateObject(ctx, emergent.TypeTestCase, &key, props, getStringSlice(content, "tags"))
	if err != nil {
		return nil, fmt.Errorf("creating test case: %w", err)
	}

	// Link to scenario if provided
	if scenarioID := getString(content, "scenario_id"); scenarioID != "" {
		if _, err := t.client.CreateRelationship(ctx, emergent.RelTests, obj.ID, scenarioID, nil); err != nil {
			return nil, fmt.Errorf("creating tests relationship: %w", err)
		}
		// Also create the inverse tested_by
		if _, err := t.client.CreateRelationship(ctx, emergent.RelTestedBy, scenarioID, obj.ID, nil); err != nil {
			return nil, fmt.Errorf("creating tested_by relationship: %w", err)
		}
	}

	return jsonResult(map[string]any{
		"type":    "test_case",
		"id":      obj.ID,
		"name":    key,
		"message": fmt.Sprintf("Created test case %q", key),
	})
}

// addConstitution creates a Constitution and links it to the Change via governed_by.
func (t *SpecArtifact) addConstitution(ctx context.Context, changeID string, content map[string]any) (*mcp.ToolsCallResult, error) {
	props := map[string]any{
		"name":                  getString(content, "name"),
		"version":               getString(content, "version"),
		"principles":            getString(content, "principles"),
		"guardrails":            getStringSlice(content, "guardrails"),
		"testing_requirements":  getString(content, "testing_requirements"),
		"security_requirements": getString(content, "security_requirements"),
		"patterns_required":     getStringSlice(content, "patterns_required"),
		"patterns_forbidden":    getStringSlice(content, "patterns_forbidden"),
	}
	key := getString(content, "name")

	obj, err := t.client.CreateObject(ctx, emergent.TypeConstitution, &key, props, getStringSlice(content, "tags"))
	if err != nil {
		return nil, fmt.Errorf("creating constitution: %w", err)
	}

	// Link change to constitution via governed_by
	if _, err := t.client.CreateRelationship(ctx, emergent.RelGovernedBy, changeID, obj.ID, nil); err != nil {
		return nil, fmt.Errorf("creating governed_by relationship: %w", err)
	}

	return jsonResult(map[string]any{
		"type":    "constitution",
		"id":      obj.ID,
		"name":    key,
		"message": fmt.Sprintf("Created constitution %q", key),
	})
}

// addGenericEntity creates or updates a generic entity with properties from content.
// If an entity with the same type+key already exists, it is updated in place.
// PATCH now preserves relationships via canonical_id resolution (Emergent #22),
// so updates are safe without dirty-checking.
func (t *SpecArtifact) addGenericEntity(ctx context.Context, typeName string, content map[string]any, labels []string, _ string) (*mcp.ToolsCallResult, error) {
	// Extract key from name field
	key := getString(content, "name")
	var keyPtr *string
	if key != "" {
		keyPtr = &key
	}

	// Use content as properties directly (remove non-property fields)
	props := make(map[string]any, len(content))
	for k, v := range content {
		props[k] = v
	}
	delete(props, "tags")

	// Remove relationship fields from properties — they are handled separately
	delete(props, "patterns")
	delete(props, "context_id")
	delete(props, "parent_context_id")
	delete(props, "component_ids")
	delete(props, "service_id")
	delete(props, "model_ids")
	delete(props, "api_ids")

	if labels == nil {
		labels = getStringSlice(content, "tags")
	}

	// Dedup: check if entity with same type+key already exists
	var obj *graph.GraphObject
	action := "Created"
	if key != "" {
		existing, err := t.client.FindByTypeAndKey(ctx, typeName, key)
		if err != nil {
			return nil, fmt.Errorf("checking for existing %s: %w", typeName, err)
		}
		if existing != nil {
			// PATCH is safe now (canonical_id resolution preserves relationships),
			// so always update to ensure properties are current.
			obj, err = t.client.UpdateObject(ctx, existing.ID, props, labels)
			if err != nil {
				return nil, fmt.Errorf("updating %s: %w", typeName, err)
			}
			action = "Updated"
		}
	}

	if obj == nil {
		var err error
		obj, err = t.client.CreateObject(ctx, typeName, keyPtr, props, labels)
		if err != nil {
			return nil, fmt.Errorf("creating %s: %w", typeName, err)
		}
	}

	// Create relationships if specified in content
	relResults, err := t.createEntityRelationships(ctx, typeName, obj.ID, content)
	if err != nil {
		// Log but don't fail — the entity was already created/updated
		relResults = append(relResults, fmt.Sprintf("relationship error: %v", err))
	}

	result := map[string]any{
		"type":    typeName,
		"id":      obj.ID,
		"name":    key,
		"message": fmt.Sprintf("%s %s %q", action, typeName, key),
	}
	if len(relResults) > 0 {
		result["relationships_created"] = relResults
	}

	return jsonResult(result)
}

// createEntityRelationships creates relationships specified in content fields:
//   - patterns: []string of pattern names → uses_pattern relationships
//   - context_id: string → uses_component relationship (Context → this UIComponent)
//   - parent_context_id: string → nested_in relationship (this Context → parent Context)
//   - component_ids: []string of component IDs → uses_component relationships (this Context → Components)
//
// Relationships are deduplicated: if a relationship with the same type+src+dst
// already exists, it is skipped rather than creating a duplicate.
func (t *SpecArtifact) createEntityRelationships(ctx context.Context, typeName, entityID string, content map[string]any) ([]string, error) {
	var results []string

	// Handle patterns: look up each by name and create uses_pattern relationship
	if patternNames := getStringSlice(content, "patterns"); len(patternNames) > 0 {
		for _, pName := range patternNames {
			pattern, err := t.client.FindByTypeAndKey(ctx, emergent.TypePattern, pName)
			if err != nil {
				results = append(results, fmt.Sprintf("pattern %q lookup failed: %v", pName, err))
				continue
			}
			if pattern == nil {
				results = append(results, fmt.Sprintf("pattern %q not found", pName))
				continue
			}
			created, err := t.ensureRelationship(ctx, emergent.RelUsesPattern, entityID, pattern.ID)
			if err != nil {
				results = append(results, fmt.Sprintf("pattern %q link failed: %v", pName, err))
				continue
			}
			if created {
				results = append(results, fmt.Sprintf("uses_pattern → %s", pName))
			} else {
				results = append(results, fmt.Sprintf("uses_pattern → %s (already exists)", pName))
			}
		}
	}

	// Handle context_id: create uses_component (Context → this UIComponent)
	if typeName == emergent.TypeUIComponent {
		if ctxID := getString(content, "context_id"); ctxID != "" {
			created, err := t.ensureRelationship(ctx, emergent.RelUsesComponent, ctxID, entityID)
			if err != nil {
				return results, fmt.Errorf("creating uses_component: %w", err)
			}
			if created {
				results = append(results, fmt.Sprintf("uses_component ← context %s", ctxID))
			} else {
				results = append(results, fmt.Sprintf("uses_component ← context %s (already exists)", ctxID))
			}
		}
	}

	// Handle parent_context_id: create nested_in (this Context → parent Context)
	if typeName == emergent.TypeContext {
		if parentID := getString(content, "parent_context_id"); parentID != "" {
			created, err := t.ensureRelationship(ctx, emergent.RelNestedIn, entityID, parentID)
			if err != nil {
				return results, fmt.Errorf("creating nested_in: %w", err)
			}
			if created {
				results = append(results, fmt.Sprintf("nested_in → context %s", parentID))
			} else {
				results = append(results, fmt.Sprintf("nested_in → context %s (already exists)", parentID))
			}
		}
	}

	// Handle component_ids: create uses_component (this Context → each UIComponent)
	if typeName == emergent.TypeContext {
		if compIDs := getStringSlice(content, "component_ids"); len(compIDs) > 0 {
			for _, compID := range compIDs {
				created, err := t.ensureRelationship(ctx, emergent.RelUsesComponent, entityID, compID)
				if err != nil {
					results = append(results, fmt.Sprintf("uses_component → %s failed: %v", compID, err))
					continue
				}
				if created {
					results = append(results, fmt.Sprintf("uses_component → %s", compID))
				} else {
					results = append(results, fmt.Sprintf("uses_component → %s (already exists)", compID))
				}
			}
		}
	}

	// Handle service_id: create belongs_to_service (this entity → Service)
	if serviceID := getString(content, "service_id"); serviceID != "" {
		created, err := t.ensureRelationship(ctx, emergent.RelBelongsToService, entityID, serviceID)
		if err != nil {
			results = append(results, fmt.Sprintf("belongs_to_service → %s failed: %v", serviceID, err))
		} else if created {
			results = append(results, fmt.Sprintf("belongs_to_service → %s", serviceID))
		} else {
			results = append(results, fmt.Sprintf("belongs_to_service → %s (already exists)", serviceID))
		}
	}

	// Handle model_ids: create uses_model (this entity → each DataModel)
	if modelIDs := getStringSlice(content, "model_ids"); len(modelIDs) > 0 {
		for _, modelID := range modelIDs {
			created, err := t.ensureRelationship(ctx, emergent.RelUsesModel, entityID, modelID)
			if err != nil {
				results = append(results, fmt.Sprintf("uses_model → %s failed: %v", modelID, err))
				continue
			}
			if created {
				results = append(results, fmt.Sprintf("uses_model → %s", modelID))
			} else {
				results = append(results, fmt.Sprintf("uses_model → %s (already exists)", modelID))
			}
		}
	}

	// Handle context_ids: create available_in (this Action → each Context)
	if typeName == emergent.TypeAction {
		if ctxIDs := getStringSlice(content, "context_ids"); len(ctxIDs) > 0 {
			for _, ctxID := range ctxIDs {
				created, err := t.ensureRelationship(ctx, emergent.RelAvailableIn, entityID, ctxID)
				if err != nil {
					results = append(results, fmt.Sprintf("available_in → %s failed: %v", ctxID, err))
					continue
				}
				if created {
					results = append(results, fmt.Sprintf("available_in → %s", ctxID))
				} else {
					results = append(results, fmt.Sprintf("available_in → %s (already exists)", ctxID))
				}
			}
		}
	}

	// Handle api_ids: create exposes_api (this Service → each APIContract)
	if typeName == emergent.TypeService {
		if apiIDs := getStringSlice(content, "api_ids"); len(apiIDs) > 0 {
			for _, apiID := range apiIDs {
				created, err := t.ensureRelationship(ctx, emergent.RelExposesAPI, entityID, apiID)
				if err != nil {
					results = append(results, fmt.Sprintf("exposes_api → %s failed: %v", apiID, err))
					continue
				}
				if created {
					results = append(results, fmt.Sprintf("exposes_api → %s", apiID))
				} else {
					results = append(results, fmt.Sprintf("exposes_api → %s (already exists)", apiID))
				}
			}
		}
	}

	return results, nil
}

// ensureRelationship creates a relationship only if one with the same type+src+dst
// doesn't already exist. Returns (true, nil) if created, (false, nil) if already exists.
func (t *SpecArtifact) ensureRelationship(ctx context.Context, relType, srcID, dstID string) (bool, error) {
	existing, err := t.client.ListRelationships(ctx, &graph.ListRelationshipsOptions{
		Type:  relType,
		SrcID: srcID,
		DstID: dstID,
		Limit: 1,
	})
	if err != nil {
		return false, err
	}
	if len(existing) > 0 {
		return false, nil // already exists
	}
	if _, err := t.client.CreateRelationship(ctx, relType, srcID, dstID, nil); err != nil {
		return false, err
	}
	return true, nil
}

// hasRelType checks if a change has at least one outgoing relationship of the given type.
func (t *SpecArtifact) hasRelType(ctx context.Context, changeID, relType string) (bool, error) {
	rels, err := t.client.ListRelationships(ctx, &graph.ListRelationshipsOptions{
		Type:  relType,
		SrcID: changeID,
		Limit: 1,
	})
	if err != nil {
		return false, err
	}
	return len(rels) > 0, nil
}

// --- Helpers ---

func getString(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func getInt(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	default:
		return 0
	}
}

func getStringSlice(m map[string]any, key string) []string {
	raw, ok := m[key].([]any)
	if !ok {
		return nil
	}
	ss := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			ss = append(ss, s)
		}
	}
	return ss
}

func jsonResult(v any) (*mcp.ToolsCallResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling result: %w", err)
	}
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{mcp.TextContent(string(b))},
	}, nil
}
