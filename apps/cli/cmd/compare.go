package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
)

var compareCmd = &cobra.Command{
	Use:   "compare <baseline-run> <candidate-run>",
	Short: "Compare runs, datasets, or benchmarks",
	Args:  cobra.ExactArgs(2),
	RunE:  runCompare,
}

func init() {
	rootCmd.AddCommand(compareCmd)
	compareCmd.Flags().String("metric", "", "Focus on a specific metric")
}

func runCompare(cmd *cobra.Command, args []string) error {
	baselineID := args[0]
	candidateID := args[1]
	metricFilter, _ := cmd.Flags().GetString("metric")

	payload := map[string]interface{}{
		"baseline_run_id":  baselineID,
		"candidate_run_id": candidateID,
	}
	body, _ := json.Marshal(payload)

	url := fmt.Sprintf("%s/api/v1/compare", apiURL())
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		// If API not yet implemented, show a local comparison
		return showLocalCompare(baselineID, candidateID, metricFilter)
	}

	formatted, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(formatted))
	return nil
}

func showLocalCompare(baselineID, candidateID, metricFilter string) error {
	// Fetch both timelines and compare locally
	baseline, err := fetchTimeline(baselineID)
	if err != nil {
		return fmt.Errorf("fetch baseline: %w", err)
	}
	candidate, err := fetchTimeline(candidateID)
	if err != nil {
		return fmt.Errorf("fetch candidate: %w", err)
	}

	bRun, _ := baseline["run"].(map[string]interface{})
	cRun, _ := candidate["run"].(map[string]interface{})

	fmt.Printf("Comparison: %s vs %s\n", baselineID, candidateID)
	fmt.Println(strings.Repeat("=", 70))

	// Run-level metrics
	fmt.Printf("\n%-25s %-20s %-20s %s\n", "METRIC", "BASELINE", "CANDIDATE", "DELTA")
	fmt.Println(strings.Repeat("-", 70))

	compareMetric("Status", bRun["status"], cRun["status"], metricFilter)
	compareNumeric("Tokens", bRun["total_tokens"], cRun["total_tokens"], metricFilter, false)
	compareNumeric("Latency (ms)", bRun["latency_ms"], cRun["latency_ms"], metricFilter, false)
	compareNumeric("Cost ($)", bRun["estimated_cost"], cRun["estimated_cost"], metricFilter, false)
	compareMetric("Model", bRun["model"], cRun["model"], metricFilter)

	// Retrieval comparison
	bRC, _ := baseline["retrieval_candidates"].([]interface{})
	cRC, _ := candidate["retrieval_candidates"].([]interface{})
	compareNumeric("Retrieval Candidates", float64(len(bRC)), float64(len(cRC)), metricFilter, false)

	// Evaluation comparison
	bEvals, _ := baseline["evaluations"].([]interface{})
	cEvals, _ := candidate["evaluations"].([]interface{})

	if len(bEvals) > 0 || len(cEvals) > 0 {
		fmt.Printf("\n\nEvaluation Scores\n")
		fmt.Println(strings.Repeat("-", 70))

		bScores := evalMap(bEvals)
		cScores := evalMap(cEvals)

		categories := mergeKeys(bScores, cScores)
		for _, cat := range categories {
			if metricFilter != "" && cat != metricFilter {
				continue
			}
			bScore := bScores[cat]
			cScore := cScores[cat]
			delta := cScore - bScore
			direction := ""
			if delta > 0.01 {
				direction = fmt.Sprintf("\033[32m+%.0f%%\033[0m", delta*100)
			} else if delta < -0.01 {
				direction = fmt.Sprintf("\033[31m%.0f%%\033[0m", delta*100)
			} else {
				direction = "="
			}
			fmt.Printf("  %-23s %.0f%%                %.0f%%                %s\n",
				cat, bScore*100, cScore*100, direction)
		}
	}

	fmt.Println()
	return nil
}

func fetchTimeline(runID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/v1/runs/%s/timeline", apiURL(), runID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-Tenant-ID", tenantID())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	return result, err
}

func compareMetric(name string, baseline, candidate interface{}, filter string) {
	if filter != "" && name != filter {
		return
	}
	bStr := fmt.Sprintf("%v", baseline)
	cStr := fmt.Sprintf("%v", candidate)
	delta := "="
	if bStr != cStr {
		delta = "CHANGED"
	}
	fmt.Printf("  %-23s %-20s %-20s %s\n", name, bStr, cStr, delta)
}

func compareNumeric(name string, baseline, candidate interface{}, filter string, higherBetter bool) {
	if filter != "" && name != filter {
		return
	}
	bVal := toFloat(baseline)
	cVal := toFloat(candidate)
	delta := cVal - bVal

	direction := "="
	if math.Abs(delta) > 0.001 {
		pct := (delta / math.Max(bVal, 0.001)) * 100
		if (higherBetter && delta > 0) || (!higherBetter && delta < 0) {
			direction = fmt.Sprintf("\033[32m%+.1f (%.1f%%)\033[0m", delta, pct)
		} else {
			direction = fmt.Sprintf("\033[31m%+.1f (%.1f%%)\033[0m", delta, pct)
		}
	}

	fmt.Printf("  %-23s %-20v %-20v %s\n", name, baseline, candidate, direction)
}

func toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0
	}
}

func evalMap(evals []interface{}) map[string]float64 {
	m := make(map[string]float64)
	for _, e := range evals {
		if em, ok := e.(map[string]interface{}); ok {
			cat, _ := em["category"].(string)
			score, _ := em["score"].(float64)
			m[cat] = score
		}
	}
	return m
}

func mergeKeys(a, b map[string]float64) []string {
	seen := map[string]bool{}
	var keys []string
	for k := range a {
		if !seen[k] {
			keys = append(keys, k)
			seen[k] = true
		}
	}
	for k := range b {
		if !seen[k] {
			keys = append(keys, k)
			seen[k] = true
		}
	}
	return keys
}
