package cmd

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/HMetcalfeW/cartographer/pkg/dependency"
	"github.com/HMetcalfeW/cartographer/pkg/helm"
	"github.com/HMetcalfeW/cartographer/pkg/parser"
)

const (
	DEFAULT_NAMESPACE = "default"
)

// AnalyzeCmd represents the analyze subcommand.
var AnalyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze Kubernetes manifests and generate a dependency graph",
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := log.WithFields(log.Fields{
			"func": "analyze",
			"args": args,
		})

		// Retrieve flags from the command (not Viper globals) so that
		// defaults are respected on each invocation.
		inputPath, _ := cmd.Flags().GetString("input")
		chartPath, _ := cmd.Flags().GetString("chart")
		valuesFile, _ := cmd.Flags().GetString("values")
		version, _ := cmd.Flags().GetString("version")
		namespace, _ := cmd.Flags().GetString("namespace")
		releaseName, _ := cmd.Flags().GetString("release")
		outputFormat, _ := cmd.Flags().GetString("output-format")
		outputFile, _ := cmd.Flags().GetString("output-file")

		// Ensure at least one input is provided.
		if inputPath == "" && chartPath == "" {
			return fmt.Errorf("no input file or chart provided; please specify --input or --chart")
		}

		// variable storing the raw YAML manifest bytes
		var k8sManifests []byte

		if namespace == "" {
			namespace = DEFAULT_NAMESPACE
		}

		// If an input file is provided, read it.
		if inputPath != "" {
			data, err := os.ReadFile(inputPath)
			if err != nil {
				return fmt.Errorf("failed to read input file: %w", err)
			}
			k8sManifests = data
		}

		// If a chart reference is provided, render it using the Helm SDK.
		if chartPath != "" {
			logger = logger.WithFields(log.Fields{
				"chart":       chartPath,
				"values":      valuesFile,
				"releaseName": releaseName,
				"version":     version,
				"namespace":   namespace,
			})
			logger.Debug("Rendering Helm chart")
			rendered, err := helm.RenderChart(chartPath, valuesFile,
				releaseName, version, namespace)
			if err != nil {
				logger.WithError(err).Error("failed to render chart")
				return err
			}
			k8sManifests = []byte(rendered)
		}

		// Parse the YAML content directly from memory.
		objs, err := parser.ParseYAML(k8sManifests)
		if err != nil {
			logger.WithError(err).Error("failed to parse YAML content")
			return err
		}
		log.Debugf("Parsed %d objects", len(objs))

		// Build the dependency map.
		deps := dependency.BuildDependencies(objs)

		switch outputFormat {
		case "dot":
			dotContent := dependency.GenerateDOT(deps)
			if outputFile == "" {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), dotContent); err != nil {
					return fmt.Errorf("failed to write DOT to stdout: %w", err)
				}
			} else {
				if err := os.WriteFile(outputFile, []byte(dotContent), 0644); err != nil {
					return fmt.Errorf("failed to write DOT output: %w", err)
				}
				log.Debugf("DOT file saved to %s", outputFile)
			}

		case "mermaid":
			mermaidContent := dependency.GenerateMermaid(deps)
			if outputFile == "" {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), mermaidContent); err != nil {
					return fmt.Errorf("failed to write Mermaid to stdout: %w", err)
				}
			} else {
				if err := os.WriteFile(outputFile, []byte(mermaidContent), 0644); err != nil {
					return fmt.Errorf("failed to write Mermaid output: %w", err)
				}
				log.Debugf("Mermaid file saved to %s", outputFile)
			}

		case "json":
			jsonContent := dependency.GenerateJSON(deps)
			if outputFile == "" {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), jsonContent); err != nil {
					return fmt.Errorf("failed to write JSON to stdout: %w", err)
				}
			} else {
				if err := os.WriteFile(outputFile, []byte(jsonContent), 0644); err != nil {
					return fmt.Errorf("failed to write JSON output: %w", err)
				}
				log.Debugf("JSON file saved to %s", outputFile)
			}

		case "png", "svg":
			if outputFile == "" {
				return fmt.Errorf("--output-file is required for %s format (binary data cannot be printed to stdout)", outputFormat)
			}
			imageData, err := dependency.RenderImage(deps, outputFormat)
			if err != nil {
				return fmt.Errorf("failed to render %s: %w", outputFormat, err)
			}
			if err := os.WriteFile(outputFile, imageData, 0644); err != nil {
				return fmt.Errorf("failed to write %s output: %w", outputFormat, err)
			}
			log.Debugf("%s file saved to %s", outputFormat, outputFile)

		default:
			log.Debug("Dependencies:")
			dependency.PrintDependencies(deps)
		}

		return nil
	},
}

func init() {
	log.WithField("func", "analyze.init").Debug("initializing cartographer subcommand analyze")

	// Define flags for the analyze command.
	AnalyzeCmd.Flags().StringP("input", "i", "", "Path to Kubernetes YAML file")
	AnalyzeCmd.Flags().StringP("chart", "c", "", "Chart reference or local path to a Helm chart (e.g. bitnami/postgres)")
	AnalyzeCmd.Flags().StringP("values", "v", "", "Path to a values file for the Helm chart")
	AnalyzeCmd.Flags().StringP("release", "l", "cartographer-release", "Release name for the Helm chart")
	AnalyzeCmd.Flags().String("version", "", "Chart version to pull (optional if remote charts specify a version)")
	AnalyzeCmd.Flags().String("namespace", "", "Namespace to inject into the Helm rendered release")
	AnalyzeCmd.Flags().String("output-format", "dot", "Output format: dot, mermaid, json, png, svg (default: dot)")
	AnalyzeCmd.Flags().String("output-file", "", "Output file path (required for png/svg formats)")

	// Bind flags with Viper.
	flags := []string{"input", "chart", "values", "release", "version", "namespace", "output-format", "output-file"}
	for _, name := range flags {
		if err := viper.BindPFlag(name, AnalyzeCmd.Flags().Lookup(name)); err != nil {
			log.WithError(err).Fatalf("failed to bind the flag `%s`", name)
		}
	}
}
