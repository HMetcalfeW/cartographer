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

		// Ensure at least one input is provided.
		if inputPath == "" && chartPath == "" {
			return fmt.Errorf("no input file or chart provided; please specify --input or --chart")
		}

		// variable storing the render Helm chart's k8s manifests
		var k8sManifests string

		// If an input file is provided, read it.
		if inputPath != "" {
			data, err := os.ReadFile(inputPath)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("error: Kubernetes manifest not found at '%s'. Please verify the file path and ensure it exists: %w", inputPath, err)
				}
				return fmt.Errorf("failed to read input file '%s': %w", inputPath, err)
			}
			k8sManifests = string(data)
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
		tmpFile, err := os.CreateTemp("", "analyze-rendered-*.yaml")
		if err != nil {
			logger.WithError(err).Error("failed to create temporary file")
			return err
		}

		defer func() {
			if err := os.Remove(tmpFile.Name()); err != nil {
				logger.WithError(err).Error("failed to remove tmp file")
			}
		}()

		if _, err = tmpFile.Write([]byte(k8sManifests)); err != nil {
			logger.WithError(err).Error("failed to write YAML content to temp file")
			return err
		}

		if err := tmpFile.Close(); err != nil {
			logger.WithError(err).Error("failed to close temp file")
			return err
		}

		// Parse the YAML content.
		objs, err := parser.ParseYAMLFile(tmpFile.Name())
		if err != nil {
			logger.WithError(err).Error("failed to parse YAML content in temp file")
			return err
		}
		log.Debugf("Parsed %d objects", len(objs))

		// Build the dependency map.
		deps := dependency.BuildDependencies(objs)

		if outputFormat == "dot" {
			dotContent := dependency.GenerateDOT(deps)
			if outputFile == "" {
				// Print to stdout
				fmt.Println(dotContent)
			} else {
				// Write to a file
				if err := os.WriteFile(outputFile, []byte(dotContent), 0644); err != nil {
					return fmt.Errorf("failed to write DOT output: %w", err)
				}
				log.Debugf("DOT file saved to %s", outputFile)
			}
		} else {
			// Default: just print dependencies in text form
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
	AnalyzeCmd.Flags().StringP("chart", "c", "", "Chart reference or local path to a Helm chart (e.g. example/chart)")
	AnalyzeCmd.Flags().StringP("values", "v", "", "Path to a values file for the Helm chart")
	AnalyzeCmd.Flags().StringP("release", "l", "cartographer-release", "Release name for the Helm chart")
	AnalyzeCmd.Flags().String("version", "", "Chart version to pull (optional if remote charts specify a version)")
	AnalyzeCmd.Flags().String("namespace", "", "Namespace to inject into the Helm rendered release")
	AnalyzeCmd.Flags().String("output-format", "dot", "Output format (e.g. 'dot' - also the default). If empty, prints text dependencies.")
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
