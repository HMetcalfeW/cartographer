package dependency_test

import (
	"testing"

	"github.com/HMetcalfeW/cartographer/pkg/dependency"
	"github.com/stretchr/testify/assert"
)

// TestGenerateMermaid ensures the Mermaid output includes edge labels and node labels.
func TestGenerateMermaid(t *testing.T) {
	deps := map[string][]dependency.Edge{
		"Deployment/my-deploy": {
			{ChildID: "Secret/my-secret", Reason: "secretRef"},
			{ChildID: "ServiceAccount/my-sa", Reason: "serviceAccountName"},
		},
	}
	mermaid := dependency.GenerateMermaid(deps)
	t.Log(mermaid)
	assert.Contains(t, mermaid, "|secretRef|")
	assert.Contains(t, mermaid, "|serviceAccountName|")
	assert.Contains(t, mermaid, `"Deployment/my-deploy"`)
	assert.Contains(t, mermaid, `"Secret/my-secret"`)
}

// TestGenerateMermaid_EmptyDeps verifies Mermaid output for an empty dependency map.
func TestGenerateMermaid_EmptyDeps(t *testing.T) {
	mermaid := dependency.GenerateMermaid(map[string][]dependency.Edge{})
	assert.Contains(t, mermaid, "graph LR")
	assert.NotContains(t, mermaid, "-->")
}

// TestGenerateMermaid_StructureValid verifies the overall Mermaid structure.
func TestGenerateMermaid_StructureValid(t *testing.T) {
	deps := map[string][]dependency.Edge{
		"Service/web": {
			{ChildID: "Deployment/web", Reason: "selector"},
		},
	}
	mermaid := dependency.GenerateMermaid(deps)
	assert.True(t, len(mermaid) > 0)
	assert.Contains(t, mermaid, "graph LR")
	// Sanitized IDs should not contain slashes
	assert.Contains(t, mermaid, "Service_web")
	assert.Contains(t, mermaid, "Deployment_web")
	// Original labels should appear in quotes
	assert.Contains(t, mermaid, `"Service/web"`)
	assert.Contains(t, mermaid, `"Deployment/web"`)
}

// TestGenerateMermaid_SanitizedIDs verifies that slashes and hyphens are replaced.
func TestGenerateMermaid_SanitizedIDs(t *testing.T) {
	deps := map[string][]dependency.Edge{
		"Deployment/my-app.v2": {
			{ChildID: "Secret/db-pass", Reason: "secretRef"},
		},
	}
	mermaid := dependency.GenerateMermaid(deps)
	assert.Contains(t, mermaid, "Deployment_my_app_v2")
	assert.Contains(t, mermaid, "Secret_db_pass")
}

// TestGenerateMermaid_ColorCoded verifies classDef directives and class assignments.
func TestGenerateMermaid_ColorCoded(t *testing.T) {
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
	mermaid := dependency.GenerateMermaid(deps)

	// No subgraph clusters (color-coded instead)
	assert.NotContains(t, mermaid, "subgraph")

	// Node declarations
	assert.Contains(t, mermaid, `Deployment_web["Deployment/web"]`)
	assert.Contains(t, mermaid, `Service_frontend["Service/frontend"]`)
	assert.Contains(t, mermaid, `Secret_db_creds["Secret/db-creds"]`)
	assert.Contains(t, mermaid, `RoleBinding_bind["RoleBinding/bind"]`)
	assert.Contains(t, mermaid, `Role_reader["Role/reader"]`)

	// Edges
	assert.Contains(t, mermaid, "Deployment_web --> |secretRef| Secret_db_creds")
	assert.Contains(t, mermaid, "Service_frontend --> |selector| Deployment_web")
	assert.Contains(t, mermaid, "RoleBinding_bind --> |roleRef| Role_reader")

	// classDef directives with category colors
	assert.Contains(t, mermaid, "classDef workloads fill:#DAEEF3,stroke:#333")
	assert.Contains(t, mermaid, "classDef networking fill:#E2EFDA,stroke:#333")
	assert.Contains(t, mermaid, "classDef config fill:#FFF2CC,stroke:#333")
	assert.Contains(t, mermaid, "classDef rbac fill:#E2D9F3,stroke:#333")

	// class assignments
	assert.Contains(t, mermaid, "class Deployment_web workloads")
	assert.Contains(t, mermaid, "class Service_frontend networking")
	assert.Contains(t, mermaid, "class Secret_db_creds config")

	// Categories with no nodes should not have classDef
	assert.NotContains(t, mermaid, "classDef autoscaling")
}

// TestGenerateMermaid_OrphansOmitted verifies that resources with no edges
// are not emitted in the Mermaid output.
func TestGenerateMermaid_OrphansOmitted(t *testing.T) {
	deps := map[string][]dependency.Edge{
		"ConfigMap/standalone": {},
		"Deployment/web": {
			{ChildID: "Secret/db-pass", Reason: "secretRef"},
		},
	}
	mermaid := dependency.GenerateMermaid(deps)
	assert.Contains(t, mermaid, `Deployment_web["Deployment/web"]`)
	assert.Contains(t, mermaid, `Secret_db_pass["Secret/db-pass"]`)
	assert.NotContains(t, mermaid, "ConfigMap/standalone")
}

// TestGenerateMermaid_DeterministicOrder verifies output is sorted and stable.
func TestGenerateMermaid_DeterministicOrder(t *testing.T) {
	deps := map[string][]dependency.Edge{
		"Service/web":    {{ChildID: "Deployment/web", Reason: "selector"}},
		"Deployment/web": {{ChildID: "Secret/db-pass", Reason: "secretRef"}},
	}
	first := dependency.GenerateMermaid(deps)
	second := dependency.GenerateMermaid(deps)
	assert.Equal(t, first, second, "Mermaid output should be deterministic")
}
