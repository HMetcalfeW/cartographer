package dependency_test

import (
	"testing"

	"github.com/HMetcalfeW/cartographer/pkg/dependency"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TestServiceSelectorOnlyMatchesPodControllers verifies that Service selectors
// only create edges to Pods/controllers, not to other resource types.
func TestServiceSelectorOnlyMatchesPodControllers(t *testing.T) {
	svc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata":   map[string]interface{}{"name": "my-svc"},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{"app": "test"},
			},
		},
	}
	// A Deployment with matching labels (should be matched)
	deploy := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":   "my-deploy",
				"labels": map[string]interface{}{"app": "test"},
			},
		},
	}
	// A ServiceAccount with matching labels (should NOT be matched)
	sa := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ServiceAccount",
			"metadata": map[string]interface{}{
				"name":   "my-sa",
				"labels": map[string]interface{}{"app": "test"},
			},
		},
	}
	// A ConfigMap with matching labels (should NOT be matched)
	cm := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":   "my-cm",
				"labels": map[string]interface{}{"app": "test"},
			},
		},
	}

	deps := dependency.BuildDependencies([]*unstructured.Unstructured{svc, deploy, sa, cm})
	svcEdges := deps["Service/my-svc"]

	require.Len(t, svcEdges, 1, "Service should only match the Deployment")
	assert.Equal(t, "Deployment/my-deploy", svcEdges[0].ChildID)
	assert.Equal(t, "selector", svcEdges[0].Reason)
}
