package helm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/engine"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"
	"sigs.k8s.io/yaml"
)

// RenderChart pulls (or locates) a Helm chart, updates its dependencies if needed,
// merges user-provided values, and renders the chart templates.
// It returns a combined multi-document YAML string (only .yaml/.yml files).
func RenderChart(chartRef, valuesFile, releaseName, version, namespace string) (string, error) {
	logger := log.WithFields(log.Fields{
		"func":     "RenderChart",
		"chartRef": chartRef,
	})
	logger.Info("Starting Helm chart render")

	settings := cli.New()
	if namespace != "" {
		settings.SetNamespace(namespace)
	}

	// Resolve chartRef to a local path.
	resolvedPath, err := resolveChartPath(chartRef, version, settings)
	if err != nil {
		return "", err
	}

	// Load the chart from the resolved path.
	ch, err := loader.Load(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("failed to load chart: %w", err)
	}

	// Update chart dependencies if needed.
	ch, err = updateDependencies(ch, resolvedPath, settings)
	if err != nil {
		return "", err
	}

	// Merge user values and render templates.
	return renderTemplates(ch, valuesFile, releaseName, namespace)
}

// resolveChartPath determines the local filesystem path for a chart reference.
// It handles local paths, repo aliases, and OCI registries.
func resolveChartPath(chartRef, version string, settings *cli.EnvSettings) (string, error) {
	// Local directory or archive.
	if pathExists(chartRef) {
		return filepath.Abs(chartRef)
	}

	// OCI registry.
	if registry.IsOCI(chartRef) {
		return pullOCIChart(chartRef, version, settings)
	}

	// Local Helm repo alias (e.g. "bitnami/postgresql").
	var cpo action.ChartPathOptions
	cpo.Version = version
	resolved, err := cpo.LocateChart(chartRef, settings)
	if err != nil {
		return "", fmt.Errorf("failed to locate chart: %w", err)
	}
	return resolved, nil
}

// pullOCIChart pulls a chart from an OCI registry and returns the local archive path.
func pullOCIChart(chartRef, version string, settings *cli.EnvSettings) (string, error) {
	actionConfig, err := initActionConfig(settings)
	if err != nil {
		return "", fmt.Errorf("failed to initialize action configuration: %w", err)
	}

	registryClient, err := newRegistryClient(settings, false)
	if err != nil {
		return "", fmt.Errorf("failed to create registry client: %w", err)
	}
	actionConfig.RegistryClient = registryClient

	pullClient := action.NewPullWithOpts(action.WithConfig(actionConfig))
	pullClient.Settings = settings
	pullClient.DestDir = os.TempDir()
	pullClient.Version = version
	pullClient.Verify = false

	if _, err := pullClient.Run(chartRef); err != nil {
		return "", fmt.Errorf("failed to pull chart %q (version %q): %w", chartRef, version, err)
	}

	// The Helm SDK doesn't return the download path, so we infer it.
	chartName, err := inferChartName(chartRef)
	if err != nil {
		return "", err
	}

	var pattern string
	if version != "" {
		pattern = fmt.Sprintf("%s-%s.tgz", chartName, version)
	} else {
		pattern = fmt.Sprintf("%s*.tgz", chartName)
	}

	matches, err := filepath.Glob(filepath.Join(os.TempDir(), pattern))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no chart file found matching pattern: %s", pattern)
	}

	return matches[0], nil
}

// updateDependencies checks if the chart's dependencies are satisfied and
// downloads them if needed, returning the (potentially reloaded) chart.
func updateDependencies(ch *chart.Chart, chartPath string, settings *cli.EnvSettings) (*chart.Chart, error) {
	if ch.Metadata.Dependencies == nil {
		return ch, nil
	}
	if err := action.CheckDependencies(ch, ch.Metadata.Dependencies); err == nil {
		return ch, nil
	}

	manager := &downloader.Manager{
		Out:              os.Stdout,
		ChartPath:        chartPath,
		Keyring:          "",
		SkipUpdate:       false,
		Getters:          getter.All(settings),
		RepositoryConfig: settings.RepositoryConfig,
		RepositoryCache:  settings.RepositoryCache,
		Debug:            settings.Debug,
	}
	if err := manager.Update(); err != nil {
		return nil, fmt.Errorf("failed to update chart dependencies: %w", err)
	}

	reloaded, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("failed to reload chart after dependency update: %w", err)
	}
	return reloaded, nil
}

// renderTemplates merges values and renders chart templates, returning combined YAML.
func renderTemplates(ch *chart.Chart, valuesFile, releaseName, namespace string) (string, error) {
	userValues := map[string]interface{}{}
	if valuesFile != "" {
		data, err := os.ReadFile(valuesFile)
		if err != nil {
			return "", fmt.Errorf("failed to read values file: %w", err)
		}
		if err := yaml.Unmarshal(data, &userValues); err != nil {
			return "", fmt.Errorf("failed to unmarshal values file: %w", err)
		}
	}

	coalesced, err := chartutil.CoalesceValues(ch, userValues)
	if err != nil {
		return "", fmt.Errorf("failed to coalesce values: %w", err)
	}

	renderVals, err := chartutil.ToRenderValues(ch, coalesced, chartutil.ReleaseOptions{
		Name:      releaseName,
		Namespace: namespace,
	}, nil)
	if err != nil {
		return "", fmt.Errorf("failed to prepare render values: %w", err)
	}

	// Filter out non-manifest templates (e.g. NOTES.txt).
	filtered := make([]*chart.File, 0, len(ch.Templates))
	for _, t := range ch.Templates {
		if strings.EqualFold(filepath.Base(t.Name), "NOTES.txt") {
			continue
		}
		filtered = append(filtered, t)
	}
	ch.Templates = filtered

	renderedFiles, err := engine.Render(ch, renderVals)
	if err != nil {
		return "", fmt.Errorf("failed to render chart templates: %w", err)
	}

	var combined strings.Builder
	for fname, content := range renderedFiles {
		if strings.HasSuffix(fname, ".yaml") || strings.HasSuffix(fname, ".yml") {
			combined.WriteString(content)
			combined.WriteString("\n---\n")
		}
	}

	return combined.String(), nil
}

func initActionConfig(settings *cli.EnvSettings) (*action.Configuration, error) {
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		return nil, err
	}
	return actionConfig, nil
}

func newRegistryClient(settings *cli.EnvSettings, plainHTTP bool) (*registry.Client, error) {
	opts := []registry.ClientOption{
		registry.ClientOptDebug(settings.Debug),
		registry.ClientOptEnableCache(true),
		registry.ClientOptWriter(log.New().Writer()),
		registry.ClientOptCredentialsFile(settings.RegistryConfig),
	}
	if plainHTTP {
		opts = append(opts, registry.ClientOptPlainHTTP())
	}
	return registry.NewClient(opts...)
}

func pathExists(p string) bool {
	if p == "" {
		return false
	}
	_, err := os.Stat(p)
	return err == nil
}

func inferChartName(chartRef string) (string, error) {
	parts := strings.Split(chartRef, "/")
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid chart reference: %s", chartRef)
	}
	return parts[len(parts)-1], nil
}
