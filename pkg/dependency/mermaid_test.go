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
