package dependency

import (
	"fmt"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
)

// sanitizeMermaidID replaces characters that are invalid in Mermaid node
// identifiers (/, -, .) with underscores.
func sanitizeMermaidID(id string) string {
	r := strings.NewReplacer("/", "_", "-", "_", ".", "_")
	return r.Replace(id)
}

// GenerateMermaid produces a Mermaid flowchart (left-to-right) with resources
// grouped into subgraphs by category and color-coded via classDef directives.
// Only nodes that participate in at least one edge are emitted.
// Node declarations go inside subgraphs; edges are emitted outside so Mermaid
// can route them across subgraph boundaries.
func GenerateMermaid(deps map[string][]Edge) string {
	var sb strings.Builder
	sb.WriteString("graph LR\n")

	// Collect only nodes that participate in edges.
	connected := make(map[string]struct{})
	for parent, edges := range deps {
		if len(edges) > 0 {
			connected[parent] = struct{}{}
		}
		for _, e := range edges {
			connected[e.ChildID] = struct{}{}
		}
	}

	// Group connected nodes by category.
	groups := make(map[string][]string)
	for id := range connected {
		cat := CategoryForNode(id)
		groups[cat] = append(groups[cat], id)
	}
	for cat := range groups {
		sort.Strings(groups[cat])
	}

	// Emit node declarations (no subgraph clusters — color-coding via classDef
	// provides visual grouping without constraining Mermaid's layout engine).
	nodeIDs := make([]string, 0, len(connected))
	for id := range connected {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)

	for _, node := range nodeIDs {
		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", sanitizeMermaidID(node), node))
	}

	// Sorted edges.
	parents := make([]string, 0, len(deps))
	for p := range deps {
		parents = append(parents, p)
	}
	sort.Strings(parents)

	edgeCount := 0
	for _, parent := range parents {
		for _, edge := range deps[parent] {
			parentID := sanitizeMermaidID(parent)
			childID := sanitizeMermaidID(edge.ChildID)
			sb.WriteString(fmt.Sprintf("    %s --> |%s| %s\n", parentID, edge.Reason, childID))
			edgeCount++
		}
	}

	log.WithFields(log.Fields{
		"func":  "GenerateMermaid",
		"nodes": len(connected),
		"edges": edgeCount,
	}).Debug("Generated Mermaid graph")

	// classDef directives for category colors.
	activeCats := make(map[string]bool)
	for id := range connected {
		activeCats[CategoryForNode(id)] = true
	}
	sb.WriteString("\n")
	for _, catKey := range CategoryOrder {
		if !activeCats[catKey] {
			continue
		}
		cat := Categories[catKey]
		sb.WriteString(fmt.Sprintf("    classDef %s fill:%s,stroke:#333\n", catKey, cat.Color))
	}

	// Apply class to each node.
	for _, catKey := range CategoryOrder {
		nodes := groups[catKey]
		if len(nodes) == 0 {
			continue
		}
		ids := make([]string, len(nodes))
		for i, node := range nodes {
			ids[i] = sanitizeMermaidID(node)
		}
		sb.WriteString(fmt.Sprintf("    class %s %s\n", strings.Join(ids, ","), catKey))
	}

	return sb.String()
}
