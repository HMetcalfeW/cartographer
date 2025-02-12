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
// values file (valuesFile), a release name (releaseName), and a repo URL (repoURL).
// If the chart is not found locally and repoURL is provided, it will return an error
// (remote chart downloading is not implemented in this example).
func RenderChart(chartPath string, valuesFile string, releaseName string, repoURL string) (string, error) {
	logger := log.WithFields(log.Fields{
		"func":        "RenderChart",
		"chartPath":   chartPath,
		"valuesFile":  valuesFile,
		"releaseName": releaseName,
		"repoURL":     repoURL,
	})

	// Check if chartPath exists locally.
	if _, err := os.Stat(chartPath); os.IsNotExist(err) {
		if repoURL != "" {
			logger.Error("chart not found locally; remote chart downloading not implemented")
			return "", err
		}
		logger.Error("chart not found locally and no repoURL provided")
		return "", err
	}

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

	// Merge the chart's default values with user-provided overrides.
	mergedValues, err := chartutil.CoalesceValues(chart, values)
	if err != nil {
		logger.Error("failed to merge values")
		return "", err
	}

	// Create release options using the provided release name.
	releaseOptions := chartutil.ReleaseOptions{
		Name:      releaseName,
		Namespace: "default", // For now, we use "default". This could be made configurable.
	}

	// Build a full render context using Helm's ToRenderValues.
	renderContext, err := chartutil.ToRenderValues(chart, mergedValues, releaseOptions, nil)
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

	logger.Info("Successfully rendered chart")
	return k8sManifests.String(), nil
}
