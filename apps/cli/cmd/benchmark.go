package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
)

var benchmarkCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "Execute benchmark suites",
}

var benchmarkListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available benchmark suites",
	RunE:  runBenchmarkList,
}

var benchmarkRunCmd = &cobra.Command{
	Use:   "run <suite-name>",
	Short: "Run a benchmark suite",
	Args:  cobra.ExactArgs(1),
	RunE:  runBenchmarkRun,
}

func init() {
	rootCmd.AddCommand(benchmarkCmd)
	benchmarkCmd.AddCommand(benchmarkListCmd)
	benchmarkCmd.AddCommand(benchmarkRunCmd)

	benchmarkRunCmd.Flags().String("dataset", "", "Path to dataset directory")
	benchmarkRunCmd.Flags().String("profile", "", "Evaluation profile (e.g. nightly, prod-safety)")
}

func runBenchmarkList(cmd *cobra.Command, args []string) error {
	url := fmt.Sprintf("%s/api/v1/benchmarks", apiURL())
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-Tenant-ID", tenantID())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	var suites []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&suites)

	if len(suites) == 0 {
		fmt.Println("No benchmark suites found.")
		fmt.Println()
		fmt.Println("Built-in benchmark packs:")
		fmt.Println("  enterprise-search   Policy retrieval, role-sensitive answers, doc freshness")
		fmt.Println("  document-copilot    Clause lookup, summarization grounding, access control")
		fmt.Println("  memory-assistant    Preference tracking, stale memory, conflict resolution")
		fmt.Println("  workflow-agent      Tool sequencing, approvals, side effects, completion")
		return nil
	}

	fmt.Printf("%-38s %-20s %-15s\n", "ID", "NAME", "CATEGORY")
	fmt.Println(strings.Repeat("-", 75))
	for _, s := range suites {
		fmt.Printf("%-38v %-20v %-15v\n", s["id"], s["name"], s["category"])
	}
	return nil
}

func runBenchmarkRun(cmd *cobra.Command, args []string) error {
	suite := args[0]
	fmt.Printf("Running benchmark suite: %s\n", suite)
	fmt.Println(strings.Repeat("=", 60))

	// Check if it's a built-in benchmark
	builtIn := map[string]string{
		"enterprise-search": "Policy retrieval, role-sensitive answers, doc freshness, citation correctness",
		"document-copilot":  "Clause lookup, summarization grounding, extraction correctness, access control",
		"memory-assistant":  "Preference tracking, stale memory suppression, conflict resolution",
		"workflow-agent":    "Tool sequencing, approvals, side effects, completion accuracy",
	}

	if desc, ok := builtIn[suite]; ok {
		fmt.Printf("\nSuite: %s\n", suite)
		fmt.Printf("Tests: %s\n", desc)
		fmt.Println()
	}

	// Try to run via API
	url := fmt.Sprintf("%s/api/v1/benchmarks", apiURL())
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-Tenant-ID", tenantID())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("API not available. Ensure services are running: contextops server start")
		return nil
	}
	defer resp.Body.Close()

	var suites []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&suites)

	var suiteID string
	for _, s := range suites {
		if name, ok := s["name"].(string); ok && name == suite {
			suiteID = fmt.Sprintf("%v", s["id"])
			break
		}
	}

	if suiteID == "" {
		fmt.Printf("Benchmark suite '%s' not found in API. Create it first or use a built-in pack.\n", suite)
		fmt.Println("(Built-in benchmark execution will be available in v0.2)")
		return nil
	}

	fmt.Printf("Executing suite %s...\n", suiteID)
	fmt.Println("(Full benchmark execution will be available in v0.2)")
	return nil
}
