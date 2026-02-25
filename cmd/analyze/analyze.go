package cmd

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/HMetcalfeW/cartographer/pkg/dependency"
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
		valuesFile, _ := cmd.Flags().GetString("values")
		version, _ := cmd.Flags().GetString("version")
		namespace, _ := cmd.Flags().GetString("namespace")
		releaseName, _ := cmd.Flags().GetString("release")
		outputFormat, _ := cmd.Flags().GetString("output-format")
		outputFile, _ := cmd.Flags().GetString("output-file")

		if inputPath == "" && chartPath == "" {
			return fmt.Errorf("no input file or chart provided; please specify --input or --chart")
		}

		if namespace == "" {
			namespace = DefaultNamespace
		}

		k8sManifests, err := loadManifests(inputPath, chartPath, valuesFile, releaseName, version, namespace)
		if err != nil {
			return err
		}

		objs, err := parser.ParseYAML(k8sManifests)
		if err != nil {
			return fmt.Errorf("failed to parse YAML content: %w", err)
		}

		deps := dependency.BuildDependencies(objs)

		return writeOutput(cmd, deps, outputFormat, outputFile)
	},
}

// loadManifests reads YAML from a file or renders a Helm chart.
func loadManifests(inputPath, chartPath, valuesFile, releaseName, version, namespace string) ([]byte, error) {
	if inputPath != "" {
		data, err := os.ReadFile(inputPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read input file: %w", err)
		}
		return data, nil
	}

	rendered, err := helm.RenderChart(chartPath, valuesFile, releaseName, version, namespace)
	if err != nil {
		return nil, err
	}
	return []byte(rendered), nil
}

// writeOutput dispatches to the appropriate output format handler.
func writeOutput(cmd *cobra.Command, deps map[string][]dependency.Edge, format, outputFile string) error {
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
		return nil
	default:
		return fmt.Errorf("unknown output format: %s", format)
	}
}

// writeTextOutput writes text content to stdout or a file.
func writeTextOutput(cmd *cobra.Command, content, outputFile, label string) error {
	if outputFile == "" {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), content)
		return err
	}
	if err := os.WriteFile(outputFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write %s output: %w", label, err)
	}
	log.Debugf("%s file saved to %s", label, outputFile)
	return nil
}

func init() {
	AnalyzeCmd.Flags().StringP("input", "i", "", "Path to Kubernetes YAML file")
	AnalyzeCmd.Flags().StringP("chart", "c", "", "Chart reference or local path to a Helm chart (e.g. bitnami/postgres)")
	AnalyzeCmd.Flags().StringP("values", "v", "", "Path to a values file for the Helm chart")
	AnalyzeCmd.Flags().StringP("release", "l", "cartographer-release", "Release name for the Helm chart")
	AnalyzeCmd.Flags().String("version", "", "Chart version to pull (optional if remote charts specify a version)")
	AnalyzeCmd.Flags().String("namespace", "", "Namespace to inject into the Helm rendered release")
	AnalyzeCmd.Flags().String("output-format", "dot", "Output format: dot, mermaid, json, png, svg (default: dot)")
	AnalyzeCmd.Flags().String("output-file", "", "Output file path (required for png/svg formats)")
}
