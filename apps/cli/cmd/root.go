package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "contextops",
	Short: "Evaluation, debugging, and regression testing for AI agents",
	Long: `ContextOps is an open-source platform for evaluating, debugging,
and regression-testing AI agents, RAG systems, and enterprise copilots.

It inspects the full AI execution path: retrieval, memory, permissions,
tool calls, grounding, correctness, cost, and latency.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default .contextops/config.yaml)")
	rootCmd.PersistentFlags().String("api-url", "http://localhost:8080", "ContextOps API URL")
	rootCmd.PersistentFlags().String("tenant", "", "Tenant ID")
	viper.BindPFlag("api.url", rootCmd.PersistentFlags().Lookup("api-url"))
	viper.BindPFlag("tenant.default", rootCmd.PersistentFlags().Lookup("tenant"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".contextops")
		viper.AddConfigPath("$HOME/.contextops")
	}
	viper.SetEnvPrefix("CONTEXTOPS")
	viper.AutomaticEnv()
	viper.ReadInConfig()
}

func apiURL() string {
	u := viper.GetString("api.url")
	if u == "" {
		return "http://localhost:8080"
	}
	return u
}

func tenantID() string {
	t := viper.GetString("tenant.default")
	if t == "" {
		return "00000000-0000-0000-0000-000000000001"
	}
	return t
}
