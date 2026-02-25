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

// TestGenerateDOT_LonersOmitted verifies that resources with no edges
// don't produce edge lines in the DOT output.
func TestGenerateDOT_LonersOmitted(t *testing.T) {
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
	assert.Contains(t, dot, "node [shape=box]")
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
