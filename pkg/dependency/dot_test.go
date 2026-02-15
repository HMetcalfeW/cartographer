package dependency_test

import (
	"testing"

	"github.com/HMetcalfeW/cartographer/pkg/dependency"
	"github.com/stretchr/testify/assert"
)

// TestPrintDependencies ensures PrintDependencies doesn't panic and prints something.
func TestPrintDependencies(t *testing.T) {
	deps := map[string][]dependency.Edge{
		"Deployment/my-deploy": {
			{ChildID: "Secret/my-secret", Reason: "secretRef"},
			{ChildID: "ServiceAccount/my-sa", Reason: "serviceAccountName"},
		},
	}
	// Just ensuring it doesn't panic or error.
	dependency.PrintDependencies(deps)
}

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
