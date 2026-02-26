package dependency

import (
	"fmt"
	"sort"
	"strings"
)

// GenerateDOT produces a DOT graph with resources color-coded by category
// (Workloads, Networking, Config & Storage, etc.). Nodes are colored with
// fill colors instead of grouped into subgraph clusters, allowing GraphViz
// to freely optimize node placement for minimal edge crossings.
// Only nodes that participate in at least one edge are emitted.
func GenerateDOT(deps map[string][]Edge) string {
	var sb strings.Builder
	sb.WriteString("digraph G {\n")
	sb.WriteString("  rankdir=\"LR\";\n")
	sb.WriteString("  node [shape=box, style=filled];\n\n")

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

	// Emit node declarations with category fill colors.
	nodeIDs := make([]string, 0, len(connected))
	for id := range connected {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)

	for _, node := range nodeIDs {
		cat := Categories[CategoryForNode(node)]
		sb.WriteString(fmt.Sprintf("  \"%s\" [fillcolor=\"%s\"];\n", node, cat.Color))
	}
	sb.WriteString("\n")

	// Edges.
	parents := make([]string, 0, len(deps))
	for p := range deps {
		parents = append(parents, p)
	}
	sort.Strings(parents)

	for _, parent := range parents {
		for _, edge := range deps[parent] {
			sb.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\" [label=\"%s\"];\n", parent, edge.ChildID, edge.Reason))
		}
	}

	// Determine which categories are present.
	activeCats := make(map[string]bool)
	for id := range connected {
		activeCats[CategoryForNode(id)] = true
	}

	// Legend as a single HTML-table node pushed to the rightmost rank.
	sb.WriteString("\n")
	sb.WriteString("  \"legend\" [shape=plaintext, label=<\n")
	sb.WriteString("    <TABLE BORDER=\"0\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"6\">\n")
	sb.WriteString("    <TR><TD COLSPAN=\"2\"><B>Legend</B></TD></TR>\n")
	for _, catKey := range CategoryOrder {
		if !activeCats[catKey] {
			continue
		}
		cat := Categories[catKey]
		htmlLabel := strings.ReplaceAll(cat.Label, "&", "&amp;")
		sb.WriteString(fmt.Sprintf("    <TR><TD BGCOLOR=\"%s\">    </TD><TD>%s</TD></TR>\n", cat.Color, htmlLabel))
	}
	sb.WriteString("    </TABLE>\n")
	sb.WriteString("  >];\n")
	sb.WriteString("  { rank=sink; \"legend\"; }\n")

	sb.WriteString("}\n")
	return sb.String()
}
