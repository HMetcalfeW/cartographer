package dependency_test

import (
	"encoding/json"
	"testing"

	"github.com/HMetcalfeW/cartographer/pkg/dependency"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGenerateJSON ensures the JSON output includes edge reasons and node IDs.
func TestGenerateJSON(t *testing.T) {
	deps := map[string][]dependency.Edge{
		"Deployment/my-deploy": {
			{ChildID: "Secret/my-secret", Reason: "secretRef"},
			{ChildID: "ServiceAccount/my-sa", Reason: "serviceAccountName"},
		},
	}
	jsonStr := dependency.GenerateJSON(deps)
	t.Log(jsonStr)
	assert.Contains(t, jsonStr, `"secretRef"`)
	assert.Contains(t, jsonStr, `"serviceAccountName"`)
	assert.Contains(t, jsonStr, `"Deployment/my-deploy"`)
}

// TestGenerateJSON_EmptyDeps verifies JSON output for an empty dependency map.
func TestGenerateJSON_EmptyDeps(t *testing.T) {
	jsonStr := dependency.GenerateJSON(map[string][]dependency.Edge{})
	assert.Contains(t, jsonStr, `"nodes"`)
	assert.Contains(t, jsonStr, `"edges"`)
}

// TestGenerateJSON_StructureValid verifies the JSON is valid and parseable.
func TestGenerateJSON_StructureValid(t *testing.T) {
	deps := map[string][]dependency.Edge{
		"Service/web": {
			{ChildID: "Deployment/web", Reason: "selector"},
		},
	}
	jsonStr := dependency.GenerateJSON(deps)

	var graph dependency.JSONGraph
	err := json.Unmarshal([]byte(jsonStr), &graph)
	require.NoError(t, err, "JSON output must be valid")

	assert.Equal(t, 2, len(graph.Nodes), "should have 2 nodes")
	assert.Equal(t, 1, len(graph.Edges), "should have 1 edge")
	assert.Equal(t, "selector", graph.Edges[0].Reason)
}

// TestGenerateJSON_DeterministicOrder verifies the output is sorted and stable.
func TestGenerateJSON_DeterministicOrder(t *testing.T) {
	deps := map[string][]dependency.Edge{
		"Service/web":    {{ChildID: "Deployment/web", Reason: "selector"}},
		"Deployment/web": {{ChildID: "Secret/db-pass", Reason: "secretRef"}},
	}
	first := dependency.GenerateJSON(deps)
	second := dependency.GenerateJSON(deps)
	assert.Equal(t, first, second, "JSON output should be deterministic")
}

// TestGenerateJSON_GroupField verifies each node has the correct group assignment.
func TestGenerateJSON_GroupField(t *testing.T) {
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
	jsonStr := dependency.GenerateJSON(deps)

	var graph dependency.JSONGraph
	err := json.Unmarshal([]byte(jsonStr), &graph)
	require.NoError(t, err)

	groupByID := make(map[string]string)
	for _, node := range graph.Nodes {
		groupByID[node.ID] = node.Group
	}

	assert.Equal(t, "workloads", groupByID["Deployment/web"])
	assert.Equal(t, "networking", groupByID["Service/frontend"])
	assert.Equal(t, "config", groupByID["Secret/db-creds"])
	assert.Equal(t, "rbac", groupByID["RoleBinding/bind"])
	assert.Equal(t, "rbac", groupByID["Role/reader"])
}
