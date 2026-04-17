package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var evalCmd = &cobra.Command{
	Use:   "eval",
	Short: "Run evaluators on runs or datasets",
}

var evalRunCmd = &cobra.Command{
	Use:   "run <run-id>",
	Short: "Evaluate a single run",
	Args:  cobra.ExactArgs(1),
	RunE:  runEvalRun,
}

var evalDatasetCmd = &cobra.Command{
	Use:   "dataset <name>",
	Short: "Evaluate all runs in a dataset",
	Args:  cobra.ExactArgs(1),
	RunE:  runEvalDataset,
}

func init() {
	rootCmd.AddCommand(evalCmd)
	evalCmd.AddCommand(evalRunCmd)
	evalCmd.AddCommand(evalDatasetCmd)

	evalRunCmd.Flags().String("only", "", "Comma-separated list of evaluators to run")
	evalRunCmd.Flags().Bool("explain", false, "Show detailed explanations")
	evalRunCmd.Flags().String("profile", "", "Evaluator profile to use")
}

func evalURL() string {
	u := viper.GetString("evaluator.url")
	if u == "" {
		return "http://localhost:8081"
	}
	return u
}

func runEvalRun(cmd *cobra.Command, args []string) error {
	runID := args[0]
	only, _ := cmd.Flags().GetString("only")
	explain, _ := cmd.Flags().GetBool("explain")

	payload := map[string]interface{}{
		"run_id":    runID,
		"tenant_id": tenantID(),
	}
	if only != "" {
		payload["categories"] = strings.Split(only, ",")
	}

	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/evaluate", evalURL())

	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("evaluator request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("evaluator returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		RunID   string `json:"run_id"`
		Results []struct {
			EvaluatorName string                 `json:"evaluator_name"`
			Category      string                 `json:"category"`
			Score         float64                `json:"score"`
			Passed        bool                   `json:"passed"`
			Reasoning     string                 `json:"reasoning"`
			Details       map[string]interface{} `json:"details"`
			EvalLatencyMs int                    `json:"eval_latency_ms"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	fmt.Printf("Evaluation Results for %s\n", runID)
	fmt.Println(strings.Repeat("=", 60))

	passCount := 0
	failCount := 0

	for _, r := range result.Results {
		status := "FAIL"
		statusColor := "31" // red
		if r.Passed {
			status = "PASS"
			statusColor = "32" // green
			passCount++
		} else {
			failCount++
		}

		scorePct := int(r.Score * 100)
		bar := renderBar(scorePct)

		fmt.Printf("\n  \033[%sm[%s]\033[0m %-25s %s %d%%\n",
			statusColor, status, r.Category, bar, scorePct)

		if explain && r.Reasoning != "" {
			fmt.Printf("         %s\n", r.Reasoning)
		}
		if explain && r.EvalLatencyMs > 0 {
			fmt.Printf("         (evaluated in %dms)\n", r.EvalLatencyMs)
		}
	}

	fmt.Println()
	fmt.Println(strings.Repeat("-", 60))
	total := passCount + failCount
	fmt.Printf("  Total: %d  Passed: \033[32m%d\033[0m  Failed: \033[31m%d\033[0m\n", total, passCount, failCount)

	if failCount > 0 {
		fmt.Println("\n  Result: \033[31mFAIL\033[0m")
	} else {
		fmt.Println("\n  Result: \033[32mPASS\033[0m")
	}

	return nil
}

func runEvalDataset(cmd *cobra.Command, args []string) error {
	name := args[0]
	fmt.Printf("Evaluating dataset: %s\n", name)
	fmt.Println("(Dataset evaluation will be available in v0.2)")
	return nil
}

func renderBar(pct int) string {
	filled := pct / 5
	if filled > 20 {
		filled = 20
	}
	empty := 20 - filled
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", empty) + "]"
}
