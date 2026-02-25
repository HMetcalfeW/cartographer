package dependency_test

import (
	"os/exec"
	"testing"

	"github.com/HMetcalfeW/cartographer/pkg/dependency"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func graphvizAvailable() bool {
	_, err := exec.LookPath("dot")
	return err == nil
}

// TestRenderImage_PNG verifies PNG output contains valid PNG header bytes.
func TestRenderImage_PNG(t *testing.T) {
	if !graphvizAvailable() {
		t.Skip("graphviz not installed, skipping image render test")
	}
	deps := map[string][]dependency.Edge{
		"Deployment/web": {
			{ChildID: "Secret/db-pass", Reason: "secretRef"},
		},
	}
	data, err := dependency.RenderImage(deps, "png")
	require.NoError(t, err)
	assert.True(t, len(data) > 0, "PNG output should not be empty")
	// PNG files start with the magic bytes: 0x89 0x50 0x4E 0x47
	assert.Equal(t, byte(0x89), data[0], "should start with PNG magic byte")
	assert.Equal(t, byte(0x50), data[1])
	assert.Equal(t, byte(0x4E), data[2])
	assert.Equal(t, byte(0x47), data[3])
}

// TestRenderImage_SVG verifies SVG output contains expected XML/SVG content.
func TestRenderImage_SVG(t *testing.T) {
	if !graphvizAvailable() {
		t.Skip("graphviz not installed, skipping image render test")
	}
	deps := map[string][]dependency.Edge{
		"Service/web": {
			{ChildID: "Deployment/web", Reason: "selector"},
		},
	}
	data, err := dependency.RenderImage(deps, "svg")
	require.NoError(t, err)
	svgStr := string(data)
	assert.Contains(t, svgStr, "<svg")
	assert.Contains(t, svgStr, "</svg>")
}

// TestRenderImage_EmptyDeps verifies rendering works with no edges.
func TestRenderImage_EmptyDeps(t *testing.T) {
	if !graphvizAvailable() {
		t.Skip("graphviz not installed, skipping image render test")
	}
	data, err := dependency.RenderImage(map[string][]dependency.Edge{}, "png")
	require.NoError(t, err)
	assert.True(t, len(data) > 0, "even empty graph should produce valid PNG")
}
