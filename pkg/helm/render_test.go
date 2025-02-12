package helm_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HMetcalfeW/cartographer/pkg/helm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderChart_WithValues(t *testing.T) {
	// Create a temporary directory to simulate a Helm chart.
	chartDir, err := os.MkdirTemp("", "testchart")
	require.NoError(t, err, "failed to create temporary chart directory")

	// Defer removal of the temp file and log any errors.
	defer func() {
		if err := os.Remove(chartDir); err != nil {
			t.Logf("failed to remove temp chart dir: %v", err)
		}
	}()

	// Write a minimal Chart.yaml.
	chartYAML := `apiVersion: v2
name: testchart
version: 0.1.0
`
	err = os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(chartYAML), 0644)
	require.NoError(t, err, "failed to write Chart.yaml")

	// Create a templates directory inside the chart.
	templatesDir := filepath.Join(chartDir, "templates")
	err = os.Mkdir(templatesDir, 0755)
	require.NoError(t, err, "failed to create templates directory")

	// Write a simple template that references .Values.name.
	templateContent := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Values.name }}
spec:
  replicas: 1
`
	err = os.WriteFile(filepath.Join(templatesDir, "deployment.yaml"), []byte(templateContent), 0644)
	require.NoError(t, err, "failed to write deployment template")

	// Create a temporary values file.
	valuesYAML := `name: my-deployment`
	valuesFile, err := os.CreateTemp("", "values-*.yaml")
	require.NoError(t, err, "failed to create temporary values file")

	// Defer removal of the temp file and log any errors.
	defer func() {
		if err := os.Remove(valuesFile.Name()); err != nil {
			t.Logf("failed to remove temp values file: %v", err)
		}
	}()

	_, err = valuesFile.Write([]byte(valuesYAML))
	require.NoError(t, err, "failed to write values file")
	err = valuesFile.Close()
	require.NoError(t, err, "failed to close values file")

	// Call RenderChart with the chart directory and the values file.
	rendered, err := helm.RenderChart(chartDir, valuesFile.Name())
	require.NoError(t, err, "RenderChart returned an error")

	// Debug output if needed.
	t.Logf("Rendered output:\n%s", rendered)

	// Verify that the rendered output contains the expected value.
	assert.True(t, strings.Contains(rendered, "my-deployment"),
		"rendered output should contain the name from values")
}
