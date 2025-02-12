package helm

import (
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"sigs.k8s.io/yaml"
)

// RenderChart loads a Helm chart from the given chartPath and renders it using an optional
// values file (valuesFile). It returns the combined YAML output.
func RenderChart(chartPath string, valuesFile string) (string, error) {
	logger := log.WithFields(log.Fields{
		"func":       "RenderChart",
		"chartPath":  chartPath,
		"valuesFile": valuesFile,
	})

	// Load the chart from the specified path.
	chart, err := loader.Load(chartPath)
	if err != nil {
		logger.Error("failed to load chart")
		return "", err
	}

	// Read user-provided values, if any.
	values := map[string]interface{}{}
	if valuesFile != "" {
		data, err := os.ReadFile(valuesFile)
		if err != nil {
			logger.Error("failed to read values file")
			return "", err
		}
		if err := yaml.Unmarshal(data, &values); err != nil {
			logger.Error("failed to unmarshal values file")
			return "", err
		}
	}

	// Merge the chart's default values with the user-provided overrides using Helm's CoalesceValues.
	mergedValues, err := chartutil.CoalesceValues(chart, values)
	if err != nil {
		logger.Error("failed to merge values")
		return "", err
	}

	// Create a full render context using ToRenderValues. This ensures that the context
	// includes the proper .Values, .Chart, etc., needed for rendering.
	renderContext, err := chartutil.ToRenderValues(chart, mergedValues, chartutil.ReleaseOptions{}, nil)
	if err != nil {
		logger.Error("failed to create render context")
		return "", err
	}

	// Render the chart templates using the Helm engine.
	renderedFiles, err := engine.Render(chart, renderContext)
	if err != nil {
		logger.Error("failed to render chart")
		return "", err
	}

	var k8sManifests strings.Builder
	// Combine rendered YAML files, filtering for files ending with .yaml or .yml.
	for file, content := range renderedFiles {
		if !strings.HasSuffix(file, ".yaml") && !strings.HasSuffix(file, ".yml") {
			continue
		}
		k8sManifests.WriteString(content)
		k8sManifests.WriteString("\n---\n")
	}

	logger.WithField("k8sManifests", k8sManifests).Info("Successfully rendered chart")
	return k8sManifests.String(), nil
}
