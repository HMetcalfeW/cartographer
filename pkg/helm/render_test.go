package helm_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/HMetcalfeW/cartographer/pkg/helm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRenderChart_WithValues simulates a local Helm chart directory with a values file.
// It expects RenderChart to load the chart from disk and render the template correctly.
func TestRenderChart_WithValues(t *testing.T) {
	// Create a temporary directory to simulate a local Helm chart.
	chartDir, err := os.MkdirTemp("", "testchart")
	require.NoError(t, err, "failed to create temporary chart directory")
	defer func() {
		if e := os.RemoveAll(chartDir); e != nil {
			t.Logf("failed to remove temp chart dir: %v", e)
		}
	}()

	// Write a minimal Chart.yaml.
	chartYAML := `apiVersion: v2
name: testchart
version: 0.1.0
`
	err = os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(chartYAML), 0644)
	require.NoError(t, err, "failed to write Chart.yaml")

	// Create a templates directory.
	templatesDir := filepath.Join(chartDir, "templates")
	err = os.Mkdir(templatesDir, 0755)
	require.NoError(t, err, "failed to create templates directory")

	// Write a simple deployment template that uses .Values.name.
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
	defer func() {
		if e := os.RemoveAll(valuesFile.Name()); e != nil {
			t.Logf("failed to remove temp values file: %v", e)
		}
	}()
	_, err = valuesFile.Write([]byte(valuesYAML))
	require.NoError(t, err, "failed to write values file")
	err = valuesFile.Close()
	require.NoError(t, err, "failed to close values file")

	// Call RenderChart with the local chart directory.
	rendered, rErr := helm.RenderChart(
		chartDir,          // chart path (local directory)
		valuesFile.Name(), // values file
		"test-release",    // release name
		"",                // version empty
		"default",         // namespace
	)
	require.NoError(t, rErr, "RenderChart returned an error")
	t.Logf("Rendered output:\n%s", rendered)
	assert.Contains(t, rendered, "my-deployment", "rendered output should contain the name from values")
}

// TestRenderChart_Remote tests remote/chart pulling scenarios using RenderChart.
func TestRenderChart_Remote(t *testing.T) {
	tests := []struct {
		name        string
		chartRef    string // Either a bare chart name or a local alias (e.g., "bitnami/postgresql")
		version     string
		expectError string // Substring expected in error (if any)
		validate    func(t *testing.T, rendered string, err error)
	}{
		{
			name:     "DirectRepo_BareChart",
			chartRef: "oci://registry-1.docker.io/bitnamicharts/postgresql",
			version:  "16.4.7",
			validate: func(t *testing.T, rendered string, err error) {
				if err != nil {
					t.Skipf("Skipping remote OCI test (network unavailable or registry error): %v", err)
				}
				assert.NotEmpty(t, rendered, "rendered chart should not be empty")
				t.Logf("Rendered chart (direct repo):\n%s", rendered)
			},
		},
		{
			name:     "LocalAlias_Bitnami",
			chartRef: "bitnami/postgresql",
			version:  "16.4.7",
			validate: func(t *testing.T, rendered string, err error) {
				if err != nil {
					t.Skipf("Skipping local alias test (repo not configured or version mismatch): %v", err)
				}
				assert.NotEmpty(t, rendered, "rendered chart should not be empty")
				t.Logf("Rendered chart (local alias):\n%s", rendered)
			},
		},
		{
			name:        "LocalPath_NoSuchDir",
			chartRef:    "./definitelyDoesNotExist",
			version:     "",
			expectError: "failed to locate chart",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rendered, err := helm.RenderChart(tc.chartRef, "", "test-remote", tc.version, "default")
			if tc.expectError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectError)
			}
			if tc.validate != nil {
				tc.validate(t, rendered, err)
			}
		})
	}
}

func TestRenderChart_BadValuesFile(t *testing.T) {
	chartDir, err := os.MkdirTemp("", "testchart-badvals")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(chartDir) }()

	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: testchart\nversion: 0.1.0\n"), 0644))
	require.NoError(t, os.Mkdir(filepath.Join(chartDir, "templates"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "templates", "cm.yaml"), []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n"), 0644))

	_, err = helm.RenderChart(chartDir, "/tmp/nonexistent-values-file.yaml", "test", "", "default")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read values file")
}

func TestRenderChart_NoNamespace(t *testing.T) {
	chartDir, err := os.MkdirTemp("", "testchart-nons")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(chartDir) }()

	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: testchart\nversion: 0.1.0\n"), 0644))
	require.NoError(t, os.Mkdir(filepath.Join(chartDir, "templates"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "templates", "cm.yaml"), []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n"), 0644))

	rendered, err := helm.RenderChart(chartDir, "", "test", "", "")
	require.NoError(t, err)
	assert.Contains(t, rendered, "ConfigMap")
}

func TestRenderChart_NotesFilteredOut(t *testing.T) {
	chartDir, err := os.MkdirTemp("", "testchart-notes")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(chartDir) }()

	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: testchart\nversion: 0.1.0\n"), 0644))
	require.NoError(t, os.Mkdir(filepath.Join(chartDir, "templates"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "templates", "cm.yaml"), []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "templates", "NOTES.txt"), []byte("Thank you for installing!"), 0644))

	rendered, err := helm.RenderChart(chartDir, "", "test", "", "default")
	require.NoError(t, err)
	assert.Contains(t, rendered, "ConfigMap")
	assert.NotContains(t, rendered, "Thank you for installing")
}

func TestRenderChart_InvalidChartDir(t *testing.T) {
	// Existing directory but not a valid chart (no Chart.yaml).
	chartDir, err := os.MkdirTemp("", "testchart-invalid")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(chartDir) }()

	_, err = helm.RenderChart(chartDir, "", "test", "", "default")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load chart")
}
