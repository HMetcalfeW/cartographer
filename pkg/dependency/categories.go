package dependency

import (
	"sort"
	"strings"
)

// ResourceCategory holds a display label, fill color, and the set of
// Kubernetes kinds that belong to this category.
type ResourceCategory struct {
	Label string
	Color string // GraphViz fill color for DOT output
	Kinds map[string]bool
}

// CategoryOrder defines the display order for resource categories.
var CategoryOrder = []string{
	"workloads",
	"networking",
	"config",
	"rbac",
	"autoscaling",
	"other",
}

// Categories maps category keys to their definitions.
var Categories = map[string]ResourceCategory{
	"workloads": {
		Label: "Workloads",
		Color: "#DAEEF3",
		Kinds: map[string]bool{
			"Deployment":  true,
			"DaemonSet":   true,
			"StatefulSet": true,
			"ReplicaSet":  true,
			"Job":         true,
			"CronJob":     true,
			"Pod":         true,
		},
	},
	"networking": {
		Label: "Networking",
		Color: "#E2EFDA",
		Kinds: map[string]bool{
			"Service":       true,
			"Ingress":       true,
			"NetworkPolicy": true,
		},
	},
	"config": {
		Label: "Config & Storage",
		Color: "#FFF2CC",
		Kinds: map[string]bool{
			"ConfigMap":             true,
			"Secret":                true,
			"PersistentVolumeClaim": true,
		},
	},
	"rbac": {
		Label: "RBAC",
		Color: "#E2D9F3",
		Kinds: map[string]bool{
			"Role":               true,
			"ClusterRole":        true,
			"RoleBinding":        true,
			"ClusterRoleBinding": true,
			"ServiceAccount":     true,
		},
	},
	"autoscaling": {
		Label: "Autoscaling & Policy",
		Color: "#FCE4D6",
		Kinds: map[string]bool{
			"HorizontalPodAutoscaler": true,
			"PodDisruptionBudget":     true,
		},
	},
	"other": {
		Label: "Other",
		Color: "#F2F2F2",
		Kinds: map[string]bool{},
	},
}

// kindToCategory is a reverse lookup from Kind → category key.
var kindToCategory map[string]string

func init() {
	kindToCategory = make(map[string]string)
	for catKey, cat := range Categories {
		for kind := range cat.Kinds {
			kindToCategory[kind] = catKey
		}
	}
}

// CategoryForNode returns the category key for a node ID ("Kind/Name").
func CategoryForNode(nodeID string) string {
	kind, _, ok := strings.Cut(nodeID, "/")
	if !ok {
		return "other"
	}
	if cat, found := kindToCategory[kind]; found {
		return cat
	}
	return "other"
}

// GroupNodesByCategory collects all unique node IDs from the dependency map
// (both parents and children) and groups them by category. Each group's
// node list is sorted for deterministic output.
func GroupNodesByCategory(deps map[string][]Edge) map[string][]string {
	nodeSet := make(map[string]struct{})
	for parent, edges := range deps {
		nodeSet[parent] = struct{}{}
		for _, e := range edges {
			nodeSet[e.ChildID] = struct{}{}
		}
	}

	groups := make(map[string][]string)
	for nodeID := range nodeSet {
		cat := CategoryForNode(nodeID)
		groups[cat] = append(groups[cat], nodeID)
	}

	for cat := range groups {
		sort.Strings(groups[cat])
	}
	return groups
}
