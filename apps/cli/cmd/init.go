package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a ContextOps project",
	Long:  `Creates a .contextops/ directory with default config.yaml and gates.yaml.`,
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().String("name", "", "Project name")
	initCmd.Flags().String("template", "", "Template to use (rag, workflow-agent, memory-assistant)")
}

func runInit(cmd *cobra.Command, args []string) error {
	dir := ".contextops"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create .contextops/: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "benchmarks"), 0o755); err != nil {
		return fmt.Errorf("create benchmarks dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "datasets"), 0o755); err != nil {
		return fmt.Errorf("create datasets dir: %w", err)
	}

	name, _ := cmd.Flags().GetString("name")
	if name == "" {
		cwd, _ := os.Getwd()
		name = filepath.Base(cwd)
	}

	configContent := fmt.Sprintf(`# ContextOps configuration
project:
  name: %s

api:
  url: http://localhost:8080

evaluator:
  url: http://localhost:8081

tenant:
  default: "00000000-0000-0000-0000-000000000001"
`, name)

	gatesContent := `# ContextOps gate configuration
# Used by: contextops gate check --config .contextops/gates.yaml

minimum_scores:
  correctness: 0.90
  groundedness: 0.85
  retrieval_recall: 0.80
  permission_safety: 1.00
  task_completion: 0.85

maximum_thresholds:
  latency_ms_p95: 5000
  cost_per_run_usd: 0.10
  memory_staleness_rate: 0.10
`

	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		return fmt.Errorf("write config.yaml: %w", err)
	}

	gatesPath := filepath.Join(dir, "gates.yaml")
	if err := os.WriteFile(gatesPath, []byte(gatesContent), 0o644); err != nil {
		return fmt.Errorf("write gates.yaml: %w", err)
	}

	fmt.Println("Initialized ContextOps project:")
	fmt.Printf("  %s/config.yaml\n", dir)
	fmt.Printf("  %s/gates.yaml\n", dir)
	fmt.Printf("  %s/benchmarks/\n", dir)
	fmt.Printf("  %s/datasets/\n", dir)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  contextops server start       # start local services")
	fmt.Println("  contextops trace ingest <file> # ingest a trace")
	fmt.Println("  contextops eval run <run-id>   # evaluate a run")

	return nil
}
