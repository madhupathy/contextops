package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate reports",
}

var reportGenerateCmd = &cobra.Command{
	Use:   "generate <run-id>",
	Short: "Generate a human-readable report for a run",
	Args:  cobra.ExactArgs(1),
	RunE:  runReportGenerate,
}

func init() {
	rootCmd.AddCommand(reportCmd)
	reportCmd.AddCommand(reportGenerateCmd)

	reportGenerateCmd.Flags().String("format", "text", "Output format (text, markdown, json, html)")
}

func runReportGenerate(cmd *cobra.Command, args []string) error {
	runID := args[0]
	format, _ := cmd.Flags().GetString("format")

	timeline, err := fetchTimeline(runID)
	if err != nil {
		return fmt.Errorf("fetch run data: %w", err)
	}

	run, _ := timeline["run"].(map[string]interface{})
	rc, _ := timeline["retrieval_candidates"].([]interface{})
	mc, _ := timeline["memory_candidates"].([]interface{})
	tc, _ := timeline["tool_calls"].([]interface{})
	evals, _ := timeline["evaluations"].([]interface{})

	switch format {
	case "json":
		out, _ := json.MarshalIndent(timeline, "", "  ")
		fmt.Println(string(out))
		return nil
	case "markdown":
		return renderMarkdown(runID, run, rc, mc, tc, evals)
	default:
		return renderText(runID, run, rc, mc, tc, evals)
	}
}

func renderText(runID string, run map[string]interface{}, rc, mc, tc, evals []interface{}) error {
	w := 70
	fmt.Println(strings.Repeat("=", w))
	fmt.Printf("  ContextOps Run Report: %s\n", runID)
	fmt.Println(strings.Repeat("=", w))

	fmt.Printf("\n  Query:   %v\n", run["query"])
	fmt.Printf("  Model:   %v\n", run["model"])
	fmt.Printf("  Status:  %v\n", run["status"])
	fmt.Printf("  Tokens:  %v\n", run["total_tokens"])
	fmt.Printf("  Latency: %vms\n", run["latency_ms"])
	fmt.Printf("  Cost:    $%v\n", run["estimated_cost"])

	if answer, ok := run["final_answer"]; ok && answer != nil {
		fmt.Printf("\n  Answer:\n    %v\n", answer)
	}

	if len(rc) > 0 {
		fmt.Printf("\n%s\n  Retrieval (%d candidates)\n%s\n", strings.Repeat("-", w), len(rc), strings.Repeat("-", w))
		for i, c := range rc {
			if cm, ok := c.(map[string]interface{}); ok {
				flags := []string{}
				if s, ok := cm["selected"].(bool); ok && s {
					flags = append(flags, "SELECTED")
				}
				if a, ok := cm["acl_passed"].(bool); ok && !a {
					flags = append(flags, "ACL-BLOCKED")
				}
				flagStr := ""
				if len(flags) > 0 {
					flagStr = " [" + strings.Join(flags, ", ") + "]"
				}
				fmt.Printf("  %d. %v (score: %v)%s\n", i+1, cm["title"], cm["score"], flagStr)
			}
		}
	}

	if len(mc) > 0 {
		fmt.Printf("\n%s\n  Memory (%d candidates)\n%s\n", strings.Repeat("-", w), len(mc), strings.Repeat("-", w))
		for i, m := range mc {
			if mm, ok := m.(map[string]interface{}); ok {
				flags := []string{}
				if s, ok := mm["selected"].(bool); ok && s {
					flags = append(flags, "SELECTED")
				}
				if s, ok := mm["is_stale"].(bool); ok && s {
					flags = append(flags, "STALE")
				}
				flagStr := ""
				if len(flags) > 0 {
					flagStr = " [" + strings.Join(flags, ", ") + "]"
				}
				fmt.Printf("  %d. [%v] %v%s\n", i+1, mm["memory_type"], mm["content"], flagStr)
			}
		}
	}

	if len(tc) > 0 {
		fmt.Printf("\n%s\n  Tool Calls (%d)\n%s\n", strings.Repeat("-", w), len(tc), strings.Repeat("-", w))
		for i, t := range tc {
			if tm, ok := t.(map[string]interface{}); ok {
				fmt.Printf("  %d. %v → %v (%vms)\n", i+1, tm["tool_name"], tm["status"], tm["latency_ms"])
			}
		}
	}

	if len(evals) > 0 {
		fmt.Printf("\n%s\n  Evaluations\n%s\n", strings.Repeat("-", w), strings.Repeat("-", w))
		for _, e := range evals {
			if em, ok := e.(map[string]interface{}); ok {
				pass := "\033[31mFAIL\033[0m"
				if p, ok := em["passed"].(bool); ok && p {
					pass = "\033[32mPASS\033[0m"
				}
				score, _ := em["score"].(float64)
				fmt.Printf("  [%s] %-25v %3.0f%%\n", pass, em["category"], score*100)
				if r, ok := em["reasoning"].(string); ok && r != "" {
					fmt.Printf("         %s\n", r)
				}
			}
		}
	}

	fmt.Println()
	fmt.Println(strings.Repeat("=", w))
	return nil
}

func renderMarkdown(runID string, run map[string]interface{}, rc, mc, tc, evals []interface{}) error {
	fmt.Printf("# ContextOps Run Report\n\n")
	fmt.Printf("**Run ID:** `%s`\n\n", runID)
	fmt.Printf("| Metric | Value |\n|--------|-------|\n")
	fmt.Printf("| Query | %v |\n", run["query"])
	fmt.Printf("| Model | %v |\n", run["model"])
	fmt.Printf("| Status | %v |\n", run["status"])
	fmt.Printf("| Tokens | %v |\n", run["total_tokens"])
	fmt.Printf("| Latency | %vms |\n", run["latency_ms"])
	fmt.Printf("| Cost | $%v |\n", run["estimated_cost"])

	if answer, ok := run["final_answer"]; ok && answer != nil {
		fmt.Printf("\n## Answer\n\n> %v\n", answer)
	}

	if len(rc) > 0 {
		fmt.Printf("\n## Retrieval Candidates (%d)\n\n", len(rc))
		fmt.Printf("| # | Title | Score | Selected | ACL |\n|---|-------|-------|----------|-----|\n")
		for i, c := range rc {
			if cm, ok := c.(map[string]interface{}); ok {
				sel := "No"
				if s, ok := cm["selected"].(bool); ok && s {
					sel = "Yes"
				}
				acl := "Pass"
				if a, ok := cm["acl_passed"].(bool); ok && !a {
					acl = "Blocked"
				}
				fmt.Printf("| %d | %v | %v | %s | %s |\n", i+1, cm["title"], cm["score"], sel, acl)
			}
		}
	}

	if len(evals) > 0 {
		fmt.Printf("\n## Evaluations\n\n")
		fmt.Printf("| Category | Score | Status | Reasoning |\n|----------|-------|--------|----------|\n")
		for _, e := range evals {
			if em, ok := e.(map[string]interface{}); ok {
				pass := "FAIL"
				if p, ok := em["passed"].(bool); ok && p {
					pass = "PASS"
				}
				score, _ := em["score"].(float64)
				reasoning, _ := em["reasoning"].(string)
				if len(reasoning) > 60 {
					reasoning = reasoning[:60] + "..."
				}
				fmt.Printf("| %v | %.0f%% | %s | %s |\n", em["category"], score*100, pass, reasoning)
			}
		}
	}

	return nil
}
