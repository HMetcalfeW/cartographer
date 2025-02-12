package dependency_test

import (
	"testing"

	"github.com/HMetcalfeW/cartographer/pkg/dependency"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestBuildDependencies(t *testing.T) {
	// Create a Deployment
	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "test-deployment",
				"labels": map[string]interface{}{
					"app": "my-app",
				},
			},
		},
	}

	// Create a ReplicaSet owned by the Deployment
	replicaSet := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "ReplicaSet",
			"metadata": map[string]interface{}{
				"name": "test-replicaset",
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"name":       "test-deployment",
					},
				},
			},
		},
	}

	// Create a Pod owned by the ReplicaSet
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test-pod",
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "ReplicaSet",
						"name":       "test-replicaset",
					},
				},
			},
		},
	}

	// Create a Service with a selector matching the Deployment label
	service := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"name": "test-service",
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"app": "my-app",
				},
			},
		},
	}

	// Convert all to Unstructured objects
	objs := []*unstructured.Unstructured{deployment, replicaSet, pod, service}

	// Build the dependencies.
	deps := dependency.BuildDependencies(objs)

	// Optional: Print them for debugging (comment out if not needed).
	// dependency.PrintDependencies(deps)

	// Check dependencies:
	// 1. Deployment -> ReplicaSet
	assert.Contains(t, deps["Deployment/test-deployment"], "ReplicaSet/test-replicaset",
		"Expected the Deployment to have a child ReplicaSet")

	// 2. ReplicaSet -> Pod
	assert.Contains(t, deps["ReplicaSet/test-replicaset"], "Pod/test-pod",
		"Expected the ReplicaSet to have a child Pod")

	// 3. Service -> Deployment
	assert.Contains(t, deps["Service/test-service"], "Deployment/test-deployment",
		"Expected the Service to target the Deployment via label selector")

	// 4. Pod has no children
	_, hasPodChildren := deps["Pod/test-pod"]
	assert.False(t, hasPodChildren, "Expected no child resources for Pod")
}
