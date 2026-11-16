package cmd

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/Mario-F/cloudflare-dyndns/internal/logger"
	"github.com/Mario-F/cloudflare-dyndns/internal/network"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const tokenEnv = "CLOUDFLARE_API_TOKEN"

var (
	cfgFile   string
	token     string
	ipVersion string
)

var Version = "development"

var IPVersions = []string{
	string(network.IPv4),
	string(network.IPv6),
}

var rootCmd = &cobra.Command{
	Use:           "cloudflare-dyndns",
	Short:         "Updates Cloudflare DNS with external IP.",
	Long:          `A tool for updating Cloudflare DNS records with your external IP.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if !slices.Contains(IPVersions, ipVersion) {
			return fmt.Errorf("--version must be one of: %s", strings.Join(IPVersions, ", "))
		}
		return nil
	},
}

// Execute runs the root command and exits non-zero on error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initLogging, initConfig)

	rootCmd.PersistentFlags().BoolVar(&logger.DebugMode, "debug", false, "Turn on debug messages")
	rootCmd.PersistentFlags().BoolVar(&logger.QuietMode, "quiet", false, "Suppress logmessages")
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cloudflare-dyndns.yaml)")
	rootCmd.PersistentFlags().StringVar(&ipVersion, "version", string(network.IPv4), fmt.Sprintf("Which ip version to check.\nAllowed values: %s", strings.Join(IPVersions, ", ")))
}

func initLogging() {
	if logger.DebugMode {
		log.Info("Debug loglevel enabled")
		log.SetLevel(log.DebugLevel)
	}
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".cloudflare-dyndns")
	}

	if err := viper.ReadInConfig(); err == nil {
		logger.Infof("Using config file: %s", viper.ConfigFileUsed())
	} else if cfgFile != "" {
		fmt.Fprintln(os.Stderr, "Error: reading config file:", err)
		os.Exit(1)
	}
}

// apiToken returns the token from --token, CLOUDFLARE_API_TOKEN or the config file, in that order.
func apiToken() (string, error) {
	for _, t := range []string{token, os.Getenv(tokenEnv), viper.GetString("token")} {
		if t = strings.TrimSpace(t); t != "" {
			return t, nil
		}
	}
	return "", errors.New("no API token given, use " + tokenEnv + ", --token or token in the config file")
}

func recordType() string {
	if ipVersion == string(network.IPv6) {
		return "AAAA"
	}
	return "A"
}
