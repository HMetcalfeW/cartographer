package dependency_test

import (
	"testing"

	"github.com/HMetcalfeW/cartographer/pkg/dependency"
	"github.com/stretchr/testify/assert"
)

// TestGenerateDOT ensures the DOT output includes reason labels.
func TestGenerateDOT(t *testing.T) {
	deps := map[string][]dependency.Edge{
		"Deployment/my-deploy": {
			{ChildID: "Secret/my-secret", Reason: "secretRef"},
			{ChildID: "ServiceAccount/my-sa", Reason: "serviceAccountName"},
		},
	}
	dot := dependency.GenerateDOT(deps)
	t.Log(dot)
	assert.Contains(t, dot, "[label=\"secretRef\"]")
	assert.Contains(t, dot, "[label=\"serviceAccountName\"]")
}

// TestGenerateDOT_EmptyDeps verifies DOT output for an empty dependency map.
func TestGenerateDOT_EmptyDeps(t *testing.T) {
	dot := dependency.GenerateDOT(map[string][]dependency.Edge{})
	assert.Contains(t, dot, "digraph G {")
	assert.Contains(t, dot, "}")
	// No edges should be present
	assert.NotContains(t, dot, "->")
}

// TestGenerateDOT_OrphansOmitted verifies that resources with no edges
// are not emitted in the DOT output.
func TestGenerateDOT_OrphansOmitted(t *testing.T) {
	deps := map[string][]dependency.Edge{
		"ConfigMap/standalone": {},
		"Deployment/web": {
			{ChildID: "Secret/db-pass", Reason: "secretRef"},
		},
	}
	dot := dependency.GenerateDOT(deps)
	assert.Contains(t, dot, `"Deployment/web" -> "Secret/db-pass"`)
	assert.NotContains(t, dot, "ConfigMap/standalone")
}

// TestGenerateDOT_ColorCoded verifies nodes are color-coded by category.
func TestGenerateDOT_ColorCoded(t *testing.T) {
	deps := map[string][]dependency.Edge{
		"Deployment/web": {
			{ChildID: "Secret/db-creds", Reason: "secretRef"},
		},
		"Service/frontend": {
			{ChildID: "Deployment/web", Reason: "selector"},
		},
		"RoleBinding/bind": {
			{ChildID: "Role/reader", Reason: "roleRef"},
		},
	}
	dot := dependency.GenerateDOT(deps)

	// Nodes should have fillcolor attributes
	assert.Contains(t, dot, `"Deployment/web" [fillcolor=`)
	assert.Contains(t, dot, `"Service/frontend" [fillcolor=`)
	assert.Contains(t, dot, `"Secret/db-creds" [fillcolor=`)
	assert.Contains(t, dot, `"RoleBinding/bind" [fillcolor=`)
	assert.Contains(t, dot, `"Role/reader" [fillcolor=`)

	// No subgraph clusters (color-coded instead)
	assert.NotContains(t, dot, "subgraph cluster_workloads")

	// Edges still present
	assert.Contains(t, dot, `"Deployment/web" -> "Secret/db-creds" [label="secretRef"]`)
	assert.Contains(t, dot, `"Service/frontend" -> "Deployment/web" [label="selector"]`)
	assert.Contains(t, dot, `"RoleBinding/bind" -> "Role/reader" [label="roleRef"]`)
}

// TestGenerateDOT_StructureValid verifies the overall DOT structure.
func TestGenerateDOT_StructureValid(t *testing.T) {
	deps := map[string][]dependency.Edge{
		"Service/web": {
			{ChildID: "Deployment/web", Reason: "selector"},
		},
	}
	dot := dependency.GenerateDOT(deps)
	assert.True(t, len(dot) > 0)
	// Must start with digraph and end with closing brace
	assert.Contains(t, dot, "digraph G {")
	assert.Contains(t, dot, `rankdir="LR"`)
	assert.Contains(t, dot, "style=filled")
	assert.Contains(t, dot, `[label="selector"]`)

	// Count braces
	open := 0
	for _, c := range dot {
		if c == '{' {
			open++
		}
		if c == '}' {
			open--
		}
	}
	assert.Equal(t, 0, open, "braces should be balanced")
}

// TestGenerateDOT_DeterministicOrder verifies node and edge output is sorted.
func TestGenerateDOT_DeterministicOrder(t *testing.T) {
	deps := map[string][]dependency.Edge{
		"Service/web":    {{ChildID: "Deployment/web", Reason: "selector"}},
		"Deployment/web": {{ChildID: "Secret/db-pass", Reason: "secretRef"}},
	}
	first := dependency.GenerateDOT(deps)
	second := dependency.GenerateDOT(deps)
	assert.Equal(t, first, second, "DOT output should be deterministic")
}
