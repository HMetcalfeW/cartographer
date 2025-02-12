package cmd

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/HMetcalfeW/cartographer/pkg/helm"
	"github.com/HMetcalfeW/cartographer/pkg/parser"
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
		repoURL := viper.GetString("repo")
		releaseName := viper.GetString("release")

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
				return fmt.Errorf("failed to read input file: %w", err)
			}
			k8sManifests = string(data)
		}

		// If a chart reference is provided, render it using the Helm SDK.
		if chartPath != "" {
			logger = logger.WithFields(log.Fields{
				"chart":       chartPath,
				"values":      valuesFile,
				"repo":        repoURL,
				"releaseName": releaseName,
			})
			logger.Info("Rendering Helm chart")
			rendered, err := helm.RenderChart(chartPath, valuesFile, releaseName, repoURL)
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
		log.Infof("Parsed %d objects", len(objs))
		return nil
	},
}

func init() {
	log.WithField("func", "analyze.init").Info("initializing cartographer subcommand analyze")

	// Define flags for the analyze command.
	AnalyzeCmd.Flags().StringP("input", "i", "", "Path to Kubernetes YAML file")
	AnalyzeCmd.Flags().StringP("chart", "c", "", "Chart reference or local path to a Helm chart (e.g. bitnami/postgres)")
	AnalyzeCmd.Flags().StringP("values", "v", "", "Path to a values file for the Helm chart")
	AnalyzeCmd.Flags().StringP("repo", "r", "", "Helm chart repository URL (optional)")
	AnalyzeCmd.Flags().StringP("release", "l", "cartographer-release", "Release name for the Helm chart")

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

	if err := viper.BindPFlag("repo", AnalyzeCmd.Flags().Lookup("repo")); err != nil {
		log.WithError(err).Fatal("failed to bind the flag `repo`")
	}

	if err := viper.BindPFlag("release", AnalyzeCmd.Flags().Lookup("release")); err != nil {
		log.WithError(err).Fatal("failed to bind the flag `release`")
	}
}
