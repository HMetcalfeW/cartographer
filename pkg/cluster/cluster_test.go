package cluster_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/HMetcalfeW/cartographer/pkg/cluster"
	"github.com/HMetcalfeW/cartographer/pkg/dependency"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

// gvrMap registers all GVRs the fake client needs to support for List calls.
var gvrMap = map[schema.GroupVersionResource]string{
	{Group: "apps", Version: "v1", Resource: "deployments"}:                              "DeploymentList",
	{Group: "apps", Version: "v1", Resource: "daemonsets"}:                               "DaemonSetList",
	{Group: "apps", Version: "v1", Resource: "statefulsets"}:                             "StatefulSetList",
	{Group: "apps", Version: "v1", Resource: "replicasets"}:                              "ReplicaSetList",
	{Group: "batch", Version: "v1", Resource: "jobs"}:                                    "JobList",
	{Group: "batch", Version: "v1", Resource: "cronjobs"}:                                "CronJobList",
	{Group: "", Version: "v1", Resource: "pods"}:                                         "PodList",
	{Group: "", Version: "v1", Resource: "services"}:                                     "ServiceList",
	{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"}:                   "IngressList",
	{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"}:             "NetworkPolicyList",
	{Group: "", Version: "v1", Resource: "configmaps"}:                                   "ConfigMapList",
	{Group: "", Version: "v1", Resource: "secrets"}:                                      "SecretList",
	{Group: "", Version: "v1", Resource: "persistentvolumeclaims"}:                       "PersistentVolumeClaimList",
	{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"}:               "RoleList",
	{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"}:        "ClusterRoleList",
	{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"}:        "RoleBindingList",
	{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"}: "ClusterRoleBindingList",
	{Group: "", Version: "v1", Resource: "serviceaccounts"}:                              "ServiceAccountList",
	{Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers"}:          "HorizontalPodAutoscalerList",
	{Group: "policy", Version: "v1", Resource: "poddisruptionbudgets"}:                   "PodDisruptionBudgetList",
}

func makeObj(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
		},
	}
}

func TestFetchResources_Basic(t *testing.T) {
	objs := []runtime.Object{
		makeObj("apps/v1", "Deployment", "default", "web"),
		makeObj("v1", "Service", "default", "web-svc"),
		makeObj("v1", "Secret", "default", "db-creds"),
	}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrMap, objs...)

	result, err := cluster.FetchResources(context.Background(), client, "default", false)
	require.NoError(t, err)
	assert.Len(t, result, 3)

	names := make(map[string]bool)
	for _, obj := range result {
		names[obj.GetName()] = true
	}
	assert.True(t, names["web"])
	assert.True(t, names["web-svc"])
	assert.True(t, names["db-creds"])
}

func TestFetchResources_AllNamespaces(t *testing.T) {
	objs := []runtime.Object{
		makeObj("apps/v1", "Deployment", "ns1", "app1"),
		makeObj("apps/v1", "Deployment", "ns2", "app2"),
		makeObj("v1", "Service", "ns1", "svc1"),
	}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrMap, objs...)

	result, err := cluster.FetchResources(context.Background(), client, "", true)
	require.NoError(t, err)
	assert.Len(t, result, 3)
}

func TestFetchResources_NamespaceScoped(t *testing.T) {
	objs := []runtime.Object{
		makeObj("apps/v1", "Deployment", "ns1", "app1"),
		makeObj("apps/v1", "Deployment", "ns2", "app2"),
	}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrMap, objs...)

	result, err := cluster.FetchResources(context.Background(), client, "ns1", false)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "app1", result[0].GetName())
}

func TestFetchResources_ClusterScopedSkippedInNamespaceMode(t *testing.T) {
	objs := []runtime.Object{
		makeObj("apps/v1", "Deployment", "default", "web"),
		makeObj("rbac.authorization.k8s.io/v1", "ClusterRole", "", "admin"),
		makeObj("rbac.authorization.k8s.io/v1", "ClusterRoleBinding", "", "admin-binding"),
	}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrMap, objs...)

	// Namespace-scoped: should skip ClusterRoles and ClusterRoleBindings.
	result, err := cluster.FetchResources(context.Background(), client, "default", false)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Deployment", result[0].GetKind())

	// All-namespaces: should include cluster-scoped resources.
	resultAll, err := cluster.FetchResources(context.Background(), client, "", true)
	require.NoError(t, err)
	kinds := make(map[string]bool)
	for _, obj := range resultAll {
		kinds[obj.GetKind()] = true
	}
	assert.True(t, kinds["Deployment"])
	assert.True(t, kinds["ClusterRole"])
	assert.True(t, kinds["ClusterRoleBinding"])
}

func TestFetchResources_NamespaceModeProducesCleanGraph(t *testing.T) {
	// Simulate a real cluster: namespace resources + many system ClusterRoles/Bindings.
	objs := []runtime.Object{
		makeObj("apps/v1", "Deployment", "myns", "web"),
		makeObj("v1", "Service", "myns", "web-svc"),
		makeObj("rbac.authorization.k8s.io/v1", "RoleBinding", "myns", "app-binding"),
		// System cluster-scoped resources that should NOT appear in namespace mode.
		makeObj("rbac.authorization.k8s.io/v1", "ClusterRole", "", "system:controller:deployment-controller"),
		makeObj("rbac.authorization.k8s.io/v1", "ClusterRole", "", "cluster-admin"),
		makeObj("rbac.authorization.k8s.io/v1", "ClusterRoleBinding", "", "system:controller:deployment-controller"),
		makeObj("rbac.authorization.k8s.io/v1", "ClusterRoleBinding", "", "cluster-admin"),
	}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrMap, objs...)

	result, err := cluster.FetchResources(context.Background(), client, "myns", false)
	require.NoError(t, err)

	// Only namespace-scoped resources should be returned.
	for _, obj := range result {
		kind := obj.GetKind()
		assert.NotEqual(t, "ClusterRole", kind, "ClusterRole should not appear in namespace mode")
		assert.NotEqual(t, "ClusterRoleBinding", kind, "ClusterRoleBinding should not appear in namespace mode")
	}
	assert.Len(t, result, 3, "expected only the 3 namespace-scoped resources")

	// Full pipeline: the graph should contain only our namespace resources.
	deps := dependency.BuildDependencies(result)
	jsonOut := dependency.GenerateJSON(deps)
	assert.NotContains(t, jsonOut, "ClusterRole/", "ClusterRoles should not appear in JSON")
	assert.NotContains(t, jsonOut, "ClusterRoleBinding/", "ClusterRoleBindings should not appear in JSON")
	assert.Contains(t, jsonOut, "Deployment/web")
	assert.Contains(t, jsonOut, "Service/web-svc")
}

func TestFetchResources_MissingGVRSkippedGracefully(t *testing.T) {
	objs := []runtime.Object{
		makeObj("apps/v1", "Deployment", "default", "web"),
	}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrMap, objs...)

	// Simulate a 404 for ingresses (e.g. networking.k8s.io not installed).
	client.PrependReactor("list", "ingresses", func(_ k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(
			schema.GroupResource{Group: "networking.k8s.io", Resource: "ingresses"}, "",
		)
	})

	// Simulate a 403 for secrets (e.g. RBAC denied).
	client.PrependReactor("list", "secrets", func(_ k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(
			schema.GroupResource{Resource: "secrets"}, "", fmt.Errorf("forbidden"),
		)
	})

	result, err := cluster.FetchResources(context.Background(), client, "default", false)
	require.NoError(t, err, "404 and 403 errors should be skipped, not returned")
	assert.NotEmpty(t, result)

	// Only the Deployment should come through.
	kinds := make(map[string]bool)
	for _, obj := range result {
		kinds[obj.GetKind()] = true
	}
	assert.True(t, kinds["Deployment"])
}

func TestFetchResources_EmptyCluster(t *testing.T) {
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrMap)

	result, err := cluster.FetchResources(context.Background(), client, "default", false)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestFetchResources_WithBuildDependencies(t *testing.T) {
	deploy := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "web",
				"namespace": "default",
				"labels":    map[string]interface{}{"app": "web"},
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{"app": "web"},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{"app": "web"},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "web",
								"image": "nginx",
								"env": []interface{}{
									map[string]interface{}{
										"name": "DB_PASS",
										"valueFrom": map[string]interface{}{
											"secretKeyRef": map[string]interface{}{
												"name": "db-creds",
												"key":  "password",
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
	secret := makeObj("v1", "Secret", "default", "db-creds")
	svc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"name":      "web-svc",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{"app": "web"},
				"ports": []interface{}{
					map[string]interface{}{"port": int64(80)},
				},
			},
		},
	}

	objs := []runtime.Object{deploy, secret, svc}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrMap, objs...)

	result, err := cluster.FetchResources(context.Background(), client, "default", false)
	require.NoError(t, err)
	assert.Len(t, result, 3)

	// Feed directly into BuildDependencies — no YAML parsing needed.
	deps := dependency.BuildDependencies(result)

	// Deployment → Secret (secretRef)
	deployEdges := deps["Deployment/web"]
	hasSecretRef := false
	for _, e := range deployEdges {
		if e.ChildID == "Secret/db-creds" && e.Reason == "secretRef" {
			hasSecretRef = true
		}
	}
	assert.True(t, hasSecretRef, "expected Deployment/web → Secret/db-creds secretRef edge")

	// Service → Deployment (selector)
	svcEdges := deps["Service/web-svc"]
	hasSelectorEdge := false
	for _, e := range svcEdges {
		if e.ChildID == "Deployment/web" && e.Reason == "selector" {
			hasSelectorEdge = true
		}
	}
	assert.True(t, hasSelectorEdge, "expected Service/web-svc → Deployment/web selector edge")
}
