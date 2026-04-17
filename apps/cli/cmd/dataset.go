package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var datasetCmd = &cobra.Command{
	Use:   "dataset",
	Short: "Manage evaluation datasets",
}

var datasetListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available datasets",
	RunE:  runDatasetList,
}

var datasetCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new dataset",
	Args:  cobra.ExactArgs(1),
	RunE:  runDatasetCreate,
}

var datasetValidateCmd = &cobra.Command{
	Use:   "validate <path>",
	Short: "Validate a dataset directory",
	Args:  cobra.ExactArgs(1),
	RunE:  runDatasetValidate,
}

func init() {
	rootCmd.AddCommand(datasetCmd)
	datasetCmd.AddCommand(datasetListCmd)
	datasetCmd.AddCommand(datasetCreateCmd)
	datasetCmd.AddCommand(datasetValidateCmd)
}

func runDatasetList(cmd *cobra.Command, args []string) error {
	dirs := []string{
		".contextops/datasets",
		"benchmarks",
		"datasets",
	}

	found := false
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				cases, _ := filepath.Glob(filepath.Join(dir, e.Name(), "*.json"))
				casesL, _ := filepath.Glob(filepath.Join(dir, e.Name(), "*.jsonl"))
				total := len(cases) + len(casesL)
				fmt.Printf("  %-30s %d cases  (%s)\n", e.Name(), total, filepath.Join(dir, e.Name()))
				found = true
			}
		}
	}

	if !found {
		fmt.Println("No datasets found.")
		fmt.Println()
		fmt.Println("Create one with: contextops dataset create <name>")
	}
	return nil
}

func runDatasetCreate(cmd *cobra.Command, args []string) error {
	name := args[0]
	dir := filepath.Join(".contextops", "datasets", name)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dataset dir: %w", err)
	}

	// Create a README
	readme := fmt.Sprintf(`# Dataset: %s

Place test case JSON files in this directory.

Each file should contain a trace with expected outcomes:

%sjson
{
  "query": "What is the latest PTO policy?",
  "expected_answer": "The 2025 PTO policy allows...",
  "expected_sources": ["doc-new-pto"],
  "retrieval": {
    "candidates": [...],
    "selected": [...]
  },
  "output": {
    "final_answer": "..."
  }
}
%s

Run evaluation: contextops eval dataset %s
`, name, "```", "```", name)

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644); err != nil {
		return fmt.Errorf("write README: %w", err)
	}

	fmt.Printf("Created dataset: %s\n", dir)
	fmt.Printf("Add test case JSON files to get started.\n")
	return nil
}

func runDatasetValidate(cmd *cobra.Command, args []string) error {
	path := args[0]
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot access %s: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}

	cases, _ := filepath.Glob(filepath.Join(path, "*.json"))
	casesL, _ := filepath.Glob(filepath.Join(path, "*.jsonl"))
	total := len(cases) + len(casesL)

	if total == 0 {
		fmt.Printf("WARNING: No JSON files found in %s\n", path)
		return nil
	}

	fmt.Printf("Dataset: %s (%d files)\n", path, total)
	fmt.Println(strings.Repeat("-", 50))

	valid := 0
	for _, f := range append(cases, casesL...) {
		data, err := os.ReadFile(f)
		if err != nil {
			fmt.Printf("  ERROR  %s: cannot read\n", filepath.Base(f))
			continue
		}
		if len(data) < 2 {
			fmt.Printf("  ERROR  %s: empty file\n", filepath.Base(f))
			continue
		}
		fmt.Printf("  OK     %s (%d bytes)\n", filepath.Base(f), len(data))
		valid++
	}

	fmt.Printf("\nValid: %d/%d\n", valid, total)
	return nil
}
