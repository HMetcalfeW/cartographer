package cluster

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

// supportedGVRs lists every GroupVersionResource the dependency engine understands.
var supportedGVRs = []schema.GroupVersionResource{
	// Workloads
	{Group: "apps", Version: "v1", Resource: "deployments"},
	{Group: "apps", Version: "v1", Resource: "daemonsets"},
	{Group: "apps", Version: "v1", Resource: "statefulsets"},
	{Group: "apps", Version: "v1", Resource: "replicasets"},
	{Group: "batch", Version: "v1", Resource: "jobs"},
	{Group: "batch", Version: "v1", Resource: "cronjobs"},
	{Group: "", Version: "v1", Resource: "pods"},

	// Networking
	{Group: "", Version: "v1", Resource: "services"},
	{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
	{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"},

	// Config & Storage
	{Group: "", Version: "v1", Resource: "configmaps"},
	{Group: "", Version: "v1", Resource: "secrets"},
	{Group: "", Version: "v1", Resource: "persistentvolumeclaims"},

	// RBAC
	{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"},
	{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
	{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
	{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"},
	{Group: "", Version: "v1", Resource: "serviceaccounts"},

	// Autoscaling & Policy
	{Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers"},
	{Group: "policy", Version: "v1", Resource: "poddisruptionbudgets"},
}

// clusterScopedGVRs identifies resources that are not namespaced.
var clusterScopedGVRs = map[schema.GroupVersionResource]bool{
	{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"}:        true,
	{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"}: true,
}

// NewClient builds a dynamic.Interface from the given kubeconfig path and
// context name. Empty strings use defaults (standard kubeconfig resolution
// and current-context, respectively).
func NewClient(kubeconfigPath, contextName string) (dynamic.Interface, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfigPath != "" {
		rules.ExplicitPath = kubeconfigPath
	}

	overrides := &clientcmd.ConfigOverrides{}
	if contextName != "" {
		overrides.CurrentContext = contextName
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		rules, overrides,
	).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}
	return client, nil
}

// FetchResources lists all supported Kubernetes resource types from the cluster.
// If allNamespaces is true, resources are listed across all namespaces and
// cluster-scoped resources (ClusterRole, ClusterRoleBinding) are included.
// When a specific namespace is given, cluster-scoped resources are skipped to
// avoid pulling every system ClusterRole/ClusterRoleBinding into the graph;
// any that are referenced (e.g. via roleRef) still appear as edge targets.
// Missing GVRs (404) and permission errors (403) are logged and skipped.
func FetchResources(
	ctx context.Context,
	client dynamic.Interface,
	namespace string,
	allNamespaces bool,
) ([]*unstructured.Unstructured, error) {
	var result []*unstructured.Unstructured

	for _, gvr := range supportedGVRs {
		items, err := fetchGVR(ctx, client, gvr, namespace, allNamespaces)
		if err != nil {
			return nil, err
		}
		result = append(result, items...)
	}

	log.WithField("func", "FetchResources").Infof("Fetched %d resources from cluster", len(result))
	return result, nil
}

func fetchGVR(
	ctx context.Context,
	client dynamic.Interface,
	gvr schema.GroupVersionResource,
	namespace string,
	allNamespaces bool,
) ([]*unstructured.Unstructured, error) {
	// Skip cluster-scoped resources when a specific namespace is requested.
	if clusterScopedGVRs[gvr] && !allNamespaces {
		return nil, nil
	}

	var ri dynamic.ResourceInterface
	if allNamespaces {
		ri = client.Resource(gvr)
	} else {
		ri = client.Resource(gvr).Namespace(namespace)
	}

	list, err := ri.List(ctx, metav1.ListOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) || apierrors.IsForbidden(err) {
			log.WithFields(log.Fields{
				"func": "fetchGVR",
				"gvr":  gvr.String(),
			}).Debug("Skipping unavailable or forbidden resource")
			return nil, nil
		}
		return nil, fmt.Errorf("failed to list %s: %w", gvr.Resource, err)
	}

	result := make([]*unstructured.Unstructured, len(list.Items))
	for i := range list.Items {
		result[i] = &list.Items[i]
	}
	return result, nil
}
