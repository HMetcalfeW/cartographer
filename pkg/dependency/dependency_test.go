package dependency_test

import (
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMetcalfeW/cartographer/pkg/dependency"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func init() {
	// If you want minimal logging noise during tests:
	log.SetLevel(log.ErrorLevel)
}

// TestBuildDependencies verifies the main BuildDependencies function end-to-end.
func TestBuildDependencies(t *testing.T) {
	// Create a Deployment referencing a Secret, ServiceAccount, etc.
	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "my-deploy",
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"kind": "HelmRelease",
						"name": "my-release",
					},
				},
			},
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"serviceAccountName": "my-sa",
						"volumes": []interface{}{
							map[string]interface{}{
								"name": "secret-vol",
								"secret": map[string]interface{}{
									"secretName": "my-secret",
								},
							},
						},
						"containers": []interface{}{
							map[string]interface{}{
								"name": "web",
								"env": []interface{}{
									map[string]interface{}{
										"name": "CONFIG",
										"valueFrom": map[string]interface{}{
											"configMapKeyRef": map[string]interface{}{
												"name": "my-cm",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// A Service that selects Pods with label app=webapp
	service := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"name": "my-service",
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"app": "webapp",
				},
			},
		},
	}

	// A Pod that is selected by the Service
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "my-pod",
				"labels": map[string]interface{}{
					"app": "webapp",
				},
			},
		},
	}

	// An Ingress referencing this service
	ing := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.k8s.io/v1",
			"kind":       "Ingress",
			"metadata": map[string]interface{}{
				"name": "my-ing",
			},
			"spec": map[string]interface{}{
				"rules": []interface{}{
					map[string]interface{}{
						"http": map[string]interface{}{
							"paths": []interface{}{
								map[string]interface{}{
									"path": "/",
									"backend": map[string]interface{}{
										"service": map[string]interface{}{
											"name": "my-service",
											"port": map[string]interface{}{
												"number": float64(80),
											},
										},
									},
								},
							},
						},
					},
				},
				"tls": []interface{}{
					map[string]interface{}{
						"secretName": "tls-secret",
					},
				},
			},
		},
	}

	// A HorizontalPodAutoscaler referencing the Deployment
	hpa := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "autoscaling/v2",
			"kind":       "HorizontalPodAutoscaler",
			"metadata": map[string]interface{}{
				"name": "my-hpa",
			},
			"spec": map[string]interface{}{
				"scaleTargetRef": map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"name":       "my-deploy",
				},
			},
		},
	}

	// Additional resources
	helmRelease := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "helm.example.com/v1",
			"kind":       "HelmRelease",
			"metadata": map[string]interface{}{
				"name": "my-release",
			},
		},
	}
	secret := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name": "my-secret",
			},
		},
	}
	cm := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name": "my-cm",
			},
		},
	}
	sa := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ServiceAccount",
			"metadata": map[string]interface{}{
				"name": "my-sa",
			},
		},
	}
	tlsSecret := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name": "tls-secret",
			},
		},
	}

	objs := []*unstructured.Unstructured{
		deployment, service, pod, ing, hpa,
		helmRelease, secret, cm, sa, tlsSecret,
	}

	deps := dependency.BuildDependencies(objs)

	// Confirm the HelmRelease -> Deployment
	hrEdges := deps["HelmRelease/my-release"]
	require.Len(t, hrEdges, 1)
	assert.Equal(t, "Deployment/my-deploy", hrEdges[0].ChildID)
	assert.Equal(t, "ownerRef", hrEdges[0].Reason)

	// Confirm the Deployment -> Secret, ConfigMap, ServiceAccount
	depEdges := deps["Deployment/my-deploy"]
	require.Len(t, depEdges, 3, "expected 3 references from Deployment/my-deploy")

	var secretRef, cmRef, saRef bool
	for _, e := range depEdges {
		if e.ChildID == "Secret/my-secret" && e.Reason == "secretRef" {
			secretRef = true
		}
		if e.ChildID == "ConfigMap/my-cm" && e.Reason == "configMapRef" {
			cmRef = true
		}
		if e.ChildID == "ServiceAccount/my-sa" && e.Reason == "serviceAccountName" {
			saRef = true
		}
	}
	assert.True(t, secretRef, "Expected secretRef to my-secret")
	assert.True(t, cmRef, "Expected configMapRef to my-cm")
	assert.True(t, saRef, "Expected serviceAccountName to my-sa")

	// Confirm the Service -> Pod (label selector)
	svcEdges := deps["Service/my-service"]
	require.Len(t, svcEdges, 1)
	assert.Equal(t, "Pod/my-pod", svcEdges[0].ChildID)
	assert.Equal(t, "selector", svcEdges[0].Reason)

	// Confirm the Ingress references
	ingEdges := deps["Ingress/my-ing"]
	require.Len(t, ingEdges, 2, "expected 2 edges from Ingress: service, secret")
	var svcFound, tlsFound bool
	for _, e := range ingEdges {
		if e.ChildID == "Service/my-service" && e.Reason == "ingressBackend" {
			svcFound = true
		}
		if e.ChildID == "Secret/tls-secret" && e.Reason == "tlsSecret" {
			tlsFound = true
		}
	}
	assert.True(t, svcFound, "Expected ingressBackend to my-service")
	assert.True(t, tlsFound, "Expected tlsSecret to Secret/tls-secret")

	// Confirm the HPA
	hpaEdges := deps["HorizontalPodAutoscaler/my-hpa"]
	require.Len(t, hpaEdges, 1)
	assert.Equal(t, "Deployment/my-deploy", hpaEdges[0].ChildID)
	assert.Equal(t, "scaleTargetRef", hpaEdges[0].Reason)
}
