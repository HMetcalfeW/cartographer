package dependency

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// BuildDependencies analyzes a list of unstructured Kubernetes objects and
// builds a map representing resource dependencies. The map’s key is the
// "parent" resource (e.g., "Deployment/my-app"), and the values are a slice
// of "child" resources that depend on it (e.g., Pods, ReplicaSets, etc.).
func BuildDependencies(objs []*unstructured.Unstructured) map[string][]string {
	dependencies := make(map[string][]string)

	log.WithFields(log.Fields{
		"func":         "BuildDependencies",
		"object_count": len(objs),
	}).Info("Starting dependency analysis")

	// Create a quick lookup table for all objects by ID (Kind/Name).
	objectsByID := make(map[string]*unstructured.Unstructured)
	for _, obj := range objs {
		id := resourceID(obj)
		objectsByID[id] = obj
	}

	// 1. Process OwnerReferences.
	//    A child's ownerReference means: Owner -> Child
	for _, obj := range objs {
		childID := resourceID(obj)
		for _, owner := range obj.GetOwnerReferences() {
			ownerID := owner.Kind + "/" + owner.Name
			dependencies[ownerID] = append(dependencies[ownerID], childID)

			log.WithFields(log.Fields{
				"func":      "BuildDependencies",
				"ownerID":   ownerID,
				"childID":   childID,
				"childKind": obj.GetKind(),
			}).Debug("Added owner reference dependency")
		}
	}

	// 2. Process Services by matching label selectors.
	//    If a Service selector matches some object's labels, record an edge:
	//    Service -> Target
	for _, obj := range objs {
		if obj.GetKind() != "Service" {
			continue
		}
		serviceID := resourceID(obj)

		spec, found, err := unstructured.NestedMap(obj.Object, "spec")
		if err != nil || !found {
			log.WithFields(log.Fields{
				"func":    "BuildDependencies",
				"service": serviceID,
				"error":   err,
			}).Debug("No spec found for service or error accessing it")
			continue
		}

		selector, found, err := unstructured.NestedStringMap(spec, "selector")
		if err != nil || !found {
			log.WithFields(log.Fields{
				"func":    "BuildDependencies",
				"service": serviceID,
				"error":   err,
			}).Debug("No selector found for service or error accessing it")
			continue
		}

		for _, target := range objs {
			targetLabels := target.GetLabels()
			if labelsMatch(selector, targetLabels) {
				targetID := resourceID(target)
				dependencies[serviceID] = append(dependencies[serviceID], targetID)

				log.WithFields(log.Fields{
					"func":      "BuildDependencies",
					"serviceID": serviceID,
					"targetID":  targetID,
				}).Debug("Added service->target dependency")
			}
		}
	}

	log.WithFields(log.Fields{
		"func":               "BuildDependencies",
		"dependencies_count": len(dependencies),
	}).Info("Finished building dependencies")

	return dependencies
}

// PrintDependencies logs the dependency map at Info level.
func PrintDependencies(deps map[string][]string) {
	log.WithField("func", "PrintDependencies").
		Info("Printing dependency relationships")

	for parent, children := range deps {
		if len(children) == 0 {
			continue
		}
		log.WithFields(log.Fields{
			"parent":   parent,
			"children": children,
		}).Info("Dependency relationship")
	}
}

// resourceID constructs a unique identifier for a resource using Kind/Name.
func resourceID(obj *unstructured.Unstructured) string {
	return fmt.Sprintf("%s/%s", obj.GetKind(), obj.GetName())
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
