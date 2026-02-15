package dependency

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Edge represents a single dependency from one Kubernetes resource (the parent)
// to another resource (the child), along with the reason describing how or why
// the parent references the child.
type Edge struct {
	// ChildID is the unique identifier of the child resource, in the form "Kind/Name".
	ChildID string

	// Reason describes the nature of this dependency, e.g., "ownerRef", "secretRef", "selector".
	Reason string
}

// IsPodOrController returns true if the object is a Pod or a common controller
// type that embeds a Pod spec (.spec.template.spec or .spec.jobTemplate...).
func IsPodOrController(obj *unstructured.Unstructured) bool {
	switch obj.GetKind() {
	case "Pod", "Deployment", "DaemonSet", "StatefulSet", "ReplicaSet", "Job", "CronJob":
		return true
	default:
		return false
	}
}

// ResourceID builds a string "Kind/Name" from the object's kind and metadata.name.
func ResourceID(obj *unstructured.Unstructured) string {
	return fmt.Sprintf("%s/%s", obj.GetKind(), obj.GetName())
}

// LabelsMatch returns true if all key-value pairs in 'selector' are present in 'labels'.
func LabelsMatch(selector, labels map[string]string) bool {
	for k, v := range selector {
		if lv, found := labels[k]; !found || lv != v {
			return false
		}
	}
	return true
}

// MapInterfaceToStringMap attempts to cast an interface{} to map[string]interface{},
// then converts each value to a string if possible. Useful for label selectors
// or other fields that store data as map[string]interface{}.
func MapInterfaceToStringMap(in interface{}) map[string]string {
	out := make(map[string]string)
	if inMap, ok := in.(map[string]interface{}); ok {
		for k, v := range inMap {
			if vs, isStr := v.(string); isStr {
				out[k] = vs
			}
		}
	}
	return out
}

// deduplicateEdges removes duplicate edges based on ChildID+Reason.
func deduplicateEdges(edges []Edge) []Edge {
	seen := make(map[string]struct{}, len(edges))
	result := make([]Edge, 0, len(edges))
	for _, e := range edges {
		key := e.ChildID + "|" + e.Reason
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			result = append(result, e)
		}
	}
	return result
}
