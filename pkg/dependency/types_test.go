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
