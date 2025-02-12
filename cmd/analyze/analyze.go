package cmd

import (
	"fmt"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

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

		inputPath := viper.GetString("input")
		chartPath := viper.GetString("chart")
		repoURL := viper.GetString("repo")

		// Ensure at least one input is provided.
		if inputPath == "" && chartPath == "" {
			return fmt.Errorf("no input file or chart provided; please specify --input or --chart")
		}

		// Check user provided input first
		if inputPath != "" {

			// Process YAML file input if provided.
			absPath, err := filepath.Abs(inputPath)
			if err != nil {
				return fmt.Errorf("failed to get absolute path: %w", err)
			}
			objs, err := parser.ParseYAMLFile(absPath)
			if err != nil {
				return fmt.Errorf("failed to parse YAML file: %w", err)
			}

			logger.Infof("Parsed %d objects from input file\n", len(objs))

		} else {
			// Placeholder for Helm chart processing.
			if chartPath != "" {
				logger.Debug("Rendering Helm chart from path")
				if repoURL != "" {
					fmt.Printf("Using Helm repository: %s\n", repoURL)
				}
				// TODO: Render and parse the Helm chart using the Helm SDK.
			}

		}

		return nil
	},
}

func init() {
	const funcName = "analyze.init"
	log.WithField("func", funcName).Info("initializing cartographer subcommand analyze")

	// Define flags for the analyze command.
	AnalyzeCmd.Flags().StringP("input", "i", "", "Path to Kubernetes YAML file")
	AnalyzeCmd.Flags().StringP("chart", "c", "", "Chart reference or local path to a Helm chart (e.g. bitnami/postgres)")
	AnalyzeCmd.Flags().StringP("values", "v", "", "Path to a values file for the Helm chart")
	AnalyzeCmd.Flags().StringP("repo", "r", "", "Helm chart repository URL (optional)")

	// Bind flags with Viper.
	if err := viper.BindPFlag("input", AnalyzeCmd.Flags().Lookup("input")); err != nil {
		log.WithError(err).Fatal("failed to bind the flag `input`")
	}

	if err := viper.BindPFlag("chart", AnalyzeCmd.Flags().Lookup("chart")); err != nil {
		log.WithError(err).Fatal("failed to bind the flag `chart`")
	}

	if err := viper.BindPFlag("repo", AnalyzeCmd.Flags().Lookup("repo")); err != nil {
		log.WithError(err).Fatal("failed to bind the flag `repo`")
	}
}
