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
//
// Example:
//
//	"Deployment/foo" -> Edge{ChildID: "Secret/bar", Reason: "secretRef"}.
func BuildDependencies(objs []*unstructured.Unstructured) map[string][]Edge {
	mainLogger := log.WithFields(log.Fields{
		"func":  "BuildDependencies",
		"count": len(objs),
	})
	mainLogger.Info("Starting dependency analysis")

	dependencies := make(map[string][]Edge)

	// 1. Create an empty slice for every resource upfront, so loners appear in the final map.
	for _, obj := range objs {
		parentKey := ResourceID(obj)
		dependencies[parentKey] = []Edge{} // ensures each resource is present
	}

	// 2. Process ownerReferences (Owner -> Child).
	for _, obj := range objs {
		childID := ResourceID(obj)
		for _, owner := range obj.GetOwnerReferences() {
			ownerID := fmt.Sprintf("%s/%s", owner.Kind, owner.Name)
			edge := Edge{ChildID: childID, Reason: "ownerRef"}
			dependencies[ownerID] = append(dependencies[ownerID], edge)

			log.WithFields(log.Fields{
				"func":    "BuildDependencies",
				"ownerID": ownerID,
				"childID": childID,
			}).Debug("Added owner->child dependency")
		}
	}

	// 3. Build a label index for O(n) selector lookups, then process selectors.
	labelIdx := BuildLabelIndex(objs)
	for _, obj := range objs {
		switch obj.GetKind() {
		case "Service":
			handleServiceLabelSelector(obj, labelIdx, dependencies)
		case "NetworkPolicy":
			handleNetworkPolicy(obj, labelIdx, dependencies)
		case "PodDisruptionBudget":
			handlePodDisruptionBudget(obj, labelIdx, dependencies)
		}
	}

	// 4. Ingress references (Ingress -> Services, Ingress -> Secrets for TLS)
	for _, obj := range objs {
		if obj.GetKind() == "Ingress" {
			handleIngressReferences(obj, dependencies)
		}
	}

	// 5. HorizontalPodAutoscaler references (HPA -> scaleTargetRef)
	for _, obj := range objs {
		if obj.GetKind() == "HorizontalPodAutoscaler" {
			handleHPAReferences(obj, dependencies)
		}
	}

	// 6. Pod spec references in Pods, Deployments, DaemonSets, etc.
	for _, obj := range objs {
		if IsPodOrController(obj) {
			podSpec, found, err := GetPodSpec(obj)
			if err != nil {
				log.WithFields(log.Fields{
					"func":  "BuildDependencies",
					"error": err,
					"kind":  obj.GetKind(),
					"name":  obj.GetName(),
				}).Warn("Error retrieving podSpec")
				continue
			}
			if !found || podSpec == nil {
				continue
			}

			parentID := ResourceID(obj)
			secrets, configMaps, pvcs, serviceAccounts := GatherPodSpecReferences(podSpec)

			for _, child := range secrets {
				dependencies[parentID] = append(dependencies[parentID], Edge{
					ChildID: child,
					Reason:  "secretRef",
				})
			}
			for _, child := range configMaps {
				dependencies[parentID] = append(dependencies[parentID], Edge{
					ChildID: child,
					Reason:  "configMapRef",
				})
			}
			for _, child := range pvcs {
				dependencies[parentID] = append(dependencies[parentID], Edge{
					ChildID: child,
					Reason:  "pvcRef",
				})
			}
			for _, child := range serviceAccounts {
				dependencies[parentID] = append(dependencies[parentID], Edge{
					ChildID: child,
					Reason:  "serviceAccountName",
				})
			}
		}
	}

	// Deduplicate edges for each parent.
	for parent, edges := range dependencies {
		dependencies[parent] = deduplicateEdges(edges)
	}

	mainLogger.WithField("dependencies_count", len(dependencies)).Info("Finished building dependencies")
	return dependencies
}
