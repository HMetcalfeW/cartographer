package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	analyze "github.com/HMetcalfeW/cartographer/cmd/analyze"
)

var cfgFile string

// rootCmd represents the base command.
var RootCmd = &cobra.Command{
	Use:   "cartographer",
	Short: "Cartographer maps your Helm Chart Kubernetes resources",
	Run: func(cmd *cobra.Command, args []string) {
		if err := cmd.Help(); err != nil {
			log.WithError(err).Fatal()
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		log.WithField("func", "root.Execute").WithError(err).Fatal()
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flag for config file.
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cartographer.yaml)")

	// Configure logrus to use a text formatter with full timestamps.
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
	// Set the log level to info (you can adjust this as needed).
	log.SetLevel(log.InfoLevel)

	// Register the analyze subcommand explicitly.
	RootCmd.AddCommand(analyze.AnalyzeCmd)

	log.WithField("func", "root.init").Debug("root initialization complete")
}

func initConfig() {
	logger := log.WithField("func", "initConfig")
	logger.Debug("Initializing root config")

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath("$HOME")
		viper.SetConfigName(".cartographer")
	}
	viper.AutomaticEnv() // read in environment variables that match

	if err := viper.ReadInConfig(); err == nil {
		logger.Info("Using config file:", viper.ConfigFileUsed())
	}
}
