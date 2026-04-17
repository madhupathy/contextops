package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run local ContextOps services",
}

var serverStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start local ContextOps services via Docker Compose",
	RunE:  runServerStart,
}

var serverStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop local ContextOps services",
	RunE:  runServerStop,
}

var serverStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of local ContextOps services",
	RunE:  runServerStatus,
}

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.AddCommand(serverStartCmd)
	serverCmd.AddCommand(serverStopCmd)
	serverCmd.AddCommand(serverStatusCmd)

	serverStartCmd.Flags().Int("port", 8080, "API port")
	serverStartCmd.Flags().Bool("dev", false, "Run in development mode")
	serverStartCmd.Flags().Bool("detach", true, "Run in background")
}

func runServerStart(cmd *cobra.Command, args []string) error {
	detach, _ := cmd.Flags().GetBool("detach")

	composeFile := findComposeFile()
	if composeFile == "" {
		return fmt.Errorf("docker-compose.yml not found. Run from the contextops root directory or set CONTEXTOPS_COMPOSE_FILE")
	}

	cmdArgs := []string{"compose", "-f", composeFile, "up"}
	if detach {
		cmdArgs = append(cmdArgs, "-d")
	}

	fmt.Println("Starting ContextOps services...")
	c := exec.Command("docker", cmdArgs...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("docker compose up failed: %w", err)
	}

	if detach {
		fmt.Println()
		fmt.Println("ContextOps services started:")
		fmt.Println("  API:       http://localhost:8080")
		fmt.Println("  Evaluator: http://localhost:8081")
		fmt.Println("  UI:        http://localhost:3000")
		fmt.Println()
		fmt.Println("Use 'contextops server status' to check health")
		fmt.Println("Use 'contextops server stop' to shut down")
	}

	return nil
}

func runServerStop(cmd *cobra.Command, args []string) error {
	composeFile := findComposeFile()
	if composeFile == "" {
		return fmt.Errorf("docker-compose.yml not found")
	}

	fmt.Println("Stopping ContextOps services...")
	c := exec.Command("docker", "compose", "-f", composeFile, "down")
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func runServerStatus(cmd *cobra.Command, args []string) error {
	composeFile := findComposeFile()
	if composeFile == "" {
		return fmt.Errorf("docker-compose.yml not found")
	}

	c := exec.Command("docker", "compose", "-f", composeFile, "ps")
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func findComposeFile() string {
	candidates := []string{
		"docker-compose.yml",
		"docker-compose.yaml",
		"../docker-compose.yml",
		"../../docker-compose.yml",
	}
	if f := os.Getenv("CONTEXTOPS_COMPOSE_FILE"); f != "" {
		return f
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}
