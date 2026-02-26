package dependency

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// BuildDependencies analyzes a slice of unstructured Kubernetes objects and
// identifies their interdependencies. It returns a map where each key is a
// "parent" resource identifier ("Kind/Name"), and each value is a slice of
// Edge structures describing the child resource and the reason for the link.
func BuildDependencies(objs []*unstructured.Unstructured) map[string][]Edge {
	mainLogger := log.WithFields(log.Fields{
		"func":  "BuildDependencies",
		"count": len(objs),
	})
	mainLogger.Info("Starting dependency analysis")

	deps := make(map[string][]Edge)

	// Ensure every resource appears in the map, even if it has no edges.
	for _, obj := range objs {
		deps[ResourceID(obj)] = []Edge{}
	}

	// Process ownerReferences (Owner -> Child).
	for _, obj := range objs {
		childID := ResourceID(obj)
		for _, owner := range obj.GetOwnerReferences() {
			ownerID := fmt.Sprintf("%s/%s", owner.Kind, owner.Name)
			deps[ownerID] = append(deps[ownerID], Edge{ChildID: childID, Reason: "ownerRef"})
		}
	}

	// Build a label index for O(n) selector lookups, then process all
	// resource-specific handlers in a single pass.
	labelIdx := BuildLabelIndex(objs)
	for _, obj := range objs {
		switch obj.GetKind() {
		case "Service":
			handleServiceLabelSelector(obj, labelIdx, deps)
		case "NetworkPolicy":
			handleNetworkPolicy(obj, labelIdx, deps)
		case "PodDisruptionBudget":
			handlePodDisruptionBudget(obj, labelIdx, deps)
		case "Ingress":
			handleIngressReferences(obj, deps)
		case "HorizontalPodAutoscaler":
			handleHPAReferences(obj, deps)
		case "RoleBinding", "ClusterRoleBinding":
			handleRoleBinding(obj, deps)
		}

		// Pod spec references (Secrets, ConfigMaps, PVCs, ServiceAccounts).
		if IsPodOrController(obj) {
			gatherPodSpecEdges(obj, deps)
		}
	}

	// Deduplicate edges for each parent.
	for parent, edges := range deps {
		deps[parent] = deduplicateEdges(edges)
	}

	mainLogger.WithField("dependencies_count", len(deps)).Info("Finished building dependencies")
	return deps
}

// gatherPodSpecEdges extracts pod spec references from a pod or controller
// and appends them as edges to the dependency map.
func gatherPodSpecEdges(obj *unstructured.Unstructured, deps map[string][]Edge) {
	podSpec, found, err := GetPodSpec(obj)
	if err != nil {
		log.WithFields(log.Fields{
			"func": "gatherPodSpecEdges",
			"kind": obj.GetKind(),
			"name": obj.GetName(),
		}).WithError(err).Warn("Error retrieving podSpec")
		return
	}
	if !found || podSpec == nil {
		return
	}

	parentID := ResourceID(obj)
	secrets, configMaps, pvcs, serviceAccounts := GatherPodSpecReferences(podSpec)

	appendEdges(deps, parentID, secrets, "secretRef")
	appendEdges(deps, parentID, configMaps, "configMapRef")
	appendEdges(deps, parentID, pvcs, "pvcRef")
	appendEdges(deps, parentID, serviceAccounts, "serviceAccountName")
}

// appendEdges adds an edge from parentID to each child with the given reason.
func appendEdges(deps map[string][]Edge, parentID string, children []string, reason string) {
	for _, child := range children {
		deps[parentID] = append(deps[parentID], Edge{ChildID: child, Reason: reason})
	}
}
