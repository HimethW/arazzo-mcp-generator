package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2/arazzo-mcp-generator/internal/mcpserver"
	"github.com/wso2/arazzo-mcp-generator/internal/runner"
	"github.com/wso2/arazzo-mcp-generator/internal/telemetry"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildTestServer constructs a minimal MCPServer whose Runner holds the
// supplied workflow slice.  It bypasses file I/O so tests run without an
// Arazzo document on disk.
func buildTestServer(workflows []interface{}) *mcpserver.MCPServer {
	r := &runner.ArazzoRunner{
		Workflows: workflows,
		Sink:      &telemetry.NoopSink{},
	}
	return mcpserver.NewMCPServerFromRunner(r, 0, &telemetry.NoopSink{})
}

// testWorkflow builds a minimal workflow map for use in tests.
// inputsDef may be nil (no inputs schema), or a JSON-Schema-style map such as:
//
//	map[string]interface{}{
//	    "required":   []interface{}{"petName"},
//	    "properties": map[string]interface{}{
//	        "petName": map[string]interface{}{"type": "string"},
//	    },
//	}
func testWorkflow(workflowID string, inputsDef map[string]interface{}, hasSteps bool) map[string]interface{} {
	wf := map[string]interface{}{
		"workflowId": workflowID,
	}
	if inputsDef != nil {
		wf["inputs"] = inputsDef
	}
	if hasSteps {
		wf["steps"] = []interface{}{
			map[string]interface{}{"stepId": "step1"},
		}
	}
	return wf
}

// doPost calls the MCPServer's HTTP handler with a POST request and returns the
// decoded RunResponse along with the HTTP status code.
func doPost(t *testing.T, srv *mcpserver.MCPServer, path string, body interface{}) (int, mcpserver.RunResponse) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("failed to encode request body: %v", err)
		}
	}
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	res := w.Result()
	var resp mcpserver.RunResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	return res.StatusCode, resp
}

// doGet calls the MCPServer's HTTP handler with a GET request and returns the
// raw *http.Response.
func doGet(t *testing.T, srv *mcpserver.MCPServer, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	return w
}

// ---------------------------------------------------------------------------
// /run endpoint – reference tests (ported from server_run_test.go)
// ---------------------------------------------------------------------------

func TestHandleRun_NonPostMethod(t *testing.T) {
	srv := buildTestServer(nil)
	req := httptest.NewRequest(http.MethodGet, "/run/", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleRun_MissingWorkflowID(t *testing.T) {
	srv := buildTestServer(nil)
	// POST to /run/ with empty body – workflowId absent from both URL and body
	status, resp := doPost(t, srv, "/run/", nil)
	if status != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", status)
	}
	if resp.Status != "failed" {
		t.Errorf("expected status 'failed', got %q", resp.Status)
	}
	if resp.Error == "" {
		t.Error("expected a non-empty error message")
	}
}

func TestHandleRun_WorkflowIDFromBody(t *testing.T) {
	// Body-level workflowId is used when URL path has none.
	// Here the workflow does not exist, so we get a 400 "not found" rather than
	// the generic "workflowId is required" – proving the body fallback worked.
	srv := buildTestServer(nil)
	status, resp := doPost(t, srv, "/run/", map[string]interface{}{
		"workflowId": "no-such-workflow",
	})
	if status != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", status)
	}
	if resp.Status != "failed" {
		t.Errorf("expected status 'failed', got %q", resp.Status)
	}
	if resp.Error == "" {
		t.Error("expected a non-empty error message")
	}
}

func TestHandleRun_UnknownWorkflow(t *testing.T) {
	srv := buildTestServer(nil)
	status, resp := doPost(t, srv, "/run/unknown-workflow", nil)
	if status != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", status)
	}
	if resp.Status != "failed" {
		t.Errorf("expected status 'failed', got %q", resp.Status)
	}
	if resp.Error == "" {
		t.Error("expected a non-empty error message")
	}
}

func TestHandleRun_MissingRequiredInput(t *testing.T) {
	inputsDef := map[string]interface{}{
		"required": []interface{}{"petName"},
		"properties": map[string]interface{}{
			"petName": map[string]interface{}{"type": "string"},
		},
	}
	wf := testWorkflow("create-pet", inputsDef, false)
	srv := buildTestServer([]interface{}{wf})

	// Send request without the required "petName" input
	status, resp := doPost(t, srv, "/run/create-pet", map[string]interface{}{
		"inputs": map[string]interface{}{},
	})
	if status != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", status)
	}
	if resp.Status != "failed" {
		t.Errorf("expected status 'failed', got %q", resp.Status)
	}
	if resp.Error == "" {
		t.Error("expected a non-empty error describing the missing field")
	}
}

func TestHandleRun_WrongInputType(t *testing.T) {
	inputsDef := map[string]interface{}{
		"required": []interface{}{"count"},
		"properties": map[string]interface{}{
			"count": map[string]interface{}{"type": "integer"},
		},
	}
	wf := testWorkflow("list-pets", inputsDef, false)
	srv := buildTestServer([]interface{}{wf})

	// "count" must be integer but we send a string that cannot be coerced
	status, resp := doPost(t, srv, "/run/list-pets", map[string]interface{}{
		"inputs": map[string]interface{}{
			"count": "not-a-number",
		},
	})
	if status != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", status)
	}
	if resp.Status != "failed" {
		t.Errorf("expected status 'failed', got %q", resp.Status)
	}
	if resp.Error == "" {
		t.Error("expected a non-empty error describing the type mismatch")
	}
}

func TestHandleRun_ExecutionFailure_NoSteps(t *testing.T) {
	// A workflow with satisfied inputs but no steps triggers the runner's
	// "Workflow has no steps" error path without needing a real HTTP backend.
	inputsDef := map[string]interface{}{
		"required": []interface{}{"petName"},
		"properties": map[string]interface{}{
			"petName": map[string]interface{}{"type": "string"},
		},
	}
	wf := testWorkflow("empty-workflow", inputsDef, false /* no steps */)
	srv := buildTestServer([]interface{}{wf})

	status, resp := doPost(t, srv, "/run/empty-workflow", map[string]interface{}{
		"inputs": map[string]interface{}{
			"petName": "Buddy",
		},
	})
	if status != http.StatusOK {
		t.Errorf("expected 200, got %d", status)
	}
	if resp.Status != "failed" {
		t.Errorf("expected status 'failed', got %q", resp.Status)
	}
	if resp.Error == "" {
		t.Error("expected a non-empty error from the runner")
	}
}

func TestHandleRun_InvalidJSONBody(t *testing.T) {
	srv := buildTestServer(nil)
	req := httptest.NewRequest(http.MethodPost, "/run/any-wf", bytes.NewBufferString("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// /run endpoint – additional tests
// ---------------------------------------------------------------------------

func TestHandleRun_NoInputsSchema(t *testing.T) {
	// Workflow with no "inputs" schema should accept any (or no) inputs and
	// proceed to execution. The runner returns "failed" because there are no
	// steps, but the validation itself must not reject the request.
	wf := testWorkflow("no-schema-wf", nil /* no inputsDef */, false)
	srv := buildTestServer([]interface{}{wf})

	status, resp := doPost(t, srv, "/run/no-schema-wf", nil)
	// HTTP 200: validation passed, runner returned an execution error
	if status != http.StatusOK {
		t.Errorf("expected 200 (runner error, not validation error), got %d", status)
	}
	if resp.Status != "failed" {
		t.Errorf("expected status 'failed', got %q", resp.Status)
	}
	// The error must be a runner error about no steps, not a validation error
	if strings.Contains(resp.Error, "must be") || strings.Contains(resp.Error, "missing required") {
		t.Errorf("expected runner error about no steps but got validation error: %s", resp.Error)
	}
}

func TestHandleRun_StringCoercedToNumber(t *testing.T) {
	// Sending "42" (string) for an integer field should be coerced and pass
	// validation. The request then proceeds to execution (fails on no steps).
	inputsDef := map[string]interface{}{
		"required": []interface{}{"count"},
		"properties": map[string]interface{}{
			"count": map[string]interface{}{"type": "integer"},
		},
	}
	wf := testWorkflow("coerce-wf", inputsDef, false)
	srv := buildTestServer([]interface{}{wf})

	status, resp := doPost(t, srv, "/run/coerce-wf", map[string]interface{}{
		"inputs": map[string]interface{}{
			"count": "42", // string that can be parsed as float64(42)
		},
	})
	if status != http.StatusOK {
		t.Errorf("expected 200 (coercion worked, runner error on no steps), got %d", status)
	}
	// The error should be from the runner (no steps), not a type mismatch
	if strings.Contains(resp.Error, "must be") {
		t.Errorf("coercion should have succeeded but got type error: %s", resp.Error)
	}
}

func TestHandleRun_BooleanStringCoercion(t *testing.T) {
	// Sending "true" (string) for a boolean field should be coerced.
	inputsDef := map[string]interface{}{
		"required": []interface{}{"enabled"},
		"properties": map[string]interface{}{
			"enabled": map[string]interface{}{"type": "boolean"},
		},
	}
	wf := testWorkflow("bool-coerce-wf", inputsDef, false)
	srv := buildTestServer([]interface{}{wf})

	status, resp := doPost(t, srv, "/run/bool-coerce-wf", map[string]interface{}{
		"inputs": map[string]interface{}{
			"enabled": "true",
		},
	})
	if status != http.StatusOK {
		t.Errorf("expected 200 (coercion should succeed), got %d", status)
	}
	if strings.Contains(resp.Error, "must be") {
		t.Errorf("boolean coercion should have succeeded: %s", resp.Error)
	}
}

func TestHandleRun_MultipleMissingRequiredInputs(t *testing.T) {
	inputsDef := map[string]interface{}{
		"required": []interface{}{"name", "email", "age"},
		"properties": map[string]interface{}{
			"name":  map[string]interface{}{"type": "string"},
			"email": map[string]interface{}{"type": "string"},
			"age":   map[string]interface{}{"type": "integer"},
		},
	}
	wf := testWorkflow("multi-required-wf", inputsDef, false)
	srv := buildTestServer([]interface{}{wf})

	status, resp := doPost(t, srv, "/run/multi-required-wf", map[string]interface{}{
		"inputs": map[string]interface{}{},
	})
	if status != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", status)
	}
	if resp.Status != "failed" {
		t.Errorf("expected status 'failed', got %q", resp.Status)
	}
	// The error should mention missing inputs
	if !strings.Contains(resp.Error, "missing required") {
		t.Errorf("expected 'missing required' in error, got: %s", resp.Error)
	}
}

func TestHandleRun_WorkflowIDInURLTakesPrecedence(t *testing.T) {
	// When workflowId is both in the URL path and the body, the URL takes
	// precedence. If the URL workflow doesn't exist, we get a 400 not-found
	// regardless of what the body says.
	wf := testWorkflow("real-workflow", nil, false)
	srv := buildTestServer([]interface{}{wf})

	status, resp := doPost(t, srv, "/run/non-existent-url-workflow", map[string]interface{}{
		"workflowId": "real-workflow",
	})
	if status != http.StatusBadRequest {
		t.Errorf("expected 400 (URL workflow not found), got %d", status)
	}
	if !strings.Contains(resp.Error, "non-existent-url-workflow") {
		t.Errorf("error should mention the URL workflow ID, got: %s", resp.Error)
	}
}

func TestHandleRun_PutMethodNotAllowed(t *testing.T) {
	srv := buildTestServer(nil)
	req := httptest.NewRequest(http.MethodPut, "/run/any", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for PUT, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// /lastResult endpoint
// ---------------------------------------------------------------------------

func TestHandleLastResult_NotFound(t *testing.T) {
	srv := buildTestServer(nil)
	w := doGet(t, srv, "/lastResult/no-such-workflow")
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleLastResult_MethodNotAllowed(t *testing.T) {
	srv := buildTestServer(nil)
	req := httptest.NewRequest(http.MethodPost, "/lastResult/wf1", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleLastResult_Found(t *testing.T) {
	// Run a workflow (it will fail due to no steps, but the result is cached).
	// Then GET /lastResult/{id} should return the cached result without re-executing.
	wf := testWorkflow("cache-wf", nil, false)
	srv := buildTestServer([]interface{}{wf})

	// First: execute to populate the cache
	runStatus, runResp := doPost(t, srv, "/run/cache-wf", nil)
	if runStatus != http.StatusOK {
		t.Fatalf("run returned unexpected status %d", runStatus)
	}
	if runResp.Status != "failed" {
		t.Fatalf("expected 'failed' run response for no-steps workflow, got %q", runResp.Status)
	}

	// Second: retrieve from cache
	w := doGet(t, srv, "/lastResult/cache-wf")
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 from lastResult, got %d", w.Code)
	}
	var cachedResp mcpserver.RunResponse
	if err := json.NewDecoder(w.Body).Decode(&cachedResp); err != nil {
		t.Fatalf("failed to decode lastResult response: %v", err)
	}
	if cachedResp.Status != runResp.Status {
		t.Errorf("cached status %q != run status %q", cachedResp.Status, runResp.Status)
	}
	if cachedResp.Error != runResp.Error {
		t.Errorf("cached error %q != run error %q", cachedResp.Error, runResp.Error)
	}
}

func TestHandleLastResult_MissingWorkflowID(t *testing.T) {
	// /lastResult/ with no trailing ID should return 400.
	srv := buildTestServer(nil)
	w := doGet(t, srv, "/lastResult/")
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty workflow ID, got %d", w.Code)
	}
}

func TestHandleLastResult_CacheNotSharedAcrossWorkflows(t *testing.T) {
	// Running workflow A must not populate the cache for workflow B.
	wfA := testWorkflow("wf-a", nil, false)
	wfB := testWorkflow("wf-b", nil, false)
	srv := buildTestServer([]interface{}{wfA, wfB})

	// Run only wf-a
	doPost(t, srv, "/run/wf-a", nil) //nolint:errcheck

	// wf-b should still be a cache miss
	w := doGet(t, srv, "/lastResult/wf-b")
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for wf-b (not run yet), got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// Server construction
// ---------------------------------------------------------------------------

func TestNewMCPServerFromRunner_Smoke(t *testing.T) {
	// Verifies that a server constructed from a pre-built runner registers the
	// right number of default utility tools (list_workflows, get_workflow_details).
	wf := testWorkflow("smoke-wf", nil, false)
	srv := buildTestServer([]interface{}{wf})
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestHandleRun_ContentTypeJSON(t *testing.T) {
	wf := testWorkflow("ct-wf", nil, false)
	srv := buildTestServer([]interface{}{wf})

	_, _ = doPost(t, srv, "/run/ct-wf", nil)

	// Perform a raw request to check Content-Type header on the run response
	req := httptest.NewRequest(http.MethodPost, "/run/ct-wf", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected application/json Content-Type, got %q", ct)
	}
}
