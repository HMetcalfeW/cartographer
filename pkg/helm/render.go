package helm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/engine"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"
	"sigs.k8s.io/yaml"
)

// initActionConfig initializes an action.Configuration using the provided Helm environment settings.
func initActionConfig(settings *cli.EnvSettings) (*action.Configuration, error) {
	actionConfig := new(action.Configuration)
	// Use the default RESTClientGetter and HELM_DRIVER environment variable.
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		return nil, err
	}
	return actionConfig, nil
}

// newRegistryClient creates a new registry.Client using the provided settings.
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

// RenderChart pulls (or locates) a Helm chart, updates its dependencies if needed,
// merges user-provided values, and renders the chart templates.
// It returns a combined multi-document YAML string (only .yaml/.yml files).
//
// chartRef can be one of:
//  1. A local directory (with a Chart.yaml),
//  2. A local archive (*.tgz),
//  //  3. A local alias (e.g. "myrepo/mychart") – in which case your local Helm repo index is used,
//  4. A bare chart name for remote pulls (when --repo is provided).
func RenderChart(
	chartRef string, // chart reference
	valuesFile string, // optional path to values file
	releaseName string, // release name to inject
	version string, // optional chart version
	namespace string, // namespace for rendering
) (string, error) {

	logger := log.WithFields(log.Fields{
		"func":        "RenderChart",
		"chartRef":    chartRef,
		"valuesFile":  valuesFile,
		"releaseName": releaseName,
		"version":     version,
		"namespace":   namespace,
	})
	logger.Info("Starting Helm chart render")

	// Initialize Helm CLI settings.
	settings := cli.New()
	if namespace != "" {
		settings.SetNamespace(namespace)
	}

	// Step 1: If chartRef exists on disk (directory or archive), use it directly.
	if pathExists(chartRef) {
		localPath, err := filepath.Abs(chartRef)
		if err != nil {
			return "", err
		}
		logger.Infof("Using local chart from disk: %s", localPath)
		chartRef = localPath
	} else {
		// check to see if the chartRef is an OCI path
		if !registry.IsOCI(chartRef) {
			var cpo action.ChartPathOptions
			cpo.Version = version
			_, err := cpo.LocateChart(chartRef, settings)
			if err != nil {
				logger.WithError(err).Error("failed to locate chart using local repo alias")
				return "", fmt.Errorf("error: Helm chart '%s' could not be found. Ensure the Helm repository is added and the chart name is spelled correctly. If it's a local path, confirm the directory exists: %w", chartRef, err)
			}
			logger.WithField("resolvedChartPath", chartRef).Info("Chart located")

		} else {
			// Initialize action configuration.
			actionConfig, err := initActionConfig(settings)
			if err != nil {
				logger.WithError(err).Error("failed to initialize action configuration")
				return "", err
			}

			registryClient, err := newRegistryClient(settings, false)
			if err != nil {
				logger.WithError(err).Error("failed to create registry client")
				return "", err
			}
			actionConfig.RegistryClient = registryClient

			// Create pull options using the action configuration.
			pullOpts := action.WithConfig(actionConfig)
			// Create a new Pull client with the pull options.
			pullClient := action.NewPullWithOpts(pullOpts)

			// Set Settings so that the pull client has access to the CLI environment.
			pullClient.Settings = settings

			// Set destination and chart path options.
			pullClient.DestDir = os.TempDir()
			pullClient.Version = version
			pullClient.Verify = false

			// Use the Pull client to resolve (and pull) the chart.
			logger.WithField("chartRef", chartRef).Debug("Attempting to pull OCI chart")
			addlInfo, pullErr := pullClient.Run(chartRef)
			if pullErr != nil {
				logger.WithError(pullErr).WithField("addInfo", addlInfo).Error("failed to pull chart using Helm pull action")
				return "", fmt.Errorf("error: Failed to pull OCI chart '%s'. Please check the chart reference, registry availability, and your authentication: %w", chartRef, pullErr)
			}
			logger.WithField("chartRef", chartRef).Info("Successfully pulled OCI chart")

			/**
			* Sadly the Helm SDK's pull function does not return a string of where it actually saved
			* the Helm chart. Looking at the pull.go reference, work would need to be done to preserve
			* this variable within the Run function so folks don't need to rewrite. Below is a workaround
			**/

			// Infer the chart name from the chartRef
			chartName, err := inferChartName(chartRef)
			if err != nil {
				return "", err
			}

			// Determine the expected file name using glob patterns
			var pattern string
			if version != "" {
				// When a version is specified, expect an exact match
				pattern = fmt.Sprintf("%s-%s.tgz", chartName, version)
			} else {
				// Otherwise, match any file that starts with the chart name
				pattern = fmt.Sprintf("%s*.tgz", chartName)
			}

			// Search for the chart file
			matches, err := filepath.Glob(filepath.Join(os.TempDir(), pattern))
			if err != nil {
				return "", err
			}
			if len(matches) == 0 {
				return "", fmt.Errorf("no chart file found matching pattern: %s", pattern)
			}

			// use the first match
			chartRef = matches[0]
			logger.Infof("Chart pulled to: %s", chartRef)
		}
	}

	// Load the chart from the resolved path.
	logger.WithField("chartPath", chartRef).Debug("Loading chart from path")
	ch, err := loader.Load(chartRef)
	if err != nil {
		logger.WithError(err).Error("failed to load chart")
		return "", fmt.Errorf("error: Failed to load Helm chart from '%s'. This might indicate a corrupted chart or an invalid chart format: %w", chartRef, err)
	}
	logger.WithField("chartName", ch.Name()).Info("Successfully loaded chart")

	// Check and update chart dependencies if necessary.
	if ch.Metadata.Dependencies != nil {
		if err := action.CheckDependencies(ch, ch.Metadata.Dependencies); err != nil {
			providers := getter.All(settings)
			manager := &downloader.Manager{
				Out:              os.Stdout,
				ChartPath:        chartRef,
				Keyring:          pullClientKeyring(), // returns empty keyring in this implementation
				SkipUpdate:       false,
				Getters:          providers,
				RepositoryConfig: settings.RepositoryConfig,
				RepositoryCache:  settings.RepositoryCache,
				Debug:            settings.Debug,
			}
			if err := manager.Update(); err != nil {
				return "", fmt.Errorf("failed to update chart dependencies: %w", err)
			}
			// Reload the chart after dependency update.
			ch, err = loader.Load(chartRef)
			if err != nil {
				return "", fmt.Errorf("failed to reload chart after dependency update: %w", err)
			}
		}
	}

	// Read user-provided values, if any.
	userValues := map[string]interface{}{}
	if valuesFile != "" {
		logger.WithField("valuesFile", valuesFile).Debug("Reading values file")
		data, err := os.ReadFile(valuesFile)
		if err != nil {
			logger.WithError(err).Error("failed to read values file")
			if os.IsNotExist(err) {
				return "", fmt.Errorf("error: Values file not found at '%s'. Please verify the file path and ensure it exists: %w", valuesFile, err)
			}
			return "", fmt.Errorf("failed to read values file '%s': %w", valuesFile, err)
		}
		logger.WithField("valuesFile", valuesFile).Debug("Unmarshaling values file")
		if err := yaml.Unmarshal(data, &userValues); err != nil {
			logger.WithError(err).Error("failed to unmarshal values file")
			return "", err
		}
		logger.WithField("valuesFile", valuesFile).Info("Successfully processed values file")
	}

	coalesced, err := chartutil.CoalesceValues(ch, userValues)
	if err != nil {
		logger.WithError(err).Error("failed to coalesce values")
		return "", fmt.Errorf("failed to coalesce values: %w", err)
	}
	logger.Debug("Successfully coalesced values")

	renderVals, err := chartutil.ToRenderValues(ch, coalesced, chartutil.ReleaseOptions{
		Name:      releaseName,
		Namespace: namespace,
	}, nil)
	if err != nil {
		logger.WithError(err).Error("failed to prepare render values")
		return "", fmt.Errorf("failed to prepare render values: %w", err)
	}
	logger.Debug("Successfully prepared render values")

	// Render the chart templates.
	logger.Debug("Rendering chart templates")
	renderedFiles, err := engine.Render(ch, renderVals)
	if err != nil {
		logger.WithError(err).Error("failed to render chart templates")
		return "", fmt.Errorf("failed to render chart templates: %w", err)
	}
	logger.Info("Successfully rendered chart templates")

	// Combine only YAML files.
	var combined strings.Builder
	for fname, content := range renderedFiles {
		if strings.HasSuffix(fname, ".yaml") || strings.HasSuffix(fname, ".yml") {
			combined.WriteString(content)
			combined.WriteString("\n---\n")
		}
	}

	logger.Info("Successfully rendered chart")
	return combined.String(), nil
}

// pullClientKeyring returns the keyring used by the pull client.
// Since our implementation does not require a keyring, we return an empty string.
func pullClientKeyring() string {
	return ""
}

// pathExists returns true if the given path exists (file or directory).
func pathExists(p string) bool {
	if p == "" {
		return false
	}
	_, err := os.Stat(p)
	return err == nil
}

// inferChartName attempts to resolve the name of a Helm chart based on the chartRef passed to Render
// the last token of a chartRef is the chart. For example, in oci://registry-1.docker.io/mycharts/mychart
// keycloak is the Chart's name
func inferChartName(chartRef string) (string, error) {
	parts := strings.Split(chartRef, "/")
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid chart reference: %s", chartRef)
	}
	return parts[len(parts)-1], nil
}
