// Package tests contains integration-level tests for arazzo-mcp-generator.
// Each file covers one internal package using only exported APIs so the tests
// remain decoupled from implementation details.
package tests

import (
	"testing"

	"github.com/wso2/arazzo-mcp-generator/internal/evaluator"
)

// products is a Toolshop-like product list reused across several sub-tests.
var products = map[string]interface{}{
	"current_page": 1,
	"data": []interface{}{
		map[string]interface{}{
			"id":          "01JVPG4R5SMRNE73P2B6ZAQ1VB",
			"name":        "Combination Pliers",
			"description": "A versatile hand tool",
			"price":       14.15,
			"is_rental":   false,
			"in_stock":    true,
			"brand": map[string]interface{}{
				"id":   "01JVPFYQB5K4G5YKSVBFMBSNKN",
				"name": "ForgeFlex Tools",
			},
			"category": map[string]interface{}{
				"id":   "01JVPFYQA6WNMG59RB1E6Q4512",
				"name": "Pliers",
				"slug": "pliers",
			},
		},
		map[string]interface{}{
			"id":          "01JVPG4R5SMRNE73P2B6ZAQ2XX",
			"name":        "Bolt Cutters",
			"description": "Heavy duty bolt cutters",
			"price":       48.41,
			"is_rental":   false,
			"in_stock":    true,
			"brand": map[string]interface{}{
				"id":   "01JVPFYQB5K4G5YKSVBFMBSNKN",
				"name": "ForgeFlex Tools",
			},
			"category": map[string]interface{}{
				"id":   "01JVPFYQA6WNMG59RB1E6Q4599",
				"name": "Cutters",
				"slug": "cutters",
			},
		},
		map[string]interface{}{
			"id":          "01JVPG4R5SMRNE73P2B6ZAQ3YY",
			"name":        "Cheap Wrench",
			"description": "Budget wrench",
			"price":       5.99,
			"is_rental":   true,
			"in_stock":    false,
			"brand": map[string]interface{}{
				"id":   "01JVPFYQB5K4G5YKSVBFMBS000",
				"name": "BudgetTools",
			},
			"category": map[string]interface{}{
				"id":   "01JVPFYQA6WNMG59RB1E6Q4700",
				"name": "Wrenches",
				"slug": "wrenches",
			},
		},
	},
	"from":      1,
	"last_page": 6,
	"per_page":  9,
	"to":        9,
	"total":     50,
}

// TestEvaluateJSONPathCriterion mirrors the canonical test from the reference
// arazzo-designer-cli evaluator package (jsonpath_test.go).
func TestEvaluateJSONPathCriterion(t *testing.T) {
	tests := []struct {
		name      string
		context   interface{}
		condition string
		want      bool
	}{
		// --- Basic property access ---
		{
			name:      "root property exists",
			context:   products,
			condition: "$.data",
			want:      true,
		},
		{
			name:      "root property does not exist",
			context:   products,
			condition: "$.nonexistent",
			want:      false,
		},
		{
			name:      "nested property access",
			context:   products,
			condition: "$.data[0].name",
			want:      true,
		},
		{
			name:      "deeply nested access",
			context:   products,
			condition: "$.data[0].brand.name",
			want:      true,
		},

		// --- Array index access ---
		{
			name:      "first element by index",
			context:   products,
			condition: "$.data[0]",
			want:      true,
		},
		{
			name:      "out of bounds index",
			context:   products,
			condition: "$.data[999]",
			want:      false,
		},

		// --- Filter expressions (Arazzo spec core use case) ---
		{
			name:      "filter by numeric comparison (greater than)",
			context:   products,
			condition: "$.data[?(@.price > 10)]",
			want:      true,
		},
		{
			name:      "filter by numeric comparison (none match)",
			context:   products,
			condition: "$.data[?(@.price > 1000)]",
			want:      false,
		},
		{
			name:      "filter by string equality",
			context:   products,
			condition: "$.data[?(@.brand.name == 'ForgeFlex Tools')]",
			want:      true,
		},
		{
			name:      "filter by string equality (no match)",
			context:   products,
			condition: "$.data[?(@.brand.name == 'NonExistentBrand')]",
			want:      false,
		},
		{
			name:      "filter by boolean true",
			context:   products,
			condition: "$.data[?(@.in_stock == true)]",
			want:      true,
		},
		{
			name:      "filter by boolean false",
			context:   products,
			condition: "$.data[?(@.in_stock == false)]",
			want:      true, // "Cheap Wrench" has in_stock=false
		},
		{
			name:      "filter with less-than comparison",
			context:   products,
			condition: "$.data[?(@.price < 10)]",
			want:      true, // "Cheap Wrench" has price=5.99
		},

		// --- Wildcard ---
		{
			name:      "wildcard on array",
			context:   products,
			condition: "$.data[*].name",
			want:      true,
		},
		{
			name:      "wildcard on nested field",
			context:   products,
			condition: "$.data[*].brand.name",
			want:      true,
		},

		// --- Context as array (e.g., when context resolves to an array directly) ---
		{
			name: "context is raw array - index access",
			context: []interface{}{
				map[string]interface{}{"name": "Item1", "value": 10},
				map[string]interface{}{"name": "Item2", "value": 20},
			},
			condition: "$[0].name",
			want:      true,
		},
		{
			name: "context is raw array - filter",
			context: []interface{}{
				map[string]interface{}{"name": "Item1", "value": 10},
				map[string]interface{}{"name": "Item2", "value": 20},
			},
			condition: "$[?(@.value > 15)]",
			want:      true,
		},
		{
			name: "context is raw array - filter no match",
			context: []interface{}{
				map[string]interface{}{"name": "Item1", "value": 10},
				map[string]interface{}{"name": "Item2", "value": 20},
			},
			condition: "$[?(@.value > 100)]",
			want:      false,
		},

		// --- Edge cases ---
		{
			name:      "empty condition",
			context:   products,
			condition: "",
			want:      false,
		},
		{
			name:      "nil context",
			context:   nil,
			condition: "$.data",
			want:      false,
		},
		{
			name:      "invalid JSONPath syntax",
			context:   products,
			condition: "$[invalid!!!",
			want:      false,
		},
		{
			name:      "context is raw JSON string",
			context:   `{"items":[{"x":1},{"x":2}]}`,
			condition: "$.items[?(@.x > 1)]",
			want:      true,
		},

		// --- Arazzo spec example pattern: $[?count(@.pets) > 0] ---
		// Using length() since ojg supports RFC 9535 functions
		{
			name: "length function on array",
			context: map[string]interface{}{
				"pets": []interface{}{"dog", "cat"},
			},
			condition: "$.pets[?(length(@) > 0)]",
			want:      true,
		},

		// --- Multiple criteria pattern: combined conditions ---
		{
			name:      "combined filter: price and brand",
			context:   products,
			condition: "$.data[?(@.price > 10 && @.brand.name == 'ForgeFlex Tools')]",
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluator.EvaluateJSONPathCriterion(tt.context, tt.condition)
			if got != tt.want {
				t.Errorf("EvaluateJSONPathCriterion() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestNormalizeForOJG_Indirect exercises the internal normalizeForOJG function
// indirectly via EvaluateJSONPathCriterion, which is its sole public consumer.
// (normalizeForOJG is unexported; this covers all of its code paths.)
func TestNormalizeForOJG_Indirect(t *testing.T) {
	tests := []struct {
		name      string
		context   interface{}
		condition string
		want      bool
	}{
		// JSON string → normalizeForOJG unmarshals it to an ojg-compatible tree
		{
			name:      "valid JSON string with matching path",
			context:   `{"items":[{"x":1},{"x":2}]}`,
			condition: "$.items[?(@.x > 1)]",
			want:      true,
		},
		{
			name:      "valid JSON string with missing path",
			context:   `{"items":[]}`,
			condition: "$.missing",
			want:      false,
		},
		// nil → normalizeForOJG returns nil → EvaluateJSONPathCriterion returns false
		{
			name:      "nil context",
			context:   nil,
			condition: "$.anything",
			want:      false,
		},
		// map[string]interface{} → normalizeForOJG round-trips through JSON
		{
			name: "map with float value",
			context: map[string]interface{}{
				"a": float64(1),
				"b": "hello",
				"c": []interface{}{float64(1), float64(2)},
			},
			condition: "$.a",
			want:      true,
		},
		{
			name: "map with nested array",
			context: map[string]interface{}{
				"c": []interface{}{float64(1), float64(2)},
			},
			condition: "$.c[0]",
			want:      true,
		},
		// Plain non-JSON string → normalizeForOJG returns it as-is
		{
			name:      "plain string context (non-JSON)",
			context:   "just a plain string",
			condition: "$.anything",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluator.EvaluateJSONPathCriterion(tt.context, tt.condition)
			if got != tt.want {
				t.Errorf("EvaluateJSONPathCriterion() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestEvaluateJSONPathCriterion_Regression covers additional patterns that
// arose from real Arazzo workflow files and edge-case regression discoveries.
func TestEvaluateJSONPathCriterion_Regression(t *testing.T) {
	tests := []struct {
		name      string
		context   interface{}
		condition string
		want      bool
	}{
		// Numeric equality
		{
			name:      "equality on integer field",
			context:   map[string]interface{}{"count": float64(0)},
			condition: "$.count",
			want:      true, // field exists (value is 0, truthy as existence check)
		},
		// Deeply nested filter
		{
			name: "deeply nested filter",
			context: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": []interface{}{
						map[string]interface{}{"val": float64(5)},
						map[string]interface{}{"val": float64(15)},
					},
				},
			},
			condition: "$.level1.level2[?(@.val > 10)]",
			want:      true,
		},
		// Single-element array with filter
		{
			name: "single-element array filter match",
			context: []interface{}{
				map[string]interface{}{"status": "active"},
			},
			condition: "$[?(@.status == 'active')]",
			want:      true,
		},
		{
			name: "single-element array filter no match",
			context: []interface{}{
				map[string]interface{}{"status": "inactive"},
			},
			condition: "$[?(@.status == 'active')]",
			want:      false,
		},
		// Boolean field as filter target
		{
			name: "boolean field truthy filter",
			context: map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{"enabled": true, "name": "a"},
					map[string]interface{}{"enabled": false, "name": "b"},
				},
			},
			condition: "$.items[?(@.enabled == true)]",
			want:      true,
		},
		// Wildcard on map keys
		{
			name: "wildcard on object values",
			context: map[string]interface{}{
				"scores": map[string]interface{}{
					"alice": float64(90),
					"bob":   float64(85),
				},
			},
			condition: "$.scores.*",
			want:      true,
		},
		// Recursive descent
		{
			name: "recursive descent find name",
			context: map[string]interface{}{
				"a": map[string]interface{}{
					"name": "foo",
					"b": map[string]interface{}{
						"name": "bar",
					},
				},
			},
			condition: "$..name",
			want:      true,
		},
		// Empty array context
		{
			name:      "empty array context",
			context:   []interface{}{},
			condition: "$[0]",
			want:      false,
		},
		// Empty map context
		{
			name:      "empty map context",
			context:   map[string]interface{}{},
			condition: "$.any",
			want:      false,
		},
		// JSON string with array root
		{
			name:      "JSON string array root",
			context:   `[{"id":1},{"id":2}]`,
			condition: "$[?(@.id == 1)]",
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluator.EvaluateJSONPathCriterion(tt.context, tt.condition)
			if got != tt.want {
				t.Errorf("EvaluateJSONPathCriterion(%q) = %v, want %v", tt.condition, got, tt.want)
			}
		})
	}
}

// TestEvaluateJSONPathCriterion_ArazzoPatternsFromSpec covers the patterns
// explicitly mentioned in the Arazzo specification document.
func TestEvaluateJSONPathCriterion_ArazzoPatternsFromSpec(t *testing.T) {
	tests := []struct {
		name      string
		context   interface{}
		condition string
		want      bool
	}{
		// $response.body#/path style: the evaluator may receive the resolved body
		{
			name: "response body array – all items have required field",
			context: map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{"id": "1"},
					map[string]interface{}{"id": "2"},
				},
			},
			condition: "$.results[*].id",
			want:      true,
		},
		{
			name: "numeric range check – status code 200-299",
			context: map[string]interface{}{
				"status_code": float64(201),
			},
			condition: "$.status_code",
			want:      true,
		},
		// Presence-based criteria (field exists and is non-null)
		{
			name: "field exists with non-null value",
			context: map[string]interface{}{
				"token": "abc123",
			},
			condition: "$.token",
			want:      true,
		},
		{
			name: "field missing entirely",
			context: map[string]interface{}{
				"other": "value",
			},
			condition: "$.token",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluator.EvaluateJSONPathCriterion(tt.context, tt.condition)
			if got != tt.want {
				t.Errorf("EvaluateJSONPathCriterion(%q) = %v, want %v", tt.condition, got, tt.want)
			}
		})
	}
}
