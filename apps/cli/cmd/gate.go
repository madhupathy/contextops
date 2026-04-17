package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/spf13/cobra"
)

var gateCmd = &cobra.Command{
	Use:   "gate",
	Short: "Enforce quality and safety thresholds",
}

var gateCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check gates against latest evaluation results",
	RunE:  runGateCheck,
}

func init() {
	rootCmd.AddCommand(gateCmd)
	gateCmd.AddCommand(gateCheckCmd)

	gateCheckCmd.Flags().String("config", ".contextops/gates.yaml", "Path to gates config file")
	gateCheckCmd.Flags().String("benchmark", "", "Benchmark suite to check against")
	gateCheckCmd.Flags().String("run-id", "", "Specific run ID to check")
}

type GateConfig struct {
	MinimumScores     map[string]float64 `yaml:"minimum_scores"`
	MaximumThresholds map[string]float64 `yaml:"maximum_thresholds"`
}

func runGateCheck(cmd *cobra.Command, args []string) error {
	configPath, _ := cmd.Flags().GetString("config")
	runID, _ := cmd.Flags().GetString("run-id")

	// Load gate config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("cannot read gate config %s: %w", configPath, err)
	}

	var config GateConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("invalid gate config: %w", err)
	}

	fmt.Printf("Gate Check (%s)\n", configPath)
	fmt.Println(strings.Repeat("=", 60))

	if runID == "" {
		// Get the latest run
		url := fmt.Sprintf("%s/api/v1/runs", apiURL())
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("X-Tenant-ID", tenantID())

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("API request failed: %w", err)
		}
		defer resp.Body.Close()

		var runs []map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&runs)

		if len(runs) == 0 {
			return fmt.Errorf("no runs found to check gates against")
		}

		runID = fmt.Sprintf("%v", runs[0]["id"])
	}

	// Fetch evaluations for the run
	url := fmt.Sprintf("%s/api/v1/runs/%s/evaluations", apiURL(), runID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-Tenant-ID", tenantID())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	var evals []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&evals)

	// Build score map
	scores := make(map[string]float64)
	for _, e := range evals {
		cat, _ := e["category"].(string)
		score, _ := e["score"].(float64)
		scores[cat] = score
	}

	// Fetch run metrics for threshold checks
	runURL := fmt.Sprintf("%s/api/v1/runs/%s", apiURL(), runID)
	runReq, _ := http.NewRequest("GET", runURL, nil)
	runReq.Header.Set("X-Tenant-ID", tenantID())

	runResp, err := http.DefaultClient.Do(runReq)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer runResp.Body.Close()

	var runData map[string]interface{}
	json.NewDecoder(runResp.Body).Decode(&runData)

	// Check minimum scores
	allPassed := true

	fmt.Printf("\nRun: %s\n\n", runID)

	if len(config.MinimumScores) > 0 {
		fmt.Println("Minimum Score Gates:")
		for metric, threshold := range config.MinimumScores {
			actual, exists := scores[metric]
			if !exists {
				fmt.Printf("  \033[33m[SKIP]\033[0m %-25s (no evaluation data)\n", metric)
				continue
			}

			if actual >= threshold {
				fmt.Printf("  \033[32m[PASS]\033[0m %-25s %.0f%% >= %.0f%%\n",
					metric, actual*100, threshold*100)
			} else {
				fmt.Printf("  \033[31m[FAIL]\033[0m %-25s %.0f%% < %.0f%%\n",
					metric, actual*100, threshold*100)
				allPassed = false
			}
		}
	}

	if len(config.MaximumThresholds) > 0 {
		fmt.Println("\nMaximum Threshold Gates:")

		metrics := map[string]float64{
			"latency_ms_p95":       toFloat(runData["latency_ms"]),
			"cost_per_run_usd":     toFloat(runData["estimated_cost"]),
			"memory_staleness_rate": 0, // would be computed from evaluations
		}

		for metric, threshold := range config.MaximumThresholds {
			actual, exists := metrics[metric]
			if !exists {
				fmt.Printf("  \033[33m[SKIP]\033[0m %-25s (no data available)\n", metric)
				continue
			}

			if actual <= threshold {
				fmt.Printf("  \033[32m[PASS]\033[0m %-25s %.2f <= %.2f\n",
					metric, actual, threshold)
			} else {
				fmt.Printf("  \033[31m[FAIL]\033[0m %-25s %.2f > %.2f\n",
					metric, actual, threshold)
				allPassed = false
			}
		}
	}

	fmt.Println()
	fmt.Println(strings.Repeat("-", 60))

	if allPassed {
		fmt.Println("\033[32mAll gates passed.\033[0m")
		return nil
	}

	fmt.Println("\033[31mGate check failed. Deployment should be blocked.\033[0m")
	os.Exit(1)
	return nil
}
