package dependency_test

import (
	"testing"

	"github.com/HMetcalfeW/cartographer/pkg/dependency"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TestIsPodOrController checks recognized Kinds.
func TestIsPodOrController(t *testing.T) {
	tests := []struct {
		kind     string
		expected bool
	}{
		{"Pod", true},
		{"Deployment", true},
		{"Job", true},
		{"CronJob", true},
		{"Service", false},
		{"CustomKind", false},
	}
	for _, tt := range tests {
		obj := &unstructured.Unstructured{}
		obj.SetKind(tt.kind)
		res := dependency.IsPodOrController(obj)
		assert.Equalf(t, tt.expected, res, "kind=%s", tt.kind)
	}
}

// TestResourceID ensures we get "Kind/Name".
func TestResourceID(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetKind("Deployment")
	obj.SetName("my-deploy")
	id := dependency.ResourceID(obj)
	assert.Equal(t, "Deployment/my-deploy", id)
}

// TestLabelsMatch covers label comparisons.
func TestLabelsMatch(t *testing.T) {
	selector := map[string]string{"app": "webapp", "tier": "frontend"}
	labels1 := map[string]string{"app": "webapp", "tier": "frontend", "extra": "yes"}
	labels2 := map[string]string{"app": "webapp"}
	assert.True(t, dependency.LabelsMatch(selector, labels1), "should match superset")
	assert.False(t, dependency.LabelsMatch(selector, labels2), "missing tier=frontend")
}

// TestMapInterfaceToStringMap ensures it handles typical map[string]interface{} input.
func TestMapInterfaceToStringMap(t *testing.T) {
	in := map[string]interface{}{
		"app": "webapp",
		"rep": 3, // non-string, should be ignored
	}
	out := dependency.MapInterfaceToStringMap(in)
	assert.Equal(t, 1, len(out))
	assert.Equal(t, "webapp", out["app"])
}

// TestMatchesExpressions covers all four operators and AND semantics.
func TestMatchesExpressions(t *testing.T) {
	labels := map[string]string{"app": "web", "tier": "frontend", "env": "prod"}

	tests := []struct {
		name     string
		exprs    []dependency.LabelSelectorRequirement
		expected bool
	}{
		{"In matching value", []dependency.LabelSelectorRequirement{
			{Key: "env", Operator: "In", Values: []string{"prod", "staging"}},
		}, true},
		{"In non-matching value", []dependency.LabelSelectorRequirement{
			{Key: "env", Operator: "In", Values: []string{"dev", "staging"}},
		}, false},
		{"In missing key", []dependency.LabelSelectorRequirement{
			{Key: "region", Operator: "In", Values: []string{"us-east"}},
		}, false},
		{"NotIn excluded value", []dependency.LabelSelectorRequirement{
			{Key: "env", Operator: "NotIn", Values: []string{"prod"}},
		}, false},
		{"NotIn allowed value", []dependency.LabelSelectorRequirement{
			{Key: "env", Operator: "NotIn", Values: []string{"staging"}},
		}, true},
		{"NotIn missing key", []dependency.LabelSelectorRequirement{
			{Key: "region", Operator: "NotIn", Values: []string{"us-east"}},
		}, true},
		{"Exists present key", []dependency.LabelSelectorRequirement{
			{Key: "app", Operator: "Exists"},
		}, true},
		{"Exists missing key", []dependency.LabelSelectorRequirement{
			{Key: "missing", Operator: "Exists"},
		}, false},
		{"DoesNotExist missing key", []dependency.LabelSelectorRequirement{
			{Key: "missing", Operator: "DoesNotExist"},
		}, true},
		{"DoesNotExist present key", []dependency.LabelSelectorRequirement{
			{Key: "app", Operator: "DoesNotExist"},
		}, false},
		{"multiple expressions AND", []dependency.LabelSelectorRequirement{
			{Key: "env", Operator: "In", Values: []string{"prod"}},
			{Key: "tier", Operator: "Exists"},
			{Key: "deprecated", Operator: "DoesNotExist"},
		}, true},
		{"multiple expressions AND fails", []dependency.LabelSelectorRequirement{
			{Key: "env", Operator: "In", Values: []string{"prod"}},
			{Key: "missing", Operator: "Exists"},
		}, false},
		{"empty expressions", []dependency.LabelSelectorRequirement{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dependency.MatchesExpressions(tt.exprs, labels)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestExtractMatchExpressions verifies extraction from unstructured maps.
func TestExtractMatchExpressions(t *testing.T) {
	// Valid expressions
	selectorMap := map[string]interface{}{
		"matchExpressions": []interface{}{
			map[string]interface{}{
				"key":      "env",
				"operator": "In",
				"values":   []interface{}{"prod", "staging"},
			},
			map[string]interface{}{
				"key":      "release",
				"operator": "Exists",
			},
		},
	}
	exprs := dependency.ExtractMatchExpressions(selectorMap)
	assert.Len(t, exprs, 2)
	assert.Equal(t, "env", exprs[0].Key)
	assert.Equal(t, "In", exprs[0].Operator)
	assert.Equal(t, []string{"prod", "staging"}, exprs[0].Values)
	assert.Equal(t, "release", exprs[1].Key)
	assert.Equal(t, "Exists", exprs[1].Operator)

	// Empty/missing matchExpressions
	assert.Nil(t, dependency.ExtractMatchExpressions(map[string]interface{}{}))

	// Malformed entries skipped
	malformed := map[string]interface{}{
		"matchExpressions": []interface{}{
			"not a map",
			map[string]interface{}{"key": "x"}, // missing operator
			map[string]interface{}{"key": "y", "operator": "In", "values": []interface{}{"v1"}},
		},
	}
	exprs = dependency.ExtractMatchExpressions(malformed)
	assert.Len(t, exprs, 1, "only the valid entry should be extracted")
	assert.Equal(t, "y", exprs[0].Key)
}

// TestLabelIndexMatchSelector verifies the combined matchLabels+matchExpressions path.
func TestLabelIndexMatchSelector(t *testing.T) {
	web := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1", "kind": "Deployment",
			"metadata": map[string]interface{}{
				"name":   "web",
				"labels": map[string]interface{}{"app": "web", "tier": "frontend", "env": "prod"},
			},
		},
	}
	api := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1", "kind": "Deployment",
			"metadata": map[string]interface{}{
				"name":   "api",
				"labels": map[string]interface{}{"app": "api", "tier": "backend", "env": "prod"},
			},
		},
	}
	worker := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1", "kind": "Deployment",
			"metadata": map[string]interface{}{
				"name":   "worker",
				"labels": map[string]interface{}{"app": "worker", "tier": "backend", "env": "staging"},
			},
		},
	}

	idx := dependency.BuildLabelIndex([]*unstructured.Unstructured{web, api, worker})

	// matchLabels only — same as Match()
	results := idx.MatchSelector(map[string]string{"tier": "backend"}, nil)
	assert.Len(t, results, 2)

	// matchExpressions only — In operator
	results = idx.MatchSelector(nil, []dependency.LabelSelectorRequirement{
		{Key: "env", Operator: "In", Values: []string{"prod"}},
	})
	assert.Len(t, results, 2) // web + api

	// Combined matchLabels + matchExpressions
	results = idx.MatchSelector(
		map[string]string{"tier": "backend"},
		[]dependency.LabelSelectorRequirement{
			{Key: "env", Operator: "NotIn", Values: []string{"staging"}},
		},
	)
	assert.Len(t, results, 1)
	assert.Equal(t, "api", results[0].GetName())

	// Both empty — nil
	results = idx.MatchSelector(nil, nil)
	assert.Nil(t, results)

	// matchExpressions with DoesNotExist
	results = idx.MatchSelector(nil, []dependency.LabelSelectorRequirement{
		{Key: "deprecated", Operator: "DoesNotExist"},
	})
	assert.Len(t, results, 3, "all objects lack the deprecated label")
}

// TestDeduplicateEdges verifies that duplicate edges are removed from the dependency map.
func TestDeduplicateEdges(t *testing.T) {
	// Create a Deployment that references the same secret in two places (volume + env)
	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   map[string]interface{}{"name": "dup-deploy"},
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"volumes": []interface{}{
							map[string]interface{}{
								"name":   "s1",
								"secret": map[string]interface{}{"secretName": "shared-secret"},
							},
							// same secret referenced again via a second volume
							map[string]interface{}{
								"name":   "s2",
								"secret": map[string]interface{}{"secretName": "shared-secret"},
							},
						},
					},
				},
			},
		},
	}

	deps := dependency.BuildDependencies([]*unstructured.Unstructured{deployment})
	edges := deps["Deployment/dup-deploy"]

	// Count secretRef edges to shared-secret — should be exactly 1 after dedup
	count := 0
	for _, e := range edges {
		if e.ChildID == "Secret/shared-secret" && e.Reason == "secretRef" {
			count++
		}
	}
	assert.Equal(t, 1, count, "duplicate secretRef edges should be deduplicated")
}
