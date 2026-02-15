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

// TestNetworkPolicyMatchesPodControllers verifies NetworkPolicy podSelector
// creates edges to matching Pods/controllers.
func TestNetworkPolicyMatchesPodControllers(t *testing.T) {
	np := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.k8s.io/v1",
			"kind":       "NetworkPolicy",
			"metadata":   map[string]interface{}{"name": "deny-all"},
			"spec": map[string]interface{}{
				"podSelector": map[string]interface{}{
					"matchLabels": map[string]interface{}{"role": "db"},
				},
			},
		},
	}
	sts := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "StatefulSet",
			"metadata": map[string]interface{}{
				"name":   "postgres",
				"labels": map[string]interface{}{"role": "db"},
			},
		},
	}
	// ConfigMap with matching label — should NOT be matched
	cm := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":   "db-config",
				"labels": map[string]interface{}{"role": "db"},
			},
		},
	}

	deps := dependency.BuildDependencies([]*unstructured.Unstructured{np, sts, cm})
	npEdges := deps["NetworkPolicy/deny-all"]

	require.Len(t, npEdges, 1, "NetworkPolicy should only match the StatefulSet")
	assert.Equal(t, "StatefulSet/postgres", npEdges[0].ChildID)
	assert.Equal(t, "podSelector", npEdges[0].Reason)
}

// TestNetworkPolicyEmptySelector verifies that a NetworkPolicy with an empty
// podSelector creates no edges.
func TestNetworkPolicyEmptySelector(t *testing.T) {
	np := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.k8s.io/v1",
			"kind":       "NetworkPolicy",
			"metadata":   map[string]interface{}{"name": "allow-all"},
			"spec": map[string]interface{}{
				"podSelector": map[string]interface{}{},
			},
		},
	}
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":   "some-pod",
				"labels": map[string]interface{}{"app": "web"},
			},
		},
	}

	deps := dependency.BuildDependencies([]*unstructured.Unstructured{np, pod})
	assert.Empty(t, deps["NetworkPolicy/allow-all"], "empty podSelector should match nothing")
}

// TestPodDisruptionBudgetSelector verifies PDB selector matching.
func TestPodDisruptionBudgetSelector(t *testing.T) {
	pdb := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "policy/v1",
			"kind":       "PodDisruptionBudget",
			"metadata":   map[string]interface{}{"name": "web-pdb"},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{"app": "web"},
				},
			},
		},
	}
	deploy := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":   "web-deploy",
				"labels": map[string]interface{}{"app": "web"},
			},
		},
	}
	// Non-matching Deployment
	other := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":   "api-deploy",
				"labels": map[string]interface{}{"app": "api"},
			},
		},
	}

	deps := dependency.BuildDependencies([]*unstructured.Unstructured{pdb, deploy, other})
	pdbEdges := deps["PodDisruptionBudget/web-pdb"]

	require.Len(t, pdbEdges, 1)
	assert.Equal(t, "Deployment/web-deploy", pdbEdges[0].ChildID)
	assert.Equal(t, "pdbSelector", pdbEdges[0].Reason)
}

// TestIngressReferences verifies Ingress backend and TLS secret edges.
func TestIngressReferences(t *testing.T) {
	ing := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.k8s.io/v1",
			"kind":       "Ingress",
			"metadata":   map[string]interface{}{"name": "my-ing"},
			"spec": map[string]interface{}{
				"rules": []interface{}{
					map[string]interface{}{
						"http": map[string]interface{}{
							"paths": []interface{}{
								map[string]interface{}{
									"path": "/api",
									"backend": map[string]interface{}{
										"service": map[string]interface{}{
											"name": "api-svc",
											"port": map[string]interface{}{"number": float64(8080)},
										},
									},
								},
								map[string]interface{}{
									"path": "/web",
									"backend": map[string]interface{}{
										"service": map[string]interface{}{
											"name": "web-svc",
											"port": map[string]interface{}{"number": float64(80)},
										},
									},
								},
							},
						},
					},
				},
				"tls": []interface{}{
					map[string]interface{}{"secretName": "tls-cert"},
				},
			},
		},
	}

	deps := dependency.BuildDependencies([]*unstructured.Unstructured{ing})
	ingEdges := deps["Ingress/my-ing"]

	require.Len(t, ingEdges, 3, "expected 2 backend services + 1 TLS secret")
	var apiFound, webFound, tlsFound bool
	for _, e := range ingEdges {
		switch {
		case e.ChildID == "Service/api-svc" && e.Reason == "ingressBackend":
			apiFound = true
		case e.ChildID == "Service/web-svc" && e.Reason == "ingressBackend":
			webFound = true
		case e.ChildID == "Secret/tls-cert" && e.Reason == "tlsSecret":
			tlsFound = true
		}
	}
	assert.True(t, apiFound, "expected ingressBackend to api-svc")
	assert.True(t, webFound, "expected ingressBackend to web-svc")
	assert.True(t, tlsFound, "expected tlsSecret to tls-cert")
}

// TestIngressOlderBackendStyle verifies the older .backend.serviceName format.
func TestIngressOlderBackendStyle(t *testing.T) {
	ing := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "extensions/v1beta1",
			"kind":       "Ingress",
			"metadata":   map[string]interface{}{"name": "legacy-ing"},
			"spec": map[string]interface{}{
				"rules": []interface{}{
					map[string]interface{}{
						"http": map[string]interface{}{
							"paths": []interface{}{
								map[string]interface{}{
									"path": "/",
									"backend": map[string]interface{}{
										"serviceName": "legacy-svc",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	deps := dependency.BuildDependencies([]*unstructured.Unstructured{ing})
	ingEdges := deps["Ingress/legacy-ing"]

	require.Len(t, ingEdges, 1)
	assert.Equal(t, "Service/legacy-svc", ingEdges[0].ChildID)
	assert.Equal(t, "ingressBackend", ingEdges[0].Reason)
}

// TestHPAReferences verifies HPA scaleTargetRef edges.
func TestHPAReferences(t *testing.T) {
	hpa := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "autoscaling/v2",
			"kind":       "HorizontalPodAutoscaler",
			"metadata":   map[string]interface{}{"name": "web-hpa"},
			"spec": map[string]interface{}{
				"scaleTargetRef": map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"name":       "web-deploy",
				},
			},
		},
	}

	deps := dependency.BuildDependencies([]*unstructured.Unstructured{hpa})
	hpaEdges := deps["HorizontalPodAutoscaler/web-hpa"]

	require.Len(t, hpaEdges, 1)
	assert.Equal(t, "Deployment/web-deploy", hpaEdges[0].ChildID)
	assert.Equal(t, "scaleTargetRef", hpaEdges[0].Reason)
}

// TestHPAMissingScaleTarget verifies HPA with no scaleTargetRef produces no edges.
func TestHPAMissingScaleTarget(t *testing.T) {
	hpa := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "autoscaling/v2",
			"kind":       "HorizontalPodAutoscaler",
			"metadata":   map[string]interface{}{"name": "empty-hpa"},
			"spec":       map[string]interface{}{},
		},
	}

	deps := dependency.BuildDependencies([]*unstructured.Unstructured{hpa})
	assert.Empty(t, deps["HorizontalPodAutoscaler/empty-hpa"])
}

// TestLabelIndexMatch verifies the label index lookup directly.
func TestLabelIndexMatch(t *testing.T) {
	deploy := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":   "web",
				"labels": map[string]interface{}{"app": "web", "tier": "frontend"},
			},
		},
	}
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":   "worker",
				"labels": map[string]interface{}{"app": "web", "tier": "backend"},
			},
		},
	}
	// ServiceAccount should NOT be indexed
	sa := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ServiceAccount",
			"metadata": map[string]interface{}{
				"name":   "web-sa",
				"labels": map[string]interface{}{"app": "web"},
			},
		},
	}

	idx := dependency.BuildLabelIndex([]*unstructured.Unstructured{deploy, pod, sa})

	// Single label match
	matches := idx.Match(map[string]string{"app": "web"})
	assert.Len(t, matches, 2, "both deploy and pod have app=web")

	// Multi-label match — only deploy has both app=web AND tier=frontend
	matches = idx.Match(map[string]string{"app": "web", "tier": "frontend"})
	assert.Len(t, matches, 1)
	assert.Equal(t, "web", matches[0].GetName())

	// No match
	matches = idx.Match(map[string]string{"app": "nonexistent"})
	assert.Empty(t, matches)

	// Empty selector
	matches = idx.Match(map[string]string{})
	assert.Empty(t, matches)
}
