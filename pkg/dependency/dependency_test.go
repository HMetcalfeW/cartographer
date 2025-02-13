package dependency_test

import (
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/HMetcalfeW/cartographer/pkg/dependency"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TestBuildDependencies checks typical references: Pod specs, volumes, env, Services, etc.
func TestBuildDependencies(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	// A Deployment referencing volumes, env, service account, and an imagePullSecret
	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "my-deployment",
			},
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"serviceAccountName": "my-service-account",
						"imagePullSecrets": []interface{}{
							map[string]interface{}{"name": "my-pull-secret"},
						},
						"volumes": []interface{}{
							map[string]interface{}{
								"name": "my-secret-vol",
								"secret": map[string]interface{}{
									"secretName": "my-secret",
								},
							},
							map[string]interface{}{
								"name": "my-pvc-vol",
								"persistentVolumeClaim": map[string]interface{}{
									"claimName": "my-claim",
								},
							},
						},
						"containers": []interface{}{
							map[string]interface{}{
								"name": "web",
								"env": []interface{}{
									map[string]interface{}{
										"name": "MY_CONFIG",
										"valueFrom": map[string]interface{}{
											"configMapKeyRef": map[string]interface{}{
												"name": "my-configmap",
												"key":  "some-key",
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

	// A Service with label selector app=webapp
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

	// A Pod owned by the Deployment, matching the Service’s label
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "my-pod",
				"labels": map[string]interface{}{
					"app": "webapp",
				},
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"kind": "Deployment",
						"name": "my-deployment",
					},
				},
			},
		},
	}

	// Additional resources: Secrets, ConfigMaps, PVC, ServiceAccount, pull secret
	secret := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name": "my-secret",
			},
		},
	}
	configMap := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name": "my-configmap",
			},
		},
	}
	pvc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "PersistentVolumeClaim",
			"metadata": map[string]interface{}{
				"name": "my-claim",
			},
		},
	}
	sa := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ServiceAccount",
			"metadata": map[string]interface{}{
				"name": "my-service-account",
			},
		},
	}
	pullSecret := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name": "my-pull-secret",
			},
		},
	}

	objs := []*unstructured.Unstructured{
		deployment, service, pod, secret, configMap, pvc, sa, pullSecret,
	}

	deps := dependency.BuildDependencies(objs)

	// Check references
	require.Contains(t, deps["Deployment/my-deployment"], "Pod/my-pod") // Owner
	require.Contains(t, deps["Service/my-service"], "Pod/my-pod")       // Service -> Pod
	require.Contains(t, deps["Deployment/my-deployment"], "Secret/my-secret")
	require.Contains(t, deps["Deployment/my-deployment"], "ConfigMap/my-configmap")
	require.Contains(t, deps["Deployment/my-deployment"], "PersistentVolumeClaim/my-claim")
	require.Contains(t, deps["Deployment/my-deployment"], "ServiceAccount/my-service-account")
	require.Contains(t, deps["Deployment/my-deployment"], "Secret/my-pull-secret")

	// Optional: generate DOT for debugging
	dot := dependency.GenerateDOT(deps)
	t.Logf("DOT Output:\n%s", dot)
}

// TestBuildDependencies_Extended covers Ingress, HPA, etc.
func TestBuildDependencies_Extended(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	// HPA referencing a Deployment
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

	// Ingress referencing a Service with float64(80) for port to avoid “cannot deep copy int”
	ing := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.k8s.io/v1",
			"kind":       "Ingress",
			"metadata": map[string]interface{}{
				"name": "my-ingress",
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
						"secretName": "my-tls-secret",
					},
				},
			},
		},
	}

	svc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"name": "my-service",
			},
		},
	}

	tlsSecret := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name": "my-tls-secret",
			},
		},
	}

	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "my-deploy",
			},
		},
	}

	objs := []*unstructured.Unstructured{hpa, ing, svc, tlsSecret, deployment}

	deps := dependency.BuildDependencies(objs)

	// HPA -> Deployment
	require.Contains(t, deps["HorizontalPodAutoscaler/my-hpa"], "Deployment/my-deploy")

	// Ingress -> Service
	require.Contains(t, deps["Ingress/my-ingress"], "Service/my-service")

	// Ingress -> TLS secret
	require.Contains(t, deps["Ingress/my-ingress"], "Secret/my-tls-secret")
}
