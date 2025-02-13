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
// This function is similar to what the Helm CLI uses.
func initActionConfig(settings *cli.EnvSettings) (*action.Configuration, error) {
	actionConfig := new(action.Configuration)
	// Use the default RESTClientGetter and the HELM_DRIVER from the environment.
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

	// Create a new registry client
	registryClient, err := registry.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return registryClient, nil
}

// RenderChart pulls (or locates) a Helm chart, updates its dependencies if needed,
// merges user-provided values, and renders the chart templates. It returns a combined
// multi-document YAML string (only .yaml/.yml files).
//
// chartRef can be one of:
//  1. A local directory (with a Chart.yaml),
//  2. A local archive (*.tgz),
//  3. A local alias (e.g. "bitnami/postgresql") – in which case your local Helm repo index is used,
//  4. A bare chart name for remote pulls (when --repo is provided).
func RenderChart(
	chartRef string, // chart reference
	valuesFile string, // optional path to values file
	releaseName string, // release name to inject
	repoURL string, // optional repository URL; if provided, chartRef should be bare (e.g. "postgresql")
	version string, // optional chart version
	namespace string, // namespace for rendering
) (string, error) {

	logger := log.WithFields(log.Fields{
		"func":        "RenderChart",
		"chartRef":    chartRef,
		"valuesFile":  valuesFile,
		"releaseName": releaseName,
		"repoURL":     repoURL,
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
		// Otherwise, use the Helm pull action.
		// If repoURL is provided, we expect chartRef to be a bare chart name.
		// If repoURL is empty, the pull action will use local repo alias information.
		if repoURL != "" && strings.Contains(chartRef, "/") {
			return "", fmt.Errorf("cannot specify --repo together with an alias/prefix in chartRef (%q). Use either `--chart postgresql --repo <repo>` or `--chart bitnami/postgresql` alone", chartRef)
		}

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

		// Create the Helm SDK pull options
		pullOpts := action.WithConfig(actionConfig)

		// Create a new Pull client with the pull options
		pullClient := action.NewPullWithOpts(pullOpts)

		// IMPORTANT: set the Settings field so that the pull action can read the local repo index.
		pullClient.Settings = settings

		// Set destination via embedded ChartPathOptions.
		pullClient.DestDir = os.TempDir()
		pullClient.ChartPathOptions.RepoURL = repoURL
		pullClient.ChartPathOptions.Version = version

		// Set verification flag as false.
		pullClient.Verify = false

		// Use the Pull client to resolve (and pull) the chart.
		resolved, pullErr := pullClient.Run(chartRef)
		if pullErr != nil {
			logger.WithError(pullErr).Error("failed to pull chart using Helm pull action")
			return "", pullErr
		}
		chartRef = resolved
		logger.Infof("Chart pulled to: %s", chartRef)
	}

	// Load the chart from the resolved path.
	ch, err := loader.Load(chartRef)
	if err != nil {
		logger.WithError(err).Error("failed to load chart")
		return "", err
	}

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
				return "", err
			}
			// Reload the chart after dependency update.
			ch, err = loader.Load(chartRef)
			if err != nil {
				return "", err
			}
		}
	}

	// Read user-provided values, if any.
	userValues := map[string]interface{}{}
	if valuesFile != "" {
		data, err := os.ReadFile(valuesFile)
		if err != nil {
			logger.WithError(err).Error("failed to read values file")
			return "", err
		}
		if err := yaml.Unmarshal(data, &userValues); err != nil {
			logger.WithError(err).Error("failed to unmarshal values file")
			return "", err
		}
	}

	coalesced, err := chartutil.CoalesceValues(ch, userValues)
	if err != nil {
		logger.WithError(err).Error("failed to coalesce values")
		return "", fmt.Errorf("failed to coalesce values: %w", err)
	}

	renderVals, err := chartutil.ToRenderValues(ch, coalesced, chartutil.ReleaseOptions{
		Name:      releaseName,
		Namespace: namespace,
	}, nil)
	if err != nil {
		logger.WithError(err).Error("failed to prepare render values")
		return "", fmt.Errorf("failed to prepare render values: %w", err)
	}

	// Render the chart templates.
	renderedFiles, err := engine.Render(ch, renderVals)
	if err != nil {
		logger.WithError(err).Error("failed to render chart templates")
		return "", fmt.Errorf("failed to render chart templates: %w", err)
	}

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
