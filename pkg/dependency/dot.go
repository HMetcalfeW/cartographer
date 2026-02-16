package dependency

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
)

// PrintDependencies logs each parent and its dependencies (Edges) at the Info level.
// It prints both the child resource identifiers and the reason for each dependency.
func PrintDependencies(deps map[string][]Edge) {
	logger := log.WithField("func", "PrintDependencies")
	logger.Info("Printing dependency relationships")

	for parent, edges := range deps {
		if len(edges) == 0 {
			continue
		}
		childStrings := make([]string, 0, len(edges))
		for _, e := range edges {
			childStrings = append(childStrings, fmt.Sprintf("%s(%s)", e.ChildID, e.Reason))
		}
		logger.WithFields(log.Fields{
			"parent": parent,
			"edges":  childStrings,
		}).Info("Dependency relationship")
	}
}

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
