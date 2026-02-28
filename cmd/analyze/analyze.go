package cmd

import (
	"context"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/HMetcalfeW/cartographer/pkg/cluster"
	"github.com/HMetcalfeW/cartographer/pkg/dependency"
	"github.com/HMetcalfeW/cartographer/pkg/filter"
	"github.com/HMetcalfeW/cartographer/pkg/helm"
	"github.com/HMetcalfeW/cartographer/pkg/parser"
)

const DefaultNamespace = "default"

// AnalyzeCmd represents the analyze subcommand.
var AnalyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze Kubernetes manifests and generate a dependency graph",
	RunE: func(cmd *cobra.Command, args []string) error {
		inputPath, _ := cmd.Flags().GetString("input")
		chartPath, _ := cmd.Flags().GetString("chart")
		clusterMode, _ := cmd.Flags().GetBool("cluster")
		allNamespaces, _ := cmd.Flags().GetBool("all-namespaces")
		valuesFile, _ := cmd.Flags().GetString("values")
		version, _ := cmd.Flags().GetString("version")
		namespace, _ := cmd.Flags().GetString("namespace")
		releaseName, _ := cmd.Flags().GetString("release")
		outputFormat, _ := cmd.Flags().GetString("output-format")
		outputFile, _ := cmd.Flags().GetString("output-file")

		// Validate mutual exclusivity of input sources.
		sources := 0
		if inputPath != "" {
			sources++
		}
		if chartPath != "" {
			sources++
		}
		if clusterMode {
			sources++
		}
		if sources == 0 {
			return fmt.Errorf("no input source provided; specify --input, --chart, or --cluster")
		}
		if sources > 1 {
			return fmt.Errorf("--input, --chart, and --cluster are mutually exclusive")
		}

		// -A only valid with --cluster.
		if allNamespaces && !clusterMode {
			return fmt.Errorf("--all-namespaces can only be used with --cluster")
		}

		if namespace == "" {
			namespace = DefaultNamespace
		}

		// Determine input source label for logging.
		source := "file"
		if chartPath != "" {
			source = "chart"
		} else if clusterMode {
			source = "cluster"
		}
		logger := log.WithFields(log.Fields{
			"func":      "analyze",
			"source":    source,
			"namespace": namespace,
		})
		logger.Info("Starting analysis")

		var objs []*unstructured.Unstructured

		switch {
		case clusterMode:
			kubeconfigPath := viper.GetString("cluster.kubeconfig")
			contextName := viper.GetString("cluster.context")

			client, err := cluster.NewClient(kubeconfigPath, contextName)
			if err != nil {
				return fmt.Errorf("failed to create cluster client: %w", err)
			}

			objs, err = cluster.FetchResources(context.Background(), client, namespace, allNamespaces)
			if err != nil {
				return fmt.Errorf("failed to fetch cluster resources: %w", err)
			}

		default:
			k8sManifests, err := loadManifests(inputPath, chartPath, valuesFile, releaseName, version, namespace)
			if err != nil {
				return err
			}

			objs, err = parser.ParseYAML(k8sManifests)
			if err != nil {
				return fmt.Errorf("failed to parse YAML content: %w", err)
			}
		}

		logger.WithField("count", len(objs)).Info("Loaded resources")

		// Apply config-driven exclusion filters.
		beforeFilter := len(objs)
		objs = filter.Apply(objs, viper.GetStringSlice("exclude.kinds"), viper.GetStringSlice("exclude.names"))
		if excluded := beforeFilter - len(objs); excluded > 0 {
			logger.WithFields(log.Fields{
				"before":   beforeFilter,
				"after":    len(objs),
				"excluded": excluded,
			}).Info("Applied exclusion filters")
		}

		deps := dependency.BuildDependencies(objs)
		logger.WithField("nodes", len(deps)).Info("Built dependency graph")

		return writeOutput(cmd, deps, outputFormat, outputFile)
	},
}

// loadManifests reads YAML from a file or renders a Helm chart.
func loadManifests(inputPath, chartPath, valuesFile, releaseName, version, namespace string) ([]byte, error) {
	if inputPath != "" {
		log.WithFields(log.Fields{
			"func": "loadManifests",
			"path": inputPath,
		}).Debug("Reading YAML file")
		data, err := os.ReadFile(inputPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read input file: %w", err)
		}
		return data, nil
	}

	log.WithFields(log.Fields{
		"func":  "loadManifests",
		"chart": chartPath,
	}).Debug("Rendering Helm chart")
	rendered, err := helm.RenderChart(chartPath, valuesFile, releaseName, version, namespace)
	if err != nil {
		return nil, err
	}
	return []byte(rendered), nil
}

// writeOutput dispatches to the appropriate output format handler.
func writeOutput(cmd *cobra.Command, deps map[string][]dependency.Edge, format, outputFile string) error {
	log.WithFields(log.Fields{
		"func":   "writeOutput",
		"format": format,
	}).Debug("Generating output")

	switch format {
	case "dot":
		return writeTextOutput(cmd, dependency.GenerateDOT(deps), outputFile, "DOT")
	case "mermaid":
		return writeTextOutput(cmd, dependency.GenerateMermaid(deps), outputFile, "Mermaid")
	case "json":
		return writeTextOutput(cmd, dependency.GenerateJSON(deps), outputFile, "JSON")
	case "png", "svg":
		if outputFile == "" {
			return fmt.Errorf("--output-file is required for %s format (binary data cannot be printed to stdout)", format)
		}
		imageData, err := dependency.RenderImage(deps, format)
		if err != nil {
			return fmt.Errorf("failed to render %s: %w", format, err)
		}
		if err := os.WriteFile(outputFile, imageData, 0644); err != nil {
			return fmt.Errorf("failed to write %s output: %w", format, err)
		}
		log.WithFields(log.Fields{
			"func":   "writeOutput",
			"format": format,
			"path":   outputFile,
			"bytes":  len(imageData),
		}).Debug("Image file saved")
		return nil
	default:
		return fmt.Errorf("unknown output format: %s", format)
	}
}

// writeTextOutput writes text content to stdout or a file.
func writeTextOutput(cmd *cobra.Command, content, outputFile, label string) error {
	if outputFile == "" {
		log.WithField("func", "writeOutput").Debug("Writing to stdout")
		_, err := fmt.Fprintln(cmd.OutOrStdout(), content)
		return err
	}
	if err := os.WriteFile(outputFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write %s output: %w", label, err)
	}
	log.WithFields(log.Fields{
		"func": "writeOutput",
		"path": outputFile,
		"type": label,
	}).Debug("File saved")
	return nil
}

func init() {
	AnalyzeCmd.Flags().StringP("input", "i", "", "Path to Kubernetes YAML file")
	AnalyzeCmd.Flags().StringP("chart", "c", "", "Chart reference or local path to a Helm chart (e.g. bitnami/postgres)")
	AnalyzeCmd.Flags().Bool("cluster", false, "Analyze resources from a live Kubernetes cluster")
	AnalyzeCmd.Flags().BoolP("all-namespaces", "A", false, "Fetch resources from all namespaces (requires --cluster)")
	AnalyzeCmd.Flags().StringP("values", "v", "", "Path to a values file for the Helm chart")
	AnalyzeCmd.Flags().StringP("release", "l", "cartographer-release", "Release name for the Helm chart")
	AnalyzeCmd.Flags().String("version", "", "Chart version to pull (optional if remote charts specify a version)")
	AnalyzeCmd.Flags().String("namespace", "", "Namespace to inject into the Helm rendered release or cluster scope")
	AnalyzeCmd.Flags().String("output-format", "dot", "Output format: dot, mermaid, json, png, svg (default: dot)")
	AnalyzeCmd.Flags().String("output-file", "", "Output file path (required for png/svg formats)")
}
