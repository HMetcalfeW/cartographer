package dependency

import (
	"encoding/json"
	"sort"
)

// JSONGraph is the top-level structure emitted by GenerateJSON.
type JSONGraph struct {
	Nodes []JSONNode `json:"nodes"`
	Edges []JSONEdge `json:"edges"`
}

// JSONNode represents a single Kubernetes resource in the graph.
type JSONNode struct {
	ID    string `json:"id"`
	Group string `json:"group"`
}

// JSONEdge represents a directed dependency between two resources.
type JSONEdge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason"`
}

// GenerateJSON produces an indented JSON string from the dependency map.
// The output is a graph object with separate "nodes" and "edges" arrays,
// suitable for consumption by jq, custom visualizers, or CI pipelines.
func GenerateJSON(deps map[string][]Edge) string {
	nodeSet := make(map[string]struct{})
	var edges []JSONEdge

	// Sort parent keys for deterministic output.
	parents := make([]string, 0, len(deps))
	for p := range deps {
		parents = append(parents, p)
	}
	sort.Strings(parents)

	for _, parent := range parents {
		nodeSet[parent] = struct{}{}
		for _, edge := range deps[parent] {
			nodeSet[edge.ChildID] = struct{}{}
			edges = append(edges, JSONEdge{
				From:   parent,
				To:     edge.ChildID,
				Reason: edge.Reason,
			})
		}
	}

	// Build sorted node list.
	nodeIDs := make([]string, 0, len(nodeSet))
	for id := range nodeSet {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)

	nodes := make([]JSONNode, len(nodeIDs))
	for i, id := range nodeIDs {
		nodes[i] = JSONNode{ID: id, Group: CategoryForNode(id)}
	}

	graph := JSONGraph{Nodes: nodes, Edges: edges}
	data, _ := json.MarshalIndent(graph, "", "  ")
	return string(data)
}
