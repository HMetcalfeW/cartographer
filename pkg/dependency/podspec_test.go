package dependency_test

import (
	"testing"

	"github.com/HMetcalfeW/cartographer/pkg/dependency"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TestGetPodSpec calls GetPodSpec with various Kinds.
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
