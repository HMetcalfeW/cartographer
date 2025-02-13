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
		"",                // repoURL empty => use local chart
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
		repoURL     string // If non-empty, must be used with a bare chart name.
		version     string
		expectError string // Substring expected in error (if any)
		validate    func(rendered string, err error)
	}{
		{
			name:     "DirectRepo_BareChart",
			chartRef: "postgresql",
			repoURL:  "https://charts.bitnami.com/bitnami",
			version:  "16.4.7",
			validate: func(rendered string, err error) {
				require.NoError(t, err, "expected direct repo fetch to succeed")
				// We expect some rendered YAML output; check that it's non-empty.
				assert.NotEmpty(t, rendered, "rendered chart should not be empty")
				t.Logf("Rendered chart (direct repo):\n%s", rendered)
			},
		},
		{
			name:     "LocalAlias_Bitnami",
			chartRef: "bitnami/postgresql",
			repoURL:  "",
			version:  "16.4.7",
			validate: func(rendered string, err error) {
				// If the local alias isn't set up or version mismatches, skip the test.
				if err != nil {
					if strings.Contains(err.Error(), "failed to pull chart") {
						t.Skipf("Skipping local alias test; alias not found or version mismatch: %v", err)
					} else {
						t.Fatalf("Unexpected error: %v", err)
					}
				} else {
					assert.NotEmpty(t, rendered, "rendered chart should not be empty")
					t.Logf("Rendered chart (local alias):\n%s", rendered)
				}
			},
		},
		{
			name:        "Conflict_BitnamiPlusRepo",
			chartRef:    "bitnami/postgresql",
			repoURL:     "https://charts.bitnami.com/bitnami",
			version:     "16.4.7",
			expectError: "cannot specify --repo together with an alias",
		},
		{
			name:        "LocalPath_NoSuchDir",
			chartRef:    "./definitelyDoesNotExist",
			repoURL:     "",
			version:     "",
			expectError: "failed to pull chart",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rendered, err := helm.RenderChart(tc.chartRef, "", "test-remote", tc.repoURL, tc.version, "default")
			if tc.expectError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectError)
			} else {
				require.NoError(t, err)
			}
			if tc.validate != nil {
				tc.validate(rendered, err)
			}
		})
	}
}
