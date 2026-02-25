package dependency

import (
	"fmt"
	"strings"
)

// GenerateDOT produces a DOT graph where each parent node has directed edges
// to its child nodes, labeled with the Reason describing why the relationship exists.
//
// Example:
//
//	"Deployment/my-deploy" -> "Secret/my-secret" [label="secretRef"];
func GenerateDOT(deps map[string][]Edge) string {
	var sb strings.Builder
	sb.WriteString("digraph G {\n")
	sb.WriteString("  rankdir=\"LR\";\n")
	sb.WriteString("  node [shape=box];\n\n")

	for parent, edges := range deps {
		for _, edge := range edges {
			sb.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\" [label=\"%s\"];\n", parent, edge.ChildID, edge.Reason))
		}
	}
	sb.WriteString("}\n")
	return sb.String()
}
