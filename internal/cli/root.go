package cli

import (
	"fmt"
	"os"

	"github.com/fluidstack/go-anta/internal/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile  string
	logLevel string
	logFile  string
	verbose  bool
)

var rootCmd = &cobra.Command{
	Use:   "go-anta",
	Short: "GANTA - Golang Network Test Automation Framework",
	Long: `GANTA (Golang ANTA) is a network testing framework inspired by the Python ANTA project.
It provides automated testing capabilities for network devices, particularly Arista EOS devices,
with support for concurrent test execution, flexible inventory management, and comprehensive reporting.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is $HOME/.go-anta.yaml)")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&logFile, "log-file", "", "log file path")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	if err := viper.BindPFlag("log.level", rootCmd.PersistentFlags().Lookup("log-level")); err != nil {
		fmt.Fprintf(os.Stderr, "Error binding flag: %v\n", err)
	}
	if err := viper.BindPFlag("log.file", rootCmd.PersistentFlags().Lookup("log-file")); err != nil {
		fmt.Fprintf(os.Stderr, "Error binding flag: %v\n", err)
	}
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting home directory: %v\n", err)
			os.Exit(1)
		}

		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".go-anta")
	}

	viper.SetEnvPrefix("GO_ANTA")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		if verbose {
			fmt.Println("Using config file:", viper.ConfigFileUsed())
		}
	}
	
	// Configure logging
	if verbose {
		logger.SetVerbose(true)
	} else if logLevel != "" {
		logger.SetLevel(logLevel)
	} else {
		// Set level from viper config
		if level := viper.GetString("log.level"); level != "" {
			logger.SetLevel(level)
		}
	}
	
	// Set log output file if specified
	if logFile != "" {
		if err := logger.SetOutput(logFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error setting log output: %v\n", err)
		}
	} else if logFileFromConfig := viper.GetString("log.file"); logFileFromConfig != "" {
		if err := logger.SetOutput(logFileFromConfig); err != nil {
			fmt.Fprintf(os.Stderr, "Error setting log output: %v\n", err)
		}
	}
}