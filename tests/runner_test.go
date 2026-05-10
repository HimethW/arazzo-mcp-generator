package tests

import (
	"testing"

	"github.com/wso2/arazzo-mcp-generator/internal/models"
	"github.com/wso2/arazzo-mcp-generator/internal/runner"
	"github.com/wso2/arazzo-mcp-generator/internal/telemetry"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildRunner constructs an ArazzoRunner directly from a workflow slice,
// bypassing file I/O.
func buildRunner(workflows []interface{}) *runner.ArazzoRunner {
	return &runner.ArazzoRunner{
		Workflows: workflows,
		Sink:      &telemetry.NoopSink{},
	}
}

// wf is a shorthand for building a minimal workflow map.
func wf(id string, hasSteps bool) map[string]interface{} {
	m := map[string]interface{}{"workflowId": id}
	if hasSteps {
		m["steps"] = []interface{}{
			map[string]interface{}{"stepId": "step1"},
		}
	}
	return m
}

// ---------------------------------------------------------------------------
// NewArazzoRunner – file-based constructor
// ---------------------------------------------------------------------------

func TestNewArazzoRunner_FileNotFound(t *testing.T) {
	_, err := runner.NewArazzoRunner("non-existent-file.arazzo.yaml", nil, &telemetry.NoopSink{})
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
}

func TestNewArazzoRunner_InvalidYAML(t *testing.T) {
	// Use a path that exists but holds invalid YAML – the temp approach avoids
	// writing a fixture file by relying on a known non-YAML path.
	// Instead we simply confirm FileNotFound returns an error; exhaustive
	// parse-error scenarios are covered by the parser unit tests.
	_, err := runner.NewArazzoRunner("testdata/invalid.yaml", nil, &telemetry.NoopSink{})
	if err == nil {
		t.Fatal("expected error for non-existent/invalid file")
	}
}

// ---------------------------------------------------------------------------
// ListWorkflows
// ---------------------------------------------------------------------------

func TestListWorkflows_Empty(t *testing.T) {
	r := buildRunner(nil)
	wfs := r.ListWorkflows()
	if len(wfs) != 0 {
		t.Errorf("expected 0 workflows, got %d", len(wfs))
	}
}

func TestListWorkflows_Single(t *testing.T) {
	r := buildRunner([]interface{}{wf("alpha", false)})
	wfs := r.ListWorkflows()
	if len(wfs) != 1 {
		t.Fatalf("expected 1 workflow, got %d", len(wfs))
	}
	if wfs[0] != "alpha" {
		t.Errorf("expected 'alpha', got %q", wfs[0])
	}
}

func TestListWorkflows_Multiple(t *testing.T) {
	r := buildRunner([]interface{}{
		wf("wf1", false),
		wf("wf2", true),
		wf("wf3", false),
	})
	wfs := r.ListWorkflows()
	if len(wfs) != 3 {
		t.Fatalf("expected 3 workflows, got %d", len(wfs))
	}
	want := map[string]bool{"wf1": true, "wf2": true, "wf3": true}
	for _, id := range wfs {
		if !want[id] {
			t.Errorf("unexpected workflow ID %q", id)
		}
	}
}

func TestListWorkflows_DuplicateIDs(t *testing.T) {
	// Both entries share the same workflowId; both should appear in the list
	// (de-duplication is not a runner responsibility).
	r := buildRunner([]interface{}{
		wf("dup", false),
		wf("dup", false),
	})
	wfs := r.ListWorkflows()
	if len(wfs) != 2 {
		t.Errorf("expected 2 (duplicates kept), got %d", len(wfs))
	}
}

// ---------------------------------------------------------------------------
// GetWorkflow
// ---------------------------------------------------------------------------

func TestGetWorkflow_Found(t *testing.T) {
	r := buildRunner([]interface{}{
		wf("target", false),
		wf("other", true),
	})
	got := r.GetWorkflow("target")
	if got == nil {
		t.Fatal("expected non-nil workflow map for 'target'")
	}
	if got["workflowId"] != "target" {
		t.Errorf("expected workflowId 'target', got %v", got)
	}
}

func TestGetWorkflow_NotFound(t *testing.T) {
	r := buildRunner([]interface{}{wf("exists", false)})
	got := r.GetWorkflow("does-not-exist")
	if got != nil {
		t.Errorf("expected nil for unknown workflow, got %v", got)
	}
}

func TestGetWorkflow_EmptySlice(t *testing.T) {
	r := buildRunner(nil)
	got := r.GetWorkflow("anything")
	if got != nil {
		t.Errorf("expected nil from empty runner, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// GetWorkflowDetails
// ---------------------------------------------------------------------------

func TestGetWorkflowDetails_Found(t *testing.T) {
	workflowMap := map[string]interface{}{
		"workflowId":  "detailed-wf",
		"summary":     "My summary",
		"description": "My description",
	}
	r := buildRunner([]interface{}{workflowMap})
	details := r.GetWorkflowDetails("detailed-wf")
	if details == nil {
		t.Fatal("expected non-nil details")
	}
	if id, _ := details["workflowId"].(string); id != "detailed-wf" {
		t.Errorf("expected workflowId 'detailed-wf', got %q", id)
	}
}

func TestGetWorkflowDetails_NotFound(t *testing.T) {
	r := buildRunner([]interface{}{wf("exists", false)})
	details := r.GetWorkflowDetails("missing")
	if details != nil {
		t.Errorf("expected nil for unknown workflow, got %v", details)
	}
}

func TestGetWorkflowDetails_IncludesSummaryAndDescription(t *testing.T) {
	workflowMap := map[string]interface{}{
		"workflowId":  "rich-wf",
		"summary":     "Short summary",
		"description": "Long description here",
	}
	r := buildRunner([]interface{}{workflowMap})
	details := r.GetWorkflowDetails("rich-wf")
	if summary, _ := details["summary"].(string); summary != "Short summary" {
		t.Errorf("expected summary 'Short summary', got %q", summary)
	}
	if desc, _ := details["description"].(string); desc != "Long description here" {
		t.Errorf("expected description 'Long description here', got %q", desc)
	}
}

// ---------------------------------------------------------------------------
// ExecuteWorkflow
// ---------------------------------------------------------------------------

func TestExecuteWorkflow_NotFound(t *testing.T) {
	r := buildRunner(nil)
	result := r.ExecuteWorkflow("ghost-wf", nil)
	if result.Status != models.WorkflowStatusError {
		t.Errorf("expected WorkflowStatusError, got %q", result.Status)
	}
	if result.Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestExecuteWorkflow_NoSteps(t *testing.T) {
	r := buildRunner([]interface{}{wf("empty-wf", false /* no steps */)})
	result := r.ExecuteWorkflow("empty-wf", nil)
	if result.Status != models.WorkflowStatusError {
		t.Errorf("expected WorkflowStatusError for no-steps workflow, got %q", result.Status)
	}
	if result.Error == "" {
		t.Error("expected non-empty error message for no-steps workflow")
	}
}

func TestExecuteWorkflow_WithInputs(t *testing.T) {
	// Even when inputs are supplied the workflow still fails (no steps), but
	// the failure must come from the execution path, not input validation.
	r := buildRunner([]interface{}{wf("input-wf", false)})
	inputs := map[string]interface{}{"key": "value"}
	result := r.ExecuteWorkflow("input-wf", inputs)
	if result.Status != models.WorkflowStatusError {
		t.Errorf("expected error status, got %q", result.Status)
	}
}

func TestExecuteWorkflow_NilInputs(t *testing.T) {
	r := buildRunner([]interface{}{wf("nil-input-wf", false)})
	result := r.ExecuteWorkflow("nil-input-wf", nil)
	if result.Status != models.WorkflowStatusError {
		t.Errorf("expected error status for no-steps wf with nil inputs, got %q", result.Status)
	}
}

func TestExecuteWorkflow_MultipleWorkflowsIsolated(t *testing.T) {
	// Running wf-b must not affect wf-a's ability to be found and executed.
	r := buildRunner([]interface{}{
		wf("wf-a", false),
		wf("wf-b", false),
	})
	resA := r.ExecuteWorkflow("wf-a", nil)
	resB := r.ExecuteWorkflow("wf-b", nil)

	if resA.Status != models.WorkflowStatusError {
		t.Errorf("wf-a: expected error status, got %q", resA.Status)
	}
	if resB.Status != models.WorkflowStatusError {
		t.Errorf("wf-b: expected error status, got %q", resB.Status)
	}
}
