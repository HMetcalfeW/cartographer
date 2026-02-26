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

// TestNetworkPolicyMatchExpressions verifies NetworkPolicy with matchExpressions.
func TestNetworkPolicyMatchExpressions(t *testing.T) {
	// Three Deployments with different labels
	web := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1", "kind": "Deployment",
			"metadata": map[string]interface{}{
				"name":   "web",
				"labels": map[string]interface{}{"app": "web", "env": "prod"},
			},
		},
	}
	api := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1", "kind": "Deployment",
			"metadata": map[string]interface{}{
				"name":   "api",
				"labels": map[string]interface{}{"app": "api", "env": "prod"},
			},
		},
	}
	worker := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1", "kind": "Deployment",
			"metadata": map[string]interface{}{
				"name":   "worker",
				"labels": map[string]interface{}{"app": "worker", "env": "staging"},
			},
		},
	}

	// NetworkPolicy with In expression — should match web + api (env=prod)
	npIn := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.k8s.io/v1",
			"kind":       "NetworkPolicy",
			"metadata":   map[string]interface{}{"name": "prod-only"},
			"spec": map[string]interface{}{
				"podSelector": map[string]interface{}{
					"matchExpressions": []interface{}{
						map[string]interface{}{
							"key": "env", "operator": "In", "values": []interface{}{"prod"},
						},
					},
				},
			},
		},
	}

	deps := dependency.BuildDependencies([]*unstructured.Unstructured{npIn, web, api, worker})
	npEdges := deps["NetworkPolicy/prod-only"]
	require.Len(t, npEdges, 2, "should match web and api")
	names := map[string]bool{}
	for _, e := range npEdges {
		names[e.ChildID] = true
		assert.Equal(t, "podSelector", e.Reason)
	}
	assert.True(t, names["Deployment/web"])
	assert.True(t, names["Deployment/api"])

	// NetworkPolicy with NotIn — should exclude worker (env=staging)
	npNotIn := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.k8s.io/v1",
			"kind":       "NetworkPolicy",
			"metadata":   map[string]interface{}{"name": "not-staging"},
			"spec": map[string]interface{}{
				"podSelector": map[string]interface{}{
					"matchExpressions": []interface{}{
						map[string]interface{}{
							"key": "env", "operator": "NotIn", "values": []interface{}{"staging"},
						},
					},
				},
			},
		},
	}
	deps = dependency.BuildDependencies([]*unstructured.Unstructured{npNotIn, web, api, worker})
	npEdges = deps["NetworkPolicy/not-staging"]
	require.Len(t, npEdges, 2, "should match web and api, not worker")

	// NetworkPolicy with mixed matchLabels + matchExpressions
	npMixed := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.k8s.io/v1",
			"kind":       "NetworkPolicy",
			"metadata":   map[string]interface{}{"name": "prod-web"},
			"spec": map[string]interface{}{
				"podSelector": map[string]interface{}{
					"matchLabels": map[string]interface{}{"env": "prod"},
					"matchExpressions": []interface{}{
						map[string]interface{}{
							"key": "app", "operator": "In", "values": []interface{}{"web"},
						},
					},
				},
			},
		},
	}
	deps = dependency.BuildDependencies([]*unstructured.Unstructured{npMixed, web, api, worker})
	npEdges = deps["NetworkPolicy/prod-web"]
	require.Len(t, npEdges, 1, "only web matches env=prod AND app In [web]")
	assert.Equal(t, "Deployment/web", npEdges[0].ChildID)

	// NetworkPolicy with Exists
	npExists := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.k8s.io/v1",
			"kind":       "NetworkPolicy",
			"metadata":   map[string]interface{}{"name": "has-app"},
			"spec": map[string]interface{}{
				"podSelector": map[string]interface{}{
					"matchExpressions": []interface{}{
						map[string]interface{}{"key": "app", "operator": "Exists"},
					},
				},
			},
		},
	}
	deps = dependency.BuildDependencies([]*unstructured.Unstructured{npExists, web, api, worker})
	assert.Len(t, deps["NetworkPolicy/has-app"], 3, "all three have the app label")
}

// TestPodDisruptionBudgetMatchExpressions verifies PDB with matchExpressions.
func TestPodDisruptionBudgetMatchExpressions(t *testing.T) {
	master := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1", "kind": "Deployment",
			"metadata": map[string]interface{}{
				"name":   "redis-master",
				"labels": map[string]interface{}{"app": "redis", "component": "master"},
			},
		},
	}
	replica := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1", "kind": "Deployment",
			"metadata": map[string]interface{}{
				"name":   "redis-replica",
				"labels": map[string]interface{}{"app": "redis", "component": "replica"},
			},
		},
	}

	// PDB with matchExpressions only
	pdb := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "policy/v1",
			"kind":       "PodDisruptionBudget",
			"metadata":   map[string]interface{}{"name": "redis-pdb"},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"matchExpressions": []interface{}{
						map[string]interface{}{
							"key": "app", "operator": "In", "values": []interface{}{"redis"},
						},
					},
				},
			},
		},
	}
	deps := dependency.BuildDependencies([]*unstructured.Unstructured{pdb, master, replica})
	pdbEdges := deps["PodDisruptionBudget/redis-pdb"]
	require.Len(t, pdbEdges, 2, "should match both redis deployments")

	// PDB with combined matchLabels + matchExpressions
	pdbMixed := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "policy/v1",
			"kind":       "PodDisruptionBudget",
			"metadata":   map[string]interface{}{"name": "redis-master-pdb"},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{"app": "redis"},
					"matchExpressions": []interface{}{
						map[string]interface{}{
							"key": "component", "operator": "In", "values": []interface{}{"master"},
						},
					},
				},
			},
		},
	}
	deps = dependency.BuildDependencies([]*unstructured.Unstructured{pdbMixed, master, replica})
	pdbEdges = deps["PodDisruptionBudget/redis-master-pdb"]
	require.Len(t, pdbEdges, 1, "only master matches app=redis AND component In [master]")
	assert.Equal(t, "Deployment/redis-master", pdbEdges[0].ChildID)
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

// TestServiceMissingSpec verifies that a Service with no .spec produces no edges.
func TestServiceMissingSpec(t *testing.T) {
	svc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata":   map[string]interface{}{"name": "no-spec-svc"},
		},
	}
	deploy := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":   "web",
				"labels": map[string]interface{}{"app": "web"},
			},
		},
	}
	deps := dependency.BuildDependencies([]*unstructured.Unstructured{svc, deploy})
	assert.Empty(t, deps["Service/no-spec-svc"])
}

// TestServiceMissingSelector verifies that a Service with spec but no selector produces no edges.
func TestServiceMissingSelector(t *testing.T) {
	svc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata":   map[string]interface{}{"name": "no-sel-svc"},
			"spec": map[string]interface{}{
				"ports": []interface{}{
					map[string]interface{}{"port": int64(80)},
				},
			},
		},
	}
	deploy := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":   "web",
				"labels": map[string]interface{}{"app": "web"},
			},
		},
	}
	deps := dependency.BuildDependencies([]*unstructured.Unstructured{svc, deploy})
	assert.Empty(t, deps["Service/no-sel-svc"])
}

// TestNetworkPolicyMissingSpec verifies NP with no .spec produces no edges.
func TestNetworkPolicyMissingSpec(t *testing.T) {
	np := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.k8s.io/v1",
			"kind":       "NetworkPolicy",
			"metadata":   map[string]interface{}{"name": "no-spec"},
		},
	}
	deps := dependency.BuildDependencies([]*unstructured.Unstructured{np})
	assert.Empty(t, deps["NetworkPolicy/no-spec"])
}

// TestPDBMissingSpec verifies PDB with no .spec produces no edges.
func TestPDBMissingSpec(t *testing.T) {
	pdb := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "policy/v1",
			"kind":       "PodDisruptionBudget",
			"metadata":   map[string]interface{}{"name": "no-spec"},
		},
	}
	deps := dependency.BuildDependencies([]*unstructured.Unstructured{pdb})
	assert.Empty(t, deps["PodDisruptionBudget/no-spec"])
}

// TestPDBEmptySelector verifies PDB with empty selector map produces no edges.
func TestPDBEmptySelector(t *testing.T) {
	pdb := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "policy/v1",
			"kind":       "PodDisruptionBudget",
			"metadata":   map[string]interface{}{"name": "empty-sel"},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{},
			},
		},
	}
	deploy := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":   "web",
				"labels": map[string]interface{}{"app": "web"},
			},
		},
	}
	deps := dependency.BuildDependencies([]*unstructured.Unstructured{pdb, deploy})
	assert.Empty(t, deps["PodDisruptionBudget/empty-sel"])
}

// TestHPAMissingKindOrName verifies HPA with incomplete scaleTargetRef produces no edges.
func TestHPAMissingKindOrName(t *testing.T) {
	// Has kind but no name
	hpaNoName := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "autoscaling/v2",
			"kind":       "HorizontalPodAutoscaler",
			"metadata":   map[string]interface{}{"name": "hpa-no-name"},
			"spec": map[string]interface{}{
				"scaleTargetRef": map[string]interface{}{
					"kind": "Deployment",
				},
			},
		},
	}
	deps := dependency.BuildDependencies([]*unstructured.Unstructured{hpaNoName})
	assert.Empty(t, deps["HorizontalPodAutoscaler/hpa-no-name"])

	// Has name but no kind
	hpaNoKind := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "autoscaling/v2",
			"kind":       "HorizontalPodAutoscaler",
			"metadata":   map[string]interface{}{"name": "hpa-no-kind"},
			"spec": map[string]interface{}{
				"scaleTargetRef": map[string]interface{}{
					"name": "my-deploy",
				},
			},
		},
	}
	deps = dependency.BuildDependencies([]*unstructured.Unstructured{hpaNoKind})
	assert.Empty(t, deps["HorizontalPodAutoscaler/hpa-no-kind"])
}

// TestIngressMissingRulesAndTLS verifies Ingress with no rules or TLS produces no edges.
func TestIngressMissingRulesAndTLS(t *testing.T) {
	ing := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.k8s.io/v1",
			"kind":       "Ingress",
			"metadata":   map[string]interface{}{"name": "empty-ing"},
			"spec":       map[string]interface{}{},
		},
	}
	deps := dependency.BuildDependencies([]*unstructured.Unstructured{ing})
	assert.Empty(t, deps["Ingress/empty-ing"])
}

// TestIngressRuleWithoutHTTP verifies that a rule without an http block is skipped.
func TestIngressRuleWithoutHTTP(t *testing.T) {
	ing := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.k8s.io/v1",
			"kind":       "Ingress",
			"metadata":   map[string]interface{}{"name": "no-http-ing"},
			"spec": map[string]interface{}{
				"rules": []interface{}{
					map[string]interface{}{
						"host": "example.com",
						// no http block
					},
				},
			},
		},
	}
	deps := dependency.BuildDependencies([]*unstructured.Unstructured{ing})
	assert.Empty(t, deps["Ingress/no-http-ing"])
}

// TestRoleBindingToRoleAndServiceAccount verifies RoleBinding creates edges
// to the referenced Role and ServiceAccount subjects.
func TestRoleBindingToRoleAndServiceAccount(t *testing.T) {
	rb := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "RoleBinding",
			"metadata":   map[string]interface{}{"name": "app-binding"},
			"roleRef": map[string]interface{}{
				"apiGroup": "rbac.authorization.k8s.io",
				"kind":     "Role",
				"name":     "app-role",
			},
			"subjects": []interface{}{
				map[string]interface{}{
					"kind":      "ServiceAccount",
					"name":      "app-sa",
					"namespace": "default",
				},
			},
		},
	}

	deps := dependency.BuildDependencies([]*unstructured.Unstructured{rb})
	edges := deps["RoleBinding/app-binding"]

	require.Len(t, edges, 2)
	edgeSet := map[string]string{}
	for _, e := range edges {
		edgeSet[e.ChildID] = e.Reason
	}
	assert.Equal(t, "roleRef", edgeSet["Role/app-role"])
	assert.Equal(t, "subject", edgeSet["ServiceAccount/app-sa"])
}

// TestClusterRoleBindingToClusterRoleAndMultipleSubjects verifies ClusterRoleBinding
// with multiple ServiceAccount subjects.
func TestClusterRoleBindingToClusterRoleAndMultipleSubjects(t *testing.T) {
	crb := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "ClusterRoleBinding",
			"metadata":   map[string]interface{}{"name": "cluster-admin-binding"},
			"roleRef": map[string]interface{}{
				"apiGroup": "rbac.authorization.k8s.io",
				"kind":     "ClusterRole",
				"name":     "cluster-admin",
			},
			"subjects": []interface{}{
				map[string]interface{}{
					"kind":      "ServiceAccount",
					"name":      "admin-sa",
					"namespace": "kube-system",
				},
				map[string]interface{}{
					"kind":      "ServiceAccount",
					"name":      "monitoring-sa",
					"namespace": "monitoring",
				},
				map[string]interface{}{
					"kind": "Group",
					"name": "system:masters",
				},
			},
		},
	}

	deps := dependency.BuildDependencies([]*unstructured.Unstructured{crb})
	edges := deps["ClusterRoleBinding/cluster-admin-binding"]

	// Should have roleRef + 2 ServiceAccounts (Group subject is skipped)
	require.Len(t, edges, 3)
	edgeSet := map[string]string{}
	for _, e := range edges {
		edgeSet[e.ChildID] = e.Reason
	}
	assert.Equal(t, "roleRef", edgeSet["ClusterRole/cluster-admin"])
	assert.Equal(t, "subject", edgeSet["ServiceAccount/admin-sa"])
	assert.Equal(t, "subject", edgeSet["ServiceAccount/monitoring-sa"])
}

// TestRoleBindingMissingRoleRef verifies RoleBinding with no roleRef still
// extracts subject edges.
func TestRoleBindingMissingRoleRef(t *testing.T) {
	rb := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "RoleBinding",
			"metadata":   map[string]interface{}{"name": "no-roleref"},
			"subjects": []interface{}{
				map[string]interface{}{
					"kind": "ServiceAccount",
					"name": "some-sa",
				},
			},
		},
	}

	deps := dependency.BuildDependencies([]*unstructured.Unstructured{rb})
	edges := deps["RoleBinding/no-roleref"]

	require.Len(t, edges, 1)
	assert.Equal(t, "ServiceAccount/some-sa", edges[0].ChildID)
	assert.Equal(t, "subject", edges[0].Reason)
}

// TestRoleBindingMissingSubjects verifies RoleBinding with no subjects
// still extracts the roleRef edge.
func TestRoleBindingMissingSubjects(t *testing.T) {
	rb := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "RoleBinding",
			"metadata":   map[string]interface{}{"name": "no-subjects"},
			"roleRef": map[string]interface{}{
				"apiGroup": "rbac.authorization.k8s.io",
				"kind":     "Role",
				"name":     "app-role",
			},
		},
	}

	deps := dependency.BuildDependencies([]*unstructured.Unstructured{rb})
	edges := deps["RoleBinding/no-subjects"]

	require.Len(t, edges, 1)
	assert.Equal(t, "Role/app-role", edges[0].ChildID)
	assert.Equal(t, "roleRef", edges[0].Reason)
}

// TestRoleBindingEmpty verifies RoleBinding with neither roleRef nor subjects
// produces no edges.
func TestRoleBindingEmpty(t *testing.T) {
	rb := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "RoleBinding",
			"metadata":   map[string]interface{}{"name": "empty-rb"},
		},
	}

	deps := dependency.BuildDependencies([]*unstructured.Unstructured{rb})
	assert.Empty(t, deps["RoleBinding/empty-rb"])
}
