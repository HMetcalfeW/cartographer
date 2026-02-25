package dependency

import (
	"fmt"
	"strings"
)

// sanitizeMermaidID replaces characters that are invalid in Mermaid node
// identifiers (/, -, .) with underscores.
func sanitizeMermaidID(id string) string {
	r := strings.NewReplacer("/", "_", "-", "_", ".", "_")
	return r.Replace(id)
}

// GenerateMermaid produces a Mermaid flowchart (left-to-right) from the
// dependency map. Each node uses a sanitized ID with the original
// "Kind/Name" shown as a label in square brackets. Edge labels show the
// dependency reason.
func GenerateMermaid(deps map[string][]Edge) string {
	var sb strings.Builder
	sb.WriteString("graph LR\n")

	for parent, edges := range deps {
		for _, edge := range edges {
			parentID := sanitizeMermaidID(parent)
			childID := sanitizeMermaidID(edge.ChildID)
			sb.WriteString(fmt.Sprintf(
				"    %s[\"%s\"] --> |%s| %s[\"%s\"]\n",
				parentID, parent, edge.Reason, childID, edge.ChildID,
			))
		}
	}
	return sb.String()
}
