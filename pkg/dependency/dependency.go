package dependency

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// BuildDependencies analyzes a list of unstructured Kubernetes objects and
// builds a map representing resource dependencies. The map’s key is the
// "parent" resource (e.g., "Deployment/my-app"), and the values are a slice
// of "child" resources that depend on it (e.g., Pods, ReplicaSets, etc.).
func BuildDependencies(objs []*unstructured.Unstructured) map[string][]string {
	dependencies := make(map[string][]string)

	// Create a quick lookup table for all objects by ID (Kind/Name).
	objectsByID := make(map[string]*unstructured.Unstructured)
	for _, obj := range objs {
		id := fmt.Sprintf("%s/%s", obj.GetKind(), obj.GetName())
		objectsByID[id] = obj
	}

	// 1. Process OwnerReferences.
	//    A child's ownerReference means: Owner -> Child
	for _, obj := range objs {
		childID := fmt.Sprintf("%s/%s", obj.GetKind(), obj.GetName())
		for _, owner := range obj.GetOwnerReferences() {
			ownerID := fmt.Sprintf("%s/%s", owner.Kind, owner.Name)
			dependencies[ownerID] = append(dependencies[ownerID], childID)
		}
	}

	// 2. Process Services by matching label selectors.
	//    If a Service selector matches some object's labels, record an edge:
	//    Service -> Target
	for _, obj := range objs {
		if obj.GetKind() != "Service" {
			continue
		}

		serviceID := fmt.Sprintf("%s/%s", obj.GetKind(), obj.GetName())
		spec, found, err := unstructured.NestedMap(obj.Object, "spec")
		if err != nil || !found {
			continue
		}
		selector, found, err := unstructured.NestedStringMap(spec, "selector")
		if err != nil || !found {
			continue
		}

		// Compare the selector with the labels of all other objects.
		for _, target := range objs {
			targetLabels := target.GetLabels()
			if labelsMatch(selector, targetLabels) {
				targetID := fmt.Sprintf("%s/%s", target.GetKind(), target.GetName())
				dependencies[serviceID] = append(dependencies[serviceID], targetID)
			}
		}
	}

	return dependencies
}

// PrintDependencies prints the dependency map to stdout.
func PrintDependencies(deps map[string][]string) {
	for parent, children := range deps {
		if len(children) == 0 {
			continue
		}
		fmt.Printf("%s -> [", parent)
		fmt.Print(strings.Join(children, ", "))
		fmt.Println("]")
	}
}

// labelsMatch checks if all key-value pairs in 'selector' are present
// in the 'labels' map.
func labelsMatch(selector, labels map[string]string) bool {
	for k, v := range selector {
		if labelsVal, found := labels[k]; !found || labelsVal != v {
			return false
		}
	}
	return true
}
