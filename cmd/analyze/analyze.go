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

		// Retrieve flags.
		inputPath := viper.GetString("input")
		chartPath := viper.GetString("chart")
		valuesFile := viper.GetString("values")
		version := viper.GetString("version")
		namespace := viper.GetString("namespace")
		releaseName := viper.GetString("release")
		outputFormat := viper.GetString("output-format")
		outputFile := viper.GetString("output-file")

		// Ensure only one input method is provided.
		if inputPath != "" && chartPath != "" {
			return fmt.Errorf("error: Cannot use both --input and --chart flags simultaneously. Please choose one input method.")
		}

		// Ensure at least one input is provided.
		if inputPath == "" && chartPath == "" {
			return fmt.Errorf("error: No input file or chart provided. Please specify either --input or --chart.")
		}

		// variable storing the render Helm chart's k8s manifests
		var k8sManifests string

		// If an input file is provided, read it.
		if inputPath != "" {
			logger.WithField("inputPath", inputPath).Debug("Reading input file")
			data, err := os.ReadFile(inputPath)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("error: Kubernetes manifest not found at '%s'. Please verify the file path and ensure it exists: %w", inputPath, err)
				}
				return fmt.Errorf("failed to read input file '%s': %w", inputPath, err)
			}
			k8sManifests = string(data)
			logger.WithField("inputPath", inputPath).Info("Successfully read input file")
		}

		if namespace == "" {
			namespace = DEFAULT_NAMESPACE
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
			k8sManifests = rendered
		}

		// Write the YAML content to a temporary file for parsing.
		logger.Debug("Creating temporary file for YAML content")
		tmpFile, err := os.CreateTemp("", "analyze-rendered-*.yaml")
		if err != nil {
			logger.WithError(err).Error("failed to create temporary file")
			return err
		}

		defer func() {
			if err := os.Remove(tmpFile.Name()); err != nil {
				logger.WithError(err).Warn("failed to remove temporary file")
			}
		}()

		logger.WithField("tmpFile", tmpFile.Name()).Debug("Writing YAML content to temporary file")
		if _, err = tmpFile.Write([]byte(k8sManifests)); err != nil {
			logger.WithError(err).Error("failed to write YAML content to temp file")
			return err
		}

		logger.WithField("tmpFile", tmpFile.Name()).Debug("Closing temporary file")
		if err := tmpFile.Close(); err != nil {
			logger.WithError(err).Error("failed to close temp file")
			return err
		}
		logger.WithField("tmpFile", tmpFile.Name()).Info("Successfully wrote YAML content to temporary file")

		// Parse the YAML content.
		logger.WithField("tmpFile", tmpFile.Name()).Debug("Parsing YAML content from temporary file")
		objs, err := parser.ParseYAMLFile(tmpFile.Name())
		if err != nil {
			logger.WithError(err).Error("failed to parse YAML content in temp file")
			return err
		}
		logger.Debugf("Parsed %d objects", len(objs))

		// Build the dependency map.
		logger.Debug("Building dependency map")
		deps := dependency.BuildDependencies(objs)
		logger.WithField("dependencies_count", len(deps)).Info("Successfully built dependency map")

		if outputFormat == "dot" {
			logger.Debug("Generating DOT content")
			dotContent := dependency.GenerateDOT(deps)
			if outputFile == "" {
				// Print to stdout
				logger.Info("Printing DOT content to stdout")
				fmt.Println(dotContent)
			} else {
				// Write to a file
				logger.WithField("outputFile", outputFile).Info("Writing DOT content to file")
				if err := os.WriteFile(outputFile, []byte(dotContent), 0644); err != nil {
										return fmt.Errorf("failed to write DOT output to '%s': %w", outputFile, err)
				}
				logger.WithField("outputFile", outputFile).Info("Successfully wrote DOT content to file")
			}
		} else if outputFormat == "mermaid" {
			logger.Debug("Generating Mermaid content")
			mermaidContent := dependency.GenerateMermaid(deps)
			if outputFile == "" {
				logger.Info("Printing Mermaid content to stdout")
				fmt.Println(mermaidContent)
			} else {
				logger.WithField("outputFile", outputFile).Info("Writing Mermaid content to file")
				if err := os.WriteFile(outputFile, []byte(mermaidContent), 0644); err != nil {
					return fmt.Errorf("failed to write Mermaid output to '%s': %w", outputFile, err)
				}
				logger.WithField("outputFile", outputFile).Info("Successfully wrote Mermaid content to file")
			}
		} else if outputFormat == "json" {
			logger.Debug("Generating JSON content")
			jsonContent, err := dependency.GenerateJSON(deps)
			if err != nil {
				return fmt.Errorf("failed to generate JSON output: %w", err)
			}
			if outputFile == "" {
				logger.Info("Printing JSON content to stdout")
				fmt.Println(jsonContent)
			} else {
				logger.WithField("outputFile", outputFile).Info("Writing JSON content to file")
				if err := os.WriteFile(outputFile, []byte(jsonContent), 0644); err != nil {
					return fmt.Errorf("failed to write JSON output to '%s': %w", outputFile, err)
				}
				logger.WithField("outputFile", outputFile).Info("Successfully wrote JSON content to file")
			}
		} else {
			return fmt.Errorf("error: Unsupported output format '%s'. Supported formats are 'dot', 'mermaid', and 'json'.", outputFormat)
		}

		return nil
	},
}

func init() {
	log.WithField("func", "analyze.init").Debug("initializing cartographer subcommand analyze")

	// Define flags for the analyze command.
	AnalyzeCmd.Flags().StringP("input", "i", "", "Path to Kubernetes YAML file")
	AnalyzeCmd.Flags().StringP("chart", "c", "", "Chart reference or local path to a Helm chart (e.g. example/chart)")
	AnalyzeCmd.Flags().StringP("values", "v", "", "Path to a values file for the Helm chart")
	AnalyzeCmd.Flags().StringP("release", "l", "cartographer-release", "Release name for the Helm chart")
	AnalyzeCmd.Flags().String("version", "", "Chart version to pull (optional if remote charts specify a version)")
	AnalyzeCmd.Flags().String("namespace", "", "Namespace to inject into the Helm rendered release")
	AnalyzeCmd.Flags().StringP("output-format", "o", "dot", "Output format (dot, mermaid, json). Defaults to 'dot'.")
	AnalyzeCmd.Flags().String("output-file", "", "Output file for the DOT data (if --output-format=dot). Prints to stdout by default.")

	// Bind flags with Viper.
	if err := viper.BindPFlag("input", AnalyzeCmd.Flags().Lookup("input")); err != nil {
		log.WithError(err).Fatal("failed to bind the flag `input`")
	}

	if err := viper.BindPFlag("chart", AnalyzeCmd.Flags().Lookup("chart")); err != nil {
		log.WithError(err).Fatal("failed to bind the flag `chart`")
	}

	if err := viper.BindPFlag("values", AnalyzeCmd.Flags().Lookup("values")); err != nil {
		log.WithError(err).Fatal("failed to bind the flag `values`")
	}

	if err := viper.BindPFlag("release", AnalyzeCmd.Flags().Lookup("release")); err != nil {
		log.WithError(err).Fatal("failed to bind the flag `release`")
	}

	if err := viper.BindPFlag("version", AnalyzeCmd.Flags().Lookup("version")); err != nil {
		log.WithError(err).Fatal("failed to bind the flag `version`")
	}

	if err := viper.BindPFlag("namespace", AnalyzeCmd.Flags().Lookup("namespace")); err != nil {
		log.WithError(err).Fatal("failed to bind the flag `namespace`")
	}

	if err := viper.BindPFlag("output-format", AnalyzeCmd.Flags().Lookup("output-format")); err != nil {
		log.WithError(err).Fatal("failed to bind the flag `output-format`")
	}

	if err := viper.BindPFlag("output-file", AnalyzeCmd.Flags().Lookup("output-file")); err != nil {
		log.WithError(err).Fatal("failed to bind the flag `output-file`")
	}
}
