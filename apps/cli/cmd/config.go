package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View and update configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE:  runConfigShow,
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration files",
	RunE:  runConfigValidate,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE:  runConfigSet,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configValidateCmd)
	configCmd.AddCommand(configSetCmd)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	fmt.Println("ContextOps Configuration")
	fmt.Println(strings.Repeat("=", 40))

	settings := viper.AllSettings()
	printMap(settings, "")

	if len(settings) == 0 {
		fmt.Println("  (no configuration loaded)")
		fmt.Println()
		fmt.Println("Run 'contextops init' to create a config file")
	}

	return nil
}

func printMap(m map[string]interface{}, prefix string) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case map[string]interface{}:
			printMap(val, key)
		default:
			fmt.Printf("  %-30s %v\n", key, val)
		}
	}
}

func runConfigValidate(cmd *cobra.Command, args []string) error {
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		fmt.Println("No config file loaded. Run 'contextops init' first.")
		return nil
	}

	fmt.Printf("Config file: %s\n", configFile)

	// Basic validation
	issues := []string{}

	apiURL := viper.GetString("api.url")
	if apiURL == "" {
		issues = append(issues, "api.url is not set")
	}

	if len(issues) > 0 {
		fmt.Println("\nWarnings:")
		for _, issue := range issues {
			fmt.Printf("  - %s\n", issue)
		}
	} else {
		fmt.Println("Configuration is valid.")
	}

	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	viper.Set(key, value)

	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		configFile = ".contextops/config.yaml"
	}

	if err := viper.WriteConfigAs(configFile); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	fmt.Printf("Set %s = %s\n", key, value)
	return nil
}
