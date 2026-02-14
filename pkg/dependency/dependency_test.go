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

// TestPrintDependencies ensures PrintDependencies doesn't panic and prints something.
func TestPrintDependencies(t *testing.T) {
	deps := map[string][]dependency.Edge{
		"Deployment/my-deploy": {
			{ChildID: "Secret/my-secret", Reason: "secretRef"},
			{ChildID: "ServiceAccount/my-sa", Reason: "serviceAccountName"},
		},
	}
	// Just ensuring it doesn't panic or error.
	dependency.PrintDependencies(deps)
}

// TestGenerateDOT ensures the DOT output includes reason labels.
func TestGenerateDOT(t *testing.T) {
	deps := map[string][]dependency.Edge{
		"Deployment/my-deploy": {
			{ChildID: "Secret/my-secret", Reason: "secretRef"},
			{ChildID: "ServiceAccount/my-sa", Reason: "serviceAccountName"},
		},
	}
	dot := dependency.GenerateDOT(deps)
	t.Log(dot)
	assert.Contains(t, dot, "[label=\"secretRef\"]")
	assert.Contains(t, dot, "[label=\"serviceAccountName\"]")
}

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

// TestGetPodSpec calls getPodSpec with various Kinds.
func TestGetPodSpec(t *testing.T) {
	// Pod
	pod := &unstructured.Unstructured{}
	pod.SetKind("Pod")
	pod.Object["spec"] = map[string]interface{}{"containers": []interface{}{}}
	spec, found, err := dependency.GetPodSpec(pod)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Contains(t, spec, "containers")

	// Deployment
	dep := &unstructured.Unstructured{}
	dep.SetKind("Deployment")
	dep.Object["spec"] = map[string]interface{}{
		"template": map[string]interface{}{
			"spec": map[string]interface{}{"initContainers": []interface{}{}},
		},
	}
	spec, found, err = dependency.GetPodSpec(dep)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Contains(t, spec, "initContainers")

	// Unknown kind
	foo := &unstructured.Unstructured{}
	foo.SetKind("FooBar")
	_, found, err = dependency.GetPodSpec(foo)
	require.Error(t, err)
	assert.False(t, found)
}

// TestParseEnvValueFrom checks parsing of env[].valueFrom.
func TestParseEnvValueFrom(t *testing.T) {
	var secretRefs, configMapRefs []string

	valFrom := map[string]interface{}{
		"secretKeyRef": map[string]interface{}{
			"name": "my-secret",
		},
	}
	dependency.ParseEnvValueFrom(valFrom, &secretRefs, &configMapRefs)
	assert.Contains(t, secretRefs, "Secret/my-secret")

	valFrom2 := map[string]interface{}{
		"configMapKeyRef": map[string]interface{}{
			"name": "my-cm",
		},
	}
	dependency.ParseEnvValueFrom(valFrom2, &secretRefs, &configMapRefs)
	assert.Contains(t, configMapRefs, "ConfigMap/my-cm")
}

// TestParseEnvFrom checks parsing of envFrom[].secretRef or configMapRef.
func TestParseEnvFrom(t *testing.T) {
	var secretRefs, configMapRefs []string

	envFrom := map[string]interface{}{
		"secretRef": map[string]interface{}{
			"name": "another-secret",
		},
	}
	dependency.ParseEnvFrom(envFrom, &secretRefs, &configMapRefs)
	assert.Contains(t, secretRefs, "Secret/another-secret")

	envFrom2 := map[string]interface{}{
		"configMapRef": map[string]interface{}{
			"name": "another-cm",
		},
	}
	dependency.ParseEnvFrom(envFrom2, &secretRefs, &configMapRefs)
	assert.Contains(t, configMapRefs, "ConfigMap/another-cm")
}

// TestGatherPodSpecReferences tries a minimal spec to confirm volumes, env, etc. are captured.
func TestGatherPodSpecReferences(t *testing.T) {
	ps := map[string]interface{}{
		"serviceAccountName": "my-sa",
		"volumes": []interface{}{
			map[string]interface{}{
				"name": "secret-vol",
				"secret": map[string]interface{}{
					"secretName": "my-secret",
				},
			},
			map[string]interface{}{
				"name": "cm-vol",
				"configMap": map[string]interface{}{
					"name": "my-cm",
				},
			},
		},
		"containers": []interface{}{
			map[string]interface{}{
				"name": "web",
				"envFrom": []interface{}{
					map[string]interface{}{
						"secretRef": map[string]interface{}{
							"name": "another-secret",
						},
					},
				},
			},
		},
		"imagePullSecrets": []interface{}{
			map[string]interface{}{
				"name": "pull-secret",
			},
		},
	}

	secrets, cms, pvcs, sas := dependency.GatherPodSpecReferences(ps)
	assert.Contains(t, secrets, "Secret/my-secret")
	assert.Contains(t, secrets, "Secret/another-secret")
	assert.Contains(t, cms, "ConfigMap/my-cm")
	assert.Contains(t, secrets, "Secret/pull-secret")
	assert.Contains(t, sas, "ServiceAccount/my-sa")
	assert.Empty(t, pvcs, "No PVC references here")
}

// TestGatherPodSpecReferences_EmptySpec ensures an empty pod spec doesn't panic.
func TestGatherPodSpecReferences_EmptySpec(t *testing.T) {
	secrets, cms, pvcs, sas := dependency.GatherPodSpecReferences(map[string]interface{}{})
	assert.Empty(t, secrets)
	assert.Empty(t, cms)
	assert.Empty(t, pvcs)
	assert.Empty(t, sas)
}

// TestGatherPodSpecReferences_MalformedVolumes ensures malformed volume entries are skipped safely.
func TestGatherPodSpecReferences_MalformedVolumes(t *testing.T) {
	ps := map[string]interface{}{
		"volumes": []interface{}{
			// volume with secret as a string instead of map (malformed)
			map[string]interface{}{
				"name":   "bad-vol",
				"secret": "not-a-map",
			},
			// volume entry that isn't a map at all
			"just-a-string",
			// valid volume to confirm processing continues
			map[string]interface{}{
				"name": "good-vol",
				"configMap": map[string]interface{}{
					"name": "valid-cm",
				},
			},
		},
	}
	secrets, cms, _, _ := dependency.GatherPodSpecReferences(ps)
	assert.Empty(t, secrets, "malformed secret volume should be skipped")
	assert.Contains(t, cms, "ConfigMap/valid-cm", "valid configMap should still be found")
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
